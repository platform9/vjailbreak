package cmd

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

const podYamlTemplate = `apiVersion: v1
kind: Pod
metadata:
  name: v2v-helper
spec:
  restartPolicy: Never
  containers:
  - name: fedora
    securityContext:
      privileged: true
    image: tanaypf9/v2v:latest
    imagePullPolicy: Always
    command:
    - /home/fedora/manager
    envFrom:
    - configMapRef:
        name: migration-config
    volumeMounts:
    - mountPath: /home/fedora/vmware-vix-disklib-distrib
      name: vddk
    - mountPath: /dev
      name: dev
  volumes:
  - name: vddk
    hostPath:
      path: /home/ubuntu/vmware-vix-disklib-distrib
      type: Directory
  - name: dev
    hostPath:
      path: /dev
      type: Directory
`

type ConfigMap struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
}

func randSeq(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate Kubernetes Pod and ConfigMap YAML",
	Long: `Generate Kubernetes Pod and ConfigMap YAML based on user input and admin.rc file. 
Your admin.rc file should atleast contain the following keys: OS_AUTH_URL, OS_DOMAIN_NAME, OS_TENANT_NAME, OS_USERNAME, OS_PASSWORD.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if kubectl exists
		_, err := exec.LookPath("kubectl")
		if err != nil {
			log.Fatal("kubectl not found. Please make sure it is installed and in your PATH.")
		}
		// Create the ConfigMap structure
		var configMap ConfigMap
		configMap.APIVersion = "v1"
		configMap.Kind = "ConfigMap"
		configMap.Metadata.Name = "migration-config"
		configMap.Data = make(map[string]string)

		envVars := []string{"VCENTER_USERNAME", "VCENTER_PASSWORD", "VCENTER_HOST", "SOURCE_VM_NAME", "CONVERT", "VCENTER_INSECURE", "NEUTRON_NETWORK_ID", "OS_TYPE"}
		for _, env := range envVars {
			var value string
			fmt.Printf("Enter value for %s", env)
			if env == "VCENTER_PASSWORD" {
				fmt.Printf(": ")
				password, _ := term.ReadPassword(0)
				value = string(password)
				fmt.Println()
			} else if env == "VCENTER_INSECURE" || env == "CONVERT" {
				fmt.Printf(" (true/false):")
				fmt.Scanln(&value)
				if strings.ToLower(value) == "true" || strings.ToLower(value) == "t" {
					value = "true"
				} else if strings.ToLower(value) == "false" || strings.ToLower(value) == "f" {
					value = "false"
				}
			} else if env == "OS_TYPE" {
				fmt.Printf(" (Windows/Linux): ")
				fmt.Scanln(&value)
			} else {
				fmt.Printf(": ")
				fmt.Scanln(&value)
			}
			configMap.Data[env] = value
		}

		randsequence := randSeq(5)

		configMap.Metadata.Name = configMap.Metadata.Name + "-" + randsequence

		adminFile := "admin.rc"
		adminConfig, err := parseAdminFile(adminFile)
		if err != nil {
			log.Fatalf("Error reading admin file: %v", err)
		}

		openStackEnvVars := []string{"OS_AUTH_URL", "OS_DOMAIN_NAME", "OS_TENANT_NAME", "OS_USERNAME", "OS_PASSWORD"}
		for _, env := range openStackEnvVars {
			value, ok := adminConfig[env]
			if !ok {
				log.Fatalf("Missing key %s in admin file", env)
			}
			configMap.Data[env] = value
		}

		// Marshal and write the ConfigMap YAML
		configMapYamlData, err := yaml.Marshal(&configMap)
		if err != nil {
			log.Fatalf("Error marshalling ConfigMap to YAML: %v", err)
		}

		configMapOutputFile := "configmap.yaml"
		if err := os.WriteFile(configMapOutputFile, configMapYamlData, 0644); err != nil {
			log.Fatalf("Error writing ConfigMap YAML file: %v", err)
		}

		// Update the Pod YAML template with the correct ConfigMap name
		podYaml := podYamlTemplate
		podYaml = strings.Replace(podYaml, "migration-config", configMap.Metadata.Name, -1)
		podYaml = strings.Replace(podYaml, "v2v-helper", "v2v-helper"+"-"+randsequence, -1)

		// Write the Pod YAML to a file
		podOutputFile := "pod.yaml"
		if err := os.WriteFile(podOutputFile, []byte(podYaml), 0644); err != nil {
			log.Fatalf("Error writing Pod YAML file: %v", err)
		}

		fmt.Printf("YAML files created successfully: %s, %s\n", podOutputFile, configMapOutputFile)

		// Apply the generated YAML files using kubectl
		applyCmd := exec.Command("kubectl", "apply", "-f", "configmap.yaml", "-f", "pod.yaml")
		applyCmd.Stdout = os.Stdout
		applyCmd.Stderr = os.Stderr
		if err := applyCmd.Run(); err != nil {
			log.Fatalf("Error applying YAML files: %v", err)
		}
		fmt.Println("YAML files applied successfully.")
	},
}

func parseAdminFile(filename string) (map[string]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	envVars := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "export ") {
			parts := strings.SplitN(line[7:], "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := strings.Trim(parts[1], "\"")
				envVars[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return envVars, nil
}
