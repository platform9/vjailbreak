import type { Meta, StoryObj } from '@storybook/react'
import { ReactNode } from 'react'
import { FormProvider, useForm } from 'react-hook-form'
import { Box } from '@mui/material'
import BucketScheduleField from './BucketScheduleField'

const FormWrapper = ({
  children,
  defaultValues = {}
}: {
  children: ReactNode
  defaultValues?: Record<string, unknown>
}) => {
  const form = useForm({ defaultValues })
  return (
    <FormProvider {...form}>
      <Box sx={{ width: 360 }}>{children}</Box>
    </FormProvider>
  )
}

const meta: Meta<typeof BucketScheduleField> = {
  title: 'Features/Inventory/BucketScheduleField',
  component: BucketScheduleField,
  parameters: { layout: 'centered' }
}

export default meta

type Story = StoryObj<typeof BucketScheduleField>

export const Default: Story = {
  render: (args) => (
    <FormWrapper>
      <BucketScheduleField {...args} />
    </FormWrapper>
  )
}

export const Disabled: Story = {
  render: (args) => (
    <FormWrapper defaultValues={{ schedule: '' }}>
      <BucketScheduleField {...args} disabled />
    </FormWrapper>
  )
}
