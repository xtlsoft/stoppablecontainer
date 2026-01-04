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

// Package main implements kubectl-sc, a kubectl plugin for StoppableContainer.
//
// Installation:
//
//	go install github.com/xtlsoft/stoppablecontainer/cmd/kubectl-sc@latest
//
// Or download from releases and place in PATH.
//
// Usage:
//
//	kubectl sc list                     # List all StoppableContainers
//	kubectl sc status <name>            # Show status of a StoppableContainer
//	kubectl sc start <name>             # Start a StoppableContainer
//	kubectl sc stop <name>              # Stop a StoppableContainer
//	kubectl sc exec <name> -- <cmd>     # Execute command in container
//	kubectl sc logs <name>              # Show logs from container
//	kubectl sc create <name> --image=<image> -- <cmd>  # Create a new StoppableContainer
//	kubectl sc delete <name>            # Delete a StoppableContainer
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// GroupVersion for StoppableContainer API
	GroupVersion = "stoppablecontainer.xtlsoft.top/v1alpha1"
)

// version is set by ldflags during build
var version = "dev"

var (
	namespace  string
	kubeconfig string
	allNs      bool
)

// StoppableContainer GVR
var scGVR = schema.GroupVersionResource{
	Group:    "stoppablecontainer.xtlsoft.top",
	Version:  "v1alpha1",
	Resource: "stoppablecontainers",
}

