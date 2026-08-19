import type { ReactElement } from 'react'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'

const getAvailableTags = vi.fn()
const cleanupApiCall = vi.fn()
const initiateUpgrade = vi.fn()
const getUpgradeProgress = vi.fn()

vi.mock('src/api/version', () => ({
  getAvailableTags: (...args: unknown[]) => getAvailableTags(...args),
  cleanupApiCall: (...args: unknown[]) => cleanupApiCall(...args),
  initiateUpgrade: (...args: unknown[]) => initiateUpgrade(...args),
  getUpgradeProgress: (...args: unknown[]) => getUpgradeProgress(...args)
}))

import { UpgradeModal } from './UpgradeModal'

function renderModal(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } }
  })
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>)
}

const selectVersion = async (user: ReturnType<typeof userEvent.setup>, version: string) => {
  await user.click(screen.getByText('Select a version...'))
  await user.click(await screen.findByRole('option', { name: version }))
}

const upgradeButton = () => screen.getByTestId('upgrade-button')

beforeEach(() => {
  vi.clearAllMocks()
  sessionStorage.clear()
  getAvailableTags.mockResolvedValue({ updates: [{ version: 'v0.1.4' }, { version: 'v0.1.5' }] })
  cleanupApiCall.mockResolvedValue({ success: true, message: 'ok' })
  initiateUpgrade.mockResolvedValue({ upgradeStarted: true })
  getUpgradeProgress.mockResolvedValue({ status: 'in_progress' })
})

describe('UpgradeModal — cleanup gate', () => {
  it('disables Upgrade before any cleanup has run', async () => {
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    expect(upgradeButton()).toBeDisabled()
  })

  it('keeps Upgrade disabled when a version is selected but cleanup has not run', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.4')

    // Shown twice: in the Select and in the step-2 status chip.
    expect(screen.getAllByText('v0.1.4').length).toBeGreaterThan(0)
    expect(upgradeButton()).toBeDisabled()
  })

  it('enables Upgrade only after cleanup succeeds and a version is selected', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.4')

    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))

    await waitFor(() => expect(cleanupApiCall).toHaveBeenCalledTimes(1))
    await waitFor(() => expect(upgradeButton()).toBeEnabled())
    expect(screen.getByText('Cleanup completed successfully')).toBeInTheDocument()
  })

  it('leaves Upgrade disabled after cleanup when no version is selected', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))

    await waitFor(() => expect(cleanupApiCall).toHaveBeenCalledTimes(1))
    expect(upgradeButton()).toBeDisabled()
  })
})

describe('UpgradeModal — cleanup warning', () => {
  it('warns that cleanup is destructive only in the confirmation dialog', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    expect(screen.queryByText(/cannot be undone/i)).not.toBeInTheDocument()

    await user.click(screen.getByTestId('cleanup-button'))

    expect(await screen.findByText('Clean up before upgrade?')).toBeInTheDocument()
    expect(screen.getByText(/cannot be undone/i)).toBeInTheDocument()

    // An upgrade is only reachable once the pre-upgrade checks pass, which requires no
    // migrations to be in flight — cleanup only ever removes succeeded and failed ones.
    expect(screen.queryByText(/running/i)).not.toBeInTheDocument()
  })

  it('does not call the cleanup API until the warning is confirmed', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await user.click(screen.getByTestId('cleanup-button'))

    expect(await screen.findByText('Clean up before upgrade?')).toBeInTheDocument()
    expect(cleanupApiCall).not.toHaveBeenCalled()
  })

  it('does not run cleanup when the warning is cancelled', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await user.click(screen.getByTestId('cleanup-button'))
    const confirmDialog = await screen.findByRole('dialog', { name: /Clean up before upgrade/ })
    await user.click(screen.getByRole('button', { name: 'Cancel', hidden: false }))

    await waitFor(() => expect(confirmDialog).not.toBeVisible())
    expect(cleanupApiCall).not.toHaveBeenCalled()
    expect(upgradeButton()).toBeDisabled()
  })
})

describe('UpgradeModal — cleanup progress placement', () => {
  it('closes the confirmation dialog immediately and shows progress on the Cleanup button', async () => {
    let resolveCleanup: (value: { success: boolean; message: string }) => void = () => {}
    cleanupApiCall.mockImplementation(
      () => new Promise((resolve) => { resolveCleanup = resolve })
    )
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))

    // Confirmation dialog is gone while cleanup runs; the upgrade modal stays interactive.
    await waitFor(() =>
      expect(screen.queryByText('Clean up before upgrade?')).not.toBeInTheDocument()
    )
    expect(screen.getByText('Upgrade vJailbreak')).toBeInTheDocument()
    expect(screen.getByTestId('cleanup-button')).toBeDisabled()
    expect(screen.getByTestId('cleanup-button-spinner')).toBeInTheDocument()
    expect(screen.getByText('Cleaning up...')).toBeInTheDocument()

    resolveCleanup({ success: true, message: 'ok' })
    await waitFor(() => expect(screen.getByText('Re-run Cleanup')).toBeInTheDocument())
    expect(screen.queryByTestId('cleanup-button-spinner')).not.toBeInTheDocument()
  })
})

