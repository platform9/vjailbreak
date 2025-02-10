// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/migrate"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/reporter"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

func removeEmptyStrings(slice []string) []string {
	var result []string
	for _, str := range slice {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	var envURL = os.Getenv("VCENTER_HOST")
	var envUserName = os.Getenv("VCENTER_USERNAME")
	var envPassword = os.Getenv("VCENTER_PASSWORD")
	var envInsecure = os.Getenv("VCENTER_INSECURE")
	var sourcevmname = os.Getenv("SOURCE_VM_NAME")
	var networknames = os.Getenv("NEUTRON_NETWORK_NAMES")
	var networkports = os.Getenv("NEUTRON_PORT_IDS")
	var volumeTypes = os.Getenv("CINDER_VOLUME_TYPES")
	var virtiowin = os.Getenv("VIRTIO_WIN_DRIVER")
	var ostype = strings.ToLower(os.Getenv("OS_TYPE"))
	var envconvert = os.Getenv("CONVERT")
	var datacopystart = os.Getenv("DATACOPYSTART")
	var vmcutoverstart = os.Getenv("CUTOVERSTART")
	var vmcutoverend = os.Getenv("CUTOVEREND")
	var migrationtype = os.Getenv("TYPE")
	var envopenstackinsecure = os.Getenv("OS_INSECURE")
	var performhealthchecks = os.Getenv("PERFORM_HEALTH_CHECKS")
	var healthcheckport = os.Getenv("HEALTH_CHECK_PORT")
	var debug = os.Getenv("DEBUG")

	log.Println("URL:", envURL)
	log.Println("Username:", envUserName)
	log.Println("Insecure:", envInsecure)
	log.Println("Source VM Name:", sourcevmname)
	log.Println("OS Type:", ostype)
	log.Println("Network Names:", strings.Split(networknames, ","))
	networkportslice := removeEmptyStrings(strings.Split(networkports, ","))
	log.Println("Network Ports:", networkportslice)
	log.Println("Volume Types:", strings.Split(volumeTypes, ","))

	starttime, _ := time.Parse(time.RFC3339, datacopystart)
	cutstart, _ := time.Parse(time.RFC3339, vmcutoverstart)
	cutend, _ := time.Parse(time.RFC3339, vmcutoverend)
	log.Println("Data Copy Start Time:", starttime)
	log.Println("VM Cutover Start Time:", cutstart)
	log.Println("VM Cutover End Time:", cutend)
	log.Println("Migration Type:", migrationtype)
	log.Println("Perform Health Checks:", performhealthchecks)
	log.Println("Health Check Port:", healthcheckport)
	log.Println("Debug:", debug)

	insecure, _ := strconv.ParseBool(envInsecure)
	convert, _ := strconv.ParseBool(envconvert)
	openstackinsecure, _ := strconv.ParseBool(envopenstackinsecure)
	performhealthchecksbool, _ := strconv.ParseBool(performhealthchecks)
	debugbool, _ := strconv.ParseBool(debug)

	// Validate vCenter and Openstack connection
	vcclient, err := vcenter.VCenterClientBuilder(ctx, envUserName, envPassword, envURL, insecure)
	if err != nil {
		log.Fatalf("Failed to validate vCenter connection: %v", err)
	}
	log.Printf("Connected to vCenter: %s\n", envURL)

	// IMP: Must have one from OS_DOMAIN_NAME or OS_DOMAIN_ID only set in the rc file
	openstackclients, err := openstack.NewOpenStackClients(openstackinsecure)
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
	vmops, err := vm.VMOpsBuilder(ctx, *vcclient, sourcevmname)
	if err != nil {
		log.Fatalf("Failed to get source VM: %s\n", err)
	}

	migrationobj := migrate.Migrate{
		URL:              envURL,
		UserName:         envUserName,
		Password:         envPassword,
		Insecure:         insecure,
		Networknames:     strings.Split(networknames, ","),
		Networkports:     networkportslice,
		Volumetypes:      strings.Split(volumeTypes, ","),
		Virtiowin:        virtiowin,
		Ostype:           ostype,
		Thumbprint:       thumbprint,
		Convert:          convert,
		Openstackclients: openstackclients,
		Vcclient:         vcclient,
		VMops:            vmops,
		Nbdops:           []nbd.NBDOperations{},
		EventReporter:    make(chan string),
		PodLabelWatcher:  make(chan string),
		InPod:            reporter.IsRunningInPod(),
		MigrationTimes: migrate.MigrationTimes{
			DataCopyStart:  starttime,
			VMCutoverStart: cutstart,
			VMCutoverEnd:   cutend,
		},
		MigrationType:       migrationtype,
		PerformHealthChecks: performhealthchecksbool,
		HealthCheckPort:     healthcheckport,
		Debug:               debugbool,
	}

	eventReporter, err := reporter.NewReporter()
	if err != nil {
		log.Fatalf("Failed to create reporter: %s\n", err)
	}
	eventReporter.UpdatePodEvents(ctx, migrationobj.EventReporter)
	eventReporter.WatchPodLabels(ctx, migrationobj.PodLabelWatcher)

	err = migrationobj.MigrateVM(ctx)
	if err != nil {
		cancel()
		log.Fatalf("Failed to migrate VM: %s\n", err)
	}

	cancel()
}
