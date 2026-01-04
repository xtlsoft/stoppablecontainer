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
	"strings"
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m"},
		{5 * time.Minute, "5m"},
		{59 * time.Minute, "59m"},
		{60 * time.Minute, "1h"},
		{2 * time.Hour, "2h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{7 * 24 * time.Hour, "7d"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatAge(tt.duration)
			if result != tt.expected {
				t.Errorf("formatAge(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestBuildStoppableContainerYAML(t *testing.T) {
	tests := []struct {
		name       string
		scName     string
		ns         string
		image      string
		command    []string
		running    bool
		workingDir string
		env        []string
		ports      []string
		contains   []string
	}{
		{
			name:    "basic yaml",
			scName:  "my-app",
			ns:      "default",
			image:   "ubuntu:22.04",
			command: []string{"/bin/bash"},
			running: true,
			contains: []string{
				"apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1",
				"kind: StoppableContainer",
				"name: my-app",
				"namespace: default",
				"running: true",
				"image: ubuntu:22.04",
				`"/bin/bash"`,
			},
		},
		{
			name:       "with workdir and env",
			scName:     "test-app",
			ns:         "test-ns",
			image:      "nginx:latest",
			command:    []string{"/bin/sh", "-c", "nginx -g 'daemon off;'"},
			running:    false,
			workingDir: "/app",
			env:        []string{"PORT=8080", "DEBUG=true"},
			contains: []string{
				"name: test-app",
				"namespace: test-ns",
				"running: false",
				"workingDir: \"/app\"",
				"name: PORT",
				"value: \"8080\"",
				"name: DEBUG",
				"value: \"true\"",
			},
		},
		{
			name:    "with ports",
			scName:  "web-app",
			ns:      "production",
			image:   "nginx:latest",
			command: nil,
			running: true,
			ports:   []string{"80:http", "443:https"},
			contains: []string{
				"containerPort: 80",
				"name: http",
				"containerPort: 443",
				"name: https",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildStoppableContainerYAML(
				tt.scName, tt.ns, tt.image, tt.command,
				tt.running, tt.workingDir, tt.env, tt.ports,
			)

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("YAML should contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestGVRDefinitions(t *testing.T) {
	// Test StoppableContainer GVR
	if scGVR.Group != "stoppablecontainer.xtlsoft.top" {
		t.Errorf("scGVR.Group = %q, want %q", scGVR.Group, "stoppablecontainer.xtlsoft.top")
	}
	if scGVR.Version != "v1alpha1" {
		t.Errorf("scGVR.Version = %q, want %q", scGVR.Version, "v1alpha1")
	}
	if scGVR.Resource != "stoppablecontainers" {
		t.Errorf("scGVR.Resource = %q, want %q", scGVR.Resource, "stoppablecontainers")
	}

	// Test StoppableContainerInstance GVR
	if sciGVR.Group != "stoppablecontainer.xtlsoft.top" {
		t.Errorf("sciGVR.Group = %q, want %q", sciGVR.Group, "stoppablecontainer.xtlsoft.top")
	}
	if sciGVR.Version != "v1alpha1" {
		t.Errorf("sciGVR.Version = %q, want %q", sciGVR.Version, "v1alpha1")
	}
	if sciGVR.Resource != "stoppablecontainerinstances" {
		t.Errorf("sciGVR.Resource = %q, want %q", sciGVR.Resource, "stoppablecontainerinstances")
	}
}

func TestVersion(t *testing.T) {
	if version == "" {
		t.Error("version should not be empty")
	}
}

func TestGroupVersion(t *testing.T) {
	if GroupVersion != "stoppablecontainer.xtlsoft.top/v1alpha1" {
		t.Errorf("GroupVersion = %q, want %q", GroupVersion, "stoppablecontainer.xtlsoft.top/v1alpha1")
	}
}
