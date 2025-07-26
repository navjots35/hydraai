package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/hydraai/hydra-route/internal/metrics"
	"github.com/hydraai/hydra-route/internal/scaler"
	"github.com/hydraai/hydra-route/pkg/config"
)

const (
	HydraRouteAnnotation            = "hydra-route.ai/enabled"
	HydraRouteMinReplicasAnnotation = "hydra-route.ai/min-replicas"
	HydraRouteMaxReplicasAnnotation = "hydra-route.ai/max-replicas"
	HydraRouteTargetAnnotation      = "hydra-route.ai/target"
	RequeueAfter                    = 30 * time.Second
)

// HydraRouteReconciler reconciles ingress resources and manages scaling
type HydraRouteReconciler struct {
	client.Client
	Scheme           *runtime.Scheme
	MetricsCollector *metrics.Collector
	AIScaler         *scaler.AIScaler
	Config           *config.Config
}

// NewController creates a new controller for HydraRoute
func NewController(mgr manager.Manager, reconciler *HydraRouteReconciler) (controller.Controller, error) {
	// Create controller
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Owns(&appsv1.Deployment{}).
		Build(reconciler)

	if err != nil {
		return nil, err
	}

	return c, nil
}

// Reconcile processes ingress resources and makes scaling decisions
func (r *HydraRouteReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logrus.WithFields(logrus.Fields{
		"namespace": req.Namespace,
		"name":      req.Name,
	})

	log.Debug("Starting reconciliation")

	// Get the ingress resource
	ingress := &networkingv1.Ingress{}
	if err := r.Get(ctx, req.NamespacedName, ingress); err != nil {
		log.WithError(err).Debug("Unable to fetch ingress")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if HydraRoute is enabled for this ingress
	if !r.isHydraRouteEnabled(ingress) {
		log.Debug("HydraRoute not enabled for this ingress")
		return ctrl.Result{}, nil
	}

	// Process each service referenced by the ingress
	for _, rule := range ingress.Spec.Rules {
		if rule.HTTP == nil {
			continue
		}

		for _, path := range rule.HTTP.Paths {
			serviceName := path.Backend.Service.Name
			if serviceName == "" {
				continue
			}

			if err := r.processService(ctx, serviceName, req.Namespace, ingress); err != nil {
				log.WithError(err).WithField("service", serviceName).Error("Failed to process service")
				continue
			}
		}
	}

	log.Debug("Reconciliation completed")
	return ctrl.Result{RequeueAfter: RequeueAfter}, nil
}

// processService handles scaling decisions for a specific service
func (r *HydraRouteReconciler) processService(ctx context.Context, serviceName, namespace string, ingress *networkingv1.Ingress) error {
	log := logrus.WithFields(logrus.Fields{
		"service":   serviceName,
		"namespace": namespace,
	})

	// Get current metrics for the service
	metricsData := r.MetricsCollector.GetLatestMetrics(serviceName, namespace)
	if metricsData == nil {
		log.Debug("No metrics available for service")
		return nil
	}

	// Make scaling decision using AI
	decision, err := r.AIScaler.MakeScalingDecision(metricsData)
	if err != nil {
		return fmt.Errorf("failed to make scaling decision: %w", err)
	}

	if decision == nil {
		log.Debug("No scaling decision made (possibly in cooldown)")
		return nil
	}

	log.WithFields(logrus.Fields{
		"current_replicas":     decision.CurrentReplicas,
		"recommended_replicas": decision.RecommendedReplicas,
		"confidence":           decision.Confidence,
		"reasoning":            decision.Reasoning,
	}).Info("Scaling decision made")

	// Skip if no scaling is needed
	if decision.CurrentReplicas == decision.RecommendedReplicas {
		log.Debug("No scaling needed")
		return nil
	}

	// Apply scaling decision
	if err := r.applyScalingDecision(ctx, decision, ingress); err != nil {
		return fmt.Errorf("failed to apply scaling decision: %w", err)
	}

	// Record the scaling event
	if err := r.recordScalingEvent(ctx, decision, ingress); err != nil {
		log.WithError(err).Warn("Failed to record scaling event")
	}

	return nil
}

// applyScalingDecision applies the scaling decision to the deployment
func (r *HydraRouteReconciler) applyScalingDecision(ctx context.Context, decision *scaler.ScalingDecision, ingress *networkingv1.Ingress) error {
	// Find the deployment for the service
	deployment, err := r.findServiceDeployment(ctx, decision.ServiceName, decision.Namespace)
	if err != nil {
		return fmt.Errorf("failed to find deployment: %w", err)
	}

	if deployment == nil {
		return fmt.Errorf("no deployment found for service %s", decision.ServiceName)
	}

	// Check if we should perform dry run
	if r.Config.General.DryRun {
		logrus.WithFields(logrus.Fields{
			"service":              decision.ServiceName,
			"namespace":            decision.Namespace,
			"current_replicas":     decision.CurrentReplicas,
			"recommended_replicas": decision.RecommendedReplicas,
		}).Info("DRY RUN: Would scale deployment")
		return nil
	}

	// Update deployment replicas
	updatedDeployment := deployment.DeepCopy()
	updatedDeployment.Spec.Replicas = &decision.RecommendedReplicas

	// Add annotations for tracking
	if updatedDeployment.Annotations == nil {
		updatedDeployment.Annotations = make(map[string]string)
	}
	updatedDeployment.Annotations["hydra-route.ai/last-scaled"] = time.Now().Format(time.RFC3339)
	updatedDeployment.Annotations["hydra-route.ai/scale-reason"] = decision.Reasoning
	updatedDeployment.Annotations["hydra-route.ai/confidence"] = fmt.Sprintf("%.2f", decision.Confidence)

	if err := r.Update(ctx, updatedDeployment); err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"service":              decision.ServiceName,
		"namespace":            decision.Namespace,
		"current_replicas":     decision.CurrentReplicas,
		"recommended_replicas": decision.RecommendedReplicas,
		"confidence":           decision.Confidence,
	}).Info("Successfully scaled deployment")

	return nil
}

