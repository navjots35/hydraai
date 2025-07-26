# HydraRoute - Intelligent AI-Based Kubernetes Pod Scaling

> **⚠️ BETA VERSION - UNDER ACTIVE DEVELOPMENT** ⚠️
> 
> This project is currently in **beta testing** and under active development. Not all features are fully implemented or tested in production environments. We welcome feedback, bug reports, and contributions from the community.
> 
> **Please note:**
> - Some AI model implementations are simplified and may not work optimally in all scenarios
> - Metrics collection from nginx ingress controllers requires proper endpoint configuration
> - The system bandwidth monitoring features are currently simulated
> - Production deployment should be done with caution and thorough testing

HydraRoute is a Kubernetes plugin that provides intelligent, AI-based automatic scaling for pods behind nginx ingress controllers. It complements existing ingress setups by analyzing multiple metrics including CPU/memory utilization, request rates, I/O bandwidth, and network bandwidth to make informed scaling decisions.

## 🚀 Features

- **AI-Powered Scaling**: Uses machine learning algorithms (linear regression, neural networks, ensemble methods) for intelligent scaling decisions *(Beta - Simplified implementations)*
- **Multi-Metric Analysis**: Monitors CPU, memory, request rate, network bandwidth, I/O bandwidth, response times, and error rates *(Beta - Some metrics simulated)*
- **Nginx Ingress Integration**: Seamlessly works with existing nginx ingress controllers *(Beta - Requires proper metrics endpoint configuration)*
- **Predictive Scaling**: Anticipates traffic patterns and scales proactively *(Beta - Basic implementation)*
- **Cooldown Management**: Prevents scaling flapping with configurable cooldown periods
- **Dry Run Mode**: Test scaling decisions without actually modifying deployments
- **Online Learning**: Continuously improves AI models based on historical performance *(Beta - Limited training data handling)*
- **Cloud Native**: Built specifically for Kubernetes with proper RBAC and security

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Nginx Ingress │    │   HydraRoute     │    │   Deployments   │
│   Controller    │◄──►│   Controller     │◄──►│   & Services    │
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Metrics API   │    │   AI Scaler      │    │   Pod Metrics   │
│   (Prometheus)  │    │   Engine         │    │   API           │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

### Core Components

1. **Metrics Collector**: Gathers data from nginx ingress, Kubernetes metrics API, and system monitoring
2. **AI Scaler**: Analyzes metrics using machine learning to make scaling decisions
3. **Controller**: Kubernetes controller that watches ingress resources and applies scaling decisions
4. **Configuration Manager**: Handles dynamic configuration and feature weights

## 📦 Installation

### Prerequisites

- Kubernetes cluster (v1.20+)
- Nginx Ingress Controller *(with metrics endpoint enabled)*
- Metrics Server (for pod metrics)
- RBAC enabled

> **⚠️ Beta Note:** The nginx ingress controller must have metrics endpoint enabled and accessible. The default configuration assumes the metrics endpoint is available at `http://nginx-ingress-controller.ingress-nginx.svc.cluster.local:10254`.

### Quick Install

```bash
# Apply RBAC and deployment manifests
kubectl apply -f https://raw.githubusercontent.com/hydraai/hydra-route/main/deploy/kubernetes/rbac.yaml
kubectl apply -f https://raw.githubusercontent.com/hydraai/hydra-route/main/deploy/kubernetes/deployment.yaml

# Verify installation
kubectl get pods -n hydra-route-system
```

### Manual Install

1. **Clone the repository**:
   ```bash
   git clone https://github.com/hydraai/hydra-route.git
   cd hydra-route
   ```

2. **Build the binary**:
   ```bash
   go build -o hydra-route ./cmd/hydra-route
   ```

3. **Build and push Docker image**:
   ```bash
   docker build -t your-registry/hydra-route:latest .
   docker push your-registry/hydra-route:latest
   ```

4. **Update deployment image and apply**:
   ```bash
   # Edit deploy/kubernetes/deployment.yaml to use your image
   kubectl apply -f deploy/kubernetes/
   ```

> **⚠️ Beta Testing Recommendation:** Start with dry-run mode enabled to test the system without making actual scaling changes. Monitor the logs and metrics to ensure proper operation before enabling live scaling.

## ⚙️ Configuration

HydraRoute is configured via a YAML configuration file. Here's the complete configuration structure:

