package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

var ovaDownloadMu sync.Mutex

func resolveOVAURL() string {
	if k8sAuthClient == nil {
		return constants.ProxyVMOVAURLDefault
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cm, err := k8sAuthClient.CoreV1().ConfigMaps(migrationSystemNamespace).Get(ctx, "vjailbreak-settings", metav1.GetOptions{})
	if err != nil {
		logrus.Warnf("ova-prefetch: could not read vjailbreak-settings: %v — using default URL", err)
		return constants.ProxyVMOVAURLDefault
	}
	if u := cm.Data[constants.ProxyVMOVAURLKey]; u != "" {
		return u
	}
	return constants.ProxyVMOVAURLDefault
}

// validateOVAURL checks that rawURL is a reachable http/https URL.
func validateOVAURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is empty")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL has no host")
	}
	resp, err := http.Head(rawURL) //nolint:gosec
	if err != nil {
		return fmt.Errorf("URL not reachable: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("URL returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// downloadOVAFromURL downloads ovaURL and atomically replaces the OVA file.
// A mutex serialises concurrent calls so only one download runs at a time.
func downloadOVAFromURL(ovaURL string) error {
	ovaDownloadMu.Lock()
	defer ovaDownloadMu.Unlock()

	dest := filepath.Join(constants.ProxyVMOVADir, constants.ProxyVMOVAFileName)
	tmp := dest + ".tmp"

	if err := os.MkdirAll(constants.ProxyVMOVADir, 0755); err != nil {
		return fmt.Errorf("mkdir: %v", err)
	}

	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create temp file: %v", err)
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	resp, err := http.Get(ovaURL) //nolint:gosec
	if err != nil {
		return fmt.Errorf("download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close: %v", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename: %v", err)
	}
	return nil
}

// prefetchProxyVMOVA downloads the OVA at startup if it is not already present.
func prefetchProxyVMOVA() {
	dest := filepath.Join(constants.ProxyVMOVADir, constants.ProxyVMOVAFileName)

	if _, err := os.Stat(dest); err == nil {
		logrus.Infof("ova-prefetch: %s already exists, skipping download", dest)
		return
	}

	ovaURL := resolveOVAURL()
	logrus.Infof("ova-prefetch: downloading %s → %s", ovaURL, dest)

	if err := downloadOVAFromURL(ovaURL); err != nil {
		logrus.Errorf("ova-prefetch: %v", err)
		return
	}

	logrus.Infof("ova-prefetch: download complete → %s", dest)
}

// watchOVAURLChanges watches vjailbreak-settings for PROXY_VM_OVA_URL updates.
// When the URL changes to a valid, reachable value the OVA is re-downloaded and
// the existing file is atomically replaced (filename stays ha-proxy-vm.ova).
func watchOVAURLChanges(ctx context.Context) {
	if k8sAuthClient == nil {
		logrus.Warn("ova-watch-url: k8s client not available, skipping URL watcher")
		return
	}

	currentURL := resolveOVAURL()

	for {
		if ctx.Err() != nil {
			return
		}

		watcher, err := k8sAuthClient.CoreV1().ConfigMaps(migrationSystemNamespace).Watch(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=vjailbreak-settings",
		})
		if err != nil {
			logrus.Warnf("ova-watch-url: watch error: %v — retrying in 30s", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
				continue
			}
		}

		for event := range watcher.ResultChan() {
			if event.Type != watch.Modified {
				continue
			}
			cm, ok := event.Object.(*corev1.ConfigMap)
			if !ok {
				continue
			}
			newURL := cm.Data[constants.ProxyVMOVAURLKey]
			if newURL == "" || newURL == currentURL {
				continue
			}

			logrus.Infof("ova-watch-url: PROXY_VM_OVA_URL changed to %q — validating...", newURL)

			if err := validateOVAURL(newURL); err != nil {
				logrus.Errorf("ova-watch-url: new URL %q failed validation: %v — keeping existing OVA", newURL, err)
				continue
			}

			logrus.Infof("ova-watch-url: downloading new OVA from %q", newURL)
			if err := downloadOVAFromURL(newURL); err != nil {
				logrus.Errorf("ova-watch-url: download failed: %v — keeping existing OVA", err)
				continue
			}

			currentURL = newURL
			logrus.Infof("ova-watch-url: OVA updated successfully from %q", newURL)
		}

		watcher.Stop()
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}
