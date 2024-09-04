// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"vjailbreak/openstack"

	"vjailbreak/nbd"
	"vjailbreak/vcenter"
	"vjailbreak/virtv2v"
	"vjailbreak/vm"

	"github.com/vmware/govmomi/vim25/types"
)

type Migrate struct {
	URL              string
	UserName         string
	Password         string
	Insecure         bool
	Networknames     []string
	Volumetypes      []string
	Virtiowin        string
	Ostype           string
	Thumbprint       string
	Convert          bool
	Openstackclients openstack.OpenstackOperations
	Vcclient         vcenter.VCenterOperations
	VMops            vm.VMOperations
	Nbdops           []nbd.NBDOperations
	EventReporter    chan string
	InPod            bool
	MigrationTimes   MigrationTimes
	MigrationType    string
}

type MigrationTimes struct {
	DataCopyStart  time.Time
	VMCutoverStart time.Time
	VMCutoverEnd   time.Time
}

func (migobj *Migrate) logMessage(message string) {
	log.Println(message)
	if migobj.InPod {
		migobj.EventReporter <- message
	}
}

// This function creates volumes in OpenStack and attaches them to the helper vm
func (migobj *Migrate) CreateVolumes(vminfo vm.VMInfo) (vm.VMInfo, error) {
	openstackops := migobj.Openstackclients
	migobj.logMessage("Creating volumes in OpenStack")
	for idx, vmdisk := range vminfo.VMDisks {
		volume, err := openstackops.CreateVolume(vminfo.Name+"-"+vmdisk.Name, vmdisk.Size, vminfo.OSType, vminfo.UEFI, migobj.Volumetypes[idx])
		if err != nil {
			return vminfo, fmt.Errorf("failed to create volume: %s", err)
		}
		vminfo.VMDisks[idx].OpenstackVol = volume
		if idx == 0 {
			err = openstackops.SetVolumeBootable(volume)
			if err != nil {
				return vminfo, fmt.Errorf("failed to set volume as bootable: %s", err)
			}
		}
	}
	migobj.logMessage("Volumes created successfully")
	return vminfo, nil
}

func (migobj *Migrate) AttachVolume(disk vm.VMDisk) (string, error) {
	openstackops := migobj.Openstackclients
	migobj.logMessage("Attaching volumes to VM")

	err := openstackops.AttachVolumeToVM(disk.OpenstackVol.ID)
	if err != nil {
		return "", fmt.Errorf("failed to attach volume to VM: %s", err)
	}

	// Get the Path of the attached volume
	devicePath, err := openstackops.FindDevice(disk.OpenstackVol.ID)
	if err != nil {
		return "", fmt.Errorf("failed to find device: %s", err)
	}
	return devicePath, nil
}

func (migobj *Migrate) DetachVolume(disk vm.VMDisk) error {
	openstackops := migobj.Openstackclients
	err := openstackops.DetachVolumeFromVM(disk.OpenstackVol.ID)
	if err != nil {
		return fmt.Errorf("failed to detach volume from VM: %s", err)
	}
	err = openstackops.WaitForVolume(disk.OpenstackVol.ID)
	if err != nil {
		return fmt.Errorf("failed to wait for volume to become available: %s", err)
	}
	return nil
}

func (migobj *Migrate) DetachAllVolumes(vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.DetachVolumeFromVM(vmdisk.OpenstackVol.ID)
		if err != nil {
			if strings.Contains(err.Error(), "is not attached to volume") {
				return nil
			}
			return fmt.Errorf("failed to detach volume from VM: %s", err)
		}
		err = openstackops.WaitForVolume(vmdisk.OpenstackVol.ID)
		if err != nil {
			return fmt.Errorf("failed to wait for volume to become available: %s", err)
		}
		log.Printf("Volume %s detached from VM\n", vmdisk.Name)
	}
	time.Sleep(1 * time.Second)
	return nil
}

func (migobj *Migrate) DeleteAllVolumes(vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.DeleteVolume(vmdisk.OpenstackVol.ID)
		if err != nil {
			return fmt.Errorf("failed to delete volume: %s", err)
		}
		log.Printf("Volume %s deleted\n", vmdisk.Name)
	}
	return nil
}

