import type { Meta, StoryObj } from '@storybook/react'
import TriggerDrawer from './TriggerDrawer'
import type { BucketStatus, MigrationBucket } from '../types'

const makeBucket = (name: string, phase: BucketStatus, isDefault = false): MigrationBucket => ({
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBucket',
  metadata: { name, namespace: 'migration-system' },
  spec: {
    vmwareCredsRef: { name: 'vcenter-prod' },
    vms: ['vm-1', 'vm-2'],
    isDefault,
    config: { securityGroups: [], advancedOptions: {} }
  },
  status: { phase }
})

const buckets: MigrationBucket[] = [
  makeBucket('default-bucket', 'NotMigrated', true),
  makeBucket('bucket-web', 'NotMigrated'),
  makeBucket('bucket-db', 'InProgress'), // disabled (not selectable)
  makeBucket('bucket-cache', 'Migrated') // disabled (not selectable)
]

const meta: Meta<typeof TriggerDrawer> = {
  title: 'Features/Inventory/TriggerDrawer',
  component: TriggerDrawer,
  args: {
    open: true,
    buckets,
    onClose: () => {},
    onContinue: () => {}
  },
  parameters: { layout: 'fullscreen' }
}

export default meta

type Story = StoryObj<typeof TriggerDrawer>

export const Default: Story = {}
