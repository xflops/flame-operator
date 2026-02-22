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
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	flamev1alpha1 "github.com/xflops/flame-operator/api/v1alpha1"
)

func TestValidateSpec(t *testing.T) {
	r := &FlameClusterReconciler{}

	tests := []struct {
		name    string
		cluster *flamev1alpha1.FlameCluster
		wantErr bool
	}{
		{
			name: "valid spec",
			cluster: &flamev1alpha1.FlameCluster{
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "valid-image",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image: "valid-image",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing session manager image",
			cluster: &flamev1alpha1.FlameCluster{
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image: "valid-image",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing executor manager image",
			cluster: &flamev1alpha1.FlameCluster{
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "valid-image",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image: "",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := r.validateSpec(tt.cluster); (err != nil) != tt.wantErr {
				t.Errorf("validateSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildFlameConfigYaml(t *testing.T) {
	r := &FlameClusterReconciler{}

	tests := []struct {
		name     string
		cluster  *flamev1alpha1.FlameCluster
		expected *FlameConfigYaml
	}{
		{
			name: "basic config generation",
			cluster: &flamev1alpha1.FlameCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image:   "session-manager:latest",
						Slot:    "cpu=1,mem=1g",
						Policy:  "priority",
						Storage: "sqlite://flame.db",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image:        "executor-manager:latest",
						Replicas:     3,
						MaxExecutors: 4,
						Shim:         "host",
					},
					ObjectCache: flamev1alpha1.ObjectCacheSpec{
						NetworkInterface: "eth0",
						Storage:          "/tmp/cache",
					},
				},
			},
			expected: &FlameConfigYaml{
				Cluster: ClusterConfig{
					Name:     "test-cluster",
					Endpoint: "http://test-cluster-session-manager:8080",
					Slot:     "cpu=1,mem=1g",
					Policy:   "priority",
					Storage:  "sqlite://flame.db",
				},
				Executors: ExecutorsConfig{
					Shim: "host",
					Limits: ExecutorLimits{
						MaxExecutors: 4,
					},
				},
				Cache: CacheConfig{
					Endpoint:         "grpc://test-cluster-object-cache:9090",
					NetworkInterface: "eth0",
					Storage:          "/tmp/cache",
				},
			},
		},
		{
			name: "default max executors when not specified",
			cluster: &flamev1alpha1.FlameCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "minimal-cluster",
					Namespace: "default",
				},
				Spec: flamev1alpha1.FlameClusterSpec{
					SessionManager: flamev1alpha1.SessionManagerSpec{
						Image: "session-manager:latest",
					},
					ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
						Image:        "executor-manager:latest",
						MaxExecutors: 0, // not specified
					},
				},
			},
			expected: &FlameConfigYaml{
				Cluster: ClusterConfig{
					Name:     "minimal-cluster",
					Endpoint: "http://minimal-cluster-session-manager:8080",
				},
				Executors: ExecutorsConfig{
					Limits: ExecutorLimits{
						MaxExecutors: 1, // default value
					},
				},
				Cache: CacheConfig{
					Endpoint: "grpc://minimal-cluster-object-cache:9090",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := r.buildFlameConfigYaml(tt.cluster)
			if config == nil {
				t.Fatal("buildFlameConfigYaml() returned nil")
			}
			// Use reflect.DeepEqual for robust struct comparison
			if !reflect.DeepEqual(config, tt.expected) {
				t.Errorf("buildFlameConfigYaml() mismatch:\ngot:  %+v\nwant: %+v", config, tt.expected)
			}
		})
	}
}

func TestMarshalFlameConfig(t *testing.T) {
	r := &FlameClusterReconciler{}

	config := &FlameConfigYaml{
		Cluster: ClusterConfig{
			Name:     "test-cluster",
			Endpoint: "http://test-session-manager:8080",
			Slot:     "cpu=2,mem=2g",
			Policy:   "fifo",
			Storage:  "sqlite://test.db",
		},
		Executors: ExecutorsConfig{
			Shim: "container",
			Limits: ExecutorLimits{
				MaxExecutors: 8,
			},
		},
		Cache: CacheConfig{
			Endpoint:         "grpc://test-object-cache:9090",
			Storage:          "/data/cache",
			NetworkInterface: "eth1",
		},
	}

	data, err := r.marshalFlameConfig(config)
	if err != nil {
		t.Fatalf("marshalFlameConfig() error = %v", err)
	}

	// Verify the YAML can be unmarshaled back
	var parsed FlameConfigYaml
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	// Use reflect.DeepEqual for robust round-trip verification
	if !reflect.DeepEqual(&parsed, config) {
		t.Errorf("Round-trip mismatch:\ngot:  %+v\nwant: %+v", parsed, config)
	}
}

func TestFlameConfigYamlOmitsEmptyFields(t *testing.T) {
	r := &FlameClusterReconciler{}

	// Config with some empty fields - only required fields set
	config := &FlameConfigYaml{
		Cluster: ClusterConfig{
			Name:     "test",
			Endpoint: "http://test:8080",
			// Slot, Policy, Storage are empty - should be omitted
		},
		Executors: ExecutorsConfig{
			// Shim is empty - should be omitted
			Limits: ExecutorLimits{
				MaxExecutors: 2,
			},
		},
		Cache: CacheConfig{
			Endpoint: "grpc://cache:9090",
			// Storage, NetworkInterface are empty - should be omitted
		},
	}

	data, err := r.marshalFlameConfig(config)
	if err != nil {
		t.Fatalf("marshalFlameConfig() error = %v", err)
	}

	yamlStr := string(data)

	// Verify that omitempty fields are NOT present in the YAML output when empty
	omitEmptyFields := []string{
		"slot:",
		"policy:",
		"storage:",      // in cluster section
		"shim:",
		"network_interface:",
	}

	for _, field := range omitEmptyFields {
		if strings.Contains(yamlStr, field) {
			t.Errorf("YAML output should not contain empty field %q, but got:\n%s", field, yamlStr)
		}
	}

	// Verify required fields ARE present
	requiredFields := []string{
		"cluster:",
		"name:",
		"endpoint:",
		"executors:",
		"limits:",
		"max_executors:",
		"cache:",
	}

	for _, field := range requiredFields {
		if !strings.Contains(yamlStr, field) {
			t.Errorf("YAML output should contain required field %q, but got:\n%s", field, yamlStr)
		}
	}

	// Verify the YAML is still valid and parseable
	var parsed FlameConfigYaml
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	// Verify the parsed values match expected
	if parsed.Cluster.Name != "test" {
		t.Errorf("parsed.Cluster.Name = %v, want test", parsed.Cluster.Name)
	}
	if parsed.Cluster.Endpoint != "http://test:8080" {
		t.Errorf("parsed.Cluster.Endpoint = %v, want http://test:8080", parsed.Cluster.Endpoint)
	}
	if parsed.Executors.Limits.MaxExecutors != 2 {
		t.Errorf("parsed.Executors.Limits.MaxExecutors = %v, want 2", parsed.Executors.Limits.MaxExecutors)
	}
	if parsed.Cache.Endpoint != "grpc://cache:9090" {
		t.Errorf("parsed.Cache.Endpoint = %v, want grpc://cache:9090", parsed.Cache.Endpoint)
	}
}

func TestBuildConfigMap(t *testing.T) {
	r := &FlameClusterReconciler{}

	cluster := &flamev1alpha1.FlameCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: flamev1alpha1.FlameClusterSpec{
			SessionManager: flamev1alpha1.SessionManagerSpec{
				Image:   "xflops/flame-session:v0.1.0",
				Slot:    "cpu=1,mem=1g",
				Policy:  "priority",
				Storage: "sqlite://flame.db",
			},
			ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
				Image:        "xflops/flame-executor:v0.1.0",
				Replicas:     3,
				Shim:         "host",
				MaxExecutors: 10,
			},
			ObjectCache: flamev1alpha1.ObjectCacheSpec{
				NetworkInterface: "eth0",
				Storage:          "/var/lib/flame/cache",
			},
		},
	}

	cm := r.buildConfigMap(cluster)

	// Verify ConfigMap metadata
	if cm.Name != "test-cluster-config" {
		t.Errorf("Expected ConfigMap name 'test-cluster-config', got '%s'", cm.Name)
	}
	if cm.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", cm.Namespace)
	}

	// Verify labels
	if cm.Labels[labelCluster] != "test-cluster" {
		t.Errorf("Expected label '%s'='test-cluster', got '%s'", labelCluster, cm.Labels[labelCluster])
	}

	// Verify ConfigMap data exists
	if _, ok := cm.Data[configMapKey]; !ok {
		t.Errorf("Expected ConfigMap to have key '%s'", configMapKey)
	}
}

