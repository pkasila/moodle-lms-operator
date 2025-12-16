/*
Copyright 2025.

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

// MoodleTenantSpec defines the desired state of MoodleTenant
type MoodleTenantSpec struct {
	// Hostname for the Moodle instance.
	// +kubebuilder:validation:Required
	Hostname string `json:"hostname"`

	// Image for the Moodle container.
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Resources for the Moodle container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// HPA configuration for the Moodle instance.
	// +optional
	HPA HPASpec `json:"hpa,omitempty"`

	// Storage configuration for the Moodle instance.
	// +kubebuilder:validation:Required
	Storage StorageSpec `json:"storage"`

	// DatabaseRef is a reference to the database to be used for this Moodle instance.
	// +kubebuilder:validation:Required
	DatabaseRef DatabaseRefSpec `json:"databaseRef"`

	// PHPSettings for the Moodle instance.
	// +optional
	PHPSettings PHPSettingsSpec `json:"phpSettings,omitempty"`

	// Memcached configuration for the Moodle instance.
	// +optional
	Memcached MemcachedSpec `json:"memcached,omitempty"`
}

// HPASpec defines the HPA configuration for a MoodleTenant.
type HPASpec struct {
	// Enabled enables or disables HPA.
	// +kubebuilder:default:=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// MinReplicas is the minimum number of replicas.
	// +kubebuilder:default:=2
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas.
	// +kubebuilder:default:=10
	// +kubebuilder:validation:Required
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetCPU is the target CPU utilization percentage.
	// +kubebuilder:default:=75
	// +optional
	TargetCPU *int32 `json:"targetCPU,omitempty"`
}

// StorageSpec defines the storage configuration for a MoodleTenant.
type StorageSpec struct {
	// Size of the persistent volume.
	// +kubebuilder:validation:Required
	Size resource.Quantity `json:"size"`

	// StorageClass for the persistent volume.
	// +kubebuilder:default:="csi-cephfs-sc"
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
}

// DatabaseRefSpec defines the database reference for a MoodleTenant.
type DatabaseRefSpec struct {
	// Host of the database.
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// AdminSecret is the name of the secret containing the admin credentials for the database.
	// +kubebuilder:validation:Required
	AdminSecret string `json:"adminSecret"`

	// Name of the database.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// User for the database.
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// Password for the database.
	// +kubebuilder:validation:Required
	Password string `json:"password"`
}

// PHPSettingsSpec defines the PHP settings for a MoodleTenant.
type PHPSettingsSpec struct {
	// MaxExecutionTime for PHP scripts.
	// +kubebuilder:default:=60
	// +optional
	MaxExecutionTime int `json:"maxExecutionTime,omitempty"`

	// MemoryLimit for PHP scripts.
	// +kubebuilder:default:="512M"
	// +optional
	MemoryLimit string `json:"memoryLimit,omitempty"`
}

// MemcachedSpec defines the Memcached configuration for a MoodleTenant.
type MemcachedSpec struct {
	// MemoryMB is the memory limit for Memcached in megabytes.
	// +kubebuilder:default:=128
	// +optional
	MemoryMB int `json:"memoryMB,omitempty"`
}

// MoodleTenantStatus defines the observed state of MoodleTenant
type MoodleTenantStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MoodleTenant is the Schema for the moodletenants API
type MoodleTenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MoodleTenantSpec   `json:"spec,omitempty"`
	Status MoodleTenantStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MoodleTenantList contains a list of MoodleTenant
type MoodleTenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MoodleTenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MoodleTenant{}, &MoodleTenantList{})
}
