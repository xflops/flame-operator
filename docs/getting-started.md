# Getting Started with Flame Operator

This guide walks you through installing the Flame Operator and deploying your first FlameCluster on Kubernetes.

## Prerequisites

Before you begin, ensure you have the following:

- **Kubernetes cluster** (v1.19 or later)
  - Local options: [minikube](https://minikube.sigs.k8s.io/), [kind](https://kind.sigs.k8s.io/), [Docker Desktop](https://www.docker.com/products/docker-desktop)
  - Cloud options: GKE, EKS, AKS, or any managed Kubernetes service
- **kubectl** installed and configured to access your cluster
- **Cluster admin permissions** (required for CRD installation)

### Verify Your Setup

```bash
# Check kubectl is configured
kubectl cluster-info

# Verify you have admin permissions
kubectl auth can-i create crd --all-namespaces
```

## Installation

### Step 1: Install the Custom Resource Definition (CRD)

The FlameCluster CRD defines the schema for Flame cluster resources:

```bash
kubectl apply -f https://raw.githubusercontent.com/xflops/flame-operator/main/config/crd/bases/flame.xflops.io_flameclusters.yaml
```

Verify the CRD is installed:

```bash
kubectl get crd flameclusters.flame.xflops.io
```

Expected output:
```
NAME                            CREATED AT
flameclusters.flame.xflops.io   2024-01-01T00:00:00Z
```

### Step 2: Deploy the Flame Operator

Deploy the operator to manage FlameCluster resources:

```bash
kubectl apply -f https://raw.githubusercontent.com/xflops/flame-operator/main/config/manager/manager.yaml
```

Verify the operator is running:

```bash
kubectl get pods -n flame-system
```

Expected output:
```
NAME                                READY   STATUS    RESTARTS   AGE
flame-operator-controller-manager   1/1     Running   0          30s
```

## Deploy Your First FlameCluster

### Step 1: Create a FlameCluster Manifest

Create a file named `my-flame-cluster.yaml`:

```yaml
apiVersion: flame.xflops.io/v1alpha1
kind: FlameCluster
metadata:
  name: my-flame
  namespace: default
spec:
  sessionManager:
    image: "xflops/flame-session:v0.1.0"
    slot: "cpu=1,mem=1g"
    policy: priority
    storage: sqlite://flame.db
  
  executorManager:
    image: "xflops/flame-executor:v0.1.0"
    replicas: 3
    shim: host
    maxExecutors: 10
  
  objectCache:
    networkInterface: "eth0"
    storage: "/var/lib/flame/cache"
```

### Step 2: Apply the Manifest

```bash
kubectl apply -f my-flame-cluster.yaml
```

### Step 3: Verify the Deployment

Check the FlameCluster status:

```bash
kubectl get flamecluster my-flame
```

Expected output when healthy:
```
NAME       STATE     SESSION-MANAGER   EXECUTORS   AGE
my-flame   Running   1/1               3/3         2m
```

View detailed status:

```bash
kubectl describe flamecluster my-flame
```

Check the created resources:

```bash
# View all pods
kubectl get pods -l flame.xflops.io/cluster=my-flame

# View services
kubectl get services -l flame.xflops.io/cluster=my-flame

# View the auto-generated ConfigMap
kubectl get configmap my-flame-config -o yaml
```

### Understanding the Created Resources

When you create a FlameCluster, the operator automatically creates:

| Resource | Name | Description |
|----------|------|-------------|
| Pod | `<name>-session-manager` | Session Manager coordinator |
| Pods | `<name>-executor-manager-0..N` | Executor Manager workers |
| Service | `<name>-session-manager` | ClusterIP service for Session Manager |
| Service | `<name>-object-cache` | ClusterIP service for Object Cache |
| ConfigMap | `<name>-config` | Auto-generated configuration |

All resources have `ownerReference` set to the FlameCluster, ensuring automatic cleanup on deletion.

## Scaling the Cluster

To scale the number of Executor Managers:

```bash
# Edit the FlameCluster
kubectl patch flamecluster my-flame --type='merge' -p '{"spec":{"executorManager":{"replicas":5}}}'

# Or edit directly
kubectl edit flamecluster my-flame
```

Verify the scaling:

```bash
kubectl get pods -l flame.xflops.io/cluster=my-flame,app=flame-executor-manager
```

## Updating Configuration

When you update configuration fields in the FlameCluster spec, the operator:

1. Regenerates the ConfigMap with new values
2. Recreates affected Pods with the updated configuration
3. Updates the `configGeneration` in status

Example - update the scheduling policy:

```bash
kubectl patch flamecluster my-flame --type='merge' -p '{"spec":{"sessionManager":{"policy":"fifo"}}}'
```

## Clean Up

To delete the FlameCluster and all its resources:

```bash
kubectl delete flamecluster my-flame
```

The Kubernetes garbage collector automatically deletes all child resources (Pods, Services, ConfigMap) via `ownerReference`.

Verify cleanup:

```bash
# Should return no resources
kubectl get pods -l flame.xflops.io/cluster=my-flame
kubectl get services -l flame.xflops.io/cluster=my-flame
kubectl get configmap my-flame-config
```

## Troubleshooting

### FlameCluster stuck in Pending state

Check the operator logs:

```bash
kubectl logs -n flame-system deployment/flame-operator-controller-manager
```

Common causes:
- Image pull errors (check image names and registry access)
- Resource constraints (check node resources)
- RBAC permissions (ensure operator has required permissions)

### Pods not starting

Check pod events:

```bash
kubectl describe pod <pod-name>
```

Check pod logs:

```bash
kubectl logs <pod-name>
```

### Session Manager not ready

The Session Manager must be ready before Executors can connect. Check:

```bash
kubectl get pod -l app=flame-session-manager,flame.xflops.io/cluster=my-flame
kubectl logs <session-manager-pod>
```

## Next Steps

- Read the [Configuration Reference](configuration.md) for all available options
- Explore [Example Configurations](examples/) for different use cases
- Check the [Flame documentation](https://github.com/xflops/flame) for application-level usage