describe('UpgradeModal — cleanup persistence', () => {
  it('remembers a completed cleanup across a page reload', async () => {
    const user = userEvent.setup()
    const first = renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))
    await waitFor(() => expect(screen.getByText('Re-run Cleanup')).toBeInTheDocument())

    // Simulate a reload: fresh component tree, same sessionStorage.
    first.unmount()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)
    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())

    expect(screen.getByText('Re-run Cleanup')).toBeInTheDocument()
    expect(screen.getByText('Completed')).toBeInTheDocument()

    await selectVersion(user, 'v0.1.4')
    expect(upgradeButton()).toBeEnabled()
  })

  it('requires cleanup again in a fresh session', async () => {
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    expect(screen.getByText('Cleanup')).toBeInTheDocument()
    expect(screen.getByText('Required')).toBeInTheDocument()
    expect(upgradeButton()).toBeDisabled()
  })
})

describe('UpgradeModal — cleanup failure', () => {
  it('keeps Upgrade disabled when the cleanup API reports failure', async () => {
    cleanupApiCall.mockResolvedValue({ success: false, message: 'cleanup blew up' })
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.4')

    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))

    expect(await screen.findByText('Cleanup failed: cleanup blew up')).toBeInTheDocument()
    expect(upgradeButton()).toBeDisabled()
  })

  it('keeps Upgrade disabled when the cleanup request throws', async () => {
    cleanupApiCall.mockRejectedValue(new Error('network down'))
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.4')

    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))

    expect(await screen.findByText('Cleanup failed: network down')).toBeInTheDocument()
    expect(upgradeButton()).toBeDisabled()
  })
})

describe('UpgradeModal — upgrade', () => {
  it('starts the upgrade for the selected version once cleanup is done', async () => {
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.5')

    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))
    await waitFor(() => expect(upgradeButton()).toBeEnabled())

    await user.click(upgradeButton())

    await waitFor(() => expect(initiateUpgrade).toHaveBeenCalledWith('v0.1.5', true))
  })
})

describe('UpgradeModal — progress and alert placement', () => {
  it('renders upgrade progress below the version card, not inside it, and above the warning', async () => {
    let resolveUpgrade: (value: { upgradeStarted: boolean }) => void = () => {}
    initiateUpgrade.mockImplementation(() => new Promise((resolve) => { resolveUpgrade = resolve }))
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.4')
    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))
    await waitFor(() => expect(upgradeButton()).toBeEnabled())

    await user.click(upgradeButton())
    resolveUpgrade({ upgradeStarted: true })

    const progress = await screen.findByTestId('upgrade-progress')
    const card = screen.getByTestId('version-step-card')

    // Status for the whole dialog, so it must not be nested in the version card.
    expect(card).not.toContainElement(progress)

    // ...and it must come after the card but before the processing warning.
    expect(card.compareDocumentPosition(progress) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    const warning = screen.getByText('Processing. Please do not close or refresh this page.')
    expect(progress.compareDocumentPosition(warning) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('centres the progress row and the processing warning', async () => {
    let resolveUpgrade: (value: { upgradeStarted: boolean }) => void = () => {}
    initiateUpgrade.mockImplementation(() => new Promise((resolve) => { resolveUpgrade = resolve }))
    const user = userEvent.setup()
    renderModal(<UpgradeModal show onClose={vi.fn()} />)

    await waitFor(() => expect(screen.getByText('Select a version...')).toBeInTheDocument())
    await selectVersion(user, 'v0.1.4')
    await user.click(screen.getByTestId('cleanup-button'))
    await user.click(await screen.findByTestId('confirm-cleanup-button'))
    await waitFor(() => expect(upgradeButton()).toBeEnabled())

    await user.click(upgradeButton())
    resolveUpgrade({ upgradeStarted: true })

    const progress = await screen.findByTestId('upgrade-progress')
    expect(progress).toHaveStyle({ justifyContent: 'center' })

    const warning = screen
      .getByText('Processing. Please do not close or refresh this page.')
      .closest('.MuiAlert-root')
    expect(warning).toHaveStyle({ justifyContent: 'center' })
  })
})
