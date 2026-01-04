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

// Package main provides the stoppablecontainer-exec binary.
// This is a wrapper that executes commands inside a chroot environment.
// It is designed to be called by kubectl exec and automatically chroot into /rootfs.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	// RootfsPath is where the actual rootfs is mounted
	RootfsPath = "/rootfs"

	// WrapperBinPath is where this wrapper binary is located
	WrapperBinPath = "/.sc-bin/sc-exec"

	// EnvSCExecOriginal is set when we are executing the original command
	EnvSCExecOriginal = "SC_EXEC_ORIGINAL"

	// EnvSCDebug enables debug logging
	EnvSCDebug = "SC_DEBUG"
)

func debug(format string, args ...interface{}) {
	if os.Getenv(EnvSCDebug) != "" {
		fmt.Fprintf(os.Stderr, "[sc-exec] "+format+"\n", args...)
	}
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[sc-exec] ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	// Check if we're being called as a wrapper (via symlink) or directly
	execName := filepath.Base(os.Args[0])
	debug("Invoked as: %s, args: %v", execName, os.Args)

	// If called directly as sc-exec, expect: sc-exec <command> [args...]
	// If called via symlink (e.g., /bin/bash -> sc-exec), execute the symlink name
	var command string
	var args []string

	if execName == "sc-exec" || execName == "stoppablecontainer-exec" {
		if len(os.Args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: %s <command> [args...]\n", os.Args[0])
			fmt.Fprintf(os.Stderr, "\nThis wrapper executes commands inside the chroot at %s\n", RootfsPath)
			os.Exit(1)
		}
		command = os.Args[1]
		args = os.Args[1:]
	} else {
		// Called via symlink, use the symlink name as the command
		command = execName
		args = os.Args
	}

	// Prevent infinite loops - if we're already inside the chroot, just exec
	if os.Getenv(EnvSCExecOriginal) == "1" {
		debug("Already in chroot context, executing directly: %s", command)
		execDirect(command, args)
		return
	}

	// Verify rootfs exists
	if _, err := os.Stat(RootfsPath); os.IsNotExist(err) {
		fatal("Rootfs not found at %s. Is the provider pod ready?", RootfsPath)
	}

	// Setup bind mounts for special filesystems
	setupMounts()

	// Find the actual binary path in the rootfs
	binaryPath := findBinary(command)
	if binaryPath == "" {
		fatal("Command not found: %s", command)
	}
	debug("Found binary at: %s", binaryPath)

	// Perform chroot and exec
	chrootExec(binaryPath, args)
}

// setupMounts creates necessary bind mounts inside rootfs.
// Note: mount failures are non-fatal as mounts may already be done by DaemonSet.
func setupMounts() {
	mounts := []struct {
		source string
		target string
		fstype string
		flags  uintptr
	}{
		{"/proc", RootfsPath + "/proc", "proc", syscall.MS_BIND},
		{"/dev", RootfsPath + "/dev", "", syscall.MS_BIND | syscall.MS_REC},
		{"/sys", RootfsPath + "/sys", "", syscall.MS_BIND | syscall.MS_REC},
		{"/etc/resolv.conf", RootfsPath + "/etc/resolv.conf", "", syscall.MS_BIND},
		{"/etc/hosts", RootfsPath + "/etc/hosts", "", syscall.MS_BIND},
		{"/etc/hostname", RootfsPath + "/etc/hostname", "", syscall.MS_BIND},
	}

	// Also bind mount any kubernetes service account tokens
	saPath := "/var/run/secrets/kubernetes.io/serviceaccount"
	if _, err := os.Stat(saPath); err == nil {
		targetPath := RootfsPath + saPath
		_ = os.MkdirAll(filepath.Dir(targetPath), 0755)
		mounts = append(mounts, struct {
			source string
			target string
			fstype string
			flags  uintptr
		}{saPath, targetPath, "", syscall.MS_BIND})
	}

	for _, m := range mounts {
		// Check if source exists
		if _, err := os.Stat(m.source); os.IsNotExist(err) {
			debug("Skipping mount, source doesn't exist: %s", m.source)
			continue
		}

		// Check if already mounted
		if isMounted(m.target) {
			debug("Already mounted: %s", m.target)
			continue
		}

		// Ensure target exists
		if err := ensureTarget(m.source, m.target); err != nil {
			debug("Failed to create mount target %s: %v", m.target, err)
			continue
		}

		// Perform bind mount
		debug("Mounting %s -> %s", m.source, m.target)
		if err := syscall.Mount(m.source, m.target, m.fstype, m.flags, ""); err != nil {
			debug("Failed to mount %s: %v", m.target, err)
			// Non-fatal, continue
		}
	}
}

