package scaler

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gonum.org/v1/gonum/mat"

	"github.com/hydraai/hydra-route/internal/metrics"
	"github.com/hydraai/hydra-route/pkg/config"
)

// ScalingDecision represents a scaling decision made by the AI
type ScalingDecision struct {
	ServiceName         string               `json:"service_name"`
	Namespace           string               `json:"namespace"`
	Timestamp           time.Time            `json:"timestamp"`
	CurrentReplicas     int32                `json:"current_replicas"`
	RecommendedReplicas int32                `json:"recommended_replicas"`
	Confidence          float64              `json:"confidence"`
	Reasoning           string               `json:"reasoning"`
	Metrics             *metrics.MetricsData `json:"metrics"`
}

// FeatureVector represents input features for the AI model
type FeatureVector struct {
	CPUUtilization    float64
	MemoryUtilization float64
	RequestRate       float64
	NetworkBandwidth  float64
	IOBandwidth       float64
	ResponseTime      float64
	ErrorRate         float64
	TimeOfDay         float64 // 0-23
	DayOfWeek         float64 // 0-6
	TrendCPU          float64 // CPU trend over time
	TrendMemory       float64 // Memory trend over time
	TrendRequests     float64 // Request rate trend
}

// AIModel interface for different scaling models
type AIModel interface {
	Predict(features FeatureVector) (float64, float64, error) // returns scale factor and confidence
	Train(data []TrainingData) error
	GetModelType() string
}

// TrainingData represents historical data for training
type TrainingData struct {
	Features    FeatureVector
	ActualScale float64
	Performance float64 // performance metric (0-1)
	Timestamp   time.Time
}

// LinearModel implements a linear regression model
type LinearModel struct {
	Weights   []float64
	Bias      float64
	IsTrained bool
	Config    config.AIModelConfig
}

// NeuralNetwork implements a simple neural network
type NeuralNetwork struct {
	InputLayer   []float64
	HiddenLayer  []float64
	OutputLayer  []float64
	Weights1     *mat.Dense // Input to hidden
	Weights2     *mat.Dense // Hidden to output
	Bias1        []float64
	Bias2        []float64
	LearningRate float64
	IsTrained    bool
	Config       config.AIModelConfig
}

// EnsembleModel combines multiple models
type EnsembleModel struct {
	Models  []AIModel
	Weights []float64
	Config  config.AIModelConfig
}

// AIScaler manages AI-based scaling decisions
type AIScaler struct {
	config          config.ScalingConfig
	model           AIModel
	trainingData    []TrainingData
	mu              sync.RWMutex
	lastDecisions   map[string]*ScalingDecision
	cooldownTracker map[string]time.Time
}

// NewAIScaler creates a new AI-based scaler
func NewAIScaler(config config.ScalingConfig) *AIScaler {
	scaler := &AIScaler{
		config:          config,
		trainingData:    make([]TrainingData, 0),
		lastDecisions:   make(map[string]*ScalingDecision),
		cooldownTracker: make(map[string]time.Time),
	}

	// Initialize the AI model based on configuration
	scaler.model = scaler.createModel()

	return scaler
}

// createModel creates the appropriate AI model based on configuration
func (s *AIScaler) createModel() AIModel {
	switch s.config.AIModel.ModelType {
	case "neural_network":
		return &NeuralNetwork{
			LearningRate: s.config.AIModel.LearningRate,
			Config:       s.config.AIModel,
		}
	case "ensemble":
		return &EnsembleModel{
			Models: []AIModel{
				&LinearModel{Config: s.config.AIModel},
				&NeuralNetwork{LearningRate: s.config.AIModel.LearningRate, Config: s.config.AIModel},
			},
			Weights: []float64{0.6, 0.4}, // Linear model gets more weight initially
			Config:  s.config.AIModel,
		}
	default: // "linear" or default
		return &LinearModel{Config: s.config.AIModel}
	}
}

