package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type VMInfo struct {
	Name             string
	OSDetails        string
	DiskSize         int // In Bytes
	RDM              bool
	IndependentDisks bool
	VTPM             bool
	Encrypted        bool
}

func validateVCenter(username, password, host string, disableSSLVerification bool) (*vim25.Client, error) {
	if host[:4] != "https" {
		host = "https://" + host
	}
	if host[len(host)-4:] != "/sdk" {
		host += "/sdk"
	}
	u, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	u.User = url.UserPassword(username, password)
	s := &cache.Session{
		URL:      u,
		Insecure: disableSSLVerification,
		Reauth:   true,
	}

	c := new(vim25.Client)
	err = s.Login(context.Background(), c, nil)
	if err != nil {
		return nil, err
	}
	property.DefaultCollector(c)

	return c, nil
}

func convertToCSV(vms []VMInfo, fileName string) error {
	// Create the CSV file
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Create a CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write the header
	header := []string{"Name", "OS Details", "Disk Size (Bytes)", "RDM", "IndependentDisks", "VTPM", "Encrypted"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write each VMInfo as a row
	for _, vm := range vms {
		row := []string{
			vm.Name,
			vm.OSDetails,
			strconv.Itoa(vm.DiskSize),
			strconv.FormatBool(vm.RDM),
			strconv.FormatBool(vm.IndependentDisks),
			strconv.FormatBool(vm.VTPM),
			strconv.FormatBool(vm.Encrypted),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Parse command line arguments
	username := flag.String("username", "", "vCenter username")
	password := flag.String("password", "", "vCenter password")
	host := flag.String("host", "", "vCenter host")

	flag.Parse()

	c, err := validateVCenter(*username, *password, *host, true)
	if err != nil {
		fmt.Errorf("failed to validate vCenter: %w", err)
	}
	defer cancel()
	fmt.Println("Connected to vCenter")

	finder := find.NewFinder(c, false)
	dc, err := finder.Datacenter(ctx, "PNAP BMC")
	if err != nil {
		fmt.Printf("failed to find datacenter: %v", err)
		return
	}
	finder.SetDatacenter(dc)

	// Get all the vms
	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		fmt.Printf("failed to get vms: %v", err)
		return
	}
	fmt.Printf("Retrieved %d VMs\n", len(vms))

	vminfolist := []VMInfo{}
	for idx, vm := range vms {
		if idx%10 == 0 {
			fmt.Printf("Processing VM %d\n", idx)
		}
		rdm := false
		independentdisks := false
		vtpm := false
		encrypted := false

		var vmProps mo.VirtualMachine
		err = vm.Properties(ctx, vm.Reference(), []string{}, &vmProps)
		if err != nil {
			fmt.Printf("failed to get VM properties: %v", err)
		}

		for _, device := range vmProps.Config.Hardware.Device {
			if disk, ok := device.(*types.VirtualDisk); ok {
				if _, ok := disk.Backing.(*types.VirtualDiskRawDiskMappingVer1BackingInfo); ok {
					rdm = true
				} else if _, ok := disk.Backing.(*types.VirtualDiskRawDiskVer2BackingInfo); ok {
					rdm = true
				} else if _, ok := disk.Backing.(*types.VirtualDiskPartitionedRawDiskVer2BackingInfo); ok {
					rdm = true
				}
				if backing, ok := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
					if backing.DiskMode == "independent_persistent" || backing.DiskMode == "independent_nonpersistent" {
						independentdisks = true
					}
				}
			}
		}
		if *vmProps.Summary.Config.TpmPresent {
			vtpm = true
		}
		// Have not been able to test this
		if vmProps.Summary.Runtime.CryptoState == "safe" {
			encrypted = true
		}

		vminfo := VMInfo{
			Name:             vm.Name(),
			OSDetails:        vmProps.Guest.GuestFullName,
			DiskSize:         int(vmProps.Summary.Storage.Committed),
			RDM:              rdm,
			IndependentDisks: independentdisks,
			VTPM:             vtpm,
			Encrypted:        encrypted,
		}
		vminfolist = append(vminfolist, vminfo)
	}
	// convert to csv
	err = convertToCSV(vminfolist, "vms.csv")
	if err != nil {
		fmt.Printf("failed to convert to csv: %v", err)
	}
	fmt.Println("Converted to CSV")
}
