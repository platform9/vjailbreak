package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"vjailbreak/migrate"
	"vjailbreak/nbd"
	"vjailbreak/openstack"
	"vjailbreak/vcenter"
	"vjailbreak/vm"
)

func main() {
	var envURL = os.Getenv("VCENTER_HOST")
	var envUserName = os.Getenv("VCENTER_USERNAME")
	var envPassword = os.Getenv("VCENTER_PASSWORD")
	var envInsecure = os.Getenv("VCENTER_INSECURE")
	var sourcevmname = os.Getenv("SOURCE_VM_NAME")
	var networkname = os.Getenv("NEUTRON_NETWORK_NAME")
	var virtiowin = os.Getenv("VIRTIO_WIN_DRIVER")
	var ostype = strings.ToLower(os.Getenv("OS_TYPE"))
	var envconvert = os.Getenv("CONVERT")

	log.Println("URL:", envURL)
	log.Println("Username:", envUserName)
	log.Println("Insecure:", envInsecure)
	log.Println("Source VM Name:", sourcevmname)
	log.Println("OS Type:", ostype)
	log.Println("Network ID:", networkname)

	insecure, _ := strconv.ParseBool(envInsecure)
	convert, _ := strconv.ParseBool(envconvert)

	// Validate vCenter and Openstack connection
	vcclient, err := vcenter.VCenterClientBuilder(envUserName, envPassword, envURL, insecure)
	if err != nil {
		log.Fatalf("Failed to validate vCenter connection: %v", err)
	}
	log.Printf("Connected to vCenter: %s\n", envURL)

	// IMP: Must have one from OS_DOMAIN_NAME or OS_DOMAIN_ID only set in the rc file
	openstackclients, err := openstack.NewOpenStackClients()
	if err != nil {
		log.Fatalf("Failed to validate OpenStack connection: %v", err)
	}
	log.Println("Connected to OpenStack")

	// Get thumbprint
	thumbprint, err := vcenter.GetThumbprint(envURL)
	if err != nil {
		log.Fatalf("Failed to get thumbprint: %s\n", err)
	}
	log.Printf("VCenter Thumbprint: %s\n", thumbprint)

	// Retrieve the source VM
	vmops, err := vm.VMOpsBuilder(*vcclient, sourcevmname)
	if err != nil {
		log.Fatalf("Failed to get source VM: %s\n", err)
	}

	// Get Info about VM
	vminfo, err := vmops.GetVMInfo(ostype)
	if err != nil {
		log.Fatalf("Failed to get all info: %s\n", err)
	}

	// Create and Add Volumes to Host
	vminfo, err = migrate.AddVolumestoHost(vminfo, openstackclients)
	if err != nil {
		log.Fatalf("Failed to add volumes to host: %s\n", err)
	}

	// Enable CBT
	err = migrate.EnableCBTWrapper(vmops)
	if err != nil {
		log.Fatalf("CBT Failure: %s\n", err)
	}

	nbdops := []nbd.NBDOperations{}
	for idx, _ := range vminfo.VMDisks {
		nbdops[idx].NewNBDNBDServer()
	}

	// Live Replicate Disks
	vminfo, err = migrate.LiveReplicateDisks(vminfo, vmops, nbdops, envURL, envUserName, envPassword, thumbprint)
	if err != nil {
		log.Printf("Failed to live replicate disks: %s\n", err)
		log.Println("Removing migration snapshot and Openstack volumes.")
		err = migrate.DetachAllDisks(vminfo, openstackclients)
		if err != nil {
			log.Fatalf("Failed to detach all volumes from VM: %s\n", err)
		}
		err = migrate.DeleteAllDisks(vminfo, openstackclients)
		if err != nil {
			log.Fatalf("Failed to delete all volumes from host: %s\n", err)
		}
		os.Exit(1)
	}

	// Convert the Boot Disk to raw format
	err = migrate.ConvertDisks(vminfo, convert, virtiowin)
	if err != nil {
		log.Fatalf("Failed to convert disks: %s\n", err)

	}

	// Detatch all volumes from VM
	err = migrate.DetachAllDisks(vminfo, openstackclients)
	if err != nil {
		log.Fatalf("Failed to detach all volumes from VM: %s\n", err)
	}

	err = migrate.CreateTargetInstance(vminfo, openstackclients, networkname)
	if err != nil {
		log.Fatalf("Failed to create target instance: %s\n", err)
	}
}
