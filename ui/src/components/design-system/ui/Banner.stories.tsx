import type { Meta, StoryObj } from '@storybook/react'
import { Box, Stack } from '@mui/material'
import Banner from './Banner'

const meta: Meta<typeof Banner> = {
  title: 'Components/Design System/Banner',
  component: Banner,
  args: {
    variant: 'warning',
    title: 'Warning',
    message: 'ESXi SSH key is not configured. Configure an SSH private key to validate ESXi host connectivity.'
  },
  parameters: {
    layout: 'padded'
  }
}

export default meta

type Story = StoryObj<typeof Banner>

export const Default: Story = {}

export const WithAction: Story = {
  args: {
    actionLabel: 'Configure Now',
    onAction: () => {
      // no-op for story
    }
  }
}

export const Variants: Story = {
  render: () => (
    <Stack spacing={2}>
      <Banner variant="info" title="Info" message="This is an informational banner." />
      <Banner variant="success" title="Success" message="This is a success banner." />
      <Banner variant="warning" title="Warning" message="This is a warning banner." />
      <Banner variant="error" title="Error" message="This is an error banner." />
      <Box>
        <Banner
          variant="warning"
          title="Warning"
          message="This banner includes a call to action."
          actionLabel="Take Action"
          onAction={() => {
            // no-op for story
          }}
        />
      </Box>
    </Stack>
  )
}
