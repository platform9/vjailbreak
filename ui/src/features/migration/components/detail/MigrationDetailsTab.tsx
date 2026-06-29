import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { alpha } from '@mui/material/styles'
import {
  Alert,
  Box,
  CircularProgress,
  Chip,
  Collapse,
  Divider,
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from '@mui/material'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import LanOutlinedIcon from '@mui/icons-material/LanOutlined'
import StorageOutlinedIcon from '@mui/icons-material/StorageOutlined'
import { useMigrationDetailResourcesQuery } from 'src/hooks/api/useMigrationDetailResourcesQuery'
import { getVolumeImageProfilesList } from 'src/api/volume-image-profiles/volumeImageProfiles'
import { OS_FAMILY_LABEL } from 'src/api/volume-image-profiles/model'
import { isDefaultishValue, normalizeMappingRows } from 'src/components/migrations/helpers'
import {
  MIGRATION_ENVIRONMENT_FIELDS,
  MIGRATION_POLICY_FIELDS,
} from 'src/components/migrations/migrationDetailConstants'
import { FieldLabel, KeyValueGrid, SurfaceCard } from 'src/components'
import { formatDateTime, formatDiskSize } from 'src/utils'
import { Migration } from '../../api/migrations'

const enabledOrNA = (value: unknown) => (value === true ? 'Enabled' : 'N/A')

const POLICY_DEFAULT_LABELS: Record<string, string> = {
  securityGroups: 'None',
  serverGroup: 'None',
  scheduleDataCopy: 'Immediate',
  cutoverPolicy: 'Immediate',
  renameSuffix: 'None',
  folderName: 'None',
  disconnectSourceNetwork: 'Off',
  fallbackToDhcp: 'Off',
  networkPersistence: 'Off',
  removeVMwareTools: 'Off',
  useGPUFlavor: 'Off',
  useFlavorless: 'Off',
}

const CHIP_SX = { fontWeight: 600, fontSize: '0.68rem', height: 20, borderRadius: '10px' } as const

function PolicyValueCell({ value }: { value: string }) {
  const timeWindowMatch = value.match(/^Time window \((.+)\)$/)
  if (timeWindowMatch) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Chip
          size="small"
          label="● Time window"
          sx={{
            ...CHIP_SX,
            bgcolor: (theme) => alpha(theme.palette.info.main, 0.12),
            color: 'info.dark',
          }}
        />
        <Typography variant="body2">{timeWindowMatch[1]}</Typography>
      </Box>
    )
  }
  const adminMatch = value.match(/^Admin initiated \((.+)\)$/)
  if (adminMatch) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
        <Chip
          size="small"
          label="● Admin initiated"
          sx={{
            ...CHIP_SX,
            bgcolor: (theme) => alpha(theme.palette.warning.main, 0.12),
            color: 'warning.dark',
          }}
        />
        <Typography variant="body2" color="text.secondary">{adminMatch[1]}</Typography>
      </Box>
    )
  }
  if (value === 'Enabled') {
    return (
      <Box sx={{ display: 'flex' }}>
        <Chip
          size="small"
          label="● On"
          sx={{
            ...CHIP_SX,
            bgcolor: (theme) => alpha(theme.palette.success.main, 0.12),
            color: 'success.dark',
          }}
        />
      </Box>
    )
  }
  return <Typography variant="body2">{value}</Typography>
}

function MappingTable({
  rows,
  sourceLabel,
  targetLabel,
  emptyLabel,
  sourceIcon,
  targetIcon,
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
              <TableCell>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, minWidth: 0 }}>
                  {sourceIcon}
                  <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                    {row.source}
                  </Typography>
                </Box>
              </TableCell>
              <TableCell>
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

interface MigrationDetailsTabProps {
  migration: Migration
}

