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

package provider

import (
	"path/filepath"

	scv1alpha1 "github.com/xtlsoft/stoppablecontainer/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConsumerPodBuilder builds consumer pods
type ConsumerPodBuilder struct {
	sci      *scv1alpha1.StoppableContainerInstance
	nodeName string
}

// NewConsumerPodBuilder creates a new ConsumerPodBuilder
func NewConsumerPodBuilder(sci *scv1alpha1.StoppableContainerInstance, nodeName string) *ConsumerPodBuilder {
	return &ConsumerPodBuilder{
		sci:      sci,
		nodeName: nodeName,
	}
}

// Build creates the consumer pod spec
func (b *ConsumerPodBuilder) Build() *corev1.Pod {
	hostPath := filepath.Join(GetHostPath(b.sci), "rootfs")
	hostPathType := corev1.HostPathDirectory

	template := b.sci.Spec.Template
	podSpec := template.Spec.DeepCopy()

	// Validate we have at least one container
	if len(podSpec.Containers) == 0 {
		// Fallback: create a minimal container if none specified
		podSpec.Containers = []corev1.Container{{
			Name:  ConsumerContainerName,
			Image: "busybox:stable",
		}}
	}

	// Get the first container as the main workload container
	mainContainer := &podSpec.Containers[0]

	// Build the user's command as args for sc-exec --entrypoint
	userCommand := b.buildUserCommand(mainContainer)

	// Build volume mounts for the main container
	mainContainer.VolumeMounts = b.buildVolumeMounts(mainContainer.VolumeMounts)

	// Build volumes
	podSpec.Volumes = b.buildVolumes(podSpec.Volumes, hostPath, hostPathType)

	// Add SC_ROOTFS environment variable
	mainContainer.Env = append(mainContainer.Env, corev1.EnvVar{
		Name:  "SC_ROOTFS",
		Value: RootfsMountPath,
	})

	// Build init containers (prepend our init container)
	podSpec.InitContainers = b.buildInitContainers(podSpec.InitContainers)

	// Override container settings for exec-wrapper
	mainContainer.Name = ConsumerContainerName
	mainContainer.Image = ExecWrapperImage
	mainContainer.ImagePullPolicy = ExecWrapperPullPolicy
	mainContainer.Command = b.buildEntrypointCommand(userCommand, mainContainer.WorkingDir)
	mainContainer.Args = nil // Args are incorporated into Command
	mainContainer.SecurityContext = b.buildSecurityContext(mainContainer.SecurityContext)
	mainContainer.ReadinessProbe = &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			Exec: &corev1.ExecAction{
				Command: []string{ExecWrapperBinPath + "/sc-exec", "--ready"},
			},
		},
		InitialDelaySeconds: 1,
		PeriodSeconds:       5,
	}

	// Override pod-level settings that must be controlled by the controller
	podSpec.NodeName = b.nodeName
	podSpec.RestartPolicy = corev1.RestartPolicyAlways

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        b.consumerPodName(),
			Namespace:   b.sci.Namespace,
			Labels:      b.buildLabels(template.Metadata.Labels),
			Annotations: b.buildAnnotations(template.Metadata.Annotations),
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
		Spec: *podSpec,
	}
}

func (b *ConsumerPodBuilder) consumerPodName() string {
	// Consumer pod uses the same name as the SCI for a seamless user experience
	// Users can use "kubectl exec <name>" directly without knowing about the -consumer suffix
	return b.sci.Name
}

func (b *ConsumerPodBuilder) buildUserCommand(container *corev1.Container) []string {
	if len(container.Command) == 0 && len(container.Args) == 0 {
		return []string{"/bin/sh"}
	}

	var parts []string
	parts = append(parts, container.Command...)
	parts = append(parts, container.Args...)

	return parts
}

// buildEntrypointCommand creates the command for the consumer container
// It uses sc-exec --entrypoint which handles waiting for rootfs, setting up
// network config, and chrooting into the rootfs to run the user command
func (b *ConsumerPodBuilder) buildEntrypointCommand(userCommand []string, workingDir string) []string {
	if workingDir == "" {
		workingDir = "/"
	}

	// Command format: sc-exec --entrypoint <workdir> <command...>
	cmd := []string{"/sc-exec", "--entrypoint", workingDir}
	cmd = append(cmd, userCommand...)
	return cmd
}

