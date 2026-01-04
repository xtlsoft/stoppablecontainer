/*
Copyright 2024.

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

// mount-helper is a DaemonSet component that handles privileged mount operations
// for StoppableContainer. It runs on each node and processes mount requests from
// provider pods, eliminating the need for privileged containers in user workloads.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	// HostRootPath is where the host root filesystem is mounted in the DaemonSet container
	HostRootPath = "/host"
	// WorkBasePath is the base path for stoppablecontainer work directories (on host)
	WorkBasePath = "/var/lib/stoppablecontainer"
	// RequestFileName is the name of the mount request file
	RequestFileName = "request.json"
	// ReadyFileName is the name of the ready signal file
	ReadyFileName = "ready.json"
	// RootfsMarkerEnv is the environment variable that identifies rootfs containers
	RootfsMarkerEnv = "ROOTFS_MARKER=true"
	// PollInterval is how often to scan for new requests
	PollInterval = 2 * time.Second
)

// MountRequest represents a request from a provider pod to set up mounts
type MountRequest struct {
	PodUID    string `json:"pod_uid"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

// MountResponse represents the response after processing a mount request
type MountResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

var log logr.Logger

func main() {
	log = zap.New(zap.UseDevMode(true))
	log.Info("mount-helper starting", "hostRoot", HostRootPath, "workBase", WorkBasePath)

	// Main loop: scan for mount requests and process them
	for {
		if err := scanAndProcessRequests(); err != nil {
			log.Error(err, "error processing requests")
		}
		time.Sleep(PollInterval)
	}
}

// scanAndProcessRequests scans the work directory for mount requests
func scanAndProcessRequests() error {
	hostWorkBase := filepath.Join(HostRootPath, WorkBasePath)

	entries, err := os.ReadDir(hostWorkBase)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No work directory yet
		}
		return fmt.Errorf("failed to read work directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		workDir := filepath.Join(hostWorkBase, entry.Name())
		requestFile := filepath.Join(workDir, RequestFileName)

		if _, err := os.Stat(requestFile); err != nil {
			continue // No request file
		}

		log.Info("found mount request", "workDir", workDir)

		if err := processRequest(workDir, requestFile); err != nil {
			log.Error(err, "failed to process request", "workDir", workDir)
			// Write error response
			_ = writeResponse(workDir, MountResponse{
				Status:  "error",
				Message: err.Error(),
			})
		}
	}

	return nil
}

// processRequest handles a single mount request
func processRequest(workDir, requestFile string) error {
	// Read request
	data, err := os.ReadFile(requestFile)
	if err != nil {
		return fmt.Errorf("failed to read request: %w", err)
	}

	var request MountRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return fmt.Errorf("failed to parse request: %w", err)
	}

	log.Info("processing request", "podUID", request.PodUID)

	// Find the rootfs container PID
	rootfsPID, err := findRootfsContainer(request.PodUID)
	if err != nil {
		return fmt.Errorf("failed to find rootfs container: %w", err)
	}

	log.Info("found rootfs container", "pid", rootfsPID)

	// Get overlayfs mount options from container
	overlayOpts, err := getOverlayfsOptions(rootfsPID)
	if err != nil {
		return fmt.Errorf("failed to get overlayfs options: %w", err)
	}

	log.Info("got overlayfs options", "opts", overlayOpts)

	// Create rootfs directory
	rootfsDir := filepath.Join(workDir, "rootfs")
	if err := os.MkdirAll(rootfsDir, 0755); err != nil {
		return fmt.Errorf("failed to create rootfs dir: %w", err)
	}

	// Adjust paths to use /host prefix
	overlayOptsHost := adjustPathsForHost(overlayOpts)

	// Mount overlayfs
	if err := mountOverlay(rootfsDir, overlayOptsHost); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}

	log.Info("mounted overlay")

	// Mount proc, dev, sys
	if err := mountProcDevSys(rootfsDir); err != nil {
		log.Error(err, "warning: failed to mount some special filesystems")
		// Continue anyway, these might already be mounted or not strictly required
	}

	// Remove request file
	if err := os.Remove(requestFile); err != nil {
		log.Error(err, "warning: failed to remove request file")
	}

	// Write ready response
	_ = writeResponse(workDir, MountResponse{Status: "ready"})

	log.Info("mount complete", "workDir", workDir)
	return nil
}

// findRootfsContainer searches /proc for a container with ROOTFS_MARKER env var
// belonging to the specified pod UID
func findRootfsContainer(podUID string) (int, error) {
	// Convert pod UID format for cgroup matching (replace - with _)
	podUIDCgroup := strings.ReplaceAll(podUID, "-", "_")

	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc: %w", err)
	}

	for _, entry := range entries {
		// Skip non-numeric entries
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		// Check cgroup for pod UID
		cgroupPath := filepath.Join(procDir, entry.Name(), "cgroup")
		cgroupData, err := os.ReadFile(cgroupPath)
		if err != nil {
			continue
		}

		if !strings.Contains(string(cgroupData), podUIDCgroup) {
			continue
		}

		// Check environ for ROOTFS_MARKER
		environPath := filepath.Join(procDir, entry.Name(), "environ")
		environData, err := os.ReadFile(environPath)
		if err != nil {
			continue
		}

		if strings.Contains(string(environData), RootfsMarkerEnv) {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("rootfs container not found for pod %s", podUID)
}

// getOverlayfsOptions reads the overlayfs mount options from a container's /proc/PID/mounts
func getOverlayfsOptions(pid int) (string, error) {
	mountsPath := fmt.Sprintf("/proc/%d/mounts", pid)
	file, err := os.Open(mountsPath)
	if err != nil {
		return "", fmt.Errorf("failed to open mounts: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Looking for: overlay / overlay <options> 0 0
		if strings.HasPrefix(line, "overlay / overlay ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				return parts[3], nil
			}
		}
	}

	return "", fmt.Errorf("overlayfs mount not found")
}

// adjustPathsForHost adds /host prefix to containerd paths in overlay options
func adjustPathsForHost(opts string) string {
	// Replace all occurrences of /var/lib/containerd with /host/var/lib/containerd
	return strings.ReplaceAll(opts, "/var/lib/containerd", HostRootPath+"/var/lib/containerd")
}

// mountOverlay creates an overlay mount
func mountOverlay(target, options string) error {
	// Parse options to verify they're valid
	if !strings.Contains(options, "lowerdir=") || !strings.Contains(options, "upperdir=") {
		return fmt.Errorf("invalid overlay options: missing lowerdir or upperdir")
	}

	// Use syscall.Mount
	err := syscall.Mount("overlay", target, "overlay", 0, options)
	if err != nil {
		return fmt.Errorf("mount syscall failed: %w", err)
	}

	return nil
}

// mountProcDevSys mounts proc, dev, and sys into the rootfs
func mountProcDevSys(rootfsDir string) error {
	var errs []string

	// Mount proc
	procDir := filepath.Join(rootfsDir, "proc")
	if err := os.MkdirAll(procDir, 0755); err != nil {
		errs = append(errs, fmt.Sprintf("mkdir proc: %v", err))
	} else if err := syscall.Mount("proc", procDir, "proc", 0, ""); err != nil {
		errs = append(errs, fmt.Sprintf("proc: %v", err))
	}

	// Bind mount dev from host
	devDir := filepath.Join(rootfsDir, "dev")
	hostDev := filepath.Join(HostRootPath, "dev")
	if err := os.MkdirAll(devDir, 0755); err != nil {
		errs = append(errs, fmt.Sprintf("mkdir dev: %v", err))
	} else if err := syscall.Mount(hostDev, devDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		errs = append(errs, fmt.Sprintf("dev: %v", err))
	} else {
		// Make it rslave to prevent propagation back
		_ = syscall.Mount("", devDir, "", syscall.MS_SLAVE|syscall.MS_REC, "")
	}

	// Bind mount sys from host
	sysDir := filepath.Join(rootfsDir, "sys")
	hostSys := filepath.Join(HostRootPath, "sys")
	if err := os.MkdirAll(sysDir, 0755); err != nil {
		errs = append(errs, fmt.Sprintf("mkdir sys: %v", err))
	} else if err := syscall.Mount(hostSys, sysDir, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		errs = append(errs, fmt.Sprintf("sys: %v", err))
	} else {
		// Make it rslave to prevent propagation back
		_ = syscall.Mount("", sysDir, "", syscall.MS_SLAVE|syscall.MS_REC, "")
	}

	// Also mount /dev/pts and /dev/shm if they exist
	devPtsDir := filepath.Join(rootfsDir, "dev", "pts")
	hostDevPts := filepath.Join(HostRootPath, "dev", "pts")
	if _, err := os.Stat(hostDevPts); err == nil {
		if err := os.MkdirAll(devPtsDir, 0755); err == nil {
			_ = syscall.Mount(hostDevPts, devPtsDir, "", syscall.MS_BIND, "")
		}
	}

	devShmDir := filepath.Join(rootfsDir, "dev", "shm")
	hostDevShm := filepath.Join(HostRootPath, "dev", "shm")
	if _, err := os.Stat(hostDevShm); err == nil {
		if err := os.MkdirAll(devShmDir, 0755); err == nil {
			_ = syscall.Mount(hostDevShm, devShmDir, "", syscall.MS_BIND, "")
		}
	}

	// Create tmp directory with proper permissions
	tmpDir := filepath.Join(rootfsDir, "tmp")
	if err := os.MkdirAll(tmpDir, 01777); err != nil {
		errs = append(errs, fmt.Sprintf("mkdir tmp: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("mount errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// writeResponse writes a response file to the work directory
func writeResponse(workDir string, response MountResponse) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}

	readyFile := filepath.Join(workDir, ReadyFileName)
	return os.WriteFile(readyFile, data, 0644)
}
