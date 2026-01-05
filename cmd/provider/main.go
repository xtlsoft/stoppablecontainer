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

// Package main provides the sc-provider binary.
// This is the main process for the provider container that coordinates
// with the mount-helper DaemonSet to set up the rootfs mount.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	// PropagatedPath is where the hostPath volume is mounted
	PropagatedPath = "/propagated"
	// RequestFile is the file written to request a mount from the DaemonSet
	RequestFile = "request.json"
	// ReadyFile is the file written by the DaemonSet when mount is complete
	ReadyFile = "ready.json"
	// RootfsDir is the directory where the rootfs is mounted
	RootfsDir = "rootfs"
	// ReadyMarker is a file we create to signal the pod is ready
	ReadyMarker = "ready"
)

// MountRequest is the request sent to the DaemonSet
type MountRequest struct {
	PodUID    string `json:"pod_uid"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// MountResponse is the response from the DaemonSet
type MountResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func log(format string, args ...interface{}) {
	fmt.Printf("[provider] "+format+"\n", args...)
}

func main() {
	log("Starting provider process...")

	// Get environment variables
	podUID := os.Getenv("POD_UID")
	podNamespace := os.Getenv("POD_NAMESPACE")
	podName := os.Getenv("POD_NAME")

	if podUID == "" {
		log("ERROR: POD_UID environment variable not set")
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Write mount request
	requestPath := filepath.Join(PropagatedPath, RequestFile)
	readyPath := filepath.Join(PropagatedPath, ReadyFile)
	rootfsPath := filepath.Join(PropagatedPath, RootfsDir)
	markerPath := filepath.Join(PropagatedPath, ReadyMarker)

	// Retry loop for writing request and waiting for mount
	var lastError error
	for attempt := 1; attempt <= 3; attempt++ {
		if attempt > 1 {
			log("Retrying mount setup (attempt %d/3)...", attempt)
			// Remove old ready file to force DaemonSet to reprocess
			_ = os.Remove(readyPath)
			time.Sleep(time.Second)
		}

		// Write mount request
		log("Writing mount request...")
		request := MountRequest{
			PodUID:    podUID,
			Namespace: podNamespace,
			Name:      podName,
		}
		requestData, err := json.Marshal(request)
		if err != nil {
			lastError = fmt.Errorf("failed to marshal request: %w", err)
			continue
		}
		if err := os.WriteFile(requestPath, requestData, 0644); err != nil {
			lastError = fmt.Errorf("failed to write request: %w", err)
			continue
		}
		log("Request written, waiting for DaemonSet to complete mount...")

		// Wait for ready signal with fast initial polling
		success := false
		for i := 0; i < 300; i++ {
			data, err := os.ReadFile(readyPath)
			if err == nil {
				var response MountResponse
				if err := json.Unmarshal(data, &response); err == nil {
					log("Mount response received: %s", response.Status)
					if response.Status == "ready" {
						success = true
						break
					} else if response.Status == "error" {
						lastError = fmt.Errorf("mount failed: %s", response.Message)
						break
					}
				}
			}
			// Fast polling for first 2 seconds, then slow down
			if i < 20 {
				time.Sleep(100 * time.Millisecond)
			} else {
				time.Sleep(500 * time.Millisecond)
			}
		}

		if success {
			// Verify rootfs is actually mounted
			if _, err := os.Stat(rootfsPath); os.IsNotExist(err) {
				lastError = fmt.Errorf("rootfs directory not found at %s", rootfsPath)
				continue
			}
			// Check for some basic directories
			if _, err := os.Stat(filepath.Join(rootfsPath, "bin")); os.IsNotExist(err) {
				if _, err := os.Stat(filepath.Join(rootfsPath, "usr", "bin")); os.IsNotExist(err) {
					lastError = fmt.Errorf("rootfs appears empty, no /bin or /usr/bin found")
					continue
				}
			}
			log("Rootfs mounted successfully at %s", rootfsPath)
			lastError = nil
			break
		}
	}

	if lastError != nil {
		log("ERROR: Failed to set up mount after 3 attempts: %v", lastError)
		os.Exit(1)
	}

	// List some contents for debugging
	entries, err := os.ReadDir(rootfsPath)
	if err == nil {
		log("Rootfs contents:")
		for i, e := range entries {
			if i >= 10 {
				log("  ... and %d more", len(entries)-10)
				break
			}
			log("  %s", e.Name())
		}
	}

	// Create ready marker file for Kubernetes readiness probe
	if err := os.WriteFile(markerPath, []byte("ready"), 0644); err != nil {
		log("WARNING: Failed to write ready marker: %v", err)
	}

	log("Provider ready, waiting for termination signal...")

	// Wait for termination signal
	sig := <-sigChan
	log("Received signal %v, shutting down...", sig)
}
