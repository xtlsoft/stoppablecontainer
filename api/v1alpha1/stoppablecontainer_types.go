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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerSpec defines the container specification for StoppableContainer
type ContainerSpec struct {
	// Image is the container image to use
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// ImagePullPolicy defines the image pull policy
	// +kubebuilder:default=IfNotPresent
	// +optional
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

	// Command is the entrypoint array (replaces ENTRYPOINT)
	// +optional
	Command []string `json:"command,omitempty"`

	// Args are the arguments to the entrypoint (replaces CMD)
	// +optional
	Args []string `json:"args,omitempty"`

	// WorkingDir is the working directory inside the container
	// +optional
	WorkingDir string `json:"workingDir,omitempty"`

	// Env defines environment variables for the container
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom defines environment variable sources
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Resources defines the resource requirements
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// VolumeMounts defines the volume mounts for the container
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// Ports defines the ports to expose
	// +optional
	Ports []corev1.ContainerPort `json:"ports,omitempty"`

	// SecurityContext defines the security context for the container
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// PodTemplateSpec defines the pod template for the consumer pod
type PodTemplateSpec struct {
	// Metadata for the pod
	// +optional
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`

	// Container is the main container specification
	// +kubebuilder:validation:Required
	Container ContainerSpec `json:"container"`

	// InitContainers defines init containers for the consumer pod
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Volumes defines the volumes for the pod
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// ServiceAccountName is the service account for the pod
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// NodeSelector defines the node selector for scheduling
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Affinity defines the affinity rules for scheduling
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations defines the tolerations for scheduling
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// RuntimeClassName is the runtime class for the pod
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// HostNetwork determines if the pod uses host networking
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty"`

	// DNSPolicy defines the DNS policy for the pod
	// +optional
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`

	// ImagePullSecrets defines the image pull secrets
	// +optional
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// ProviderSpec defines the provider pod specification
type ProviderSpec struct {
	// Resources defines the resource requirements for the provider pod
	// These should be minimal as the provider only holds the filesystem
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector defines the node selector for the provider pod
	// The consumer pod will be scheduled on the same node
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for the provider pod
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// StoppableContainerSpec defines the desired state of StoppableContainer
type StoppableContainerSpec struct {
	// Running indicates whether the container should be running
	// Set to true to start the container, false to stop it
	// +kubebuilder:default=false
	Running bool `json:"running"`

	// Template defines the pod template for the consumer (workload) pod
	// +kubebuilder:validation:Required
	Template PodTemplateSpec `json:"template"`

	// Provider defines the provider pod specification
	// +optional
	Provider ProviderSpec `json:"provider,omitempty"`

	// HostPathPrefix is the prefix for the hostPath used for mount propagation
	// +kubebuilder:default="/var/lib/stoppablecontainer"
	// +optional
	HostPathPrefix string `json:"hostPathPrefix,omitempty"`
}

// Phase represents the current phase of the StoppableContainer
// +kubebuilder:validation:Enum=Pending;ProviderReady;Running;Stopped;Failed
type Phase string

const (
	// PhasePending indicates the StoppableContainer is being set up
	PhasePending Phase = "Pending"

	// PhaseProviderReady indicates the provider pod is ready
	PhaseProviderReady Phase = "ProviderReady"

	// PhaseRunning indicates the container is running
	PhaseRunning Phase = "Running"

	// PhaseStopped indicates the container is stopped but rootfs is preserved
	PhaseStopped Phase = "Stopped"

	// PhaseFailed indicates the container has failed
	PhaseFailed Phase = "Failed"
)

// StoppableContainerStatus defines the observed state of StoppableContainer.
type StoppableContainerStatus struct {
	// Phase represents the current phase of the StoppableContainer
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// InstanceName is the name of the associated StoppableContainerInstance
	// +optional
	InstanceName string `json:"instanceName,omitempty"`

	// ProviderPodName is the name of the provider pod
	// +optional
	ProviderPodName string `json:"providerPodName,omitempty"`

	// ConsumerPodName is the name of the consumer pod
	// +optional
	ConsumerPodName string `json:"consumerPodName,omitempty"`

	// HostPath is the path on the host where the rootfs is exposed
	// +optional
	HostPath string `json:"hostPath,omitempty"`

	// NodeName is the node where the provider pod is running
	// +optional
	NodeName string `json:"nodeName,omitempty"`

	// Conditions represent the current state of the StoppableContainer resource
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Running",type="boolean",JSONPath=".spec.running",description="Whether the container should be running"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Current phase"
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".status.nodeName",description="Node where provider is running"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// StoppableContainer is the Schema for the stoppablecontainers API
// It represents a container that can be stopped and started while preserving its ephemeral filesystem
type StoppableContainer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoppableContainerSpec   `json:"spec,omitempty"`
	Status StoppableContainerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StoppableContainerList contains a list of StoppableContainer
type StoppableContainerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StoppableContainer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StoppableContainer{}, &StoppableContainerList{})
}
