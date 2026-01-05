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

// Package provider implements the pod builders for StoppableContainer.
// It creates and configures the provider and consumer pods that make up
// a running StoppableContainerInstance.
//
// Architecture (DaemonSet-based):
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                  mount-helper DaemonSet (privileged)        │
//	│  - Scans /proc for rootfs containers by pod UID             │
//	│  - Creates overlayfs mount from container's rootfs          │
//	│  - Mounts proc/dev/sys for consumer use                     │
//	└───────────────────────────────────┬─────────────────────────┘
//	                                    │ mount
//	                                    ▼
//	┌─────────────────────────────────────────────────────────────┐
//	│                     Provider Pod (non-privileged)           │
//	│  ┌─────────────────┐     ┌─────────────────────────────────┐│
//	│  │ rootfs container│     │ request container               ││
//	│  │ (user image)    │     │ writes request.json to hostPath ││
//	│  │ keeps running   │     │ waits for ready.json            ││
//	│  └─────────────────┘     └─────────────────────────────────┘│
//	│                               │                              │
//	│                               ▼                              │
//	│                    hostPath: /var/lib/sc/ns/name/           │
//	└───────────────────────────────┼─────────────────────────────┘
//	                                │ HostToContainer propagation
//	                                ▼
//	┌─────────────────────────────────────────────────────────────┐
//	│                     Consumer Pod (SYS_CHROOT only)          │
//	│  ┌─────────────────────────────────────────────────────────┐│
//	│  │ consumer container                                       ││
//	│  │ chroot into /propagated/rootfs, runs user command       ││
//	│  └─────────────────────────────────────────────────────────┘│
//	└─────────────────────────────────────────────────────────────┘
package provider

import (
	"fmt"
	"os"

	scv1alpha1 "github.com/xtlsoft/stoppablecontainer/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Container and volume names used in provider and consumer pods
const (
	// ProviderContainerName is the name of the main provider container (writes mount request)
	ProviderContainerName = "provider"
	// RootfsContainerName is the name of the sidecar that holds the user's rootfs
	RootfsContainerName = "rootfs"
	// ConsumerContainerName is the name of the main consumer container
	ConsumerContainerName = "consumer"
	// ExecWrapperInitName is the name of the init container that installs exec-wrapper
	ExecWrapperInitName = "exec-wrapper-init"
)

// Volume names and mount paths
const (
	// PropagatedVolumeName is the volume name for the shared rootfs directory
	PropagatedVolumeName = "sc-propagated"
	// ExecWrapperVolumeName is the volume name for the exec-wrapper binary
	ExecWrapperVolumeName = "sc-exec-wrapper"
	// BinOverlayVolumeName is the volume name for /bin overlay to intercept commands
	BinOverlayVolumeName = "sc-bin-overlay"
	// PauseVolumeName is the volume name for the pause binary injection
	PauseVolumeName = "sc-pause-bin"
	// PropagatedMountPath is where the hostPath is mounted in the provider pod
	PropagatedMountPath = "/propagated"
	// HostMountPath is where the hostPath is mounted in the rootfs container
	HostMountPath = "/hostmount"
	// RootfsMountPath is where the rootfs is mounted in the consumer pod
	RootfsMountPath = "/rootfs"
	// ExecWrapperBinPath is where the exec-wrapper binary is installed
	ExecWrapperBinPath = "/.sc-bin"
	// PauseBinPath is where the pause binary is injected into the rootfs container
	PauseBinPath = "/.sc-pause"
)

// Environment variable names for DaemonSet communication
const (
	// RootfsMarkerEnv is the environment variable that identifies rootfs containers
	RootfsMarkerEnv = "ROOTFS_MARKER"
	// PodUIDEnv is the environment variable containing the pod UID
	PodUIDEnv = "POD_UID"
)

// Default images used by the operator (can be overridden via environment variables)
var (
	// ExecWrapperImage is the image containing the exec-wrapper, pause, and provider binaries
	ExecWrapperImage = getEnvOrDefault("STOPPABLECONTAINER_EXEC_WRAPPER_IMAGE", "ghcr.io/xtlsoft/stoppablecontainer-exec:latest")
	// ExecWrapperPullPolicy is the image pull policy for the exec-wrapper image
	ExecWrapperPullPolicy = corev1.PullPolicy(getEnvOrDefault("STOPPABLECONTAINER_EXEC_WRAPPER_PULL_POLICY", string(corev1.PullIfNotPresent)))
)

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// Labels and annotations used by the operator
const (
	// LabelManagedBy identifies resources managed by this operator
	LabelManagedBy = "stoppablecontainer.xtlsoft.top/managed-by"
	// LabelInstance identifies which StoppableContainerInstance owns a pod
	LabelInstance = "stoppablecontainer.xtlsoft.top/instance"
	// LabelRole identifies the role of a pod (provider or consumer)
	LabelRole = "stoppablecontainer.xtlsoft.top/role"
)

// ProviderPodBuilder builds provider pods for StoppableContainerInstances.
// The provider pod consists of:
//   - A rootfs sidecar container running the user's image
//   - A provider container that mounts the rootfs to a shared hostPath
type ProviderPodBuilder struct {
	sci *scv1alpha1.StoppableContainerInstance
}

// NewProviderPodBuilder creates a new ProviderPodBuilder for the given instance.
func NewProviderPodBuilder(sci *scv1alpha1.StoppableContainerInstance) *ProviderPodBuilder {
	return &ProviderPodBuilder{sci: sci}
}

// Build creates the provider Pod specification.
// The provider pod uses a DaemonSet-based architecture where:
// - The rootfs container runs the user's image and marks itself with ROOTFS_MARKER
// - The provider container runs the sc-provider binary that coordinates with the DaemonSet
// - The mount-helper DaemonSet handles all privileged mount operations
func (b *ProviderPodBuilder) Build() *corev1.Pod {
	hostPath := GetHostPath(b.sci)
	hostPathType := corev1.HostPathDirectoryOrCreate

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-provider", b.sci.Name),
			Namespace: b.sci.Namespace,
			Labels: map[string]string{
				LabelManagedBy: "stoppablecontainer",
				LabelInstance:  b.sci.Name,
				LabelRole:      "provider",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         scv1alpha1.GroupVersion.String(),
					Kind:               "StoppableContainerInstance",
					Name:               b.sci.Name,
					UID:                b.sci.UID,
					Controller:         boolPtr(true),
					BlockOwnerDeletion: boolPtr(true),
				},
			},
		},
		Spec: corev1.PodSpec{
			ShareProcessNamespace: boolPtr(true),
			RestartPolicy:         corev1.RestartPolicyAlways,
			NodeSelector:          b.sci.Spec.Provider.NodeSelector,
			Tolerations:           b.sci.Spec.Provider.Tolerations,
			Containers: []corev1.Container{
				{
					Name:            ProviderContainerName,
					Image:           ExecWrapperImage,
					ImagePullPolicy: ExecWrapperPullPolicy,
					// Use the sc-provider binary instead of shell script
					Command: []string{"/sc-provider"},
					Env: []corev1.EnvVar{
						{
							Name: PodUIDEnv,
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.uid",
								},
							},
						},
						{
							Name: "POD_NAMESPACE",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.namespace",
								},
							},
						},
						{
							Name: "POD_NAME",
							ValueFrom: &corev1.EnvVarSource{
								FieldRef: &corev1.ObjectFieldSelector{
									FieldPath: "metadata.name",
								},
							},
						},
					},
					// No privileged required - DaemonSet handles all privileged operations
					Resources: b.providerResources(),
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:             PropagatedVolumeName,
							MountPath:        PropagatedMountPath,
							MountPropagation: mountPropagationPtr(corev1.MountPropagationHostToContainer),
						},
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"test", "-f", "/propagated/ready"},
							},
						},
						InitialDelaySeconds: 1,
						PeriodSeconds:       1,
						FailureThreshold:    120,
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"test", "-d", "/propagated/rootfs"},
							},
						},
						InitialDelaySeconds: 30,
						PeriodSeconds:       10,
					},
				},
				// Rootfs container runs the user's image with ROOTFS_MARKER for DaemonSet to find
				b.buildRootfsContainer(),
			},
			InitContainers: []corev1.Container{
				// Pause init container: copies the pause binary to a shared volume
				{
					Name:            "pause-init",
					Image:           ExecWrapperImage,
					ImagePullPolicy: ExecWrapperPullPolicy,
					Command:         []string{"cp", "/sc-pause", PauseBinPath + "/sc-pause"},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      PauseVolumeName,
							MountPath: PauseBinPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: PropagatedVolumeName,
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: hostPath,
							Type: &hostPathType,
						},
					},
				},
				{
					Name: PauseVolumeName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			ImagePullSecrets: b.sci.Spec.Template.ImagePullSecrets,
		},
	}
}

