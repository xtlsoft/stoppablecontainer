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
	"testing"

	corev1 "k8s.io/api/core/v1"
)

const (
	testAppName = "my-app"
)

func TestNewConsumerPodBuilder(t *testing.T) {
	sci := createTestSCI("test", "default", "busybox:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	if builder == nil {
		t.Fatal("NewConsumerPodBuilder returned nil")
	}
	if builder.sci != sci {
		t.Error("Builder sci does not match input")
	}
	if builder.nodeName != "node-1" {
		t.Errorf("Builder nodeName = %q, want %q", builder.nodeName, "node-1")
	}
}

func TestConsumerPodBuilder_Build(t *testing.T) {
	sci := createTestSCI(testAppName, "production", "nginx:latest")
	builder := NewConsumerPodBuilder(sci, "worker-node-1")
	pod := builder.Build()

	// Check pod name - consumer pod uses the same name as SCI for seamless experience
	if pod.Name != testAppName {
		t.Errorf("Pod name = %q, want %q", pod.Name, testAppName)
	}

	// Check namespace
	if pod.Namespace != "production" {
		t.Errorf("Pod namespace = %q, want %q", pod.Namespace, "production")
	}

	// Check node name is set
	if pod.Spec.NodeName != "worker-node-1" {
		t.Errorf("Pod NodeName = %q, want %q", pod.Spec.NodeName, "worker-node-1")
	}

	// Check labels
	if pod.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("Label %s = %q, want %q", LabelManagedBy, pod.Labels[LabelManagedBy], ManagedByValue)
	}
	if pod.Labels[LabelInstance] != testAppName {
		t.Errorf("Label %s = %q, want %q", LabelInstance, pod.Labels[LabelInstance], testAppName)
	}
	if pod.Labels[LabelRole] != "consumer" {
		t.Errorf("Label %s = %q, want %q", LabelRole, pod.Labels[LabelRole], "consumer")
	}

	// Check containers
	if len(pod.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(pod.Spec.Containers))
	}

	container := pod.Spec.Containers[0]
	if container.Name != ConsumerContainerName {
		t.Errorf("Container name = %q, want %q", container.Name, ConsumerContainerName)
	}

	// Check security context uses capabilities, not privileged
	if container.SecurityContext == nil {
		t.Fatal("SecurityContext is nil")
	}
	if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
		t.Error("Consumer container should NOT be privileged")
	}
	if container.SecurityContext.Capabilities == nil {
		t.Fatal("Capabilities are nil")
	}

	// Check required capabilities - only SYS_CHROOT is needed now (DaemonSet handles mounts)
	caps := container.SecurityContext.Capabilities.Add
	hasSysAdmin := false
	hasSysChroot := false
	for _, cap := range caps {
		if cap == "SYS_ADMIN" {
			hasSysAdmin = true
		}
		if cap == "SYS_CHROOT" {
			hasSysChroot = true
		}
	}
	if hasSysAdmin {
		t.Error("Consumer should NOT have SYS_ADMIN capability - DaemonSet handles mount operations")
	}
	if !hasSysChroot {
		t.Error("Consumer should have SYS_CHROOT capability")
	}

	// Check init containers
	if len(pod.Spec.InitContainers) != 1 {
		t.Fatalf("Expected 1 init container, got %d", len(pod.Spec.InitContainers))
	}
	if pod.Spec.InitContainers[0].Name != ExecWrapperInitName {
		t.Errorf("Init container name = %q, want %q", pod.Spec.InitContainers[0].Name, ExecWrapperInitName)
	}
}

func TestConsumerPodBuilder_BuildSecurityContext(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	tests := []struct {
		name           string
		userCtx        *corev1.SecurityContext
		expectSysAdmin bool // Should always be false now - DaemonSet handles mounts
		expectChroot   bool
		expectExtraCap string
	}{
		{
			name:           "nil user context",
			userCtx:        nil,
			expectSysAdmin: false, // No longer needed - DaemonSet handles mounts
			expectChroot:   true,
		},
		{
			name: "user context with group",
			userCtx: &corev1.SecurityContext{
				RunAsGroup: int64Ptr(1000),
			},
			expectSysAdmin: false,
			expectChroot:   true,
		},
		{
			name: "user context with extra capability",
			userCtx: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
			},
			expectSysAdmin: false,
			expectChroot:   true,
			expectExtraCap: "NET_ADMIN",
		},
		{
			name: "user context with SYS_ADMIN - should still be added if user requests",
			userCtx: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"SYS_ADMIN", "NET_RAW"},
				},
			},
			expectSysAdmin: true, // User explicitly requested it
			expectChroot:   true,
			expectExtraCap: "NET_RAW",
		},
		{
			name: "user context with SYS_CHROOT - should not duplicate",
			userCtx: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"SYS_CHROOT", "NET_ADMIN"},
				},
			},
			expectSysAdmin: false,
			expectChroot:   true, // Already required, should not duplicate
			expectExtraCap: "NET_ADMIN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := builder.buildSecurityContext(tt.userCtx)

			if ctx == nil {
				t.Fatal("buildSecurityContext returned nil")
			}
			if ctx.Capabilities == nil {
				t.Fatal("Capabilities is nil")
			}

			caps := ctx.Capabilities.Add
			hasSysAdmin := false
			hasChroot := false
			hasExtra := tt.expectExtraCap == ""

			for _, cap := range caps {
				if cap == "SYS_ADMIN" {
					hasSysAdmin = true
				}
				if cap == "SYS_CHROOT" {
					hasChroot = true
				}
				if tt.expectExtraCap != "" && string(cap) == tt.expectExtraCap {
					hasExtra = true
				}
			}

			if tt.expectSysAdmin != hasSysAdmin {
				if tt.expectSysAdmin {
					t.Error("Expected SYS_ADMIN capability")
				} else {
					t.Error("Did not expect SYS_ADMIN capability (DaemonSet handles mounts)")
				}
			}
			if tt.expectChroot && !hasChroot {
				t.Error("Expected SYS_CHROOT capability")
			}
			if !hasExtra {
				t.Errorf("Expected %s capability", tt.expectExtraCap)
			}

			// Check no duplicates
			capCount := make(map[corev1.Capability]int)
			for _, cap := range caps {
				capCount[cap]++
				if capCount[cap] > 1 {
					t.Errorf("Duplicate capability: %s", cap)
				}
			}
		})
	}
}

