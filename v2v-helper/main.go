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

	// Create EventReporter channel early
	eventReporterChan := make(chan string)
	podLabelWatcher := make(chan string)
	inPod := reporter.IsRunningInPod()

	// Initialize the reporter earlier
	eventReporter, err := reporter.NewReporter()
	if err != nil {
		log.Fatalf("Failed to migrate VM: Failed to create reporter: %v", err)
	}
	eventReporter.UpdatePodEvents(ctx, eventReporterChan)
	eventReporter.WatchPodLabels(ctx, podLabelWatcher)

	// Helper function to handle errors consistently and clean up resources
	handleFatalError := func(msg string, err error) {
		errorMsg := fmt.Sprintf("%s: %v", msg, err)
		if inPod {
			eventReporterChan <- errorMsg
		}
		// Cancel context to signal goroutines to exit
		cancel()
		// Close channels to prevent goroutine leaks
		close(eventReporterChan)
		close(podLabelWatcher)
		log.Fatalf(errorMsg)
	}

	migrationparams, err := utils.GetMigrationParams(ctx, client)
	if err != nil {
		handleFatalError("Failed to migrate VM: Failed to get migration parameters", err)
	}
	var vCenterURL = strings.TrimSpace(os.Getenv("VCENTER_HOST"))
	var vCenterUserName = strings.TrimSpace(os.Getenv("VCENTER_USERNAME"))
	var vCenterPassword = strings.TrimSpace(os.Getenv("VCENTER_PASSWORD"))
	var vCenterInsecure = strings.TrimSpace(os.Getenv("VCENTER_INSECURE")) == constants.TrueString
	var openstackInsecure = strings.EqualFold(strings.TrimSpace(os.Getenv("OS_INSECURE")), constants.TrueString)

	starttime, _ := time.Parse(time.RFC3339, migrationparams.DataCopyStart)
	cutstart, _ := time.Parse(time.RFC3339, migrationparams.VMcutoverStart)
	cutend, _ := time.Parse(time.RFC3339, migrationparams.VMcutoverEnd)

	// Validate vCenter and Openstack connection
	vcclient, err := vcenter.VCenterClientBuilder(ctx, vCenterUserName, vCenterPassword, vCenterURL, vCenterInsecure)
	if err != nil {
		handleFatalError("Failed to migrate VM: Failed to validate vCenter connection", err)
	}
	log.Printf("Connected to vCenter: %s\n", vCenterURL)

	// IMP: Must have one from OS_DOMAIN_NAME or OS_DOMAIN_ID only set in the rc file
	openstackclients, err := openstack.NewOpenStackClients(openstackInsecure)
	if err != nil {
		handleFatalError("Failed to migrate VM: Failed to validate OpenStack connection", err)
	}
	log.Println("Connected to OpenStack successfully")

	// Get thumbprint
	thumbprint, err := vcenter.GetThumbprint(vCenterURL)
	if err != nil {
		handleFatalError("Failed to migrate VM: Failed to get thumbprint", err)
	}
	log.Printf("VCenter Thumbprint: %s\n", thumbprint)

	// Retrieve the source VM
	vmops, err := vm.VMOpsBuilder(ctx, *vcclient, migrationparams.SourceVMName)
	if err != nil {
		handleFatalError("Failed to migrate VM: Failed to get source VM", err)
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
		EventReporter:    eventReporterChan,
		PodLabelWatcher:  podLabelWatcher,
		InPod:            inPod,
		MigrationTimes: migrate.MigrationTimes{
			DataCopyStart:  starttime,
			VMCutoverStart: cutstart,
			VMCutoverEnd:   cutend,
		},
		MigrationType:       migrationparams.MigrationType,
		PerformHealthChecks: migrationparams.PerformHealthChecks,
		HealthCheckPort:     migrationparams.HealthCheckPort,
		K8sClient:           client,
		TargetFlavorId:      migrationparams.TARGET_FLAVOR_ID,
	}

	err = migrationobj.MigrateVM(ctx)
	if err != nil {
		// Power on the VM
		poweronerr := vmops.VMPowerOn()
		if poweronerr != nil {
			log.Printf("Failed to power on VM after migration failure: %s\n", poweronerr)
			handleFatalError("Failed to migrate VM", err)
		}
		log.Printf("VM powered on after migration failure\n")
	}

}
