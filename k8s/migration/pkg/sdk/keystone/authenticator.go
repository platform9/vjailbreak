package keystone

import (
	"context"
	"encoding/json"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AuthOptions struct {
	// PropagateCacheErrors will cause failures in caching layers to be bubbled up
	PropagateCacheErrors bool
}

// Authenticator provides an abstraction layer over the regular keystone client
// to simplify getting and refreshing authentication tokens.
type Authenticator interface {
	Auth(ctx context.Context, opts ...AuthOptions) (AuthInfo, error)
	ResetCache() error
}

type BasicAuthenticator struct {
	client      Client
	credentials Credentials
}

var _ Authenticator = (*BasicAuthenticator)(nil)

func NewBasicTokenGenerator(client Client, credentials Credentials) *BasicAuthenticator {
	return &BasicAuthenticator{client: client, credentials: credentials}
}

func (t *BasicAuthenticator) Auth(ctx context.Context, _ ...AuthOptions) (AuthInfo, error) {
	return t.client.Auth(ctx, t.credentials)
}

func (t *BasicAuthenticator) ResetCache() error {
	// no cache
	return nil
}

type CachedAuthenticator struct {
	authenticator Authenticator
	RenewBefore   time.Duration
	cached        *AuthInfo
}

var _ Authenticator = (*CachedAuthenticator)(nil)

func NewCachedAuthenticator(authenticator Authenticator) *CachedAuthenticator {
	return &CachedAuthenticator{
		authenticator: authenticator,
		RenewBefore:   10 * time.Minute,
	}
}

func (t *CachedAuthenticator) Auth(ctx context.Context, _ ...AuthOptions) (AuthInfo, error) {
	if t.cached != nil && t.cached.ExpiresAt.After(time.Now().Add(t.RenewBefore)) {
		return *t.cached, nil
	}
	authInfo, err := t.authenticator.Auth(ctx)
	if err != nil {
		return AuthInfo{}, err
	}
	t.cached = &authInfo

	return authInfo, nil
}

func (t *CachedAuthenticator) ResetCache() error {
	t.cached = nil
	return nil
}

type FileCachedAuthenticator struct {
	authenticator Authenticator
	RenewBefore   time.Duration
	CachePath     string
}

var _ Authenticator = (*FileCachedAuthenticator)(nil)

func NewFileCachedAuthenticator(authenticator Authenticator, cachePath string) *FileCachedAuthenticator {
	return &FileCachedAuthenticator{
		authenticator: authenticator,
		CachePath:     cachePath,
		RenewBefore:   10 * time.Minute,
	}
}

func (t *FileCachedAuthenticator) Auth(ctx context.Context, opts ...AuthOptions) (AuthInfo, error) {
	var authOpts AuthOptions
	if len(opts) > 0 {
		authOpts = opts[0]
	}

	cached, err := t.readFromFile()
	if err == nil && cached != nil && cached.ExpiresAt.After(time.Now().Add(t.RenewBefore)) {
		return *cached, nil
	}

	authInfo, err := t.authenticator.Auth(ctx)
	if err != nil {
		return AuthInfo{}, err
	}

	err = t.writeToFile(&authInfo)
	if err != nil && authOpts.PropagateCacheErrors {
		return AuthInfo{}, err
	}
	return authInfo, nil
}

func (t *FileCachedAuthenticator) readFromFile() (*AuthInfo, error) {
	data, err := os.ReadFile(t.CachePath)
	if err != nil {
		return nil, err
	}
	authInfo := &AuthInfo{}
	err = json.Unmarshal(data, authInfo)
	if err != nil {
		return nil, err
	}
	return authInfo, nil
}

func (t *FileCachedAuthenticator) writeToFile(authInfo *AuthInfo) error {
	data, err := json.Marshal(authInfo)
	if err != nil {
		return err
	}
	return os.WriteFile(t.CachePath, data, 0700)
}

func (t *FileCachedAuthenticator) ResetCache() error {
	return os.WriteFile(t.CachePath, []byte{}, 0700)
}

// SecretCachedAuthenticator caches the token in a Secret
type SecretCachedAuthenticator struct {
	authenticator Authenticator
	Client        client.Client
	SecretRef     metav1.ObjectMeta
	RenewBefore   time.Duration
}

func NewSecretCachedAuthenticator(client client.Client, SecretRef metav1.ObjectMeta, authenticator Authenticator) *SecretCachedAuthenticator {
	return &SecretCachedAuthenticator{
		authenticator: authenticator,
		Client:        client,
		SecretRef:     SecretRef,
		RenewBefore:   10 * time.Minute,
	}
}

var _ Authenticator = (*SecretCachedAuthenticator)(nil)

func (a *SecretCachedAuthenticator) Auth(ctx context.Context, opts ...AuthOptions) (AuthInfo, error) {
	var authOpts AuthOptions
	if len(opts) > 0 {
		authOpts = opts[0]
	}

	cached, err := a.readFromSecret(ctx)
	if err != nil && authOpts.PropagateCacheErrors {
		return AuthInfo{}, err
	}
	if err == nil && cached != nil && cached.ExpiresAt.After(time.Now().Add(a.RenewBefore)) {
		return *cached, nil
	}

	authInfo, err := a.authenticator.Auth(ctx)
	if err != nil {
		return AuthInfo{}, err
	}

	err = a.writeToSecret(ctx, &authInfo)
	if err != nil && authOpts.PropagateCacheErrors {
		return AuthInfo{}, err
	}
	return authInfo, nil
}

func (a *SecretCachedAuthenticator) ResetCache() error {
	return a.Client.Delete(context.Background(), &corev1.Secret{
		ObjectMeta: a.SecretRef,
	})
}

func (a *SecretCachedAuthenticator) readFromSecret(ctx context.Context) (*AuthInfo, error) {
	tokenSecret := &corev1.Secret{}
	err := a.Client.Get(ctx, types.NamespacedName{
		Namespace: a.SecretRef.Namespace,
		Name:      a.SecretRef.Name,
	}, tokenSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	expiresAt, err := time.Parse(time.RFC3339, string(tokenSecret.Data["expiresAt"]))
	if err != nil {
		return nil, err
	}
	return &AuthInfo{
		Token:     string(tokenSecret.Data["token"]),
		UserID:    string(tokenSecret.Data["userID"]),
		ProjectID: string(tokenSecret.Data["projectID"]),
		ExpiresAt: expiresAt,
	}, nil
}

func (a *SecretCachedAuthenticator) writeToSecret(ctx context.Context, info *AuthInfo) error {
	tokenSecret := &corev1.Secret{
		ObjectMeta: a.SecretRef,
		Data: map[string][]byte{
			"token":     []byte(info.Token),
			"userID":    []byte(info.UserID),
			"projectID": []byte(info.ProjectID),
			"expiresAt": []byte(info.ExpiresAt.Format(time.RFC3339)),
		},
		Immutable: pointer.Bool(true),
	}
	err := a.Client.Delete(ctx, tokenSecret.DeepCopy())
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return a.Client.Create(ctx, tokenSecret)
}
