import type { Meta, StoryObj } from '@storybook/react'
import { Alert } from '@mui/material'
import EditBucketDrawer from './EditBucketDrawer'

// EditBucketDrawer now embeds the shared Migration Form (<MigrationConfigForm>), which fetches
// credentials, clusters, networks, and VMs from live APIs. It therefore can't render in isolation
// in Storybook — exercise it in the running app via Inventory → bucket → Edit.
const meta: Meta<typeof EditBucketDrawer> = {
  title: 'Features/Inventory/EditBucketDrawer',
  component: EditBucketDrawer,
  parameters: { layout: 'centered' }
}

export default meta

type Story = StoryObj<typeof EditBucketDrawer>

export const RequiresAppContext: Story = {
  render: () => (
    <Alert severity="info">
      EditBucketDrawer reuses the live Migration Form and needs app data (credentials, clusters,
      VMs). Open it in the running app: Inventory → a bucket → Edit.
    </Alert>
  )
}
