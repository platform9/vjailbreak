import {
  Alert,
  Box,
  CircularProgress,
  Divider,
  FormControlLabel,
  Paper,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Typography
} from '@mui/material'
import LanOutlinedIcon from '@mui/icons-material/LanOutlined'
import StorageOutlinedIcon from '@mui/icons-material/StorageOutlined'
import { SyntheticEvent, useCallback, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  FieldLabel,
  KeyValueGrid,
  NavTab,
  NavTabs,
  Section,
  StatusChip,
  SurfaceCard
} from 'src/components'
import { formatDateTime, formatDiskSize } from 'src/utils'
import type { Migration } from 'src/features/migration/api/migrations'
import { useMigrationDetailResourcesQuery } from 'src/hooks/api/useMigrationDetailResourcesQuery'
import { getVolumeImageProfilesList } from 'src/api/volume-image-profiles/volumeImageProfiles'
import { OS_FAMILY_LABEL } from 'src/api/volume-image-profiles/model'

import { isDefaultishValue, normalizeMappingRows } from './helpers'
import { MIGRATION_ENVIRONMENT_FIELDS, MIGRATION_POLICY_FIELDS } from './migrationDetailConstants'

export interface MigrationDetailModalProps {
  open: boolean
  migration: Migration | null
  onClose: () => void
  isDuplicate?: boolean
}

const splitCommaSeparated = (value: unknown): string[] => {
  if (!value) return []
  return String(value)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
}

const enabledOrNA = (value: unknown) => {
  return value === true ? 'Enabled' : 'N/A'
}

