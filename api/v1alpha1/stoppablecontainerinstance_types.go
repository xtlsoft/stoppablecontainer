/*
Copyright 2026.

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstancePhase represents the current phase of the StoppableContainerInstance
// +kubebuilder:validation:Enum=Pending;ProviderStarting;ProviderReady;ConsumerStarting;Running;Stopping;Stopped;Failed
type InstancePhase string

const (
	// InstancePhasePending indicates the instance is being created
	InstancePhasePending InstancePhase = "Pending"

	// InstancePhaseProviderStarting indicates the provider pod is starting
	InstancePhaseProviderStarting InstancePhase = "ProviderStarting"

	// InstancePhaseProviderReady indicates the provider pod is ready
	InstancePhaseProviderReady InstancePhase = "ProviderReady"

	// InstancePhaseConsumerStarting indicates the consumer pod is starting
	InstancePhaseConsumerStarting InstancePhase = "ConsumerStarting"

	// InstancePhaseRunning indicates both pods are running
	InstancePhaseRunning InstancePhase = "Running"

	// InstancePhaseStopping indicates the consumer is being stopped
	InstancePhaseStopping InstancePhase = "Stopping"

	// InstancePhaseStopped indicates the consumer is stopped but provider is running
	InstancePhaseStopped InstancePhase = "Stopped"

	// InstancePhaseFailed indicates a failure occurred
	InstancePhaseFailed InstancePhase = "Failed"
)

// StoppableContainerInstanceSpec defines the desired state of StoppableContainerInstance
type StoppableContainerInstanceSpec struct {
	// StoppableContainerName is the name of the parent StoppableContainer
	// +kubebuilder:validation:Required
	StoppableContainerName string `json:"stoppableContainerName"`

	// Running indicates whether the consumer should be running
	// +kubebuilder:default=true
	Running bool `json:"running"`

	// Template is copied from the parent StoppableContainer at creation time
	// +kubebuilder:validation:Required
	Template PodTemplateSpec `json:"template"`

	// Provider is copied from the parent StoppableContainer
	// +optional
	Provider ProviderSpec `json:"provider,omitempty"`

	// HostPathPrefix is the prefix for the hostPath
	// +kubebuilder:default="/var/lib/stoppablecontainer"
	// +optional
	HostPathPrefix string `json:"hostPathPrefix,omitempty"`
}

// StoppableContainerInstanceStatus defines the observed state of StoppableContainerInstance.
type StoppableContainerInstanceStatus struct {
	// Phase represents the current phase
	// +optional
	Phase InstancePhase `json:"phase,omitempty"`

	// ProviderPodName is the name of the provider pod
	// +optional
	ProviderPodName string `json:"providerPodName,omitempty"`

	// ProviderPodUID is the UID of the provider pod
	// +optional
	ProviderPodUID string `json:"providerPodUID,omitempty"`

	// ConsumerPodName is the name of the consumer pod
	// +optional
	ConsumerPodName string `json:"consumerPodName,omitempty"`

	// ConsumerPodUID is the UID of the consumer pod
	// +optional
	ConsumerPodUID string `json:"consumerPodUID,omitempty"`

	// HostPath is the full path on the host where rootfs is exposed
	// +optional
	HostPath string `json:"hostPath,omitempty"`

	// NodeName is the node where the pods are running
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// RootfsPID is the PID of the process whose rootfs is being used
	// +optional
	RootfsPID int32 `json:"rootfsPID,omitempty"`

	// Message provides additional information about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// Conditions represent the current state
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SC",type="string",JSONPath=".spec.stoppableContainerName",description="Parent StoppableContainer"
// +kubebuilder:printcolumn:name="Running",type="boolean",JSONPath=".spec.running",description="Whether consumer should be running"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Current phase"
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".status.nodeName",description="Node name"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// StoppableContainerInstance is the Schema for the stoppablecontainerinstances API
// It represents a running instance of a StoppableContainer
type StoppableContainerInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoppableContainerInstanceSpec   `json:"spec,omitempty"`
	Status StoppableContainerInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StoppableContainerInstanceList contains a list of StoppableContainerInstance
type StoppableContainerInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StoppableContainerInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StoppableContainerInstance{}, &StoppableContainerInstanceList{})
}
