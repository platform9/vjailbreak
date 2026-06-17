package server

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/ovf/importer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	proxyVMName = "vjailbreak-ha-proxy"

	// Hardcoded deployment targets — update when generalising.
	deployDatacenter   = "prison"
	deployDatastore    = "datastore-nfs"
	deployNetwork      = "network-19"
	deployVMwareCreds  = "vmware-1"
)

var vmwareCredsGVR = schema.GroupVersionResource{
	Group:    "vjailbreak.k8s.pf9.io",
	Version:  "v1alpha1",
	Resource: "vmwarecreds",
}

// watchAndDeployProxyVM watches for VMwareCreds "vmware-1" to become validated
// then deploys the proxy VM from the locally cached OVA exactly once.
func watchAndDeployProxyVM(ctx context.Context) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		logrus.Errorf("ova-watch: in-cluster config: %v", err)
		return
	}
	dynClient, err := dynamic.NewForConfig(cfg)
	if err != nil {
		logrus.Errorf("ova-watch: dynamic client: %v", err)
		return
	}

	var once sync.Once

	for {
		if ctx.Err() != nil {
			return
		}

		w, err := dynClient.Resource(vmwareCredsGVR).Namespace(migrationSystemNamespace).Watch(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + deployVMwareCreds,
		})
		if err != nil {
			logrus.Warnf("ova-watch: watch VMwareCreds failed: %v — retrying in 30s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				continue
			}
		}

		logrus.Infof("ova-watch: watching VMwareCreds %q for validation", deployVMwareCreds)

		for event := range w.ResultChan() {
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			item, ok := event.Object.(*unstructured.Unstructured)
			if !ok {
				continue
			}
			status, _, _ := unstructured.NestedString(item.Object, "status", "vmwareValidationStatus")
			if !strings.EqualFold(status, "succeeded") {
				continue
			}
			logrus.Infof("ova-watch: VMwareCreds %q validated — scheduling proxy VM deploy", deployVMwareCreds)
			once.Do(func() { go deployWhenOVAReady(ctx) })
		}

		w.Stop()
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// deployWhenOVAReady polls until the OVA file is present, then deploys.
func deployWhenOVAReady(ctx context.Context) {
	dest := filepath.Join(ovaDir, ovaFileName)
	for {
		if _, err := os.Stat(dest); err == nil {
			deployProxyVMFromOVA(dest)
			return
		}
		logrus.Infof("ova-watch: OVA not yet present at %s — retrying in 30s", dest)
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

// deployProxyVMFromOVA imports the OVA at ovaPath into vCenter as proxyVMName.
func deployProxyVMFromOVA(ovaPath string) {
	ctx := context.Background()

	vcHost, vcUsername, vcPassword, err := credsFromSecret(ctx, deployVMwareCreds)
	if err != nil {
		logrus.Errorf("ova-deploy: %v", err)
		return
	}

	client, finder, err := connectVCenter(ctx, vcHost, vcUsername, vcPassword)
	if err != nil {
		logrus.Errorf("ova-deploy: %v", err)
		return
	}
	defer client.Logout(ctx)

	// Datastore
	ds, err := finder.Datastore(ctx, deployDatastore)
	if err != nil {
		logrus.Errorf("ova-deploy: datastore %q: %v", deployDatastore, err)
		return
	}

	// Resource pool — no cluster, so use the first host's Resources pool.
	hosts, err := finder.HostSystemList(ctx, "*")
	if err != nil || len(hosts) == 0 {
		logrus.Errorf("ova-deploy: no hosts found: %v", err)
		return
	}
	rp, err := hosts[0].ResourcePool(ctx)
	if err != nil {
		logrus.Errorf("ova-deploy: resource pool for host %s: %v", hosts[0].Name(), err)
		return
	}

	// VM folder — first folder in the datacenter.
	folders, err := finder.FolderList(ctx, "*")
	if err != nil || len(folders) == 0 {
		logrus.Errorf("ova-deploy: no folder found: %v", err)
		return
	}
	folder := folders[0]

	name := proxyVMName
	imp := &importer.Importer{
		Log: func(msg string) (int, error) {
			logrus.Infof("ova-deploy: %s", strings.TrimRight(msg, "\n"))
			return len(msg), nil
		},
		Client:       client.Client,
		Finder:       finder,
		ResourcePool: rp,
		Datastore:    ds,
		Folder:       folder,
		Archive:      &importer.TapeArchive{Path: ovaPath},
	}

	opts := importer.Options{
		Name: &name,
		NetworkMapping: []importer.Network{
			{Name: "VM Network", Network: deployNetwork},
		},
	}

	ref, err := imp.Import(ctx, ovaPath, opts)
	if err != nil {
		logrus.Errorf("ova-deploy: import failed: %v", err)
		return
	}

	logrus.Infof("ova-deploy: VM %q deployed successfully (ref=%s)", proxyVMName, ref.Value)
}

// credsFromSecret reads VCENTER_HOST/USERNAME/PASSWORD from the named k8s secret.
func credsFromSecret(ctx context.Context, secretName string) (host, username, password string, err error) {
	if k8sAuthClient == nil {
		return "", "", "", fmt.Errorf("k8s client not initialized")
	}
	secret, err := k8sAuthClient.CoreV1().Secrets(migrationSystemNamespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", "", fmt.Errorf("get secret %q: %v", secretName, err)
	}
	host = string(secret.Data["VCENTER_HOST"])
	username = string(secret.Data["VCENTER_USERNAME"])
	password = string(secret.Data["VCENTER_PASSWORD"])
	if host == "" || username == "" {
		return "", "", "", fmt.Errorf("incomplete credentials in secret %q", secretName)
	}
	return host, username, password, nil
}

// connectVCenter connects to vCenter and sets the datacenter on the finder.
func connectVCenter(ctx context.Context, host, username, password string) (*govmomi.Client, *find.Finder, error) {
	stripped := strings.TrimRight(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	u, err := url.Parse(fmt.Sprintf("https://%s/sdk", stripped))
	if err != nil {
		return nil, nil, fmt.Errorf("parse vCenter URL: %v", err)
	}
	u.User = url.UserPassword(username, password)

	client, err := govmomi.NewClient(ctx, u, true)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to vCenter %q: %v", host, err)
	}

	finder := find.NewFinder(client.Client, true)

	dc, err := finder.Datacenter(ctx, deployDatacenter)
	if err != nil {
		client.Logout(ctx)
		return nil, nil, fmt.Errorf("datacenter %q: %v", deployDatacenter, err)
	}
	finder.SetDatacenter(dc)

	return client, finder, nil
}
