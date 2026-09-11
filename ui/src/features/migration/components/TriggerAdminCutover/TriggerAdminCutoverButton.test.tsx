import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { vi } from 'vitest'
import { TriggerAdminCutoverButton } from './TriggerAdminCutoverButton'
import * as migrationsApi from '../../api/migrations'
import * as amplitudeService from 'src/services/amplitudeService'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'

vi.mock('../../api/migrations', () => ({
  triggerAdminCutover: vi.fn(),
}))

vi.mock('src/services/amplitudeService', () => ({
  trackEvent: vi.fn(),
  setUserContext: vi.fn(),
  resetUserContext: vi.fn(),
}))

const mockTriggerAdminCutover = migrationsApi.triggerAdminCutover as ReturnType<typeof vi.fn>
const mockTrackEvent = amplitudeService.trackEvent as ReturnType<typeof vi.fn>

const defaultProps = {
  migrationName: 'migration-my-vm-abc12',
  namespace: 'migration-system',
}

const openAndConfirm = async () => {
  render(<TriggerAdminCutoverButton {...defaultProps} />)
  fireEvent.click(screen.getByTestId('cutover-trigger-button'))
  fireEvent.click(await screen.findByTestId('cutover-confirm-button'))
}

describe('TriggerAdminCutoverButton', () => {
  beforeEach(() => vi.clearAllMocks())

  it('tracks Cutover Triggered event on success', async () => {
    mockTriggerAdminCutover.mockResolvedValue({ success: true })

    await openAndConfirm()

    await waitFor(() => {
      expect(mockTrackEvent).toHaveBeenCalledWith(
        AMPLITUDE_EVENTS.CUTOVER_TRIGGERED,
        expect.objectContaining({
          migrationName: defaultProps.migrationName,
          namespace: defaultProps.namespace,
        })
      )
    })
  })

  it('tracks Cutover Trigger Failed event when API reports failure', async () => {
    mockTriggerAdminCutover.mockResolvedValue({ success: false, message: 'not allowed' })

    await openAndConfirm()

    await waitFor(() => {
      expect(mockTrackEvent).toHaveBeenCalledWith(
        AMPLITUDE_EVENTS.CUTOVER_TRIGGER_FAILED,
        expect.objectContaining({
          migrationName: defaultProps.migrationName,
          namespace: defaultProps.namespace,
          errorMessage: 'not allowed',
        })
      )
    })
  })

  it('tracks Cutover Trigger Failed event when API throws', async () => {
    mockTriggerAdminCutover.mockRejectedValue(new Error('network error'))

    await openAndConfirm()

    await waitFor(() => {
      expect(mockTrackEvent).toHaveBeenCalledWith(
        AMPLITUDE_EVENTS.CUTOVER_TRIGGER_FAILED,
        expect.objectContaining({
          migrationName: defaultProps.migrationName,
          namespace: defaultProps.namespace,
          errorMessage: 'network error',
        })
      )
    })
  })
})
