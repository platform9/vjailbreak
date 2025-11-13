// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
)

func main() {
	fmt.Println("Starting ESXi SSH test...")

	host := os.Getenv("ESXI_HOST")
	user := os.Getenv("ESXI_USER")
	keyPath := os.Getenv("ESXI_SSH_KEY_PATH")

	if host == "" || user == "" || keyPath == "" {
		fmt.Println("Usage:")
		fmt.Println("  export ESXI_HOST=10.96.6.203")
		fmt.Println("  export ESXI_USER=root")
		fmt.Println("  export ESXI_SSH_KEY_PATH=~/.ssh/id_rsa")
		fmt.Println("  go run test_esxi_simple.go")
		os.Exit(1)
	}

	privateKey, err := os.ReadFile(os.ExpandEnv(keyPath))
	if err != nil {
		fmt.Printf("Failed to read SSH key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Connecting to %s as %s...\n", host, user)

	client := esxissh.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.Connect(ctx, host, user, privateKey); err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	fmt.Println("Connected successfully!")

	if err := client.TestConnection(); err != nil {
		fmt.Printf("Connection test failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Connection test passed!")

	fmt.Println("\nTesting hostname command...")
	output, err := client.ExecuteCommand("hostname")
	if err != nil {
		fmt.Printf("Hostname command failed: %v\n", err)
	} else {
		fmt.Printf("Hostname: %s\n", output)
	}

	fmt.Println("\nListing datastores...")
	datastores, err := client.ListDatastores()
	if err != nil {
		fmt.Printf("Failed to list datastores: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d datastore(s):\n", len(datastores))
	for _, ds := range datastores {
		sizeGB := float64(ds.Capacity) / (1024 * 1024 * 1024)
		freeGB := float64(ds.FreeSpace) / (1024 * 1024 * 1024)
		fmt.Printf("  - %s: %.2f GB total, %.2f GB free (Type: %s)\n",
			ds.Name, sizeGB, freeGB, ds.Type)
	}

	fmt.Println("\nListing VMs...")
	vms, err := client.ListVMs()
	if err != nil {
		fmt.Printf("Failed to list VMs: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d VM(s):\n", len(vms))
	for i, vm := range vms {
		fmt.Printf("  %d. %s (ID: %s, Datastore: %s)\n", i+1, vm.Name, vm.ID, vm.Datastore)
	}

	if len(vms) > 0 {
		vmName := vms[0].Name
		fmt.Printf("\nGetting details for VM: %s\n", vmName)
		vmInfo, err := client.GetVMInfo(vmName)
		if err != nil {
			fmt.Printf("Failed to get VM info: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("VM Details:")
		fmt.Printf("  Name: %s\n", vmInfo.Name)
		fmt.Printf("  ID: %s\n", vmInfo.ID)
		fmt.Printf("  Datastore: %s\n", vmInfo.Datastore)
		fmt.Printf("  VMX Path: %s\n", vmInfo.VMXPath)
		fmt.Printf("  Disks: %d\n", len(vmInfo.Disks))

		if len(vmInfo.Disks) > 0 {
			fmt.Println("\nDisk information:")
			for i, disk := range vmInfo.Disks {
				sizeGB := float64(disk.SizeBytes) / (1024 * 1024 * 1024)
				fmt.Printf("  Disk %d:\n", i+1)
				fmt.Printf("    Name: %s\n", disk.Name)
				fmt.Printf("    Path: %s\n", disk.Path)
				fmt.Printf("    Size: %.2f GB\n", sizeGB)
				fmt.Printf("    Type: %s\n", disk.ProvisionType)
				fmt.Printf("    Datastore: %s\n", disk.Datastore)
			}
		}
	} else {
		fmt.Println("No VMs found on this ESXi host")
	}

	fmt.Println("\nAll tests completed!")
}
