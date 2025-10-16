package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	api "github.com/platform9/vjailbreak/pkg/vpwned/api/proto/v1/service"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	DexNamespace = "dex"
	DexConfigMap = "dex-config"
	DexConfigKey = "config.yaml"
)

type idpGRPC struct {
	api.UnimplementedIdentityProviderServer
	k8sClient *kubernetes.Clientset
}

// DexConfig represents the Dex configuration structure
type DexConfig struct {
	Issuer         string                   `yaml:"issuer"`
	Storage        map[string]interface{}   `yaml:"storage"`
	Web            map[string]interface{}   `yaml:"web"`
	Connectors     []map[string]interface{} `yaml:"connectors,omitempty"`
	StaticPasswords []LocalUser             `yaml:"staticPasswords,omitempty"`
	StaticClients  []map[string]interface{} `yaml:"staticClients"`
	OAuth2         map[string]interface{}   `yaml:"oauth2,omitempty"`
	EnablePasswordDB bool                   `yaml:"enablePasswordDB"`
}

// LocalUser represents a local static user
type LocalUser struct {
	Email    string   `yaml:"email" json:"email"`
	Hash     string   `yaml:"hash" json:"hash"`
	Username string   `yaml:"username" json:"username"`
	UserID   string   `yaml:"userID" json:"userID"`
	Groups   []string `yaml:"groups,omitempty" json:"groups,omitempty"`
	Role     string   `yaml:"role,omitempty" json:"role,omitempty"`
}

func newIDPServer() *idpGRPC {
	// Create Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		logrus.Errorf("Failed to get in-cluster config: %v", err)
		return &idpGRPC{}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logrus.Errorf("Failed to create Kubernetes client: %v", err)
		return &idpGRPC{}
	}

	return &idpGRPC{
		k8sClient: clientset,
	}
}

// getDexConfig retrieves the current Dex configuration
func (s *idpGRPC) getDexConfig(ctx context.Context) (*DexConfig, error) {
	cm, err := s.k8sClient.CoreV1().ConfigMaps(DexNamespace).Get(ctx, DexConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	configYAML, ok := cm.Data[DexConfigKey]
	if !ok {
		return nil, fmt.Errorf("config.yaml not found in ConfigMap")
	}

	var dexConfig DexConfig
	if err := yaml.Unmarshal([]byte(configYAML), &dexConfig); err != nil {
		return nil, fmt.Errorf("failed to parse Dex config: %w", err)
	}

	return &dexConfig, nil
}

// updateDexConfig updates the Dex configuration
func (s *idpGRPC) updateDexConfig(ctx context.Context, dexConfig *DexConfig) error {
	cm, err := s.k8sClient.CoreV1().ConfigMaps(DexNamespace).Get(ctx, DexConfigMap, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap: %w", err)
	}

	configYAML, err := yaml.Marshal(dexConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	cm.Data[DexConfigKey] = string(configYAML)

	_, err = s.k8sClient.CoreV1().ConfigMaps(DexNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %w", err)
	}

	return nil
}

// ListProviders lists all configured providers
func (s *idpGRPC) ListProviders(ctx context.Context, req *api.ListProvidersRequest) (*api.ListProvidersResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return nil, err
	}

	var providers []*api.ProviderInfo

	// Convert connectors to providers
	for _, conn := range dexConfig.Connectors {
		providerType, _ := conn["type"].(string)
		providerID, _ := conn["id"].(string)
		providerName, _ := conn["name"].(string)

		configJSON, _ := json.Marshal(conn["config"])

		providers = append(providers, &api.ProviderInfo{
			Id:          providerID,
			Type:        providerType,
			Name:        providerName,
			Description: "",
			Enabled:     true,
			ConfigJson:  string(configJSON),
		})
	}

	// Add local provider if static passwords exist
	if len(dexConfig.StaticPasswords) > 0 {
		usersJSON, _ := json.Marshal(dexConfig.StaticPasswords)
		providers = append(providers, &api.ProviderInfo{
			Id:          "local",
			Type:        "local",
			Name:        "Local Users",
			Description: "Static username/password authentication",
			Enabled:     dexConfig.EnablePasswordDB,
			ConfigJson:  string(usersJSON),
		})
	}

	return &api.ListProvidersResponse{
		Providers: providers,
	}, nil
}

// GetProvider returns a specific provider
func (s *idpGRPC) GetProvider(ctx context.Context, req *api.GetProviderRequest) (*api.GetProviderResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return nil, err
	}

	// Check connectors
	for _, conn := range dexConfig.Connectors {
		if providerID, ok := conn["id"].(string); ok && providerID == req.Id {
			providerType, _ := conn["type"].(string)
			providerName, _ := conn["name"].(string)
			configJSON, _ := json.Marshal(conn["config"])

			return &api.GetProviderResponse{
				Provider: &api.ProviderInfo{
					Id:          providerID,
					Type:        providerType,
					Name:        providerName,
					Description: "",
					Enabled:     true,
					ConfigJson:  string(configJSON),
				},
			}, nil
		}
	}

	// Check local provider
	if req.Id == "local" && len(dexConfig.StaticPasswords) > 0 {
		usersJSON, _ := json.Marshal(dexConfig.StaticPasswords)
		return &api.GetProviderResponse{
			Provider: &api.ProviderInfo{
				Id:          "local",
				Type:        "local",
				Name:        "Local Users",
				Description: "Static username/password authentication",
				Enabled:     dexConfig.EnablePasswordDB,
				ConfigJson:  string(usersJSON),
			},
		}, nil
	}

	return nil, fmt.Errorf("provider not found")
}

