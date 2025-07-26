package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/hydraai/hydra-route/pkg/config"
)

// MetricsData represents collected metrics for a service
type MetricsData struct {
	Timestamp   time.Time `json:"timestamp"`
	ServiceName string    `json:"service_name"`
	Namespace   string    `json:"namespace"`

	// Resource utilization metrics
	CPUUtilization    float64 `json:"cpu_utilization"`
	MemoryUtilization float64 `json:"memory_utilization"`

	// Request metrics
	RequestRate  float64 `json:"request_rate"`
	ResponseTime float64 `json:"response_time"`
	ErrorRate    float64 `json:"error_rate"`

	// Bandwidth metrics
	NetworkBandwidth float64 `json:"network_bandwidth"`
	IOBandwidth      float64 `json:"io_bandwidth"`

	// Pod information
	CurrentReplicas int32 `json:"current_replicas"`
	DesiredReplicas int32 `json:"desired_replicas"`

	// Additional context
	IngressClass   string `json:"ingress_class"`
	LoadBalancerIP string `json:"load_balancer_ip"`
}

// NginxMetrics represents nginx ingress controller metrics
type NginxMetrics struct {
	RequestsPerSecond float64            `json:"requests_per_second"`
	ResponseTime      float64            `json:"response_time"`
	ErrorRate         float64            `json:"error_rate"`
	ActiveConnections int64              `json:"active_connections"`
	BytesPerSecond    float64            `json:"bytes_per_second"`
	UpstreamMetrics   map[string]float64 `json:"upstream_metrics"`
}

// SystemMetrics represents system-level metrics
type SystemMetrics struct {
	NetworkIO struct {
		BytesIn  float64 `json:"bytes_in"`
		BytesOut float64 `json:"bytes_out"`
	} `json:"network_io"`
	DiskIO struct {
		ReadBytesPerSec  float64 `json:"read_bytes_per_sec"`
		WriteBytesPerSec float64 `json:"write_bytes_per_sec"`
	} `json:"disk_io"`
}

// Collector manages metrics collection from various sources
type Collector struct {
	client    client.Client
	k8sClient kubernetes.Interface
	config    config.MetricsConfig

	// Metrics storage
	mu           sync.RWMutex
	metricsStore map[string][]*MetricsData

	// HTTP client for external metrics
	httpClient *http.Client

	// Collection state
	isRunning bool
	stopCh    chan struct{}
}

// NewCollector creates a new metrics collector
func NewCollector(client client.Client, cfg config.MetricsConfig) *Collector {
	return &Collector{
		client:       client,
		config:       cfg,
		metricsStore: make(map[string][]*MetricsData),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start begins metrics collection
func (c *Collector) Start(ctx context.Context) error {
	if c.isRunning {
		return fmt.Errorf("collector is already running")
	}

	c.isRunning = true
	logrus.Info("Starting metrics collector")

	// Start collection ticker
	ticker := time.NewTicker(c.config.CollectionInterval)
	defer ticker.Stop()

	// Initial collection
	if err := c.collectMetrics(ctx); err != nil {
		logrus.WithError(err).Error("Initial metrics collection failed")
	}

	for {
		select {
		case <-ctx.Done():
			logrus.Info("Stopping metrics collector due to context cancellation")
			return ctx.Err()
		case <-c.stopCh:
			logrus.Info("Stopping metrics collector")
			return nil
		case <-ticker.C:
			if err := c.collectMetrics(ctx); err != nil {
				logrus.WithError(err).Error("Metrics collection failed")
			}
		}
	}
}

// Stop stops the metrics collector
func (c *Collector) Stop() {
	if c.isRunning {
		close(c.stopCh)
		c.isRunning = false
	}
}

// GetMetrics returns metrics for a specific service
func (c *Collector) GetMetrics(serviceName, namespace string) []*MetricsData {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, serviceName)
	return c.metricsStore[key]
}

// GetLatestMetrics returns the most recent metrics for a service
func (c *Collector) GetLatestMetrics(serviceName, namespace string) *MetricsData {
	metrics := c.GetMetrics(serviceName, namespace)
	if len(metrics) == 0 {
		return nil
	}
	return metrics[len(metrics)-1]
}

// collectMetrics performs a single collection cycle
func (c *Collector) collectMetrics(ctx context.Context) error {
	logrus.Debug("Starting metrics collection cycle")

	// Get all services with ingress annotations
	services, err := c.getIngressServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get ingress services: %w", err)
	}

	// Collect metrics for each service
	for _, service := range services {
		metrics, err := c.collectServiceMetrics(ctx, service)
		if err != nil {
			logrus.WithError(err).WithFields(logrus.Fields{
				"service":   service.Name,
				"namespace": service.Namespace,
			}).Error("Failed to collect service metrics")
			continue
		}

		c.storeMetrics(metrics)
	}

	// Clean old metrics
	c.cleanOldMetrics()

	logrus.Debug("Metrics collection cycle completed")
	return nil
}

