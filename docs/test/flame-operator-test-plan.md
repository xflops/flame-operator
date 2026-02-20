# Test Plan: Flame Operator MVP

**HLD Reference:** [docs/design/flame-operator-hld.md](../design/flame-operator-hld.md)
**Author:** Diana (QA)
**Date:** 2026-02-20
**Status:** Draft

## 1. Scope

### In Scope
- **Deployment (UC1):** Validation of `FlameCluster` CR creation and resource generation.
- **Scaling (UC2):** Validation of Executor Manager scaling operations.
- **Cleanup (UC3):** Validation of cluster deletion and garbage collection.
- **Config Update (UC4):** Validation of configuration updates and propagation.
- **Negative Testing:** Validation of error handling for invalid configurations and state.

### Out of Scope
- Advanced Networking (Ingress/External access)
- Persistent Storage for Session Manager
- Multi-cluster Federation
- High Availability (HA) for Session Manager
- Performance/Load testing (Functional validation only)

## 2. Test Strategy

### Test Types
- [x] Functional Testing (Positive/Negative)
- [x] Integration Testing (K8s API interactions)
- [x] Regression Testing (Not applicable for MVP)
- [ ] Performance Testing
- [ ] Security Testing

### Test Environment
- **Environment:** Kubernetes Cluster (Kind/Minikube/EKS)
- **Dependencies:** 
  - Kubernetes API Server
  - Flame Operator installed
  - Container Registry (for Flame images)

## 3. Use Case Coverage

| Use Case | Test Scenarios | Priority |
|----------|----------------|----------|
| UC1: Deploy Flame Cluster | 3 scenarios | High |
| UC2: Scale Executors | 3 scenarios | High |
| UC3: Delete Cluster | 1 scenario | High |
| UC4: Update Configuration | 2 scenarios | Medium |
| Negative Cases | 4 scenarios | High |

## 4. Test Scenarios

### UC1: Deploy Flame Cluster

| ID | Scenario | Type | Priority | Expected Result |
|----|----------|------|----------|-----------------|
| TC-001 | Deploy valid FlameCluster CR | Functional | High | CR created, Status=Running, All resources created |
| TC-002 | Verify Resource Ownership | Functional | High | Pods, Services, ConfigMap have OwnerReference to CR |
| TC-003 | Verify Service Discovery | Functional | High | Services created with correct names/ports, DNS resolvable |

### UC2: Scale Executors

| ID | Scenario | Type | Priority | Expected Result |
|----|----------|------|----------|-----------------|
| TC-004 | Scale Up Executors | Functional | High | Pod count increases to match `replicas`, Status updates |
| TC-005 | Scale Down Executors | Functional | High | Pod count decreases to match `replicas`, Status updates |
| TC-006 | Scale to Zero Executors | Edge Case | Medium | All executor pods terminated, Session Manager remains |

### UC3: Delete Cluster

| ID | Scenario | Type | Priority | Expected Result |
|----|----------|------|----------|-----------------|
| TC-007 | Delete FlameCluster CR | Functional | High | CR deleted, all child resources (Pods, SVC, CM) garbage collected |

### UC4: Update Configuration

| ID | Scenario | Type | Priority | Expected Result |
|----|----------|------|----------|-----------------|
| TC-008 | Update `maxExecutors` config | Functional | Medium | ConfigMap updated, Executor Pods rolled out with new config |
| TC-009 | Update `sessionManager` image | Functional | Medium | Session Manager Pod recreated with new image |

### Negative Cases

| ID | Scenario | Type | Priority | Expected Result |
|----|----------|------|----------|-----------------|
| TC-010 | Invalid Image Name | Negative | High | Pods in ImagePullBackOff, CR Status reflects failure/error |
| TC-011 | Negative Replicas | Negative | High | Admission Webhook rejects CR (if implemented) or Controller handles gracefully |
| TC-012 | Missing Required Spec Fields | Negative | High | CR creation fails (Validation error) |
| TC-013 | Invalid Resource Quotas | Negative | Medium | Pods Pending (Insufficient cpu/mem), Status reflects Pending |

## 5. Test Data Requirements

| Data Type | Description | Source |
|-----------|-------------|--------|
| Valid CR | Standard FlameCluster manifest | `examples/flame-cluster.yaml` |
| Invalid CR | Manifest with missing fields | Created manually |
| Update CR | Manifest with modified fields | Created manually |

## 6. Dependencies & Risks

### Dependencies
- Operator must be deployed and running in the cluster.
- CRD must be installed (`kubectl apply -f config/crd/bases`).

### Risks
| Risk | Impact | Mitigation |
|------|--------|------------|
| Race conditions in startup | High | Verify Readiness Probes and Init Containers logic |
| Resource exhaustion on test node | Medium | Use small resource requests for test pods |

## 7. Exit Criteria

- [ ] All High priority test cases passed.
- [ ] No Critical/High severity defects open.
- [ ] Deployment, Scaling, and Cleanup flows verified.