// CreateProvider creates a new provider
func (s *idpGRPC) CreateProvider(ctx context.Context, req *api.CreateProviderRequest) (*api.CreateProviderResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return &api.CreateProviderResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Parse config JSON
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(req.Provider.ConfigJson), &config); err != nil {
		return &api.CreateProviderResponse{
			Success: false,
			Message: "Invalid config JSON",
		}, err
	}

	// Create connector
	connector := map[string]interface{}{
		"type":   req.Provider.Type,
		"id":     req.Provider.Id,
		"name":   req.Provider.Name,
		"config": config,
	}

	dexConfig.Connectors = append(dexConfig.Connectors, connector)

	if err := s.updateDexConfig(ctx, dexConfig); err != nil {
		return &api.CreateProviderResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Restart Dex
	go s.restartDexPod(context.Background())

	return &api.CreateProviderResponse{
		Provider: req.Provider,
		Success:  true,
		Message:  "Provider created successfully",
	}, nil
}

// UpdateProvider updates an existing provider
func (s *idpGRPC) UpdateProvider(ctx context.Context, req *api.UpdateProviderRequest) (*api.UpdateProviderResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return &api.UpdateProviderResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Find and update connector
	found := false
	for i, conn := range dexConfig.Connectors {
		if providerID, ok := conn["id"].(string); ok && providerID == req.Id {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(req.Provider.ConfigJson), &config); err != nil {
				return &api.UpdateProviderResponse{
					Success: false,
					Message: "Invalid config JSON",
				}, err
			}

			dexConfig.Connectors[i] = map[string]interface{}{
				"type":   req.Provider.Type,
				"id":     req.Provider.Id,
				"name":   req.Provider.Name,
				"config": config,
			}
			found = true
			break
		}
	}

	if !found {
		return &api.UpdateProviderResponse{
			Success: false,
			Message: "Provider not found",
		}, fmt.Errorf("provider not found")
	}

	if err := s.updateDexConfig(ctx, dexConfig); err != nil {
		return &api.UpdateProviderResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Restart Dex
	go s.restartDexPod(context.Background())

	return &api.UpdateProviderResponse{
		Provider: req.Provider,
		Success:  true,
		Message:  "Provider updated successfully",
	}, nil
}

