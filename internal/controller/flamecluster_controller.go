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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	Shim   string         `yaml:"shim,omitempty"`
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

const (
	// Label keys
	labelApp     = "app"
	labelCluster = "flame.xflops.io/cluster"

	// Component names
	componentSessionManager  = "flame-session-manager"
	componentExecutorManager = "flame-executor-manager"

	// Ports
	sessionManagerPort = 8080
	objectCachePort    = 9090

	// ConfigMap key
	configMapKey = "flame-cluster.yaml"

	// Environment variables
	envFlameConfig        = "FLAME_CONFIG"
	envSessionManagerAddr = "SESSION_MANAGER_ADDR"
	envObjectCacheAddr    = "OBJECT_CACHE_ADDR"
)

// FlameClusterReconciler reconciles a FlameCluster object
type FlameClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=flame.xflops.io,resources=flameclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=flame.xflops.io,resources=flameclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=flame.xflops.io,resources=flameclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods;services;configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop
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
		logger.Info("FlameCluster resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	// Defensive coding: Validate the spec before proceeding
	if err := r.validateSpec(&flameCluster); err != nil {
		logger.Error(err, "Invalid FlameCluster spec")
		// Update status to reflect invalid spec using Patch for better conflict handling
		if statusErr := r.updateStatusWithPatch(ctx, &flameCluster, "Failed", err.Error()); statusErr != nil {
			logger.Error(statusErr, "Failed to update status")
		}
		// Invalid spec is a user error, not a transient condition - don't requeue
		return ctrl.Result{}, nil
	}

	// 1. Reconcile ConfigMap
	if err := r.reconcileConfigMap(ctx, &flameCluster); err != nil {
		logger.Error(err, "Failed to reconcile ConfigMap")
		return r.updateStatusWithError(ctx, &flameCluster, "Failed", fmt.Sprintf("ConfigMap error: %v", err))
	}

	// 2. Reconcile Session Manager Service and Pod
	if err := r.reconcileSessionManager(ctx, &flameCluster); err != nil {
		logger.Error(err, "Failed to reconcile Session Manager")
		return r.updateStatusWithError(ctx, &flameCluster, "Failed", fmt.Sprintf("Session Manager error: %v", err))
	}

	// 3. Reconcile Object Cache Service
	if err := r.reconcileObjectCacheService(ctx, &flameCluster); err != nil {
		logger.Error(err, "Failed to reconcile Object Cache Service")
		return r.updateStatusWithError(ctx, &flameCluster, "Failed", fmt.Sprintf("Object Cache error: %v", err))
	}

	// 4. Reconcile Executor Manager Pods
	if err := r.reconcileExecutorManager(ctx, &flameCluster); err != nil {
		logger.Error(err, "Failed to reconcile Executor Manager")
		return r.updateStatusWithError(ctx, &flameCluster, "Failed", fmt.Sprintf("Executor Manager error: %v", err))
	}

	// 5. Update Status
	if err := r.updateStatus(ctx, &flameCluster); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

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

// ============================================================================
// ConfigMap Reconciliation
// ============================================================================

// reconcileConfigMap creates or updates the ConfigMap with cluster configuration
func (r *FlameClusterReconciler) reconcileConfigMap(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	logger := log.FromContext(ctx)
	configMap := r.buildConfigMap(cluster)

	// Set owner reference for garbage collection
	if err := controllerutil.SetControllerReference(cluster, configMap, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	// Create or update
	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKeyFromObject(configMap), existing)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating ConfigMap", "name", configMap.Name)
			return r.Create(ctx, configMap)
		}
		return err
	}

	// Update if data changed
	if existing.Data[configMapKey] != configMap.Data[configMapKey] {
		logger.Info("Updating ConfigMap", "name", configMap.Name)
		existing.Data = configMap.Data
		return r.Update(ctx, existing)
	}

	return nil
}

