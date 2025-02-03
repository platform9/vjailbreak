// Copyright © 2024 The vjailbreak authors

package migrate

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/openstack"

	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/utils"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/virtv2v"
	"github.com/platform9/vjailbreak/v2v-helper/vm"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/vmware/govmomi/vim25/types"
)

type Migrate struct {
	URL                 string
	UserName            string
	Password            string
	Insecure            bool
	Networknames        []string
	Networkports        []string
	Volumetypes         []string
	Virtiowin           string
	Ostype              string
	Thumbprint          string
	Convert             bool
	Openstackclients    openstack.OpenstackOperations
	Vcclient            vcenter.VCenterOperations
	VMops               vm.VMOperations
	Nbdops              []nbd.NBDOperations
	EventReporter       chan string
	PodLabelWatcher     chan string
	InPod               bool
	MigrationTimes      MigrationTimes
	MigrationType       string
	PerformHealthChecks bool
	HealthCheckPort     string
	Debug               bool
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
		if vminfo.VMDisks[idx].Boot {
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

// This function enables CBT on the VM if it is not enabled and takes a snapshot for initializing CBT
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
		migobj.logMessage("Creating temporary snapshot of the source VM")
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

func (migobj *Migrate) WaitforAdminCutover() error {
	migobj.logMessage("Waiting for Cutover conditions to be met")
	for {
		label := <-migobj.PodLabelWatcher
		migobj.logMessage(fmt.Sprintf("Label: %s", label))
		if label == "yes" {
			break
		}
	}
	migobj.logMessage("Cutover conditions met")
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
					err = nbdops[idx].CopyChangedBlocks(ctx, changedAreas, vminfo.VMDisks[idx].Path)
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
				if err := migobj.WaitforAdminCutover(); err != nil {
					return vminfo, fmt.Errorf("failed to start Admin initated Cutover: %s", err)
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
	osRelease := ""
	bootVolumeIndex := 0
	getBootCommand := ""

	if vminfo.OSType == "windows" {
		getBootCommand = "ls /Windows"
	} else if vminfo.OSType == "linux" {
		getBootCommand = "ls /boot"
	} else {
		getBootCommand = "inspect-os"
	}

	for idx, _ := range vminfo.VMDisks {
		path, err := migobj.AttachVolume(vminfo.VMDisks[idx])
		if err != nil {
			return fmt.Errorf("failed to attach volume: %s", err)
		}
		ans, err := RunCommandInGuest(path, getBootCommand)
		if err != nil {
			fmt.Printf("Error running '%s'. Error: '%s', Output: %s\n", getBootCommand, err, ans)
			detachError := migobj.DetachVolume(vminfo.VMDisks[idx])
			if detachError != nil {
				return fmt.Errorf("failed to detach volume: %s", detachError)
			}
			continue
		}

		fmt.Printf("Output from '%s' - '%s'\n", getBootCommand, ans)

		if ans == "" {
			err := migobj.DetachVolume(vminfo.VMDisks[idx])
			if err != nil {
				return fmt.Errorf("failed to detach volume: %s", err)
			}
			continue
		}

		if vminfo.OSType == "linux" {
			osRelease, err = virtv2v.GetOsRelease(path)
			if err != nil {
				return fmt.Errorf("failed to get os release: %s", err)
			}
		}

		// save the index of bootVolume
		bootVolumeIndex = idx
		log.Printf("Setting up boot volume as: %s", vminfo.VMDisks[bootVolumeIndex].Name)

		vminfo.VMDisks[bootVolumeIndex].Boot = true
		if migobj.Convert {
			firstbootscripts := []string{}
			// Fix NTFS
			if vminfo.OSType == "windows" {
				err = virtv2v.NTFSFix(path)
				if err != nil {
					return fmt.Errorf("failed to run ntfsfix: %s", err)
				}
			}
			// Turn on DHCP for interfaces in rhel VMs
			if vminfo.OSType == "linux" {
				if strings.Contains(osRelease, "rhel") {
					firstbootscriptname := "rhel_enable_dhcp"
					firstbootscript := utils.RhelFirstBootScript
					firstbootscripts = append(firstbootscripts, firstbootscriptname)
					err = virtv2v.AddFirstBootScript(firstbootscript, firstbootscriptname)
					if err != nil {
						return fmt.Errorf("failed to add first boot script: %s", err)
					}
				}
			}
			err := virtv2v.ConvertDisk(ctx, path, vminfo.OSType, migobj.Virtiowin, firstbootscripts)
			if err != nil {
				return fmt.Errorf("failed to run virt-v2v: %s", err)
			}
			openstackops := migobj.Openstackclients
			err = openstackops.SetVolumeBootable(vminfo.VMDisks[bootVolumeIndex].OpenstackVol)
			if err != nil {
				return fmt.Errorf("failed to set volume as bootable: %s", err)
			}
		}
	}
	//TODO(omkar): can disable DHCP here
	if vminfo.OSType == "linux" {
		if strings.Contains(osRelease, "ubuntu") {
			// Add Wildcard Netplan
			log.Println("Adding wildcard netplan")
			err := virtv2v.AddWildcardNetplan(vminfo.VMDisks[bootVolumeIndex].Path)
			if err != nil {
				return fmt.Errorf("failed to add wildcard netplan: %s", err)
			}
			log.Println("Wildcard netplan added successfully")
		}
	}
	err := migobj.DetachVolume(vminfo.VMDisks[bootVolumeIndex])
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
	ipaddresses := []string{}
	portids := []string{}

	if len(migobj.Networkports) != 0 {
		if len(migobj.Networkports) != len(networknames) {
			return fmt.Errorf("number of network ports does not match number of network names")
		}
		for _, port := range migobj.Networkports {
			retrPort, err := openstackops.GetPort(port)
			if err != nil {
				return fmt.Errorf("failed to get port: %s", err)
			}
			networkids = append(networkids, retrPort.NetworkID)
			portids = append(portids, retrPort.ID)
			ipaddresses = append(ipaddresses, retrPort.FixedIPs[0].IPAddress)
		}
	} else {
		for idx, networkname := range networknames {
			// Create Port Group with the same mac address as the source VM
			// Find the network with the given ID
			network, err := openstackops.GetNetwork(networkname)
			if err != nil {
				return fmt.Errorf("failed to get network: %s", err)
			}

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
			ipaddresses = append(ipaddresses, port.FixedIPs[0].IPAddress)
		}
	}

	// Create a new VM in OpenStack
	newVM, err := openstackops.CreateVM(closestFlavour, networkids, portids, vminfo)
	if err != nil {
		return fmt.Errorf("failed to create VM: %s", err)
	}
	migobj.logMessage(fmt.Sprintf("VM created successfully: ID: %s", newVM.ID))

	if migobj.PerformHealthChecks {
		err = migobj.HealthCheck(vminfo, ipaddresses)
		if err != nil {
			migobj.logMessage(fmt.Sprintf("Health Check failed: %s", err))
		}
	} else {
		migobj.logMessage("Skipping Health Checks")
	}

	return nil
}

func (migobj *Migrate) pingVM(ips []string) error {
	for _, ip := range ips {
		migobj.logMessage(fmt.Sprintf("Pinging VM: %s", ip))
		pinger, err := probing.NewPinger(ip)
		if err != nil {
			return fmt.Errorf("failed to create pinger: %s", err)
		}
		pinger.Count = 1
		pinger.Timeout = time.Second * 10
		err = pinger.Run()
		if err != nil {
			return fmt.Errorf("failed to run pinger: %s", err)
		}
		if pinger.Statistics().PacketLoss == 0 {
			migobj.logMessage("Ping succeeded")
		} else {
			return fmt.Errorf("Ping failed")
		}
	}
	return nil
}

func (migobj *Migrate) checkHTTPGet(ips []string, port string) error {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: time.Second * 10,
	}
	for _, ip := range ips {
		// Try HTTP first
		httpURL := fmt.Sprintf("http://%s:%s", ip, port)
		if err := migobj.tryConnection(client, httpURL); err == nil {
			migobj.logMessage("HTTP succeeded")
			continue // Success with HTTP, move to next IP
		}

		// If HTTP fails, try HTTPS
		httpsURL := fmt.Sprintf("https://%s:%s", ip, port)
		if err := migobj.tryConnection(client, httpsURL); err == nil {
			migobj.logMessage("HTTPS succeeded")
			continue // Success with HTTPS, move to next IP
		}

		// Both HTTP and HTTPS failed
		return fmt.Errorf("Both HTTP and HTTPS failed for %s:%s", ip, port)
	}

	return nil
}

func (migobj *Migrate) tryConnection(client *http.Client, url string) error {
	resp, err := client.Get(url)
	if err != nil {
		migobj.logMessage(fmt.Sprintf("GET failed for %s: %v", url, err))
		return err
	}
	defer resp.Body.Close()

	migobj.logMessage(fmt.Sprintf("GET response for %s: %d", url, resp.StatusCode))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET returned non-OK status for %s: %d", url, resp.StatusCode)
	}

	return nil
}

func (migobj *Migrate) HealthCheck(vminfo vm.VMInfo, ips []string) error {
	migobj.logMessage("Performing Health Checks")
	healthChecks := make(map[string]bool)
	healthChecks["Ping"] = false
	healthChecks["HTTP Get"] = false
	for i := 0; i < len(vminfo.IPs); i++ {
		if ips[i] != vminfo.IPs[i] {
			migobj.logMessage(fmt.Sprintf("VM has been assigned a new IP: %s instead of the original IP %s. Using the new IP for tests", ips[i], vminfo.IPs[i]))
		}
	}
	for i := 0; i < 10; i++ {
		migobj.logMessage(fmt.Sprintf("Health Check Attempt %d", i+1))
		// 1. Ping
		if !healthChecks["Ping"] {
			err := migobj.pingVM(ips)
			if err != nil {
				migobj.logMessage(fmt.Sprintf("Ping(s) failed: %s", err))
			} else {
				healthChecks["Ping"] = true
			}
		}
		// 2. HTTP GET check
		if !healthChecks["HTTP Get"] {
			err := migobj.checkHTTPGet(ips, migobj.HealthCheckPort)
			if err != nil {
				migobj.logMessage(fmt.Sprintf("HTTP Get failed: %s", err))
			} else {
				healthChecks["HTTP Get"] = true
			}
		}
		if healthChecks["Ping"] && healthChecks["HTTP Get"] {
			break
		}
		migobj.logMessage("Waiting for 60 seconds before retrying health checks")
		time.Sleep(60 * time.Second)
	}
	for key, value := range healthChecks {
		if !value {
			migobj.logMessage(fmt.Sprintf("Health Check %s failed", key))
		} else {
			migobj.logMessage(fmt.Sprintf("Health Check %s succeeded", key))
		}
	}
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
	if len(vminfo.VMDisks) != len(migobj.Volumetypes) {
		return fmt.Errorf("number of volume types does not match number of disks")
	}
	if len(vminfo.Mac) != len(migobj.Networknames) {
		return fmt.Errorf("number of mac addresses does not match number of network names")
	}

	// Graceful Termination clean-up volumes and snapshots
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
		migobj.Nbdops = append(migobj.Nbdops, &nbd.NBDServer{
			Debug: migobj.Debug,
		})
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

// Runs command inside temporary qemu-kvm that virt-v2v creates
func RunCommandInGuest(path string, command string) (string, error) {
	// Get the os-release file
	os.Setenv("LIBGUESTFS_BACKEND", "direct")
	cmd := exec.Command(
		"guestfish",
		"--ro",
		"-a",
		path,
		"-i")
	cmd.Stdin = strings.NewReader(command)
	log.Printf("Executing %s", cmd.String()+" "+command)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to run command (%s): %v", command, err)
	}
	return strings.ToLower(string(out)), nil
}