func TestBuildSessionManagerService(t *testing.T) {
	r := &FlameClusterReconciler{}

	cluster := &flamev1alpha1.FlameCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: flamev1alpha1.FlameClusterSpec{
			SessionManager: flamev1alpha1.SessionManagerSpec{
				Image: "xflops/flame-session:v0.1.0",
			},
		},
	}

	svc := r.buildSessionManagerService(cluster)

	// Verify Service metadata
	if svc.Name != "test-cluster-session-manager" {
		t.Errorf("Expected Service name 'test-cluster-session-manager', got '%s'", svc.Name)
	}
	if svc.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", svc.Namespace)
	}

	// Verify Service type
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("Expected ServiceType ClusterIP, got '%s'", svc.Spec.Type)
	}

	// Verify selector
	if svc.Spec.Selector[labelApp] != componentSessionManager {
		t.Errorf("Expected selector app='%s', got '%s'", componentSessionManager, svc.Spec.Selector[labelApp])
	}
	if svc.Spec.Selector[labelCluster] != "test-cluster" {
		t.Errorf("Expected selector cluster='test-cluster', got '%s'", svc.Spec.Selector[labelCluster])
	}

	// Verify port
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("Expected 1 port, got %d", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Port != sessionManagerPort {
		t.Errorf("Expected port %d, got %d", sessionManagerPort, svc.Spec.Ports[0].Port)
	}
}