// MakeScalingDecision analyzes metrics and returns a scaling decision
func (s *AIScaler) MakeScalingDecision(metricsData *metrics.MetricsData) (*ScalingDecision, error) {
	if metricsData == nil {
		return nil, fmt.Errorf("metrics data is nil")
	}

	// Check if we're in cooldown period
	key := fmt.Sprintf("%s/%s", metricsData.Namespace, metricsData.ServiceName)
	if s.isInCooldown(key) {
		logrus.WithFields(logrus.Fields{
			"service":   metricsData.ServiceName,
			"namespace": metricsData.Namespace,
		}).Debug("Service is in cooldown period, skipping scaling decision")
		return nil, nil
	}

	// Convert metrics to feature vector
	features := s.extractFeatures(metricsData)

	// Get prediction from AI model
	scaleFactor, confidence, err := s.model.Predict(features)
	if err != nil {
		return nil, fmt.Errorf("model prediction failed: %w", err)
	}

	// Calculate recommended replicas
	currentReplicas := metricsData.CurrentReplicas
	if currentReplicas == 0 {
		currentReplicas = 1 // Default to 1 if not set
	}

	recommendedReplicas := s.calculateRecommendedReplicas(currentReplicas, scaleFactor)

	// Apply constraints
	recommendedReplicas = s.applyConstraints(recommendedReplicas)

	// Generate reasoning
	reasoning := s.generateReasoning(features, scaleFactor, confidence)

	decision := &ScalingDecision{
		ServiceName:         metricsData.ServiceName,
		Namespace:           metricsData.Namespace,
		Timestamp:           time.Now(),
		CurrentReplicas:     currentReplicas,
		RecommendedReplicas: recommendedReplicas,
		Confidence:          confidence,
		Reasoning:           reasoning,
		Metrics:             metricsData,
	}

	// Store decision and update cooldown
	s.storeDecision(key, decision)

	return decision, nil
}

// extractFeatures converts metrics data to feature vector
func (s *AIScaler) extractFeatures(metricsData *metrics.MetricsData) FeatureVector {
	now := time.Now()

	features := FeatureVector{
		CPUUtilization:    metricsData.CPUUtilization,
		MemoryUtilization: metricsData.MemoryUtilization,
		RequestRate:       metricsData.RequestRate,
		NetworkBandwidth:  metricsData.NetworkBandwidth,
		IOBandwidth:       metricsData.IOBandwidth,
		ResponseTime:      metricsData.ResponseTime,
		ErrorRate:         metricsData.ErrorRate,
		TimeOfDay:         float64(now.Hour()),
		DayOfWeek:         float64(now.Weekday()),
	}

	// Calculate trends (simplified implementation)
	features.TrendCPU = s.calculateTrend(metricsData.ServiceName, metricsData.Namespace, "cpu")
	features.TrendMemory = s.calculateTrend(metricsData.ServiceName, metricsData.Namespace, "memory")
	features.TrendRequests = s.calculateTrend(metricsData.ServiceName, metricsData.Namespace, "requests")

	return features
}

// calculateTrend calculates the trend for a specific metric (simplified)
func (s *AIScaler) calculateTrend(serviceName, namespace, metricType string) float64 {
	// This is a simplified implementation
	// In a real system, you'd analyze historical data to calculate actual trends
	return 0.0
}

// calculateRecommendedReplicas calculates the number of replicas based on scale factor
func (s *AIScaler) calculateRecommendedReplicas(currentReplicas int32, scaleFactor float64) int32 {
	if scaleFactor > 1.1 { // Scale up threshold
		return int32(math.Ceil(float64(currentReplicas) * scaleFactor))
	} else if scaleFactor < 0.9 { // Scale down threshold
		return int32(math.Floor(float64(currentReplicas) * scaleFactor))
	}
	return currentReplicas // No scaling needed
}

// applyConstraints applies min/max replica constraints
func (s *AIScaler) applyConstraints(replicas int32) int32 {
	if replicas < s.config.MinReplicas {
		return s.config.MinReplicas
	}
	if replicas > s.config.MaxReplicas {
		return s.config.MaxReplicas
	}
	return replicas
}

// generateReasoning creates a human-readable explanation for the scaling decision
func (s *AIScaler) generateReasoning(features FeatureVector, scaleFactor float64, confidence float64) string {
	var reasons []string

	if features.CPUUtilization > 80 {
		reasons = append(reasons, "high CPU utilization")
	}
	if features.MemoryUtilization > 80 {
		reasons = append(reasons, "high memory utilization")
	}
	if features.RequestRate > 100 {
		reasons = append(reasons, "high request rate")
	}
	if features.ErrorRate > 5 {
		reasons = append(reasons, "elevated error rate")
	}
	if features.ResponseTime > 1000 {
		reasons = append(reasons, "slow response times")
	}

	if len(reasons) == 0 {
		if scaleFactor > 1.1 {
			return fmt.Sprintf("AI model recommends scaling up (factor: %.2f, confidence: %.2f)", scaleFactor, confidence)
		} else if scaleFactor < 0.9 {
			return fmt.Sprintf("AI model recommends scaling down (factor: %.2f, confidence: %.2f)", scaleFactor, confidence)
		}
		return "No scaling needed based on current metrics"
	}

	action := "up"
	if scaleFactor < 1.0 {
		action = "down"
	}

	return fmt.Sprintf("Scaling %s due to: %v (factor: %.2f, confidence: %.2f)", action, reasons, scaleFactor, confidence)
}

