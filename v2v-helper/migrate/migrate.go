package migrate

import (
	"fmt"
	"log"
	"time"
	"vjailbreak/openstack"

	"vjailbreak/nbd"
	// "vjailbreak/vcenter"
	"vjailbreak/virtv2v"
	"vjailbreak/vm"

	"github.com/vmware/govmomi/vim25/types"
)

// This function creates volumes in OpenStack and attaches them to the helper vm
func AddVolumestoHost(vminfo vm.VMInfo, openstackops openstack.OpenstackOperations) (vm.VMInfo, error) {
	log.Println("Creating volumes in OpenStack")
	for idx, vmdisk := range vminfo.VMDisks {
		volume, err := openstackops.CreateVolume(vminfo.Name+"-"+vmdisk.Name, vmdisk.Size, vminfo.OSType, vminfo.UEFI)
		if err != nil {
			return vminfo, fmt.Errorf("Failed to create volume: %s\n", err)
		}
		vminfo.VMDisks[idx].OpenstackVol = volume
		if idx == 0 {
			err = openstackops.SetVolumeBootable(volume)
			if err != nil {
				return vminfo, fmt.Errorf("Failed to set volume as bootable: %s\n", err)
			}
		}
	}
	log.Println("Volumes created successfully")

	log.Println("Attaching volumes to VM")
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.AttachVolumeToVM(vmdisk.OpenstackVol.ID)
		if err != nil {
			return vminfo, fmt.Errorf("Failed to attach volume to VM: %s\n", err)
		}
		log.Printf("Volume attached to VM: %s\n", vmdisk.OpenstackVol.Name)
	}

	// Get the Path of the attached volume
	for idx, vmdisk := range vminfo.VMDisks {
		devicePath, err := openstackops.FindDevice(vmdisk.OpenstackVol.ID)
		if err != nil {
			return vminfo, fmt.Errorf("Failed to find device: %s\n", err)
		}
		vminfo.VMDisks[idx].Path = devicePath
		log.Printf("Volume %s attached successfully at %s\n", vmdisk.Name, vminfo.VMDisks[idx].Path)
	}
	return vminfo, nil
}

// This function enables CBT on the VM if it not enabled and takes a snapshot for initializing CBT
func EnableCBTWrapper(vmops vm.VMOperations) error {
	cbt, err := vmops.IsCBTEnabled()
	if err != nil {
		return fmt.Errorf("Failed to check if CBT is enabled: %s\n", err)
	}
	log.Printf("CBT Enabled: %t\n", cbt)

	if !cbt {
		// 7.5. Enable CBT
		log.Println("CBT is not enabled. Enabling CBT")
		err = vmops.EnableCBT()
		if err != nil {
			return fmt.Errorf("Failed to enable CBT: %s\n", err)
		}
		_, err := vmops.IsCBTEnabled()
		if err != nil {
			return fmt.Errorf("Failed to check if CBT is enabled: %s\n", err)
		}
		log.Println("CBT enabled successfully")

		log.Println("Creating temporary snapshot of the source VM")
		err = vmops.TakeSnapshot("tmp-snap")
		if err != nil {
			return fmt.Errorf("Failed to take snapshot of source VM: %s\n", err)
		}
		log.Println("Snapshot created successfully")
		err = vmops.DeleteSnapshot("tmp-snap")
		if err != nil {
			return fmt.Errorf("Failed to delete snapshot of source VM: %s\n", err)
		}
		log.Println("Snapshot deleted successfully")
	}
	return nil
}