// findServiceDeployment finds the deployment that backs a service
func (r *HydraRouteReconciler) findServiceDeployment(ctx context.Context, serviceName, namespace string) (*appsv1.Deployment, error) {
	// Get the service first
	service := &v1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: serviceName, Namespace: namespace}, service); err != nil {
		return nil, err
	}

	// Get all deployments in the namespace
	deploymentList := &appsv1.DeploymentList{}
	if err := r.List(ctx, deploymentList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	// Find deployment with matching labels
	for _, deployment := range deploymentList.Items {
		if r.deploymentMatchesService(&deployment, service) {
			return &deployment, nil
		}
	}

	return nil, nil
}

// deploymentMatchesService checks if a deployment's pods would be selected by a service
func (r *HydraRouteReconciler) deploymentMatchesService(deployment *appsv1.Deployment, service *v1.Service) bool {
	// Check if deployment selector labels match service selector
	if deployment.Spec.Selector == nil || deployment.Spec.Selector.MatchLabels == nil {
		return false
	}

	for key, value := range service.Spec.Selector {
		if deploymentValue, exists := deployment.Spec.Selector.MatchLabels[key]; !exists || deploymentValue != value {
			return false
		}
	}

	return true
}

// recordScalingEvent creates an event to record the scaling decision
func (r *HydraRouteReconciler) recordScalingEvent(ctx context.Context, decision *scaler.ScalingDecision, ingress *networkingv1.Ingress) error {
	// In a real implementation, you would create a Kubernetes event
	// For now, we'll just log it
	logrus.WithFields(logrus.Fields{
		"service":              decision.ServiceName,
		"namespace":            decision.Namespace,
		"current_replicas":     decision.CurrentReplicas,
		"recommended_replicas": decision.RecommendedReplicas,
		"confidence":           decision.Confidence,
		"reasoning":            decision.Reasoning,
	}).Info("Scaling event recorded")

	return nil
}

// isHydraRouteEnabled checks if HydraRoute is enabled for an ingress
func (r *HydraRouteReconciler) isHydraRouteEnabled(ingress *networkingv1.Ingress) bool {
	if ingress.Annotations == nil {
		return false
	}

	enabled, exists := ingress.Annotations[HydraRouteAnnotation]
	if !exists {
		return false
	}

	return enabled == "true"
}

// getAnnotationValue gets an annotation value with a default
func (r *HydraRouteReconciler) getAnnotationValue(ingress *networkingv1.Ingress, key, defaultValue string) string {
	if ingress.Annotations == nil {
		return defaultValue
	}

	if value, exists := ingress.Annotations[key]; exists {
		return value
	}

	return defaultValue
}

// SetupWithManager sets up the controller with the Manager
func (r *HydraRouteReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.Ingress{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}
