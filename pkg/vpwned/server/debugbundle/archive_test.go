package debugbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"time"
)

// untarGz extracts an archive into a path→content map for assertions.
func untarGz(t *testing.T, data []byte) map[string]string {
	t.Helper()
	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to open gzip: %v", err)
	}
	defer gzReader.Close()

	out := map[string]string{}
	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar: %v", err)
		}
		content, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatalf("failed to read tar entry %s: %v", header.Name, err)
		}
		out[header.Name] = string(content)
	}
	return out
}

func TestTarGzRoundTrip(t *testing.T) {
	files := []ArchiveFile{
		{Path: "kubernetes/migrations/m1.yaml", Data: []byte("kind: Migration\n")},
		{Path: "pod-logs/pod1.log", Data: []byte("log line")},
	}

	data, err := TarGz("vm1-debug-bundle-2026", files, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	extracted := untarGz(t, data)
	if got := extracted["vm1-debug-bundle-2026/kubernetes/migrations/m1.yaml"]; got != "kind: Migration\n" {
		t.Errorf("unexpected yaml entry content %q", got)
	}
	if got := extracted["vm1-debug-bundle-2026/pod-logs/pod1.log"]; got != "log line" {
		t.Errorf("unexpected log entry content %q", got)
	}
	if len(extracted) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(extracted), extracted)
	}
}

func TestTarGzEmpty(t *testing.T) {
	data, err := TarGz("empty", nil, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(untarGz(t, data)) != 0 {
		t.Error("expected empty archive")
	}
}