func LiveReplicateDisks(vminfo vm.VMInfo, vmops vm.VMOperations, nbdops []nbd.NBDOperations, envURL, envUserName, envPassword, thumbprint string) (vm.VMInfo, error) {
	log.Println("Starting NBD server")
	err := vmops.TakeSnapshot("migration-snap")
	if err != nil {
		return vminfo, fmt.Errorf("Failed to take snapshot of source VM: %s\n", err)
	}

	vminfo, err = vmops.UpdateDiskInfo(vminfo)
	if err != nil {
		return vminfo, fmt.Errorf("Failed to update disk info: %s\n", err)
	}

	// var nbdservers []nbd.NBDOperations
	for idx, vmdisk := range vminfo.VMDisks {
		err := nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vmdisk.Snapname, vmdisk.SnapBackingDisk)
		if err != nil {
			return vminfo, fmt.Errorf("Failed to start NBD server: %s\n", err)
		}
		// nbdservers = append(nbdservers, nbdserver)

	}
	// sleep for 2 seconds to allow the NBD server to start
	time.Sleep(2 * time.Second)

	incrementalCopyCount := 0
	for {
		// If its the first copy, copy the entire disk
		if incrementalCopyCount == 0 {
			log.Println("Copying disk")
			for idx, vmdisk := range vminfo.VMDisks {
				err = nbdops[idx].CopyDisk(vmdisk.Path)
				if err != nil {
					return vminfo, fmt.Errorf("Failed to copy disk: %s\n", err)
				}
				log.Printf("Disk copied successfully: %s\n", vminfo.VMDisks[idx].Path)
			}
		} else if incrementalCopyCount > 20 {
			log.Println("20 incremental copies done, will proceed with the conversion now")
			break
		} else {
			migration_snapshot, err := vmops.GetSnapshot("migration-snap")
			if err != nil {
				return vminfo, fmt.Errorf("Failed to get snapshot: %s\n", err)
			}

			var changedAreas types.DiskChangeInfo
			done := true

			for idx, _ := range vminfo.VMDisks {
				// done = true
				// changedAreas, err = source_vm.QueryChangedDiskAreas(ctx, initial_snapshot, final_snapshot, disk, 0)
				changedAreas, err = vmops.CustomQueryChangedDiskAreas(vminfo.VMDisks[idx].ChangeID, migration_snapshot, vminfo.VMDisks[idx].Disk, 0)
				if err != nil {
					return vminfo, fmt.Errorf("Failed to get changed disk areas: %s\n", err)
				}

				if len(changedAreas.ChangedArea) == 0 {
					log.Println("No changed blocks found. Skipping copy")
				} else {
					log.Println("Blocks have Changed.")

					log.Println("Restarting NBD server")
					err = nbdops[idx].StopNBDServer()
					if err != nil {
						return vminfo, fmt.Errorf("Failed to stop NBD server: %s\n", err)
					}

					err = nbdops[idx].StartNBDServer(vmops.GetVMObj(), envURL, envUserName, envPassword, thumbprint, vminfo.VMDisks[idx].Snapname, vminfo.VMDisks[idx].SnapBackingDisk)
					if err != nil {
						return vminfo, fmt.Errorf("Failed to start NBD server: %s\n", err)
					}
					// sleep for 2 seconds to allow the NBD server to start
					time.Sleep(2 * time.Second)

					// 11. Copy Changed Blocks over
					done = false
					log.Println("Copying changed blocks")
					err = nbdops[idx].CopyChangedBlocks(changedAreas, vminfo.VMDisks[idx].Path)
					if err != nil {
						return vminfo, fmt.Errorf("Failed to copy changed blocks: %s\n", err)
					}
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
			return vminfo, fmt.Errorf("Failed to update disk info: %s\n", err)
		}
		err = vmops.DeleteSnapshot("migration-snap")
		if err != nil {
			return vminfo, fmt.Errorf("Failed to delete snapshot of source VM: %s\n", err)
		}
		err = vmops.TakeSnapshot("migration-snap")
		if err != nil {
			return vminfo, fmt.Errorf("Failed to take snapshot of source VM: %s\n", err)
		}

		incrementalCopyCount += 1

	}
	log.Println("Stopping NBD server")
	for _, nbdserver := range nbdops {
		err = nbdserver.StopNBDServer()
		if err != nil {
			return vminfo, fmt.Errorf("Failed to stop NBD server: %s\n", err)
		}
	}

	log.Println("Deleting migration snapshot")
	err = vmops.DeleteSnapshot("migration-snap")
	if err != nil {
		return vminfo, fmt.Errorf("Failed to delete snapshot of source VM: %s\n", err)
	}
	return vminfo, nil
}

func ConvertDisks(vminfo vm.VMInfo, convert bool, virtiowin string) error {
	if convert {
		// Fix NTFS
		if vminfo.OSType == "windows" {
			err := virtv2v.NTFSFix(vminfo.VMDisks[0].Path)
			if err != nil {
				return fmt.Errorf("Failed to run ntfsfix: %s\n", err)
			}
		}

		err := virtv2v.ConvertDisk(vminfo.VMDisks[0].Path, vminfo.OSType, virtiowin)

		if err != nil {
			return fmt.Errorf("Failed to run virt-v2v: %s\n", err)
		}
	}

	if vminfo.OSType == "linux" {
		// Add Wildcard Netplan
		log.Println("Adding wildcard netplan")
		err := virtv2v.AddWildcardNetplan(vminfo.VMDisks[0].Path)
		if err != nil {
			return fmt.Errorf("Failed to add wildcard netplan: %s\n", err)
		}
		log.Println("Wildcard netplan added successfully")
	}
	return nil
}

func DetachAllDisks(vminfo vm.VMInfo, openstackops openstack.OpenstackOperations) error {
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.DetachVolumeFromVM(vmdisk.OpenstackVol.ID)
		if err != nil {
			return fmt.Errorf("Failed to detach volume from VM: %s\n", err)
		}
		err = openstackops.WaitForVolume(vmdisk.OpenstackVol.ID)
		if err != nil {
			return fmt.Errorf("Failed to wait for volume to become available: %s\n", err)
		}
		log.Printf("Volume %s detached from VM\n", vmdisk.Name)
	}
	time.Sleep(1 * time.Second)
	return nil
}