// isInCooldown checks if a service is in cooldown period
func (s *AIScaler) isInCooldown(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lastTime, exists := s.cooldownTracker[key]
	if !exists {
		return false
	}

	// Check both scale up and scale down cooldowns
	now := time.Now()
	scaleUpCooldown := now.Sub(lastTime) < s.config.Cooldown.ScaleUpCooldown
	scaleDownCooldown := now.Sub(lastTime) < s.config.Cooldown.ScaleDownCooldown

	return scaleUpCooldown || scaleDownCooldown
}

// storeDecision stores a scaling decision and updates cooldown
func (s *AIScaler) storeDecision(key string, decision *ScalingDecision) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastDecisions[key] = decision

	// Update cooldown only if scaling is recommended
	if decision.CurrentReplicas != decision.RecommendedReplicas {
		s.cooldownTracker[key] = decision.Timestamp
	}
}

// AddTrainingData adds new training data for model improvement
func (s *AIScaler) AddTrainingData(data TrainingData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.trainingData = append(s.trainingData, data)

	// Limit training data size
	maxSize := 10000
	if len(s.trainingData) > maxSize {
		s.trainingData = s.trainingData[len(s.trainingData)-maxSize:]
	}

	// Retrain model periodically
	if s.config.AIModel.EnableOnlineLearning && len(s.trainingData)%100 == 0 {
		go s.retrainModel()
	}
}

// retrainModel retrains the AI model with collected data
func (s *AIScaler) retrainModel() {
	s.mu.RLock()
	trainingData := make([]TrainingData, len(s.trainingData))
	copy(trainingData, s.trainingData)
	s.mu.RUnlock()

	logrus.Info("Retraining AI model with %d data points", len(trainingData))

	if err := s.model.Train(trainingData); err != nil {
		logrus.WithError(err).Error("Failed to retrain AI model")
	} else {
		logrus.Info("AI model retrained successfully")
	}
}

// Linear Model Implementation

func (lm *LinearModel) Predict(features FeatureVector) (float64, float64, error) {
	if !lm.IsTrained {
		// Use default heuristic-based prediction
		return lm.heuristicPredict(features), 0.5, nil
	}

	// Convert features to slice
	featureSlice := lm.featuresToSlice(features)

	// Calculate weighted sum
	prediction := lm.Bias
	for i, feature := range featureSlice {
		if i < len(lm.Weights) {
			prediction += lm.Weights[i] * feature
		}
	}

	// Apply sigmoid to get scale factor between 0.5 and 2.0
	scaleFactor := 0.5 + 1.5*sigmoid(prediction)
	confidence := 0.8 // Static confidence for linear model

	return scaleFactor, confidence, nil
}

func (lm *LinearModel) Train(data []TrainingData) error {
	if len(data) < 10 {
		return fmt.Errorf("insufficient training data")
	}

	// Prepare training data
	numFeatures := 12 // Number of features in FeatureVector
	X := mat.NewDense(len(data), numFeatures, nil)
	y := mat.NewVecDense(len(data), nil)

	for i, sample := range data {
		features := lm.featuresToSlice(sample.Features)
		for j, feature := range features {
			if j < numFeatures {
				X.Set(i, j, feature)
			}
		}
		y.SetVec(i, sample.ActualScale)
	}

	// Simple linear regression using normal equation
	var xT mat.Dense
	xT.CloneFrom(X.T())

	var xTx mat.Dense
	xTx.Mul(&xT, X)

	var xTxInv mat.Dense
	if err := xTxInv.Inverse(&xTx); err != nil {
		return fmt.Errorf("failed to compute matrix inverse: %w", err)
	}

	var xTy mat.VecDense
	xTy.MulVec(&xT, y)

	var weights mat.VecDense
	weights.MulVec(&xTxInv, &xTy)

	// Extract weights
	lm.Weights = make([]float64, numFeatures)
	for i := 0; i < numFeatures; i++ {
		lm.Weights[i] = weights.AtVec(i)
	}

	lm.IsTrained = true
	return nil
}