// buildConfigMap constructs the ConfigMap from FlameCluster spec using type-safe FlameConfigYaml
func (r *FlameClusterReconciler) buildConfigMap(cluster *flamev1alpha1.FlameCluster) *corev1.ConfigMap {
	config := r.buildFlameConfigYaml(cluster)
	configYAML, _ := r.marshalFlameConfig(config)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-config", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    r.buildLabels(cluster, "config"),
		},
		Data: map[string]string{
			configMapKey: string(configYAML),
		},
	}
}

// buildFlameConfigYaml creates a FlameConfigYaml struct from the FlameCluster spec.
// This builds the configuration in the format expected by Flame:
// https://raw.githubusercontent.com/xflops/flame/refs/heads/main/ci/flame-cluster.yaml
func (r *FlameClusterReconciler) buildFlameConfigYaml(cluster *flamev1alpha1.FlameCluster) *FlameConfigYaml {
	// Build service endpoints based on naming convention
	sessionManagerEndpoint := fmt.Sprintf("http://%s-session-manager:%d", cluster.Name, sessionManagerPort)
	cacheEndpoint := fmt.Sprintf("grpc://%s-object-cache:%d", cluster.Name, objectCachePort)

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

// ============================================================================
// Session Manager Reconciliation
// ============================================================================

// reconcileSessionManager ensures the Session Manager Service and Pod exist
func (r *FlameClusterReconciler) reconcileSessionManager(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	// First, create the Service
	if err := r.reconcileSessionManagerService(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile service: %w", err)
	}

	// Then, create the Pod
	if err := r.reconcileSessionManagerPod(ctx, cluster); err != nil {
		return fmt.Errorf("failed to reconcile pod: %w", err)
	}

	return nil
}

// reconcileSessionManagerService creates or updates the Session Manager Service
func (r *FlameClusterReconciler) reconcileSessionManagerService(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	logger := log.FromContext(ctx)
	service := r.buildSessionManagerService(cluster)

	if err := controllerutil.SetControllerReference(cluster, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(service), existing)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating Session Manager Service", "name", service.Name)
			return r.Create(ctx, service)
		}
		return err
	}

	// Service exists, no update needed for ClusterIP services
	return nil
}

// buildSessionManagerService constructs the Session Manager Service
func (r *FlameClusterReconciler) buildSessionManagerService(cluster *flamev1alpha1.FlameCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-session-manager", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    r.buildLabels(cluster, componentSessionManager),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				labelApp:     componentSessionManager,
				labelCluster: cluster.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       sessionManagerPort,
					TargetPort: intstr.FromInt(sessionManagerPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// reconcileSessionManagerPod creates or updates the Session Manager Pod
func (r *FlameClusterReconciler) reconcileSessionManagerPod(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	logger := log.FromContext(ctx)
	pod := r.buildSessionManagerPod(cluster)

	if err := controllerutil.SetControllerReference(cluster, pod, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	existing := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKeyFromObject(pod), existing)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating Session Manager Pod", "name", pod.Name)
			return r.Create(ctx, pod)
		}
		return err
	}

	// Fix #1: Bounds check before accessing Containers[0]
	if len(existing.Spec.Containers) == 0 {
		logger.Error(nil, "Session Manager Pod has no containers, recreating", "name", pod.Name)
		if err := r.Delete(ctx, existing); err != nil && !errors.IsNotFound(err) {
			return err
		}
		return r.Create(ctx, pod)
	}

	// Pod exists - check if it needs recreation (image changed)
	if existing.Spec.Containers[0].Image != cluster.Spec.SessionManager.Image {
		logger.Info("Recreating Session Manager Pod due to image change", "name", pod.Name)
		if err := r.Delete(ctx, existing); err != nil {
			return err
		}
		return r.Create(ctx, pod)
	}

	return nil
}

// buildSessionManagerPod constructs the Session Manager Pod
func (r *FlameClusterReconciler) buildSessionManagerPod(cluster *flamev1alpha1.FlameCluster) *corev1.Pod {
	configMapName := fmt.Sprintf("%s-config", cluster.Name)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-session-manager", cluster.Name),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				labelApp:     componentSessionManager,
				labelCluster: cluster.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:      "session-manager",
					Image:     cluster.Spec.SessionManager.Image,
					Resources: cluster.Spec.SessionManager.Resources,
					Ports: []corev1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: sessionManagerPort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  envFlameConfig,
							Value: fmt.Sprintf("/etc/flame/%s", configMapKey),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/etc/flame",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
						},
					},
				},
			},
		},
	}
}