// getIngressServices finds services that are exposed via ingress
func (c *Collector) getIngressServices(ctx context.Context) ([]v1.Service, error) {
	var services []v1.Service

	// Get all services
	serviceList := &v1.ServiceList{}
	if err := c.client.List(ctx, serviceList); err != nil {
		return nil, err
	}

	// Filter services that have ingress
	for _, service := range serviceList.Items {
		// Check if service has ingress annotation or is referenced by ingress
		if c.isServiceExposed(ctx, service) {
			services = append(services, service)
		}
	}

	return services, nil
}

// isServiceExposed checks if a service is exposed via ingress
func (c *Collector) isServiceExposed(ctx context.Context, service v1.Service) bool {
	// For now, we'll consider all services as potentially exposed
	// In a real implementation, you'd check ingress resources
	return true
}

// collectServiceMetrics collects all metrics for a specific service
func (c *Collector) collectServiceMetrics(ctx context.Context, service v1.Service) (*MetricsData, error) {
	metrics := &MetricsData{
		Timestamp:   time.Now(),
		ServiceName: service.Name,
		Namespace:   service.Namespace,
	}

	// Collect resource utilization metrics
	if err := c.collectResourceMetrics(ctx, service, metrics); err != nil {
		logrus.WithError(err).Debug("Failed to collect resource metrics")
	}

	// Collect nginx metrics
	if c.config.NginxMetricsURL != "" {
		if err := c.collectNginxMetrics(ctx, service, metrics); err != nil {
			logrus.WithError(err).Debug("Failed to collect nginx metrics")
		}
	}

	// Collect system metrics
	if c.config.BandwidthMonitoring.EnableNetworkBandwidth || c.config.BandwidthMonitoring.EnableIOBandwidth {
		if err := c.collectSystemMetrics(ctx, service, metrics); err != nil {
			logrus.WithError(err).Debug("Failed to collect system metrics")
		}
	}

	// Collect deployment information
	if err := c.collectDeploymentInfo(ctx, service, metrics); err != nil {
		logrus.WithError(err).Debug("Failed to collect deployment info")
	}

	return metrics, nil
}

// collectResourceMetrics collects CPU and memory utilization
func (c *Collector) collectResourceMetrics(ctx context.Context, service v1.Service, metrics *MetricsData) error {
	// Get pods for the service
	pods, err := c.getServicePods(ctx, service)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		return nil
	}

	var totalCPU, totalMemory, totalCPURequests, totalMemoryRequests float64

	// Aggregate metrics from all pods
	for _, pod := range pods {
		podMetrics, err := c.getPodMetrics(ctx, pod)
		if err != nil {
			logrus.WithError(err).WithField("pod", pod.Name).Debug("Failed to get pod metrics")
			continue
		}

		for _, container := range podMetrics.Containers {
			// CPU utilization (convert from nano cores to cores)
			cpuUsage := float64(container.Usage.Cpu().MilliValue()) / 1000.0
			totalCPU += cpuUsage

			// Memory utilization (convert to MB)
			memoryUsage := float64(container.Usage.Memory().Value()) / (1024 * 1024)
			totalMemory += memoryUsage
		}

		// Get resource requests for utilization percentage
		for _, container := range pod.Spec.Containers {
			if requests := container.Resources.Requests; requests != nil {
				if cpu := requests.Cpu(); cpu != nil {
					totalCPURequests += float64(cpu.MilliValue()) / 1000.0
				}
				if memory := requests.Memory(); memory != nil {
					totalMemoryRequests += float64(memory.Value()) / (1024 * 1024)
				}
			}
		}
	}

	// Calculate utilization percentages
	if totalCPURequests > 0 {
		metrics.CPUUtilization = (totalCPU / totalCPURequests) * 100
	}
	if totalMemoryRequests > 0 {
		metrics.MemoryUtilization = (totalMemory / totalMemoryRequests) * 100
	}

	return nil
}

