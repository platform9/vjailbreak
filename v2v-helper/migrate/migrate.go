// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"fmt"
	"log"
	"os"
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
	Networkname      string
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
}

func (migobj *Migrate) logMessage(message string) {
	log.Println(message)
	if migobj.InPod {
		migobj.EventReporter <- message
	}
}

// This function creates volumes in OpenStack and attaches them to the helper vm
func (migobj *Migrate) AddVolumestoHost(vminfo vm.VMInfo) (vm.VMInfo, error) {
	openstackops := migobj.Openstackclients
	migobj.logMessage("Creating volumes in OpenStack")
	for idx, vmdisk := range vminfo.VMDisks {
		volume, err := openstackops.CreateVolume(vminfo.Name+"-"+vmdisk.Name, vmdisk.Size, vminfo.OSType, vminfo.UEFI)
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

	migobj.logMessage("Attaching volumes to VM")
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.AttachVolumeToVM(vmdisk.OpenstackVol.ID)
		if err != nil {
			return vminfo, fmt.Errorf("failed to attach volume to VM: %s", err)
		}
		migobj.logMessage(fmt.Sprintf("Volume attached to VM: %s", vmdisk.OpenstackVol.Name))
	}

	// Get the Path of the attached volume
	for idx, vmdisk := range vminfo.VMDisks {
		devicePath, err := openstackops.FindDevice(vmdisk.OpenstackVol.ID)
		if err != nil {
			return vminfo, fmt.Errorf("failed to find device: %s", err)
		}
		vminfo.VMDisks[idx].Path = devicePath
		migobj.logMessage(fmt.Sprintf("Volume %s attached successfully at %s", vmdisk.Name, vminfo.VMDisks[idx].Path))
	}
	return vminfo, nil
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

func (migobj *Migrate) LiveReplicateDisks(vminfo vm.VMInfo) (vm.VMInfo, error) {
	vmops := migobj.VMops
	nbdops := migobj.Nbdops
	envURL := migobj.URL
	envUserName := migobj.UserName
	envPassword := migobj.Password
	thumbprint := migobj.Thumbprint

	log.Println("Starting NBD server")
	err := vmops.TakeSnapshot("migration-snap")
	if err != nil {
		return vminfo, fmt.Errorf("failed to take snapshot of source VM: %s", err)
	}

	vminfo, err = vmops.UpdateDiskInfo(vminfo)
	if err != nil {
		return vminfo, fmt.Errorf("failed to update disk info: %s", err)
	}

	// var nbdservers []nbd.NBDOperations
	for idx, vmdisk := range vminfo.VMDisks {
		err := nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vmdisk.Snapname, vmdisk.SnapBackingDisk, migobj.EventReporter)
		if err != nil {
			return vminfo, fmt.Errorf("failed to start NBD server: %s", err)
		}

	}
	// sleep for 2 seconds to allow the NBD server to start
	time.Sleep(2 * time.Second)

	incrementalCopyCount := 0
	for {
		// If its the first copy, copy the entire disk
		if incrementalCopyCount == 0 {
			for idx, vmdisk := range vminfo.VMDisks {
				migobj.logMessage(fmt.Sprintf("Copying disk %d", idx))

				err = nbdops[idx].CopyDisk(vmdisk.Path)
				if err != nil {
					return vminfo, fmt.Errorf("failed to copy disk: %s", err)
				}
				migobj.logMessage(fmt.Sprintf("Disk %d copied successfully: %s", idx, vminfo.VMDisks[idx].Path))
			}
		} else if incrementalCopyCount > 20 {
			log.Println("20 incremental copies done, will proceed with the conversion now")
			break
		} else {
			migration_snapshot, err := vmops.GetSnapshot("migration-snap")
			if err != nil {
				return vminfo, fmt.Errorf("failed to get snapshot: %s", err)
			}

			var changedAreas types.DiskChangeInfo
			done := true

			for idx, _ := range vminfo.VMDisks {
				// done = true
				// changedAreas, err = source_vm.QueryChangedDiskAreas(ctx, initial_snapshot, final_snapshot, disk, 0)
				changedAreas, err = vmops.CustomQueryChangedDiskAreas(vminfo.VMDisks[idx].ChangeID, migration_snapshot, vminfo.VMDisks[idx].Disk, 0)
				if err != nil {
					return vminfo, fmt.Errorf("failed to get changed disk areas: %s", err)
				}

				if len(changedAreas.ChangedArea) == 0 {
					log.Println("No changed blocks found. Skipping copy")
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
					err = nbdops[idx].CopyChangedBlocks(changedAreas, vminfo.VMDisks[idx].Path)
					if err != nil {
						return vminfo, fmt.Errorf("failed to copy changed blocks: %s", err)
					}
					migobj.logMessage("Finished copying changed blocks")
				}
			}
			if done {
				break
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

func (migobj *Migrate) ConvertDisks(vminfo vm.VMInfo) error {
	migobj.logMessage("Converting disk")
	if migobj.Convert {
		// Fix NTFS
		if vminfo.OSType == "windows" {
			err := virtv2v.NTFSFix(vminfo.VMDisks[0].Path)
			if err != nil {
				return fmt.Errorf("failed to run ntfsfix: %s", err)
			}
		}

		err := virtv2v.ConvertDisk(vminfo.VMDisks[0].Path, vminfo.OSType, migobj.Virtiowin)
		if err != nil {
			return fmt.Errorf("failed to run virt-v2v: %s", err)
		}
	}

	if vminfo.OSType == "linux" {
		// Add Wildcard Netplan
		log.Println("Adding wildcard netplan")
		err := virtv2v.AddWildcardNetplan(vminfo.VMDisks[0].Path)
		if err != nil {
			return fmt.Errorf("failed to add wildcard netplan: %s", err)
		}
		log.Println("Wildcard netplan added successfully")
	}
	migobj.logMessage("Successfully converted disk")
	return nil
}

func (migobj *Migrate) DetachAllDisks(vminfo vm.VMInfo) error {
	openstackops := migobj.Openstackclients
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.DetachVolumeFromVM(vmdisk.OpenstackVol.ID)
		if err != nil {
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

func (migobj *Migrate) DeleteAllDisks(vminfo vm.VMInfo) error {
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

func (migobj *Migrate) CreateTargetInstance(vminfo vm.VMInfo) error {
	migobj.logMessage("Creating target instance")
	openstackops := migobj.Openstackclients
	networkname := migobj.Networkname
	closestFlavour, err := openstackops.GetClosestFlavour(vminfo.CPU, vminfo.Memory)
	if err != nil {
		return fmt.Errorf("failed to get closest OpenStack flavor: %s", err)
	}
	log.Printf("Closest OpenStack flavor: %s: CPU: %dvCPUs\tMemory: %dMB\n", closestFlavour.Name, closestFlavour.VCPUs, closestFlavour.RAM)

	// Create Port Group with the same mac address as the source VM
	// Find the network with the given ID
	networkid, err := openstackops.GetNetworkID(networkname)
	if err != nil {
		return fmt.Errorf("failed to get network ID: %s", err)
	}
	log.Printf("Network ID: %s\n", networkid)

	port, err := openstackops.CreatePort(networkid, vminfo)
	if err != nil {
		return fmt.Errorf("failed to create port group: %s", err)
	}

	log.Printf("Port Group created successfully: MAC:%s IP:%s\n", port.MACAddress, port.FixedIPs[0].IPAddress)

	// Create a new VM in OpenStack
	newVM, err := openstackops.CreateVM(closestFlavour, networkid, port, vminfo)
	if err != nil {
		return fmt.Errorf("failed to create VM: %s", err)
	}
	migobj.logMessage(fmt.Sprintf("VM created successfully: ID: %s", newVM.ID))
	return nil
}

func (migobj *Migrate) MigrateVM(ctx context.Context) error {
	// Get Info about VM
	vminfo, err := migobj.VMops.GetVMInfo(migobj.Ostype)
	if err != nil {
		return fmt.Errorf("failed to get all info: %s", err)
	}

	// Wait for context cancellation to cleanup
	go func() {
		<-ctx.Done()
		migobj.cleanup(vminfo)
	}()

	// Create and Add Volumes to Host
	vminfo, err = migobj.AddVolumestoHost(vminfo)
	if err != nil {
		return fmt.Errorf("failed to add volumes to host: %s", err)
	}

	// Enable CBT
	err = migobj.EnableCBTWrapper()
	if err != nil {
		return fmt.Errorf("CBT Failure: %s", err)
	}

	for range vminfo.VMDisks {
		migobj.Nbdops = append(migobj.Nbdops, &nbd.NBDServer{})
	}

	// Live Replicate Disks
	vminfo, err = migobj.LiveReplicateDisks(vminfo)
	if err != nil {
		log.Printf("Failed to live replicate disks: %s\n", err)
		log.Println("Removing migration snapshot and Openstack volumes.")
		err = migobj.VMops.DeleteSnapshot("migration-snap")
		if err != nil {
			return fmt.Errorf("failed to delete snapshot of source VM: %s", err)
		}
		err = migobj.DetachAllDisks(vminfo)
		if err != nil {
			return fmt.Errorf("failed to detach all volumes from VM: %s", err)
		}
		err = migobj.DeleteAllDisks(vminfo)
		if err != nil {
			return fmt.Errorf("failed to delete all volumes from host: %s", err)
		}
		os.Exit(1)
	}

	// Convert the Boot Disk to raw format
	err = migobj.ConvertDisks(vminfo)
	if err != nil {
		return fmt.Errorf("failed to convert disks: %s", err)
	}

	// Detatch all volumes from VM
	err = migobj.DetachAllDisks(vminfo)
	if err != nil {
		return fmt.Errorf("failed to detach all volumes from VM: %s", err)
	}

	err = migobj.CreateTargetInstance(vminfo)
	if err != nil {
		return fmt.Errorf("failed to create target instance: %s", err)
	}
	return nil
}

func (migobj *Migrate) cleanup(vminfo vm.VMInfo) {
	log.Println("Trying to perform cleanup")
	err := migobj.VMops.DeleteSnapshot("migration-snap")
	if err != nil {
		log.Printf("Failed to delete snapshot of source VM: %s\n", err)
	}
	err = migobj.DetachAllDisks(vminfo)
	if err != nil {
		log.Printf("Failed to detach all volumes from VM: %s\n", err)
	}
	err = migobj.DeleteAllDisks(vminfo)
	if err != nil {
		log.Printf("Failed to delete all volumes from host: %s\n", err)
	}
}