func TestBuildObjectCacheService(t *testing.T) {
	r := &FlameClusterReconciler{}

	cluster := &flamev1alpha1.FlameCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	svc := r.buildObjectCacheService(cluster)

	// Verify Service metadata
	if svc.Name != "test-cluster-object-cache" {
		t.Errorf("Expected Service name 'test-cluster-object-cache', got '%s'", svc.Name)
	}

	// Verify selector points to executor manager (per HLD)
	if svc.Spec.Selector[labelApp] != componentExecutorManager {
		t.Errorf("Expected selector app='%s', got '%s'", componentExecutorManager, svc.Spec.Selector[labelApp])
	}

	// Verify port
	if len(svc.Spec.Ports) != 1 {
		t.Fatalf("Expected 1 port, got %d", len(svc.Spec.Ports))
	}
	if svc.Spec.Ports[0].Port != objectCachePort {
		t.Errorf("Expected port %d, got %d", objectCachePort, svc.Spec.Ports[0].Port)
	}
}

func TestBuildSessionManagerPod(t *testing.T) {
	r := &FlameClusterReconciler{}

	cluster := &flamev1alpha1.FlameCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: flamev1alpha1.FlameClusterSpec{
			SessionManager: flamev1alpha1.SessionManagerSpec{
				Image: "xflops/flame-session:v0.1.0",
			},
		},
	}

	pod := r.buildSessionManagerPod(cluster)

	// Verify Pod metadata
	if pod.Name != "test-cluster-session-manager" {
		t.Errorf("Expected Pod name 'test-cluster-session-manager', got '%s'", pod.Name)
	}
	if pod.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", pod.Namespace)
	}

	// Verify labels
	if pod.Labels[labelApp] != componentSessionManager {
		t.Errorf("Expected label app='%s', got '%s'", componentSessionManager, pod.Labels[labelApp])
	}
	if pod.Labels[labelCluster] != "test-cluster" {
		t.Errorf("Expected label cluster='test-cluster', got '%s'", pod.Labels[labelCluster])
	}

	// Verify container
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	if container.Image != "xflops/flame-session:v0.1.0" {
		t.Errorf("Expected image 'xflops/flame-session:v0.1.0', got '%s'", container.Image)
	}

	// Verify volume mount
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != "/etc/flame" {
		t.Errorf("Expected mount path '/etc/flame', got '%s'", container.VolumeMounts[0].MountPath)
	}

	// Verify env var
	foundEnv := false
	for _, env := range container.Env {
		if env.Name == envFlameConfig {
			foundEnv = true
			break
		}
	}
	if !foundEnv {
		t.Errorf("Expected env var '%s' not found", envFlameConfig)
	}
}

