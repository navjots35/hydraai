package config

import (
	"fmt"
	"io/ioutil"
	"time"

	"gopkg.in/yaml.v2"
)

// Config represents the main configuration for HydraRoute
type Config struct {
	Metrics MetricsConfig `yaml:"metrics"`
	Scaling ScalingConfig `yaml:"scaling"`
	General GeneralConfig `yaml:"general"`
}

// MetricsConfig defines metrics collection settings
type MetricsConfig struct {
	// Collection interval for metrics
	CollectionInterval time.Duration `yaml:"collection_interval"`

	// Nginx Ingress Controller metrics endpoint
	NginxMetricsURL string `yaml:"nginx_metrics_url"`

	// Prometheus endpoint for additional metrics
	PrometheusURL string `yaml:"prometheus_url"`

	// Enable custom metrics collection
	EnableCustomMetrics bool `yaml:"enable_custom_metrics"`

	// Metrics retention period
	RetentionPeriod time.Duration `yaml:"retention_period"`

	// Request rate window for analysis
	RequestRateWindow time.Duration `yaml:"request_rate_window"`

	// Bandwidth monitoring settings
	BandwidthMonitoring BandwidthConfig `yaml:"bandwidth_monitoring"`
}

// BandwidthConfig defines bandwidth monitoring settings
type BandwidthConfig struct {
	// Enable network bandwidth monitoring
	EnableNetworkBandwidth bool `yaml:"enable_network_bandwidth"`

	// Enable I/O bandwidth monitoring
	EnableIOBandwidth bool `yaml:"enable_io_bandwidth"`

	// Bandwidth measurement interval
	MeasurementInterval time.Duration `yaml:"measurement_interval"`

	// Network interface to monitor (empty for auto-detect)
	NetworkInterface string `yaml:"network_interface"`
}

// ScalingConfig defines AI-based scaling parameters
type ScalingConfig struct {
	// Enable AI-based scaling
	EnableAIScaling bool `yaml:"enable_ai_scaling"`

	// Minimum number of replicas
	MinReplicas int32 `yaml:"min_replicas"`

	// Maximum number of replicas
	MaxReplicas int32 `yaml:"max_replicas"`

	// Scaling evaluation interval
	EvaluationInterval time.Duration `yaml:"evaluation_interval"`

	// Scale up threshold settings
	ScaleUpThresholds ThresholdConfig `yaml:"scale_up_thresholds"`

	// Scale down threshold settings
	ScaleDownThresholds ThresholdConfig `yaml:"scale_down_thresholds"`

	// AI model configuration
	AIModel AIModelConfig `yaml:"ai_model"`

	// Cooldown periods to prevent flapping
	Cooldown CooldownConfig `yaml:"cooldown"`

	// Prediction settings
	Prediction PredictionConfig `yaml:"prediction"`
}

// ThresholdConfig defines threshold values for scaling decisions
type ThresholdConfig struct {
	// CPU utilization threshold (percentage)
	CPUUtilization float64 `yaml:"cpu_utilization"`

	// Memory utilization threshold (percentage)
	MemoryUtilization float64 `yaml:"memory_utilization"`

	// Request rate threshold (requests per second)
	RequestRate float64 `yaml:"request_rate"`

	// Network bandwidth threshold (MB/s)
	NetworkBandwidth float64 `yaml:"network_bandwidth"`

	// I/O bandwidth threshold (MB/s)
	IOBandwidth float64 `yaml:"io_bandwidth"`

	// Response time threshold (milliseconds)
	ResponseTime float64 `yaml:"response_time"`

	// Error rate threshold (percentage)
	ErrorRate float64 `yaml:"error_rate"`
}