func TestConsumerPodBuilder_BuildUserCommand(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	tests := []struct {
		name      string
		container corev1.Container
		expected  []string
	}{
		{
			name:      "no command or args",
			container: corev1.Container{},
			expected:  []string{"/bin/sh"},
		},
		{
			name: "simple command",
			container: corev1.Container{
				Command: []string{"/bin/bash"},
			},
			expected: []string{"/bin/bash"},
		},
		{
			name: "command with args",
			container: corev1.Container{
				Command: []string{"/bin/sh", "-c"},
				Args:    []string{"echo hello"},
			},
			expected: []string{"/bin/sh", "-c", "echo hello"},
		},
		{
			name: "command with quotes",
			container: corev1.Container{
				Command: []string{"/bin/sh", "-c", "echo 'hello world'"},
			},
			expected: []string{"/bin/sh", "-c", "echo 'hello world'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildUserCommand(&tt.container)
			if len(result) != len(tt.expected) {
				t.Errorf("buildUserCommand() = %v, want %v", result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("buildUserCommand()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestConsumerPodBuilder_BuildEntrypointCommand(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	tests := []struct {
		name       string
		userCmd    []string
		workingDir string
		expected   []string
	}{
		{
			name:       "simple command with workdir",
			userCmd:    []string{"/bin/sh"},
			workingDir: "/app",
			expected:   []string{"/sc-exec", "--entrypoint", "/app", "/bin/sh"},
		},
		{
			name:       "command with args",
			userCmd:    []string{"/bin/sh", "-c", "echo hello"},
			workingDir: "/",
			expected:   []string{"/sc-exec", "--entrypoint", "/", "/bin/sh", "-c", "echo hello"},
		},
		{
			name:       "empty workdir defaults to /",
			userCmd:    []string{"/bin/bash"},
			workingDir: "",
			expected:   []string{"/sc-exec", "--entrypoint", "/", "/bin/bash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildEntrypointCommand(tt.userCmd, tt.workingDir)
			if len(result) != len(tt.expected) {
				t.Errorf("buildEntrypointCommand() = %v, want %v", result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("buildEntrypointCommand()[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestConsumerPodBuilder_BuildVolumeMounts(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	userMounts := []corev1.VolumeMount{
		{
			Name:      "data",
			MountPath: "/data",
		},
		{
			Name:      "config",
			MountPath: "/etc/config",
			ReadOnly:  true,
		},
	}

	mounts := builder.buildVolumeMounts(userMounts)

	// Should have base mounts + 2 user mounts * 2 (original + rootfs)
	// Base: PropagatedVolume, ExecWrapperVolume, BinOverlayVolume = 3
	// User: 2 * 2 = 4
	// Total = 7
	if len(mounts) != 7 {
		t.Errorf("Expected 7 mounts, got %d", len(mounts))
	}

	// Check base mounts exist
	foundPropagated := false
	foundExecWrapper := false
	foundBinOverlay := false
	for _, m := range mounts {
		if m.Name == PropagatedVolumeName && m.MountPath == RootfsMountPath {
			foundPropagated = true
		}
		if m.Name == ExecWrapperVolumeName && m.MountPath == ExecWrapperBinPath {
			foundExecWrapper = true
		}
		if m.Name == BinOverlayVolumeName && m.MountPath == "/bin" {
			foundBinOverlay = true
		}
	}
	if !foundPropagated {
		t.Error("Propagated volume mount not found")
	}
	if !foundExecWrapper {
		t.Error("Exec wrapper volume mount not found")
	}
	if !foundBinOverlay {
		t.Error("Bin overlay volume mount not found")
	}

	// Check user mounts have both original and rootfs versions
	foundDataOriginal := false
	foundDataRootfs := false
	for _, m := range mounts {
		if m.Name == "user-data" && m.MountPath == "/data" {
			foundDataOriginal = true
		}
		if m.Name == "user-data-rootfs" && m.MountPath == "/rootfs/data" {
			foundDataRootfs = true
		}
	}
	if !foundDataOriginal {
		t.Error("User data original mount not found")
	}
	if !foundDataRootfs {
		t.Error("User data rootfs mount not found")
	}
}

func TestConsumerPodBuilder_BuildVolumes(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	userVolumes := []corev1.Volume{
		{
			Name: "data",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "my-config",
					},
				},
			},
		},
	}

	hostPathType := corev1.HostPathDirectory
	volumes := builder.buildVolumes(userVolumes, "/var/lib/test", hostPathType)

	// Should have base volumes + 2 user volumes * 2 (original + rootfs)
	// Base: PropagatedVolume, ExecWrapperVolume, BinOverlayVolume = 3
	// User: 2 * 2 = 4
	// Total = 7
	if len(volumes) != 7 {
		t.Errorf("Expected 7 volumes, got %d", len(volumes))
	}

	// Check base volumes exist
	foundPropagated := false
	foundExecWrapper := false
	foundBinOverlay := false
	for _, v := range volumes {
		if v.Name == PropagatedVolumeName {
			foundPropagated = true
			if v.HostPath == nil || v.HostPath.Path != "/var/lib/test" {
				t.Error("Propagated volume has wrong hostPath")
			}
		}
		if v.Name == ExecWrapperVolumeName {
			foundExecWrapper = true
			if v.EmptyDir == nil {
				t.Error("ExecWrapper volume should be emptyDir")
			}
		}
		if v.Name == BinOverlayVolumeName {
			foundBinOverlay = true
			if v.EmptyDir == nil {
				t.Error("BinOverlay volume should be emptyDir")
			}
		}
	}
	if !foundPropagated {
		t.Error("Propagated volume not found")
	}
	if !foundExecWrapper {
		t.Error("Exec wrapper volume not found")
	}
	if !foundBinOverlay {
		t.Error("Bin overlay volume not found")
	}

	// Check user volumes have both original and rootfs versions
	foundDataOriginal := false
	foundDataRootfs := false
	for _, v := range volumes {
		if v.Name == "user-data" {
			foundDataOriginal = true
		}
		if v.Name == "user-data-rootfs" {
			foundDataRootfs = true
		}
	}
	if !foundDataOriginal {
		t.Error("User data volume not found")
	}
	if !foundDataRootfs {
		t.Error("User data rootfs volume not found")
	}
}

func TestConsumerPodBuilder_BuildAnnotations(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	tests := []struct {
		name        string
		annotations map[string]string
		expected    int
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    0,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    0,
		},
		{
			name: "with annotations",
			annotations: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := builder.buildAnnotations(tt.annotations)
			if len(result) != tt.expected {
				t.Errorf("Expected %d annotations, got %d", tt.expected, len(result))
			}
			for k, v := range tt.annotations {
				if result[k] != v {
					t.Errorf("Annotation %s: expected %s, got %s", k, v, result[k])
				}
			}
		})
	}
}

func TestConsumerPodBuilder_BuildInitContainers(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	userInitContainers := []corev1.Container{
		{
			Name:  "init-db",
			Image: "busybox:stable",
			Command: []string{
				"/bin/sh", "-c", "echo init",
			},
		},
	}

	initContainers := builder.buildInitContainers(userInitContainers)

	// Should have exec-wrapper init + 1 user init = 2
	if len(initContainers) != 2 {
		t.Errorf("Expected 2 init containers, got %d", len(initContainers))
	}

	// First should be exec-wrapper
	if initContainers[0].Name != ExecWrapperInitName {
		t.Errorf("First init container should be %s, got %s", ExecWrapperInitName, initContainers[0].Name)
	}

	// Second should be user init with prefixed name
	if initContainers[1].Name != "user-init-db" {
		t.Errorf("User init container should be prefixed, got %s", initContainers[1].Name)
	}
}

func TestConsumerPodBuilder_BuildInitContainers_VolumeNameMapping(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewConsumerPodBuilder(sci, "node-1")

	userInitContainers := []corev1.Container{
		{
			Name:    "my-init",
			Image:   "busybox:stable",
			Command: []string{"/bin/sh", "-c", "cp /bin/echo /shared/"},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "shared-bin",
					MountPath: "/shared",
				},
			},
		},
	}

	initContainers := builder.buildInitContainers(userInitContainers)

	// Find the user init container
	var userInit *corev1.Container
	for i := range initContainers {
		if initContainers[i].Name == "user-my-init" {
			userInit = &initContainers[i]
			break
		}
	}

	if userInit == nil {
		t.Fatal("User init container not found")
	}

	// Check volumeMount name was updated
	if len(userInit.VolumeMounts) != 1 {
		t.Fatalf("Expected 1 volumeMount, got %d", len(userInit.VolumeMounts))
	}

	if userInit.VolumeMounts[0].Name != "user-shared-bin" {
		t.Errorf("VolumeMount name = %q, want %q",
			userInit.VolumeMounts[0].Name, "user-shared-bin")
	}
}

func int64Ptr(i int64) *int64 {
	return &i
}
