package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdjustPathsForHost(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple path",
			input:    "lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/1/fs",
			expected: "lowerdir=/host/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/1/fs",
		},
		{
			name:     "multiple paths",
			input:    "lowerdir=/var/lib/containerd/a,upperdir=/var/lib/containerd/b,workdir=/var/lib/containerd/c",
			expected: "lowerdir=/host/var/lib/containerd/a,upperdir=/host/var/lib/containerd/b,workdir=/host/var/lib/containerd/c",
		},
		{
			name:     "no containerd paths",
			input:    "lowerdir=/other/path,upperdir=/other/path2",
			expected: "lowerdir=/other/path,upperdir=/other/path2",
		},
		{
			name:     "mixed paths",
			input:    "lowerdir=/var/lib/containerd/a:/other/b,upperdir=/var/lib/containerd/c",
			expected: "lowerdir=/host/var/lib/containerd/a:/other/b,upperdir=/host/var/lib/containerd/c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustPathsForHost(tt.input)
			if result != tt.expected {
				t.Errorf("adjustPathsForHost(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMountRequest_JSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantUID string
	}{
		{
			name:    "basic request",
			json:    `{"pod_uid":"abc-123","namespace":"default","name":"test"}`,
			wantUID: "abc-123",
		},
		{
			name:    "uid with underscores",
			json:    `{"pod_uid":"abc_123_def","namespace":"ns","name":"n"}`,
			wantUID: "abc_123_def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req MountRequest
			if err := json.Unmarshal([]byte(tt.json), &req); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}
			if req.PodUID != tt.wantUID {
				t.Errorf("PodUID = %q, want %q", req.PodUID, tt.wantUID)
			}
		})
	}
}

func TestWriteResponse(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "mount-helper-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write a response
	response := MountResponse{
		Status:  "ready",
		Message: "test message",
	}

	if err := writeResponse(tmpDir, response); err != nil {
		t.Fatalf("writeResponse failed: %v", err)
	}

	// Verify the file was created
	readyFile := filepath.Join(tmpDir, ReadyFileName)
	data, err := os.ReadFile(readyFile)
	if err != nil {
		t.Fatalf("Failed to read ready file: %v", err)
	}

	// Verify content
	if !strings.Contains(string(data), "ready") {
		t.Errorf("Ready file should contain 'ready', got %s", string(data))
	}
	if !strings.Contains(string(data), "test message") {
		t.Errorf("Ready file should contain 'test message', got %s", string(data))
	}
}
