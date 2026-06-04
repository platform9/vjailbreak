import type { Meta, StoryObj } from '@storybook/react'
import { Box } from '@mui/material'
import BucketCard from './BucketCard'
import type { BucketStatus, MigrationBucket } from '../types'

const makeBucket = (
  name: string,
  opts: { isDefault?: boolean; vms?: string[]; phase?: BucketStatus } = {}
): MigrationBucket => ({
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBucket',
  metadata: { name, namespace: 'migration-system' },
  spec: {
    vmwareCredsRef: { name: 'vcenter-prod' },
    vms: opts.vms ?? ['vm-app-01', 'vm-app-02', 'vm-app-03'],
    isDefault: opts.isDefault ?? false,
    config: { securityGroups: [], advancedOptions: {} }
  },
  status: { phase: opts.phase ?? 'NotMigrated' }
})

const meta: Meta<typeof BucketCard> = {
  title: 'Features/Inventory/BucketCard',
  component: BucketCard,
  args: {
    bucket: makeBucket('bucket-web'),
    onEdit: () => {},
    onDuplicate: () => {},
    onDelete: () => {}
  },
  parameters: { layout: 'padded' }
}

export default meta

type Story = StoryObj<typeof BucketCard>

export const NonDefault: Story = {}

export const DefaultBucket: Story = {
  args: {
    bucket: makeBucket('default-bucket', {
      isDefault: true,
      vms: ['vm-poweredoff-01', 'vm-poweredoff-02']
    })
  }
}

export const AllStatuses: Story = {
  render: (args) => (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <BucketCard {...args} bucket={makeBucket('bucket-not-migrated', { phase: 'NotMigrated' })} />
      <BucketCard {...args} bucket={makeBucket('bucket-scheduled', { phase: 'Scheduled' })} />
      <BucketCard {...args} bucket={makeBucket('bucket-in-progress', { phase: 'InProgress' })} />
      <BucketCard {...args} bucket={makeBucket('bucket-migrated', { phase: 'Migrated' })} />
    </Box>
  )
}