func (b *ProviderPodBuilder) providerResources() corev1.ResourceRequirements {
	if b.sci.Spec.Provider.Resources.Requests != nil || b.sci.Spec.Provider.Resources.Limits != nil {
		return b.sci.Spec.Provider.Resources
	}
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("16Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		},
	}
}

func (b *ProviderPodBuilder) minimalResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1m"),
			corev1.ResourceMemory: resource.MustParse("4Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("10m"),
			corev1.ResourceMemory: resource.MustParse("16Mi"),
		},
	}
}

// buildRootfsContainer creates the rootfs sidecar container that keeps the user's
// filesystem available for the DaemonSet to mount.
//
// This container uses an injected static pause binary that works with ANY image,
// including scratch and distroless images that have no shell. The pause binary
// is mounted from a shared volume populated by the pause-init container.
//
// The container is marked with ROOTFS_MARKER environment variable so the
// mount-helper DaemonSet can identify it and create the appropriate mounts.
func (b *ProviderPodBuilder) buildRootfsContainer() corev1.Container {
	container := corev1.Container{
		Name:  RootfsContainerName,
		Image: b.sci.Spec.Template.Container.Image,
		// Use the injected static pause binary as the command
		// This works for any image because:
		// 1. The binary is statically compiled (no library dependencies)
		// 2. It's injected via volume mount (no need for the image to contain it)
		Command:   []string{PauseBinPath + "/sc-pause"},
		Resources: b.minimalResources(),
		Env: []corev1.EnvVar{
			{
				// ROOTFS_MARKER identifies this container to the DaemonSet
				Name:  RootfsMarkerEnv,
				Value: "true",
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      PauseVolumeName,
				MountPath: PauseBinPath,
			},
			{
				// Mount hostPath with HostToContainer propagation
				// This allows the container to see mounts created by the DaemonSet
				Name:             PropagatedVolumeName,
				MountPath:        HostMountPath,
				MountPropagation: mountPropagationPtr(corev1.MountPropagationHostToContainer),
			},
		},
	}

	// If user specified ImagePullPolicy, use it
	if b.sci.Spec.Template.Container.ImagePullPolicy != "" {
		container.ImagePullPolicy = b.sci.Spec.Template.Container.ImagePullPolicy
	}

	return container
}