func (lm *LinearModel) GetModelType() string {
	return "linear"
}

func (lm *LinearModel) featuresToSlice(features FeatureVector) []float64 {
	return []float64{
		features.CPUUtilization / 100.0,
		features.MemoryUtilization / 100.0,
		features.RequestRate / 1000.0,
		features.NetworkBandwidth / 100.0,
		features.IOBandwidth / 100.0,
		features.ResponseTime / 1000.0,
		features.ErrorRate / 100.0,
		features.TimeOfDay / 24.0,
		features.DayOfWeek / 7.0,
		features.TrendCPU,
		features.TrendMemory,
		features.TrendRequests,
	}
}

func (lm *LinearModel) heuristicPredict(features FeatureVector) float64 {
	// Simple heuristic-based scaling
	scaleFactor := 1.0

	// CPU-based scaling
	if features.CPUUtilization > 80 {
		scaleFactor *= 1.5
	} else if features.CPUUtilization < 30 {
		scaleFactor *= 0.7
	}

	// Memory-based scaling
	if features.MemoryUtilization > 80 {
		scaleFactor *= 1.3
	} else if features.MemoryUtilization < 30 {
		scaleFactor *= 0.8
	}

	// Request rate-based scaling
	if features.RequestRate > 100 {
		scaleFactor *= 1.2
	} else if features.RequestRate < 10 {
		scaleFactor *= 0.9
	}

	return scaleFactor
}

// Utility functions

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

// Neural Network Implementation (simplified)

func (nn *NeuralNetwork) Predict(features FeatureVector) (float64, float64, error) {
	if !nn.IsTrained {
		// Use linear model heuristic as fallback
		lm := &LinearModel{}
		return lm.heuristicPredict(features), 0.3, nil
	}

	// Forward pass (simplified)
	input := nn.featuresToSlice(features)

	// Hidden layer activation
	hiddenOutput := make([]float64, len(nn.HiddenLayer))
	for i := range hiddenOutput {
		sum := nn.Bias1[i]
		for j, inp := range input {
			if i < nn.Weights1.RawMatrix().Rows && j < nn.Weights1.RawMatrix().Cols {
				sum += nn.Weights1.At(i, j) * inp
			}
		}
		hiddenOutput[i] = sigmoid(sum)
	}

	// Output layer
	output := nn.Bias2[0]
	for i, hidden := range hiddenOutput {
		if i < nn.Weights2.RawMatrix().Rows {
			output += nn.Weights2.At(i, 0) * hidden
		}
	}

	scaleFactor := 0.5 + 1.5*sigmoid(output)
	confidence := 0.9 // Higher confidence for neural network

	return scaleFactor, confidence, nil
}

func (nn *NeuralNetwork) Train(data []TrainingData) error {
	// Simplified training implementation
	// In production, you'd implement proper backpropagation
	nn.IsTrained = true
	return nil
}

func (nn *NeuralNetwork) GetModelType() string {
	return "neural_network"
}

func (nn *NeuralNetwork) featuresToSlice(features FeatureVector) []float64 {
	lm := &LinearModel{}
	return lm.featuresToSlice(features)
}

// Ensemble Model Implementation

func (em *EnsembleModel) Predict(features FeatureVector) (float64, float64, error) {
	var weightedSum, totalWeight, weightedConfidence float64

	for i, model := range em.Models {
		prediction, confidence, err := model.Predict(features)
		if err != nil {
			continue // Skip models that fail
		}

		weight := em.Weights[i]
		weightedSum += prediction * weight
		weightedConfidence += confidence * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 1.0, 0.0, fmt.Errorf("all models failed to predict")
	}

	finalPrediction := weightedSum / totalWeight
	finalConfidence := weightedConfidence / totalWeight

	return finalPrediction, finalConfidence, nil
}

func (em *EnsembleModel) Train(data []TrainingData) error {
	var errors []error

	for _, model := range em.Models {
		if err := model.Train(data); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) == len(em.Models) {
		return fmt.Errorf("all models failed to train")
	}

	return nil
}

func (em *EnsembleModel) GetModelType() string {
	return "ensemble"
}
