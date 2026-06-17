package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

const (
	ovaURL      = "https://vjailbreak-dev.s3.us-west-2.amazonaws.com/hot-add/ha-proxy-vm.ova"
	ovaDir      = "/home/ubuntu/proxy-vm-template"
	ovaFileName = "ha-proxy-vm.ova"
)

func prefetchProxyVMOVA() {
	dest := filepath.Join(ovaDir, ovaFileName)

	if err := os.MkdirAll(ovaDir, 0755); err != nil {
		logrus.Errorf("ova-prefetch: failed to create directory %s: %v", ovaDir, err)
		return
	}

	if _, err := os.Stat(dest); err == nil {
		logrus.Infof("ova-prefetch: %s already exists, skipping download", dest)
		return
	}

	logrus.Infof("ova-prefetch: downloading %s → %s", ovaURL, dest)

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

	resp, err := http.Get(ovaURL) //nolint:gosec
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
