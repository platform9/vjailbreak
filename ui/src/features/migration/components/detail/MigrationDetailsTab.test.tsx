import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Migration } from '../../api/migrations'
import MigrationDetailsTab from './MigrationDetailsTab'

const mockResources = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))

vi.mock('src/hooks/api/useMigrationDetailResourcesQuery', () => ({
  useMigrationDetailResourcesQuery: () => ({
    data: mockResources.current,
    isLoading: false,
    error: null
  })
}))

const DATA_ONLY_LABEL = 'Data only (no VM creation)'

interface Scenario {
  migrationSpec?: Record<string, unknown>
  planStrategy?: Record<string, unknown>
}

function renderDetails({ migrationSpec = {}, planStrategy = {} }: Scenario) {
  mockResources.current = {
    migrationPlan: { spec: { migrationStrategy: { type: 'cold', ...planStrategy } } },
    migrationTemplate: { spec: {} },
    vmwareCredsCount: 1,
    openstackCredsCount: 1
  }
  const migration = {
    metadata: { name: 'migration-vm-1', namespace: 'migration-system' },
    spec: { vmName: 'vm-1', ...migrationSpec },
    status: {}
  } as unknown as Migration

  render(
    <QueryClientProvider client={new QueryClient()}>
      <MigrationDetailsTab migration={migration} />
    </QueryClientProvider>
  )
}

// Configured rows render the value as an "● On" chip beside the label; rows sitting at
// their default render the flat default label instead. Both are label + value in one
// container, so reading the row's text tells us which list the policy landed in.
const policyRowValue = (label: string) => {
  const row = screen.getByText(label).parentElement
  return (row?.textContent ?? '').replace(label, '').trim()
}

describe('MigrationDetailsTab — data only policy row', () => {
  it('shows Data only as on when the migration ran in data-only mode', () => {
    renderDetails({ migrationSpec: { dataOnly: true } })

    expect(policyRowValue(DATA_ONLY_LABEL)).toBe('● On')
  })

  it('falls back to the plan strategy for migrations created before dataOnly was mirrored onto the Migration spec', () => {
    renderDetails({ planStrategy: { dataOnly: true } })

    expect(policyRowValue(DATA_ONLY_LABEL)).toBe('● On')
  })

  it('prefers the Migration spec over a plan that was edited after the migration was created', () => {
    renderDetails({ migrationSpec: { dataOnly: false }, planStrategy: { dataOnly: true } })

    expect(policyRowValue(DATA_ONLY_LABEL)).toBe('Off')
  })

  it('lists Data only among the defaults for a normal migration', () => {
    renderDetails({})

    expect(policyRowValue(DATA_ONLY_LABEL)).toBe('Off')
  })
})

describe('MigrationDetailsTab — cutover policy for data-only migrations', () => {
  it('reports no cutover for a data-only migration even when the plan has a cutover window', () => {
    renderDetails({
      migrationSpec: { dataOnly: true, initiateCutover: true },
      planStrategy: {
        vmCutoverStart: '2026-08-06T10:00:00Z',
        vmCutoverEnd: '2026-08-06T12:00:00Z'
      }
    })

    expect(screen.getByText('N/A (data only)')).toBeInTheDocument()
    expect(screen.queryByText(/Time window/)).not.toBeInTheDocument()
  })

  it('still describes the cutover window for a normal migration', () => {
    renderDetails({
      migrationSpec: { initiateCutover: true },
      planStrategy: {
        vmCutoverStart: '2026-08-06T10:00:00Z',
        vmCutoverEnd: '2026-08-06T12:00:00Z'
      }
    })

    expect(screen.getByText('● Time window')).toBeInTheDocument()
    expect(screen.queryByText('N/A (data only)')).not.toBeInTheDocument()
  })
})