// This function enables CBT on the VM if it not enabled and takes a snapshot for initializing CBT
func (migobj *Migrate) EnableCBTWrapper() error {
	vmops := migobj.VMops
	cbt, err := vmops.IsCBTEnabled()
	if err != nil {
		return fmt.Errorf("failed to check if CBT is enabled: %s", err)
	}
	migobj.logMessage(fmt.Sprintf("CBT Enabled: %t", cbt))

	if !cbt {
		// 7.5. Enable CBT
		migobj.logMessage("CBT is not enabled. Enabling CBT")
		err = vmops.EnableCBT()
		if err != nil {
			return fmt.Errorf("failed to enable CBT: %s", err)
		}
		_, err := vmops.IsCBTEnabled()
		if err != nil {
			return fmt.Errorf("failed to check if CBT is enabled: %s", err)
		}
		fmt.Println("Creating temporary snapshot of the source VM")
		err = vmops.TakeSnapshot("tmp-snap")
		if err != nil {
			return fmt.Errorf("failed to take snapshot of source VM: %s", err)
		}
		log.Println("Snapshot created successfully")
		err = vmops.DeleteSnapshot("tmp-snap")
		if err != nil {
			return fmt.Errorf("failed to delete snapshot of source VM: %s", err)
		}
		fmt.Println("Snapshot deleted successfully")
		migobj.logMessage("CBT enabled successfully")
	}
	return nil
}

func (migobj *Migrate) WaitforCutover() error {
	var zerotime time.Time
	if !migobj.MigrationTimes.VMCutoverStart.Equal(zerotime) && migobj.MigrationTimes.VMCutoverStart.After(time.Now()) {
		migobj.logMessage("Waiting for VM Cutover start time")
		time.Sleep(time.Until(migobj.MigrationTimes.VMCutoverStart))
		migobj.logMessage("VM Cutover start time reached")
	} else {
		if !migobj.MigrationTimes.VMCutoverEnd.Equal(zerotime) && migobj.MigrationTimes.VMCutoverEnd.Before(time.Now()) {
			return fmt.Errorf("VM Cutover End time has already passed")
		}
	}
	return nil
}

func (migobj *Migrate) LiveReplicateDisks(ctx context.Context, vminfo vm.VMInfo) (vm.VMInfo, error) {
	vmops := migobj.VMops
	nbdops := migobj.Nbdops
	envURL := migobj.URL
	envUserName := migobj.UserName
	envPassword := migobj.Password
	thumbprint := migobj.Thumbprint

	if migobj.MigrationType == "cold" {
		if err := vmops.VMPowerOff(); err != nil {
			return vminfo, fmt.Errorf("failed to power off VM: %s", err)
		}
	}

	log.Println("Starting NBD server")
	err := vmops.TakeSnapshot("migration-snap")
	if err != nil {
		return vminfo, fmt.Errorf("failed to take snapshot of source VM: %s", err)
	}

	vminfo, err = vmops.UpdateDiskInfo(vminfo)
	if err != nil {
		return vminfo, fmt.Errorf("failed to update disk info: %s", err)
	}

	for idx, vmdisk := range vminfo.VMDisks {
		err := nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vmdisk.Snapname, vmdisk.SnapBackingDisk, migobj.EventReporter)
		if err != nil {
			return vminfo, fmt.Errorf("failed to start NBD server: %s", err)
		}
	}
	// sleep for 2 seconds to allow the NBD server to start
	time.Sleep(2 * time.Second)
	final := false

	incrementalCopyCount := 0
	for {
		// If its the first copy, copy the entire disk
		if incrementalCopyCount == 0 {
			for idx, vmdisk := range vminfo.VMDisks {
				migobj.logMessage(fmt.Sprintf("Copying disk %d", idx))

				vminfo.VMDisks[idx].Path, err = migobj.AttachVolume(vmdisk)
				if err != nil {
					return vminfo, fmt.Errorf("failed to attach volume: %s", err)
				}

				err = nbdops[idx].CopyDisk(ctx, vminfo.VMDisks[idx].Path)
				if err != nil {
					return vminfo, fmt.Errorf("failed to copy disk: %s", err)
				}
				err = migobj.DetachVolume(vmdisk)
				if err != nil {
					return vminfo, fmt.Errorf("failed to detach volume: %s", err)
				}
				migobj.logMessage(fmt.Sprintf("Disk %d copied successfully: %s", idx, vminfo.VMDisks[idx].Path))
			}
		} else {
			migration_snapshot, err := vmops.GetSnapshot("migration-snap")
			if err != nil {
				return vminfo, fmt.Errorf("failed to get snapshot: %s", err)
			}

			var changedAreas types.DiskChangeInfo
			done := true

			for idx, vmdisk := range vminfo.VMDisks {
				changedAreas, err = vmops.CustomQueryChangedDiskAreas(vminfo.VMDisks[idx].ChangeID, migration_snapshot, vminfo.VMDisks[idx].Disk, 0)
				if err != nil {
					return vminfo, fmt.Errorf("failed to get changed disk areas: %s", err)
				}

				if len(changedAreas.ChangedArea) == 0 {
					migobj.logMessage(fmt.Sprintf("Disk %d: No changed blocks found. Skipping copy", idx))
				} else {
					migobj.logMessage(fmt.Sprintf("Disk %d: Blocks have Changed.", idx))

					log.Println("Restarting NBD server")
					err = nbdops[idx].StopNBDServer()
					if err != nil {
						return vminfo, fmt.Errorf("failed to stop NBD server: %s", err)
					}

					err = nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vminfo.VMDisks[idx].Snapname, vminfo.VMDisks[idx].SnapBackingDisk, migobj.EventReporter)
					if err != nil {
						return vminfo, fmt.Errorf("failed to start NBD server: %s", err)
					}
					// sleep for 2 seconds to allow the NBD server to start
					time.Sleep(2 * time.Second)

					// 11. Copy Changed Blocks over
					done = false
					migobj.logMessage("Copying changed blocks")
					vminfo.VMDisks[idx].Path, err = migobj.AttachVolume(vmdisk)
					if err != nil {
						return vminfo, fmt.Errorf("failed to attach volume: %s", err)
					}
					err = nbdops[idx].CopyChangedBlocks(changedAreas, vminfo.VMDisks[idx].Path)
					if err != nil {
						return vminfo, fmt.Errorf("failed to copy changed blocks: %s", err)
					}
					err = migobj.DetachVolume(vmdisk)
					if err != nil {
						return vminfo, fmt.Errorf("failed to detach volume: %s", err)
					}
					migobj.logMessage("Finished copying changed blocks")
				}
			}
			if final {
				break
			}
			if done || incrementalCopyCount > 20 {
				log.Println("Shutting down source VM and performing final copy")
				if err := migobj.WaitforCutover(); err != nil {
					return vminfo, fmt.Errorf("failed to start VM Cutover: %s", err)
				}
				err = vmops.VMPowerOff()
				if err != nil {
					return vminfo, fmt.Errorf("failed to power off VM: %s", err)
				}
				final = true
			}

		}

		//Update old change id to the new base change id value
		// Only do this after you have gone through all disks with old change id.
		// If you dont, only your first disk will have the updated changes
		vminfo, err = vmops.UpdateDiskInfo(vminfo)
		if err != nil {
			return vminfo, fmt.Errorf("failed to update disk info: %s", err)
		}
		err = vmops.DeleteSnapshot("migration-snap")
		if err != nil {
			return vminfo, fmt.Errorf("failed to delete snapshot of source VM: %s", err)
		}
		err = vmops.TakeSnapshot("migration-snap")
		if err != nil {
			return vminfo, fmt.Errorf("failed to take snapshot of source VM: %s", err)
		}

		incrementalCopyCount += 1

	}
	log.Println("Stopping NBD server")
	for _, nbdserver := range nbdops {
		err = nbdserver.StopNBDServer()
		if err != nil {
			return vminfo, fmt.Errorf("failed to stop NBD server: %s", err)
		}
	}

	log.Println("Deleting migration snapshot")
	err = vmops.DeleteSnapshot("migration-snap")
	if err != nil {
		return vminfo, fmt.Errorf("failed to delete snapshot of source VM: %s", err)
	}
	return vminfo, nil
}

