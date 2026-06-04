import type { Meta, StoryObj } from '@storybook/react'
import { useState } from 'react'
import AgentCountStepper from './AgentCountStepper'

const meta: Meta<typeof AgentCountStepper> = {
  title: 'Features/Inventory/AgentCountStepper',
  component: AgentCountStepper,
  parameters: { layout: 'centered' }
}

export default meta

type Story = StoryObj<typeof AgentCountStepper>

function StepperDemo({ initial, max }: { initial: number; max: number }) {
  const [value, setValue] = useState(initial)
  return <AgentCountStepper value={value} onChange={setValue} max={max} />
}

export const Interactive: Story = {
  render: () => <StepperDemo initial={3} max={10} />
}

export const AtMax: Story = {
  render: () => <StepperDemo initial={10} max={10} />
}
