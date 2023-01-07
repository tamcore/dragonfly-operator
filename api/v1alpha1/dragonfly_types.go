/*
Copyright 2023.

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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DragonflySpec defines the desired state of Dragonfly
type DragonflySpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	Image Image `json:"image,omitempty"`

	// +optional
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	// Size defines the number of Dragonfly instances
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +kubebuilder:validation:ExclusiveMaximum=false

	// +optional
	ReplicaCount int32 `json:"replicaCount,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Resources",xDescriptors="urn:alm:descriptor:com.tectonic.ui:resourceRequirements"
	// +optional
	Resources v1.ResourceRequirements `json:"resources,omitempty"`

	// +optional
	Affinity *v1.Affinity `json:"affinity,omitempty"`

	// +optional
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`

	// +optional
	SecurityContext *v1.SecurityContext `json:"securityContext,omitempty"`

	// +optional
	PodSecurityContext *v1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// +operator-sdk:csv:custom	resourcedefinitions:type=spec,displayName="ServiceAccount name",xDescriptors="urn:alm:descriptor:io.kubernetes:ServiceAccount"
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// +optional
	Containers []v1.Container `json:"containers,omitempty"`

	// +optional
	InitContainers []v1.Container `json:"initContainers,omitempty"`

	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`

	// +optional
	RedisPort string `json:"redisPort,omitempty"`

	// +optional
	MemcachePort string `json:"memcachePort,omitempty"`

	// +optional
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// +optional
	CommandOverride []string `json:"commandOverride,omitempty"`

	// +optional
	ExtraEnvs []v1.EnvVar `json:"extraEnvs,omitempty"`

	// +optional
	PodMonitor bool `json:"podMonitor,omitempty"`

	// +optional
	StatefulMode bool `json:"statefulMode,omitempty"`

	// +optional
	StatefulStorage v1.PersistentVolumeClaimSpec `json:"statefulStorage,omitempty"`

	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +optional
	Secrets []string `json:"secrets,omitempty"`

	// +optional
	ConfigMaps []string `json:"configMaps,omitempty"`

	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty"`

	// +optional
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`

	// // +optional
	// ReadinessProbe []v1.Probe `json:"readinessProbe,omitempty"`
	// // +optional
	// LivenessProbe []v1.Probe `json:"livenessProbe,omitempty"`
}

type Image struct {
	Repository string        `json:"repository,omitempty"`
	Tag        string        `json:"tag,omitempty"`
	PullPolicy v1.PullPolicy `json:"pullPolicy,omitempty"`
}

// DragonflyStatus defines the observed state of Dragonfly
type DragonflyStatus struct {
	// Represents the observations of a Dragonfly's current state.
	// Dragonfly.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// Dragonfly.status.conditions.status are one of True, False, Unknown.
	// Dragonfly.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// Dragonfly.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Dragonfly is the Schema for the dragonflies API
type Dragonfly struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DragonflySpec   `json:"spec,omitempty"`
	Status DragonflyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DragonflyList contains a list of Dragonfly
type DragonflyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Dragonfly `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Dragonfly{}, &DragonflyList{})
}
