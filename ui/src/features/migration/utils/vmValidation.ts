import type { VmData } from 'src/features/migration/api/migration-templates/model'

export type VmOsValidationResult = {
  hasError: boolean
  errorMessage: string
}

const isPoweredOn = (vm: VmData): boolean => {
  // Keep behavior identical to existing code: vmState === 'running' => powered-on
  return vm.vmState === 'running'
}

const isOsMissingOrUnknown = (vm: VmData): boolean => {
  return !vm.osFamily || vm.osFamily === 'Unknown' || vm.osFamily.trim() === ''
}

export const validateSelectedVmsOsAssigned = (vms: VmData[] | undefined): VmOsValidationResult => {
  if (!vms || vms.length === 0) {
    return { hasError: false, errorMessage: '' }
  }

  const poweredOffVMs = vms.filter((vm) => !isPoweredOn(vm))
  const poweredOnVMs = vms.filter((vm) => isPoweredOn(vm))

  const vmsWithoutOSAssigned = poweredOffVMs
    .filter(isOsMissingOrUnknown)
    .concat(poweredOnVMs.filter(isOsMissingOrUnknown))

  if (vmsWithoutOSAssigned.length > 0) {
    const count = vmsWithoutOSAssigned.length
    const errorMessage =
      `Cannot proceed with migration: ` +
      `We could not detect the operating system for ${count} VM${count === 1 ? '' : 's'}. ` +
      `Please assign the required information before continuing.`

    return { hasError: true, errorMessage }
  }

  return { hasError: false, errorMessage: '' }
}
