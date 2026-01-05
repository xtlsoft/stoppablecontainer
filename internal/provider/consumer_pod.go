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
	"fmt"
	"path/filepath"
	"strings"

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
	container := template.Container

	// Build the user's command
	userCommand := b.buildUserCommand(container)

	// The entrypoint script that sets up chroot and runs the command
	entrypointScript := b.buildEntrypointScript(userCommand, container.WorkingDir)

	// Build volume mounts
	volumeMounts := b.buildVolumeMounts(container.VolumeMounts)

	// Build volumes
	volumes := b.buildVolumes(template.Volumes, hostPath, hostPathType)

	// Environment variables
	env := container.Env
	env = append(env, corev1.EnvVar{
		Name:  "SC_ROOTFS",
		Value: RootfsMountPath,
	})

	// Build init containers
	initContainers := b.buildInitContainers(template.InitContainers)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.consumerPodName(),
			Namespace: b.sci.Namespace,
			Labels: map[string]string{
				LabelManagedBy: "stoppablecontainer",
				LabelInstance:  b.sci.Name,
				LabelRole:      "consumer",
			},
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
		Spec: corev1.PodSpec{
			// Must run on the same node as provider
			NodeName: b.nodeName,

			// Use a minimal base image that has chroot capabilities
			RestartPolicy: corev1.RestartPolicyAlways,

			// Service account from template
			ServiceAccountName: template.ServiceAccountName,

			// Affinity and tolerations
			Affinity:    template.Affinity,
			Tolerations: template.Tolerations,

			// Network settings
			HostNetwork: template.HostNetwork,
			DNSPolicy:   template.DNSPolicy,

			// Runtime class
			RuntimeClassName: template.RuntimeClassName,

			// Init containers
			InitContainers: initContainers,

			// Main container
			Containers: []corev1.Container{
				{
					Name:            ConsumerContainerName,
					Image:           "busybox:stable",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/bin/sh", "-c", entrypointScript},
					Env:             env,
					EnvFrom:         container.EnvFrom,
					Resources:       container.Resources,
					Ports:           container.Ports,
					VolumeMounts:    volumeMounts,
					SecurityContext: b.buildSecurityContext(container.SecurityContext),
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							Exec: &corev1.ExecAction{
								Command: []string{"test", "-d", RootfsMountPath + "/bin"},
							},
						},
						InitialDelaySeconds: 1,
						PeriodSeconds:       5,
					},
				},
			},

			Volumes: volumes,

			// Image pull secrets
			ImagePullSecrets: template.ImagePullSecrets,
		},
	}
}

func (b *ConsumerPodBuilder) consumerPodName() string {
	return fmt.Sprintf("%s-consumer", b.sci.Name)
}

func (b *ConsumerPodBuilder) buildUserCommand(container scv1alpha1.ContainerSpec) string {
	if len(container.Command) == 0 && len(container.Args) == 0 {
		return "exec /bin/sh"
	}

	var parts []string
	parts = append(parts, container.Command...)
	parts = append(parts, container.Args...)

	quoted := make([]string, len(parts))
	for i, p := range parts {
		quoted[i] = fmt.Sprintf("'%s'", strings.ReplaceAll(p, "'", "'\\''"))
	}

	return "exec " + strings.Join(quoted, " ")
}