// ============================================================================
// Object Cache Service Reconciliation
// ============================================================================

// reconcileObjectCacheService creates or updates the Object Cache Service
func (r *FlameClusterReconciler) reconcileObjectCacheService(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	logger := log.FromContext(ctx)
	service := r.buildObjectCacheService(cluster)

	if err := controllerutil.SetControllerReference(cluster, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	existing := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKeyFromObject(service), existing)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating Object Cache Service", "name", service.Name)
			return r.Create(ctx, service)
		}
		return err
	}

	return nil
}

// buildObjectCacheService constructs the Object Cache Service
// Per HLD, the Object Cache Service routes to Executor Manager pods
func (r *FlameClusterReconciler) buildObjectCacheService(cluster *flamev1alpha1.FlameCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-object-cache", cluster.Name),
			Namespace: cluster.Namespace,
			Labels:    r.buildLabels(cluster, "object-cache"),
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				labelApp:     componentExecutorManager,
				labelCluster: cluster.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       objectCachePort,
					TargetPort: intstr.FromInt(objectCachePort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// ============================================================================
// Executor Manager Reconciliation
// ============================================================================

// reconcileExecutorManager ensures the correct number of Executor Manager Pods exist
func (r *FlameClusterReconciler) reconcileExecutorManager(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	logger := log.FromContext(ctx)
	desiredReplicas := int(cluster.Spec.ExecutorManager.Replicas)

	// List existing executor pods
	existingPods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			labelApp:     componentExecutorManager,
			labelCluster: cluster.Name,
		},
	}

	if err := r.List(ctx, existingPods, listOpts...); err != nil {
		return fmt.Errorf("failed to list executor pods: %w", err)
	}

	currentCount := len(existingPods.Items)
	logger.Info("Executor Manager reconciliation", "current", currentCount, "desired", desiredReplicas)

	// Scale up: Create missing pods
	if currentCount < desiredReplicas {
		for i := currentCount; i < desiredReplicas; i++ {
			pod := r.buildExecutorManagerPod(cluster, i)
			if err := controllerutil.SetControllerReference(cluster, pod, r.Scheme); err != nil {
				return fmt.Errorf("failed to set owner reference: %w", err)
			}
			logger.Info("Creating Executor Manager Pod", "name", pod.Name)
			if err := r.Create(ctx, pod); err != nil {
				if !errors.IsAlreadyExists(err) {
					return err
				}
			}
		}
	}

	// Scale down: Delete excess pods
	if currentCount > desiredReplicas {
		// Delete pods in reverse order (highest index first)
		for i := currentCount - 1; i >= desiredReplicas; i-- {
			podName := fmt.Sprintf("%s-executor-manager-%d", cluster.Name, i)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: cluster.Namespace,
				},
			}
			logger.Info("Deleting Executor Manager Pod", "name", podName)
			if err := r.Delete(ctx, pod); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
		}
	}

	// Check for image updates on existing pods
	for _, existingPod := range existingPods.Items {
		if len(existingPod.Spec.Containers) > 0 &&
			existingPod.Spec.Containers[0].Image != cluster.Spec.ExecutorManager.Image {
			logger.Info("Recreating Executor Manager Pod due to image change", "name", existingPod.Name)
			if err := r.Delete(ctx, &existingPod); err != nil {
				if !errors.IsNotFound(err) {
					return err
				}
			}
			// Pod will be recreated in next reconciliation
		}
	}

	return nil
}

