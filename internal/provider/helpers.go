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
)

// DefaultHostPathPrefix is the default prefix for host paths used to share rootfs
const DefaultHostPathPrefix = "/var/lib/stoppablecontainer"

// ManagedByValue is the value used for the managed-by label
const ManagedByValue = "stoppablecontainer"

// GetHostPath returns the host path directory for a StoppableContainerInstance.
// The host path is used to share the rootfs between provider and consumer pods.
func GetHostPath(sci *scv1alpha1.StoppableContainerInstance) string {
	prefix := sci.Spec.HostPathPrefix
	if prefix == "" {
		prefix = DefaultHostPathPrefix
	}
	return filepath.Join(prefix, sci.Namespace, sci.Name)
}

// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool {
	return &b
}

// mountPropagationPtr returns a pointer to a mount propagation mode value.
func mountPropagationPtr(mp corev1.MountPropagationMode) *corev1.MountPropagationMode {
	return &mp
}
