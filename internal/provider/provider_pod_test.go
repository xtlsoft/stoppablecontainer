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
	"strings"
	"testing"

	scv1alpha1 "github.com/xtlsoft/stoppablecontainer/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testProviderAppName = "my-app"
)

func createTestSCI(name, namespace, image string) *scv1alpha1.StoppableContainerInstance {
	return &scv1alpha1.StoppableContainerInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       "test-uid-12345",
		},
		Spec: scv1alpha1.StoppableContainerInstanceSpec{
			Running: true,
			Template: scv1alpha1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "main",
							Image:   image,
							Command: []string{"/bin/sh", "-c", "echo hello"},
						},
					},
				},
			},
		},
	}
}

func TestNewProviderPodBuilder(t *testing.T) {
	sci := createTestSCI("test", "default", "busybox:latest")
	builder := NewProviderPodBuilder(sci)

	if builder == nil {
		t.Fatal("NewProviderPodBuilder returned nil")
	}
	if builder.sci != sci {
		t.Error("Builder sci does not match input")
	}
}

func TestProviderPodBuilder_Build(t *testing.T) {
	sci := createTestSCI(testProviderAppName, "production", "nginx:latest")
	builder := NewProviderPodBuilder(sci)
	pod := builder.Build()

	// Check pod name
	if pod.Name != testProviderAppName+"-provider" {
		t.Errorf("Pod name = %q, want %q", pod.Name, testProviderAppName+"-provider")
	}

	// Check namespace
	if pod.Namespace != "production" {
		t.Errorf("Pod namespace = %q, want %q", pod.Namespace, "production")
	}

	// Check labels
	if pod.Labels[LabelManagedBy] != ManagedByValue {
		t.Errorf("Label %s = %q, want %q", LabelManagedBy, pod.Labels[LabelManagedBy], ManagedByValue)
	}
	if pod.Labels[LabelInstance] != testProviderAppName {
		t.Errorf("Label %s = %q, want %q", LabelInstance, pod.Labels[LabelInstance], testProviderAppName)
	}
	if pod.Labels[LabelRole] != "provider" {
		t.Errorf("Label %s = %q, want %q", LabelRole, pod.Labels[LabelRole], "provider")
	}

	// Check owner reference
	if len(pod.OwnerReferences) != 1 {
		t.Fatalf("Expected 1 owner reference, got %d", len(pod.OwnerReferences))
	}
	if pod.OwnerReferences[0].Name != testProviderAppName {
		t.Errorf("OwnerReference name = %q, want %q", pod.OwnerReferences[0].Name, testProviderAppName)
	}

	// Check containers
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("Expected 2 containers, got %d", len(pod.Spec.Containers))
	}

	// Find provider container
	var providerContainer *corev1.Container
	var rootfsContainer *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == ProviderContainerName {
			providerContainer = &pod.Spec.Containers[i]
		}
		if pod.Spec.Containers[i].Name == RootfsContainerName {
			rootfsContainer = &pod.Spec.Containers[i]
		}
	}

	if providerContainer == nil {
		t.Fatal("Provider container not found")
	}
	if rootfsContainer == nil {
		t.Fatal("Rootfs container not found")
	}

	// Check provider container is NOT privileged (DaemonSet handles privileged operations)
	if providerContainer.SecurityContext != nil && providerContainer.SecurityContext.Privileged != nil && *providerContainer.SecurityContext.Privileged {
		t.Error("Provider container should NOT be privileged - DaemonSet handles privileged operations")
	}

	// Check rootfs container uses the user's image
	if rootfsContainer.Image != "nginx:latest" {
		t.Errorf("Rootfs container image = %q, want %q", rootfsContainer.Image, "nginx:latest")
	}

	// Check rootfs container has ROOTFS_MARKER environment variable
	foundRootfsMarker := false
	for _, env := range rootfsContainer.Env {
		if env.Name == RootfsMarkerEnv && env.Value == "true" {
			foundRootfsMarker = true
			break
		}
	}
	if !foundRootfsMarker {
		t.Error("Rootfs container should have ROOTFS_MARKER=true environment variable")
	}

	// Check shared process namespace is NOT required (DaemonSet uses /proc scanning)
	// The provider pod no longer needs shareProcessNamespace since it doesn't use nsenter

	// Check volumes exist
	foundPropagatedVol := false
	foundPauseVol := false
	for _, v := range pod.Spec.Volumes {
		if v.Name == PropagatedVolumeName {
			foundPropagatedVol = true
			if v.HostPath == nil {
				t.Error("Propagated volume should be hostPath")
			}
		}
		if v.Name == PauseVolumeName {
			foundPauseVol = true
		}
	}
	if !foundPropagatedVol {
		t.Error("Propagated volume not found")
	}
	if !foundPauseVol {
		t.Error("Pause volume not found")
	}
}

func TestProviderPodBuilder_ProviderBinary(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewProviderPodBuilder(sci)
	pod := builder.Build()

	// Find provider container
	var providerContainer *corev1.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == ProviderContainerName {
			providerContainer = &pod.Spec.Containers[i]
			break
		}
	}

	if providerContainer == nil {
		t.Fatal("Provider container not found")
	}

	// Check that provider uses the sc-provider binary
	if len(providerContainer.Command) != 1 {
		t.Fatalf("Provider container should have exactly 1 command, got %d", len(providerContainer.Command))
	}
	if providerContainer.Command[0] != "/sc-provider" {
		t.Errorf("Provider container should run /sc-provider, got %s", providerContainer.Command[0])
	}

	// Check that provider uses ExecWrapperImage
	if providerContainer.Image != ExecWrapperImage {
		t.Errorf("Provider should use ExecWrapperImage, got %s", providerContainer.Image)
	}

	// Check environment variables are set
	envNames := make(map[string]bool)
	for _, env := range providerContainer.Env {
		envNames[env.Name] = true
	}
	requiredEnvs := []string{PodUIDEnv, "POD_NAMESPACE", "POD_NAME"}
	for _, env := range requiredEnvs {
		if !envNames[env] {
			t.Errorf("Missing required environment variable: %s", env)
		}
	}
}

