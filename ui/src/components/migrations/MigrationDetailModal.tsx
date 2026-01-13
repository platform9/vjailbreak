import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
  Paper,
  Switch,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  Typography
} from '@mui/material'
import LanOutlinedIcon from '@mui/icons-material/LanOutlined'
import StorageOutlinedIcon from '@mui/icons-material/StorageOutlined'
import { SyntheticEvent, useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  FieldLabel,
  Section,
  SurfaceCard
} from 'src/components'
import type { OpenstackCreds } from 'src/api/openstack-creds/model'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import type { NetworkMapping } from 'src/api/network-mapping/model'
import { getNetworkMapping } from 'src/api/network-mapping/networkMappings'
import type { StorageMapping } from 'src/api/storage-mappings/model'
import { getStorageMapping } from 'src/api/storage-mappings/storageMappings'
import type { MigrationPlan } from 'src/api/migration-plans/model'
import { getMigrationPlan } from 'src/api/migration-plans/migrationPlans'
import type { MigrationTemplate } from 'src/api/migration-templates/model'
import { getMigrationTemplate } from 'src/api/migration-templates/migrationTemplates'
import type { VMwareMachine } from 'src/api/vmware-machines/model'
import { getVMwareMachines } from 'src/api/vmware-machines/vmwareMachines'
import type { RdmDisk } from 'src/api/rdm-disks/model'
import { getRdmDisksList } from 'src/api/rdm-disks/rdmDisks'
import { formatDiskSize } from 'src/utils'
import type { Migration } from 'src/features/migration/api/migrations'

export interface MigrationDetailModalProps {
  open: boolean
  migration: Migration | null
  onClose: () => void
}

interface MigrationDetailResources {
  migrationPlan: MigrationPlan | null
  migrationTemplate: MigrationTemplate | null
  openstackCreds: OpenstackCreds | null
  networkMapping: NetworkMapping | null
  storageMapping: StorageMapping | null
  vmwareMachine: VMwareMachine | null
  rdmDisks: RdmDisk[]
}

const formatDateTime = (value: unknown) => {
  if (!value) return '-'
  const d = value instanceof Date ? value : new Date(String(value))
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

const formatCommaSeparated = (value: unknown) => {
  if (!value) return 'N/A'
  const parts = String(value)
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
  if (!parts.length) return 'N/A'
  return parts.join(', ')
}

const enabledOrNA = (value: unknown) => {
  return value === true ? 'Enabled' : 'N/A'
}

const isDefaultishValue = (value: string) => {
  const v = String(value ?? '').trim().toLowerCase()
  return (
    v === '' ||
    v === 'n/a' ||
    v === '-' ||
    v === '—' ||
    v === 'no' ||
    v === 'false' ||
    v === 'disabled'
  )
}

type KeyValueItem = {
  label: string
  value: string
}

function KeyValueGrid({ items }: { items: KeyValueItem[] }) {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: '220px 1fr' },
        columnGap: 2,
        rowGap: 1.5,
        alignItems: 'start'
      }}
    >
      {items.map((item) => (
        <Box
          key={item.label}
          sx={{
            display: 'contents'
          }}
        >
          <FieldLabel label={item.label} />
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              gap: 1,
              minWidth: 0
            }}
          >
            <Typography variant="body2" sx={{ wordBreak: 'break-word', flex: 1 }}>
              {item.value || '—'}
            </Typography>
          </Box>
        </Box>
      ))}
    </Box>
  )
}

function StatusChip({ phase }: { phase: unknown }) {
  const phaseLabel = (phase as string) || 'Unknown'

  const color = (() => {
    if (phaseLabel === 'Succeeded') return 'success'
    if (phaseLabel === 'Failed' || phaseLabel === 'ValidationFailed') return 'error'
    if (phaseLabel === 'Pending' || phaseLabel === 'Unknown') return 'default'
    return 'info'
  })()

  return <Chip label={phaseLabel} color={color as any} size="small" variant="filled" />
}

