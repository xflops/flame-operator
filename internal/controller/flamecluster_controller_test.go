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

	// Verify empty fields remain empty after round-trip
	if parsed.Cluster.Slot != "" {
		t.Errorf("parsed.Cluster.Slot should be empty, got %q", parsed.Cluster.Slot)
	}
	if parsed.Cluster.Policy != "" {
		t.Errorf("parsed.Cluster.Policy should be empty, got %q", parsed.Cluster.Policy)
	}
	if parsed.Executors.Shim != "" {
		t.Errorf("parsed.Executors.Shim should be empty, got %q", parsed.Executors.Shim)
	}
	if parsed.Cache.NetworkInterface != "" {
		t.Errorf("parsed.Cache.NetworkInterface should be empty, got %q", parsed.Cache.NetworkInterface)
	}
}