```yaml
# Metrics collection settings
metrics:
  collection_interval: 30s
  nginx_metrics_url: "http://nginx-ingress-controller.ingress-nginx.svc.cluster.local:10254"
  prometheus_url: "http://prometheus.monitoring.svc.cluster.local:9090"
  enable_custom_metrics: true
  retention_period: 24h
  request_rate_window: 5m
  bandwidth_monitoring:
    enable_network_bandwidth: true
    enable_io_bandwidth: true
    measurement_interval: 10s
    network_interface: ""  # Auto-detect

# AI-based scaling configuration
scaling:
  enable_ai_scaling: true
  min_replicas: 1
  max_replicas: 20
  evaluation_interval: 30s
  
  # Thresholds for scaling up
  scale_up_thresholds:
    cpu_utilization: 70.0      # Percentage
    memory_utilization: 75.0   # Percentage
    request_rate: 100.0        # Requests per second
    network_bandwidth: 80.0    # MB/s
    io_bandwidth: 50.0         # MB/s
    response_time: 1000.0      # Milliseconds
    error_rate: 5.0           # Percentage
  
  # Thresholds for scaling down
  scale_down_thresholds:
    cpu_utilization: 30.0
    memory_utilization: 40.0
    request_rate: 20.0
    network_bandwidth: 20.0
    io_bandwidth: 10.0
    response_time: 200.0
    error_rate: 1.0
  
  # AI model configuration
  ai_model:
    model_type: "ensemble"     # linear, neural_network, ensemble
    learning_rate: 0.01
    historical_window: 24h
    enable_online_learning: true
    retrain_interval: 2h
    
    # Feature importance weights
    feature_weights:
      cpu_utilization: 0.25
      memory_utilization: 0.20
      request_rate: 0.30
      network_bandwidth: 0.10
      io_bandwidth: 0.05
      response_time: 0.08
      error_rate: 0.02
  
  # Cooldown periods to prevent flapping
  cooldown:
    scale_up_cooldown: 3m
    scale_down_cooldown: 5m
  
  # Predictive scaling settings
  prediction:
    enable_predictive_scaling: true
    prediction_horizon: 10m
    confidence_threshold: 0.8
    enable_seasonality_detection: true

# General settings
general:
  log_level: "info"
  ingress_class: "nginx"
  watch_namespaces: []  # Empty for all namespaces
  dry_run: false
  
  leader_election:
    enabled: true
    lease_duration: 15s
    renew_deadline: 10s
    retry_period: 2s
  
  health_check:
    interval: 30s
    timeout: 5s
    failure_threshold: 3
```

## 🎯 Usage

### Enable HydraRoute for an Ingress

Add the `hydra-route.ai/enabled` annotation to your ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app-ingress
  annotations:
    kubernetes.io/ingress.class: nginx
    hydra-route.ai/enabled: "true"
    hydra-route.ai/min-replicas: "2"
    hydra-route.ai/max-replicas: "50"
spec:
  rules:
  - host: myapp.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: my-app-service
            port:
              number: 80
```

### Available Annotations

- `hydra-route.ai/enabled`: Enable HydraRoute for this ingress (`"true"` or `"false"`)
- `hydra-route.ai/min-replicas`: Minimum number of replicas (overrides global config)
- `hydra-route.ai/max-replicas`: Maximum number of replicas (overrides global config)
- `hydra-route.ai/target`: Target service name (if different from backend service)

### Monitor Scaling Decisions

```bash
# Check controller logs
kubectl logs -n hydra-route-system deployment/hydra-route-controller

# Check scaling events
kubectl get events --field-selector reason=ScalingDecision

# View deployment annotations added by HydraRoute
kubectl get deployment my-app -o yaml | grep "hydra-route.ai"
```

## 🤖 AI Models

HydraRoute supports three types of AI models:

### 1. Linear Model
- Simple linear regression
- Fast and interpretable
- Good for predictable workloads
- Minimal resource usage

### 2. Neural Network
- Multi-layer perceptron
- Handles complex patterns
- Better for dynamic workloads
- Requires more training data

### 3. Ensemble Model (Recommended)
- Combines linear and neural network models
- Best of both worlds
- Weighted predictions
- Most robust performance

### Feature Engineering

The AI models analyze the following features:

- **Resource Metrics**: CPU and memory utilization
- **Traffic Metrics**: Request rate, response time, error rate
- **Bandwidth Metrics**: Network and I/O bandwidth
- **Temporal Features**: Time of day, day of week
- **Trend Analysis**: CPU, memory, and request trends

## 📊 Monitoring and Observability

### Metrics Endpoint

HydraRoute exposes Prometheus metrics at `:8080/metrics`:

```
# Scaling decisions made
hydra_route_scaling_decisions_total{service, namespace, action}

# AI model confidence
hydra_route_model_confidence{service, namespace, model_type}

# Feature importance
hydra_route_feature_weight{feature_name}

# Metrics collection status
hydra_route_metrics_collection_duration_seconds
hydra_route_metrics_collection_errors_total
```

### Health Checks

- **Liveness**: `/healthz` on port 8081
- **Readiness**: `/readyz` on port 8081

### Logging

Structured JSON logging with configurable levels:
- `debug`: Detailed information for troubleshooting
- `info`: General operational information
- `warn`: Warning conditions
- `error`: Error conditions

## 🔧 Development

### Prerequisites

- Go 1.21+
- Docker
- Kubernetes cluster for testing

### Local Development

```bash
# Clone repository
git clone https://github.com/hydraai/hydra-route.git
cd hydra-route

