import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Condition, Phase } from '../api/migrations'
import MigrationProgress from './MigrationProgress'

const condition = (type: string, message: string, reason = '', status = 'True'): Condition =>
  ({
    type,
    status,
    reason,
    message,
    lastTransitionTime: new Date().toISOString()
  }) as unknown as Condition

describe('MigrationProgress', () => {
  it('renders an in-progress phase with its step count, percent, and a 9-segment track', () => {
    render(
      <MigrationProgress
        phase={Phase.CopyingBlocks}
        conditions={[]}
        currentDisk="1"
        totalDisks={2}
      />
    )

    expect(screen.getByText('Copying Blocks')).toBeInTheDocument()
    expect(screen.getByText('Step 4 of 9')).toBeInTheDocument()
    expect(screen.getByText('50%')).toBeInTheDocument()
    expect(screen.getByText(/Disk 2 of 2/)).toBeInTheDocument()
    expect(screen.getAllByTestId('progress-segment')).toHaveLength(9)
  })

  it('renders "Ready" while awaiting admin cutover', () => {
    render(<MigrationProgress phase={Phase.AwaitingAdminCutOver} conditions={[]} />)

    expect(screen.getByText('Ready')).toBeInTheDocument()
    expect(screen.getByText('Step 8 of 9')).toBeInTheDocument()
  })

  it('renders "Done" and all segments done when succeeded', () => {
    render(
      <MigrationProgress
        phase={Phase.Succeeded}
        conditions={[condition('Migrated', 'Migration completed successfully')]}
      />
    )

    expect(screen.getByText('Done')).toBeInTheDocument()
    const segments = screen.getAllByTestId('progress-segment')
    expect(segments).toHaveLength(9)
    expect(segments.every((s) => s.dataset.status === 'done')).toBe(true)
  })

  it('renders "Error" and the fail point for a failed migration', () => {
    render(
      <MigrationProgress
        phase={Phase.Failed}
        conditions={[condition('Failed', 'ESXi host unreachable from migration agent', 'ESXiUnreachable')]}
      />
    )

    expect(screen.getByText('Error')).toBeInTheDocument()
    expect(screen.getByText('at Step 2 of 9')).toBeInTheDocument()
    expect(screen.getByText(/ESXiUnreachable/)).toBeInTheDocument()
    expect(screen.getByText(/ESXi host unreachable from migration agent/)).toBeInTheDocument()
  })

  it('renders "Queued" and step 0 while pending', () => {
    render(<MigrationProgress phase={Phase.Pending} conditions={[]} />)

    expect(screen.getByText('Queued')).toBeInTheDocument()
    expect(screen.getByText('Step 0 of 9')).toBeInTheDocument()
    expect(screen.getByText(/In queue/)).toBeInTheDocument()
    expect(
      screen.getAllByTestId('progress-segment').every((s) => s.dataset.status === 'pending')
    ).toBe(true)
  })

  it('surfaces a sync warning as a warning-colored status instead of the plain percent', () => {
    render(
      <MigrationProgress
        phase={Phase.CopyingChangedBlocks}
        conditions={[]}
        syncWarningMessage="Replication lagging behind schedule"
      />
    )

    expect(screen.getByText(/Replication lagging behind schedule/)).toBeInTheDocument()
  })
})
