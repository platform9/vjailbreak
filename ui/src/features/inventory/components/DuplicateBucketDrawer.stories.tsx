import type { Meta, StoryObj } from '@storybook/react'
import DuplicateBucketDrawer from './DuplicateBucketDrawer'
import type { BucketIdByVm, InventoryVm, MigrationBucket } from '../types'

const vmOptions: InventoryVm[] = [
  { id: '1', name: 'vm-app-01', powerState: 'powered-off', nicCount: 1, diskCount: 1, clusterName: 'cluster-a', networks: ['VM Network'], datastores: ['ds1'] },
  { id: '2', name: 'vm-app-02', powerState: 'powered-off', nicCount: 1, diskCount: 2, clusterName: 'cluster-a', networks: ['VM Network'], datastores: ['ds1'] },
  { id: '3', name: 'vm-web-01', powerState: 'powered-on', nicCount: 2, diskCount: 3, clusterName: 'cluster-a', networks: ['VM Network', 'DMZ'], datastores: ['ds2'] },
  { id: '4', name: 'vm-db-01', powerState: 'powered-on', nicCount: 3, diskCount: 5, clusterName: 'cluster-a', networks: ['VM Network'], datastores: ['ds3'] }
]

// vm-web-01 already belongs to another bucket → disabled + labelled in the selector.
const bucketIdByVm: BucketIdByVm = { 'vm-web-01': 'bucket-web' }

const sourceBucket: MigrationBucket = {
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBucket',
  metadata: { name: 'default-bucket', namespace: 'migration-system' },
  spec: {
    vmwareCredsRef: { name: 'vcenter-prod' },
    vms: ['vm-app-01', 'vm-app-02'],
    isDefault: true,
    config: { securityGroups: [], advancedOptions: {} }
  },
  status: { phase: 'NotMigrated' }
}

const meta: Meta<typeof DuplicateBucketDrawer> = {
  title: 'Features/Inventory/DuplicateBucketDrawer',
  component: DuplicateBucketDrawer,
  args: {
    open: true,
    sourceBucket,
    vmOptions,
    bucketIdByVm,
    defaultName: 'default-bucket-copy',
    submitting: false,
    onClose: () => {},
    onSubmit: () => {}
  },
  parameters: { layout: 'fullscreen' }
}

export default meta

type Story = StoryObj<typeof DuplicateBucketDrawer>

export const Default: Story = {}

export const Submitting: Story = {
  args: { submitting: true }
}
