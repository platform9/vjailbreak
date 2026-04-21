export interface VolumeImageProfile {
  apiVersion: string
  kind: string
  metadata: VolumeImageProfileMetadata
  spec: VolumeImageProfileSpec
  status?: Record<string, never>
}

export interface VolumeImageProfileMetadata {
  name: string
  namespace: string
  creationTimestamp?: string
  uid?: string
  resourceVersion?: string
}

export type VolumeImageProfileOSFamily = 'windowsGuest' | 'linuxGuest' | 'any'

export interface VolumeImageProfileSpec {
  osFamily: VolumeImageProfileOSFamily
  properties: Record<string, string>
  description?: string
}

export const OS_FAMILY_LABEL: Record<string, string> = {
  windowsGuest: 'Windows',
  linuxGuest: 'Linux',
  any: 'Any'
}

export interface VolumeImageProfileList {
  apiVersion: string
  kind: string
  metadata: { resourceVersion: string }
  items: VolumeImageProfile[]
}

export const KNOWN_IMAGE_PROPERTIES = [
  { key: 'hw_firmware_type', hint: 'uefi | bios' },
  { key: 'hw_machine_type', hint: 'q35 | pc-i440fx' },
  { key: 'hw_disk_bus', hint: 'virtio | scsi | ide' },
  { key: 'hw_scsi_model', hint: 'virtio-scsi | buslogic' },
  { key: 'hw_tpm_model', hint: 'tpm-crb | tpm-tis' },
  { key: 'hw_tpm_version', hint: '1.2 | 2.0' },
  { key: 'os_secure_boot', hint: 'required | disabled | optional' },
  { key: 'os_require_quiesce', hint: 'yes | no' },
  { key: 'os_type', hint: 'windows | linux' },
  { key: 'hw_qemu_guest_agent', hint: 'yes | no' },
  { key: 'hw_video_model', hint: 'virtio | qxl | vga' },
  { key: 'hw_cdrom_bus', hint: 'sata | ide | virtio' },
  { key: 'hw_boot_menu', hint: 'true | false' },
  { key: 'hw_pointer_model', hint: 'usbtablet | ps2mouse' }
]

export const DEFAULT_PROFILE_NAMES = ['default-windows', 'default-linux']
