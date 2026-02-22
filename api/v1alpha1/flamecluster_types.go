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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SlotSpec defines the resource slot for task scheduling.
// Uses standard Kubernetes resource.Quantity for proper unit handling and validation.
type SlotSpec struct {
	// CPU is the CPU resource quantity for the slot.
	// +optional
	CPU resource.Quantity `json:"cpu,omitempty"`

	// Memory is the memory resource quantity for the slot.
	// +optional
	Memory resource.Quantity `json:"memory,omitempty"`

	// GPU is the GPU resource quantity for the slot.
	// +optional
	GPU resource.Quantity `json:"gpu,omitempty"`
}

// StorageConfig defines the storage backend configuration with secure credential handling.
type StorageConfig struct {
	// Type is the storage backend type (e.g., "sqlite", "postgres", "mysql").
	// +kubebuilder:validation:Enum=sqlite;postgres;mysqlntType string `json:"type"`

	// SecretRef is a reference to a secret key containing the connection string or credentials.
	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`

	// Path is the filesystem path for file-based storage (e.g., SQLite).
	// +optional
	Path string `json:"path,omitempty"`
}

// FlameClusterSpec defines the desired state of FlameCluster
type FlameClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SessionManager defines the configuration for the session manager component.
	SessionManager SessionManagerSpec `json:"sessionManager,omitempty"`

	// ExecutorManager defines the configuration for the executor manager component.
	ExecutorManager ExecutorManagerSpec `json:"executorManager,omitempty"`

	// ObjectCache defines the configuration for the object cache component.
	ObjectCache ObjectCacheSpec `json:"objectCache,omitempty"`
}

// SessionManagerSpec defines the configuration for the session manager
type SessionManagerSpec struct {
	// Image is the container image to use.
	Image string `json:"image,omitempty"`

	// Resources defines the compute resource requirements.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Slot defines the resource slot for task scheduling.
	// +optional
	Slot *SlotSpec `json:"slot,omitempty"`

	// Policy defines the scheduling policy.
	// +kubebuilder:validation:Enum=priority;fifo;fair
	// +optional
	Policy string `json:"policy,omitempty"`

	// Storage defines the storage backend configuration.
	// +optional
	Storage *StorageConfig `json:"storage,omitempty"`
}


// ExecutorManagerSpec defines the configuration for the executor manager
type ExecutorManagerSpec struct {
	// Image is the container image to use.
	Image string `json:"image,omitempty"`

	// Replicas is the number of executor manager replicas.
	// +kubebuilder:validation:Minimum=0
	Replicas int32 `json:"replicas,omitempty"`

	// Resources defines the compute resource requirements.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Shim defines the executor shim type.
	// +kubebuilder:validation:Enum=host;docker;kubernetes
	// +optional
	Shim string `json:"shim,omitempty"`

	// MaxExecutors is the maximum number of executors per node.
	// +kubebuilder:validation:Minimum=1
	MaxExecutors int32 `json:"maxExecutors,omitempty"`
}

// ObjectCacheSpec defines the configuration for the object cache
type ObjectCacheSpec struct {
	// NetworkInterface is the network interface to use for cache communication.
	NetworkInterface string `json:"networkInterface,omitempty"`

	// VolumeSource defines the storage volume for the cache.
	// Supports EmptyDir, PVC, HostPath, etc. via standard Kubernetes VolumeSource.
	// +optional
	VolumeSource *corev1.VolumeSource `json:"volumeSource,omitempty"`
}

// FlameClusterStatus defines the observed state of FlameCluster
type FlameClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SessionManager status aggregation
	SessionManager SessionManagerStatus `json:"sessionManager,omitempty"`

	// ExecutorManager status aggregation
	ExecutorManager ExecutorManagerStatus `json:"executorManager,omitempty"`

	// Overall cluster state
	// +kubebuilder:validation:Enum=Pending;Running;Failed
	State string `json:"state,omitempty"`

	// ObservedGeneration reflects the generation of the most recently observed FlameClusterSpec.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Human-readable message for debugging
	Message string `json:"message,omitempty"`
}

// SessionManagerStatus defines the status of the session manager
type SessionManagerStatus struct {
	// Ready is the number of ready Session Manager instances.
	Ready int32 `json:"ready,omitempty"`

	// Endpoint is the service endpoint (e.g., "http://my-flame-session-manager:8080").
	Endpoint string `json:"endpoint,omitempty"`
}

// ExecutorManagerStatus defines the status of the executor manager
type ExecutorManagerStatus struct {
	// Replicas is the desired number of executor manager replicas.
	Replicas int32 `json:"replicas,omitempty"`

	// Ready is the number of ready executor manager pods.
	Ready int32 `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// FlameCluster is the Schema for the flameclusters API
type FlameCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FlameClusterSpec   `json:"spec,omitempty"`
	Status FlameClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FlameClusterList contains a list of FlameCluster
type FlameClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FlameCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FlameCluster{}, &FlameClusterList{})
}