// buildExecutorManagerPod constructs an Executor Manager Pod
func (r *FlameClusterReconciler) buildExecutorManagerPod(cluster *flamev1alpha1.FlameCluster, index int) *corev1.Pod {
	configMapName := fmt.Sprintf("%s-config", cluster.Name)
	sessionManagerAddr := fmt.Sprintf("%s-session-manager:%d", cluster.Name, sessionManagerPort)
	objectCacheAddr := fmt.Sprintf("%s-object-cache:%d", cluster.Name, objectCachePort)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-executor-manager-%d", cluster.Name, index),
			Namespace: cluster.Namespace,
			Labels: map[string]string{
				labelApp:     componentExecutorManager,
				labelCluster: cluster.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:      "executor-manager",
					Image:     cluster.Spec.ExecutorManager.Image,
					Resources: cluster.Spec.ExecutorManager.Resources,
					Ports: []corev1.ContainerPort{
						{
							Name:          "grpc",
							ContainerPort: objectCachePort,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  envFlameConfig,
							Value: fmt.Sprintf("/etc/flame/%s", configMapKey),
						},
						{
							Name:  envSessionManagerAddr,
							Value: sessionManagerAddr,
						},
						{
							Name:  envObjectCacheAddr,
							Value: objectCacheAddr,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "config",
							MountPath: "/etc/flame",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
						},
					},
				},
			},
		},
	}
}

// ============================================================================
// Status Update
// ============================================================================

// updateStatus aggregates Pod status and updates FlameCluster.Status using Patch
func (r *FlameClusterReconciler) updateStatus(ctx context.Context, cluster *flamev1alpha1.FlameCluster) error {
	logger := log.FromContext(ctx)

	// Get Session Manager Pod status
	smPod := &corev1.Pod{}
	smPodName := fmt.Sprintf("%s-session-manager", cluster.Name)
	smReady := int32(0)
	if err := r.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: smPodName}, smPod); err == nil {
		if isPodReady(smPod) {
			smReady = 1
		}
	}

	// Get Executor Manager Pods status
	executorPods := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			labelApp:     componentExecutorManager,
			labelCluster: cluster.Name,
		},
	}

	emReady := int32(0)
	if err := r.List(ctx, executorPods, listOpts...); err == nil {
		for _, pod := range executorPods.Items {
			if isPodReady(&pod) {
				emReady++
			}
		}
	}

	// Determine overall state
	state := "Pending"
	message := "Waiting for components to be ready"

	if smReady > 0 && emReady > 0 {
		state = "Running"
		message = "All components healthy"
	} else if smReady == 0 {
		state = "Pending"
		message = "Session Manager not ready"
	} else if emReady == 0 {
		state = "Pending"
		message = fmt.Sprintf("Waiting for Executor Managers (0/%d ready)", cluster.Spec.ExecutorManager.Replicas)
	}

	// Create a patch from the current state (better conflict handling)
	patch := client.MergeFrom(cluster.DeepCopy())

	// Update status fields
	cluster.Status.SessionManager.Ready = smReady
	cluster.Status.SessionManager.Endpoint = fmt.Sprintf("http://%s-session-manager:%d", cluster.Name, sessionManagerPort)
	cluster.Status.ExecutorManager.Replicas = cluster.Spec.ExecutorManager.Replicas
	cluster.Status.ExecutorManager.Ready = emReady
	cluster.Status.State = state
	cluster.Status.Message = message
	cluster.Status.ObservedGeneration = cluster.Generation

	logger.Info("Updating status", "state", state, "smReady", smReady, "emReady", emReady)
	return r.Status().Patch(ctx, cluster, patch)
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

// updateStatusWithError updates the FlameCluster status with an error state
func (r *FlameClusterReconciler) updateStatusWithError(ctx context.Context, cluster *flamev1alpha1.FlameCluster, state, message string) (ctrl.Result, error) {
	if err := r.updateStatusWithPatch(ctx, cluster, state, message); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// buildLabels creates standard labels for resources
func (r *FlameClusterReconciler) buildLabels(cluster *flamev1alpha1.FlameCluster, component string) map[string]string {
	return map[string]string{
		labelApp:     component,
		labelCluster: cluster.Name,
	}
}

// isPodReady checks if a Pod is in Ready condition
func isPodReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
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