// DeleteProvider deletes a provider
func (s *idpGRPC) DeleteProvider(ctx context.Context, req *api.DeleteProviderRequest) (*api.DeleteProviderResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return &api.DeleteProviderResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Filter out the provider
	newConnectors := []map[string]interface{}{}
	found := false
	for _, conn := range dexConfig.Connectors {
		if providerID, ok := conn["id"].(string); ok && providerID == req.Id {
			found = true
			continue
		}
		newConnectors = append(newConnectors, conn)
	}

	if !found {
		return &api.DeleteProviderResponse{
			Success: false,
			Message: "Provider not found",
		}, fmt.Errorf("provider not found")
	}

	dexConfig.Connectors = newConnectors

	if err := s.updateDexConfig(ctx, dexConfig); err != nil {
		return &api.DeleteProviderResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Restart Dex
	go s.restartDexPod(context.Background())

	return &api.DeleteProviderResponse{
		Success: true,
		Message: "Provider deleted successfully",
	}, nil
}

// TestProvider tests a provider connection
func (s *idpGRPC) TestProvider(ctx context.Context, req *api.TestProviderRequest) (*api.TestProviderResponse, error) {
	// Get provider details
	provider, err := s.GetProvider(ctx, &api.GetProviderRequest{Id: req.Id})
	if err != nil {
		return &api.TestProviderResponse{
			Success: false,
			Message: "Provider not found",
		}, err
	}

	// Test based on type
	switch provider.Provider.Type {
	case "oidc":
		return s.testOIDCProvider(provider.Provider)
	case "saml":
		return &api.TestProviderResponse{
			Success: true,
			Message: "SAML provider configured (connection test not implemented)",
		}, nil
	case "local":
		return &api.TestProviderResponse{
			Success: true,
			Message: "Local provider is always available",
		}, nil
	default:
		return &api.TestProviderResponse{
			Success: false,
			Message: "Unknown provider type",
		}, nil
	}
}

// testOIDCProvider tests an OIDC provider
func (s *idpGRPC) testOIDCProvider(provider *api.ProviderInfo) (*api.TestProviderResponse, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(provider.ConfigJson), &config); err != nil {
		return &api.TestProviderResponse{
			Success: false,
			Message: "Invalid config",
		}, nil
	}

	issuer, ok := config["issuer"].(string)
	if !ok {
		return &api.TestProviderResponse{
			Success: false,
			Message: "Issuer not found in config",
		}, nil
	}

	// Try to fetch OIDC discovery document
	discoveryURL := issuer + "/.well-known/openid-configuration"
	resp, err := http.Get(discoveryURL)
	if err != nil {
		return &api.TestProviderResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to connect to issuer: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &api.TestProviderResponse{
			Success: false,
			Message: fmt.Sprintf("OIDC discovery returned status %d", resp.StatusCode),
		}, nil
	}

	return &api.TestProviderResponse{
		Success: true,
		Message: "Successfully connected to OIDC provider",
		Details: fmt.Sprintf("Discovery URL: %s", discoveryURL),
	}, nil
}

// RestartDex restarts the Dex pod
func (s *idpGRPC) RestartDex(ctx context.Context, req *api.RestartDexRequest) (*api.RestartDexResponse, error) {
	if err := s.restartDexPod(ctx); err != nil {
		return &api.RestartDexResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	return &api.RestartDexResponse{
		Success: true,
		Message: "Dex pod restarted successfully",
	}, nil
}

// restartDexPod restarts the Dex pod by deleting it
func (s *idpGRPC) restartDexPod(ctx context.Context) error {
	logrus.Info("Restarting Dex pod...")

	pods, err := s.k8sClient.CoreV1().Pods(DexNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=dex",
	})
	if err != nil {
		return fmt.Errorf("failed to list Dex pods: %w", err)
	}

	for _, pod := range pods.Items {
		if err := s.k8sClient.CoreV1().Pods(DexNamespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("Failed to delete pod %s: %v", pod.Name, err)
		} else {
			logrus.Infof("Deleted Dex pod: %s", pod.Name)
		}
	}

	// Wait a bit for pod to restart
	time.Sleep(2 * time.Second)

	return nil
}

// ListLocalUsers lists all local users
func (s *idpGRPC) ListLocalUsers(ctx context.Context, req *api.ListLocalUsersRequest) (*api.ListLocalUsersResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return nil, err
	}

	var users []*api.LocalUserInfo
	for _, user := range dexConfig.StaticPasswords {
		users = append(users, &api.LocalUserInfo{
			Email:    user.Email,
			Username: user.Username,
			Hash:     user.Hash,
			UserId:   user.UserID,
			Groups:   user.Groups,
			Role:     user.Role,
		})
	}

	return &api.ListLocalUsersResponse{
		Users: users,
	}, nil
}