# Install dependencies
go mod download

# Run tests
go test ./...

# Build binary
go build -o hydra-route ./cmd/hydra-route

# Run locally (requires kubeconfig)
./hydra-route --config=config/default-config.yaml --dry-run=true
```

### Testing

```bash
# Unit tests
go test ./internal/...

# Integration tests
go test ./test/integration/...

# Load tests
go test ./test/load/...
```

## 🚦 Troubleshooting

### Common Issues

1. **No metrics available**
   - Verify nginx ingress controller metrics endpoint
   - Check metrics server installation
   - Confirm RBAC permissions
   - **Beta Issue:** Some metrics endpoints may not be properly configured

2. **Scaling decisions not applied**
   - Check dry-run mode setting
   - Verify deployment RBAC permissions
   - Review cooldown periods
   - **Beta Issue:** AI models may not make optimal decisions initially

3. **AI model not learning**
   - Ensure sufficient training data (>100 samples)
   - Check online learning configuration
   - Review feature weights
   - **Beta Issue:** Training data collection and model retraining are simplified

### Beta-Specific Issues

4. **Simulated metrics showing unrealistic values**
   - This is expected in beta - system bandwidth metrics are currently simulated
   - Real implementation will require actual system monitoring integration

5. **AI predictions seem inaccurate**
   - Beta models use simplified algorithms
   - Feature engineering is basic and may need tuning for your specific workload
   - Consider adjusting feature weights in configuration

### Debug Commands

```bash
# Check controller status
kubectl get pods -n hydra-route-system

# View detailed logs
kubectl logs -n hydra-route-system deployment/hydra-route-controller -f

# Check configuration
kubectl get configmap -n hydra-route-system hydra-route-config -o yaml

# Verify RBAC
kubectl auth can-i get deployments --as=system:serviceaccount:hydra-route-system:hydra-route-controller
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Beta Testing Feedback

As this project is in beta, we especially value:

- **Bug Reports**: Please report any issues you encounter
- **Feature Requests**: Let us know what features would be most valuable
- **Performance Feedback**: Share your experience with scaling decisions
- **Configuration Suggestions**: Help improve default settings
- **Documentation Improvements**: Suggest better explanations or examples

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Submit a pull request

### Beta Testing Checklist

Before reporting issues, please check:
- [ ] Running with dry-run mode first
- [ ] Nginx ingress metrics endpoint is accessible
- [ ] Kubernetes metrics server is installed
- [ ] RBAC permissions are properly configured
- [ ] Configuration file is valid YAML

## 📄 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🆘 Support

- **Documentation**: [Wiki](https://github.com/hydraai/hydra-route/wiki)
- **Issues**: [GitHub Issues](https://github.com/hydraai/hydra-route/issues) *(Please use "beta" label for beta-related issues)*
- **Discussions**: [GitHub Discussions](https://github.com/hydraai/hydra-route/discussions)
- **Slack**: [#hydra-route](https://kubernetes.slack.com/channels/hydra-route)

> **💡 Beta Support:** For beta testing issues, please include:
> - Your Kubernetes version
> - Nginx ingress controller version
> - Configuration file (with sensitive data redacted)
> - Logs from the hydra-route controller
> - Description of expected vs actual behavior

## 🗺️ Roadmap

### Beta Phase (Current)
- [x] Basic AI scaling engine
- [x] Nginx ingress integration
- [x] Metrics collection framework
- [x] Kubernetes controller
- [ ] Real system bandwidth monitoring *(Currently simulated)*
- [ ] Advanced AI model training *(Currently simplified)*
- [ ] Production-ready error handling

### Future Releases
- [ ] Support for Istio ingress gateway
- [ ] Custom Resource Definitions (CRDs) for scaling policies
- [ ] Multi-cluster scaling coordination
- [ ] Advanced anomaly detection
- [ ] Web dashboard for monitoring and configuration
- [ ] Integration with GitOps workflows
- [ ] Support for multiple ingress controllers
- [ ] Advanced predictive scaling algorithms

---

**HydraRoute** - Intelligent scaling for the cloud-native era. 🌊

---

> **🎯 Beta Testing Call to Action**
> 
> We're actively seeking beta testers to help improve HydraRoute! If you're interested in testing intelligent Kubernetes scaling, please:
> 
> 1. **Try it out** in a non-production environment
> 2. **Share your feedback** via GitHub Issues or Discussions
> 3. **Report bugs** with detailed information
> 4. **Suggest improvements** for features and configuration
> 
> Your feedback will help shape the future of intelligent Kubernetes scaling! 🚀 