/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package migrationplan

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	vjailbreakv1alpha1 "github.com/platform9/vjailbreak/k8s/migration/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// StatusUpdater is an interface for updating migration plan status
type StatusUpdater interface {
	UpdateMigrationPlanStatus(ctx context.Context, migrationplan *vjailbreakv1alpha1.MigrationPlan, phase corev1.PodPhase, message string) error
}

// VMFetcher is an interface for fetching VMware machines
type VMFetcher interface {
	GetVMwareMachineForVM(ctx context.Context, vm string, migrationtemplate *vjailbreakv1alpha1.MigrationTemplate, vmwcreds *vjailbreakv1alpha1.VMwareCreds) (*vjailbreakv1alpha1.VMwareMachine, error)
}

// ValidateMigrationPlanVMs validates all VMs in the migration plan
func ValidateMigrationPlanVMs(
	ctx context.Context,
	migrationplan *vjailbreakv1alpha1.MigrationPlan,
	vmFetcher VMFetcher,
	logger logr.Logger,
	statusUpdater StatusUpdater,
	migrationtemplate *vjailbreakv1alpha1.MigrationTemplate,
	vmwcreds *vjailbreakv1alpha1.VMwareCreds) ([]*vjailbreakv1alpha1.VMwareMachine, []*vjailbreakv1alpha1.VMwareMachine, error) {
	var (
		validVMs, skippedVMs []*vjailbreakv1alpha1.VMwareMachine
	)

	if len(migrationplan.Spec.VirtualMachines) == 0 {
		return nil, nil, fmt.Errorf("no VMs to migrate in migration plan")
	}

	for _, vmGroup := range migrationplan.Spec.VirtualMachines {
		for _, vm := range vmGroup {
			vmMachine, err := vmFetcher.GetVMwareMachineForVM(ctx, vm, migrationtemplate, vmwcreds)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get VMwareMachine for VM %s: %w", vm, err)
			}

			_, skipped, err := validateVMOS(vmMachine, logger)
			if err != nil {
				return nil, nil, err
			}
			if skipped {
				skippedVMs = append(skippedVMs, vmMachine)
				continue
			}

			validVMs = append(validVMs, vmMachine)
		}
	}

	if len(validVMs) == 0 {
		if len(skippedVMs) > 0 {
			skippedVMNames := make([]string, len(skippedVMs))
			for i, vm := range skippedVMs {
				skippedVMNames[i] = vm.Spec.VMInfo.Name
			}
			msg := fmt.Sprintf("Skipped VMs due to unsupported or unknown OS: %v", skippedVMNames)
			logger.Info(msg)
		}
		return nil, skippedVMs, fmt.Errorf("all VMs have unknown or unsupported OS types; no migrations to run")
	}

	if len(skippedVMs) > 0 {
		skippedVMNames := make([]string, len(skippedVMs))
		for i, vm := range skippedVMs {
			skippedVMNames[i] = vm.Spec.VMInfo.Name
		}
		msg := fmt.Sprintf("Skipped VMs due to unsupported or unknown OS: %v", skippedVMNames)
		logger.Info(msg)
		if updateErr := statusUpdater.UpdateMigrationPlanStatus(ctx, migrationplan, corev1.PodPending, msg); updateErr != nil {
			logger.Error(updateErr, "Failed to update migration plan status for skipped VMs")
		}
	}

	return validVMs, skippedVMs, nil
}

// validateVMOS validates that the VM has a valid OS type
func validateVMOS(vmMachine *vjailbreakv1alpha1.VMwareMachine, logger logr.Logger) (bool, bool, error) {
	validOSTypes := []string{"windowsGuest", "linuxGuest"}
	osFamily := strings.TrimSpace(vmMachine.Spec.VMInfo.OSFamily)

	if osFamily == "" || osFamily == "unknown" {
		logger.Info("VM has unknown or unspecified OS type and will be skipped",
			"vmName", vmMachine.Spec.VMInfo.Name)
		return false, true, nil
	}

	for _, validOS := range validOSTypes {
		if osFamily == validOS {
			return true, false, nil
		}
	}

	return false, false, fmt.Errorf("vm '%s' has an unsupported OS type: %s",
		vmMachine.Spec.VMInfo.Name, osFamily)
}