func (b *ConsumerPodBuilder) buildEntrypointScript(userCommand, workingDir string) string {
	if workingDir == "" {
		workingDir = "/"
	}

	// The DaemonSet has already:
	// 1. Created the overlayfs mount at $ROOTFS
	// 2. Mounted /proc, /dev, /sys into the rootfs
	//
	// Consumer only needs to:
	// 1. Wait for rootfs to be ready (mounts propagated)
	// 2. Copy network configuration
	// 3. Mount service account secrets (if available)
	// 4. chroot into the rootfs and run the user command
	return fmt.Sprintf(`
set -e

ROOTFS="%s"
WORKDIR="%s"

echo "[consumer] Starting consumer container..."

# Wait for rootfs to be available (mounted by DaemonSet)
# Use a fast polling loop initially, then slow down
attempt=0
while [ $attempt -lt 120 ]; do
    if [ -d "$ROOTFS/bin" ] || [ -d "$ROOTFS/usr/bin" ]; then
        # Also check that proc is mounted (indicates DaemonSet has completed setup)
        if mountpoint -q "$ROOTFS/proc" 2>/dev/null; then
            break
        fi
    fi
    attempt=$((attempt + 1))
    if [ $attempt -le 10 ]; then
        # Fast polling for first 2 seconds
        sleep 0.2
    else
        echo "[consumer] Waiting for DaemonSet to complete rootfs setup... ($attempt/120)"
        sleep 1
    fi
done

if [ ! -d "$ROOTFS/bin" ] && [ ! -d "$ROOTFS/usr/bin" ]; then
    echo "[consumer] ERROR: Rootfs not ready at $ROOTFS"
    exit 1
fi

if ! mountpoint -q "$ROOTFS/proc" 2>/dev/null; then
    echo "[consumer] ERROR: Proc mount not ready at $ROOTFS/proc"
    exit 1
fi

echo "[consumer] Rootfs ready with mounts from DaemonSet"

# Copy network configuration
mkdir -p "$ROOTFS/etc"
if [ -f "/etc/resolv.conf" ] && [ ! -f "$ROOTFS/etc/resolv.conf" ]; then
    cp /etc/resolv.conf "$ROOTFS/etc/resolv.conf" 2>/dev/null || true
fi

if [ -f "/etc/hosts" ]; then
    cp /etc/hosts "$ROOTFS/etc/hosts" 2>/dev/null || true
fi

# Mount service account secrets if available
# This needs to be done by consumer as it has access to its own secrets
# Note: Some images use /run instead of /var/run (with /var/run being a symlink)
SA_PATH="/var/run/secrets/kubernetes.io/serviceaccount"
if [ -d "$SA_PATH" ]; then
    # Determine the actual path to use in rootfs
    # If /var/run is a symlink to /run, use /run directly
    if [ -L "$ROOTFS/var/run" ]; then
        ROOTFS_SA_PATH="$ROOTFS/run/secrets/kubernetes.io/serviceaccount"
    else
        ROOTFS_SA_PATH="$ROOTFS$SA_PATH"
    fi
    
    # Create parent directories
    mkdir -p "$(dirname "$ROOTFS_SA_PATH")" 2>/dev/null || true
    mkdir -p "$ROOTFS_SA_PATH" 2>/dev/null || true
    
    # Try mount --bind first, fallback to cp if not available
    if [ -d "$ROOTFS_SA_PATH" ]; then
        mount --bind "$SA_PATH" "$ROOTFS_SA_PATH" 2>/dev/null || \
            cp -r "$SA_PATH"/* "$ROOTFS_SA_PATH/" 2>/dev/null || true
    fi
fi

echo "[consumer] Setup complete, chrooting..."

cd "$ROOTFS"

exec chroot "$ROOTFS" /bin/sh -c "cd '%s' && %s"
`, RootfsMountPath, workingDir, workingDir, userCommand)
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

func (b *ConsumerPodBuilder) buildAnnotations(userAnnotations map[string]string) map[string]string {
	annotations := make(map[string]string)
	for k, v := range userAnnotations {
		annotations[k] = v
	}
	return annotations
}

func (b *ConsumerPodBuilder) buildInitContainers(userInitContainers []corev1.Container) []corev1.Container {
	initContainers := []corev1.Container{
		{
			Name:  ExecWrapperInitName,
			Image: ExecWrapperImage,
			Command: []string{
				"/bin/sh", "-c",
				fmt.Sprintf("cp /sc-exec %s/ && chmod +x %s/sc-exec", ExecWrapperBinPath, ExecWrapperBinPath),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      ExecWrapperVolumeName,
					MountPath: ExecWrapperBinPath,
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
		initContainers = append(initContainers, *userInit)
	}

	return initContainers
}
