package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vmware/govmomi/vim25/types"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Load environment variables from .env and admin.rc file
	// This file should be in the same directory as the main.go file
	// err := godotenv.Load(".env")
	// if err != nil {
	// 	log.Fatalf("Error loading .env file")
	// }

	// err = godotenv.Load("admin.rc")
	// if err != nil {
	// 	log.Fatalf("Error loading admin.rc file")
	// }
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
	insecure, _ := strconv.ParseBool(envInsecure)
	convert, _ := strconv.ParseBool(envconvert)

	// 1. Validate vCenter and Openstack connection
	client, err := ValidateVCenter(ctx, envUserName, envPassword, envURL, insecure)
	if err != nil {
		log.Fatalf("Failed to validate vCenter connection: %v", err)
	}
	ctx = context.WithValue(ctx, "govmomi_client", client)
	log.Printf("Connected to vCenter: %s\n", envURL)

	// IMP: Must have one from OS_DOMAIN_NAME or OS_DOMAIN_ID only set in the rc file
	openstackclients, err := ValidateOpenStack(ctx)
	if err != nil {
		log.Fatalf("Failed to validate OpenStack connection: %v", err)
	}
	ctx = context.WithValue(ctx, "openstack_clients", openstackclients)
	log.Println("Connected to OpenStack")

	// 2. Get thumbprint
	thumbprint, err := GetThumbprint(envURL)
	if err != nil {
		log.Fatalf("Failed to get thumbprint: %s\n", err)
	}
	log.Printf("VCenter Thumbprint: %s\n", thumbprint)

	// data, err := getAllInfo(ctx, client, envURL, envUserName)
	// if err != nil {
	// 	log.Fatalf("Failed to get all info: %s\n", err)
	// 	return
	// }

	// 3. Retrieve the source VM
	source_vm, err := GetVMByName(ctx, sourcevmname)
	if err != nil {
		log.Fatalf("Failed to get source VM: %s\n", err)
	}
	if source_vm == nil {
		log.Fatalf("Source VM not found")
	}
	log.Printf("Source VM: %+v\n", source_vm)
	ctx = context.WithValue(ctx, "vm", source_vm)
	log.Println("Source VM retrieved successfully")

	// 4. Get Info about VM
	vminfo, err := GetVMInfo(ctx)
	if err != nil {
		log.Fatalf("Failed to get all info: %s\n", err)
	}

	// Get the disks of the VM
	log.Printf("VM Disk Info: %+v\n", vminfo.VMDisks)

	log.Printf("VM MAC Info: %+v\n", vminfo.Mac)

	// 5. Create a new volume in openstack
	for idx, vmdisk := range vminfo.VMDisks {
		volume, err := CreateVolume(ctx, vminfo.VM.Name+"-"+vmdisk.Name, vmdisk.Size, ostype, vminfo.UEFI)
		if err != nil {
			log.Fatalf("Failed to create volume: %s\n", err)
		}
		vminfo.VMDisks[idx].OpenstackVol = volume
		if idx == 0 {
			err = SetVolumeBootable(ctx, volume)
			if err != nil {
				log.Fatalf("Failed to set volume as bootable: %s\n", err)
			}
		}
	}
	log.Printf("Volumes created successfully: %+v\n", vminfo.VMDisks[0].OpenstackVol.ID)

	// TODO: 6. Mount Volumes to Appliance VM
	applianceid, err := GetCurrentInstanceUUID()
	if err != nil {
		log.Fatalf("Failed to get current instance UUID: %s\n", err)
	}

	log.Println("Attaching volumes to VM")
	for _, vmdisk := range vminfo.VMDisks {
		err = AttachVolumeToVM(ctx, vmdisk.OpenstackVol.ID, applianceid)
		if err != nil {
			log.Fatalf("Failed to attach volume to VM: %s\n", err)
		}
		log.Printf("Volume attached to VM: %+v\n", vmdisk.OpenstackVol)
	}

	// Get the Path of the attached volume
	for idx, vmdisk := range vminfo.VMDisks {
		devicePath, err := findDevice(vmdisk.OpenstackVol.ID)
		if err != nil {
			log.Fatalf("Failed to find device: %s\n", err)
		}
		vminfo.VMDisks[idx].Path = devicePath
		log.Printf("Volumes attached successfully at %s: %+v\n", vminfo.VMDisks[idx].Path, vminfo.VMDisks)
	}

	// 7. Check If CBT is enabled
	cbt, err := IsCBTEnabled(ctx)
	if err != nil {
		log.Fatalf("Failed to check if CBT is enabled: %s\n", err)
	}
	log.Printf("CBT Enabled: %t\n", cbt)

	if !cbt {
		// 7.5. Enable CBT
		log.Println("CBT is not enabled. Enabling CBT")
		err = EnableCBT(ctx)
		if err != nil {
			log.Fatalf("Failed to enable CBT: %s\n", err)
		}
		_, err := IsCBTEnabled(ctx)
		if err != nil {
			log.Fatalf("Failed to check if CBT is enabled: %s\n", err)
		}
		log.Println("CBT enabled successfully")

		// 8. Create a snapshot of the source VM to Enable cbt
		// // check if the source VM has a snapshot
		// if vminfo.VM.Snapshot != nil {
		// 	log.Fatalf("Source VM has a snapshot. Please delete the snapshot before proceeding")
		// }
		log.Println("Creating temporary snapshot of the source VM")
		err = TakeSnapshot(ctx, "tmp-snap")
		if err != nil {
			log.Fatalf("Failed to take snapshot of source VM: %s\n", err)
		}
		log.Println("Snapshot created successfully")
		err = DeleteSnapshot(ctx, "tmp-snap")
		if err != nil {
			log.Fatalf("Failed to delete snapshot of source VM: %s\n", err)
		}
		log.Println("Snapshot deleted successfully")
	}

	// 9. Start NBD Server
	log.Println("Starting NBD server")
	err = TakeSnapshot(ctx, "migration-snap")
	if err != nil {
		log.Fatalf("Failed to take snapshot of source VM: %s\n", err)
	}

	// vminfo, err = GetVMInfo(ctx)
	// if err != nil {
	// 	log.Fatalf("Failed to get all info: %s\n", err)
	// }
	// log.Printf("VM Disk Info: %+v\n", vminfo.VMDisks)
	vminfo, err = UpdateDiskInfo(ctx, vminfo)
	if err != nil {
		log.Fatalf("Failed to update disk info: %s\n", err)
	}

	log.Printf("Before starting NBD %+v\n", vminfo.VMDisks)

	var nbdservers []NBDServer
	for _, vmdisk := range vminfo.VMDisks {
		nbdserver, err := StartNBDServer(ctx, envURL, envUserName, envPassword, thumbprint, vmdisk.Snapname, vmdisk.SnapBackingDisk)
		if err != nil {
			log.Fatalf("Failed to start NBD server: %s\n", err)
		}
		nbdservers = append(nbdservers, nbdserver)

	}
	// sleep for 2 seconds to allow the NBD server to start
	time.Sleep(2 * time.Second)

	incrementalCopyCount := 0
	// oldsnapname := "migration-snap"
	for {
		// If its the firt copy, copy the entire disk
		if incrementalCopyCount == 0 {
			log.Println("Copying disk")
			for idx, vmdisk := range vminfo.VMDisks {
				err = CopyDisk(nbdservers[idx], vmdisk.Path)
				if err != nil {
					log.Fatalf("Failed to copy disk: %s\n", err)
				}
				log.Printf("Disk copied successfully: %+v\n", vminfo.VMDisks[idx].Path)
			}
		} else if incrementalCopyCount > 20 {
			log.Println("20 incremental copies done, will proceed with the conversion now")
			break
		} else {
			migration_snapshot, err := GetSnapshot(ctx, "migration-snap")
			if err != nil {
				log.Fatalf("Failed to get snapshot: %s\n", err)
			}
			log.Printf("Old ChangeID: %+v\n", vminfo.VMDisks[0].ChangeID)

			var changedAreas types.DiskChangeInfo
			done := true

			for idx, vmdisk := range vminfo.VMDisks {
				done = true
				// changedAreas, err = source_vm.QueryChangedDiskAreas(ctx, initial_snapshot, final_snapshot, disk, 0)
				changedAreas, err = CustomQueryChangedDiskAreas(ctx, vmdisk.ChangeID, migration_snapshot, vmdisk.Disk, 0)
				if err != nil {
					log.Fatalf("Failed to get changed disk areas: %s\n", err)
				}
				log.Printf("Changed Areas: %+v\n", changedAreas)

				log.Println("Restarting NBD server")
				for idx, nbdserver := range nbdservers {
					err = StopNBDServer(nbdserver)
					if err != nil {
						log.Fatalf("Failed to stop NBD server: %s\n", err)
					}

					vminfo, err = UpdateDiskInfo(ctx, vminfo)
					if err != nil {
						log.Fatalf("Failed to update snapshot info: %s\n", err)
					}
					nbdservers[idx], err = StartNBDServer(ctx, envURL, envUserName, envPassword, thumbprint, vminfo.VMDisks[idx].Snapname, vminfo.VMDisks[idx].SnapBackingDisk)
					// sleep for 2 seconds to allow the NBD server to start
					time.Sleep(2 * time.Second)
				}

				if len(changedAreas.ChangedArea) == 0 {
					log.Println("No changed blocks found. Skipping copy")
				} else {
					// 11. Copy Changed Blocks over
					done = false
					log.Println("Copying changed blocks")
					err = CopyChangedBlocks(ctx, changedAreas, nbdservers[idx], vmdisk.Path)
					if err != nil {
						log.Fatalf("Failed to copy changed blocks: %s\n", err)
					}
				}
			}
			if done {
				break
			}
		}
		err = DeleteSnapshot(ctx, "migration-snap")
		if err != nil {
			log.Fatalf("Failed to delete snapshot of source VM: %s\n", err)
		}
		err = TakeSnapshot(ctx, "migration-snap")
		if err != nil {
			log.Fatalf("Failed to take snapshot of source VM: %s\n", err)
		}

		incrementalCopyCount += 1

	}
	// run v2v only for the first disk as it is the boot disk
	if convert {
		// Fix NTFS
		if ostype == "windows" {
			err = NTFSFix(ctx, vminfo.VMDisks[0].Path)
			if err != nil {
				log.Fatalf("Failed to run ntfsfix: %s\n", err)
			}
		}

		err = ConvertDisk(ctx, vminfo.VMDisks[0].Path, ostype, virtiowin)

		if err != nil {
			log.Fatalf("Failed to run virt-v2v: %s\n", err)
		}
	}

	// Detatch volumes from VM
	for _, vmdisk := range vminfo.VMDisks {
		err = DetachVolumeFromVM(ctx, vmdisk.OpenstackVol.ID, applianceid)
		if err != nil {
			log.Fatalf("Failed to detach volume from VM: %s\n", err)
		}
		log.Printf("Volume detached from VM: %+v\n", vmdisk.OpenstackVol)
	}

	log.Println("Stopping NBD server")
	for _, nbdserver := range nbdservers {
		err = StopNBDServer(nbdserver)
		if err != nil {
			log.Fatalf("Failed to stop NBD server: %s\n", err)
		}
	}

	log.Println("DEV: deleting snapshot")
	err = DeleteSnapshot(ctx, "migration-snap")
	if err != nil {
		log.Fatalf("Failed to delete snapshot of source VM: %s\n", err)
	}

	// Get closest openstack flavour
	closestFlavour, err := GetClosestFlavour(ctx, vminfo.CPU, vminfo.Memory)
	if err != nil {
		log.Fatalf("Failed to get closest OpenStack flavor: %s\n", err)
	}
	log.Printf("Closest OpenStack flavor: %+v\n", closestFlavour)

	// Create Port Group with the same mac address as the source VM
	// Find the network with the given ID
	networkid, err := GetNetworkID(ctx, networkname)
	if err != nil {
		log.Fatalf("Failed to get network ID: %s\n", err)
	}
	log.Printf("Network ID: %s\n", networkid)

	port, err := CreatePort(ctx, networkid, vminfo)
	if err != nil {
		log.Fatalf("Failed to create port group: %s\n", err)
	}

	log.Printf("Port Group created successfully: %+v\n", port)

	// Create a new VM in OpenStack

	newVM, err := CreateVM(ctx, closestFlavour, networkid, port, vminfo)
	if err != nil {
		log.Fatalf("Failed to create VM: %s\n", err)
	}

	log.Printf("VM created successfully: %+v\n", newVM)

	// // Check if the source VM is powered off. if not, power it off
	// if source_vm.State != "poweredOff" {
	// 	log.Println("Source VM is not powered off. Powering off the VM")
	// 	task, err := source_vm.VM.PowerOff(ctx)
	// 	if err != nil {
	// 		log.Fatalf("Failed to power off source VM: %s\n", err)
	// 		return
	// 	}
	// 	err = task.Wait(ctx)
	// 	if err != nil {
	// 		log.Fatalf("Failed to wait for source VM power off task: %s\n", err)
	// 		return
	// 	}
	// 	log.Println("Source VM powered off successfully")
	// }

}
