import { Condition, Phase } from '../api/migrations'

export const PHASE_STEPS: Record<string, number> = {
  [Phase.Pending]: 1,
  [Phase.Validating]: 2,
  [Phase.AwaitingDataCopyStart]: 3,
  [Phase.CopyingBlocks]: 4,
  [Phase.SnapshottingSourceVM]: 4,
  [Phase.AttachingDisksToProxy]: 4,
  [Phase.IdentifyingBlockDevices]: 4,
  [Phase.CopyingChangedBlocks]: 5,
  [Phase.HotAddTransferInProgress]: 5,
  [Phase.HotAddCleanup]: 5,
  [Phase.ConvertingDisk]: 6,
  [Phase.AwaitingCutOverStartTime]: 7,
  [Phase.AwaitingAdminCutOver]: 8,
  [Phase.Succeeded]: 9,
  [Phase.Failed]: 10,
  [Phase.ValidationFailed]: 11
}

export const IN_PROGRESS_PHASES: Phase[] = [
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
  Phase.AwaitingAdminCutOver
]

export const getProgressText = (
  phase: Phase | undefined,
  conditions: Condition[] | undefined,
  currentDisk?: string,
  totalDisks?: number
): string => {
  if (!phase || phase === Phase.Unknown) {
    return 'Unknown Status'
  }

  const stepNumber = PHASE_STEPS[phase] || 0
  const totalSteps = 9

  const latestCondition = conditions?.sort(
    (a, b) => new Date(b.lastTransitionTime).getTime() - new Date(a.lastTransitionTime).getTime()
  )[0]

  const message = latestCondition?.message || phase

  if (phase === Phase.Failed || phase === Phase.ValidationFailed || phase === Phase.Succeeded) {
    return `${phase} - ${message}`
  }

  let diskInfo = ''
  if (
    currentDisk &&
    totalDisks &&
    (phase === Phase.CopyingBlocks || phase === Phase.CopyingChangedBlocks)
  ) {
    const parsedDisk = parseInt(currentDisk, 10)
    const currentDiskNum = Number.isNaN(parsedDisk) ? 1 : parsedDisk + 1
    diskInfo = ` (disk ${currentDiskNum}/${totalDisks})`
  }

  return `STEP ${stepNumber}/${totalSteps}: ${phase}${diskInfo} - ${message}`
}
