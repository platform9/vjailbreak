// Copyright © 2024 The vjailbreak authors

package main

import (
	"context"
	"fmt"
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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func InitTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpoint("localhost:4318"), otlptracehttp.WithInsecure())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func main() {
	ctx := context.Background()
	shutdown, err := InitTracer(ctx, "vjailbreak-v2v-helper")
	if err != nil {
		fmt.Printf("Failed to init tracer: %v\n", err)
		os.Exit(1)
	}
	defer shutdown(ctx)
	tracer := otel.Tracer("vjailbreak")
	ctx, span := tracer.Start(ctx, "main")
	defer span.End()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	utils.WriteToLogFile(fmt.Sprintf("----- Migration started at %s for VM %s -----", time.Now().Format(time.RFC3339), os.Getenv("SOURCE_VM_NAME")))

	ctx, span = tracer.Start(ctx, "reporter.NewReporter")
	eventReporter, err := reporter.NewReporter()
	span.End()
	if err != nil {
		utils.PrintLog(fmt.Sprintf("Failed to create reporter: %v", err))
		return
	}

	eventReporterChan := make(chan string)
	podLabelWatcherChan := make(chan string)
	ackChan := make(chan struct{})
	defer close(eventReporterChan)
	defer close(podLabelWatcherChan)

	eventReporter.UpdatePodEvents(ctx, eventReporterChan, ackChan)
	eventReporter.WatchPodLabels(ctx, podLabelWatcherChan)

	handleError := func(msg string) {
		if reporter.IsRunningInPod() {
			eventReporterChan <- msg
			<-ackChan
		}
		utils.PrintLog(msg)
		return
	}

	ctx, span = tracer.Start(ctx, "utils.GetInclusterClient")
	client, err := utils.GetInclusterClient()
	span.End()
	if err != nil {
		handleError(fmt.Sprintf("Failed to get in-cluster client: %v", err))
	}

	ctx, span = tracer.Start(ctx, "utils.GetMigrationParams")
	migrationparams, err := utils.GetMigrationParams(ctx, client)
	span.End()
	if err != nil {
		handleError(fmt.Sprintf("Failed to get migration parameters: %v", err))
	}

	var (
		vCenterURL        = strings.TrimSpace(os.Getenv("VCENTER_HOST"))
		vCenterUserName   = strings.TrimSpace(os.Getenv("VCENTER_USERNAME"))
		vCenterPassword   = strings.TrimSpace(os.Getenv("VCENTER_PASSWORD"))
		vCenterInsecure   = strings.EqualFold(strings.TrimSpace(os.Getenv("VCENTER_INSECURE")), constants.TrueString)
		openstackInsecure = strings.EqualFold(strings.TrimSpace(os.Getenv("OS_INSECURE")), constants.TrueString)
	)

	starttime, _ := time.Parse(time.RFC3339, migrationparams.DataCopyStart)
	cutstart, _ := time.Parse(time.RFC3339, migrationparams.VMcutoverStart)
	cutend, _ := time.Parse(time.RFC3339, migrationparams.VMcutoverEnd)

	ctx, span = tracer.Start(ctx, "vcenter.VCenterClientBuilder")
	vcclient, err := vcenter.VCenterClientBuilder(ctx, vCenterUserName, vCenterPassword, vCenterURL, vCenterInsecure)
	span.End()
	if err != nil {
		handleError(fmt.Sprintf("Failed to validate vCenter connection: %v", err))
	}
	utils.PrintLog(fmt.Sprintf("Connected to vCenter: %s", vCenterURL))

	ctx, span = tracer.Start(ctx, "openstack.NewOpenStackClients")
	openstackclients, err := openstack.NewOpenStackClients(openstackInsecure)
	span.End()
	if err != nil {
		handleError(fmt.Sprintf("Failed to validate OpenStack connection: %v", err))
	}
	utils.PrintLog("Connected to OpenStack")

	ctx, span = tracer.Start(ctx, "vcenter.GetThumbprint")
	thumbprint, err := vcenter.GetThumbprint(vCenterURL)
	span.End()
	if err != nil {
		handleError(fmt.Sprintf("Failed to get thumbprint: %s", err))
	}
	utils.PrintLog(fmt.Sprintf("VCenter Thumbprint: %s", thumbprint))

	ctx, span = tracer.Start(ctx, "vm.VMOpsBuilder")
	vmops, err := vm.VMOpsBuilder(ctx, *vcclient, migrationparams.SourceVMName)
	span.End()
	if err != nil {
		handleError(fmt.Sprintf("Failed to get source VM: %v", err))
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
		PodLabelWatcher:  podLabelWatcherChan,
		InPod:            reporter.IsRunningInPod(),
		MigrationTimes: migrate.MigrationTimes{
			DataCopyStart:  starttime,
			VMCutoverStart: cutstart,
			VMCutoverEnd:   cutend,
		},
		MigrationType:          migrationparams.MigrationType,
		PerformHealthChecks:    migrationparams.PerformHealthChecks,
		HealthCheckPort:        migrationparams.HealthCheckPort,
		K8sClient:              client,
		TargetFlavorId:         migrationparams.TARGET_FLAVOR_ID,
		TargetAvailabilityZone: migrationparams.TargetAvailabilityZone,
		AssignedIP:             migrationparams.AssignedIP,
	}

	ctx, span = tracer.Start(ctx, "migrationobj.MigrateVM")
	if err := migrationobj.MigrateVM(ctx); err != nil {
		span.End()
		msg := fmt.Sprintf("Failed to migrate VM: %v", err)

		ctx, span = tracer.Start(ctx, "vmops.VMPowerOn")
		powerOnErr := vmops.VMPowerOn()
		span.End()

		if powerOnErr != nil {
			msg += fmt.Sprintf("\nAlso Failed to power on VM after migration failure: %v", powerOnErr)
		} else {
			msg += fmt.Sprintf("\nVM %s was powered on after migration failure", migrationparams.SourceVMName)
		}

		handleError(msg)
	}
	span.End()

	utils.PrintLog(fmt.Sprintf("----- Migration completed successfully at %s for VM %s -----", time.Now().Format(time.RFC3339), migrationparams.SourceVMName))
}
