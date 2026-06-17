package server

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	sshKeyPairLabelResourceType = "vjailbreak.k8s.pf9.io/resource-type"
	sshKeyPairLabelKeypairType  = "vjailbreak.k8s.pf9.io/keypair-type"
	sshKeyPairResourceTypeValue = "ssh-keypair"
	sshKeyPairTypeGenerated     = "generated"
)

type generateSSHKeyPairRequest struct {
	Name string `json:"name"`
}

type generateSSHKeyPairResponse struct {
	PublicKey string `json:"publicKey"`
}

func HandleGenerateSSHKeyPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if err := validateUIToken(token); err != nil {
		logrus.Warnf("generate-ssh-keypair: rejected request: %v", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req generateSSHKeyPairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "invalid request: name is required", http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(req.Name)

	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		logrus.Errorf("generate-ssh-keypair: key generation failed: %v", err)
		http.Error(w, "failed to generate key pair", http.StatusInternalServerError)
		return
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		logrus.Errorf("generate-ssh-keypair: public key marshal failed: %v", err)
		http.Error(w, "failed to encode public key", http.StatusInternalServerError)
		return
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(pub)

	if k8sAuthClient == nil {
		http.Error(w, "k8s client not initialized", http.StatusServiceUnavailable)
		return
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: migrationSystemNamespace,
			Labels: map[string]string{
				sshKeyPairLabelResourceType: sshKeyPairResourceTypeValue,
				sshKeyPairLabelKeypairType:  sshKeyPairTypeGenerated,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"ssh-privatekey": privateKeyPEM,
			"ssh-publickey":  publicKeyBytes,
		},
	}

	if _, err := k8sAuthClient.CoreV1().Secrets(migrationSystemNamespace).Create(
		context.Background(), secret, metav1.CreateOptions{},
	); err != nil {
		logrus.Errorf("generate-ssh-keypair: failed to create secret %q: %v", name, err)
		http.Error(w, "failed to store key pair: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(generateSSHKeyPairResponse{
		PublicKey: string(publicKeyBytes),
	}); err != nil {
		logrus.Errorf("generate-ssh-keypair: failed to write response: %v", err)
	}
}