func (migobj *Migrate) ConvertVolumes(ctx context.Context, vminfo vm.VMInfo) error {
	migobj.logMessage("Converting disk")
	path, err := migobj.AttachVolume(vminfo.VMDisks[0])
	if err != nil {
		return fmt.Errorf("failed to attach volume: %s", err)
	}
	if migobj.Convert {
		// Fix NTFS
		if vminfo.OSType == "windows" {
			err = virtv2v.NTFSFix(path)
			if err != nil {
				return fmt.Errorf("failed to run ntfsfix: %s", err)
			}
		}

		err := virtv2v.ConvertDisk(ctx, path, vminfo.OSType, migobj.Virtiowin)
		if err != nil {
			return fmt.Errorf("failed to run virt-v2v: %s", err)
		}
	}

	if vminfo.OSType == "linux" {
		osRelease, err := virtv2v.GetOsRelease(path)
		if err != nil {
			return fmt.Errorf("failed to get os release: %s", err)
		}
		if strings.Contains(osRelease, "ubuntu") {
			// Add Wildcard Netplan
			log.Println("Adding wildcard netplan")
			err := virtv2v.AddWildcardNetplan(path)
			if err != nil {
				return fmt.Errorf("failed to add wildcard netplan: %s", err)
			}
			log.Println("Wildcard netplan added successfully")
		}
	}
	err = migobj.DetachVolume(vminfo.VMDisks[0])
	if err != nil {
		return fmt.Errorf("failed to detach volume: %s", err)
	}
	migobj.logMessage("Successfully converted disk")
	return nil
}

