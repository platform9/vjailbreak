package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"unicode"
)

func RetainAlphanumeric(input string) string {
	var builder strings.Builder
	for _, char := range input {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func GetPartitions(disk string) ([]string, error) {
	// Execute lsblk command to get partition information
	cmd := exec.Command("lsblk", "-no", "NAME", disk)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to execute lsblk: %w", err)
	}

	var partitions []string
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && line != disk {
			partitions = append(partitions, "/dev/"+RetainAlphanumeric(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading lsblk output: %w", err)
	}

	return partitions, nil
}

func NTFSFix(ctx context.Context, path string) error {
	// Fix NTFS
	partitions, err := GetPartitions(path)
	if err != nil {
		return err
	}
	log.Printf("Partitions: %v", partitions)
	for _, partition := range partitions {
		if partition == path {
			continue
		}
		cmd := exec.Command("ntfsfix", partition)
		log.Println(cmd.String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			log.Printf("Failed to fix NTFS on %s: %v", partition, err)
		}
	}
	return nil
}

func downloadFile(url, filePath string) error {
	// Get the data from the URL
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func ConvertDisk(ctx context.Context, path string, ostype string, virtiowindriver string) error {
	// Convert the disk

	if ostype == "windows" {
		// url := "https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/archive-virtio/virtio-win-0.1.189-1/virtio-win-0.1.189.iso"
		filePath := "/home/fedora/virtio-win.iso"
		log.Println("Downloading virtio windrivers")
		err := downloadFile(virtiowindriver, filePath)
		if err != nil {
			return err
		}
		log.Println("Downloaded virtio windrivers")
		defer os.Remove(filePath)
		os.Setenv("VIRTIO_WIN", filePath)

	}
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd := exec.Command("virt-v2v-in-place", "-i", "disk", path)
	log.Println(cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