// AIModelConfig defines AI model parameters
type AIModelConfig struct {
	// Model type (linear, neural_network, ensemble)
	ModelType string `yaml:"model_type"`

	// Learning rate for adaptive models
	LearningRate float64 `yaml:"learning_rate"`

	// Historical data window for training
	HistoricalWindow time.Duration `yaml:"historical_window"`

	// Feature weights for different metrics
	FeatureWeights FeatureWeights `yaml:"feature_weights"`

	// Enable online learning
	EnableOnlineLearning bool `yaml:"enable_online_learning"`

	// Model retrain interval
	RetrainInterval time.Duration `yaml:"retrain_interval"`
}

// FeatureWeights defines importance weights for different metrics
type FeatureWeights struct {
	CPUUtilization    float64 `yaml:"cpu_utilization"`
	MemoryUtilization float64 `yaml:"memory_utilization"`
	RequestRate       float64 `yaml:"request_rate"`
	NetworkBandwidth  float64 `yaml:"network_bandwidth"`
	IOBandwidth       float64 `yaml:"io_bandwidth"`
	ResponseTime      float64 `yaml:"response_time"`
	ErrorRate         float64 `yaml:"error_rate"`
}

// CooldownConfig defines cooldown periods
type CooldownConfig struct {
	// Scale up cooldown period
	ScaleUpCooldown time.Duration `yaml:"scale_up_cooldown"`

	// Scale down cooldown period
	ScaleDownCooldown time.Duration `yaml:"scale_down_cooldown"`
}

