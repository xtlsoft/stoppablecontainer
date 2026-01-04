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

	scv1alpha1 "github.com/xtlsoft/stoppablecontainer/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetHostPath(t *testing.T) {
	tests := []struct {
		name     string
		sci      *scv1alpha1.StoppableContainerInstance
		expected string
	}{
		{
			name: "default prefix",
			sci: &scv1alpha1.StoppableContainerInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-instance",
					Namespace: "default",
				},
				Spec: scv1alpha1.StoppableContainerInstanceSpec{},
			},
			expected: "/var/lib/stoppablecontainer/default/test-instance",
		},
		{
			name: "custom prefix",
			sci: &scv1alpha1.StoppableContainerInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-app",
					Namespace: "production",
				},
				Spec: scv1alpha1.StoppableContainerInstanceSpec{
					HostPathPrefix: "/custom/path",
				},
			},
			expected: "/custom/path/production/my-app",
		},
		{
			name: "empty prefix uses default",
			sci: &scv1alpha1.StoppableContainerInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app",
					Namespace: "ns",
				},
				Spec: scv1alpha1.StoppableContainerInstanceSpec{
					HostPathPrefix: "",
				},
			},
			expected: "/var/lib/stoppablecontainer/ns/app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetHostPath(tt.sci)
			if result != tt.expected {
				t.Errorf("GetHostPath() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	if trueVal == nil || *trueVal != true {
		t.Errorf("boolPtr(true) failed")
	}

	falseVal := boolPtr(false)
	if falseVal == nil || *falseVal != false {
		t.Errorf("boolPtr(false) failed")
	}
}

func TestMountPropagationPtr(t *testing.T) {
	mp := mountPropagationPtr(corev1.MountPropagationBidirectional)
	if mp == nil || *mp != corev1.MountPropagationBidirectional {
		t.Errorf("mountPropagationPtr() failed")
	}
}
