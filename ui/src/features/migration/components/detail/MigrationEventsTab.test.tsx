import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Migration } from '../../api/migrations'
import MigrationEventsTab from './MigrationEventsTab'

const condition = (type: string, isoTime: string, message = '', reason = '') => ({
  type,
  status: 'True',
  reason,
  message,
  lastTransitionTime: isoTime,
})

const buildMigration = (creationTimestamp: string, conditions: unknown[]): Migration =>
  ({
    metadata: { creationTimestamp },
    status: { conditions },
  }) as unknown as Migration

describe('MigrationEventsTab', () => {
  it('always shows a Created row from the migration creation timestamp, even with no conditions yet', () => {
    render(<MigrationEventsTab migration={buildMigration('2026-01-01T00:00:00Z', [])} />)

    expect(screen.getByText('Created')).toBeInTheDocument()
  })

  it('shows the real PodRunning condition as "Migration Started"', () => {
    render(
      <MigrationEventsTab
        migration={buildMigration('2026-01-01T00:00:00Z', [
          condition('PodRunning', '2026-01-01T00:50:00Z', 'Migration pod started running'),
        ])}
      />
    )

    expect(screen.getByText('Migration Started')).toBeInTheDocument()
    expect(screen.queryByText('PodRunning')).not.toBeInTheDocument()
  })

  it('shows the real CutoverTriggered condition as "Cutover", and does not also synthesize one', () => {
    render(
      <MigrationEventsTab
        migration={buildMigration('2026-01-01T00:00:00Z', [
          { ...condition('DataCopy', '2026-01-01T00:10:00Z'), reason: 'Copying disk 0' },
          condition('CutoverTriggered', '2026-01-01T05:00:00Z', 'Admin cutover triggered'),
          condition('Migrating', '2026-01-01T05:01:00Z'),
        ])}
      />
    )

    expect(screen.getAllByText('Cutover')).toHaveLength(1)
    expect(screen.getByText('Admin cutover triggered')).toBeInTheDocument()
  })

  it('synthesizes a Cutover entry from the last disk copy when there is no admin cutover trigger, once conversion has started', () => {
    render(
      <MigrationEventsTab
        migration={buildMigration('2026-01-01T00:00:00Z', [
          { ...condition('DataCopy', '2026-01-01T00:10:00Z'), reason: 'Copying disk 0' },
          { ...condition('DataCopy', '2026-01-01T00:13:00Z'), reason: 'Copying disk 1' },
          condition('Migrating', '2026-01-01T00:14:00Z'),
        ])}
      />
    )

    expect(screen.getByText('Cutover')).toBeInTheDocument()
    expect(screen.getByText('Cutover started immediately after data copy')).toBeInTheDocument()
  })

  it('does not show a Cutover entry before conversion has actually started (cutover has not happened yet)', () => {
    render(
      <MigrationEventsTab
        migration={buildMigration('2026-01-01T00:00:00Z', [
          { ...condition('DataCopy', '2026-01-01T00:10:00Z'), reason: 'Copying disk 0' },
        ])}
      />
    )

    expect(screen.queryByText('Cutover')).not.toBeInTheDocument()
  })

  it('still shows real conditions unchanged - Validated, DataCopy, Migrating, Migrated', () => {
    render(
      <MigrationEventsTab
        migration={buildMigration('2026-01-01T00:00:00Z', [
          condition('Validated', '2026-01-01T00:02:00Z'),
          { ...condition('DataCopy', '2026-01-01T00:10:00Z'), reason: 'Copying disk 0' },
          condition('Migrating', '2026-01-01T00:14:00Z'),
          condition('Migrated', '2026-01-01T00:20:00Z'),
        ])}
      />
    )

    expect(screen.getByText('Validated')).toBeInTheDocument()
    expect(screen.getByText('DataCopy')).toBeInTheDocument()
    expect(screen.getByText('Migrating')).toBeInTheDocument()
    expect(screen.getByText('Migrated')).toBeInTheDocument()
  })
})
