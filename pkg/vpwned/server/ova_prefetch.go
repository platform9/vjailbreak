package server

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/platform9/vjailbreak/pkg/common/constants"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func prefetchProxyVMOVA() {
	dest := filepath.Join(constants.ProxyVMOVADir, constants.ProxyVMOVAFileName)

	if err := os.MkdirAll(constants.ProxyVMOVADir, 0755); err != nil {
		logrus.Errorf("ova-prefetch: failed to create directory %s: %v", constants.ProxyVMOVADir, err)
		return
	}

	if _, err := os.Stat(dest); err == nil {
		logrus.Infof("ova-prefetch: %s already exists, skipping download", dest)
		return
	}

	url := resolveOVAURL()
	logrus.Infof("ova-prefetch: downloading %s → %s", url, dest)

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		logrus.Errorf("ova-prefetch: failed to create temp file: %v", err)
		return
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		logrus.Errorf("ova-prefetch: download failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logrus.Errorf("ova-prefetch: unexpected HTTP status %d", resp.StatusCode)
		return
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		logrus.Errorf("ova-prefetch: write failed: %v", err)
		return
	}

	if err := f.Close(); err != nil {
		logrus.Errorf("ova-prefetch: close failed: %v", err)
		return
	}

	if err := os.Rename(tmp, dest); err != nil {
		logrus.Errorf("ova-prefetch: rename failed: %v", err)
		return
	}

	logrus.Infof("ova-prefetch: download complete → %s", dest)
}
