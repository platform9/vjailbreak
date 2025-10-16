package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// APIEndpoint represents an API endpoint to test
type APIEndpoint struct {
	Name        string
	Method      string
	Path        string
	Description string
	RequiresAuth bool
}

// TestResult represents the result of testing an endpoint
type TestResult struct {
	Endpoint    APIEndpoint
	StatusCode  int
	Success     bool
	Error       string
	ResponseTime time.Duration
	Timestamp   time.Time
}

// Report represents the complete test report
type Report struct {
	TotalTests    int
	SuccessCount  int
	FailureCount  int
	TestResults   []TestResult
	GeneratedAt   time.Time
	ClusterHost   string
}

var (
	// Base URLs
	baseURL    = getEnv("BASE_URL", "https://10.9.2.145")
	skipSSL    = getEnv("SKIP_SSL_VERIFY", "true") == "true"
	httpClient *http.Client
	saToken    string // Cached service account token
)

func init() {
	// Create HTTP client with optional SSL skip
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipSSL,
		},
	}
	httpClient = &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	
	// Pre-load service account token
	token, err := getServiceAccountToken()
	if err == nil {
		saToken = token
		fmt.Printf("Service account token loaded (length: %d)\n", len(saToken))
	} else {
		fmt.Printf("Warning: No service account token available: %v\n", err)
		fmt.Printf("Set SA_TOKEN environment variable or run inside K8s cluster\n")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Define all API endpoints to test
func getAPIEndpoints() []APIEndpoint {
	return []APIEndpoint{
		// Kubernetes Core API endpoints
		{
			Name:        "List Namespaces",
			Method:      "GET",
			Path:        "/api/v1/namespaces",
			Description: "Kubernetes API - List namespaces",
			RequiresAuth: true,
		},
		
		// VJailbreak CRD endpoints (use /apis/vjailbreak.k8s.pf9.io/v1alpha1/)
		{
			Name:        "List VJailbreakNodes",
			Method:      "GET",
			Path:        "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/vjailbreaknodes",
			Description: "VJailbreak CRD - List nodes",
			RequiresAuth: true,
		},
		{
			Name:        "List VJailbreakMigrations",
			Method:      "GET",
			Path:        "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/migrations",
			Description: "VJailbreak CRD - List migrations",
			RequiresAuth: true,
		},
		{
			Name:        "List VMwareCredentials",
			Method:      "GET",
			Path:        "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/vmwarecreds",
			Description: "VJailbreak CRD - List VMware credentials",
			RequiresAuth: true,
		},
		{
			Name:        "List OpenStackCredentials",
			Method:      "GET",
			Path:        "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/openstackcreds",
			Description: "VJailbreak CRD - List OpenStack credentials",
			RequiresAuth: true,
		},
		{
			Name:        "List ClusterMigrations",
			Method:      "GET",
			Path:        "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/clustermigrations",
			Description: "VJailbreak CRD - List cluster migrations",
			RequiresAuth: true,
		},
		{
			Name:        "List ESXiMigrations",
			Method:      "GET",
			Path:        "/apis/vjailbreak.k8s.pf9.io/v1alpha1/namespaces/migration-system/esximigrations",
			Description: "VJailbreak CRD - List ESXi migrations",
			RequiresAuth: true,
		},
		
		// vpwned SDK endpoints (gRPC-Gateway)
		{
			Name:        "SDK Version",
			Method:      "GET",
			Path:        "/dev-api/sdk/vpw/v1/version",
			Description: "SDK - Get version",
			RequiresAuth: false,
		},
		{
			Name:        "List Identity Providers",
			Method:      "GET",
			Path:        "/dev-api/sdk/vpw/v1/idp/providers",
			Description: "SDK IDP - List providers",
			RequiresAuth: false,
		},
		{
			Name:        "List Local Users",
			Method:      "GET",
			Path:        "/dev-api/sdk/vpw/v1/idp/local/users",
			Description: "SDK IDP - List local users",
			RequiresAuth: false,
		},
		{
			Name:        "Validate OpenStack IP",
			Method:      "POST",
			Path:        "/dev-api/sdk/vpw/v1/validate_openstack_ip",
			Description: "SDK - Validate OpenStack IP",
			RequiresAuth: false,
		},
		
		// OAuth2 Proxy endpoints
		{
			Name:        "OAuth2 Auth Check",
			Method:      "GET",
			Path:        "/oauth2/auth",
			Description: "OAuth2 Proxy - Auth check",
			RequiresAuth: false,
		},
		{
			Name:        "OAuth2 User Info",
			Method:      "GET",
			Path:        "/oauth2/userinfo",
			Description: "OAuth2 Proxy - User info",
			RequiresAuth: true,
		},
		
		// UI Static endpoints
		{
			Name:        "UI Root",
			Method:      "GET",
			Path:        "/",
			Description: "UI - Root page",
			RequiresAuth: false,
		},
		{
			Name:        "UI Dashboard",
			Method:      "GET",
			Path:        "/dashboard",
			Description: "UI - Dashboard page",
			RequiresAuth: true,
		},
	}
}

// testEndpoint tests a single API endpoint
func testEndpoint(endpoint APIEndpoint) TestResult {
	result := TestResult{
		Endpoint:  endpoint,
		Timestamp: time.Now(),
	}

	url := baseURL + endpoint.Path
	startTime := time.Now()

	// Create request
	var req *http.Request
	var err error

	if endpoint.Method == "POST" {
		// For POST requests, send empty JSON body
		req, err = http.NewRequest(endpoint.Method, url, strings.NewReader("{}"))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
		}
	} else {
		req, err = http.NewRequest(endpoint.Method, url, nil)
	}

	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		result.Success = false
		return result
	}

	// Add service account token for Kubernetes API endpoints
	// OAuth2 endpoints need session cookie auth, not SA token
	if endpoint.RequiresAuth || strings.HasPrefix(endpoint.Path, "/api/") || strings.HasPrefix(endpoint.Path, "/apis/") {
		token, err := getServiceAccountToken()
		if err == nil && token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		}
	}

	// Execute request
	resp, err := httpClient.Do(req)
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Success = false
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode

	// Read response body
	body, _ := io.ReadAll(resp.Body)

	// Determine success based on status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		result.Success = true
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		if len(result.Error) > 200 {
			result.Error = result.Error[:200] + "..."
		}
	}

	return result
}

