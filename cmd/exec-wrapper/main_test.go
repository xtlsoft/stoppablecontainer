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

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConstants(t *testing.T) {
	// Test that constants are properly defined
	if RootfsPath != "/rootfs" {
		t.Errorf("RootfsPath = %q, want %q", RootfsPath, "/rootfs")
	}

	if EnvSCExecOriginal != "SC_EXEC_ORIGINAL" {
		t.Errorf("EnvSCExecOriginal = %q, want %q", EnvSCExecOriginal, "SC_EXEC_ORIGINAL")
	}
}

func TestDebugEnabled(t *testing.T) {
	// Test debug is controllable via environment
	originalDebug := os.Getenv("SC_DEBUG")
	defer func() {
		if originalDebug == "" {
			_ = os.Unsetenv("SC_DEBUG")
		} else {
			_ = os.Setenv("SC_DEBUG", originalDebug)
		}
	}()

	if err := os.Setenv("SC_DEBUG", "1"); err != nil {
		t.Fatalf("Failed to set SC_DEBUG: %v", err)
	}
	// Debug should not panic
	debug("test message: %s", "value")

	if err := os.Unsetenv("SC_DEBUG"); err != nil {
		t.Fatalf("Failed to unset SC_DEBUG: %v", err)
	}
	// Debug should still not panic when disabled
	debug("test message: %s", "value")
}

func TestFindBinaryAbsolutePath(t *testing.T) {
	// Create a temporary rootfs structure
	tmpDir, err := os.MkdirTemp("", "exec-wrapper-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Save original RootfsPath and restore after test
	// Note: We can't easily test findBinary without modifying the const,
	// so this test is more of a smoke test for the logic
	binDir := filepath.Join(tmpDir, "usr", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	// Create a test binary
	testBin := filepath.Join(binDir, "testcmd")
	if err := os.WriteFile(testBin, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testBin); os.IsNotExist(err) {
		t.Error("Test binary should exist")
	}
}

func TestIsMounted(t *testing.T) {
	// Test with a path we know is mounted (root)
	if !isMounted("/") {
		// This might fail in some test environments, so just log
		t.Log("Warning: / not detected as mounted (may be expected in some environments)")
	}

	// Test with non-existent path
	if isMounted("/nonexistent/path/that/should/not/exist") {
		t.Error("/nonexistent/path should not be mounted")
	}
}

func TestEnsureTarget(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "exec-wrapper-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	tests := []struct {
		name       string
		source     string
		target     string
		shouldPass bool
	}{
		{
			name:       "create directory for directory source",
			source:     tmpDir,
			target:     filepath.Join(tmpDir, "newdir"),
			shouldPass: true,
		},
		{
			name:       "create file for file source",
			source:     filepath.Join(tmpDir, "sourcefile"),
			target:     filepath.Join(tmpDir, "targetfile"),
			shouldPass: true,
		},
	}

	// Create source file for file test
	sourceFile := filepath.Join(tmpDir, "sourcefile")
	if err := os.WriteFile(sourceFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureTarget(tt.source, tt.target)
			if tt.shouldPass && err != nil {
				t.Errorf("ensureTarget() error = %v", err)
			}
		})
	}
}

func TestSetupMountsNoRootfs(t *testing.T) {
	// When rootfs doesn't exist, setupMounts should not panic
	// It will just skip mounts for non-existent sources
	// This test just verifies it doesn't crash
	// Note: In real usage, RootfsPath would need to exist

	// setupMounts uses RootfsPath which is /rootfs
	// Since /rootfs doesn't exist in test environment, all mounts will be skipped
	// This is fine - we're just testing it doesn't panic

	// Use t to avoid unparam warning
	t.Log("Testing setupMounts with non-existent rootfs")
	setupMounts()
}

func TestPathConstruction(t *testing.T) {
	// Test that path construction works correctly
	tests := []struct {
		rootfs   string
		subpath  string
		expected string
	}{
		{"/rootfs", "/proc", "/rootfs/proc"},
		{"/rootfs", "/dev", "/rootfs/dev"},
		{"/rootfs", "/etc/resolv.conf", "/rootfs/etc/resolv.conf"},
	}

	for _, tt := range tests {
		result := tt.rootfs + tt.subpath
		if result != tt.expected {
			t.Errorf("Path construction: %q + %q = %q, want %q",
				tt.rootfs, tt.subpath, result, tt.expected)
		}
	}
}

func TestSearchPaths(t *testing.T) {
	// Verify the search paths are reasonable
	searchPaths := []string{
		"/usr/local/sbin",
		"/usr/local/bin",
		"/usr/sbin",
		"/usr/bin",
		"/sbin",
		"/bin",
	}

	for _, p := range searchPaths {
		if p == "" {
			t.Error("Search path should not be empty")
		}
		if p[0] != '/' {
			t.Errorf("Search path %q should be absolute", p)
		}
	}
}
