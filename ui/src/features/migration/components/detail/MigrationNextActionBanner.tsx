import { Alert } from '@mui/material'
import { Migration, Phase } from '../../api/migrations'

interface MigrationNextActionBannerProps {
  migration: Migration
}

export default function MigrationNextActionBanner({ migration }: MigrationNextActionBannerProps) {
  const phase = migration.status?.phase as Phase | undefined

  if (!phase) return null

  switch (phase) {
    case Phase.CopyingBlocks:
    case Phase.CopyingChangedBlocks:
    case Phase.ConvertingDisk:
    case Phase.AwaitingDataCopyStart:
    case Phase.Validating:
      return (
        <Alert severity="info" sx={{ mb: 2 }}>
          Migration is running. No action required.
        </Alert>
      )

    case Phase.Pending:
      return (
        <Alert severity="info" sx={{ mb: 2 }}>
          Migration is queued. Waiting for an available agent.
        </Alert>
      )

    case Phase.AwaitingAdminCutOver:
    case Phase.AwaitingCutOverStartTime:
      return (
        <Alert severity="warning" sx={{ mb: 2 }}>
          <strong>Action required.</strong> Data copy is complete. A migration administrator must
          initiate the final cutover.
        </Alert>
      )

    case Phase.Succeeded:
      return (
        <Alert severity="success" sx={{ mb: 2 }}>
          <strong>Migration succeeded.</strong> The target VM is running in PCD.
        </Alert>
      )

    case Phase.Failed:
    case Phase.ValidationFailed:
      return (
        <Alert severity="error" sx={{ mb: 2 }}>
          <strong>Migration halted.</strong> Review the error details below before retrying. The
          source VM has not been modified.
        </Alert>
      )

    default:
      return null
  }
}
