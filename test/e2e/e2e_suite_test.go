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
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/xtlsoft/stoppablecontainer/test/utils"
)

var (
	// Optional Environment Variables:
	// - CERT_MANAGER_INSTALL_SKIP=true: Skips CertManager installation during test setup.
	// - DOCKER_BUILD_SKIP=true: Skips Docker image build and Kind loading during test setup.
	// - PROJECT_IMAGE=<image>: Override the default project image.
	// - MOUNT_HELPER_IMAGE=<image>: Override the default mount-helper image.
	// - EXEC_WRAPPER_IMAGE=<image>: Override the default exec-wrapper image.
	// These variables are useful if CertManager is already installed or image is pre-built,
	// avoiding re-installation and conflicts.
	skipCertManagerInstall = os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true"
	skipDockerBuild        = os.Getenv("DOCKER_BUILD_SKIP") == "true"
	// isCertManagerAlreadyInstalled will be set true when CertManager CRDs be found on the cluster
	isCertManagerAlreadyInstalled = false

	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	projectImage = getEnvOrDefault("PROJECT_IMAGE", "stoppablecontainer:e2e-test")

	// mountHelperImage is the name of the mount-helper DaemonSet image
	mountHelperImage = getEnvOrDefault("MOUNT_HELPER_IMAGE", "stoppablecontainer-mount-helper:e2e-test")

	// execWrapperImage is the name of the exec-wrapper image (used for pause binary and consumer chroot)
	execWrapperImage = getEnvOrDefault("EXEC_WRAPPER_IMAGE", "stoppablecontainer-exec-wrapper:e2e-test")
)

func getEnvOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purpose of being used in CI jobs.
// The default setup requires Kind, builds/loads the Manager Docker image locally, and installs
// CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting stoppablecontainer integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	if !skipDockerBuild {
		By("building the manager(Operator) image")
		cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectImage))
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager(Operator) image")

		By("building the mount-helper DaemonSet image")
		cmd = exec.Command("make", "docker-build-mount-helper", fmt.Sprintf("MOUNT_HELPER_IMG=%s", mountHelperImage))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the mount-helper image")

		By("building the exec-wrapper image")
		cmd = exec.Command("make", "docker-build-exec-wrapper", fmt.Sprintf("EXEC_WRAPPER_IMG=%s", execWrapperImage))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the exec-wrapper image")

		// TODO(user): If you want to change the e2e test vendor from Kind, ensure the image is
		// built and available before running the tests. Also, remove the following block.
		By("loading the manager(Operator) image on Kind")
		err = utils.LoadImageToKindClusterWithName(projectImage)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into Kind")

		By("loading the mount-helper image on Kind")
		err = utils.LoadImageToKindClusterWithName(mountHelperImage)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the mount-helper image into Kind")

		By("loading the exec-wrapper image on Kind")
		err = utils.LoadImageToKindClusterWithName(execWrapperImage)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the exec-wrapper image into Kind")
	} else {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping Docker build (DOCKER_BUILD_SKIP=true)\n")
	}

	// Pre-pull and load required images for provider/consumer pods
	// These images are needed by provider pods and may not be available in Kind by default
	requiredImages := []string{
		"alpine:latest",  // Provider container
		"busybox:stable", // Test user image (rootfs container)
	}

	for _, img := range requiredImages {
		By(fmt.Sprintf("pulling image %s", img))
		cmd := exec.Command("docker", "pull", img)
		_, err := utils.Run(cmd)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Warning: failed to pull %s: %v\n", img, err)
			continue
		}

		By(fmt.Sprintf("loading image %s on Kind", img))
		err = utils.LoadImageToKindClusterWithName(img)
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Warning: failed to load %s to Kind: %v\n", img, err)
		}
	}

	// The tests-e2e are intended to run on a temporary cluster that is created and destroyed for testing.
	// To prevent errors when tests run in environments with CertManager already installed,
	// we check for its presence before execution.
	// Setup CertManager before the suite if not skipped and if not already installed
	if !skipCertManagerInstall {
		By("checking if cert manager is installed already")
		isCertManagerAlreadyInstalled = utils.IsCertManagerCRDsInstalled()
		if !isCertManagerAlreadyInstalled {
			_, _ = fmt.Fprintf(GinkgoWriter, "Installing CertManager...\n")
			Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: CertManager is already installed. Skipping installation...\n")
		}
	}
})

var _ = AfterSuite(func() {
	// Teardown CertManager after the suite if not skipped and if it was not already installed
	if !skipCertManagerInstall && !isCertManagerAlreadyInstalled {
		_, _ = fmt.Fprintf(GinkgoWriter, "Uninstalling CertManager...\n")
		utils.UninstallCertManager()
	}
})
