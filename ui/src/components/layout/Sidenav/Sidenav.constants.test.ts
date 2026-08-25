import { describe, expect, it } from 'vitest'
import { Phase } from 'src/api/migrations/model'
import { canUpgrade } from './Sidenav.constants'

const withPhase = (...phases: (Phase | string | undefined)[]) =>
  phases.map((phase) => (phase === undefined ? {} : { status: { phase } }))

describe('canUpgrade', () => {
  it('allows an upgrade when there are no migrations', () => {
    expect(canUpgrade([])).toBe(true)
  })

  it('allows an upgrade when every migration has finished', () => {
    expect(canUpgrade(withPhase(Phase.Succeeded, Phase.Failed, Phase.ValidationFailed))).toBe(true)
  })

  // The bug this replaces: a migration the controller has not reconciled yet has no status
  // at all, which the old active-phase list did not match, so the button stayed enabled.
  it('blocks an upgrade for a migration with no status yet', () => {
    expect(canUpgrade(withPhase(undefined))).toBe(false)
  })

  it('blocks an upgrade for a migration with an empty phase', () => {
    expect(canUpgrade(withPhase(''))).toBe(false)
  })

  // Every non-terminal phase must block, including the ones the old list had never been
  // updated for: the Hot-Add phases and DataCopied.
  it.each([
    Phase.Pending,
    Phase.Validating,
    Phase.AwaitingDataCopyStart,
    Phase.CopyingBlocks,
    Phase.CopyingChangedBlocks,
    Phase.SnapshottingSourceVM,
    Phase.AttachingDisksToProxy,
    Phase.IdentifyingBlockDevices,
    Phase.HotAddTransferInProgress,
    Phase.HotAddCleanup,
    Phase.ConvertingDisk,
    Phase.AwaitingCutOverStartTime,
    Phase.AwaitingAdminCutOver,
    Phase.WaitingForLDMBootSuccess,
    Phase.PromotingToVirtio,
    Phase.DataCopied,
    Phase.Unknown
  ])('blocks an upgrade while a migration is %s', (phase) => {
    expect(canUpgrade(withPhase(phase))).toBe(false)
  })

  it('blocks an upgrade when one migration among finished ones is still running', () => {
    expect(canUpgrade(withPhase(Phase.Succeeded, Phase.CopyingBlocks, Phase.Failed))).toBe(false)
  })

  // A phase added to the CRD but not to the allowlist must block rather than slip through,
  // which is the failure mode of the previous active-phase list.
  it('blocks an upgrade for a phase it has never heard of', () => {
    expect(canUpgrade(withPhase('SomeFuturePhase'))).toBe(false)
  })

  it('blocks an upgrade while the migration list is still loading', () => {
    expect(canUpgrade(undefined)).toBe(false)
  })
})
