import type { VmDataWithFlavor, CanonicalVM, VM } from '../types'

/**
 * Convert standard-migration VmDataWithFlavor → CanonicalVM.
 * Maps vmState ('running'/'stopped') → powerState ('powered-on'/'powered-off')
 * Maps ipAddress → ip, cpuCount → cpu, flavor/flavorName → flavor.
 */
export function fromVmDataWithFlavor(vm: VmDataWithFlavor): CanonicalVM {
  return {
    id: vm.id,
    name: vm.name,
    powerState: vm.vmState === 'running' ? 'powered-on' : 'powered-off',
    ip: vm.ipAddress || '—',
    cpu: vm.cpuCount,
    memory: vm.memory,
    osFamily: vm.osFamily,
    flavor: vm.flavor ?? vm.flavorName,
    targetFlavorId: vm.targetFlavorId,
    networkInterfaces: vm.networkInterfaces,
    esxHost: vm.esxHost,
    networks: vm.networks,
    datastores: vm.datastores,
    preserveIp: vm.preserveIp,
    preserveMac: vm.preserveMac,
    ipValidationStatus: vm.ipValidationStatus,
    ipValidationMessage: vm.ipValidationMessage,
    isMigrated: vm.isMigrated,
    flavorNotFound: vm.flavorNotFound,
    hasSharedRdm: vm.hasSharedRdm,
    vmKey: vm.vmKey,
    vmid: vm.vmid,
    labels: vm.labels,
    vmWareMachineName: vm.vmWareMachineName,
    disks: vm.disks,
  }
}

/**
 * Convert CanonicalVM → VmDataWithFlavor for standard-migration hooks.
 * Maps powerState → vmState, ip → ipAddress, cpu → cpuCount.
 */
export function toVmDataWithFlavor(vm: CanonicalVM): VmDataWithFlavor {
  return {
    id: vm.id,
    name: vm.name,
    datastores: vm.datastores || [],
    vmState: vm.powerState === 'powered-on' ? 'running' : 'stopped',
    ipAddress: vm.ip === '—' ? undefined : vm.ip,
    cpuCount: vm.cpu,
    memory: vm.memory,
    osFamily: vm.osFamily,
    flavor: vm.flavor,
    flavorName: vm.flavor,
    targetFlavorId: vm.targetFlavorId,
    networkInterfaces: vm.networkInterfaces,
    esxHost: vm.esxHost,
    networks: vm.networks,
    preserveIp: vm.preserveIp,
    preserveMac: vm.preserveMac,
    ipValidationStatus: vm.ipValidationStatus,
    ipValidationMessage: vm.ipValidationMessage,
    isMigrated: vm.isMigrated,
    flavorNotFound: vm.flavorNotFound,
    hasSharedRdm: vm.hasSharedRdm,
    vmKey: vm.vmKey,
    vmid: vm.vmid,
    labels: vm.labels,
    vmWareMachineName: vm.vmWareMachineName,
    disks: vm.disks,
    powerState: vm.powerState,
  }
}

/**
 * Convert rolling-migration VM → CanonicalVM.
 * VM is already close to canonical shape — mostly a type-lift.
 */
export function fromVM(vm: VM): CanonicalVM {
  return {
    id: vm.id,
    name: vm.name,
    powerState: vm.powerState === 'powered-on' ? 'powered-on' : 'powered-off',
    ip: vm.ip,
    cpu: vm.cpu,
    memory: vm.memory,
    osFamily: vm.osFamily,
    flavor: vm.flavor,
    targetFlavorId: vm.targetFlavorId,
    networkInterfaces: vm.networkInterfaces,
    esxHost: vm.esxHost,
    networks: vm.networks,
    datastores: vm.datastores,
    preserveIp: vm.preserveIp,
    preserveMac: vm.preserveMac,
    ipValidationStatus: vm.ipValidationStatus,
    ipValidationMessage: vm.ipValidationMessage,
  }
}

/**
 * Convert CanonicalVM → rolling-migration VM for rolling hooks.
 */
export function toVM(vm: CanonicalVM): VM {
  return {
    id: vm.id,
    name: vm.name,
    ip: vm.ip,
    esxHost: vm.esxHost || '',
    networks: vm.networks,
    datastores: vm.datastores,
    cpu: vm.cpu,
    memory: vm.memory,
    powerState: vm.powerState,
    osFamily: vm.osFamily,
    flavor: vm.flavor,
    targetFlavorId: vm.targetFlavorId,
    ipValidationStatus: vm.ipValidationStatus,
    ipValidationMessage: vm.ipValidationMessage,
    networkInterfaces: vm.networkInterfaces,
    preserveIp: vm.preserveIp,
    preserveMac: vm.preserveMac,
  }
}
