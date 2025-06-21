/*
Copyright 2025.

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
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hofman-tan/k8s-timed-rolebinding/test/utils"
)

// namespace where the project is deployed in
const namespace = "k8s-timed-rolebinding-system"

// serviceAccountName created for the project
const serviceAccountName = "k8s-timed-rolebinding-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "k8s-timed-rolebinding-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "k8s-timed-rolebinding-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
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

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
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
	SetDefaultConsistentlyPollingInterval(time.Second)

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
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=k8s-timed-rolebinding-metrics-reader",
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

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

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
						"serviceAccount": "%s"
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
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		It("should provisioned cert-manager", func() {
			By("validating that cert-manager has the certificate Secret")
			verifyCertManager := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "secrets", "webhook-server-cert", "-n", namespace)
				_, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
			}
			Eventually(verifyCertManager).Should(Succeed())
		})

		It("should have CA injection for mutating webhooks", func() {
			By("checking CA injection for mutating webhooks")
			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"mutatingwebhookconfigurations.admissionregistration.k8s.io",
					"k8s-timed-rolebinding-mutating-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				mwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(mwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		It("should have CA injection for validating webhooks", func() {
			By("checking CA injection for validating webhooks")
			verifyCAInjection := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"validatingwebhookconfigurations.admissionregistration.k8s.io",
					"k8s-timed-rolebinding-validating-webhook-configuration",
					"-o", "go-template={{ range .webhooks }}{{ .clientConfig.caBundle }}{{ end }}")
				vwhOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(len(vwhOutput)).To(BeNumerically(">", 10))
			}
			Eventually(verifyCAInjection).Should(Succeed())
		})

		It("should create and delete RoleBinding when the TimedRoleBinding is activated and expired", func() {
			const (
				EventuallyTimeout   = time.Second * 20
				ConsistentlyTimeout = time.Second * 10
			)

			By("creating a TimedRoleBinding")
			trbName := "timedrolebinding-sample"
			subjectName := "user1"
			roleName := "role1"
			startTime := time.Now().UTC().Add(time.Minute)
			endTime := startTime.Add(time.Minute)
			keepExpiredFor := time.Minute

			data := TimedRoleBindingManifest{
				Name:           trbName,
				Namespace:      namespace,
				SubjectName:    subjectName,
				RoleName:       roleName,
				StartTime:      startTime.Format(time.RFC3339), // UTC time
				EndTime:        endTime.Format(time.RFC3339),   // UTC time
				KeepExpiredFor: keepExpiredFor.String(),
			}

			manifestFile, err := createTimedRoleBindingManifest(data)
			Expect(err).NotTo(HaveOccurred(), "Failed to create TimedRoleBinding")

			cmd := exec.Command("kubectl", "create", "-f", manifestFile)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create TimedRoleBinding")

			By("waiting for the TimedRoleBinding to be created")
			verifyFunc := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "timedrolebindings", trbName, "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(trbName))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("validating that the TimedRoleBinding is in the Pending phase")
			cmd = exec.Command("kubectl", "get", "timedrolebindings", trbName, "-n", namespace, "-o", "jsonpath={.status.phase}")
			output, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(Equal("Pending"))

			By("validating that RoleBinding is not created yet")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "rolebindings", trbName, "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
				g.Expect(output).To(ContainSubstring("not found"))
			}
			Consistently(verifyFunc, ConsistentlyTimeout).Should(Succeed())

			By("validating that the post-activate job is not created")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobs", trbName+"-post-activate", "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
				g.Expect(output).To(ContainSubstring("not found"))
			}
			Consistently(verifyFunc, ConsistentlyTimeout).Should(Succeed())

			// TODO: use Consistently to probe until startTime
			By("waiting until startTime is reached")
			diff := time.Until(startTime)
			_, _ = fmt.Fprintf(GinkgoWriter, "Sleeping for %d seconds\n", int(diff.Seconds()))
			time.Sleep(diff)

			By("validating that the TimedRoleBinding is in the Active phase")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "timedrolebindings", trbName, "-n", namespace, "-o", "jsonpath={.status.phase}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Active"))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("validating that a RoleBinding is created with the same name as the TimedRoleBinding")
			verifyFunc = func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "rolebindings", trbName, "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(trbName))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("validating that the RoleBinding has the same subjects as the TimedRoleBinding")
			cmd = exec.Command("kubectl", "get", "rolebindings", trbName, "-n", namespace, "-o", "jsonpath={.subjects}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(subjectName))

			By("validating that the RoleBinding has the same roleRef as the TimedRoleBinding")
			cmd = exec.Command("kubectl", "get", "rolebindings", trbName, "-n", namespace, "-o", "jsonpath={.roleRef}")
			output, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).To(ContainSubstring(roleName))

			By("validating that the post-activate job is created")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobs", trbName+"-post-activate", "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(trbName + "-post-activate"))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("validating that the post-expire job is not created")
			cmd = exec.Command("kubectl", "get", "jobs", trbName+"-post-expire", "-n", namespace, "-o", "jsonpath={.metadata.name}")
			output, err = utils.Run(cmd)
			Expect(err).To(HaveOccurred())
			Expect(output).To(ContainSubstring("not found"))

			By("waiting until endTime is reached")
			diff = time.Until(endTime)
			_, _ = fmt.Fprintf(GinkgoWriter, "Sleeping for %d seconds\n", int(diff.Seconds()))
			time.Sleep(diff)

			By("validating that the TimedRoleBinding is in the Expired phase")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "timedrolebindings", trbName, "-n", namespace, "-o", "jsonpath={.status.phase}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Expired"))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("validating that the post-expire job is created")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobs", trbName+"-post-expire", "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err = utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal(trbName + "-post-expire"))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("validating that the RoleBinding is deleted")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "rolebindings", trbName, "-n", namespace, "-o", "jsonpath={.metadata.name}")
				output, err = utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
				g.Expect(output).To(ContainSubstring("not found"))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())

			By("waiting until keepExpiredFor is reached")
			diff = time.Until(endTime.Add(keepExpiredFor))
			_, _ = fmt.Fprintf(GinkgoWriter, "Sleeping for %d seconds\n", int(diff.Seconds()))
			time.Sleep(diff)

			By("validating that the TimedRoleBinding is deleted")
			verifyFunc = func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "timedrolebindings", trbName, "-n", namespace)
				output, err = utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
				g.Expect(output).To(ContainSubstring("not found"))
			}
			Eventually(verifyFunc, EventuallyTimeout).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		// TODO: Customize the e2e test suite with scenarios specific to your project.
		// Consider applying sample/CR(s) and check their status and/or verifying
		// the reconciliation by using the metrics, i.e.:
		// metricsOutput := getMetricsOutput()
		// Expect(metricsOutput).To(ContainSubstring(
		//    fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"} 1`,
		//    strings.ToLower(<Kind>),
		// ))
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
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

// TimedRoleBindingManifest represents the data needed to create a TimedRoleBinding manifest
type TimedRoleBindingManifest struct {
	Name           string
	Namespace      string
	SubjectName    string
	RoleName       string
	StartTime      string
	EndTime        string
	KeepExpiredFor string
}

func createTimedRoleBindingManifest(data TimedRoleBindingManifest) (string, error) {
	const manifestTemplate = `
apiVersion: rbac.hhh.github.io/v1alpha1
kind: TimedRoleBinding
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  subjects:
    - kind: User
      name: {{ .SubjectName }}
      apiGroup: rbac.authorization.k8s.io
  roleRef:
    kind: Role
    name: {{ .RoleName }}
    apiGroup: rbac.authorization.k8s.io
  startTime: {{ .StartTime }}
  endTime: {{ .EndTime }}
  keepExpiredFor: {{ .KeepExpiredFor }}
  postActivate:
    jobTemplate:
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
              - name: post-activate-job
                image: busybox
                command: ["/bin/sh", "-c"]
                args: ["echo $TIMED_ROLE_BINDING_NAME has been activated"]
  postExpire:
    jobTemplate:
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
              - name: post-expire-job
                image: busybox
                command: ["/bin/sh", "-c"]
                args: ["echo $TIMED_ROLE_BINDING_NAME has expired"]
`

	tmpl, err := template.New("manifest").Parse(manifestTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	manifestFile := filepath.Join("/tmp", "timedrolebinding.yaml")
	if err := os.WriteFile(manifestFile, buf.Bytes(), os.FileMode(0o644)); err != nil {
		return "", fmt.Errorf("failed to write manifest file: %w", err)
	}

	return manifestFile, nil
}
