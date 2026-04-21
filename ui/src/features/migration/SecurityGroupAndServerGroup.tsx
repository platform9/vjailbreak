import { Box, Alert, Autocomplete, Chip, TextField } from '@mui/material'
import { useMemo, useEffect, useRef, useState } from 'react'
import { Step, RHFAutocomplete } from 'src/shared/components/forms'
import { FormGrid } from 'src/components'
import { FieldLabel } from 'src/components/design-system/ui/FieldLabel'
import {
  OpenstackCreds,
  SecurityGroupOption,
  ServerGroupOption,
  PCDNetworkInfo
} from 'src/api/openstack-creds/model'
import { ResourceMap } from './NetworkAndStorageMappingStep'
import { hasSelectedLayer2Network } from 'src/shared/utils/network'
import { useVolumeImageProfilesQuery } from 'src/hooks/api/useVolumeImageProfilesQuery'
import { VolumeImageProfile } from 'src/api/volume-image-profiles/model'

interface SecurityGroupAndServerGroupProps {
  params: {
    vms?: any[]
    securityGroups?: string[]
    serverGroup?: string
    networkMappings?: ResourceMap[]
    imageProfiles?: string[]
  }
  onChange: (key: string) => (value: any) => void
  openstackCredentials?: OpenstackCreds
  openstackNetworks?: PCDNetworkInfo[]
  stepNumber?: string
  showHeader?: boolean
}

