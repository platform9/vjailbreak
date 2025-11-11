// Copyright © 2024 The vjailbreak authors

package esxissh_test

import (
	"context"
	"fmt"
	"os"

	"github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
)

// ExampleClient_basic demonstrates basic ESXi SSH client usage
func ExampleClient_basic() {
	// Create credentials
	creds := &esxissh.ESXiCredentials{
		Host:     "esxi-host.example.com",
		Port:     22,
		Username: "root",
		Password: "password",
	}

	// Create client
	client := esxissh.NewClient(creds)

	// Connect
	if err := client.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer client.Disconnect()

	// Test connection
	if err := client.TestConnection(); err != nil {
		fmt.Printf("Connection test failed: %v\n", err)
		return
	}

	fmt.Println("Connected successfully!")
}

// ExampleClient_listVMs demonstrates listing VMs on ESXi
func ExampleClient_listVMs() {
	creds := &esxissh.ESXiCredentials{
		Host:     "esxi-host.example.com",
		Username: "root",
		Password: "password",
	}

	client := esxissh.NewClient(creds)
	if err := client.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer client.Disconnect()

	// List all VMs
	vms, err := client.ListVMs()
	if err != nil {
		fmt.Printf("Failed to list VMs: %v\n", err)
		return
	}

	fmt.Printf("Found %d VMs:\n", len(vms))
	for _, vm := range vms {
		fmt.Printf("  - %s (ID: %s, Datastore: %s)\n", vm.Name, vm.ID, vm.Datastore)
	}
}

// ExampleClient_getVMDisks demonstrates getting disk info for a VM
func ExampleClient_getVMDisks() {
	creds := &esxissh.ESXiCredentials{
		Host:     "esxi-host.example.com",
		Username: "root",
		Password: "password",
	}

	client := esxissh.NewClient(creds)
	if err := client.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer client.Disconnect()

	// Get VM info
	vmInfo, err := client.GetVMInfo("my-vm")
	if err != nil {
		fmt.Printf("Failed to get VM info: %v\n", err)
		return
	}

	fmt.Printf("VM: %s\n", vmInfo.Name)
	fmt.Printf("Disks:\n")
	for i, disk := range vmInfo.Disks {
		sizeGB := float64(disk.SizeBytes) / (1024 * 1024 * 1024)
		fmt.Printf("  Disk %d: %s (%.2f GB, %s)\n", i, disk.Name, sizeGB, disk.ProvisionType)
	}
}

// ExampleClient_streamDisk demonstrates streaming a disk to a local file
func ExampleClient_streamDisk() {
	creds := &esxissh.ESXiCredentials{
		Host:     "esxi-host.example.com",
		Username: "root",
		Password: "password",
	}

	client := esxissh.NewClient(creds)
	if err := client.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer client.Disconnect()

	// Create output file
	outFile, err := os.Create("/tmp/disk-export.vmdk")
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		return
	}
	defer outFile.Close()

	// Setup progress tracking
	progressChan := make(chan esxissh.TransferProgress, 10)
	go func() {
		for progress := range progressChan {
			fmt.Printf("Progress: %.2f%% (%s / %s) - %s - ETA: %s\n",
				progress.Percentage,
				esxissh.FormatBytes(progress.TransferredBytes),
				esxissh.FormatBytes(progress.TotalBytes),
				esxissh.FormatTransferSpeed(progress.BytesPerSecond),
				esxissh.FormatDuration(progress.EstimatedTimeLeft),
			)
		}
	}()

	// Setup transfer options
	options := esxissh.DefaultTransferOptions()
	options.ProgressChan = progressChan
	options.UseCompression = false

	// Stream disk
	diskPath := "/vmfs/volumes/datastore1/my-vm/my-vm.vmdk"
	ctx := context.Background()
	if err := client.StreamDisk(ctx, diskPath, outFile, options); err != nil {
		fmt.Printf("Failed to stream disk: %v\n", err)
		return
	}

	close(progressChan)
	fmt.Println("Disk transfer completed!")
}

// ExampleClient_cloneDisk demonstrates cloning a disk on ESXi
func ExampleClient_cloneDisk() {
	creds := &esxissh.ESXiCredentials{
		Host:     "esxi-host.example.com",
		Username: "root",
		Password: "password",
	}

	client := esxissh.NewClient(creds)
	if err := client.Connect(); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}
	defer client.Disconnect()

	// Setup progress tracking
	progressChan := make(chan esxissh.TransferProgress, 10)
	go func() {
		for progress := range progressChan {
			fmt.Printf("Clone progress: %.2f%%\n", progress.Percentage)
		}
	}()

	// Setup transfer options
	options := esxissh.DefaultTransferOptions()
	options.ProgressChan = progressChan

	// Clone disk
	sourcePath := "/vmfs/volumes/datastore1/my-vm/my-vm.vmdk"
	destPath := "/vmfs/volumes/datastore1/my-vm/my-vm-clone.vmdk"
	ctx := context.Background()

	if err := client.CloneDisk(ctx, sourcePath, destPath, options); err != nil {
		fmt.Printf("Failed to clone disk: %v\n", err)
		return
	}

	close(progressChan)
	fmt.Println("Disk clone completed!")
}

// ExampleClient_withRetry demonstrates connection with retry logic
func ExampleClient_withRetry() {
	creds := &esxissh.ESXiCredentials{
		Host:     "esxi-host.example.com",
		Username: "root",
		Password: "password",
	}

	client := esxissh.NewClient(creds)

	// Connect with retry
	retryConfig := esxissh.DefaultRetryConfig()
	if err := client.ConnectWithRetry(retryConfig); err != nil {
		fmt.Printf("Failed to connect after retries: %v\n", err)
		return
	}
	defer client.Disconnect()

	fmt.Println("Connected successfully with retry!")
}
