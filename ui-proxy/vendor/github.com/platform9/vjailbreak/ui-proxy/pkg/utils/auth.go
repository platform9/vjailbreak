package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/ui-proxy/pkg/types"
)

func GetOpenStackToken(creds types.OpenStackSecretData) (string, error) {
	authURL := strings.TrimSuffix(creds.OSAuthURL, "/") + "/v3/auth/tokens"

	payload := map[string]interface{}{
		"auth": map[string]interface{}{
			"identity": map[string]interface{}{
				"methods": []string{"password"},
				"password": map[string]interface{}{
					"user": map[string]interface{}{
						"name":     creds.OSUsername,
						"password": creds.OSPassword,
						"domain":   map[string]interface{}{"name": creds.OSDomainName},
					},
				},
			},
			"scope": map[string]interface{}{
				"project": map[string]interface{}{
					"name":   creds.OSTenantName,
					"domain": map[string]interface{}{"name": creds.OSDomainName},
				},
			},
		},
	}
	data, _ := json.Marshal(payload)

	req, _ := http.NewRequest(http.MethodPost, authURL, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("OpenStack auth failed: %s", string(body))
	}

	return resp.Header.Get("X-Subject-Token"), nil
}

func GetVMwareToken(creds types.VsphereSecretData) (string, error) {
	url := strings.TrimSuffix(creds.VcenterURL, "/") + "/rest/com/vmware/cis/session"
	req, _ := http.NewRequest(http.MethodPost, url, nil)
	req.SetBasicAuth(creds.VcenterUsername, creds.VcenterPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Value string `json:"value"`
	}
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse VMware token: %v", err)
	}
	return result.Value, nil
}
