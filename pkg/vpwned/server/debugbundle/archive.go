package debugbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"time"
)

// TarGz packs the archive files into a gzip-compressed tar. Every entry is
// placed under baseDir so extraction yields a single top-level directory.
func TarGz(baseDir string, files []ArchiveFile, modTime time.Time) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for _, file := range files {
		header := &tar.Header{
			Name:    baseDir + "/" + file.Path,
			Mode:    0o644,
			Size:    int64(len(file.Data)),
			ModTime: modTime,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("failed to write tar header for %s: %w", file.Path, err)
		}
		if _, err := tarWriter.Write(file.Data); err != nil {
			return nil, fmt.Errorf("failed to write tar entry %s: %w", file.Path, err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize tar: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize gzip: %w", err)
	}
	return buf.Bytes(), nil
}
