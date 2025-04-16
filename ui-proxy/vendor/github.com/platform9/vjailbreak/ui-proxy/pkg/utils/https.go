package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/platform9/vjailbreak/ui-proxy/pkg/types"
)

func ProxyRequestToEndpoint(c *gin.Context, req types.ProxyRequest, token, baseURL string) {
	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	// Build the full endpoint
	targetURL, err := buildFullURL(baseURL, req.Endpoint)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL: " + err.Error()})
		return
	}

	var requestBody io.Reader
	if req.Data != nil {
		bodyBytes, _ := json.Marshal(req.Data)
		requestBody = bytes.NewReader(bodyBytes)
	}

	proxyReq, err := http.NewRequest(method, targetURL, requestBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("X-Auth-Token", token)
	proxyReq.Header.Set("vmware-api-session-id", token)

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Header(k, v)
		}
	}
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

func buildFullURL(base, endpoint string) (string, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if endpointURL.IsAbs() {
		return endpointURL.String(), nil
	}
	return baseURL.ResolveReference(endpointURL).String(), nil
}
