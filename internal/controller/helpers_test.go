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

package controller

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestIsPodReady(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod not running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expected: false,
		},
		{
			name: "pod running but not ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionFalse,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod running and ready",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod running with multiple conditions",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodScheduled,
							Status: corev1.ConditionTrue,
						},
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod running without ready condition",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      corev1.PodRunning,
					Conditions: []corev1.PodCondition{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodReady(tt.pod)
			if result != tt.expected {
				t.Errorf("isPodReady() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsPodFailed(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod pending",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expected: false,
		},
		{
			name: "pod running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expected: false,
		},
		{
			name: "pod succeeded",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodSucceeded,
				},
			},
			expected: false,
		},
		{
			name: "pod failed",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPodFailed(tt.pod)
			if result != tt.expected {
				t.Errorf("isPodFailed() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetPodFailureReason(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected string
	}{
		{
			name: "pod with message",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Message: "Pod was evicted",
				},
			},
			expected: "Pod was evicted",
		},
		{
			name: "pod with reason",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Reason: "OOMKilled",
				},
			},
			expected: "OOMKilled",
		},
		{
			name: "pod with waiting container",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{
									Reason: "ImagePullBackOff",
								},
							},
						},
					},
				},
			},
			expected: "ImagePullBackOff",
		},
		{
			name: "pod with terminated container",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									Reason: "Error",
								},
							},
						},
					},
				},
			},
			expected: "Error",
		},
		{
			name: "pod with no clear reason",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
						},
					},
				},
			},
			expected: "Unknown",
		},
		{
			name: "empty pod status",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{},
			},
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPodFailureReason(tt.pod)
			if result != tt.expected {
				t.Errorf("getPodFailureReason() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Test helper functions from stoppablecontainer_controller.go
func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	falseVal := boolPtr(false)

	if trueVal == nil || *trueVal != true {
		t.Error("boolPtr(true) should return pointer to true")
	}
	if falseVal == nil || *falseVal != false {
		t.Error("boolPtr(false) should return pointer to false")
	}
}

func TestStringPtr(t *testing.T) {
	hello := stringPtr("hello")
	empty := stringPtr("")

	if hello == nil || *hello != "hello" {
		t.Error("stringPtr(\"hello\") should return pointer to \"hello\"")
	}
	if empty == nil || *empty != "" {
		t.Error("stringPtr(\"\") should return pointer to empty string")
	}
}

func TestErrorf(t *testing.T) {
	err := errorf("test error: %s", "details")
	if err == nil {
		t.Fatal("errorf should return an error")
	}
	if err.Error() != "test error: details" {
		t.Errorf("errorf returned wrong message: %s", err.Error())
	}
}

// Test constants
func TestFinalizerName(t *testing.T) {
	if FinalizerName != "stoppablecontainer.xtlsoft.top/finalizer" {
		t.Errorf("FinalizerName = %s, want stoppablecontainer.xtlsoft.top/finalizer", FinalizerName)
	}
}

func TestSCIFinalizerName(t *testing.T) {
	if SCIFinalizerName != "stoppablecontainerinstance.xtlsoft.top/finalizer" {
		t.Errorf("SCIFinalizerName = %s, want stoppablecontainerinstance.xtlsoft.top/finalizer", SCIFinalizerName)
	}
}

func TestConditionTypes(t *testing.T) {
	if ConditionTypeReady != "Ready" {
		t.Errorf("ConditionTypeReady = %s, want Ready", ConditionTypeReady)
	}
	if ConditionTypeProviderReady != "ProviderReady" {
		t.Errorf("ConditionTypeProviderReady = %s, want ProviderReady", ConditionTypeProviderReady)
	}
	if ConditionTypeConsumerReady != "ConsumerReady" {
		t.Errorf("ConditionTypeConsumerReady = %s, want ConsumerReady", ConditionTypeConsumerReady)
	}
}
