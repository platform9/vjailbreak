package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run update-dex-password.go <new-password>")
		fmt.Println("Example: go run update-dex-password.go admin")
		os.Exit(1)
	}

	password := os.Args[1]

	// Generate bcrypt hash using Go's bcrypt library (same as Dex)
	fmt.Printf("Generating bcrypt hash for password: %s\n", password)
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to generate bcrypt hash: %v", err)
	}

	hashString := string(hash)
	fmt.Printf("Generated hash: %s\n", hashString)

	// Verify the hash works
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		log.Fatalf("Hash verification failed: %v", err)
	}
	fmt.Println("✓ Hash verification successful")

	// Load kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("Failed to load kubeconfig: %v", err)
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	ctx := context.Background()
	namespace := "dex"
	configMapName := "dex-config"

	// Get the ConfigMap
	fmt.Printf("\nFetching ConfigMap: %s/%s\n", namespace, configMapName)
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Failed to get ConfigMap: %v", err)
	}

	// Get the current config
	configYaml, ok := cm.Data["config.yaml"]
	if !ok {
		log.Fatal("config.yaml not found in ConfigMap")
	}

	// Simple string replacement - find the hash line and replace it
	// This is a simple approach - in production you'd use a YAML parser
	fmt.Println("\nUpdating password hash in config...")
	
	// Read line by line and replace the hash
	lines := []string{}
	inStaticPasswords := false
	for i, line := range splitLines(configYaml) {
		if contains(line, "staticPasswords:") {
			inStaticPasswords = true
		}
		
		// Look for hash line within staticPasswords section
		if inStaticPasswords && contains(line, "hash:") && contains(line, "$2") {
			// Replace the hash
			indent := getIndent(line)
			lines = append(lines, fmt.Sprintf("%shash: \"%s\"", indent, hashString))
			fmt.Printf("Updated line %d: hash: \"%s\"\n", i+1, hashString)
		} else {
			lines = append(lines, line)
		}
	}

	// Update the ConfigMap
	cm.Data["config.yaml"] = joinLines(lines)

	fmt.Printf("\nUpdating ConfigMap %s/%s...\n", namespace, configMapName)
	_, err = clientset.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		log.Fatalf("Failed to update ConfigMap: %v", err)
	}

	fmt.Println("✓ ConfigMap updated successfully")
	fmt.Println("\nNow restart the Dex pod:")
	fmt.Printf("  kubectl delete pod -n %s -l app=dex\n", namespace)
	fmt.Printf("\nNew credentials:\n")
	fmt.Printf("  Email: admin@vjailbreak.local\n")
	fmt.Printf("  Password: %s\n", password)
}

func splitLines(s string) []string {
	lines := []string{}
	current := ""
	for _, c := range s {
		if c == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func getIndent(line string) string {
	indent := ""
	for _, c := range line {
		if c == ' ' || c == '\t' {
			indent += string(c)
		} else {
			break
		}
	}
	return indent
}
