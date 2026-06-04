import type { Meta, StoryObj } from '@storybook/react'
import { useState } from 'react'
import TriggerPlanDialog, { TriggerScheduleMode } from './TriggerPlanDialog'
import { recommendAgents } from '../utils/agentRecommendation'
import { DEFAULT_AGENT_PARAMS } from '../constants'
import type { MigrationBucket } from '../types'

const makeBucket = (name: string, vms: string[], isDefault = false): MigrationBucket => ({
  apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
  kind: 'MigrationBucket',
  metadata: { name, namespace: 'migration-system' },
  spec: {
    vmwareCredsRef: { name: 'vcenter-prod' },
    vms,
    isDefault,
    config: { securityGroups: [], advancedOptions: {} }
  },
  status: { phase: 'NotMigrated' }
})

const orderedBuckets: MigrationBucket[] = [
  makeBucket('default-bucket', ['vm-1', 'vm-2'], true),
  makeBucket('bucket-web', ['vm-3']),
  makeBucket('bucket-db', ['vm-4', 'vm-5', 'vm-6'])
]

const meta: Meta<typeof TriggerPlanDialog> = {
  title: 'Features/Inventory/TriggerPlanDialog',
  component: TriggerPlanDialog,
  parameters: { layout: 'centered' }
}

export default meta

type Story = StoryObj<typeof TriggerPlanDialog>

function PlanDemo({ totalVms, selectedCount }: { totalVms: number; selectedCount: number }) {
  const recommendation = recommendAgents({ totalVms, ...DEFAULT_AGENT_PARAMS })
  const [agentCount, setAgentCount] = useState(recommendation.value)
  const [scheduleMode, setScheduleMode] = useState<TriggerScheduleMode>('now')
  return (
    <TriggerPlanDialog
      open
      onClose={() => {}}
      selectedCount={selectedCount}
      totalVms={totalVms}
      recommendation={recommendation}
      agentCount={agentCount}
      onAgentCountChange={setAgentCount}
      orderedBuckets={orderedBuckets}
      scheduleMode={scheduleMode}
      onScheduleModeChange={setScheduleMode}
      onConfirm={() => {}}
    />
  )
}

export const Default: Story = {
  render: () => <PlanDemo totalVms={30} selectedCount={3} />
}

export const ExceedsCapacity: Story = {
  render: () => <PlanDemo totalVms={200} selectedCount={8} />
}