// PredictionConfig defines prediction settings
type PredictionConfig struct {
	// Enable predictive scaling
	EnablePredictiveScaling bool `yaml:"enable_predictive_scaling"`

	// Prediction horizon
	PredictionHorizon time.Duration `yaml:"prediction_horizon"`

	// Prediction confidence threshold
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`

	// Seasonality detection
	EnableSeasonalityDetection bool `yaml:"enable_seasonality_detection"`
}

// GeneralConfig defines general settings
type GeneralConfig struct {
	// Log level
	LogLevel string `yaml:"log_level"`

	// Ingress class to watch
	IngressClass string `yaml:"ingress_class"`

	// Namespaces to watch (empty for all)
	WatchNamespaces []string `yaml:"watch_namespaces"`

	// Enable dry run mode
	DryRun bool `yaml:"dry_run"`

	// Leader election settings
	LeaderElection LeaderElectionConfig `yaml:"leader_election"`

	// Health check settings
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// LeaderElectionConfig defines leader election settings
type LeaderElectionConfig struct {
	// Enable leader election
	Enabled bool `yaml:"enabled"`

	// Lease duration
	LeaseDuration time.Duration `yaml:"lease_duration"`

	// Renew deadline
	RenewDeadline time.Duration `yaml:"renew_deadline"`

	// Retry period
	RetryPeriod time.Duration `yaml:"retry_period"`
}

// HealthCheckConfig defines health check settings
type HealthCheckConfig struct {
	// Health check interval
	Interval time.Duration `yaml:"interval"`

	// Health check timeout
	Timeout time.Duration `yaml:"timeout"`

	// Failure threshold
	FailureThreshold int `yaml:"failure_threshold"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set defaults
	setDefaults(config)

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	if config.Metrics.CollectionInterval == 0 {
		config.Metrics.CollectionInterval = 30 * time.Second
	}
	if config.Metrics.RetentionPeriod == 0 {
		config.Metrics.RetentionPeriod = 24 * time.Hour
	}
	if config.Metrics.RequestRateWindow == 0 {
		config.Metrics.RequestRateWindow = 5 * time.Minute
	}
	if config.Metrics.BandwidthMonitoring.MeasurementInterval == 0 {
		config.Metrics.BandwidthMonitoring.MeasurementInterval = 10 * time.Second
	}

	if config.Scaling.MinReplicas == 0 {
		config.Scaling.MinReplicas = 1
	}
	if config.Scaling.MaxReplicas == 0 {
		config.Scaling.MaxReplicas = 10
	}
	if config.Scaling.EvaluationInterval == 0 {
		config.Scaling.EvaluationInterval = 30 * time.Second
	}
	if config.Scaling.Cooldown.ScaleUpCooldown == 0 {
		config.Scaling.Cooldown.ScaleUpCooldown = 3 * time.Minute
	}
	if config.Scaling.Cooldown.ScaleDownCooldown == 0 {
		config.Scaling.Cooldown.ScaleDownCooldown = 5 * time.Minute
	}
	if config.Scaling.AIModel.LearningRate == 0 {
		config.Scaling.AIModel.LearningRate = 0.01
	}
	if config.Scaling.AIModel.HistoricalWindow == 0 {
		config.Scaling.AIModel.HistoricalWindow = 24 * time.Hour
	}
	if config.Scaling.Prediction.PredictionHorizon == 0 {
		config.Scaling.Prediction.PredictionHorizon = 10 * time.Minute
	}
	if config.Scaling.Prediction.ConfidenceThreshold == 0 {
		config.Scaling.Prediction.ConfidenceThreshold = 0.8
	}

	if config.General.LogLevel == "" {
		config.General.LogLevel = "info"
	}
	if config.General.IngressClass == "" {
		config.General.IngressClass = "nginx"
	}
	if config.General.LeaderElection.LeaseDuration == 0 {
		config.General.LeaderElection.LeaseDuration = 15 * time.Second
	}
	if config.General.LeaderElection.RenewDeadline == 0 {
		config.General.LeaderElection.RenewDeadline = 10 * time.Second
	}
	if config.General.LeaderElection.RetryPeriod == 0 {
		config.General.LeaderElection.RetryPeriod = 2 * time.Second
	}
	if config.General.HealthCheck.Interval == 0 {
		config.General.HealthCheck.Interval = 30 * time.Second
	}
	if config.General.HealthCheck.Timeout == 0 {
		config.General.HealthCheck.Timeout = 5 * time.Second
	}
	if config.General.HealthCheck.FailureThreshold == 0 {
		config.General.HealthCheck.FailureThreshold = 3
	}

	// Set default feature weights
	if config.Scaling.AIModel.FeatureWeights.CPUUtilization == 0 {
		config.Scaling.AIModel.FeatureWeights.CPUUtilization = 0.25
	}
	if config.Scaling.AIModel.FeatureWeights.MemoryUtilization == 0 {
		config.Scaling.AIModel.FeatureWeights.MemoryUtilization = 0.20
	}
	if config.Scaling.AIModel.FeatureWeights.RequestRate == 0 {
		config.Scaling.AIModel.FeatureWeights.RequestRate = 0.30
	}
	if config.Scaling.AIModel.FeatureWeights.NetworkBandwidth == 0 {
		config.Scaling.AIModel.FeatureWeights.NetworkBandwidth = 0.10
	}
	if config.Scaling.AIModel.FeatureWeights.IOBandwidth == 0 {
		config.Scaling.AIModel.FeatureWeights.IOBandwidth = 0.05
	}
	if config.Scaling.AIModel.FeatureWeights.ResponseTime == 0 {
		config.Scaling.AIModel.FeatureWeights.ResponseTime = 0.08
	}
	if config.Scaling.AIModel.FeatureWeights.ErrorRate == 0 {
		config.Scaling.AIModel.FeatureWeights.ErrorRate = 0.02
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Scaling.MinReplicas < 1 {
		return fmt.Errorf("min_replicas must be at least 1")
	}
	if config.Scaling.MaxReplicas < config.Scaling.MinReplicas {
		return fmt.Errorf("max_replicas must be greater than or equal to min_replicas")
	}
	if config.Scaling.AIModel.LearningRate <= 0 || config.Scaling.AIModel.LearningRate >= 1 {
		return fmt.Errorf("learning_rate must be between 0 and 1")
	}
	if config.Scaling.Prediction.ConfidenceThreshold <= 0 || config.Scaling.Prediction.ConfidenceThreshold >= 1 {
		return fmt.Errorf("confidence_threshold must be between 0 and 1")
	}

	return nil
}