func TestProviderPodBuilder_WithTolerations(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	sci.Spec.Provider.Tolerations = []corev1.Toleration{
		{
			Key:      "node.kubernetes.io/not-ready",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	builder := NewProviderPodBuilder(sci)
	pod := builder.Build()

	if len(pod.Spec.Tolerations) != 1 {
		t.Fatalf("Expected 1 toleration, got %d", len(pod.Spec.Tolerations))
	}
	if pod.Spec.Tolerations[0].Key != "node.kubernetes.io/not-ready" {
		t.Error("Toleration key mismatch")
	}
}

func TestProviderPodBuilder_WithNodeSelector(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	sci.Spec.Provider.NodeSelector = map[string]string{
		"kubernetes.io/os": "linux",
		"node-type":        "gpu",
	}

	builder := NewProviderPodBuilder(sci)
	pod := builder.Build()

	if len(pod.Spec.NodeSelector) != 2 {
		t.Fatalf("Expected 2 node selector entries, got %d", len(pod.Spec.NodeSelector))
	}
	if pod.Spec.NodeSelector["kubernetes.io/os"] != "linux" {
		t.Error("Node selector kubernetes.io/os mismatch")
	}
	if pod.Spec.NodeSelector["node-type"] != "gpu" {
		t.Error("Node selector node-type mismatch")
	}
}

func TestProviderPodBuilder_ProviderResources(t *testing.T) {
	t.Run("default resources", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		builder := NewProviderPodBuilder(sci)
		resources := builder.providerResources()

		if resources.Requests == nil {
			t.Error("Default resources should have requests")
		}
		if resources.Limits == nil {
			t.Error("Default resources should have limits")
		}
		// Check default CPU request is 10m
		cpuReq := resources.Requests[corev1.ResourceCPU]
		if cpuReq.String() != "10m" {
			t.Errorf("Expected CPU request 10m, got %s", cpuReq.String())
		}
	})

	t.Run("custom resources", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		sci.Spec.Provider.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		}
		builder := NewProviderPodBuilder(sci)
		resources := builder.providerResources()

		cpuReq := resources.Requests[corev1.ResourceCPU]
		if cpuReq.String() != "100m" {
			t.Errorf("Expected CPU request 100m, got %s", cpuReq.String())
		}
	})

	t.Run("custom limits only", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		sci.Spec.Provider.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"),
			},
		}
		builder := NewProviderPodBuilder(sci)
		resources := builder.providerResources()

		cpuLimit := resources.Limits[corev1.ResourceCPU]
		if cpuLimit.String() != "500m" {
			t.Errorf("Expected CPU limit 500m, got %s", cpuLimit.String())
		}
	})
}

func TestProviderPodBuilder_BuildRootfsContainer(t *testing.T) {
	t.Run("default image pull policy", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		builder := NewProviderPodBuilder(sci)
		container := builder.buildRootfsContainer()

		if container.Name != RootfsContainerName {
			t.Errorf("Container name = %q, want %q", container.Name, RootfsContainerName)
		}
		if container.Image != "alpine:latest" {
			t.Errorf("Container image = %q, want %q", container.Image, "alpine:latest")
		}
		// ImagePullPolicy should not be set (default)
		if container.ImagePullPolicy != "" {
			t.Errorf("ImagePullPolicy should be empty by default, got %s", container.ImagePullPolicy)
		}
	})

	t.Run("with image pull policy", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		sci.Spec.Template.Spec.Containers[0].ImagePullPolicy = corev1.PullAlways
		builder := NewProviderPodBuilder(sci)
		container := builder.buildRootfsContainer()

		if container.ImagePullPolicy != corev1.PullAlways {
			t.Errorf("ImagePullPolicy = %q, want %q", container.ImagePullPolicy, corev1.PullAlways)
		}
	})

	t.Run("has ROOTFS_MARKER env", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		builder := NewProviderPodBuilder(sci)
		container := builder.buildRootfsContainer()

		found := false
		for _, env := range container.Env {
			if env.Name == RootfsMarkerEnv && env.Value == "true" {
				found = true
				break
			}
		}
		if !found {
			t.Error("ROOTFS_MARKER environment variable not found")
		}
	})

	t.Run("uses pause binary command", func(t *testing.T) {
		sci := createTestSCI("test", "default", "alpine:latest")
		builder := NewProviderPodBuilder(sci)
		container := builder.buildRootfsContainer()

		if len(container.Command) != 1 || !strings.Contains(container.Command[0], "sc-pause") {
			t.Errorf("Command should be pause binary, got %v", container.Command)
		}
	})
}

func TestProviderPodBuilder_MinimalResources(t *testing.T) {
	sci := createTestSCI("test", "default", "alpine:latest")
	builder := NewProviderPodBuilder(sci)
	resources := builder.minimalResources()

	if resources.Requests == nil {
		t.Error("Minimal resources should have requests")
	}
	cpuReq := resources.Requests[corev1.ResourceCPU]
	if cpuReq.String() != "1m" {
		t.Errorf("Expected CPU request 1m, got %s", cpuReq.String())
	}
	memReq := resources.Requests[corev1.ResourceMemory]
	if memReq.String() != "4Mi" {
		t.Errorf("Expected memory request 4Mi, got %s", memReq.String())
	}
}
