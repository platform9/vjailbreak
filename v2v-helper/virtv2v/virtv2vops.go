// Copyright © 2024 The vjailbreak authors

package virtv2v

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

//go:generate mockgen -source=../virtv2v/virtv2vops.go -destination=../virtv2v/virtv2vops_mock.go -package=virtv2v

type VirtV2VOperations interface {
	RetainAlphanumeric(input string) string
	GetPartitions(disk string) ([]string, error)
	NTFSFix(path string) error
	ConvertDisk(ctx context.Context, path, ostype, virtiowindriver string, firstbootscripts []string) error
	AddWildcardNetplan(path string) error
	GetOsRelease(path string) (string, error)
	AddFirstBootScript(firstbootscript, firstbootscriptname string) error
}

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
		return fmt.Errorf("failed to get partitions: %w", err)
	}
	log.Printf("Partitions: %v", partitions)
	for _, partition := range partitions {
		if partition == path {
			continue
		}
		cmd := exec.Command("ntfsfix", partition)
		log.Printf("Executing %s", cmd.String())

		err := cmd.Run()
		if err != nil {
			log.Printf("Skipping NTFS fix on %s", partition)
		}
		log.Printf("Fixed NTFS on %s", partition)
	}
	return nil
}

func downloadFile(url, filePath string) error {
	// Get the data from the URL
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %s", err)
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Create the file
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %s", err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to file: %s", err)
	}
	return nil
}

func ConvertDisk(ctx context.Context, path, ostype, virtiowindriver string, firstbootscripts []string) error {
	// Convert the disk

	if ostype == "windows" {
		filePath := "/home/fedora/virtio-win.iso"
		log.Println("Downloading virtio windrivers")
		err := downloadFile(virtiowindriver, filePath)
		if err != nil {
			return fmt.Errorf("failed to download virtio-win: %s", err)
		}
		log.Println("Downloaded virtio windrivers")
		defer os.Remove(filePath)
		os.Setenv("VIRTIO_WIN", filePath)

	}
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	args := []string{"--firstboot", "/home/fedora/scripts/user_firstboot.sh"}
	for _, script := range firstbootscripts {
		args = append(args, "--firstboot", fmt.Sprintf("/home/fedora/%s.sh", script))
	}
	args = append(args, "-i", "disk", path)
	cmd := exec.CommandContext(ctx,
		"virt-v2v-in-place",
		args...,
	)
	log.Printf("Executing %s", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run virt-v2v-in-place: %s", err)
	}
	return nil
}

func GetOsRelease(path string) (string, error) {
	// Get the os-release file
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd := exec.Command(
		"guestfish",
		"--ro",
		"-a",
		path,
		"-i")
	input := `cat /etc/os-release`
	cmd.Stdin = strings.NewReader(input)
	log.Printf("Executing %s", cmd.String()+" "+input)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get os-release: %s", err)
	}
	return strings.ToLower(string(out)), nil
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
		return fmt.Errorf("failed to create netplan file: %s", err)
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
		return fmt.Errorf("failed to upload netplan file: %s", err)
	}
	return nil
}
func AddFirstBootScript(firstbootscript, firstbootscriptname string) error {
	// Create the firstboot script
	firstbootscriptpath := fmt.Sprintf("/home/fedora/%s.sh", firstbootscriptname)
	err := os.WriteFile(firstbootscriptpath, []byte(firstbootscript), 0644)
	if err != nil {
		return fmt.Errorf("failed to create firstboot script: %s", err)
	}
	log.Printf("Created firstboot script %s", firstbootscriptname)
	return nil
}
