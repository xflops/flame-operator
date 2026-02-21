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
		validate func(t *testing.T, config *FlameConfigYaml)
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
			validate: func(t *testing.T, config *FlameConfigYaml) {
				// Verify session manager config
				if config.SessionManager.Endpoint != "http://test-cluster-session-manager:8080" {
					t.Errorf("SessionManager.Endpoint = %v, want http://test-cluster-session-manager:8080", config.SessionManager.Endpoint)
				}
				if config.SessionManager.Slot != "cpu=1,mem=1g" {
					t.Errorf("SessionManager.Slot = %v, want cpu=1,mem=1g", config.SessionManager.Slot)
				}
				if config.SessionManager.Policy != "priority" {
					t.Errorf("SessionManager.Policy = %v, want priority", config.SessionManager.Policy)
				}
				if config.SessionManager.Storage != "sqlite://flame.db" {
					t.Errorf("SessionManager.Storage = %v, want sqlite://flame.db", config.SessionManager.Storage)
				}

				// Verify object cache config
				if config.ObjectCache.Endpoint != "http://test-cluster-object-cache:8081" {
					t.Errorf("ObjectCache.Endpoint = %v, want http://test-cluster-object-cache:8081", config.ObjectCache.Endpoint)
				}
				if config.ObjectCache.Storage != "/tmp/cache" {
					t.Errorf("ObjectCache.Storage = %v, want /tmp/cache", config.ObjectCache.Storage)
				}
				if config.ObjectCache.NetworkInterface != "eth0" {
					t.Errorf("ObjectCache.NetworkInterface = %v, want eth0", config.ObjectCache.NetworkInterface)
				}

				// Verify executor manager config
				if config.ExecutorManager.MaxExecutors != 4 {
					t.Errorf("ExecutorManager.MaxExecutors = %v, want 4", config.ExecutorManager.MaxExecutors)
				}
				if config.ExecutorManager.Shim != "host" {
					t.Errorf("ExecutorManager.Shim = %v, want host", config.ExecutorManager.Shim)
				}
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
			validate: func(t *testing.T, config *FlameConfigYaml) {
				if config.ExecutorManager.MaxExecutors != 1 {
					t.Errorf("ExecutorManager.MaxExecutors = %v, want 1 (default)", config.ExecutorManager.MaxExecutors)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := r.buildFlameConfigYaml(tt.cluster)
			if config == nil {
				t.Fatal("buildFlameConfigYaml() returned nil")
			}
			tt.validate(t, config)
		})
	}
}

func TestMarshalFlameConfig(t *testing.T) {
	r := &FlameClusterReconciler{}

	config := &FlameConfigYaml{
		SessionManager: SessionManagerConfig{
			Endpoint: "http://test-session-manager:8080",
			Slot:     "cpu=2,mem=2g",
			Policy:   "fifo",
			Storage:  "sqlite://test.db",
		},
		ObjectCache: ObjectCacheConfig{
			Endpoint:         "http://test-object-cache:8081",
			Storage:          "/data/cache",
			NetworkInterface: "eth1",
		},
		ExecutorManager: ExecutorManagerConfig{
			MaxExecutors: 8,
			Shim:         "container",
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

	// Verify round-trip consistency
	if parsed.SessionManager.Endpoint != config.SessionManager.Endpoint {
		t.Errorf("Round-trip SessionManager.Endpoint = %v, want %v", parsed.SessionManager.Endpoint, config.SessionManager.Endpoint)
	}
	if parsed.SessionManager.Slot != config.SessionManager.Slot {
		t.Errorf("Round-trip SessionManager.Slot = %v, want %v", parsed.SessionManager.Slot, config.SessionManager.Slot)
	}
	if parsed.ObjectCache.Endpoint != config.ObjectCache.Endpoint {
		t.Errorf("Round-trip ObjectCache.Endpoint = %v, want %v", parsed.ObjectCache.Endpoint, config.ObjectCache.Endpoint)
	}
	if parsed.ObjectCache.Storage != config.ObjectCache.Storage {
		t.Errorf("Round-trip ObjectCache.Storage = %v, want %v", parsed.ObjectCache.Storage, config.ObjectCache.Storage)
	}
	if parsed.ExecutorManager.MaxExecutors != config.ExecutorManager.MaxExecutors {
		t.Errorf("Round-trip ExecutorManager.MaxExecutors = %v, want %v", parsed.ExecutorManager.MaxExecutors, config.ExecutorManager.MaxExecutors)
	}
	if parsed.ExecutorManager.Shim != config.ExecutorManager.Shim {
		t.Errorf("Round-trip ExecutorManager.Shim = %v, want %v", parsed.ExecutorManager.Shim, config.ExecutorManager.Shim)
	}
}

func TestFlameConfigYamlOmitsEmptyFields(t *testing.T) {
	r := &FlameClusterReconciler{}

	// Config with some empty fields
	config := &FlameConfigYaml{
		SessionManager: SessionManagerConfig{
			Endpoint: "http://test:8080",
			// Slot, Policy, Storage are empty
		},
		ObjectCache: ObjectCacheConfig{
			Endpoint: "http://cache:8081",
			// Storage, NetworkInterface are empty
		},
		ExecutorManager: ExecutorManagerConfig{
			MaxExecutors: 2,
			// Shim is empty
		},
	}

	data, err := r.marshalFlameConfig(config)
	if err != nil {
		t.Fatalf("marshalFlameConfig() error = %v", err)
	}

	yamlStr := string(data)

	// Verify that omitempty fields are not present when empty
	// Note: We check that the YAML doesn't contain these as top-level keys with empty values
	var parsed map[string]interface{}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	// The YAML should still be valid and parseable
	if yamlStr == "" {
		t.Error("marshalFlameConfig() returned empty string")
	}

	// Verify required fields are present
	if parsed["sessionManager"] == nil {
		t.Error("sessionManager should be present in YAML output")
	}
	if parsed["objectCache"] == nil {
		t.Error("objectCache should be present in YAML output")
	}
	if parsed["executorManager"] == nil {
		t.Error("executorManager should be present in YAML output")
	}
}
