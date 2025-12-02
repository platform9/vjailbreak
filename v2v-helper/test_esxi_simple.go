// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	esxissh "github.com/platform9/vjailbreak/v2v-helper/esxi-ssh"
)

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func main() {
	programStart := time.Now()

	// Disable klog output for cleaner display
	flag.Set("logtostderr", "false")
	flag.Set("v", "0")

	fmt.Printf("=== ESXi VAAI XCOPY Clone Test ===\n")
	fmt.Printf("Program started at: %s\n\n", programStart.Format("15:04:05.000"))

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

	fmt.Printf("Connecting to ESXi: %s@%s\n", user, host)

	client := esxissh.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := client.Connect(ctx, host, user, privateKey); err != nil {
		fmt.Printf("✗ Connection failed: %v\n", err)
		os.Exit(1)
	}
	defer client.Disconnect()

	fmt.Println("✓ Connected\n")

	// Commented out for cleaner clone testing output - uncomment to see ESXi inventory
	/*
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

	// Test VM power management
	fmt.Println("\n=== Testing VM Power Management ===")
	sourceVMName := os.Getenv("ESXI_SOURCE_VM_NAME")
	if sourceVMName != "" && len(vms) > 0 {
		// Find the VM by name
		var sourceVM *esxissh.VMInfo
		for _, vm := range vms {
			if vm.Name == sourceVMName {
				sourceVM = &vm
				break
			}
		}

		if sourceVM != nil {
			fmt.Printf("\nTesting power operations on VM: %s (ID: %s)\n", sourceVM.Name, sourceVM.ID)

			// Get current power state
			powerState, err := client.GetVMPowerState(sourceVM.ID)
			if err != nil {
				fmt.Printf("Warning: Failed to get power state: %v\n", err)
			} else {
				fmt.Printf("Current power state: %s\n", powerState)

				// If VM is powered on, offer to power it off for clone testing
				if powerState == "on" {
					fmt.Println("\nWARNING: VM is powered on. VMDK cloning requires VM to be powered off.")
					fmt.Println("To test clone functionality, power off the VM first:")
					fmt.Printf("  ssh root@%s \"vim-cmd vmsvc/power.off %s\"\n", host, sourceVM.ID)
				} else {
					fmt.Println("✓ VM is powered off - ready for VMDK cloning")
				}
			}
		} else {
			fmt.Printf("VM '%s' not found in VM list\n", sourceVMName)
		}
	} else if sourceVMName == "" {
		fmt.Println("\nSkipping VM power management test (ESXI_SOURCE_VM_NAME not set)")
		fmt.Println("To test power management:")
		fmt.Println("  export ESXI_SOURCE_VM_NAME=<vm-name>")
	}
	*/

	// Test vmkfstools clone functionality
	fmt.Println("=== VAAI XCOPY Clone Test ===\n")

	// Get source and target from environment variables
	sourceVMDK := os.Getenv("ESXI_SOURCE_VMDK")
	targetLUN := os.Getenv("ESXI_TARGET_LUN")

	if sourceVMDK != "" && targetLUN != "" {
		fmt.Printf("Source VMDK: %s\n", sourceVMDK)
		fmt.Printf("Target Path: %s\n\n", targetLUN)

		// Check if target already exists
		targetExists, err := client.CheckVMDKExists(targetLUN)
		if err == nil && targetExists {
			fmt.Printf("⚠ Target already exists - cleanup:\n")
			fmt.Printf("  ssh root@%s \"vmkfstools -U %s\"\n\n", host, targetLUN)
		}

		preCloneTime := time.Now()
		fmt.Printf("[%s] Calling StartVmkfstoolsClone...\n", time.Since(programStart).Round(time.Millisecond))
		task, err := client.StartVmkfstoolsClone(sourceVMDK, targetLUN)
		fmt.Printf("[%s] StartVmkfstoolsClone returned (took %s)\n",
			time.Since(programStart).Round(time.Millisecond),
			time.Since(preCloneTime).Round(time.Millisecond))
		if err != nil {
			fmt.Printf("✗ Failed to start clone: %v\n", err)
		} else {
			fmt.Printf("Clone started (PID: %d)\n", task.Pid)
			fmt.Println("Monitoring progress...\n")

			// Use CloneTracker for live monitoring
			if task.Pid > 0 {

				tracker := esxissh.NewCloneTracker(client, task, sourceVMDK, targetLUN)
				tracker.SetPollInterval(3 * time.Second)

				startTime := time.Now()
				maxDuration := 30 * time.Minute

				// Monitor with callback
				err := tracker.Monitor(func(status *esxissh.CloneStatus) bool {
					elapsed := time.Since(startTime)

					// Check timeout
					if elapsed > maxDuration {
						fmt.Printf("\nWARNING: Clone exceeded maximum duration of %s\n", maxDuration)
						return false
					}

					if status.IsRunning {
						// Show progress on same line using \r
						if status.TotalBytes > 0 && status.PercentDone > 0 {
							fmt.Printf("\r  [%s] %.1f%% complete - %s / %s copied",
								elapsed.Round(time.Second),
								status.PercentDone,
								formatBytes(status.BytesCopied),
								formatBytes(status.TotalBytes))
							if status.EstimatedTime > 0 {
								fmt.Printf(" (ETA: %s)", status.EstimatedTime.Round(time.Second))
							}
						} else {
							fmt.Printf("\r  [%s] Clone in progress...", elapsed.Round(time.Second))
						}
						return true
					} else {
						// Clone finished - clear line and show completion
						fmt.Printf("\r\n✓ Clone completed in %s\n\n", elapsed.Round(time.Second))
						return false // Stop monitoring
					}
				})

				if err != nil {
					fmt.Printf("✗ Clone error: %v\n", err)
				} else {
					cloneDuration := time.Since(startTime)

					// Show vmkfstools log (only last few lines)
					cloneLog, logErr := client.GetCloneLog(task.Pid)
					if logErr == nil && cloneLog != "" && strings.Contains(cloneLog, "100%") {
						fmt.Println("✓ VAAI XCOPY succeeded (hardware accelerated)")
					}

					// Performance metrics
					fmt.Println("\n=== Performance Metrics ===")
					fmt.Printf("Clone Duration: %s\n", cloneDuration.Round(time.Millisecond))
					fmt.Printf("Source: %s\n", sourceVMDK)
					fmt.Printf("Target: %s\n", targetLUN)

					// Verification command
					fmt.Println("\nVerify clone:")
					fmt.Printf("  ssh root@%s \"ls -lh %s\"\n", host, targetLUN[:strings.LastIndex(targetLUN, "/")])

					// Cleanup command
					fmt.Println("\nCleanup:")
					fmt.Printf("  ssh root@%s \"vmkfstools -U %s\"\n", host, targetLUN)
				}
			}
		}
	} else {
		fmt.Println("Missing configuration. Set these environment variables:")
		fmt.Println("")
		fmt.Println("  export ESXI_SOURCE_VMDK=/vmfs/volumes/pure-ds/vm-name/disk.vmdk")
		fmt.Println("  export ESXI_TARGET_LUN=/vmfs/volumes/pure-ds/test-clone/cloned.vmdk")
		fmt.Println("")
		fmt.Println("Note: Source VM must be powered off to avoid file locks")
		fmt.Println("      Source and target must be on same Pure array for XCOPY (can be different datastores)")
	}
}
