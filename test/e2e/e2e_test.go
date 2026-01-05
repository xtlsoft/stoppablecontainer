//go:build e2e
// +build e2e

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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/xtlsoft/stoppablecontainer/test/utils"
)

// namespace where the project is deployed in
const namespace = "stoppablecontainer-system"

// serviceAccountName created for the project
const serviceAccountName = "stoppablecontainer-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "stoppablecontainer-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "stoppablecontainer-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating or ensuring manager namespace exists")
		cmd := exec.Command("kubectl", "get", "ns", namespace)
		_, err := utils.Run(cmd)
		if err != nil {
			// Namespace doesn't exist, create it
			cmd = exec.Command("kubectl", "create", "ns", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
		}

		By("labeling the namespace to allow privileged workloads (mount-helper requires hostPID, privileged mode)")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=privileged")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with privileged policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("waiting for CRDs to be established")
		verifyCRDsReady := func(g Gomega) {
			cmd := exec.Command("kubectl", "wait", "--for=condition=Established",
				"crd/stoppablecontainers.stoppablecontainer.xtlsoft.top",
				"crd/stoppablecontainerinstances.stoppablecontainer.xtlsoft.top",
				"--timeout=30s")
			_, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred(), "CRDs not established")
		}
		Eventually(verifyCRDsReady, time.Minute, time.Second).Should(Succeed())

		By("deploying the mount-helper DaemonSet")
		cmd = exec.Command("make", "deploy-daemonset", fmt.Sprintf("MOUNT_HELPER_IMG=%s", mountHelperImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy mount-helper DaemonSet")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

		// Set the exec-wrapper image environment variable for E2E tests
		// This overrides the default image with the locally built one
		execWrapperImg := os.Getenv("EXEC_WRAPPER_IMAGE")
		if execWrapperImg == "" {
			execWrapperImg = "stoppablecontainer-exec-wrapper:e2e-test"
		}
		By(fmt.Sprintf("setting exec-wrapper image to %s", execWrapperImg))
		cmd = exec.Command("kubectl", "set", "env", "deployment/stoppablecontainer-controller-manager",
			"-n", namespace, fmt.Sprintf("STOPPABLECONTAINER_EXEC_WRAPPER_IMAGE=%s", execWrapperImg))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to set exec-wrapper image environment variable")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace,
			"--ignore-not-found", "--timeout=30s")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy", "ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("undeploying the mount-helper DaemonSet")
		cmd = exec.Command("make", "undeploy-daemonset", "ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall", "ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace,
			"--ignore-not-found", "--timeout=60s")
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events in system namespace")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events (system namespace):\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching Kubernetes events in default namespace")
			cmd = exec.Command("kubectl", "get", "events", "-n", "default", "--sort-by=.lastTimestamp")
			eventsOutput, err = utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events (default namespace):\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events (default): %s", err)
			}

			By("Fetching mount-helper DaemonSet pod logs")
			cmd = exec.Command("kubectl", "logs", "-l", "app.kubernetes.io/component=mount-helper",
				"-n", namespace, "--tail=100")
			mountHelperLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Mount-helper logs:\n%s", mountHelperLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get mount-helper logs: %s", err)
			}

			By("Fetching provider pod logs")
			cmd = exec.Command("kubectl", "logs", "e2e-test-sc-provider", "-n", "default", "-c", "provider", "--tail=100")
			providerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Provider container logs:\n%s", providerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get provider logs: %s", err)
			}

			By("Fetching provider pod description")
			cmd = exec.Command("kubectl", "describe", "pod", "e2e-test-sc-provider", "-n", "default")
			providerDesc, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Provider pod description:\n%s", providerDesc)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to describe provider pod: %s", err)
			}

			By("Fetching StoppableContainer details")
			cmd = exec.Command("kubectl", "get", "stoppablecontainer", "e2e-test-sc", "-n", "default", "-o", "yaml")
			scDetails, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "StoppableContainer:\n%s", scDetails)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get StoppableContainer: %s", err)
			}

			By("Fetching StoppableContainerInstance details")
			cmd = exec.Command("kubectl", "get", "stoppablecontainerinstance", "e2e-test-sc", "-n", "default", "-o", "yaml")
			sciDetails, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "StoppableContainerInstance:\n%s", sciDetails)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get StoppableContainerInstance: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating or ensuring ClusterRoleBinding for the service account to allow access to metrics")
			// Delete existing clusterrolebinding if it exists
			cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
			_, _ = utils.Run(cmd)
			// Create the clusterrolebinding
			cmd = exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=stoppablecontainer-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", controllerPodName, "-n", namespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted, 3*time.Minute, time.Second).Should(Succeed())

			// +kubebuilder:scaffold:e2e-metrics-webhooks-readiness

			By("deleting any existing curl-metrics pod")
			cmd = exec.Command("kubectl", "delete", "pod", "curl-metrics",
				"-n", namespace, "--ignore-not-found", "--timeout=30s")
			_, _ = utils.Run(cmd) // Ignore errors if pod doesn't exist

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable, 2*time.Minute).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		It("should create and manage a StoppableContainer", func() {
			testNamespace := "default"
			scName := "e2e-test-sc"

			By("creating a StoppableContainer resource")
			scYAML := fmt.Sprintf(`
apiVersion: stoppablecontainer.xtlsoft.top/v1alpha1
kind: StoppableContainer
metadata:
  name: %s
  namespace: %s
spec:
  running: true
  template:
    container:
      image: busybox:stable
      command: ["/bin/sh", "-c", "echo 'E2E test running'; while true; do date; sleep 5; done"]
`, scName, testNamespace)

			cmd := exec.Command("kubectl", "apply", "-f", "-")
			cmd.Stdin = strings.NewReader(scYAML)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create StoppableContainer")

			By("waiting for StoppableContainer to become Running")
			verifyRunning := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "stoppablecontainer", scName,
					"-n", testNamespace, "-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "StoppableContainer not running")
			}
			Eventually(verifyRunning, 3*time.Minute, time.Second).Should(Succeed())

			By("verifying provider pod is running")
			verifyProviderPod := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", fmt.Sprintf("%s-provider", scName),
					"-n", testNamespace, "-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Provider pod not running")
			}
			Eventually(verifyProviderPod, 2*time.Minute, time.Second).Should(Succeed())

			By("verifying consumer pod is running")
			verifyConsumerPod := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", scName,
					"-n", testNamespace, "-o", "jsonpath={.status.phase}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Consumer pod not running")
			}
			Eventually(verifyConsumerPod, 2*time.Minute, time.Second).Should(Succeed())

			By("verifying consumer pod uses only SYS_CHROOT capability (not privileged, no SYS_ADMIN)")
			cmd = exec.Command("kubectl", "get", "pod", scName,
				"-n", testNamespace, "-o", "jsonpath={.spec.containers[0].securityContext.capabilities.add}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring("SYS_CHROOT"), "Consumer should have SYS_CHROOT capability")
			// Verify SYS_ADMIN is NOT present (DaemonSet handles mounts now)
			Expect(output).NotTo(ContainSubstring("SYS_ADMIN"), "Consumer should NOT have SYS_ADMIN capability - DaemonSet handles mounts")

			By("verifying consumer can execute commands")
			verifyConsumerOutput := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", scName,
					"-n", testNamespace, "--tail=10")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("E2E test running"), "Consumer not outputting expected message")
			}
			Eventually(verifyConsumerOutput, time.Minute, time.Second).Should(Succeed())

			By("stopping the StoppableContainer")
			cmd = exec.Command("kubectl", "patch", "stoppablecontainer", scName,
				"-n", testNamespace, "--type=merge", "-p", `{"spec":{"running":false}}`)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to stop StoppableContainer")

			By("waiting for consumer pod to be deleted")
			verifyConsumerDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", scName,
					"-n", testNamespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				// Pod is deleted when we get NotFound error or empty output
				if err != nil && strings.Contains(output, "NotFound") {
					return // Pod is deleted, success
				}
				g.Expect(output).To(BeEmpty(), "Consumer pod should be deleted")
			}
			Eventually(verifyConsumerDeleted, 2*time.Minute, time.Second).Should(Succeed())

			By("verifying provider pod is still running (preserving rootfs)")
			cmd = exec.Command("kubectl", "get", "pod", fmt.Sprintf("%s-provider", scName),
				"-n", testNamespace, "-o", "jsonpath={.status.phase}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("Running"), "Provider pod should still be running")

			By("restarting the StoppableContainer")
			cmd = exec.Command("kubectl", "patch", "stoppablecontainer", scName,
				"-n", testNamespace, "--type=merge", "-p", `{"spec":{"running":true}}`)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to restart StoppableContainer")

			By("waiting for consumer pod to be recreated")
			Eventually(verifyConsumerPod, 2*time.Minute, time.Second).Should(Succeed())

			By("cleaning up the StoppableContainer")
			cmd = exec.Command("kubectl", "delete", "stoppablecontainer", scName, "-n", testNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete StoppableContainer")

			By("waiting for all pods to be deleted")
			verifyPodsDeleted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", "-n", testNamespace,
					"-l", fmt.Sprintf("stoppablecontainer.xtlsoft.top/instance=%s", scName),
					"-o", "jsonpath={.items}")
				output, _ := utils.Run(cmd)
				g.Expect(output).To(Or(BeEmpty(), Equal("[]")), "Pods should be deleted")
			}
			Eventually(verifyPodsDeleted, 2*time.Minute, time.Second).Should(Succeed())
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	return utils.Run(cmd)
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
