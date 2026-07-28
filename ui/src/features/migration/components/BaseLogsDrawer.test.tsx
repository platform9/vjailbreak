import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import BaseLogsDrawer from './BaseLogsDrawer'

const MATCHING_LINE =
  '2026-07-27T10:36:49Z\tINFO\tmigration-controller\tReconciling Migration object\t' +
  '{"controllerKind": "Migration", "name": "migration-windows-11-vtpm-automation-2466-fb547", ' +
  '"reconcileID": "935246a7-8480-49a2-a6f0-cbc5527d1c87"}'

const OTHER_LINE =
  '2026-07-27T12:01:11Z\tINFO\tmigration-controller\tReconciling VMwareCreds object\t' +
  '{"controllerKind": "VMwareCreds", "name": "vcenter-pune"}'

function renderDrawer(logs: string[]) {
  return render(
    <BaseLogsDrawer
      open
      onClose={vi.fn()}
      title="Controller Logs"
      logs={logs}
      isLoading={false}
      error={null}
      isPaused={false}
      onPausedChange={vi.fn()}
      onReconnect={vi.fn()}
      data-testid="controller-logs-drawer"
    />
  )
}

function typeSearch(value: string) {
  const input = screen.getByTestId('logs-search-input').querySelector('input')
  expect(input).not.toBeNull()
  fireEvent.change(input!, { target: { value } })
}

describe('BaseLogsDrawer search', () => {
  it('shows every line before searching', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    expect(screen.getByText(/windows-11-vtpm-automation/)).toBeDefined()
    expect(screen.getByText(/VMwareCreds object/)).toBeDefined()
    expect(screen.getByText('2 / 2 lines')).toBeDefined()
  })

  it('keeps lines containing the term deep inside a long line and hides the rest', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    typeSearch('windows-11-vtpm-automation')

    expect(screen.getByText(/windows-11-vtpm-automation/)).toBeDefined()
    expect(screen.queryByText(/VMwareCreds object/)).toBeNull()
    expect(screen.getByText('1 / 2 lines')).toBeDefined()
  })

  it('does not match near-miss text', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    typeSearch('windows-10-vtpm-automation')

    expect(screen.getByText('0 / 2 lines')).toBeDefined()
    expect(screen.getByText(/No lines match the current filters/)).toBeDefined()
  })

  it('explains that only the buffered tail is searched when nothing matches', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    typeSearch('zzz-not-present')

    expect(screen.getByText(/Only the most recent 2 lines are loaded/)).toBeDefined()
  })

  it('ANDs space-separated tokens', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    typeSearch('reconciling vcenter-pune')

    expect(screen.getByText(/VMwareCreds object/)).toBeDefined()
    expect(screen.queryByText(/windows-11-vtpm-automation/)).toBeNull()
  })

  it('excludes lines with a negated token', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    typeSearch('reconciling -vmwarecreds')

    expect(screen.getByText(/windows-11-vtpm-automation/)).toBeDefined()
    expect(screen.queryByText(/VMwareCreds object/)).toBeNull()
  })

  it('restores all lines when the search box is cleared', () => {
    renderDrawer([MATCHING_LINE, OTHER_LINE])
    typeSearch('vcenter-pune')
    expect(screen.getByText('1 / 2 lines')).toBeDefined()

    typeSearch('')
    expect(screen.getByText('2 / 2 lines')).toBeDefined()
  })
})
