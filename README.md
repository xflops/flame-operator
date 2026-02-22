# Flame Operator

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

The Flame Operator is a Kubernetes Operator that automates the deployment and lifecycle management of [Flame](https://github.com/xflops/flame) clusters. It provides a declarative API through the `FlameCluster` Custom Resource Definition (CRD), enabling you to deploy and manage distributed Flame clusters with a single YAML manifest.

## Features

- **Declarative Configuration**: Define your entire Flame cluster with a single `FlameCluster` resource
- **Automated Lifecycle Management**: The operator handles creation, scaling, and deletion of all cluster components
- **Service Discovery**: Automatic DNS-based service discovery for inter-component communication
- **Configuration Management**: Auto-generated ConfigMaps from CRD spec with endpoint injection
- **Automatic Cleanup**: All resources are garbage collected when the FlameCluster is deleted
- **Scaling**: Easily scale Executor Manager replicas by updating the CRD

## Architecture

A FlameCluster consists of:

- **Session Manager**: The central coordinator (single instance)
- **Executor Managers**: Workers that execute tasks (configurable replicas)
- **Object Cache**: Distributed caching service for the cluster

```
┌─────────────────────────────────────────────────────┐
│                   FlameCluster                       │
│  ┌─────────────────┐    ┌─────────────────────────┐ │
│  │ Session Manager │    │   Executor Managers     │ │
│  │   (1 replica)   │◄───│   (N replicas)          │ │
│  └────────┬────────┘    └───────────┬─────────────┘ │
│           │                         │               │
│           ▼                         ▼               │
│  ┌─────────────────┐    ┌─────────────────────────┐ │
│  │ Session Manager │    │   Object Cache Service  │ │
│  │    Service      │    │                         │ │
│  └─────────────────┘    └─────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.19+)
- kubectl configured to access your cluster
- Cluster admin permissions (for CRD installation)

### Installation

1. Install the FlameCluster CRD:

```bash
kubectl apply -f https://raw.githubusercontent.com/xflops/flame-operator/main/config/crd/bases/flame.xflops.io_flameclusters.yaml
```

2. Deploy the Flame Operator:

```bash
kubectl apply -f https://raw.githubusercontent.com/xflops/flame-operator/main/config/manager/manager.yaml
```

### Deploy Your First Cluster

Create a minimal FlameCluster:

```yaml
apiVersion: flame.xflops.io/v1alpha1
kind: FlameCluster
metadata:
  name: my-flame
spec:
  sessionManager:
    image: "xflops/flame-session:v0.1.0"
  executorManager:
    image: "xflops/flame-executor:v0.1.0"
    replicas: 3
```

Apply it:

```bash
kubectl apply -f my-flame-cluster.yaml
```

Check the status:

```bash
kubectl get flamecluster my-flame
```

## Documentation

- [Getting Started Guide](docs/getting-started.md) - Step-by-step installation and first cluster deployment
- [Configuration Reference](docs/configuration.md) - Complete CRD specification and all available options
- [Examples](docs/examples/) - Sample FlameCluster configurations

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/xflops/flame-operator.git
cd flame-operator

# Build the operator
make build

# Run tests
make test
```

### Project Structure

```
flame-operator/
├── api/                    # CRD type definitions
├── cmd/                    # Operator entrypoint
├── config/                 # Kubernetes manifests
├── docs/                   # Documentation
│   ├── design/            # Internal design documents
│   ├── examples/          # Example configurations
│   └── test/              # Test documentation
└── internal/              # Controller implementation
```

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting pull requests.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