// getServiceAccountToken reads the service account token
// First checks SA_TOKEN env variable, then falls back to mounted token file
func getServiceAccountToken() (string, error) {
	// Check environment variable first
	if token := os.Getenv("SA_TOKEN"); token != "" {
		return token, nil
	}
	
	// Fall back to reading from mounted service account token
	tokenPath := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	token, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}
	return string(token), nil
}

// runTests executes all API tests
func runTests() Report {
	endpoints := getAPIEndpoints()
	report := Report{
		TotalTests:  len(endpoints),
		GeneratedAt: time.Now(),
		ClusterHost: baseURL,
		TestResults: make([]TestResult, 0, len(endpoints)),
	}

	fmt.Printf("Starting API Health Check...\n")
	fmt.Printf("Testing %d endpoints on %s\n", len(endpoints), baseURL)
	fmt.Printf("SSL Verification: %v\n\n", !skipSSL)

	for i, endpoint := range endpoints {
		fmt.Printf("[%d/%d] Testing: %s %s\n", i+1, len(endpoints), endpoint.Method, endpoint.Path)
		
		result := testEndpoint(endpoint)
		report.TestResults = append(report.TestResults, result)

		if result.Success {
			report.SuccessCount++
			fmt.Printf("  ✓ SUCCESS - %d - %dms\n", result.StatusCode, result.ResponseTime.Milliseconds())
		} else {
			report.FailureCount++
			fmt.Printf("  ✗ FAILED - %s\n", result.Error)
		}
		fmt.Println()
	}

	return report
}

// printReport prints a human-readable report
func printReport(report Report) {
	fmt.Println("=====================================")
	fmt.Println("API HEALTH CHECK REPORT")
	fmt.Println("=====================================")
	fmt.Printf("Generated At: %s\n", report.GeneratedAt.Format(time.RFC3339))
	fmt.Printf("Cluster Host: %s\n", report.ClusterHost)
	fmt.Printf("Total Tests:  %d\n", report.TotalTests)
	fmt.Printf("Successes:    %d (%.1f%%)\n", report.SuccessCount, float64(report.SuccessCount)/float64(report.TotalTests)*100)
	fmt.Printf("Failures:     %d (%.1f%%)\n", report.FailureCount, float64(report.FailureCount)/float64(report.TotalTests)*100)
	fmt.Println("=====================================")
	fmt.Println()

	// Print failures in detail
	if report.FailureCount > 0 {
		fmt.Println("FAILED ENDPOINTS:")
		fmt.Println("-------------------------------------")
		for _, result := range report.TestResults {
			if !result.Success {
				fmt.Printf("✗ %s\n", result.Endpoint.Name)
				fmt.Printf("  Method: %s\n", result.Endpoint.Method)
				fmt.Printf("  Path:   %s\n", result.Endpoint.Path)
				fmt.Printf("  Error:  %s\n", result.Error)
				fmt.Println()
			}
		}
	}

	// Print successes summary
	fmt.Println("SUCCESSFUL ENDPOINTS:")
	fmt.Println("-------------------------------------")
	for _, result := range report.TestResults {
		if result.Success {
			fmt.Printf("✓ %s - HTTP %d (%dms)\n", 
				result.Endpoint.Name, 
				result.StatusCode, 
				result.ResponseTime.Milliseconds())
		}
	}
	fmt.Println()
}

// saveJSONReport saves the report as JSON
func saveJSONReport(report Report, filename string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func main() {
	// Run tests
	report := runTests()

	// Print report to stdout
	printReport(report)

	// Save JSON report
	jsonFile := getEnv("REPORT_FILE", "/tmp/api-health-report.json")
	if err := saveJSONReport(report, jsonFile); err != nil {
		fmt.Printf("Warning: Failed to save JSON report: %v\n", err)
	} else {
		fmt.Printf("JSON report saved to: %s\n", jsonFile)
	}

	// Exit with error code if any tests failed
	if report.FailureCount > 0 {
		os.Exit(1)
	}
}