export default function SecurityGroupAndServerGroup({
  params,
  onChange,
  openstackCredentials,
  openstackNetworks = [],
  stepNumber = '4',
  showHeader = true
}: SecurityGroupAndServerGroupProps) {
  const securityGroupOptions: SecurityGroupOption[] =
    openstackCredentials?.status?.openstack?.securityGroups || []

  const serverGroupOptions: ServerGroupOption[] =
    openstackCredentials?.status?.openstack?.serverGroups || []

  const hasL2Network = hasSelectedLayer2Network(params.networkMappings, openstackNetworks)

  const hasWindowsVMSelected = useMemo(
    () => Boolean(params?.vms?.some((vm) => vm.osFamily === 'windowsGuest')),
    [params?.vms]
  )

  const hasLinuxVMSelected = useMemo(
    () => Boolean(params?.vms?.some((vm) => vm.osFamily === 'linuxGuest')),
    [params?.vms]
  )

  const { data: volumeImageProfiles = [], isLoading: loadingProfiles } =
    useVolumeImageProfilesQuery()

  const applicableProfiles = useMemo(() => {
    const list = Array.isArray(volumeImageProfiles) ? volumeImageProfiles : []
    return list.filter((p) => {
      const fam = p.spec?.osFamily || ''
      if (fam === 'any' || !fam) return true
      if (fam === 'windowsGuest') return hasWindowsVMSelected
      if (fam === 'linuxGuest') return hasLinuxVMSelected
      return false
    })
  }, [volumeImageProfiles, hasWindowsVMSelected, hasLinuxVMSelected])

  const selectedImageProfiles: string[] = useMemo(
    () => (Array.isArray(params?.imageProfiles) ? params.imageProfiles : []),
    [params?.imageProfiles]
  )

  // Auto-select OS-matching default profiles whenever the VM selection changes.
  const lastVmsKeyRef = useRef<string>('')
  const vmsKey = (params?.vms ?? []).map((vm) => vm.name ?? vm.id ?? '').sort().join(',')
  useEffect(() => {
    if (loadingProfiles) return
    if (vmsKey === lastVmsKeyRef.current) return

    lastVmsKeyRef.current = vmsKey

    if (!params?.vms || params.vms.length === 0) return

    const current = new Set(selectedImageProfiles)
    const toAdd = applicableProfiles
      .filter((p) => {
        const name = p.metadata?.name || ''
        if (current.has(name)) return false
        const fam = p.spec?.osFamily || ''
        if (fam === 'windowsGuest' && hasWindowsVMSelected && name === 'default-windows')
          return true
        if (fam === 'linuxGuest' && hasLinuxVMSelected && name === 'default-linux')
          return true
        return false
      })
      .map((p) => p.metadata.name)

    if (toAdd.length > 0) {
      onChange('imageProfiles')([...selectedImageProfiles, ...toAdd])
    }
  }, [
    vmsKey,
    loadingProfiles,
    applicableProfiles,
    hasLinuxVMSelected,
    hasWindowsVMSelected,
    onChange,
    selectedImageProfiles,
    params?.vms
  ])

  const [profileConflictError, setProfileConflictError] = useState('')

  const detectConflict = (profiles: VolumeImageProfile[]) => {
    const scan = (bucket: VolumeImageProfile[]) => {
      const keyMap: Record<string, { value: string; profile: string }> = {}
      for (const p of bucket) {
        for (const [k, v] of Object.entries(p.spec?.properties || {})) {
          const existing = keyMap[k]
          if (existing && existing.value !== v) {
            return {
              key: k,
              profiles: [existing.profile, p.metadata.name],
              values: [existing.value, v]
            }
          }
          if (!existing) keyMap[k] = { value: v, profile: p.metadata.name }
        }
      }
      return null
    }

    const windowsBucket = profiles.filter(
      (p) => p.spec?.osFamily === 'windowsGuest' || p.spec?.osFamily === 'any'
    )
    const linuxBucket = profiles.filter(
      (p) => p.spec?.osFamily === 'linuxGuest' || p.spec?.osFamily === 'any'
    )

    return scan(windowsBucket) || scan(linuxBucket)
  }

  useEffect(() => {
    if (loadingProfiles) return
    if (selectedImageProfiles.length === 0) return
    const applicableNames = new Set(applicableProfiles.map((p) => p.metadata?.name))
    const pruned = selectedImageProfiles.filter((name) => applicableNames.has(name))
    if (pruned.length !== selectedImageProfiles.length) {
      onChange('imageProfiles')(pruned)
    }
  }, [applicableProfiles, selectedImageProfiles, loadingProfiles, onChange])

  return (
    <Box>
      {showHeader ? (
        <Step
          stepNumber={stepNumber}
          label="Security Groups, Server Group & Profiles (Optional)"
        />
      ) : null}

      <Box sx={{ display: 'grid', gap: 3 }}>
        <Box>
          {hasL2Network && (
            <Alert severity="info" sx={{ mb: 2 }}>
              Security Groups are not available when using Layer 2 Networks.
            </Alert>
          )}
          <FormGrid minWidth={320} gap={3}>
            <Box>
              <RHFAutocomplete<SecurityGroupOption>
                name="securityGroups"
                multiple
                options={securityGroupOptions}
                label="Security Groups"
                placeholder={
                  params.securityGroups && params.securityGroups.length > 0
                    ? ''
                    : 'Select Security Groups'
                }
                getOptionLabel={(option) =>
                  option.requiresIdDisplay
                    ? `${option.name} (${option.id.substring(0, 8)}...)`
                    : option.name
                }
                getOptionValue={(option) => option.id}
                renderOptionLabel={(option) =>
                  option.requiresIdDisplay
                    ? `${option.name} (${option.id.substring(0, 8)}...)`
                    : option.name
                }
                showCheckboxes
                onValueChange={(value) => onChange('securityGroups')(value)}
                data-testid="security-groups-autocomplete"
                labelProps={{ tooltip: 'Assign security groups to the selected VMs.' }}
                disabled={hasL2Network}
              />
            </Box>

            <Box>
              <RHFAutocomplete<ServerGroupOption>
                name="serverGroup"
                options={serverGroupOptions}
                label="Server Group"
                placeholder="Select Server Group"
                getOptionLabel={(option) => `${option.name} (${option.policy})`}
                getOptionValue={(option) => option.id}
                renderOptionLabel={(option) => `${option.name} (${option.policy})`}
                onValueChange={(value) => onChange('serverGroup')(value)}
                data-testid="server-group-autocomplete"
                labelProps={{ tooltip: 'Control VM affinity/anti-affinity placement.' }}
              />
            </Box>
          </FormGrid>
        </Box>

        {/* Profiles */}
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
          <FieldLabel
            label="Profiles"
            tooltip="Apply OpenStack image metadata to the boot volume. Profiles matching the selected VMs' OS family are pre-selected. Properties from later profiles override earlier ones on duplicate keys."
            align="flex-start"
          />
          <Autocomplete
            multiple
            size="small"
            loading={loadingProfiles}
            options={applicableProfiles}
            getOptionLabel={(o: VolumeImageProfile) => o.metadata?.name || ''}
            isOptionEqualToValue={(o, v) => o.metadata?.name === v.metadata?.name}
            value={applicableProfiles.filter((p) =>
              selectedImageProfiles.includes(p.metadata.name)
            )}
            onChange={(_, values) => {
              const oldNames = new Set(selectedImageProfiles)
              const added = values.filter((v) => !oldNames.has(v.metadata.name))

              if (added.length > 0) {
                const conflict = detectConflict(values)
                if (conflict) {
                  setProfileConflictError(
                    `"${added[0].metadata.name}" conflicts with "${conflict.profiles.find(
                      (n) => n !== added[0].metadata.name
                    )}": both set "${conflict.key}" but to different values ("${
                      conflict.values[0]
                    }" vs "${conflict.values[1]}"). Remove one before adding the other.`
                  )
                  return
                }
              }

              setProfileConflictError('')
              onChange('imageProfiles')(values.map((v) => v.metadata.name))
            }}
            renderTags={(value, getTagProps) =>
              value.map((option, index) => (
                <Chip
                  size="small"
                  label={`${option.metadata.name} (${option.spec.osFamily})`}
                  {...getTagProps({ index })}
                  key={option.metadata.name}
                />
              ))
            }
            renderOption={(props, option) => (
              <li {...props} key={option.metadata.name}>
                <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                  <Box component="span" sx={{ fontSize: 14 }}>{option.metadata.name}</Box>
                  <Box component="span" sx={{ fontSize: 12, color: 'text.secondary' }}>
                    {option.spec.osFamily} ·{' '}
                    {Object.keys(option.spec.properties || {}).length} prop(s)
                    {option.spec.description ? ` · ${option.spec.description}` : ''}
                  </Box>
                </Box>
              </li>
            )}
            renderInput={(params) => (
              <TextField
                {...params}
                size="small"
                placeholder={
                  selectedImageProfiles.length > 0
                    ? ''
                    : applicableProfiles.length === 0
                      ? 'No applicable profiles'
                      : 'Select profiles'
                }
              />
            )}
          />
          {profileConflictError && (
            <Alert severity="error" sx={{ mt: 1 }} onClose={() => setProfileConflictError('')}>
              {profileConflictError}
            </Alert>
          )}
        </Box>
      </Box>
    </Box>
  )
}