function MappingTable({
  rows,
  sourceLabel,
  targetLabel,
  emptyLabel,
  sourceIcon,
  targetIcon
}: {
  rows: Array<{ source: string; target: string }>
  sourceLabel: string
  targetLabel: string
  emptyLabel: string
  sourceIcon?: React.ReactNode
  targetIcon?: React.ReactNode
}) {
  if (!rows.length) {
    return <Typography variant="body2">{emptyLabel}</Typography>
  }

  return (
    <TableContainer component={Paper} variant="outlined">
      <Table size="small" sx={{ tableLayout: 'fixed' }}>
        <TableHead>
          <TableRow>
            <TableCell sx={{ width: '50%' }}>{sourceLabel}</TableCell>
            <TableCell sx={{ width: '50%' }}>{targetLabel}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {rows.map((row, idx) => (
            <TableRow key={`${row.source}-${row.target}-${idx}`}>
              <TableCell sx={{ width: '50%' }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
                  {sourceIcon}
                  <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                    {row.source}
                  </Typography>
                </Box>
              </TableCell>
              <TableCell sx={{ width: '50%' }}>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
                  {targetIcon}
                  <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                    {row.target}
                  </Typography>
                </Box>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  )
}

export default function MigrationDetailModal({
  open,
  migration,
  onClose,
  isDuplicate = false
}: MigrationDetailModalProps) {
  const [tab, setTab] = useState(0)
  const [onlyShowOverrides, setOnlyShowOverrides] = useState(true)

  useEffect(() => {
    if (open) {
      setTab(0)
      setOnlyShowOverrides(true)
    }
  }, [open])

  const vmName = useMemo(() => ((migration?.spec as any)?.vmName as string) || '', [migration])
  const vmKey =
    ((migration?.metadata as any)?.labels?.['vjailbreak.k8s.pf9.io/vm-key'] as string) || ''
  const displayVmName = isDuplicate && vmKey ? vmKey : vmName
  const phase = migration?.status?.phase
  const phaseLabel = (phase as string) || 'Unknown'

  const { data, isLoading, error } = useMigrationDetailResourcesQuery({ open, migration })

  const migrationSpec = (migration?.spec as any) || {}
  const migrationStatus = (migration?.status as any) || {}

  const vmSpec = (data?.vmwareMachine?.spec as any)?.vms || {}
  const vmMeta = (data?.vmwareMachine?.metadata as any) || {}

  const createdAt = formatDateTime(migration?.metadata?.creationTimestamp)

  const planSpec = (data?.migrationPlan?.spec as any) || {}
  const planStrategy = (planSpec?.migrationStrategy as any) || {}
  const planAdvanced = (planSpec?.advancedOptions as any) || {}
  const planPostAction = (planSpec?.postMigrationAction as any) || {}

  const assignedIpRaw =
    (migrationSpec?.assignedIP as string) ||
    (migrationSpec?.assignedIp as string) ||
    (migrationSpec?.assignedIpAddress as string) ||
    ''
  const assignedIpFromPlan =
    ((planSpec?.assignedIPsPerVM as Record<string, string> | undefined) || {})[vmName] || ''

  const { assignedIps, preservedIps } = useMemo(() => {
    const NA: string = 'N/A'

    const sourceIfaces = Array.isArray(vmSpec?.networkInterfaces)
      ? (vmSpec.networkInterfaces as any[])
      : []

    const sourceIpsPerIndex = sourceIfaces.map((nic) => {
      const ips = Array.isArray(nic?.ipAddress)
        ? nic.ipAddress
        : nic?.ipAddress
          ? [nic.ipAddress]
          : []
      return ips.map((ip) => String(ip || '').trim()).filter(Boolean)
    })

    const rawOverrides = migrationSpec?.networkOverrides
    let parsedOverrides: any[] = []
    if (rawOverrides) {
      try {
        parsedOverrides = Array.isArray(rawOverrides)
          ? rawOverrides
          : JSON.parse(String(rawOverrides))
      } catch {
        parsedOverrides = []
      }
    }

    const overrides = new Map<number, { preserveIP: boolean; userAssignedIps: string[] }>()
    for (const item of Array.isArray(parsedOverrides) ? parsedOverrides : []) {
      const idx = Number(item?.interfaceIndex)
      if (Number.isNaN(idx)) continue
      const preserveIP = item?.preserveIP !== false
      const userAssignedIps = splitCommaSeparated(item?.UserAssignedIP)
      overrides.set(idx, { preserveIP, userAssignedIps })
    }

    const maxOverrideIndex = Math.max(-1, ...Array.from(overrides.keys()))
    const interfaceCount = Math.max(sourceIpsPerIndex.length, maxOverrideIndex + 1, 1)

    const assignedPerIndex: string[] = []
    const preservedPerIndex: string[] = []

    for (let idx = 0; idx < interfaceCount; idx += 1) {
      const ov = overrides.get(idx)
      const sourceIps = sourceIpsPerIndex[idx] || []

      if (ov) {
        if (ov.preserveIP) {
          assignedPerIndex.push(NA)
          preservedPerIndex.push(sourceIps.length ? sourceIps.join(', ') : NA)
        } else {
          assignedPerIndex.push(ov.userAssignedIps.length ? ov.userAssignedIps.join(', ') : NA)
          preservedPerIndex.push(NA)
        }
      } else {
        assignedPerIndex.push(NA)
        preservedPerIndex.push(sourceIps.length ? sourceIps.join(', ') : NA)
      }
    }

    const legacyAssigned = splitCommaSeparated(assignedIpRaw || assignedIpFromPlan)
    const allAssignedNA = assignedPerIndex.every((v) => v === NA)
    if (legacyAssigned.length && allAssignedNA) {
      assignedPerIndex.splice(0, 1, legacyAssigned.join(', '))
    }

    const assignedJoined = assignedPerIndex.every((v) => v === NA)
      ? NA
      : assignedPerIndex.join(', ')

    const preservedJoined = preservedPerIndex.every((v) => v === NA)
      ? NA
      : preservedPerIndex.join(', ')

    return {
      assignedIps: assignedJoined,
      preservedIps: preservedJoined
    }
  }, [
    assignedIpFromPlan,
    assignedIpRaw,
    migrationSpec?.networkOverrides,
    vmSpec?.networkInterfaces
  ])

  const initiateCutoverEnabled = migrationSpec?.initiateCutover === true

  const templateSpec = (data?.migrationTemplate?.spec as any) || {}
  const useFlavorless = templateSpec?.useFlavorless === true
  const useGPUFlavor = templateSpec?.useGPUFlavor === true
  const storageCopyMethod = (templateSpec?.storageCopyMethod as string) || 'normal'
  const isStorageAcceleratedCopy = storageCopyMethod === 'StorageAcceleratedCopy'

  const sourceDatacenter =
    (vmMeta?.annotations?.['vjailbreak.k8s.pf9.io/datacenter'] as string) ||
    (templateSpec?.source?.datacenter as string) ||
    'N/A'
  const sourceCluster =
    (vmSpec?.clusterName as string) ||
    (vmMeta?.labels?.['vjailbreak.k8s.pf9.io/vmware-cluster'] as string) ||
    'N/A'
  const esxiHost =
    (vmSpec?.esxiName as string) ||
    (vmMeta?.labels?.['vjailbreak.k8s.pf9.io/esxi-name'] as string) ||
    'N/A'

  const guestOS = (vmSpec?.osFamily as string) || 'N/A'
  const cpu = typeof vmSpec?.cpu === 'number' ? String(vmSpec.cpu) : 'N/A'
  const memory = typeof vmSpec?.memory === 'number' ? `${vmSpec.memory} MB` : 'N/A'

  const diskCount = useMemo(() => {
    const disks = vmSpec?.disks
    if (Array.isArray(disks)) return String(disks.length)
    return 'N/A'
  }, [vmSpec?.disks])

  const networkAdapterCount = useMemo(() => {
    const ifaces = vmSpec?.networkInterfaces
    if (Array.isArray(ifaces)) return String(ifaces.length)
    const networks = vmSpec?.networks
    if (Array.isArray(networks)) return String(networks.length)
    return 'N/A'
  }, [vmSpec?.networkInterfaces, vmSpec?.networks])

  const destinationCluster = (templateSpec?.targetPCDClusterName as string) || 'N/A'

  // Destination tenant is derived from the referenced OpenStack creds when available.
  // If that creds was deleted, derive the tenant using the destination cluster -> PCDCluster label
  // mapping to an OpenStack creds name, and then resolve its projectName from the remaining creds list.
  const openstackCredNameToProjectName = useMemo(() => {
    const entries = (data?.openstackCredsList || [])
      .map((c) => {
        const name = String(c?.metadata?.name || '').trim()
        const project = String((c?.spec as any)?.projectName || '').trim()
        return name && project ? ([name, project] as const) : null
      })
      .filter(Boolean) as Array<readonly [string, string]>

    return new Map(entries)
  }, [data?.openstackCredsList])

  const destinationClusterToOpenstackCredName = useMemo(() => {
    const entries = (data?.pcdClusters || [])
      .map((c) => {
        const clusterName = String((c as any)?.spec?.clusterName || '').trim()
        const openstackCredName = String(
          (c as any)?.metadata?.labels?.['vjailbreak.k8s.pf9.io/openstackcreds'] || ''
        ).trim()

        return clusterName && openstackCredName ? ([clusterName, openstackCredName] as const) : null
      })
      .filter(Boolean) as Array<readonly [string, string]>

    return new Map(entries)
  }, [data?.pcdClusters])

  const destinationClusterToProjectNameFromHostConfig = useMemo(() => {
    const map = new Map<string, string>()
    for (const cred of data?.openstackCredsList || []) {
      const projectName = String((cred?.spec as any)?.projectName || '').trim()
      if (!projectName) continue
      const hostConfigs = ((cred?.spec as any)?.pcdHostConfig as any[]) || []
      for (const cfg of hostConfigs) {
        const clusterName = String((cfg as any)?.clusterName || '').trim()
        if (clusterName && !map.has(clusterName)) {
          map.set(clusterName, projectName)
        }
      }
    }
    return map
  }, [data?.openstackCredsList])

  const destinationTenant = useMemo(() => {
    const direct = ((data?.openstackCreds?.spec as any)?.projectName as string) || ''
    if (direct) return direct

    const clusterName = String(destinationCluster || '').trim()
    if (!clusterName || clusterName === 'N/A') return 'N/A'

    const mappedCredName = destinationClusterToOpenstackCredName.get(clusterName)
    if (mappedCredName) {
      const mappedProjectName = openstackCredNameToProjectName.get(mappedCredName)
      if (mappedProjectName) return mappedProjectName
    }

    const fromHostConfig = destinationClusterToProjectNameFromHostConfig.get(clusterName)
    if (fromHostConfig) return fromHostConfig

    return 'N/A'
  }, [
    data?.openstackCreds,
    data?.openstackCredsList,
    destinationCluster,
    destinationClusterToOpenstackCredName,
    openstackCredNameToProjectName,
    destinationClusterToProjectNameFromHostConfig
  ])

  const rawNetworkMappings = useMemo(
    () =>
      normalizeMappingRows(
        ((((data?.networkMapping?.spec as any)?.networks as any[]) || []) as any) || []
      ),
    [data?.networkMapping]
  )

  const rawStorageMappings = useMemo(
    () =>
      normalizeMappingRows(
        ((((data?.storageMapping?.spec as any)?.storages as any[]) || []) as any) || []
      ),
    [data?.storageMapping]
  )

  const rawArrayCredsMappings = useMemo(
    () =>
      normalizeMappingRows(
        ((((data?.arrayCredsMapping?.spec as any)?.mappings as any[]) || []) as any) || []
      ),
    [data?.arrayCredsMapping]
  )

  const vmSourceNetworks = useMemo(() => {
    const directNetworks = (vmSpec?.networks as string[]) || []
    const ifaceNetworks = Array.isArray(vmSpec?.networkInterfaces)
      ? (vmSpec.networkInterfaces as any[])
          .map((n) => (n?.network as string) || '')
          .map((s) => s.trim())
          .filter(Boolean)
      : []
    return Array.from(
      new Set([...directNetworks, ...ifaceNetworks].map((s) => String(s).trim()).filter(Boolean))
    )
  }, [vmSpec?.networks, vmSpec?.networkInterfaces])

  const vmSourceDatastores = useMemo(() => {
    const ds = (vmSpec?.datastores as string[]) || []
    return Array.from(new Set(ds.map((s) => String(s).trim()).filter(Boolean)))
  }, [vmSpec?.datastores])

  const networkMappings = useMemo(() => {
    if (!vmSourceNetworks.length) return rawNetworkMappings
    return rawNetworkMappings.filter((row) => vmSourceNetworks.includes(row.source))
  }, [rawNetworkMappings, vmSourceNetworks])

  const storageMappings = useMemo(() => {
    if (!vmSourceDatastores.length) return rawStorageMappings
    return rawStorageMappings.filter((row) => vmSourceDatastores.includes(row.source))
  }, [rawStorageMappings, vmSourceDatastores])

  const arrayCredsMappings = useMemo(() => {
    if (!vmSourceDatastores.length) return rawArrayCredsMappings
    return rawArrayCredsMappings.filter((row) => vmSourceDatastores.includes(row.source))
  }, [rawArrayCredsMappings, vmSourceDatastores])

  const migrationType = (planStrategy?.type as string) || 'N/A'
  const scheduleDataCopy = planStrategy?.dataCopyStart
    ? formatDateTime(planStrategy?.dataCopyStart)
    : 'N/A'

  const periodicSyncEnabled = planAdvanced?.periodicSyncEnabled === true
  const periodicSyncInterval = (planAdvanced?.periodicSyncInterval as string) || ''

  const cutoverPolicy = useMemo(() => {
    if (!initiateCutoverEnabled) return 'N/A'

    if (planStrategy?.adminInitiatedCutOver === true) {
      const periodicSyncValue = periodicSyncEnabled
        ? periodicSyncInterval
          ? `Enabled (${periodicSyncInterval})`
          : 'Enabled'
        : 'Disabled'
      return `Admin initiated (Periodic sync: ${periodicSyncValue})`
    }

    if (planStrategy?.vmCutoverStart || planStrategy?.vmCutoverEnd) {
      const start = planStrategy?.vmCutoverStart
        ? formatDateTime(planStrategy?.vmCutoverStart)
        : 'N/A'
      const end = planStrategy?.vmCutoverEnd ? formatDateTime(planStrategy?.vmCutoverEnd) : 'N/A'
      return `Time window (${start} - ${end})`
    }

    return 'Immediately after data copy'
  }, [
    initiateCutoverEnabled,
    periodicSyncEnabled,
    periodicSyncInterval,
    planStrategy?.adminInitiatedCutOver,
    planStrategy?.vmCutoverEnd,
    planStrategy?.vmCutoverStart
  ])

  const securityGroupOptions =
    ((data?.openstackCreds as any)?.status?.openstack?.securityGroups as any[]) || []
  const serverGroupOptions =
    ((data?.openstackCreds as any)?.status?.openstack?.serverGroups as any[]) || []

  const securityGroups = useMemo(() => {
    const configured = planSpec?.securityGroups
    if (!Array.isArray(configured) || !configured.length) return 'N/A'

    const names = configured
      .map((value) => {
        const match = securityGroupOptions.find((opt) => opt?.id === value || opt?.name === value)
        return (match?.name as string) || String(value)
      })
      .filter(Boolean)

    return names.length ? names.join(', ') : 'N/A'
  }, [planSpec?.securityGroups, securityGroupOptions])

  const serverGroup = useMemo(() => {
    const configured = (planSpec?.serverGroup as string) || ''
    if (!configured) return 'N/A'
    const match = serverGroupOptions.find(
      (opt) => opt?.id === configured || opt?.name === configured
    )
    return (match?.name as string) || configured
  }, [planSpec?.serverGroup, serverGroupOptions])

  const renameVmEnabled = planPostAction?.renameVm === true
  const renameSuffix = renameVmEnabled ? (planPostAction?.suffix as string) || 'N/A' : 'N/A'

  const moveToFolderEnabled = planPostAction?.moveToFolder === true
  const folderName = moveToFolderEnabled ? (planPostAction?.folderName as string) || 'N/A' : 'N/A'

  const disconnectSourceNetwork = enabledOrNA(planStrategy?.disconnectSourceNetwork)
  const fallbackToDhcp = enabledOrNA(planSpec?.fallbackToDHCP)
  const networkPersistence = enabledOrNA(planAdvanced?.networkPersistence)
  const removeVMwareTools = enabledOrNA(planAdvanced?.removeVMwareTools)

  const selectedImageProfileNames = useMemo(() => {
    const names = planAdvanced?.imageProfiles
    if (!Array.isArray(names)) return [] as string[]
    return names.map((n) => String(n).trim()).filter(Boolean)
  }, [planAdvanced?.imageProfiles])

  const profileNamespace = migration?.metadata?.namespace
  const { data: allProfiles } = useQuery({
    queryKey: ['volume-image-profiles', profileNamespace],
    queryFn: () => getVolumeImageProfilesList(profileNamespace),
    enabled: open && selectedImageProfileNames.length > 0,
    staleTime: 60_000,
    refetchOnWindowFocus: false
  })

  const imageProfilesForVM = useMemo(() => {
    if (!selectedImageProfileNames.length || !Array.isArray(allProfiles)) return []
    const vmOs = String(guestOS || '').trim()
    const byName = new Map(allProfiles.map((p) => [String(p?.metadata?.name || ''), p] as const))
    return selectedImageProfileNames
      .map((name) => byName.get(name))
      .filter(Boolean)
      .filter((p) => {
        const os = String(p?.spec?.osFamily || '').trim()
        if (!os || os === 'any') return true
        if (!vmOs || vmOs === 'N/A') return true
        return os === vmOs
      }) as NonNullable<ReturnType<typeof byName.get>>[]
  }, [allProfiles, selectedImageProfileNames, guestOS])

  const postMigrationScript = useMemo(() => {
    const raw = (planSpec?.firstBootScript as string) || ''
    const trimmed = raw.trim()
    if (!trimmed) return ''
    if (trimmed === 'echo "Add your startup script here!"') return ''
    if (trimmed === 'echo "Add your startup script here!"\n') return ''
    return raw
  }, [planSpec?.firstBootScript])

  const handleTabChange = useCallback((_: SyntheticEvent, newValue: number) => {
    setTab(newValue)
  }, [])

  const rdmDisksSummary = useMemo(() => {
    if (data?.rdmDisks?.length) return `${data.rdmDisks.length} disk(s)`
    if (Array.isArray(vmSpec?.rdmDisks) && vmSpec.rdmDisks.length)
      return `${vmSpec.rdmDisks.length} disk(s)`
    return 'N/A'
  }, [data?.rdmDisks, vmSpec?.rdmDisks])

  const generalInfoItems = useMemo(
    () => [
      { label: 'VM Name', value: displayVmName || 'N/A' },
      { label: 'Migration Type', value: migrationType },
      { label: 'Assigned IP(s)', value: assignedIps },
      { label: 'Preserved IP(s)', value: preservedIps },
      { label: 'Created At', value: createdAt },
      { label: 'Guest OS', value: guestOS },
      { label: 'CPU', value: cpu },
      { label: 'Memory', value: memory },
      { label: 'Total Disks', value: diskCount },
      { label: 'Network Adapters', value: networkAdapterCount },
      { label: 'vJailbreak Agent', value: (migrationStatus?.agentName as string) || 'N/A' },
      { label: 'RDM Disks', value: rdmDisksSummary }
    ],
    [
      assignedIps,
      cpu,
      createdAt,
      diskCount,
      guestOS,
      memory,
      migrationStatus?.agentName,
      migrationType,
      networkAdapterCount,
      preservedIps,
      rdmDisksSummary,
      vmName
    ]
  )

  const handleOnlyShowOverridesChange = useCallback((checked: boolean) => {
    setOnlyShowOverrides(checked)
  }, [])

  const migrationPolicyValues = useMemo(
    () => ({
      securityGroups,
      serverGroup,
      scheduleDataCopy,
      cutoverPolicy,
      renameSuffix,
      folderName,
      disconnectSourceNetwork,
      fallbackToDhcp,
      networkPersistence,
      removeVMwareTools,
      useGPUFlavor: enabledOrNA(useGPUFlavor),
      useFlavorless: enabledOrNA(useFlavorless)
    }),
    [
      cutoverPolicy,
      disconnectSourceNetwork,
      fallbackToDhcp,
      folderName,
      networkPersistence,
      removeVMwareTools,
      renameSuffix,
      scheduleDataCopy,
      securityGroups,
      serverGroup,
      useFlavorless,
      useGPUFlavor
    ]
  )

  const migrationPolicyItems = useMemo(
    () =>
      MIGRATION_POLICY_FIELDS.map((field) => ({
        label: field.label,
        value: (migrationPolicyValues as any)[field.key] as string
      })),
    [migrationPolicyValues]
  )

  const migrationEnvironmentValues = useMemo(
    () => ({
      sourceDatacenter,
      sourceCluster,
      esxiHost,
      destinationTenant,
      destinationCluster
    }),
    [destinationCluster, destinationTenant, esxiHost, sourceCluster, sourceDatacenter]
  )

  const migrationEnvironmentItems = useMemo(
    () =>
      MIGRATION_ENVIRONMENT_FIELDS.map((field) => ({
        label: field.label,
        value: (migrationEnvironmentValues as any)[field.key] as string
      })),
    [migrationEnvironmentValues]
  )

  const visiblePolicyItems = useMemo(() => {
    if (!onlyShowOverrides) return migrationPolicyItems
    return migrationPolicyItems.filter((item) => !isDefaultishValue(item.value))
  }, [migrationPolicyItems, onlyShowOverrides])

  return (
    <DrawerShell
      open={open}
      onClose={onClose}
      requireCloseConfirmation={false}
      width={860}
      header={
        <Box>
          <DrawerHeader
            title="Migration Details"
            subtitle={displayVmName || migration?.metadata?.name || ''}
            onClose={onClose}
            actions={<StatusChip label={phaseLabel} size="small" variant="filled" />}
          />
          <NavTabs
            value={tab}
            onChange={handleTabChange}
            sx={{ px: 2, borderBottom: (theme) => `1px solid ${theme.palette.divider}` }}
          >
            <NavTab label="General" value={0} />
            <NavTab label="Advanced" value={1} />
          </NavTabs>
        </Box>
      }
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={onClose}>
            Close
          </ActionButton>
        </DrawerFooter>
      }
    >
      <Box sx={{ display: 'grid', gap: 2 }} data-testid="migration-detail">
        {!migration ? (
          <Alert severity="info">No migration selected.</Alert>
        ) : isLoading ? (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={24} sx={{ mr: 2 }} />
            <Typography variant="body2" color="text.secondary">
              Loading migration details…
            </Typography>
          </Box>
        ) : error ? (
          <Alert severity="error">Failed to load migration details.</Alert>
        ) : (
          <Section>
            {tab === 0 ? (
              <Box sx={{ display: 'grid', gap: 2 }}>
                <SurfaceCard
                  variant="card"
                  title="Migration Environment"
                  subtitle="Source and destination overview"
                >
                  {data?.vmwareCredsCount === 0 || data?.openstackCredsCount === 0 ? (
                    <Alert severity="warning" sx={{ mb: 2 }}>
                      {data?.vmwareCredsCount === 0 && data?.openstackCredsCount === 0
                        ? 'No VMware or PCD credentials are present. Migration details may be incomplete.'
                        : data?.vmwareCredsCount === 0
                          ? 'No VMware credentials are present. Migration details may be incomplete.'
                          : 'No PCD credentials are present. Migration details may be incomplete.'}
                    </Alert>
                  ) : null}
                  <KeyValueGrid items={migrationEnvironmentItems} />
                </SurfaceCard>

                <SurfaceCard variant="card" title="General Info" subtitle="VM specifications">
                  <KeyValueGrid items={generalInfoItems} />

                  {data?.rdmDisks?.length ? (
                    <Box sx={{ mt: 2 }}>
                      <Divider sx={{ mb: 2 }} />
                      <TableContainer component={Paper} variant="outlined">
                        <Table size="small">
                          <TableHead>
                            <TableRow>
                              <TableCell>Disk</TableCell>
                              <TableCell>Size</TableCell>
                              <TableCell>Phase</TableCell>
                            </TableRow>
                          </TableHead>
                          <TableBody>
                            {data.rdmDisks.map((d) => (
                              <TableRow key={d.metadata.name}>
                                <TableCell>
                                  <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                                    {d.spec.displayName || d.spec.diskName || d.metadata.name}
                                  </Typography>
                                </TableCell>
                                <TableCell>{formatDiskSize(d.spec.diskSize)}</TableCell>
                                <TableCell>{d.status?.phase || 'N/A'}</TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </TableContainer>
                    </Box>
                  ) : null}
                </SurfaceCard>

                <SurfaceCard
                  variant="card"
                  title="Mappings"
                  subtitle="Network and storage mappings"
                >
                  <Box sx={{ display: 'grid', gap: 2.5 }}>
                    <Box sx={{ display: 'grid', gap: 1 }}>
                      <FieldLabel label="Network Mapping" />
                      <MappingTable
                        rows={networkMappings}
                        sourceLabel="Source Network"
                        targetLabel="Target Network"
                        emptyLabel="N/A"
                        sourceIcon={<LanOutlinedIcon fontSize="small" color="action" />}
                        targetIcon={<LanOutlinedIcon fontSize="small" color="action" />}
                      />
                    </Box>

                    <Box sx={{ display: 'grid', gap: 1 }}>
                      <FieldLabel label="Storage Mapping" />
                      {isStorageAcceleratedCopy ? (
                        <MappingTable
                          rows={arrayCredsMappings}
                          sourceLabel="Source Datastore"
                          targetLabel="Array Credentials"
                          emptyLabel="N/A"
                          sourceIcon={<StorageOutlinedIcon fontSize="small" color="action" />}
                          targetIcon={<StorageOutlinedIcon fontSize="small" color="action" />}
                        />
                      ) : (
                        <MappingTable
                          rows={storageMappings}
                          sourceLabel="Source Datastore"
                          targetLabel="Target Volume Type"
                          emptyLabel="N/A"
                          sourceIcon={<StorageOutlinedIcon fontSize="small" color="action" />}
                          targetIcon={<StorageOutlinedIcon fontSize="small" color="action" />}
                        />
                      )}
                    </Box>
                  </Box>
                </SurfaceCard>
              </Box>
            ) : (
              <Box sx={{ display: 'grid', gap: 2 }}>
                <SurfaceCard
                  variant="card"
                  title="Migration Policies"
                  subtitle="Flags and post-migration actions"
                >
                  <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 1 }}>
                    <FormControlLabel
                      control={
                        <Switch
                          size="small"
                          checked={onlyShowOverrides}
                          onChange={(e) => handleOnlyShowOverridesChange(e.target.checked)}
                        />
                      }
                      label={<Typography variant="body2">View only enabled options</Typography>}
                    />
                  </Box>

                  {visiblePolicyItems.length ? (
                    <KeyValueGrid items={visiblePolicyItems} />
                  ) : (
                    <Typography variant="body2">No advanced options configured.</Typography>
                  )}

                  {postMigrationScript || !onlyShowOverrides ? (
                    <Box sx={{ mt: 2, display: 'grid', gap: 1.5 }}>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 2
                        }}
                      >
                        <FieldLabel label="Post-migration script" />
                      </Box>

                      {postMigrationScript ? (
                        <Paper
                          variant="outlined"
                          sx={{
                            p: 2,
                            backgroundColor: (theme) => theme.palette.action.hover
                          }}
                        >
                          <Typography
                            variant="caption"
                            component="pre"
                            sx={{
                              m: 0,
                              whiteSpace: 'pre-wrap'
                            }}
                          >
                            {postMigrationScript}
                          </Typography>
                        </Paper>
                      ) : (
                        <Typography variant="body2">N/A</Typography>
                      )}
                    </Box>
                  ) : null}
                </SurfaceCard>

                {selectedImageProfileNames.length ? (
                  <SurfaceCard
                    variant="card"
                    title="Image Profiles"
                    subtitle="Cinder volume image metadata applied to the boot volume"
                  >
                    {imageProfilesForVM.length ? (
                      <Box sx={{ display: 'grid', gap: 1.5 }}>
                        {imageProfilesForVM.map((profile) => {
                          const name = profile.metadata?.name || ''
                          const osLabel =
                            OS_FAMILY_LABEL[profile.spec?.osFamily as string] ||
                            profile.spec?.osFamily ||
                            'N/A'
                          const description = profile.spec?.description || ''
                          const properties = profile.spec?.properties || {}
                          const propertyItems = Object.entries(properties).map(([k, v]) => ({
                            label: k,
                            value: String(v ?? '')
                          }))
                          return (
                            <SurfaceCard
                              key={name}
                              variant="section"
                              title={name}
                              subtitle={description || undefined}
                              actions={
                                <StatusChip label={osLabel} size="small" variant="outlined" />
                              }
                              sx={{
                                border: (theme) => `1px solid ${theme.palette.divider}`
                              }}
                            >
                              {propertyItems.length ? (
                                <KeyValueGrid items={propertyItems} />
                              ) : (
                                <Typography variant="body2">No properties configured.</Typography>
                              )}
                            </SurfaceCard>
                          )
                        })}
                      </Box>
                    ) : (
                      <Typography variant="body2">
                        No image profiles apply to this VM's OS family.
                      </Typography>
                    )}
                  </SurfaceCard>
                ) : null}
              </Box>
            )}
          </Section>
        )}
      </Box>
    </DrawerShell>
  )
}
