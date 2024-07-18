package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"
	"vjailbreak/nbd"
	"vjailbreak/vcenter"
	"vjailbreak/vm"

	"github.com/vmware/govmomi/vim25/types"
)

func main() {
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()
	var envURL = os.Getenv("VCENTER_HOST")
	var envUserName = os.Getenv("VCENTER_USERNAME")
	var envPassword = os.Getenv("VCENTER_PASSWORD")
	var envInsecure = os.Getenv("VCENTER_INSECURE")
	var sourcevmname = os.Getenv("SOURCE_VM_NAME")
	var networkname = os.Getenv("NEUTRON_NETWORK_NAME")
	var ostype = strings.ToLower(os.Getenv("OS_TYPE"))
	var virtiowin = os.Getenv("VIRTIO_WIN_DRIVER")
	var envconvert = os.Getenv("CONVERT")

	log.Println("URL:", envURL)
	log.Println("Username:", envUserName)
	log.Println("Insecure:", envInsecure)
	log.Println("Source VM Name:", sourcevmname)
	log.Println("Network ID:", networkname)
	log.Println("OS Type:", ostype)

	insecure, _ := strconv.ParseBool(envInsecure)
	convert, _ := strconv.ParseBool(envconvert)

	// 1. Validate vCenter and Openstack connection
	vcclient, err := vcenter.VCenterClientBuilder(envUserName, envPassword, envURL, insecure)
	if err != nil {
		log.Fatalf("Failed to validate vCenter connection: %v", err)
	}
	// ctx = context.WithValue(ctx, "govmomi_client", vcclient)
	log.Printf("Connected to vCenter: %s\n", envURL)

	// IMP: Must have one from OS_DOMAIN_NAME or OS_DOMAIN_ID only set in the rc file
	openstackclients, err := OpenStackClientsBuilder()
	if err != nil {
		log.Fatalf("Failed to validate OpenStack connection: %v", err)
	}
	// ctx = context.WithValue(ctx, "openstack_clients", openstackclients)
	log.Println("Connected to OpenStack")

	// 2. Get thumbprint
	thumbprint, err := vcenter.GetThumbprint(envURL)
	if err != nil {
		log.Fatalf("Failed to get thumbprint: %s\n", err)
	}
	log.Printf("VCenter Thumbprint: %s\n", thumbprint)

	// 3. Retrieve the source VM
	// source_vm, err := vcclient.GetVMByName(sourcevmname)
	// if err != nil {
	// 	log.Fatalf("Failed to get source VM: %s\n", err)
	// }
	// if source_vm == nil {
	// 	log.Fatalf("Source VM not found")
	// }
	// log.Printf("Source VM: %+v\n", source_vm)
	// ctx = context.WithValue(ctx, "vm", source_vm)
	// log.Println("Source VM retrieved successfully")

	// 3. Retrieve the source VM
	vmops, err := vm.VMOpsBuilder(*vcclient, sourcevmname)
	if err != nil {
		log.Fatalf("Failed to get source VM: %s\n", err)
	}

	// 4. Get Info about VM
	vminfo, err := vmops.GetVMInfo()
	if err != nil {
		log.Fatalf("Failed to get all info: %s\n", err)
	}

	// Get the disks of the VM
	// log.Printf("VM Disk Info: %+v\n", vminfo.VMDisks)

	// log.Printf("VM MAC Info: %+v\n", vminfo.Mac)

	// 5. Create a new volume in openstack
	log.Println("Creating volumes in OpenStack")
	for idx, vmdisk := range vminfo.VMDisks {
		volume, err := openstackclients.CreateVolume(vminfo.Name+"-"+vmdisk.Name, vmdisk.Size, ostype, vminfo.UEFI)
		if err != nil {
			log.Fatalf("Failed to create volume: %s\n", err)
		}
		vminfo.VMDisks[idx].OpenstackVol = volume
		if idx == 0 {
			err = openstackclients.SetVolumeBootable(volume)
			if err != nil {
				log.Fatalf("Failed to set volume as bootable: %s\n", err)
			}
		}
	}
	log.Println("Volumes created successfully")

	log.Println("Attaching volumes to VM")
	for _, vmdisk := range vminfo.VMDisks {
		err = openstackclients.AttachVolumeToVM(vmdisk.OpenstackVol.ID)
		if err != nil {
			log.Fatalf("Failed to attach volume to VM: %s\n", err)
		}
		log.Printf("Volume attached to VM: %s\n", vmdisk.OpenstackVol.Name)
	}

	// Get the Path of the attached volume
	for idx, vmdisk := range vminfo.VMDisks {
		devicePath, err := findDevice(vmdisk.OpenstackVol.ID)
		if err != nil {
			log.Fatalf("Failed to find device: %s\n", err)
		}
		vminfo.VMDisks[idx].Path = devicePath
		log.Printf("Volume %s attached successfully at %s\n", vmdisk.Name, vminfo.VMDisks[idx].Path)
	}

	// 7. Check If CBT is enabled
	cbt, err := vmops.IsCBTEnabled()
	if err != nil {
		log.Fatalf("Failed to check if CBT is enabled: %s\n", err)
	}
	log.Printf("CBT Enabled: %t\n", cbt)

	if !cbt {
		// 7.5. Enable CBT
		log.Println("CBT is not enabled. Enabling CBT")
		err = vmops.EnableCBT()
		if err != nil {
			log.Fatalf("Failed to enable CBT: %s\n", err)
		}
		_, err := vmops.IsCBTEnabled()
		if err != nil {
			log.Fatalf("Failed to check if CBT is enabled: %s\n", err)
		}
		log.Println("CBT enabled successfully")

		log.Println("Creating temporary snapshot of the source VM")
		err = vmops.TakeSnapshot("tmp-snap")
		if err != nil {
			log.Fatalf("Failed to take snapshot of source VM: %s\n", err)
		}
		log.Println("Snapshot created successfully")
		err = vmops.DeleteSnapshot("tmp-snap")
		if err != nil {
			log.Fatalf("Failed to delete snapshot of source VM: %s\n", err)
		}
		log.Println("Snapshot deleted successfully")
	}

	// 9. Start NBD Server
	log.Println("Starting NBD server")
	err = vmops.TakeSnapshot("migration-snap")
	if err != nil {
		log.Fatalf("Failed to take snapshot of source VM: %s\n", err)
	}

	vminfo, err = vmops.UpdateDiskInfo(vminfo)
	if err != nil {
		log.Fatalf("Failed to update disk info: %s\n", err)
	}

	// log.Printf("Before starting NBD %+v\n", vminfo.VMDisks)

	var nbdservers []nbd.NBDServer
	for _, vmdisk := range vminfo.VMDisks {
		nbdserver, err := nbd.StartNBDServer(vmops.VMObj, envURL, envUserName, envPassword, thumbprint, vmdisk.Snapname, vmdisk.SnapBackingDisk)
		if err != nil {
			log.Fatalf("Failed to start NBD server: %s\n", err)
		}
		nbdservers = append(nbdservers, nbdserver)

	}
	// sleep for 2 seconds to allow the NBD server to start
	time.Sleep(2 * time.Second)

	incrementalCopyCount := 0
	for {
		// If its the first copy, copy the entire disk
		if incrementalCopyCount == 0 {
			log.Println("Copying disk")
			for idx, vmdisk := range vminfo.VMDisks {
				err = nbdservers[idx].CopyDisk(vmdisk.Path)
				if err != nil {
					log.Fatalf("Failed to copy disk: %s\n", err)
				}
				log.Printf("Disk copied successfully: %s\n", vminfo.VMDisks[idx].Path)
			}
		} else if incrementalCopyCount > 20 {
			log.Println("20 incremental copies done, will proceed with the conversion now")
			break
		} else {
			migration_snapshot, err := vmops.GetSnapshot("migration-snap")
			if err != nil {
				log.Fatalf("Failed to get snapshot: %s\n", err)
			}

			var changedAreas types.DiskChangeInfo
			done := true

			for idx, _ := range vminfo.VMDisks {
				// done = true
				// changedAreas, err = source_vm.QueryChangedDiskAreas(ctx, initial_snapshot, final_snapshot, disk, 0)
				changedAreas, err = vmops.CustomQueryChangedDiskAreas(vminfo.VMDisks[idx].ChangeID, migration_snapshot, vminfo.VMDisks[idx].Disk, 0)
				if err != nil {
					log.Fatalf("Failed to get changed disk areas: %s\n", err)
				}

				if len(changedAreas.ChangedArea) == 0 {
					log.Println("No changed blocks found. Skipping copy")
				} else {
					log.Println("Blocks have Changed.")

					log.Println("Restarting NBD server")
					err = nbdservers[idx].StopNBDServer()
					if err != nil {
						log.Fatalf("Failed to stop NBD server: %s\n", err)
					}

					nbdservers[idx], err = nbd.StartNBDServer(vmops.VMObj, envURL, envUserName, envPassword, thumbprint, vminfo.VMDisks[idx].Snapname, vminfo.VMDisks[idx].SnapBackingDisk)
					// sleep for 2 seconds to allow the NBD server to start
					time.Sleep(2 * time.Second)

					// 11. Copy Changed Blocks over
					done = false
					log.Println("Copying changed blocks")
					err = nbdservers[idx].CopyChangedBlocks(changedAreas, vminfo.VMDisks[idx].Path)
					if err != nil {
						log.Fatalf("Failed to copy changed blocks: %s\n", err)
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
			log.Fatalf("Failed to update snapshot info: %s\n", err)
		}
		err = vmops.DeleteSnapshot("migration-snap")
		if err != nil {
			log.Fatalf("Failed to delete snapshot of source VM: %s\n", err)
		}
		err = vmops.TakeSnapshot("migration-snap")
		if err != nil {
			log.Fatalf("Failed to take snapshot of source VM: %s\n", err)
		}

		incrementalCopyCount += 1

	}
	// run v2v only for the first disk as it is the boot disk
	if convert {
		// Fix NTFS
		if ostype == "windows" {
			err = NTFSFix(vminfo.VMDisks[0].Path)
			if err != nil {
				log.Fatalf("Failed to run ntfsfix: %s\n", err)
			}
		}

		err = ConvertDisk(vminfo.VMDisks[0].Path, ostype, virtiowin)

		if err != nil {
			log.Fatalf("Failed to run virt-v2v: %s\n", err)
		}
	}

	if ostype == "linux" {
		// Add Wildcard Netplan
		log.Println("Adding wildcard netplan")
		err = AddWildcardNetplan(vminfo.VMDisks[0].Path)
		if err != nil {
			log.Fatalf("Failed to add wildcard netplan: %s\n", err)
		}
		log.Println("Wildcard netplan added successfully")
	}

	// Detatch volumes from VM
	for _, vmdisk := range vminfo.VMDisks {
		err = openstackclients.DetachVolumeFromVM(vmdisk.OpenstackVol.ID)
		if err != nil {
			log.Fatalf("Failed to detach volume from VM: %s\n", err)
		}
		log.Printf("Volume %s detached from VM\n", vmdisk.Name)
	}

	log.Println("Stopping NBD server")
	for _, nbdserver := range nbdservers {
		err = nbdserver.StopNBDServer()
		if err != nil {
			log.Fatalf("Failed to stop NBD server: %s\n", err)
		}
	}

	log.Println("Deleting migration snapshot")
	err = vmops.DeleteSnapshot("migration-snap")
	if err != nil {
		log.Fatalf("Failed to delete snapshot of source VM: %s\n", err)
	}

	// Get closest openstack flavour
	closestFlavour, err := openstackclients.GetClosestFlavour(vminfo.CPU, vminfo.Memory)
	if err != nil {
		log.Fatalf("Failed to get closest OpenStack flavor: %s\n", err)
	}
	log.Printf("Closest OpenStack flavor: %s: CPU: %dvCPUs\tMemory: %dMB\n", closestFlavour.Name, closestFlavour.VCPUs, closestFlavour.RAM)

	// Create Port Group with the same mac address as the source VM
	// Find the network with the given ID
	networkid, err := openstackclients.GetNetworkID(networkname)
	if err != nil {
		log.Fatalf("Failed to get network ID: %s\n", err)
	}
	log.Printf("Network ID: %s\n", networkid)

	port, err := openstackclients.CreatePort(networkid, vminfo)
	if err != nil {
		log.Fatalf("Failed to create port group: %s\n", err)
	}

	log.Printf("Port Group created successfully: MAC:%s IP:%s\n", port.MACAddress, port.FixedIPs[0].IPAddress)

	// Create a new VM in OpenStack
	newVM, err := openstackclients.CreateVM(closestFlavour, networkid, port, vminfo)
	if err != nil {
		log.Fatalf("Failed to create VM: %s\n", err)
	}

	log.Printf("VM created successfully: ID: %s\n", newVM.ID)
}