func DeleteAllDisks(vminfo vm.VMInfo, openstackops openstack.OpenstackOperations) error {
	for _, vmdisk := range vminfo.VMDisks {
		err := openstackops.DeleteVolume(vmdisk.OpenstackVol.ID)
		if err != nil {
			return fmt.Errorf("Failed to delete volume: %s\n", err)
		}
		log.Printf("Volume %s deleted\n", vmdisk.Name)
	}
	return nil
}

func CreateTargetInstance(vminfo vm.VMInfo, openstackops openstack.OpenstackOperations, networkname string) error {
	closestFlavour, err := openstackops.GetClosestFlavour(vminfo.CPU, vminfo.Memory)
	if err != nil {
		return fmt.Errorf("Failed to get closest OpenStack flavor: %s\n", err)
	}
	log.Printf("Closest OpenStack flavor: %s: CPU: %dvCPUs\tMemory: %dMB\n", closestFlavour.Name, closestFlavour.VCPUs, closestFlavour.RAM)

	// Create Port Group with the same mac address as the source VM
	// Find the network with the given ID
	networkid, err := openstackops.GetNetworkID(networkname)
	if err != nil {
		return fmt.Errorf("Failed to get network ID: %s\n", err)
	}
	log.Printf("Network ID: %s\n", networkid)

	port, err := openstackops.CreatePort(networkid, vminfo)
	if err != nil {
		return fmt.Errorf("Failed to create port group: %s\n", err)
	}

	log.Printf("Port Group created successfully: MAC:%s IP:%s\n", port.MACAddress, port.FixedIPs[0].IPAddress)

	// Create a new VM in OpenStack
	newVM, err := openstackops.CreateVM(closestFlavour, networkid, port, vminfo)
	if err != nil {
		return fmt.Errorf("Failed to create VM: %s\n", err)
	}
	log.Printf("VM created successfully: ID: %s\n", newVM.ID)
	return nil
}