// collectNginxMetrics collects metrics from nginx ingress controller
func (c *Collector) collectNginxMetrics(ctx context.Context, service v1.Service, metrics *MetricsData) error {
	// Build metrics URL
	url := fmt.Sprintf("%s/api/v1/nginx/stats", c.config.NginxMetricsURL)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("nginx metrics endpoint returned status %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var nginxMetrics NginxMetrics
	if err := json.Unmarshal(body, &nginxMetrics); err != nil {
		return err
	}

	// Map nginx metrics to our metrics structure
	metrics.RequestRate = nginxMetrics.RequestsPerSecond
	metrics.ResponseTime = nginxMetrics.ResponseTime
	metrics.ErrorRate = nginxMetrics.ErrorRate
	metrics.NetworkBandwidth = nginxMetrics.BytesPerSecond / (1024 * 1024) // Convert to MB/s

	return nil
}

// collectSystemMetrics collects system-level bandwidth metrics
func (c *Collector) collectSystemMetrics(ctx context.Context, service v1.Service, metrics *MetricsData) error {
	// This is a simplified implementation
	// In production, you'd integrate with actual system monitoring tools

	if c.config.BandwidthMonitoring.EnableNetworkBandwidth {
		// Simulate network bandwidth measurement
		metrics.NetworkBandwidth = c.estimateNetworkBandwidth(service)
	}

	if c.config.BandwidthMonitoring.EnableIOBandwidth {
		// Simulate I/O bandwidth measurement
		metrics.IOBandwidth = c.estimateIOBandwidth(service)
	}

	return nil
}

// collectDeploymentInfo collects deployment replica information
func (c *Collector) collectDeploymentInfo(ctx context.Context, service v1.Service, metrics *MetricsData) error {
	// Get deployment for the service
	deployments, err := c.getServiceDeployments(ctx, service)
	if err != nil {
		return err
	}

	if len(deployments) > 0 {
		deployment := deployments[0] // Use first deployment
		metrics.CurrentReplicas = deployment.Status.Replicas
		if deployment.Spec.Replicas != nil {
			metrics.DesiredReplicas = *deployment.Spec.Replicas
		}
	}

	return nil
}

// Helper methods (simplified implementations)

func (c *Collector) getServicePods(ctx context.Context, service v1.Service) ([]v1.Pod, error) {
	// Implementation would get pods using service selector
	return []v1.Pod{}, nil
}

func (c *Collector) getPodMetrics(ctx context.Context, pod v1.Pod) (*metricsv1beta1.PodMetrics, error) {
	// Implementation would get pod metrics from metrics API
	return &metricsv1beta1.PodMetrics{}, nil
}

func (c *Collector) getServiceDeployments(ctx context.Context, service v1.Service) ([]*appsv1.Deployment, error) {
	// Implementation would find deployments for the service
	return []*appsv1.Deployment{}, nil
}

func (c *Collector) estimateNetworkBandwidth(service v1.Service) float64 {
	// Simplified bandwidth estimation
	return 10.0 // MB/s
}

func (c *Collector) estimateIOBandwidth(service v1.Service) float64 {
	// Simplified I/O bandwidth estimation
	return 5.0 // MB/s
}

// storeMetrics stores metrics in the in-memory store
func (c *Collector) storeMetrics(metrics *MetricsData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s/%s", metrics.Namespace, metrics.ServiceName)
	c.metricsStore[key] = append(c.metricsStore[key], metrics)
}

// cleanOldMetrics removes metrics older than retention period
func (c *Collector) cleanOldMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-c.config.RetentionPeriod)

	for key, metrics := range c.metricsStore {
		var filtered []*MetricsData
		for _, metric := range metrics {
			if metric.Timestamp.After(cutoff) {
				filtered = append(filtered, metric)
			}
		}
		c.metricsStore[key] = filtered
	}
}
