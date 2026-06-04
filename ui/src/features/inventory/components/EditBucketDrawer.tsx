import { useMemo, useState } from 'react'
import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell } from 'src/components'
import MigrationConfigForm, {
  MigrationConfigState
} from 'src/features/migration/components/MigrationConfigForm'
import type { FormValues } from 'src/features/migration/types'
import type { VmData } from 'src/features/migration/api/migration-templates/model'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import type { MigrationBucket } from '../types'

export interface EditBucketDrawerProps {
  open: boolean
  onClose: () => void
  bucket?: MigrationBucket
  /** Single OpenStack (PCD) credential name for v1 — seeds the destination. */
  openstackCredName?: string
  submitting?: boolean
  /** Persist the updated bucket (container owns the mutation). */
  onSave: (updated: MigrationBucket) => void
}

/** Build the form seed from a bucket: prefer an exact round-trip, else structured defaults. */
function buildSeed(bucket?: MigrationBucket, openstackCredName?: string): Partial<FormValues> {
  if (!bucket) return {}
  const config = bucket.spec.config
  if (config?.formValues) return config.formValues

  return {
    vmwareCreds: {
      existingCredName: bucket.spec.vmwareCredsRef.name
    } as FormValues['vmwareCreds'],
    ...(openstackCredName
      ? { openstackCreds: { existingCredName: openstackCredName } as FormValues['openstackCreds'] }
      : {}),
    vmwareCluster: config?.sourceCluster,
    pcdCluster: config?.pcdCluster,
    networkMappings: config?.networkMappings,
    storageMappings: config?.storageMappings,
    securityGroups: config?.securityGroups,
    serverGroup: config?.serverGroup,
    vms: bucket.spec.vms.map((name) => ({ id: name, name, vmKey: name }) as VmData)
  }
}

/**
 * Edit drawer (FR-010). Reuses the SAME Migration Form (<MigrationConfigForm>) so editing a
 * bucket looks and behaves identically to starting a migration — the only difference is the
 * primary action: **Save** writes the chosen configuration into the bucket's `spec.config`
 * (mirroring a MigrationPlan) instead of launching a migration.
 */
export default function EditBucketDrawer({
  open,
  onClose,
  bucket,
  openstackCredName,
  submitting = false,
  onSave
}: EditBucketDrawerProps) {
  const [sessionId] = useState(() => `bucket-edit-${Date.now()}`)
  const [cfg, setCfg] = useState<MigrationConfigState | null>(null)

  const seed = useMemo(() => buildSeed(bucket, openstackCredName), [bucket, openstackCredName])
  const seedOptions = bucket?.spec.config?.selectedOptions

  const isDefault = bucket?.spec.isDefault ?? false
  const title = isDefault ? `Edit ${DEFAULT_BUCKET_LABEL}` : `Edit ${bucket?.metadata.name ?? 'bucket'}`

  const handleSave = () => {
    if (!bucket || !cfg) return
    const params = cfg.params
    const vmNames = (params.vms ?? []).map((vm) => vm.vmKey || vm.name)

    const updated: MigrationBucket = {
      ...bucket,
      spec: {
        ...bucket.spec,
        vms: vmNames.length > 0 ? vmNames : bucket.spec.vms,
        schedule: params.dataCopyStartTime || undefined,
        config: {
          sourceCluster: params.vmwareCluster,
          pcdCluster: cfg.targetPCDClusterName ?? params.pcdCluster,
          networkMappings: params.networkMappings,
          storageMappings: params.storageMappings,
          securityGroups: params.securityGroups ?? [],
          serverGroup: params.serverGroup,
          dataCopyMethod: params.dataCopyMethod,
          advancedOptions: {},
          formValues: params,
          selectedOptions: cfg.selectedMigrationOptions
        }
      }
    }
    onSave(updated)
  }

  const submitDisabled = !cfg || cfg.disableSubmit || submitting

  return (
    <MigrationConfigForm
      open={open}
      sessionId={sessionId}
      seed={seed}
      seedOptions={seedOptions}
      initialSelectedVmNames={bucket?.spec.vms}
      autoDefaults
      onStateChange={setCfg}
      onSubmit={handleSave}
      onClose={onClose}
      submitDisabled={submitDisabled}
    >
      {(content) => (
        <DrawerShell
          data-testid="edit-bucket-drawer"
          open={open}
          onClose={onClose}
          width={1400}
          header={<DrawerHeader title={title} subtitle={bucket?.metadata.name} onClose={onClose} />}
          footer={
            <DrawerFooter>
              <ActionButton tone="secondary" onClick={onClose} disabled={submitting}>
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={handleSave}
                disabled={submitDisabled}
                loading={submitting}
              >
                Save
              </ActionButton>
            </DrawerFooter>
          }
        >
          {content}
        </DrawerShell>
      )}
    </MigrationConfigForm>
  )
}