func TestBuildExecutorManagerPod(t *testing.T) {
	r := &FlameClusterReconciler{}

	cluster := &flamev1alpha1.FlameCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
		Spec: flamev1alpha1.FlameClusterSpec{
			ExecutorManager: flamev1alpha1.ExecutorManagerSpec{
				Image:    "xflops/flame-executor:v0.1.0",
				Replicas: 3,
			},
		},
	}

	pod := r.buildExecutorManagerPod(cluster, 2)

	// Verify Pod metadata
	if pod.Name != "test-cluster-executor-manager-2" {
		t.Errorf("Expected Pod name 'test-cluster-executor-manager-2', got '%s'", pod.Name)
	}
	if pod.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", pod.Namespace)
	}

	// Verify labels
	if pod.Labels[labelApp] != componentExecutorManager {
		t.Errorf("Expected label app='%s', got '%s'", componentExecutorManager, pod.Labels[labelApp])
	}
	if pod.Labels[labelCluster] != "test-cluster" {
		t.Errorf("Expected label cluster='test-cluster', got '%s'", pod.Labels[labelCluster])
	}

	// Verify container
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(pod.Spec.Containers))
	}
	container := pod.Spec.Containers[0]
	if container.Image != "xflops/flame-executor:v0.1.0" {
		t.Errorf("Expected image 'xflops/flame-executor:v0.1.0', got '%s'", container.Image)
	}

	// Verify env vars
	envVars := make(map[string]string)
	for _, env := range container.Env {
		envVars[env.Name] = env.Value
	}

	if _, ok := envVars[envFlameConfig]; !ok {
		t.Errorf("Expected env var '%s' not found", envFlameConfig)
	}
	if _, ok := envVars[envSessionManagerAddr]; !ok {
		t.Errorf("Expected env var '%s' not found", envSessionManagerAddr)
	}
	if _, ok := envVars[envObjectCacheAddr]; !ok {
		t.Errorf("Expected env var '%s' not found", envObjectCacheAddr)
	}

	// Verify volume mount
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volume mount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].MountPath != "/etc/flame" {
		t.Errorf("Expected mount path '/etc/flame', got '%s'", container.VolumeMounts[0].MountPath)
	}
}

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod running and ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod running but not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod pending",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expected: false,
		},
		{
			name: "pod failed",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			if result != tt.expected {
				t.Errorf("isPodReady() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestBuildLabels(t *testing.T) {
	r := &FlameClusterReconciler{}

	cluster := &flamev1alpha1.FlameCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	labels := r.buildLabels(cluster, "test-component")

	if labels[labelApp] != "test-component" {
		t.Errorf("Expected label app='test-component', got '%s'", labels[labelApp])
	}
	if labels[labelCluster] != "test-cluster" {
		t.Errorf("Expected label cluster='test-cluster', got '%s'", labels[labelCluster])
	}
}
