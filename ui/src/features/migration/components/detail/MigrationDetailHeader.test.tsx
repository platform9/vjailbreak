import type { ReactElement } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Migration, Phase } from '../../api/migrations'
import MigrationDetailHeader from './MigrationDetailHeader'

const baseMigration = {
  metadata: { name: 'migration-2k19-vj-test', namespace: 'migration-system' },
  spec: { vmName: '2k19-vj-test' },
  status: { phase: Phase.Succeeded },
} as unknown as Migration

// DeleteMigrationDialog (rendered unconditionally, closed) needs a QueryClient in scope.
function renderHeader(ui: ReactElement) {
  const queryClient = new QueryClient()
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

describe('MigrationDetailHeader — source label', () => {
  it('shows the real datacenter from the VMwareMachine annotation, not the VMwareCreds name', () => {
    renderHeader(
      <MigrationDetailHeader
        migration={baseMigration}
        onBack={vi.fn()}
        resources={{
          vmwareMachine: {
            metadata: { annotations: { 'vjailbreak.k8s.pf9.io/datacenter': 'Datacenter1' } },
          },
          vmwareCredsRef: 'vmware',
        } as any}
      />
    )

    expect(screen.getByText('Datacenter1')).toBeInTheDocument()
    expect(screen.queryByText('vmware')).not.toBeInTheDocument()
  })

  it('falls back to the migration template\'s source datacenter when the VM annotation is missing', () => {
    renderHeader(
      <MigrationDetailHeader
        migration={baseMigration}
        onBack={vi.fn()}
        resources={{
          vmwareMachine: { metadata: {} },
          migrationTemplate: { spec: { source: { datacenter: 'Datacenter2' } } },
          vmwareCredsRef: 'vmware',
        } as any}
      />
    )

    expect(screen.getByText('Datacenter2')).toBeInTheDocument()
  })

  it('does not fall back to the VMwareCreds name when no datacenter is available', () => {
    renderHeader(
      <MigrationDetailHeader
        migration={baseMigration}
        onBack={vi.fn()}
        resources={{ vmwareCredsRef: 'vmware' } as any}
      />
    )

    expect(screen.queryByText('vmware')).not.toBeInTheDocument()
    expect(screen.queryByText(/from/)).not.toBeInTheDocument()
  })
})
