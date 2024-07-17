package main

import (
	"bufio"
	"bytes"
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

func NTFSFix(path string) error {
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
		log.Printf("Executing %s", cmd.String())
		// cmd.Stdout = os.Stdout
		// cmd.Stderr = os.Stderr

		err := cmd.Run()
		if err != nil {
			log.Printf("Failed to fix NTFS on %s: %v", partition, err)
		}
		log.Printf("Fixed NTFS on %s", partition)
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

func ConvertDisk(path string, ostype string, virtiowindriver string) error {
	// Convert the disk

	if ostype == "windows" {
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
	log.Printf("Executing %s", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

func AddWildcardNetplan(path string) error {
	// Add wildcard to netplan
	netplan := `[Match]
Name=en*

[Network]
DHCP=yes`

	// Create the netplan file
	err := os.WriteFile("/home/fedora/99-wildcard.network", []byte(netplan), 0644)
	if err != nil {
		return err
	}
	log.Println("Created local netplan file")
	log.Println("Uploading netplan file to disk")
	// Upload it to the disk
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd := exec.Command(
		"guestfish",
		"--rw",
		"-a",
		path,
		"-i")
	input := `upload /home/fedora/99-wildcard.network /etc/systemd/network/99-wildcard.network`
	cmd.Stdin = strings.NewReader(input)
	log.Printf("Executing %s", cmd.String()+" "+input)
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil
}
