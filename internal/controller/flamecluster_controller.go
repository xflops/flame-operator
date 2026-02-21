/*
Copyright 2024 The Flame Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	flamev1alpha1 "github.com/xflops/flame-operator/api/v1alpha1"
)

// FlameConfigYaml represents the flame-cluster.yaml configuration.
// This struct matches the actual Flame configuration format from:
// https://raw.githubusercontent.com/xflops/flame/refs/heads/main/ci/flame-cluster.yaml
type FlameConfigYaml struct {
	Cluster   ClusterConfig   `yaml:"cluster"`
	Executors ExecutorsConfig `yaml:"executors"`
	Cache     CacheConfig     `yaml:"cache"`
}

// ClusterConfig holds the cluster configuration section.
type ClusterConfig struct {
	Name     string `yaml:"name"`
	Endpoint string `yaml:"endpoint"`
	Slot     string `yaml:"slot,omitempty"`
	Policy   string `yaml:"policy,omitempty"`
	Storage  string `yaml:"storage,omitempty"`
}

// ExecutorsConfig holds the executors configuration section.
type ExecutorsConfig struct {
	Shim   string        `yaml:"shim,omitempty"`
	Limits ExecutorLimits `yaml:"limits,omitempty"`
}

// ExecutorLimits holds the executor limits configuration.
type ExecutorLimits struct {
	MaxExecutors int32 `yaml:"max_executors,omitempty"`
}

// CacheConfig holds the cache configuration section.
type CacheConfig struct {
	Endpoint         string `yaml:"endpoint"`
	NetworkInterface string `yaml:"network_interface,omitempty"`
	Storage          string `yaml:"storage,omitempty"`
}

// FlameClusterReconciler reconciles a FlameCluster object
type FlameClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=flame.xflops.io,resources=flameclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=flame.xflops.io,resources=flameclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=flame.xflops.io,resources=flameclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods;services;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *FlameClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Starting reconciliation", "request", req.NamespacedName)

	// Fetch the FlameCluster instance
	var flameCluster flamev1alpha1.FlameCluster
	if err := r.Get(ctx, req.NamespacedName, &flameCluster); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to fetch FlameCluster")
			return ctrl.Result{}, fmt.Errorf("failed to fetch FlameCluster: %w", err)
		}
		// Request object not found, could have been deleted after reconcile request.
		// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
		// Return and don't requeue
		logger.Info("FlameCluster resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	// Defensive coding: Validate the spec before proceeding
	if err := r.validateSpec(&flameCluster); err != nil {
		logger.Error(err, "Invalid FlameCluster spec")
		// Update status to reflect invalid spec
		if statusErr := r.updateStatusWithPatch(ctx, &flameCluster, "Failed", err.Error()); statusErr != nil {
			logger.Error(statusErr, "Failed to update status")
		}
		// Invalid spec is a user error, not a transient condition - don't requeue
		return ctrl.Result{}, nil
	}


	// 1. Generate ConfigMap from FlameCluster.spec
	//    - Populate cluster.endpoint, cache.endpoint
	//    - Create/Update ConfigMap

	// 2. Reconcile Session Manager
	//    - Create Service for Session Manager
	//    - Ensure single Pod exists
	//    - Mount ConfigMap

	// 3. Reconcile Object Cache
	//    - Create Service for Object Cache
	//    - Note: Object Cache runs as sidecar or separate deployment? HLD implies separate service.
	//    - HLD says "Operator creates the Object Cache Service". It doesn't explicitly mention a separate Pod for cache,
	//      but implies it might be part of Executor Manager or Session Manager, or standalone.
	//      Actually, "Executor Manager: Workers that execute tasks".
	//      Let's assume Object Cache logic needs clarification or is part of the standard deployment.

	// 4. Reconcile Executor Manager
	//    - Ensure N Pods exist (from CRD replicas)
	//    - Mount ConfigMap
	//    - Inject SESSION_MANAGER_ADDR and OBJECT_CACHE_ADDR env vars

	// 5. Update Status
	//    - Aggregate Pod status
	//    - Update FlameCluster.Status

	logger.Info("Reconciliation completed successfully")
	return ctrl.Result{}, nil
}

// validateSpec checks if the FlameCluster spec is valid
func (r *FlameClusterReconciler) validateSpec(cluster *flamev1alpha1.FlameCluster) error {
	if cluster.Spec.SessionManager.Image == "" {
		return fmt.Errorf("spec.sessionManager.image is required")
	}
	if cluster.Spec.ExecutorManager.Image == "" {
		return fmt.Errorf("spec.executorManager.image is required")
	}
	return nil
}

// updateStatusWithPatch updates the FlameCluster status using Patch instead of Update.
// Using Patch with MergeFrom is more resilient to concurrent modifications
// and reduces conflict errors in high-concurrency scenarios.
func (r *FlameClusterReconciler) updateStatusWithPatch(ctx context.Context, cluster *flamev1alpha1.FlameCluster, state, message string) error {
	// Create a patch from the current state
	patch := client.MergeFrom(cluster.DeepCopy())

	// Update status fields
	cluster.Status.State = state
	cluster.Status.Message = message
	cluster.Status.ObservedGeneration = cluster.Generation

	// Apply the patch - only updates changed fields
	return r.Status().Patch(ctx, cluster, patch)
}

// buildFlameConfigYaml creates a FlameConfigYaml struct from the FlameCluster spec.
// This builds the configuration in the format expected by Flame:
// https://raw.githubusercontent.com/xflops/flame/refs/heads/main/ci/flame-cluster.yaml
func (r *FlameClusterReconciler) buildFlameConfigYaml(cluster *flamev1alpha1.FlameCluster) *FlameConfigYaml {
	// Build service endpoints based on naming convention
	sessionManagerEndpoint := fmt.Sprintf("http://%s-session-manager:8080", cluster.Name)
	cacheEndpoint := fmt.Sprintf("grpc://%s-executor-manager:9090", cluster.Name)

	config := &FlameConfigYaml{
		Cluster: ClusterConfig{
			Name:     cluster.Name,
			Endpoint: sessionManagerEndpoint,
			Slot:     cluster.Spec.SessionManager.Slot,
			Policy:   cluster.Spec.SessionManager.Policy,
			Storage:  cluster.Spec.SessionManager.Storage,
		},
		Executors: ExecutorsConfig{
			Shim: cluster.Spec.ExecutorManager.Shim,
			Limits: ExecutorLimits{
				MaxExecutors: cluster.Spec.ExecutorManager.MaxExecutors,
			},
		},
		Cache: CacheConfig{
			Endpoint:         cacheEndpoint,
			NetworkInterface: cluster.Spec.ObjectCache.NetworkInterface,
			Storage:          cluster.Spec.ObjectCache.Storage,
		},
	}

	// Set default max executors if not specified
	if config.Executors.Limits.MaxExecutors == 0 {
		config.Executors.Limits.MaxExecutors = 1
	}

	return config
}

// marshalFlameConfig serializes the FlameConfigYaml to YAML bytes.
// Returns the YAML content suitable for ConfigMap data.
func (r *FlameClusterReconciler) marshalFlameConfig(config *FlameConfigYaml) ([]byte, error) {
	return yaml.Marshal(config)
}

// SetupWithManager sets up the controller with the Manager.
func (r *FlameClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flamev1alpha1.FlameCluster{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