// StoppableContainerInstance GVR
var sciGVR = schema.GroupVersionResource{
	Group:    "stoppablecontainer.xtlsoft.top",
	Version:  "v1alpha1",
	Resource: "stoppablecontainerinstances",
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "kubectl-sc",
		Short: "kubectl plugin for StoppableContainer",
		Long: `kubectl-sc is a kubectl plugin for managing StoppableContainers.

StoppableContainer is a Kubernetes operator that enables fast container
start/stop operations while preserving the container's root filesystem.

Examples:
  # List all StoppableContainers
  kubectl sc list

  # Create a new StoppableContainer
  kubectl sc create my-app --image=ubuntu:22.04 -- /bin/bash

  # Start/Stop a container
  kubectl sc start my-app
  kubectl sc stop my-app

  # Execute a command in the container
  kubectl sc exec my-app -- ls -la /

  # View logs
  kubectl sc logs my-app

  # Delete a StoppableContainer
  kubectl sc delete my-app`,
		Version: version,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(
		&namespace, "namespace", "n", "",
		"Kubernetes namespace (default: current context)",
	)
	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.PersistentFlags().BoolVarP(&allNs, "all-namespaces", "A", false, "List across all namespaces")

	// Add commands
	rootCmd.AddCommand(listCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(startCmd())
	rootCmd.AddCommand(stopCmd())
	rootCmd.AddCommand(execCmd())
	rootCmd.AddCommand(logsCmd())
	rootCmd.AddCommand(createCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getClient() (dynamic.Interface, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if namespace != "" {
		configOverrides.Context.Namespace = namespace
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get kubernetes config: %w", err)
	}

	ns, _, err := kubeConfig.Namespace()
	if err != nil {
		ns = "default"
	}
	if namespace != "" {
		ns = namespace
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return client, ns, nil
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls", "get"},
		Short:   "List StoppableContainers",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ns, err := getClient()
			if err != nil {
				return err
			}

			ctx := context.Background()

			var list *unstructured.UnstructuredList
			if allNs {
				list, err = client.Resource(scGVR).List(ctx, metav1.ListOptions{})
			} else {
				list, err = client.Resource(scGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
			}
			if err != nil {
				return fmt.Errorf("failed to list StoppableContainers: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			if allNs {
				_, _ = fmt.Fprintln(w, "NAMESPACE\tNAME\tRUNNING\tPHASE\tAGE")
			} else {
				_, _ = fmt.Fprintln(w, "NAME\tRUNNING\tPHASE\tAGE")
			}

			for _, item := range list.Items {
				name := item.GetName()
				namespace := item.GetNamespace()
				running, _, _ := unstructured.NestedBool(item.Object, "spec", "running")
				phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
				creationTime := item.GetCreationTimestamp()
				age := formatAge(time.Since(creationTime.Time))

				runningStr := "No"
				if running {
					runningStr = "Yes"
				}
				if phase == "" {
					phase = "Pending"
				}

				if allNs {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", namespace, name, runningStr, phase, age)
				} else {
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, runningStr, phase, age)
				}
			}
			return w.Flush()
		},
	}
}

func statusCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show status of a StoppableContainer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			client, ns, err := getClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			sc, err := client.Resource(scGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get StoppableContainer %s: %w", name, err)
			}

			if output == "json" {
				data, _ := json.MarshalIndent(sc.Object, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			if output == "yaml" {
				return runKubectl("get", "stoppablecontainer", name, "-n", ns, "-o", "yaml")
			}

			// Pretty print status
			fmt.Printf("Name:        %s\n", name)
			fmt.Printf("Namespace:   %s\n", ns)

			running, _, _ := unstructured.NestedBool(sc.Object, "spec", "running")
			fmt.Printf("Running:     %v\n", running)

			phase, _, _ := unstructured.NestedString(sc.Object, "status", "phase")
			if phase == "" {
				phase = "Pending"
			}
			fmt.Printf("Phase:       %s\n", phase)

			message, _, _ := unstructured.NestedString(sc.Object, "status", "message")
			if message != "" {
				fmt.Printf("Message:     %s\n", message)
			}

			instanceName, _, _ := unstructured.NestedString(sc.Object, "status", "instanceName")
			if instanceName != "" {
				fmt.Printf("Instance:    %s\n", instanceName)
			}

			nodeName, _, _ := unstructured.NestedString(sc.Object, "status", "nodeName")
			if nodeName != "" {
				fmt.Printf("Node:        %s\n", nodeName)
			}

			// Show conditions
			conditions, found, _ := unstructured.NestedSlice(sc.Object, "status", "conditions")
			if found && len(conditions) > 0 {
				fmt.Println("\nConditions:")
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					condType, _, _ := unstructured.NestedString(cond, "type")
					status, _, _ := unstructured.NestedString(cond, "status")
					reason, _, _ := unstructured.NestedString(cond, "reason")
					fmt.Printf("  %s: %s (%s)\n", condType, status, reason)
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format (json, yaml)")
	return cmd
}

func startCmd() *cobra.Command {
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a StoppableContainer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			client, ns, err := getClient()
			if err != nil {
				return err
			}

			ctx := context.Background()

			// Get current resource
			sc, err := client.Resource(scGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get StoppableContainer %s: %w", name, err)
			}

			// Check if already running
			running, _, _ := unstructured.NestedBool(sc.Object, "spec", "running")
			if running {
				fmt.Printf("StoppableContainer %s is already running\n", name)
				return nil
			}

			// Update to running
			if err := unstructured.SetNestedField(sc.Object, true, "spec", "running"); err != nil {
				return err
			}

			_, err = client.Resource(scGVR).Namespace(ns).Update(ctx, sc, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to start StoppableContainer %s: %w", name, err)
			}

			fmt.Printf("StoppableContainer %s starting...\n", name)

			if wait {
				return waitForPhase(client, ns, name, "Running", timeout)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for container to be running")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Timeout for wait")
	return cmd
}

func stopCmd() *cobra.Command {
	var wait bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a StoppableContainer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			client, ns, err := getClient()
			if err != nil {
				return err
			}

			ctx := context.Background()

			// Get current resource
			sc, err := client.Resource(scGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get StoppableContainer %s: %w", name, err)
			}

			// Check if already stopped
			running, _, _ := unstructured.NestedBool(sc.Object, "spec", "running")
			if !running {
				fmt.Printf("StoppableContainer %s is already stopped\n", name)
				return nil
			}

			// Update to stopped
			if err := unstructured.SetNestedField(sc.Object, false, "spec", "running"); err != nil {
				return err
			}

			_, err = client.Resource(scGVR).Namespace(ns).Update(ctx, sc, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to stop StoppableContainer %s: %w", name, err)
			}

			fmt.Printf("StoppableContainer %s stopping...\n", name)

			if wait {
				return waitForPhase(client, ns, name, "Stopped", timeout)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&wait, "wait", "w", false, "Wait for container to be stopped")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "Timeout for wait")
	return cmd
}

func execCmd() *cobra.Command {
	var stdin bool
	var tty bool
	var container string

	cmd := &cobra.Command{
		Use:   "exec <name> -- <command> [args...]",
		Short: "Execute a command in a StoppableContainer",
		Long: `Execute a command inside the consumer container of a StoppableContainer.

The command runs inside the chroot environment with the container's rootfs.

Examples:
  # Run a shell
  kubectl sc exec my-app -it -- /bin/bash

  # Run a command
  kubectl sc exec my-app -- ls -la /

  # Run with environment variables
  kubectl sc exec my-app -- env`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Find the -- separator
			cmdArgs := []string{}
			for i, arg := range args {
				if arg == "--" && i > 0 {
					cmdArgs = args[i+1:]
					break
				}
			}

			if len(cmdArgs) == 0 {
				return fmt.Errorf("no command specified after --")
			}

			_, ns, err := getClient()
			if err != nil {
				return err
			}

			// Build kubectl exec command
			kubectlArgs := []string{"exec"}
			if stdin {
				kubectlArgs = append(kubectlArgs, "-i")
			}
			if tty {
				kubectlArgs = append(kubectlArgs, "-t")
			}
			kubectlArgs = append(kubectlArgs, "-n", ns)

			podName := name + "-consumer"
			if container != "" {
				kubectlArgs = append(kubectlArgs, "-c", container)
			}
			kubectlArgs = append(kubectlArgs, podName, "--")

			// Use sc-exec wrapper to run commands in the chroot environment
			kubectlArgs = append(kubectlArgs, "/.sc-bin/sc-exec")
			kubectlArgs = append(kubectlArgs, cmdArgs...)

			return runKubectl(kubectlArgs...)
		},
	}
	cmd.Flags().BoolVarP(&stdin, "stdin", "i", false, "Pass stdin to the container")
	cmd.Flags().BoolVarP(&tty, "tty", "t", false, "Stdin is a TTY")
	cmd.Flags().StringVarP(&container, "container", "c", "", "Container name")
	return cmd
}

func logsCmd() *cobra.Command {
	var follow bool
	var tail int64
	var previous bool
	var timestamps bool
	var container string

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Show logs from a StoppableContainer",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			_, ns, err := getClient()
			if err != nil {
				return err
			}

			podName := name + "-consumer"
			kubectlArgs := []string{"logs", "-n", ns}

			if follow {
				kubectlArgs = append(kubectlArgs, "-f")
			}
			if tail > 0 {
				kubectlArgs = append(kubectlArgs, "--tail", fmt.Sprintf("%d", tail))
			}
			if previous {
				kubectlArgs = append(kubectlArgs, "-p")
			}
			if timestamps {
				kubectlArgs = append(kubectlArgs, "--timestamps")
			}
			if container != "" {
				kubectlArgs = append(kubectlArgs, "-c", container)
			}
			kubectlArgs = append(kubectlArgs, podName)

			return runKubectl(kubectlArgs...)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int64Var(&tail, "tail", 0, "Lines of recent log file to display")
	cmd.Flags().BoolVarP(&previous, "previous", "p", false, "Print logs from previous container")
	cmd.Flags().BoolVar(&timestamps, "timestamps", false, "Include timestamps")
	cmd.Flags().StringVarP(&container, "container", "c", "", "Container name")
	return cmd
}

func createCmd() *cobra.Command {
	var image string
	var running bool
	var workingDir string
	var env []string
	var ports []string

	cmd := &cobra.Command{
		Use:   "create <name> [--image=<image>] [-- <command> [args...]]",
		Short: "Create a new StoppableContainer",
		Long: `Create a new StoppableContainer with the specified image and command.

Examples:
  # Create a simple container
  kubectl sc create my-app --image=ubuntu:22.04 -- /bin/bash

  # Create with environment variables
  kubectl sc create my-app --image=nginx:latest -e PORT=8080

  # Create with port mapping
  kubectl sc create my-app --image=nginx:latest -p 80:http`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			if image == "" {
				return fmt.Errorf("--image is required")
			}

			// Find the -- separator for command
			var command []string
			for i, arg := range args {
				if arg == "--" && i > 0 {
					command = args[i+1:]
					break
				}
			}

			if len(command) == 0 {
				command = []string{"/bin/sh", "-c", "sleep infinity"}
			}

			_, ns, err := getClient()
			if err != nil {
				return err
			}

			// Build the YAML
			yaml := buildStoppableContainerYAML(name, ns, image, command, running, workingDir, env, ports)

			// Apply using kubectl
			kubectlCmd := exec.Command("kubectl", "apply", "-f", "-")
			kubectlCmd.Stdin = strings.NewReader(yaml)
			kubectlCmd.Stdout = os.Stdout
			kubectlCmd.Stderr = os.Stderr

			if err := kubectlCmd.Run(); err != nil {
				return fmt.Errorf("failed to create StoppableContainer: %w", err)
			}

			fmt.Printf("StoppableContainer %s created\n", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&image, "image", "", "Container image (required)")
	cmd.Flags().BoolVar(&running, "running", true, "Start container immediately")
	cmd.Flags().StringVarP(&workingDir, "workdir", "w", "", "Working directory")
	cmd.Flags().StringArrayVarP(&env, "env", "e", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().StringArrayVarP(&ports, "port", "p", nil, "Port mappings (port:name)")
	return cmd
}

func deleteCmd() *cobra.Command {
	var force bool
	var wait bool

	cmd := &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm", "remove"},
		Short:   "Delete a StoppableContainer",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			_, ns, err := getClient()
			if err != nil {
				return err
			}

			kubectlArgs := []string{"delete", "stoppablecontainer", name, "-n", ns}
			if force {
				kubectlArgs = append(kubectlArgs, "--force", "--grace-period=0")
			}
			if wait {
				kubectlArgs = append(kubectlArgs, "--wait")
			}

			return runKubectl(kubectlArgs...)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force deletion")
	cmd.Flags().BoolVarP(&wait, "wait", "w", true, "Wait for deletion to complete")
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kubectl-sc version %s\n", version)
		},
	}
}

// Helper functions

func runKubectl(args ...string) error {
	kubectlCmd := exec.Command("kubectl", args...)
	kubectlCmd.Stdin = os.Stdin
	kubectlCmd.Stdout = os.Stdout
	kubectlCmd.Stderr = os.Stderr
	return kubectlCmd.Run()
}

func waitForPhase(client dynamic.Interface, ns, name, targetPhase string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("Waiting for phase %s...\n", targetPhase)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for phase %s", targetPhase)
		case <-ticker.C:
			sc, err := client.Resource(scGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				continue
			}

			phase, _, _ := unstructured.NestedString(sc.Object, "status", "phase")
			if phase == targetPhase {
				fmt.Printf("StoppableContainer %s is now %s\n", name, targetPhase)
				return nil
			}
			if phase == "Failed" {
				message, _, _ := unstructured.NestedString(sc.Object, "status", "message")
				return fmt.Errorf("StoppableContainer failed: %s", message)
			}
		}
	}
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// buildStoppableContainerYAML generates a StoppableContainer YAML manifest.
//
//nolint:lll // Function signature is long but readable
func buildStoppableContainerYAML(
	name, ns, image string,
	command []string,
	running bool,
	workingDir string,
	env, ports []string,
) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`apiVersion: %s
kind: StoppableContainer
metadata:
  name: %s
  namespace: %s
spec:
  running: %v
  template:
    container:
      image: %s
`, GroupVersion, name, ns, running, image))

	if len(command) > 0 {
		sb.WriteString("      command:\n")
		for _, c := range command {
			sb.WriteString(fmt.Sprintf("        - %q\n", c))
		}
	}

	if workingDir != "" {
		sb.WriteString(fmt.Sprintf("      workingDir: %q\n", workingDir))
	}

	if len(env) > 0 {
		sb.WriteString("      env:\n")
		for _, e := range env {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				sb.WriteString(fmt.Sprintf("        - name: %s\n          value: %q\n", parts[0], parts[1]))
			}
		}
	}

	if len(ports) > 0 {
		sb.WriteString("      ports:\n")
		for _, p := range ports {
			parts := strings.SplitN(p, ":", 2)
			if len(parts) == 2 {
				sb.WriteString(fmt.Sprintf("        - containerPort: %s\n          name: %s\n", parts[0], parts[1]))
			} else {
				sb.WriteString(fmt.Sprintf("        - containerPort: %s\n", parts[0]))
			}
		}
	}

	return sb.String()
}
