package proxyclient

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ProxyAwareClient wraps a controller-runtime client.Client so that all
// Kubernetes Secret and Pod operations are routed through the vjailbreak proxy
// instead of hitting the API server directly. All other resource types pass
// through to the underlying client unchanged.
type ProxyAwareClient struct {
	client.Client
	proxy *Client
}

// NewProxyAwareClient returns a ProxyAwareClient that routes secret/pod
// operations through proxy and all other calls through base.
func NewProxyAwareClient(base client.Client, proxy *Client) *ProxyAwareClient {
	return &ProxyAwareClient{Client: base, proxy: proxy}
}

// Get intercepts Get calls for corev1.Secret and corev1.Pod and routes them
// through the proxy. All other types fall through to the underlying client.
func (c *ProxyAwareClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch o := obj.(type) {
	case *corev1.Secret:
		return c.proxy.GetSecret(ctx, key.Namespace, key.Name, o)
	case *corev1.Pod:
		return c.proxy.GetPod(ctx, key.Namespace, key.Name, o)
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

// List intercepts List calls for corev1.PodList and routes them through the proxy.
func (c *ProxyAwareClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if l, ok := list.(*corev1.PodList); ok {
		listOpts := &client.ListOptions{}
		for _, opt := range opts {
			opt.ApplyToList(listOpts)
		}
		labelSelector := ""
		if listOpts.LabelSelector != nil {
			labelSelector = listOpts.LabelSelector.String()
			if labelSelector == labels.Everything().String() {
				labelSelector = ""
			}
		}
		return c.proxy.ListPods(ctx, listOpts.Namespace, labelSelector, l)
	}
	return c.Client.List(ctx, list, opts...)
}

// Create intercepts Create calls for corev1.Secret and routes them through the proxy.
func (c *ProxyAwareClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if o, ok := obj.(*corev1.Secret); ok {
		return c.proxy.CreateSecret(ctx, o)
	}
	return c.Client.Create(ctx, obj, opts...)
}

// Update intercepts Update calls for corev1.Secret and corev1.Pod and routes
// them through the proxy.
func (c *ProxyAwareClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	switch o := obj.(type) {
	case *corev1.Secret:
		return c.proxy.UpdateSecret(ctx, o)
	case *corev1.Pod:
		// Pod updates from the controller only touch labels (startCutover label).
		return c.proxy.UpdatePodLabels(ctx, o.Namespace, o.Name, o.Labels)
	}
	return c.Client.Update(ctx, obj, opts...)
}

// Delete intercepts Delete calls for corev1.Secret and routes them through the proxy.
func (c *ProxyAwareClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if o, ok := obj.(*corev1.Secret); ok {
		if o.Namespace == "" || o.Name == "" {
			return fmt.Errorf("proxyclient: delete secret requires both namespace and name")
		}
		return c.proxy.DeleteSecret(ctx, o.Namespace, o.Name)
	}
	return c.Client.Delete(ctx, obj, opts...)
}

// Patch falls through to the underlying client for all types.
func (c *ProxyAwareClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return c.Client.Patch(ctx, obj, patch, opts...)
}