// AddLocalUser adds a new local user
func (s *idpGRPC) AddLocalUser(ctx context.Context, req *api.AddLocalUserRequest) (*api.AddLocalUserResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return &api.AddLocalUserResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Check if user already exists
	for _, user := range dexConfig.StaticPasswords {
		if user.Email == req.User.Email {
			return &api.AddLocalUserResponse{
				Success: false,
				Message: "User already exists",
			}, fmt.Errorf("user already exists")
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.User.Password), 10)
	if err != nil {
		return &api.AddLocalUserResponse{
			Success: false,
			Message: "Failed to hash password",
		}, err
	}

	// Add user
	newUser := LocalUser{
		Email:    req.User.Email,
		Username: req.User.Username,
		Hash:     string(hash),
		UserID:   req.User.UserId,
		Groups:   req.User.Groups,
		Role:     req.User.Role,
	}

	dexConfig.StaticPasswords = append(dexConfig.StaticPasswords, newUser)
	dexConfig.EnablePasswordDB = true

	if err := s.updateDexConfig(ctx, dexConfig); err != nil {
		return &api.AddLocalUserResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Restart Dex
	go s.restartDexPod(context.Background())

	return &api.AddLocalUserResponse{
		User: &api.LocalUserInfo{
			Email:    newUser.Email,
			Username: newUser.Username,
			Hash:     newUser.Hash,
			UserId:   newUser.UserID,
			Groups:   newUser.Groups,
			Role:     newUser.Role,
		},
		Success: true,
		Message: "User added successfully",
	}, nil
}

// UpdateLocalUser updates an existing local user
func (s *idpGRPC) UpdateLocalUser(ctx context.Context, req *api.UpdateLocalUserRequest) (*api.UpdateLocalUserResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return &api.UpdateLocalUserResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Find and update user
	found := false
	for i, user := range dexConfig.StaticPasswords {
		if user.Email == req.Email {
			// Update fields
			if req.User.Username != "" {
				dexConfig.StaticPasswords[i].Username = req.User.Username
			}
			if req.User.Password != "" {
				hash, err := bcrypt.GenerateFromPassword([]byte(req.User.Password), 10)
				if err != nil {
					return &api.UpdateLocalUserResponse{
						Success: false,
						Message: "Failed to hash password",
					}, err
				}
				dexConfig.StaticPasswords[i].Hash = string(hash)
			}
			if len(req.User.Groups) > 0 {
				dexConfig.StaticPasswords[i].Groups = req.User.Groups
			}
			if req.User.Role != "" {
				dexConfig.StaticPasswords[i].Role = req.User.Role
			}
			found = true
			break
		}
	}

	if !found {
		return &api.UpdateLocalUserResponse{
			Success: false,
			Message: "User not found",
		}, fmt.Errorf("user not found")
	}

	if err := s.updateDexConfig(ctx, dexConfig); err != nil {
		return &api.UpdateLocalUserResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Restart Dex
	go s.restartDexPod(context.Background())

	return &api.UpdateLocalUserResponse{
		Success: true,
		Message: "User updated successfully",
	}, nil
}

// DeleteLocalUser deletes a local user
func (s *idpGRPC) DeleteLocalUser(ctx context.Context, req *api.DeleteLocalUserRequest) (*api.DeleteLocalUserResponse, error) {
	dexConfig, err := s.getDexConfig(ctx)
	if err != nil {
		return &api.DeleteLocalUserResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Filter out the user
	newUsers := []LocalUser{}
	found := false
	for _, user := range dexConfig.StaticPasswords {
		if user.Email == req.Email {
			found = true
			continue
		}
		newUsers = append(newUsers, user)
	}

	if !found {
		return &api.DeleteLocalUserResponse{
			Success: false,
			Message: "User not found",
		}, fmt.Errorf("user not found")
	}

	dexConfig.StaticPasswords = newUsers

	if err := s.updateDexConfig(ctx, dexConfig); err != nil {
		return &api.DeleteLocalUserResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}

	// Restart Dex
	go s.restartDexPod(context.Background())

	return &api.DeleteLocalUserResponse{
		Success: true,
		Message: "User deleted successfully",
	}, nil
}
