import { useEffect, useMemo, useRef, useState } from 'react'
import { Box } from '@mui/material'
import { useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { ActionButton, Banner, Section, SectionHeader } from 'src/components'
import { ConfirmationDialog } from 'src/components/dialogs'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'
import { useMigrationsQuery, MIGRATIONS_QUERY_KEY } from 'src/hooks/api/useMigrationsQuery'
import { launchBucketMigration, scaleAgentsForTrigger } from '../utils/launchBucket'
import { useClusterData } from 'src/features/migration/hooks/useClusterData'
import { useInventoryVms } from '../hooks/useInventoryVms'
import { deriveBucketStatus } from '../utils/bucketStatus'
import type { BucketStatus } from '../types'
import { buildBucketConfigDefaults, findNoClusterSourceClusterId } from '../utils/bucketDefaults'
import { selectDefaultBucketVms } from '../utils/defaultBucketSelection'
import { DEFAULT_BUCKET_NAME } from '../constants'
import { recommendAgents } from '../utils/agentRecommendation'
import { orderBucketsBySuccess } from '../utils/bucketOrdering'
import { DEFAULT_AGENT_PARAMS } from '../constants'
import TriggerDrawer from '../components/TriggerDrawer'
import TriggerPlanDialog, { TriggerScheduleMode } from '../components/TriggerPlanDialog'
import {
  useCreateBucket,
  useDeleteBucket,
  useUpdateBucket
} from '../hooks/useMigrationBucketsQuery'
import { buildMigrationBucket } from '../api/migration-buckets/migrationBuckets'
import BucketList from '../components/BucketList'
import DuplicateBucketDrawer, {
  DuplicateBucketValues
} from '../components/DuplicateBucketDrawer'
import EditBucketDrawer from '../components/EditBucketDrawer'
import type { MigrationBucket } from '../types'

/** Generate a unique "<base>-copy" name that doesn't collide with existing buckets. */
const makeCopyName = (base: string, existing: string[]): string => {
  const names = new Set(existing)
  let candidate = `${base}-copy`
  let i = 2
  while (names.has(candidate)) candidate = `${base}-copy-${i++}`
  return candidate
}

/**
 * Inventory Management / Migration Planner page (container).
 *
 * Phase 3 (US1): discovery + bucket list. Phase 4 (US2): duplicate / edit / delete with
 * invariants. Trigger flow + scheduling arrive in Phases 5–7.
 */
export default function InventoryPage() {
  const navigate = useNavigate()
  const data = useInventoryVms()

  const openstackCredsQuery = useOpenstackCredentialsQuery(undefined, { staleTime: 0 })
  const openstackCreds = useMemo(() => {
    const creds = Array.isArray(openstackCredsQuery.data) ? openstackCredsQuery.data : []
    return creds.find((c) => c?.status?.openstackValidationStatus === 'Succeeded') ?? creds[0]
  }, [openstackCredsQuery.data])

  // Cluster lists (VMware source + PCD destination) — used to compute a complete, correct
  // default-bucket config at creation so the editor is deterministic (no edit-time resolving).
  const { sourceData, pcdData } = useClusterData()

  const createBucket = useCreateBucket()
  const updateBucket = useUpdateBucket()
  const deleteBucket = useDeleteBucket()

  const [duplicateSource, setDuplicateSource] = useState<MigrationBucket | null>(null)
  const [editBucket, setEditBucket] = useState<MigrationBucket | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<MigrationBucket | null>(null)
  const [triggerOpen, setTriggerOpen] = useState(false)
  const [planSelection, setPlanSelection] = useState<string[] | null>(null)
  const [agentCount, setAgentCount] = useState(0)
  const [scheduleMode, setScheduleMode] = useState<TriggerScheduleMode>('now')
  const [confirming, setConfirming] = useState(false)
  const [triggerError, setTriggerError] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const showNoCredential = !data.isLoading && !data.credName
  const bucketNames = useMemo(() => data.buckets.map((b) => b.metadata.name), [data.buckets])

  // Live bucket status (T049 / FR-017): derive from real Migration objects of member VMs.
  const migrationsQuery = useMigrationsQuery()
  const phaseByVmName = useMemo(() => {
    const map: Record<string, string> = {}
    const list = Array.isArray(migrationsQuery.data) ? migrationsQuery.data : []
    for (const m of list) {
      const vm = m.spec?.vmName as unknown as string
      const phase = m.status?.phase as unknown as string
      if (vm && phase) map[vm] = phase
    }
    return map
  }, [migrationsQuery.data])
  const statusByBucket = useMemo(() => {
    const map: Record<string, BucketStatus> = {}
    for (const b of data.buckets) map[b.metadata.name] = deriveBucketStatus(b, phaseByVmName)
    return map
  }, [data.buckets, phaseByVmName])

  const selectedBuckets = useMemo(
    () => (planSelection ? data.buckets.filter((b) => planSelection.includes(b.metadata.name)) : []),
    [planSelection, data.buckets]
  )
  const totalSelectedVms = useMemo(
    () => selectedBuckets.reduce((n, b) => n + b.spec.vms.length, 0),
    [selectedBuckets]
  )
  const recommendation = useMemo(
    () => recommendAgents({ totalVms: totalSelectedVms, ...DEFAULT_AGENT_PARAMS }),
    [totalSelectedVms]
  )
  const orderedBuckets = useMemo(
    () => orderBucketsBySuccess(selectedBuckets, data.byName),
    [selectedBuckets, data.byName]
  )
  useEffect(() => {
    if (planSelection) setAgentCount(recommendation.value)
  }, [planSelection, recommendation.value])

  // Auto-create the default bucket once, with a COMPLETE, correct config (FR-005/006).
  // Gated until cluster lists + OpenStack status are loaded so the stored config is right the
  // first time — editing then just loads stored values (deterministic, no edit-time resolving).
  const defaultInitRef = useRef(false)
  useEffect(() => {
    if (defaultInitRef.current) return
    if (data.isLoading || !data.credName) return
    if (data.buckets.length > 0) {
      defaultInitRef.current = true
      return
    }
    if (data.vms.length === 0) return // still loading VMs

    const osNetworks = openstackCreds?.status?.openstack?.networks
    const osVolumeTypes = openstackCreds?.status?.openstack?.volumeTypes
    // Wait until everything needed for a complete config has loaded.
    if (
      sourceData.length === 0 ||
      pcdData.length === 0 ||
      !osNetworks?.length ||
      !osVolumeTypes?.length
    ) {
      return
    }

    const selection = selectDefaultBucketVms(data.vms)
    defaultInitRef.current = true
    if (selection.tier === 'none' || selection.vmNames.length === 0) return // defer (FR-006)

    const selectedVms = data.vms.filter((vm) => selection.vmNames.includes(vm.name))

    // Source cluster = the datacenter's "NO CLUSTER" pseudo-cluster, which surfaces every VM
    // (the bucket may span clusters; NO CLUSTER lets them all be selected/migrated together).
    // Fall back to the first real cluster if no NO-CLUSTER source exists.
    const sourceClusterId =
      findNoClusterSourceClusterId(sourceData) ?? sourceData[0]?.clusters?.[0]?.id

    const firstNetwork = osNetworks[0]?.name
    const firstVolumeType = osVolumeTypes[0]
    const sourceNetworks = Array.from(
      new Set(selectedVms.flatMap((vm) => vm.networks).filter(Boolean))
    )
    const sourceDatastores = Array.from(
      new Set(selectedVms.flatMap((vm) => vm.datastores).filter(Boolean))
    )

    createBucket.mutate(
      buildMigrationBucket(DEFAULT_BUCKET_NAME, {
        vmwareCredsRef: { name: data.credName },
        vms: selection.vmNames,
        isDefault: true,
        config: {
          sourceCluster: sourceClusterId,
          pcdCluster: pcdData[0].id,
          networkMappings: firstNetwork
            ? sourceNetworks.map((source) => ({ source, target: firstNetwork }))
            : [],
          storageMappings: firstVolumeType
            ? sourceDatastores.map((source) => ({ source, target: firstVolumeType }))
            : [],
          securityGroups: [],
          advancedOptions: {}
        }
      })
    )
  }, [
    data.isLoading,
    data.credName,
    data.buckets.length,
    data.vms,
    openstackCreds,
    sourceData,
    pcdData,
    createBucket
  ])

  const handleDuplicateSubmit = async ({ name, vmNames }: DuplicateBucketValues) => {
    const credName = data.credName ?? duplicateSource?.spec.vmwareCredsRef.name ?? 'vmware-creds'
    // Inherit the source bucket's config; if it has no mappings yet, compute auto-defaults.
    const inherited = duplicateSource?.spec.config
    const selectedVms = data.vms.filter((vm) => vmNames.includes(vm.name))
    const config =
      inherited && inherited.networkMappings?.length
        ? inherited
        : buildBucketConfigDefaults(selectedVms, openstackCreds)
    const bucket = buildMigrationBucket(name, {
      vmwareCredsRef: { name: credName },
      vms: vmNames,
      isDefault: false,
      config
    })
    await createBucket.mutateAsync(bucket)
    setDuplicateSource(null)
  }

  const handleEditSave = async (updated: MigrationBucket) => {
    await updateBucket.mutateAsync(updated)
    setEditBucket(null)
  }

  const handleDeleteConfirm = async () => {
    if (!deleteTarget) return
    await deleteBucket.mutateAsync(deleteTarget.metadata.name)
  }

  const handleTriggerContinue = (names: string[]) => {
    setTriggerOpen(false)
    setPlanSelection(names)
  }

  const handlePlanConfirm = async () => {
    // Scale agents toward `agentCount`, then turn each selected bucket — in `orderedBuckets`
    // (success-first) order — into the SAME objects the Migration Form creates (NetworkMapping,
    // StorageMapping, MigrationTemplate, MigrationPlan). `scheduleMode === 'now'` overrides each
    // bucket's stored schedule (FR-022); 'scheduled' honors the per-bucket dataCopyStart.
    setConfirming(true)
    setTriggerError(null)
    const scheduleNow = scheduleMode === 'now'
    try {
      await scaleAgentsForTrigger(agentCount) // helper accounts for the master's own agent.
    } catch (err) {
      // Non-fatal: agents can be scaled later; surface but continue to launch migrations.
      console.error('Agent scale-up failed', err)
    }
    try {
      for (const bucket of orderedBuckets) {
        await launchBucketMigration(bucket, { scheduleNow, pcdData })
      }
      queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY })
      setConfirming(false)
      setPlanSelection(null)
      navigate('/dashboard/migrations')
    } catch (err) {
      setConfirming(false)
      setTriggerError(err instanceof Error ? err.message : String(err))
    }
  }

  const triggerAction =
    !showNoCredential && data.buckets.length > 0 ? (
      <ActionButton tone="primary" onClick={() => setTriggerOpen(true)}>
        Trigger migrations
      </ActionButton>
    ) : undefined

  return (
    <Box sx={{ p: 3, height: '100%', width: '100%', overflow: 'auto' }}>
      <Section>
        <SectionHeader
          title="Inventory"
          subtitle="Discover VMs and organize them into migration buckets."
          actions={triggerAction}
        />

        {showNoCredential ? (
          <Banner
            variant="info"
            title="No VMware credential connected"
            message="Add and validate a VMware credential to discover VMs and start planning migrations."
            actionLabel="Add VMware credential"
            onAction={() => navigate('/dashboard/credentials/vm')}
          />
        ) : (
          <BucketList
            data={data}
            statusByBucket={statusByBucket}
            onEdit={setEditBucket}
            onDuplicate={setDuplicateSource}
            onDelete={setDeleteTarget}
          />
        )}
      </Section>

      <DuplicateBucketDrawer
        open={Boolean(duplicateSource)}
        onClose={() => setDuplicateSource(null)}
        sourceBucket={duplicateSource ?? undefined}
        vmOptions={data.vms}
        bucketIdByVm={data.bucketIdByVm}
        defaultName={duplicateSource ? makeCopyName(duplicateSource.metadata.name, bucketNames) : ''}
        submitting={createBucket.isPending}
        onSubmit={handleDuplicateSubmit}
      />

      <EditBucketDrawer
        open={Boolean(editBucket)}
        onClose={() => setEditBucket(null)}
        bucket={editBucket ?? undefined}
        openstackCredName={openstackCreds?.metadata?.name}
        submitting={updateBucket.isPending}
        onSave={handleEditSave}
      />

      <ConfirmationDialog
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="Delete bucket"
        message={`Delete bucket "${deleteTarget?.metadata.name}"? This does not affect any VMs or migrations.`}
        actionLabel="Delete"
        actionColor="error"
        onConfirm={handleDeleteConfirm}
      />

      <TriggerDrawer
        open={triggerOpen}
        onClose={() => setTriggerOpen(false)}
        buckets={data.buckets}
        onContinue={handleTriggerContinue}
      />

      <TriggerPlanDialog
        open={Boolean(planSelection)}
        onClose={() => {
          if (confirming) return
          setTriggerError(null)
          setPlanSelection(null)
        }}
        selectedCount={selectedBuckets.length}
        totalVms={totalSelectedVms}
        recommendation={recommendation}
        agentCount={agentCount}
        onAgentCountChange={setAgentCount}
        orderedBuckets={orderedBuckets}
        scheduleMode={scheduleMode}
        onScheduleModeChange={setScheduleMode}
        confirming={confirming}
        error={triggerError}
        onConfirm={handlePlanConfirm}
      />
    </Box>
  )
}