// ensureTarget creates the mount target (file or directory as appropriate)
func ensureTarget(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return os.MkdirAll(target, 0755)
	}

	// It's a file, ensure parent dir exists and create empty file
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(target, os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// isMounted checks if a path is already a mount point
func isMounted(path string) bool {
	// Simple check: see if we can stat it and it's not under rootfs's parent mount
	// A more robust check would parse /proc/mounts
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}

	// Read /proc/mounts to check
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true
		}
	}
	return false
}

// findBinary locates a binary in the rootfs
func findBinary(name string) string {
	// If it's an absolute path, use it directly
	if strings.HasPrefix(name, "/") {
		fullPath := RootfsPath + name
		if _, err := os.Stat(fullPath); err == nil {
			return name // Return path relative to rootfs
		}
		return ""
	}

	// Search in PATH-like locations within rootfs
	searchPaths := []string{
		"/usr/local/sbin",
		"/usr/local/bin",
		"/usr/sbin",
		"/usr/bin",
		"/sbin",
		"/bin",
	}

	for _, dir := range searchPaths {
		fullPath := RootfsPath + dir + "/" + name
		if _, err := os.Stat(fullPath); err == nil {
			return dir + "/" + name
		}
	}

	return ""
}

// chrootExec performs chroot and exec
func chrootExec(binaryPath string, args []string) {
	debug("Chrooting to %s and executing %s", RootfsPath, binaryPath)

	// Set environment variable to prevent recursion
	_ = os.Setenv(EnvSCExecOriginal, "1")

	// Get current working directory and try to preserve it
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "/"
	}

	// Chroot
	if err := syscall.Chroot(RootfsPath); err != nil {
		fatal("Failed to chroot: %v", err)
	}

	// Change to root first, then try to restore cwd
	if err := os.Chdir("/"); err != nil {
		fatal("Failed to chdir to /: %v", err)
	}

	// Try to change to the original working directory
	if cwd != "/" {
		if err := os.Chdir(cwd); err != nil {
			debug("Could not restore cwd %s: %v", cwd, err)
			// Stay in /
		}
	}

	// Prepare environment
	env := os.Environ()
	// Filter out our special env var from the child's environment
	filteredEnv := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, EnvSCExecOriginal+"=") {
			filteredEnv = append(filteredEnv, e)
		}
	}

	// Exec the command
	debug("Execing: %s with args %v", binaryPath, args)
	if err := syscall.Exec(binaryPath, args, filteredEnv); err != nil {
		fatal("Failed to exec %s: %v", binaryPath, err)
	}
}

// execDirect executes a command directly without chroot (for when we're already inside)
func execDirect(command string, args []string) {
	// Find the binary in PATH
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}

	var binaryPath string
	if strings.HasPrefix(command, "/") {
		binaryPath = command
	} else {
		for _, dir := range strings.Split(path, ":") {
			candidate := dir + "/" + command
			if _, err := os.Stat(candidate); err == nil {
				binaryPath = candidate
				break
			}
		}
	}

	if binaryPath == "" {
		fatal("Command not found: %s", command)
	}

	env := os.Environ()
	if err := syscall.Exec(binaryPath, args, env); err != nil {
		fatal("Failed to exec %s: %v", binaryPath, err)
	}
}