func (migobj *Migrate) CreateTargetInstance(vminfo vm.VMInfo) error {
	migobj.logMessage("Creating target instance")
	openstackops := migobj.Openstackclients
	networknames := migobj.Networknames
	closestFlavour, err := openstackops.GetClosestFlavour(vminfo.CPU, vminfo.Memory)
	if err != nil {
		return fmt.Errorf("failed to get closest OpenStack flavor: %s", err)
	}
	log.Printf("Closest OpenStack flavor: %s: CPU: %dvCPUs\tMemory: %dMB\n", closestFlavour.Name, closestFlavour.VCPUs, closestFlavour.RAM)

	networkids := []string{}
	portids := []string{}
	for idx, networkname := range networknames {
		// Create Port Group with the same mac address as the source VM
		// Find the network with the given ID
		network, err := openstackops.GetNetwork(networkname)
		if err != nil {
			return fmt.Errorf("failed to get network: %s", err)
		}
		log.Printf("Network ID: %s\n", network.ID)

		ip := ""
		if len(vminfo.Mac) != len(vminfo.IPs) {
			ip = ""
		} else {
			ip = vminfo.IPs[idx]
		}
		port, err := openstackops.CreatePort(network, vminfo.Mac[idx], ip, vminfo.Name)
		if err != nil {
			return fmt.Errorf("failed to create port group: %s", err)
		}

		log.Printf("Port created successfully: MAC:%s IP:%s\n", port.MACAddress, port.FixedIPs[0].IPAddress)
		networkids = append(networkids, network.ID)
		portids = append(portids, port.ID)
	}

	// Create a new VM in OpenStack
	newVM, err := openstackops.CreateVM(closestFlavour, networkids, portids, vminfo)
	if err != nil {
		return fmt.Errorf("failed to create VM: %s", err)
	}
	migobj.logMessage(fmt.Sprintf("VM created successfully: ID: %s", newVM.ID))
	return nil
}

func (migobj *Migrate) gracefulTerminate(vminfo vm.VMInfo, cancel context.CancelFunc) {
	gracefulShutdown := make(chan os.Signal, 1)
	// Handle SIGTERM
	signal.Notify(gracefulShutdown, syscall.SIGTERM, syscall.SIGINT)
	<-gracefulShutdown
	migobj.logMessage("Gracefully terminating")
	cancel()
	migobj.cleanup(vminfo)
	os.Exit(0)
}

func (migobj *Migrate) MigrateVM(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	// Wait until the data copy start time
	var zerotime time.Time
	if !migobj.MigrationTimes.DataCopyStart.Equal(zerotime) && migobj.MigrationTimes.DataCopyStart.After(time.Now()) {
		migobj.logMessage("Waiting for data copy start time")
		time.Sleep(time.Until(migobj.MigrationTimes.DataCopyStart))
		migobj.logMessage("Data copy start time reached")
	}
	// Get Info about VM
	vminfo, err := migobj.VMops.GetVMInfo(migobj.Ostype)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to get all info: %s", err)
	}

	// Graceful Termination
	go migobj.gracefulTerminate(vminfo, cancel)

	// Create and Add Volumes to Host
	vminfo, err = migobj.CreateVolumes(vminfo)
	if err != nil {
		return fmt.Errorf("failed to add volumes to host: %s", err)
	}

	// Enable CBT
	err = migobj.EnableCBTWrapper()
	if err != nil {
		migobj.cleanup(vminfo)
		return fmt.Errorf("CBT Failure: %s", err)
	}

	for range vminfo.VMDisks {
		migobj.Nbdops = append(migobj.Nbdops, &nbd.NBDServer{})
	}

	// Live Replicate Disks
	vminfo, err = migobj.LiveReplicateDisks(ctx, vminfo)
	if err != nil {
		migobj.cleanup(vminfo)
		return fmt.Errorf("failed to live replicate disks: %s", err)
	}

	// Convert the Boot Disk to raw format
	err = migobj.ConvertVolumes(ctx, vminfo)
	if err != nil {
		migobj.cleanup(vminfo)
		return fmt.Errorf("failed to convert disks: %s", err)
	}

	err = migobj.CreateTargetInstance(vminfo)
	if err != nil {
		migobj.cleanup(vminfo)
		return fmt.Errorf("failed to create target instance: %s", err)
	}
	cancel()
	return nil
}

func (migobj *Migrate) cleanup(vminfo vm.VMInfo) {
	log.Println("Trying to perform cleanup")
	err := migobj.DetachAllVolumes(vminfo)
	if err != nil {
		log.Printf("Failed to detach all volumes from VM: %s\n", err)
	} else if err = migobj.DeleteAllVolumes(vminfo); err != nil {
		log.Printf("Failed to delete all volumes from host: %s\n", err)
	}
	err = migobj.VMops.DeleteSnapshot("migration-snap")
	if err != nil {
		log.Printf("Failed to delete snapshot of source VM: %s\n", err)
	}
}
