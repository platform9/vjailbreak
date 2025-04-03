// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/platform9/vjailbreak/v2v-helper/migrate"
	"github.com/platform9/vjailbreak/v2v-helper/nbd"
	"github.com/platform9/vjailbreak/v2v-helper/openstack"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/constants"
	"github.com/platform9/vjailbreak/v2v-helper/pkg/utils"
	"github.com/platform9/vjailbreak/v2v-helper/reporter"
	"github.com/platform9/vjailbreak/v2v-helper/vcenter"
	"github.com/platform9/vjailbreak/v2v-helper/vm"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	client, err := utils.GetInclusterClient()
	if err != nil {
		log.Fatalf("Failed to get in-cluster client: %v", err)
	}

	migrationparams, err := utils.GetMigrationParams(ctx, client)
	if err != nil {
		log.Fatalf("Failed to get migration parameters: %v", err)
	}
	var vCenterURL = strings.TrimSpace(os.Getenv("VCENTER_HOST"))
	var vCenterUserName = strings.TrimSpace(os.Getenv("VCENTER_USERNAME"))
	var vCenterPassword = strings.TrimSpace(os.Getenv("VCENTER_PASSWORD"))
	var vCenterInsecure = strings.TrimSpace(os.Getenv("VCENTER_INSECURE")) == constants.TrueString
	var openstackInsecure = strings.TrimSpace(os.Getenv("OS_INSECURE")) == constants.TrueString

	starttime, _ := time.Parse(time.RFC3339, migrationparams.DataCopyStart)
	cutstart, _ := time.Parse(time.RFC3339, migrationparams.VMcutoverStart)
	cutend, _ := time.Parse(time.RFC3339, migrationparams.VMcutoverEnd)

	// Validate vCenter and Openstack connection
	vcclient, err := vcenter.VCenterClientBuilder(ctx, vCenterUserName, vCenterPassword, vCenterURL, vCenterInsecure)
	if err != nil {
		log.Fatalf("Failed to migrate VM: Failed to validate vCenter connection: %v", err)
	}
	log.Printf("Connected to vCenter: %s\n", vCenterURL)

	// IMP: Must have one from OS_DOMAIN_NAME or OS_DOMAIN_ID only set in the rc file
	openstackclients, err := openstack.NewOpenStackClients(openstackInsecure)
	if err != nil {
		log.Fatalf("Failed to migrate VM: Failed to validate OpenStack connection: %v", err)
	}
	log.Println("Connected to OpenStack")

	// Get thumbprint
	thumbprint, err := vcenter.GetThumbprint(vCenterURL)
	if err != nil {
		log.Fatalf("Failed to migrate VM: Failed to get thumbprint: %s\n", err)
	}
	log.Printf("VCenter Thumbprint: %s\n", thumbprint)

	// Retrieve the source VM
	vmops, err := vm.VMOpsBuilder(ctx, *vcclient, migrationparams.SourceVMName)
	if err != nil {
		log.Fatalf("Failed to migrate VM: Failed to get source VM: %v", err)
	}

	migrationobj := migrate.Migrate{
		URL:              vCenterURL,
		UserName:         vCenterUserName,
		Password:         vCenterPassword,
		Insecure:         vCenterInsecure,
		Networknames:     utils.RemoveEmptyStrings(strings.Split(migrationparams.OpenstackNetworkNames, ",")),
		Networkports:     utils.RemoveEmptyStrings(strings.Split(migrationparams.OpenstackNetworkPorts, ",")),
		Volumetypes:      utils.RemoveEmptyStrings(strings.Split(migrationparams.OpenstackVolumeTypes, ",")),
		Virtiowin:        migrationparams.OpenstackVirtioWin,
		Ostype:           migrationparams.OpenstackOSType,
		Thumbprint:       thumbprint,
		Convert:          migrationparams.OpenstackConvert,
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
		MigrationType:       migrationparams.MigrationType,
		PerformHealthChecks: migrationparams.PerformHealthChecks,
		HealthCheckPort:     migrationparams.HealthCheckPort,
		K8sClient:           client,
	}

	eventReporter, err := reporter.NewReporter()
	if err != nil {
		log.Fatalf("Failed to migrate VM: Failed to create reporter: %v", err)
	}
	eventReporter.UpdatePodEvents(ctx, migrationobj.EventReporter)
	eventReporter.WatchPodLabels(ctx, migrationobj.PodLabelWatcher)

	err = migrationobj.MigrateVM(ctx)
	if err != nil {
		msg := fmt.Sprintf("Failed to migrate VM: %s\n", err)
		cancel()
		if migrationobj.InPod {
			migrationobj.EventReporter <- msg
		}
		log.Fatalf(msg)

		// Power on the VM
		poweronerr := vmops.VMPowerOn()
		if poweronerr != nil {
			log.Fatalf("Failed to power on VM after migration failure: %s\n", poweronerr)
		}

		log.Printf("VM powered on after migration failure\n")
	}
	cancel()
}
