import { Box, Alert, Autocomplete, Checkbox, Chip, TextField } from '@mui/material'
import { useMemo, useEffect, useRef, useState } from 'react'
import {
  hasOsFamilySelected,
  isProfileApplicable,
  reconcileImageProfiles
} from '../utils/imageProfiles'
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
  // Template authoring: there is no VM selection to infer an OS family from, so offer
  // every profile in the system. Applying the template narrows the saved selection down
  // to the profiles that fit the VMs actually chosen.
  showAllProfiles?: boolean
  // Profiles saved by an applied template, kept whole so one becomes selected again as
  // soon as a VM of its OS family is chosen.
  templateImageProfiles?: string[]
}

export default function SecurityGroupAndServerGroup({
  params,
  onChange,
  openstackCredentials,
  openstackNetworks = [],
  stepNumber = '4',
  showHeader = true,
  showAllProfiles = false,
  templateImageProfiles
}: SecurityGroupAndServerGroupProps) {
  const userRemovedProfilesRef = useRef<Set<string>>(new Set())
  const securityGroupOptions: SecurityGroupOption[] =
    openstackCredentials?.status?.openstack?.securityGroups || []

  const serverGroupOptions: ServerGroupOption[] =
    openstackCredentials?.status?.openstack?.serverGroups || []

  const hasL2Network = hasSelectedLayer2Network(params.networkMappings, openstackNetworks)

  const hasWindowsVMSelected = useMemo(
    () => hasOsFamilySelected(params?.vms, 'windows'),
    [params?.vms]
  )

  const hasLinuxVMSelected = useMemo(() => hasOsFamilySelected(params?.vms, 'linux'), [params?.vms])

  const { data: volumeImageProfiles = [], isLoading: loadingProfiles } =
    useVolumeImageProfilesQuery()

  const applicableProfiles = useMemo(() => {
    const list = Array.isArray(volumeImageProfiles) ? volumeImageProfiles : []
    // Applicability is decided by the selected VMs' OS families, which don't exist while
    // authoring a template — filtering there would leave the field permanently empty.
    if (showAllProfiles) return list
    return list.filter((profile) =>
      isProfileApplicable(profile, hasWindowsVMSelected, hasLinuxVMSelected)
    )
  }, [volumeImageProfiles, hasWindowsVMSelected, hasLinuxVMSelected, showAllProfiles])

  const selectedImageProfiles: string[] = useMemo(
    () => (Array.isArray(params?.imageProfiles) ? params.imageProfiles : []),
    [params?.imageProfiles]
  )

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
    // Applicability is VM-OS-family gated (hasWindowsVMSelected/hasLinuxVMSelected), which
    // is undetermined — not "neither" — before any VM is selected (e.g. right after
    // applying a saved template, which intentionally doesn't restore VM selection). Pruning
    // here would permanently wipe a real, saved profile choice before the user even gets to
    // pick VMs. Only reconcile once VM selection is actually known.
    if (!params?.vms || params.vms.length === 0) return
    if (selectedImageProfiles.length === 0 && !templateImageProfiles?.length) return

    const applicableNames = new Set(
      applicableProfiles
        .map((profile) => profile.metadata?.name)
        .filter((name): name is string => Boolean(name))
    )

    // Prune and re-apply together: selecting a Linux VM prunes a template's Windows
    // profile, and adding a Windows VM later has to bring it back.
    const { next } = reconcileImageProfiles({
      current: selectedImageProfiles,
      pool: templateImageProfiles,
      applicableNames,
      suppressedProfiles: userRemovedProfilesRef.current
    })

    if (next === selectedImageProfiles) return
    onChange('imageProfiles')(next)
  }, [
    applicableProfiles,
    selectedImageProfiles,
    loadingProfiles,
    onChange,
    params?.vms,
    templateImageProfiles
  ])

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
            tooltip="Apply OpenStack volume image properties."
            align="flex-start"
          />
          <Autocomplete
            multiple
            disableCloseOnSelect
            size="small"
            loading={loadingProfiles}
            options={applicableProfiles}
            data-testid="volume-image-profiles-autocomplete"
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

              // Record deliberate deselections so a template can't re-add them, and clear
              // the record for anything selected again. Only a user edit can tell these
              // apart from a profile pruned because its VM was de-selected.
              const nextNames = values.map((v) => v.metadata.name)
              const nextSet = new Set(nextNames)
              selectedImageProfiles.forEach((name) => {
                if (!nextSet.has(name)) userRemovedProfilesRef.current.add(name)
              })
              nextSet.forEach((name) => userRemovedProfilesRef.current.delete(name))

              onChange('imageProfiles')(nextNames)
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
            renderOption={(props, option, { selected }) => (
              <li {...props} key={option.metadata.name}>
                <Checkbox style={{ marginRight: 8 }} checked={selected} size="small" />
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
                      ? "No profiles created. Create one at profile's page to select here"
                      : "Select profiles"
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