func (b *ConsumerPodBuilder) buildVolumeMounts(userMounts []corev1.VolumeMount) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      PropagatedVolumeName,
			MountPath: RootfsMountPath,
			// HostToContainer propagation allows the consumer to see mounts
			// created by the DaemonSet on the host path
			MountPropagation: mountPropagationPtr(corev1.MountPropagationHostToContainer),
		},
		{
			Name:      ExecWrapperVolumeName,
			MountPath: ExecWrapperBinPath,
		},
		{
			// Mount /bin as an overlay so we can intercept all commands
			Name:      BinOverlayVolumeName,
			MountPath: "/bin",
		},
	}

	for _, m := range userMounts {
		userMount := m.DeepCopy()
		userMount.Name = "user-" + m.Name
		mounts = append(mounts, *userMount)

		rootfsMount := m.DeepCopy()
		rootfsMount.Name = "user-" + m.Name + "-rootfs"
		rootfsMount.MountPath = RootfsMountPath + m.MountPath
		mounts = append(mounts, *rootfsMount)
	}

	return mounts
}

func (b *ConsumerPodBuilder) buildVolumes(userVolumes []corev1.Volume, hostPath string, hostPathType corev1.HostPathType) []corev1.Volume {
	volumes := []corev1.Volume{
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
			Name: ExecWrapperVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: BinOverlayVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	for _, v := range userVolumes {
		userVol := v.DeepCopy()
		userVol.Name = "user-" + v.Name
		volumes = append(volumes, *userVol)

		rootfsVol := v.DeepCopy()
		rootfsVol.Name = "user-" + v.Name + "-rootfs"
		volumes = append(volumes, *rootfsVol)
	}

	return volumes
}

func (b *ConsumerPodBuilder) buildSecurityContext(userCtx *corev1.SecurityContext) *corev1.SecurityContext {
	// Consumer container only needs CAP_SYS_CHROOT for chroot operations.
	// Mount operations (bind mount /proc, /dev, /sys) are performed by the DaemonSet,
	// which propagates mounts to the consumer container via HostToContainer propagation.
	//
	// This is a significant security improvement - we no longer require CAP_SYS_ADMIN
	// which is a very powerful capability.
	requiredCaps := []corev1.Capability{
		"SYS_CHROOT",
	}

	ctx := &corev1.SecurityContext{
		Capabilities: &corev1.Capabilities{
			Add: requiredCaps,
		},
	}

	if userCtx != nil {
		if userCtx.RunAsGroup != nil {
			ctx.RunAsGroup = userCtx.RunAsGroup
		}
		// Merge user-requested capabilities with required ones
		if userCtx.Capabilities != nil {
			for _, cap := range userCtx.Capabilities.Add {
				// Avoid duplicates
				found := false
				for _, existing := range ctx.Capabilities.Add {
					if existing == cap {
						found = true
						break
					}
				}
				if !found {
					ctx.Capabilities.Add = append(ctx.Capabilities.Add, cap)
				}
			}
			// Note: We don't apply Drop here as it could break functionality
		}
	}

	return ctx
}

func (b *ConsumerPodBuilder) buildLabels(userLabels map[string]string) map[string]string {
	labels := make(map[string]string)
	// Copy user labels first
	for k, v := range userLabels {
		labels[k] = v
	}
	// System labels override user labels to ensure correct operation
	labels[LabelManagedBy] = "stoppablecontainer"
	labels[LabelInstance] = b.sci.Name
	labels[LabelRole] = "consumer"
	return labels
}

func (b *ConsumerPodBuilder) buildAnnotations(userAnnotations map[string]string) map[string]string {
	annotations := make(map[string]string)
	for k, v := range userAnnotations {
		annotations[k] = v
	}
	return annotations
}

func (b *ConsumerPodBuilder) buildInitContainers(userInitContainers []corev1.Container) []corev1.Container {
	// Use sc-exec --init to set up the bin overlay
	// This copies sc-exec to /.sc-bin and creates symlinks for common commands
	initContainers := []corev1.Container{
		{
			Name:            ExecWrapperInitName,
			Image:           ExecWrapperImage,
			ImagePullPolicy: ExecWrapperPullPolicy,
			Command:         []string{"/sc-exec", "--init", "/sc-bin-overlay"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      ExecWrapperVolumeName,
					MountPath: ExecWrapperBinPath,
				},
				{
					Name:      BinOverlayVolumeName,
					MountPath: "/sc-bin-overlay",
				},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("10m"),
					corev1.ResourceMemory: resource.MustParse("16Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("64Mi"),
				},
			},
		},
	}

	for _, c := range userInitContainers {
		userInit := c.DeepCopy()
		userInit.Name = "user-" + c.Name
		// Update volumeMount names to use user- prefix to match renamed volumes
		for i := range userInit.VolumeMounts {
			userInit.VolumeMounts[i].Name = "user-" + userInit.VolumeMounts[i].Name
		}
		initContainers = append(initContainers, *userInit)
	}

	return initContainers
}