function TabPanel({
  value,
  index,
  children
}: {
  value: number
  index: number
  children: React.ReactNode
}) {
  if (value !== index) return null
  return <Box sx={{ pt: 2 }}>{children}</Box>
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

const normalizeMappingRows = (
  entries: Array<{ source?: unknown; target?: unknown }>
): Array<{ source: string; target: string }> => {
  const normalizeTokens = (value: unknown): string[] => {
    if (!value) return []
    return String(value)
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
  }

  const rows: Array<{ source: string; target: string }> = []
  for (const entry of entries || []) {
    const sources = normalizeTokens(entry?.source)
    const targets = normalizeTokens(entry?.target)

    if (sources.length > 1 && targets.length > 1 && sources.length === targets.length) {
      for (let i = 0; i < sources.length; i += 1) {
        rows.push({ source: sources[i], target: targets[i] })
      }
      continue
    }

    const safeSources = sources.length ? sources : ['-']
    const safeTargets = targets.length ? targets : ['-']
    for (const source of safeSources) {
      for (const target of safeTargets) {
        rows.push({ source, target })
      }
    }
  }

  return rows
}

const useMigrationDetailResources = ({
  open,
  migration
}: {
  open: boolean
  migration: Migration | null
}) => {
  return useQuery({
    queryKey: ['migration-detail', migration?.metadata?.namespace, migration?.metadata?.name],
    enabled: open && Boolean(migration?.metadata?.name),
    refetchOnWindowFocus: false,
    staleTime: 0,
    queryFn: async (): Promise<MigrationDetailResources> => {
      if (!migration) {
        return {
          migrationPlan: null,
          migrationTemplate: null,
          openstackCreds: null,
          networkMapping: null,
          storageMapping: null,
          vmwareMachine: null,
          rdmDisks: []
        }
      }

      const namespace = migration?.metadata?.namespace
      const migrationSpec = migration?.spec as any
      const vmName = (migrationSpec?.vmName as string) || ''
      const vmStableId =
        (migrationSpec?.vmId as string) ||
        (migrationSpec?.vmID as string) ||
        (migrationSpec?.vmwareMachine as string) ||
        (migrationSpec?.vmwareMachineName as string) ||
        (migrationSpec?.vmwareMachineRef as string) ||
        ''
      const migrationPlanName =
        (migrationSpec?.migrationPlan as string) || (migration?.metadata as any)?.labels?.migrationplan

      const safeGet = async <T,>(fn: () => Promise<T>): Promise<T | null> => {
        try {
          return await fn()
        } catch {
          return null
        }
      }

      const migrationPlan = migrationPlanName
        ? await safeGet(() => getMigrationPlan(migrationPlanName, namespace))
        : null

      const migrationTemplateName = (migrationPlan?.spec as any)?.migrationTemplate as string | undefined
      const migrationTemplate = migrationTemplateName
        ? await safeGet(() => getMigrationTemplate(migrationTemplateName, namespace))
        : null

      const templateSpec = (migrationTemplate?.spec as any) || {}
      const openstackRef = templateSpec?.destination?.openstackRef as string | undefined
      const networkMappingName = templateSpec?.networkMapping as string | undefined
      const storageMappingName = templateSpec?.storageMapping as string | undefined
      const vmwareRef = templateSpec?.source?.vmwareRef as string | undefined

      const [openstackCreds, networkMapping, storageMapping] = await Promise.all([
        openstackRef ? safeGet(() => getOpenstackCredentials(openstackRef, namespace)) : Promise.resolve(null),
        networkMappingName
          ? safeGet(() => getNetworkMapping(networkMappingName, namespace))
          : Promise.resolve(null),
        storageMappingName
          ? safeGet(() => getStorageMapping(storageMappingName, namespace))
          : Promise.resolve(null)
      ])

      const vmwareMachinesList = await safeGet(() => getVMwareMachines(namespace, vmwareRef))
      const vmwareMachines = vmwareMachinesList?.items || []

      const vmwareMachine =
        vmwareMachines.length
          ? vmwareMachines.find((m) => vmStableId && m?.metadata?.name === vmStableId) ||
            vmwareMachines.find((m) => vmName && (m?.spec as any)?.vms?.name === vmName) ||
            null
          : null

      const rdmDiskNames = ((vmwareMachine?.spec as any)?.vms?.rdmDisks as string[]) || []
      const effectiveVmName = vmName || ((vmwareMachine?.spec as any)?.vms?.name as string) || ''
      const allRdmDisks = effectiveVmName ? await safeGet(() => getRdmDisksList(namespace)) : null
      const rdmDisks = (allRdmDisks || []).filter((d: any) => {
        const ownerVMs = (d?.spec?.ownerVMs as string[]) || []
        const diskName = (d?.spec?.diskName as string) || ''
        const metaName = (d?.metadata?.name as string) || ''
        return (
          (effectiveVmName && ownerVMs.includes(effectiveVmName)) ||
          (rdmDiskNames.length && (rdmDiskNames.includes(metaName) || rdmDiskNames.includes(diskName)))
        )
      })

      return {
        migrationPlan,
        migrationTemplate,
        openstackCreds,
        networkMapping,
        storageMapping,
        vmwareMachine,
        rdmDisks
      }
    }
  })
}

export default function MigrationDetailModal({
  open,
  migration,
  onClose
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
  const phase = migration?.status?.phase

  const { data, isLoading, error } = useMigrationDetailResources({ open, migration })

  const migrationSpec = (migration?.spec as any) || {}
  const migrationStatus = (migration?.status as any) || {}

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
  const assignedIpFromPlan = ((planSpec?.assignedIPsPerVM as Record<string, string> | undefined) || {})[vmName] || ''
  const assignedIps = formatCommaSeparated(assignedIpRaw || assignedIpFromPlan)

  const initiateCutoverEnabled = migrationSpec?.initiateCutover === true

  const templateSpec = (data?.migrationTemplate?.spec as any) || {}
  const useFlavorless = templateSpec?.useFlavorless === true
  const useGPUFlavor = templateSpec?.useGPUFlavor === true

  const vmSpec = (data?.vmwareMachine?.spec as any)?.vms || {}
  const vmMeta = (data?.vmwareMachine?.metadata as any) || {}
  const sourceDatacenter =
    (vmMeta?.annotations?.['vjailbreak.k8s.pf9.io/datacenter'] as string) ||
    (templateSpec?.source?.datacenter as string) ||
    'N/A'
  const sourceCluster =
    (vmSpec?.clusterName as string) || (vmMeta?.labels?.['vjailbreak.k8s.pf9.io/vmware-cluster'] as string) || 'N/A'
  const esxiHost =
    (vmSpec?.esxiName as string) || (vmMeta?.labels?.['vjailbreak.k8s.pf9.io/esxi-name'] as string) || 'N/A'

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
  const destinationTenant = (data?.openstackCreds?.spec as any)?.projectName || 'N/A'

  const rawNetworkMappings = normalizeMappingRows(
    ((((data?.networkMapping?.spec as any)?.networks as any[]) || []) as any) || []
  )
  const rawStorageMappings = normalizeMappingRows(
    ((((data?.storageMapping?.spec as any)?.storages as any[]) || []) as any) || []
  )

  const vmSourceNetworks = useMemo(() => {
    const directNetworks = (vmSpec?.networks as string[]) || []
    const ifaceNetworks = Array.isArray(vmSpec?.networkInterfaces)
      ? (vmSpec.networkInterfaces as any[])
          .map((n) => (n?.network as string) || '')
          .map((s) => s.trim())
          .filter(Boolean)
      : []
    return Array.from(new Set([...directNetworks, ...ifaceNetworks].map((s) => String(s).trim()).filter(Boolean)))
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

  const migrationType = (planStrategy?.type as string) || 'N/A'
  const scheduleDataCopy = planStrategy?.dataCopyStart ? formatDateTime(planStrategy?.dataCopyStart) : 'N/A'

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
      const start = planStrategy?.vmCutoverStart ? formatDateTime(planStrategy?.vmCutoverStart) : 'N/A'
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

  const securityGroupOptions = ((data?.openstackCreds as any)?.status?.openstack?.securityGroups as any[]) || []
  const serverGroupOptions = ((data?.openstackCreds as any)?.status?.openstack?.serverGroups as any[]) || []

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
  const renameSuffix = renameVmEnabled ? ((planPostAction?.suffix as string) || 'N/A') : 'N/A'

  const moveToFolderEnabled = planPostAction?.moveToFolder === true
  const folderName = moveToFolderEnabled ? ((planPostAction?.folderName as string) || 'N/A') : 'N/A'

  const disconnectSourceNetwork = enabledOrNA(planStrategy?.disconnectSourceNetwork)
  const fallbackToDhcp = enabledOrNA(planSpec?.fallbackToDHCP)
  const networkPersistence = enabledOrNA(planAdvanced?.networkPersistence)

  const postMigrationScript = useMemo(() => {
    const raw = (planSpec?.firstBootScript as string) || ''
    const trimmed = raw.trim()
    if (!trimmed) return ''
    if (trimmed === 'echo "Add your startup script here!"') return ''
    if (trimmed === 'echo "Add your startup script here!"\n') return ''
    return raw
  }, [planSpec?.firstBootScript])

  const handleTabChange = (_: SyntheticEvent, newValue: number) => {
    setTab(newValue)
  }

  const migrationPolicyItems: KeyValueItem[] = useMemo(
    () => [
      { label: 'Security Groups', value: securityGroups },
      { label: 'Server Group', value: serverGroup },
      { label: 'Schedule Data Copy', value: scheduleDataCopy },
      { label: 'Cutover Policy', value: cutoverPolicy },
      { label: 'Rename Suffix', value: renameSuffix },
      { label: 'Folder Name', value: folderName },
      { label: 'Disconnect source network', value: disconnectSourceNetwork },
      { label: 'Fallback to DHCP', value: fallbackToDhcp },
      { label: 'Persist source network', value: networkPersistence },
      { label: 'Use GPU-enabled flavours', value: enabledOrNA(useGPUFlavor) },
      { label: 'Use dynamic hotplug-enabled flavors', value: enabledOrNA(useFlavorless) }
    ],
    [
      cutoverPolicy,
      disconnectSourceNetwork,
      fallbackToDhcp,
      folderName,
      networkPersistence,
      renameSuffix,
      scheduleDataCopy,
      securityGroups,
      serverGroup,
      useFlavorless,
      useGPUFlavor
    ]
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
            subtitle={vmName || migration?.metadata?.name || ''}
            onClose={onClose}
            actions={<StatusChip phase={phase} />}
          />
          <Tabs
            value={tab}
            onChange={handleTabChange}
            variant="scrollable"
            scrollButtons="auto"
            sx={{ px: 2, borderBottom: (theme) => `1px solid ${theme.palette.divider}` }}
          >
            <Tab label="General" />
            <Tab label="Advanced" />
          </Tabs>
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
            <TabPanel value={tab} index={0}>
              <SurfaceCard
                variant="card"
                title="Migration Environment"
                subtitle="Source and destination overview"
              >
                <KeyValueGrid
                  items={[
                    { label: 'Datacenter', value: sourceDatacenter },
                    { label: 'Source Cluster', value: sourceCluster },
                    { label: 'ESX Host', value: esxiHost },
                    { label: 'Destination Tenant', value: destinationTenant },
                    { label: 'Destination Cluster', value: destinationCluster }
                  ]}
                />
              </SurfaceCard>

              <SurfaceCard variant="card" title="General Info" subtitle="VM specifications">
                <KeyValueGrid
                  items={[
                    { label: 'VM Name', value: vmName || 'N/A' },
                    { label: 'Migration Type', value: migrationType },
                    { label: 'Assigned IP(s)', value: assignedIps },
                    { label: 'Created At', value: createdAt },
                    { label: 'Guest OS', value: guestOS },
                    { label: 'CPU', value: cpu },
                    { label: 'Memory', value: memory },
                    { label: 'Total Disks', value: diskCount },
                    { label: 'Network Adapters', value: networkAdapterCount },
                    { label: 'vJailbreak Agent', value: (migrationStatus?.agentName as string) || 'N/A' },
                    {
                      label: 'RDM Disks',
                      value: data?.rdmDisks?.length
                        ? `${data.rdmDisks.length} disk(s)`
                        : Array.isArray(vmSpec?.rdmDisks) && vmSpec.rdmDisks.length
                          ? `${vmSpec.rdmDisks.length} disk(s)`
                          : 'N/A'
                    }
                  ]}
                />

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
                    <MappingTable
                      rows={storageMappings}
                      sourceLabel="Source Datastore"
                      targetLabel="Target Volume Type"
                      emptyLabel="N/A"
                      sourceIcon={<StorageOutlinedIcon fontSize="small" color="action" />}
                      targetIcon={<StorageOutlinedIcon fontSize="small" color="action" />}
                    />
                  </Box>
                </Box>
              </SurfaceCard>
            </TabPanel>

            <TabPanel value={tab} index={1}>
              <SurfaceCard variant="card" title="Migration Policies" subtitle="Flags and post-migration actions">
                <Box sx={{ display: 'flex', justifyContent: 'flex-end', mb: 1 }}>
                  <FormControlLabel
                    control={
                      <Switch
                        size="small"
                        checked={onlyShowOverrides}
                        onChange={(e) => setOnlyShowOverrides(e.target.checked)}
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
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 2 }}>
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
                          variant="body2"
                          component="pre"
                          sx={{
                            m: 0,
                            whiteSpace: 'pre-wrap',
                            fontSize: 13,
                            lineHeight: 1.6,
                            fontFamily:
                              'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace'
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
            </TabPanel>
          </Section>
        )}
      </Box>
    </DrawerShell>
  )
}
