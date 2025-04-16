package handlers

import (
	"context"
	"fmt"
	"net/http"

	v1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	"github.com/platform9/vjailbreak/ui-proxy/pkg/types"
	"github.com/platform9/vjailbreak/ui-proxy/pkg/utils"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProxyServer struct {
	kubeClient client.Client
}

func NewProxyServer(kubeClient client.Client) *ProxyServer {
	return &ProxyServer{kubeClient: kubeClient}
}

func (s *ProxyServer) HandleOpenStackProxy(c *gin.Context) {
	var req types.ProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if req.CredentialName == "" || req.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	creds, err := s.getOpenStackCreds(req.CredentialName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := utils.GetOpenStackToken(creds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.ProxyRequestToEndpoint(c, req, token, creds.OSAuthURL)
}

func (s *ProxyServer) HandleVMwareProxy(c *gin.Context) {
	var req types.ProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	if req.CredentialName == "" || req.Endpoint == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields"})
		return
	}

	creds, err := s.getVMwareCreds(req.CredentialName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := utils.GetVMwareToken(creds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.ProxyRequestToEndpoint(c, req, token, creds.VcenterURL)
}

func (s *ProxyServer) getOpenStackCreds(name string) (types.OpenStackSecretData, error) {
	var openstackCred v1.OpenstackCreds
	if err := s.kubeClient.Get(context.Background(), k8stypes.NamespacedName{
		Name: name, Namespace: "vjailbreak-system",
	}, &openstackCred); err != nil {
		return types.OpenStackSecretData{}, fmt.Errorf("failed to get OpenstackCreds: %w", err)
	}

	var secret corev1.Secret
	if err := s.kubeClient.Get(context.Background(), openstackCred.Spec.SecretRef, &secret); err != nil {
		return types.OpenStackSecretData{}, fmt.Errorf("failed to get secret: %w", err)
	}

	return types.OpenStackSecretData{
		OSAuthURL:    string(secret.Data["OS_AUTH_URL"]),
		OSUsername:   string(secret.Data["OS_USERNAME"]),
		OSPassword:   string(secret.Data["OS_PASSWORD"]),
		OSDomainName: string(secret.Data["OS_USER_DOMAIN_NAME"]),
		OSInsecure:   string(secret.Data["OS_INSECURE"]),
		OSRegionName: string(secret.Data["OS_REGION_NAME"]),
		OSTenantName: string(secret.Data["OS_TENANT_NAME"]),
	}, nil
}

func (s *ProxyServer) getVMwareCreds(name string) (types.VsphereSecretData, error) {
	var vmwareCred v1.VMwareCreds
	if err := s.kubeClient.Get(context.Background(), k8stypes.NamespacedName{
		Name: name, Namespace: "vjailbreak-system",
	}, &vmwareCred); err != nil {
		return types.VsphereSecretData{}, fmt.Errorf("failed to get VMwareCreds: %w", err)
	}

	var secret corev1.Secret
	if err := s.kubeClient.Get(context.Background(), vmwareCred.Spec.SecretRef, &secret); err != nil {
		return types.VsphereSecretData{}, fmt.Errorf("failed to get secret: %w", err)
	}

	return types.VsphereSecretData{
		VcenterURL:        string(secret.Data["VCENTER_HOST"]),
		VcenterUsername:   string(secret.Data["VCENTER_USERNAME"]),
		VcenterPassword:   string(secret.Data["VCENTER_PASSWORD"]),
		VcenterDatacenter: string(secret.Data["VCENTER_DATACENTER"]),
	}, nil
}
