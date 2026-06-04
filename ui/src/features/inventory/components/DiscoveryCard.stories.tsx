import type { Meta, StoryObj } from '@storybook/react'
import DiscoveryCard from './DiscoveryCard'

const meta: Meta<typeof DiscoveryCard> = {
  title: 'Features/Inventory/DiscoveryCard',
  component: DiscoveryCard,
  args: {
    vmCount: 42,
    credName: 'vcenter-prod'
  },
  parameters: { layout: 'padded' }
}

export default meta

type Story = StoryObj<typeof DiscoveryCard>

export const Default: Story = {}

export const SingleVm: Story = {
  args: { vmCount: 1, credName: 'vcenter-prod' }
}

export const NoCredential: Story = {
  args: { vmCount: 0, credName: undefined }
}
