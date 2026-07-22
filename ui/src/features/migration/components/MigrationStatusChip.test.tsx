import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Phase } from '../api/migrations'
import MigrationStatusChip from './MigrationStatusChip'

describe('MigrationStatusChip', () => {
  it('renders a chip for Pending', () => {
    render(<MigrationStatusChip phase={Phase.Pending} />)
    expect(screen.getByText('Pending')).toHaveClass('MuiChip-label')
  })

  it('renders a chip for Succeeded', () => {
    render(<MigrationStatusChip phase={Phase.Succeeded} />)
    expect(screen.getByText('Succeeded')).toHaveClass('MuiChip-label')
  })

  it('renders a chip for Failed', () => {
    render(<MigrationStatusChip phase={Phase.Failed} />)
    expect(screen.getByText('Failed')).toHaveClass('MuiChip-label')
  })

  it('falls back to plain text for Unknown', () => {
    render(<MigrationStatusChip phase={Phase.Unknown} />)
    const label = screen.getByText('Unknown')
    expect(label).not.toHaveClass('MuiChip-label')
  })
})
