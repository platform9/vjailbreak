// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
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
		fmt.Printf("    Path: %s\n", ds.Path)
	}

	fmt.Println("\nListing storage devices/LUNs...")
	devices, err := client.ListStorageDevices()
	if err != nil {
		fmt.Printf("Failed to list storage devices: %v\n", err)
	} else {
		fmt.Printf("Found %d storage device(s):\n", len(devices))
		for i, dev := range devices {
			sizeGB := float64(dev.Size) / (1024 * 1024 * 1024)
			fmt.Printf("\n  Device %d:\n", i+1)
			fmt.Printf("    Device ID: %s\n", dev.DeviceID)
			fmt.Printf("    Display Name: %s\n", dev.DisplayName)
			fmt.Printf("    Size: %.2f GB\n", sizeGB)
			fmt.Printf("    Type: %s\n", dev.DeviceType)
			fmt.Printf("    Vendor: %s\n", dev.Vendor)
			fmt.Printf("    Model: %s\n", dev.Model)
			fmt.Printf("    Is Local: %t\n", dev.IsLocal)
			fmt.Printf("    Is SSD: %t\n", dev.IsSSD)
			if dev.DevfsPath != "" {
				fmt.Printf("    Device Path: %s\n", dev.DevfsPath)
			}
		}
	}

	fmt.Println("\nListing VMs...")
	vms, err := client.ListVMs()
	if err != nil {
		fmt.Printf("Failed to list VMs: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Found %d VM(s):\n", len(vms))
	for i, vm := range vms {
		// Check power state
		powerState, _ := client.ExecuteCommand(fmt.Sprintf("vim-cmd vmsvc/power.getstate %s", vm.ID))
		state := "unknown"
		if strings.Contains(powerState, "Powered on") {
			state = "ON"
		} else if strings.Contains(powerState, "Powered off") {
			state = "OFF"
		}
		fmt.Printf("  %d. %s (ID: %s, Datastore: %s, Power: %s)\n", i+1, vm.Name, vm.ID, vm.Datastore, state)
	}

	// Try to get detailed VM info, but don't fail if it errors
	// (VMs that are powered off may have locked disks)
	if len(vms) > 0 {
		vmName := vms[0].Name
		fmt.Printf("\nGetting details for VM: %s\n", vmName)
		vmInfo, err := client.GetVMInfo(vmName)
		if err != nil {
			fmt.Printf("Warning: Failed to get VM info (VM may be powered off or locked): %v\n", err)
			fmt.Println("Skipping detailed disk information...")
		} else {
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
		}
	} else {
		fmt.Println("No VMs found on this ESXi host")
	}

	// Test vmkfstools clone functionality
	fmt.Println("\n=== Testing vmkfstools clone functionality ===")

	// Get source and target from environment variables
	sourceVMDK := os.Getenv("ESXI_SOURCE_VMDK")
	targetLUN := os.Getenv("ESXI_TARGET_LUN")

	if sourceVMDK != "" && targetLUN != "" {
		fmt.Printf("\nTesting vmkfstools clone:\n")
		fmt.Printf("  Source: %s\n", sourceVMDK)
		fmt.Printf("  Target: %s\n", targetLUN)

		// Reconnect to get a fresh SSH connection for the clone operation
		// (previous commands may have exhausted the connection)
		fmt.Println("\nRefreshing SSH connection...")
		client.Disconnect()
		ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel2()
		if err := client.Connect(ctx2, host, user, privateKey); err != nil {
			fmt.Printf("Failed to reconnect: %v\n", err)
		} else {
			fmt.Println("Reconnected successfully!")
		}

		task, err := client.StartVmkfstoolsClone(sourceVMDK, targetLUN)
		if err != nil {
			fmt.Printf("Failed to start vmkfstools clone: %v\n", err)
		} else {
			fmt.Println("\nSuccessfully started vmkfstools clone task:")
			fmt.Printf("  Task ID: %s\n", task.TaskId)
			fmt.Printf("  PID: %d\n", task.Pid)
			fmt.Printf("  Info: %s\n", task.LastLine)

			// Monitor clone progress
			if task.Pid > 0 {
				fmt.Println("\nMonitoring clone progress (checking every 2 seconds)...")

				for i := 0; i < 30; i++ { // Check for up to 60 seconds
					time.Sleep(2 * time.Second)
					isRunning, err := client.CheckCloneStatus(task.Pid)
					if err != nil {
						fmt.Printf("Error checking status: %v\n", err)
						break
					}

					if !isRunning {
						fmt.Println("\nClone completed!")
						// Check if target file was created
						checkCmd := fmt.Sprintf("ls -lh %s 2>/dev/null", targetLUN)
						if output, err := client.ExecuteCommand(checkCmd); err == nil && output != "" {
							fmt.Printf("Target file created:\n%s\n", output)
						}
						break
					} else {
						fmt.Printf("  [%ds] Clone still running...\n", (i+1)*2)
					}
				}
			}
		}
	} else {
		fmt.Println("\nSkipping vmkfstools clone test (environment variables not set)")
		fmt.Println("\nTo test vmkfstools clone with XCOPY on Pure Storage:")
		fmt.Println("")
		fmt.Println("IMPORTANT: The source VM must be POWERED OFF to avoid file locks!")
		fmt.Println("           Check the VM power states listed above.")
		fmt.Println("")
		fmt.Println("Step 1: Power off the VM (if needed):")
		fmt.Println("  ssh root@<esxi-host> \"vim-cmd vmsvc/power.off <VM-ID>\"")
		fmt.Println("")
		fmt.Println("Step 2: Create target directory:")
		fmt.Println("  ssh root@<esxi-host> \"mkdir -p /vmfs/volumes/pure-ds/test-clone\"")
		fmt.Println("")
		fmt.Println("Step 3: Set source (use a POWERED OFF VM disk from above):")
		fmt.Println("  export ESXI_SOURCE_VMDK=/vmfs/volumes/pure-ds/test-pure-vm/test-pure-vm.vmdk")
		fmt.Println("")
		fmt.Println("Step 4: Set target (NEW file path on SAME datastore for XCOPY):")
		fmt.Println("  export ESXI_TARGET_LUN=/vmfs/volumes/pure-ds/test-clone/cloned-disk.vmdk")
		fmt.Println("")
		fmt.Println("Step 5: Run the test:")
		fmt.Println("  go run test_esxi_simple.go")
		fmt.Println("")
		fmt.Println("NOTE: Source and target must be on the same Pure datastore for XCOPY to work!")
		fmt.Println("      XCOPY will offload the clone to Pure FlashArray (hardware acceleration)")
	}

	fmt.Println("\nAll tests completed!")
}