export default function MigrationDetailsTab({ migration }: MigrationDetailsTabProps) {
  const [showDefaults, setShowDefaults] = useState(false)

  const { data, isLoading, error } = useMigrationDetailResourcesQuery({ open: true, migration })

  const migrationSpec = (migration?.spec as any) || {}
  const migrationStatus = (migration?.status as any) || {}

  const vmName = (migrationSpec?.vmName as string) || ''
  const vmKey =
    ((migration?.metadata as any)?.annotations?.['vjailbreak.k8s.pf9.io/original-vm-name'] as string) ||
    ((migration?.metadata as any)?.labels?.['vjailbreak.k8s.pf9.io/vm-key'] as string) ||
    ''
  const displayVmName = vmKey || vmName

  const createdAt = formatDateTime(migration?.metadata?.creationTimestamp)

  const planSpec = (data?.migrationPlan?.spec as any) || {}
  const planStrategy = (planSpec?.migrationStrategy as any) || {}
  const planAdvanced = (planSpec?.advancedOptions as any) || {}
  const planPostAction = (planSpec?.postMigrationAction as any) || {}

  const templateSpec = (data?.migrationTemplate?.spec as any) || {}
  const useFlavorless = templateSpec?.useFlavorless === true
  const useGPUFlavor = templateSpec?.useGPUFlavor === true
  const storageCopyMethod = (templateSpec?.storageCopyMethod as string) || 'normal'
  const isStorageAcceleratedCopy = storageCopyMethod === 'StorageAcceleratedCopy'

  const vmSpec = (data?.vmwareMachine?.spec as any)?.vms || {}
  const vmMeta = (data?.vmwareMachine?.metadata as any) || {}

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

  const networkDetails = useMemo(() => {
    const ifaces = Array.isArray(vmSpec?.networkInterfaces) ? (vmSpec.networkInterfaces as any[]) : []
    const rawOverrides = migrationSpec?.networkOverrides
    let parsedOverrides: any[] = []
    try {
      parsedOverrides = Array.isArray(rawOverrides)
        ? rawOverrides
        : rawOverrides
          ? JSON.parse(String(rawOverrides))
          : []
    } catch {
      parsedOverrides = []
    }
    const overridesByIndex = new Map<number, any>()
    if (Array.isArray(parsedOverrides)) {
      for (const o of parsedOverrides) {
        const idx = Number(o?.interfaceIndex)
        if (!Number.isNaN(idx)) overridesByIndex.set(idx, o)
      }
    }
    return ifaces.map((nic, index) => {
      const override = overridesByIndex.get(index)
      const preserveIP =
        override?.preserveIP !== undefined ? override.preserveIP !== false : nic?.preserveIP !== false
      const preserveMAC =
        override?.preserveMAC !== undefined ? override.preserveMAC !== false : nic?.preserveMAC !== false
      const ipType = preserveIP ? 'Preserved' : 'User Assigned'
      const macType = preserveMAC ? 'Preserved' : 'Auto Assigned'
      const mac = String(nic?.mac || '').trim()
      const preservedIps = Array.isArray(nic?.ipAddress)
        ? nic.ipAddress.map((ip: any) => String(ip || '').trim()).filter(Boolean)
        : []
      const assignedIps = String(override?.UserAssignedIP || '')
        .split(',')
        .map((s) => s.trim())
        .filter(Boolean)
      const ips = preserveIP ? preservedIps : assignedIps
      return { mac: mac || 'N/A', macType, ipType, ips }
    })
  }, [migrationSpec?.networkOverrides, vmSpec?.networkInterfaces])

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
  const destinationCluster = (templateSpec?.targetPCDClusterName as string) || 'N/A'

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
        if (clusterName && !map.has(clusterName)) map.set(clusterName, projectName)
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
    destinationCluster,
    destinationClusterToOpenstackCredName,
    openstackCredNameToProjectName,
    destinationClusterToProjectNameFromHostConfig,
  ])

  const rawNetworkMappings = useMemo(
    () => normalizeMappingRows(((data?.networkMapping?.spec as any)?.networks as any[]) || []),
    [data?.networkMapping]
  )
  const rawStorageMappings = useMemo(
    () => normalizeMappingRows(((data?.storageMapping?.spec as any)?.storages as any[]) || []),
    [data?.storageMapping]
  )
  const rawArrayCredsMappings = useMemo(
    () => normalizeMappingRows(((data?.arrayCredsMapping?.spec as any)?.mappings as any[]) || []),
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

  const networkMappings = useMemo(
    () =>
      vmSourceNetworks.length
        ? rawNetworkMappings.filter((row) => vmSourceNetworks.includes(row.source))
        : rawNetworkMappings,
    [rawNetworkMappings, vmSourceNetworks]
  )
  const storageMappings = useMemo(
    () =>
      vmSourceDatastores.length
        ? rawStorageMappings.filter((row) => vmSourceDatastores.includes(row.source))
        : rawStorageMappings,
    [rawStorageMappings, vmSourceDatastores]
  )
  const arrayCredsMappings = useMemo(
    () =>
      vmSourceDatastores.length
        ? rawArrayCredsMappings.filter((row) => vmSourceDatastores.includes(row.source))
        : rawArrayCredsMappings,
    [rawArrayCredsMappings, vmSourceDatastores]
  )

  const migrationType = (planStrategy?.type as string) || 'N/A'
  const scheduleDataCopy = planStrategy?.dataCopyStart
    ? formatDateTime(planStrategy?.dataCopyStart)
    : 'N/A'
  const periodicSyncEnabled = planAdvanced?.periodicSyncEnabled === true
  const periodicSyncInterval = (planAdvanced?.periodicSyncInterval as string) || ''
  const initiateCutoverEnabled = migrationSpec?.initiateCutover === true

  const cutoverPolicy = useMemo(() => {
    if (planStrategy?.adminInitiatedCutOver === true) {
      const periodicSyncValue = periodicSyncEnabled
        ? periodicSyncInterval
          ? `Enabled (${periodicSyncInterval})`
          : 'Enabled'
        : 'Disabled'
      return `Admin initiated (Periodic sync: ${periodicSyncValue})`
    }
    if (planStrategy?.vmCutoverStart || planStrategy?.vmCutoverEnd) {
      const start = planStrategy?.vmCutoverStart ? formatDateTime(planStrategy.vmCutoverStart) : 'N/A'
      const end = planStrategy?.vmCutoverEnd ? formatDateTime(planStrategy.vmCutoverEnd) : 'N/A'
      return `Time window (${start} - ${end})`
    }
    if (!initiateCutoverEnabled) return 'N/A'
    return 'Immediately after data copy'
  }, [
    initiateCutoverEnabled,
    periodicSyncEnabled,
    periodicSyncInterval,
    planStrategy?.adminInitiatedCutOver,
    planStrategy?.vmCutoverEnd,
    planStrategy?.vmCutoverStart,
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
    const match = serverGroupOptions.find((opt) => opt?.id === configured || opt?.name === configured)
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
      { label: 'Created At', value: createdAt },
      { label: 'Guest OS', value: guestOS },
      { label: 'CPU', value: cpu },
      { label: 'Memory', value: memory },
      { label: 'Total Disks', value: diskCount },
      { label: 'Network Adapters', value: networkAdapterCount },
      { label: 'vJailbreak Agent', value: (migrationStatus?.agentName as string) || 'N/A' },
      { label: 'RDM Disks', value: rdmDisksSummary },
    ],
    [cpu, createdAt, diskCount, displayVmName, guestOS, memory, migrationStatus?.agentName, migrationType, networkAdapterCount, rdmDisksSummary]
  )

  const migrationEnvironmentValues = useMemo(
    () => ({ sourceDatacenter, sourceCluster, esxiHost, destinationTenant, destinationCluster }),
    [destinationCluster, destinationTenant, esxiHost, sourceCluster, sourceDatacenter]
  )
  const migrationEnvironmentItems = useMemo(
    () =>
      MIGRATION_ENVIRONMENT_FIELDS.map((field) => ({
        label: field.label,
        value: (migrationEnvironmentValues as any)[field.key] as string,
      })),
    [migrationEnvironmentValues]
  )

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
      useFlavorless: enabledOrNA(useFlavorless),
    }),
    [cutoverPolicy, disconnectSourceNetwork, fallbackToDhcp, folderName, networkPersistence, removeVMwareTools, renameSuffix, scheduleDataCopy, securityGroups, serverGroup, useFlavorless, useGPUFlavor]
  )
  const migrationPolicyItems = useMemo(
    () =>
      MIGRATION_POLICY_FIELDS.map((field) => ({
        key: field.key,
        label: field.label,
        value: (migrationPolicyValues as any)[field.key] as string,
      })),
    [migrationPolicyValues]
  )
  const configuredPolicyItems = useMemo(
    () => migrationPolicyItems.filter((item) => !isDefaultishValue(item.value)),
    [migrationPolicyItems]
  )
  const defaultPolicyItems = useMemo(
    () => migrationPolicyItems.filter((item) => isDefaultishValue(item.value)),
    [migrationPolicyItems]
  )

  const selectedImageProfileNames = useMemo(() => {
    const names = planAdvanced?.imageProfiles
    if (!Array.isArray(names)) return [] as string[]
    return names.map((n) => String(n).trim()).filter(Boolean)
  }, [planAdvanced?.imageProfiles])

  const profileNamespace = migration?.metadata?.namespace
  const { data: allProfiles } = useQuery({
    queryKey: ['volume-image-profiles', profileNamespace],
    queryFn: () => getVolumeImageProfilesList(profileNamespace),
    enabled: selectedImageProfileNames.length > 0,
    staleTime: 60_000,
    refetchOnWindowFocus: false,
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

  const configuredCount = configuredPolicyItems.length + (postMigrationScript ? 1 : 0)
  const defaultCount = defaultPolicyItems.length
  const defaultRowCount = Math.ceil(defaultPolicyItems.length / 2)

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'center', py: 6 }}>
        <CircularProgress size={24} sx={{ mr: 2 }} />
        <Typography variant="body2" color="text.secondary">
          Loading migration details…
        </Typography>
      </Box>
    )
  }

  if (error) {
    return <Alert severity="error">Failed to load migration details.</Alert>
  }

  return (
    <Box sx={{ display: 'grid', gap: 2 }}>
      {/* Migration Environment */}
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

      {/* General Info */}
      <SurfaceCard variant="card" title="General Info" subtitle="VM specifications">
        <KeyValueGrid items={generalInfoItems} />

        {networkDetails.length ? (
          <Box sx={{ mt: 2 }}>
            <Divider sx={{ mb: 2 }} />
            <Box sx={{ display: 'grid', gap: 1 }}>
              <FieldLabel label="Network Details" />
              <TableContainer component={Paper} variant="outlined">
                <Table size="small" sx={{ tableLayout: 'fixed' }}>
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ width: '22%' }}>
                        <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5 }}>
                          <Typography component="span" variant="inherit">
                            MAC Handling
                          </Typography>
                          <Tooltip
                            slotProps={{ tooltip: { sx: { whiteSpace: 'pre-line' } } }}
                            title={
                              "Preserved: keeps the VM's original MAC address.\nAuto Assigned: assigns a new MAC address at destination."
                            }
                            placement="top"
                          >
                            <IconButton size="small" sx={{ p: 0.25 }}>
                              <InfoOutlinedIcon fontSize="inherit" />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      </TableCell>
                      <TableCell sx={{ width: '20%' }}>MAC Address</TableCell>
                      <TableCell sx={{ width: '22%' }}>
                        <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5 }}>
                          <Typography component="span" variant="inherit">
                            IP Handling
                          </Typography>
                          <Tooltip
                            slotProps={{ tooltip: { sx: { whiteSpace: 'pre-line' } } }}
                            title={
                              "Preserved: keeps the VM's original IP address.\nUser Assigned: uses the IP address you provided in overrides."
                            }
                            placement="top"
                          >
                            <IconButton size="small" sx={{ p: 0.25 }}>
                              <InfoOutlinedIcon fontSize="inherit" />
                            </IconButton>
                          </Tooltip>
                        </Box>
                      </TableCell>
                      <TableCell sx={{ width: '36%' }}>IP Addresses</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {networkDetails.map((row) => (
                      <TableRow key={`${row.mac}-${row.macType}-${row.ipType}`}>
                        <TableCell>
                          <Typography variant="body2" color="text.secondary">
                            {row.macType}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2" sx={{ wordBreak: 'break-word' }}>
                            {row.mac}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Typography variant="body2" color="text.secondary">
                            {row.ipType}
                          </Typography>
                        </TableCell>
                        <TableCell>
                          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
                            {row.ips.length ? (
                              row.ips.map((ip) => (
                                <Chip
                                  key={`${row.mac}-${ip}`}
                                  label={ip}
                                  size="small"
                                  color="info"
                                  variant="outlined"
                                />
                              ))
                            ) : (
                              <Chip label="N/A" size="small" variant="outlined" />
                            )}
                          </Box>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>
          </Box>
        ) : null}

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

      {/* Mappings */}
      <SurfaceCard variant="card" title="Mappings" subtitle="Network and storage mappings">
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

      {/* Migration Policies */}
      <SurfaceCard
        variant="card"
        title="Migration Policies"
        subtitle="Flags and post-migration actions"
        actions={
          <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 500 }}>
            {configuredCount} configured · {defaultCount} default
          </Typography>
        }
      >
        {/* Configured rows */}
        {configuredPolicyItems.length === 0 && !postMigrationScript ? (
          <Typography variant="body2" color="text.secondary">No policies configured.</Typography>
        ) : (
          <Box>
            {configuredPolicyItems.map((item, idx) => {
              const isLast = idx === configuredPolicyItems.length - 1 && !postMigrationScript
              return (
                <Box
                  key={item.key}
                  sx={{
                    display: 'grid',
                    gridTemplateColumns: '220px 1fr',
                    alignItems: 'center',
                    py: 1.25,
                    borderBottom: isLast ? 'none' : '1px solid',
                    borderColor: 'divider',
                    gap: 2,
                  }}
                >
                  <Typography variant="body2" sx={{ fontWeight: 600 }}>
                    {item.label}
                  </Typography>
                  <PolicyValueCell value={item.value} />
                </Box>
              )
            })}
            {postMigrationScript && (
              <Box
                sx={{
                  display: 'grid',
                  gridTemplateColumns: '220px 1fr',
                  alignItems: 'flex-start',
                  py: 1.25,
                  gap: 2,
                }}
              >
                <Typography variant="body2" sx={{ fontWeight: 600 }}>
                  Post-migration script
                </Typography>
                <Typography
                  variant="body2"
                  sx={{ fontFamily: '"Fira Code", monospace', wordBreak: 'break-all' }}
                >
                  {postMigrationScript.split('\n')[0].trim() || postMigrationScript}
                </Typography>
              </Box>
            )}
          </Box>
        )}

        {/* Defaults accordion */}
        {defaultPolicyItems.length > 0 && (
          <Box sx={{ mt: configuredCount > 0 ? 1.5 : 0 }}>
            <Box
              onClick={() => setShowDefaults((v) => !v)}
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                py: 1.25,
                px: 1.5,
                cursor: 'pointer',
                bgcolor: 'action.hover',
                borderRadius: 1,
                '&:hover': { bgcolor: 'action.selected' },
              }}
            >
              <Typography variant="body2" sx={{ fontWeight: 500 }}>
                {showDefaults
                  ? `Policies using defaults (${defaultCount})`
                  : `Show ${defaultCount} policies using defaults`}
              </Typography>
              {showDefaults ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
            </Box>
            <Collapse in={showDefaults}>
              <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1, overflow: 'hidden', mt: 1 }}>
                {Array.from({ length: defaultRowCount }).map((_, rowIdx) => {
                  const left = defaultPolicyItems[rowIdx * 2]
                  const right = defaultPolicyItems[rowIdx * 2 + 1]
                  const isLastRow = rowIdx === defaultRowCount - 1
                  return (
                    <Box
                      key={left.key}
                      sx={{
                        display: 'grid',
                        gridTemplateColumns: '1fr 1fr',
                        borderBottom: isLastRow ? 'none' : '1px solid',
                        borderColor: 'divider',
                      }}
                    >
                      <Box
                        sx={{
                          display: 'flex',
                          justifyContent: 'space-between',
                          alignItems: 'center',
                          px: 1.5,
                          py: 0.875,
                          borderRight: '1px solid',
                          borderColor: 'divider',
                        }}
                      >
                        <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.78rem' }}>
                          {left.label}
                        </Typography>
                        <Typography variant="body2" color="text.disabled" sx={{ fontSize: '0.78rem', ml: 1 }}>
                          {POLICY_DEFAULT_LABELS[left.key] || 'N/A'}
                        </Typography>
                      </Box>
                      {right ? (
                        <Box
                          sx={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            px: 1.5,
                            py: 0.875,
                          }}
                        >
                          <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.78rem' }}>
                            {right.label}
                          </Typography>
                          <Typography variant="body2" color="text.disabled" sx={{ fontSize: '0.78rem', ml: 1 }}>
                            {POLICY_DEFAULT_LABELS[right.key] || 'N/A'}
                          </Typography>
                        </Box>
                      ) : (
                        <Box />
                      )}
                    </Box>
                  )
                })}
              </Box>
            </Collapse>
          </Box>
        )}
      </SurfaceCard>

      {/* Image Profiles */}
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
                  OS_FAMILY_LABEL[profile.spec?.osFamily as string] || profile.spec?.osFamily || 'N/A'
                const description = profile.spec?.description || ''
                const properties = profile.spec?.properties || {}
                const propertyItems = Object.entries(properties).map(([k, v]) => ({
                  label: k,
                  value: String(v ?? ''),
                }))
                return (
                  <SurfaceCard
                    key={name}
                    variant="section"
                    title={name}
                    subtitle={description || undefined}
                    actions={<Box component="span" sx={{ fontSize: '0.75rem', color: 'text.secondary' }}>{osLabel}</Box>}
                    sx={{ border: (theme) => `1px solid ${theme.palette.divider}` }}
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
  )
}
