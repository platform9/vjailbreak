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

package e2e

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/platform9/vjailbreak/k8s/migration/test/utils"
)

const namespace = "migration-system"

var _ = ginkgo.Describe("controller", ginkgo.Ordered, func() {
	ginkgo.BeforeAll(func() {
		ginkgo.By("installing prometheus operator")
		gomega.Expect(utils.InstallPrometheusOperator()).To(gomega.Succeed())

		ginkgo.By("installing the cert-manager")
		gomega.Expect(utils.InstallCertManager()).To(gomega.Succeed())

		ginkgo.By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	ginkgo.AfterAll(func() {
		ginkgo.By("uninstalling the Prometheus manager bundle")
		utils.UninstallPrometheusOperator()

		ginkgo.By("uninstalling the cert-manager bundle")
		utils.UninstallCertManager()

		ginkgo.By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	ginkgo.Context("Operator", func() {
		ginkgo.It("should run successfully", func() {
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/migration:v0.0.1"

			ginkgo.By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage, "kind")
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			gomega.ExpectWithOffset(1, err).NotTo(gomega.HaveOccurred())

			ginkgo.By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				gomega.ExpectWithOffset(2, controllerPodName).Should(gomega.ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				gomega.ExpectWithOffset(2, err).NotTo(gomega.HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			gomega.Eventually(verifyControllerUp).WithTimeout(time.Minute).WithPolling(time.Second).Should(gomega.Succeed())

		})
	})
})
