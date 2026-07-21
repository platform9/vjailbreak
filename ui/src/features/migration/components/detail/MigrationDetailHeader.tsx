import { useState } from 'react'
import {
  Box,
  Breadcrumbs,
  Button,
  Chip,
  Link,
  Tooltip,
  Typography,
} from '@mui/material'
import NavigateNextIcon from '@mui/icons-material/NavigateNext'
import RefreshIcon from '@mui/icons-material/Refresh'
import { Migration, Phase } from '../../api/migrations'
import { useMigrationFormActions } from '../../context/MigrationFormContext'
import { getPhaseColorKey, getPhaseLabel } from '../../utils/phaseUtils'
import { TriggerAdminCutoverButton } from '../TriggerAdminCutover/TriggerAdminCutoverButton'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { MigrationDetailResources } from 'src/hooks/api/useMigrationDetailResourcesQuery'
import { VMwareCreds } from 'src/api/vmware-creds/model'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import DeleteMigrationDialog from '../DeleteMigrationDialog'

const PHASE_CHIP_COLOR: Record<
  ReturnType<typeof getPhaseColorKey>,
  'default' | 'primary' | 'error' | 'info' | 'success' | 'warning'
> = {
  info: 'info',
  success: 'success',
  error: 'error',
  warning: 'warning',
  default: 'default',
}

interface MigrationDetailHeaderProps {
  migration: Migration
  onBack: () => void
  onCutoverSuccess?: () => void
  resources?: MigrationDetailResources | null
}

export default function MigrationDetailHeader({
  migration,
  onBack,
  onCutoverSuccess,
  resources,
}: MigrationDetailHeaderProps) {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const { openMigrationForm } = useMigrationFormActions()

  const phase = migration.status?.phase as Phase | undefined
  const phaseLabel = getPhaseLabel(phase)
  const colorKey = getPhaseColorKey(phase)
  const chipColor = PHASE_CHIP_COLOR[colorKey]

  const vmName = (migration.spec?.vmName as string | undefined) || migration.metadata?.name || '—'
  const migrationName = migration.metadata?.name ?? ''
  const namespace =
    (migration.metadata?.namespace as string | undefined) ?? VJAILBREAK_DEFAULT_NAMESPACE

  const vmwareCreds = resources?.vmwareCreds as VMwareCreds | null | undefined
  const sourceLabel =
    vmwareCreds?.spec?.hostName ||
    vmwareCreds?.spec?.datacenter ||
    resources?.vmwareCredsRef ||
    null

  const openstackCreds = resources?.openstackCreds as OpenstackCreds | null | undefined
  const destLabel =
    openstackCreds?.spec?.projectName ||
    resources?.openstackCredsRef ||
    null

  const isFailed = phase === Phase.Failed || phase === Phase.ValidationFailed
  const showRetry = phase === Phase.Failed
  const isRetryDisabled = (migration.status as { retryable?: boolean } | undefined)?.retryable === false
  const isAwaitingCutover =
    phase === Phase.AwaitingAdminCutOver || phase === Phase.AwaitingCutOverStartTime
  const isTerminal = phase === Phase.Succeeded || isFailed

  const planName =
    (migration.metadata?.labels as unknown as Record<string, string> | undefined)?.migrationplan ||
    (migration.spec as { migrationPlan?: string } | undefined)?.migrationPlan ||
    ''

  const handleRetry = () => {
    if (!migrationName) return
    openMigrationForm('standard', { migrationName, namespace, planName, vmName })
  }

  return (
    <Box sx={{ mb: 3 }}>
      {/* Breadcrumbs */}
      <Breadcrumbs
        separator={<NavigateNextIcon fontSize="small" />}
        sx={{ mb: 1.5, fontSize: '0.8rem' }}
      >
        <Link
          component="button"
          underline="hover"
          color="text.secondary"
          variant="body2"
          onClick={onBack}
          sx={{ cursor: 'pointer' }}
        >
          Migrations
        </Link>
        <Typography variant="body2" color="text.primary">
          {vmName}
        </Typography>
      </Breadcrumbs>

      {/* Title row */}
      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          gap: 2,
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 0 }}>
          <Typography variant="h5" fontWeight={700} noWrap sx={{ letterSpacing: '-0.015em' }}>
            {vmName}
          </Typography>
          <Chip label={phaseLabel} color={chipColor} size="small" sx={{ flexShrink: 0 }} />
        </Box>

        {/* Action buttons */}
        <Box sx={{ display: 'flex', gap: 1, flexShrink: 0 }}>
          {isFailed && (
            <>
              {showRetry && (
                <Tooltip
                  title={
                    isRetryDisabled
                      ? 'This migration cannot be retried because the VM has RDM disks. To retry, manually restart the migration.'
                      : 'Retry migration'
                  }
                >
                  <span>
                    <Button
                      variant="contained"
                      size="small"
                      startIcon={<RefreshIcon />}
                      disabled={isRetryDisabled}
                      onClick={handleRetry}
                    >
                      Retry
                    </Button>
                  </span>
                </Tooltip>
              )}
              <Button
                variant="outlined"
                color="error"
                size="small"
                onClick={() => setDeleteDialogOpen(true)}
              >
                Delete migration
              </Button>
            </>
          )}

          {isAwaitingCutover && (
            <>
              <TriggerAdminCutoverButton
                migrationName={migrationName}
                namespace={namespace}
                onSuccess={onCutoverSuccess}
              />
              <Button
                variant="outlined"
                color="error"
                size="small"
                onClick={() => setDeleteDialogOpen(true)}
              >
                Delete migration
              </Button>
            </>
          )}

          {!isFailed && !isAwaitingCutover && !isTerminal && (
            <Button
              variant="outlined"
              color="error"
              size="small"
              onClick={() => setDeleteDialogOpen(true)}
            >
              Delete migration
            </Button>
          )}
        </Box>
      </Box>

      {/* Subtitle */}
      <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
        {sourceLabel || destLabel ? (
          <>
            {'Migrating '}
            <Box component="span" sx={{ color: 'text.primary', fontWeight: 500 }}>
              {vmName}
            </Box>
            {sourceLabel && (
              <>
                {' from '}
                <Box
                  component="span"
                  sx={{ fontFamily: '"Fira Code", monospace', fontSize: '0.85em', color: 'text.primary' }}
                >
                  {sourceLabel}
                </Box>
              </>
            )}
            {destLabel && (
              <>
                {' to '}
                <Box
                  component="span"
                  sx={{ fontFamily: '"Fira Code", monospace', fontSize: '0.85em', color: 'text.primary' }}
                >
                  {destLabel}
                </Box>
              </>
            )}
          </>
        ) : (
          <>
            {'Migration: '}
            <Box component="span" sx={{ fontFamily: '"Fira Code", monospace', fontSize: '0.85em', color: 'text.primary' }}>
              {migration.metadata?.name ?? vmName}
            </Box>
            {migration.spec?.migrationPlan && (
              <>
                {' · Plan: '}
                <Box component="span" sx={{ fontFamily: '"Fira Code", monospace', fontSize: '0.85em', color: 'text.primary' }}>
                  {String(migration.spec.migrationPlan)}
                </Box>
              </>
            )}
          </>
        )}
      </Typography>

      <DeleteMigrationDialog
        open={deleteDialogOpen}
        onClose={() => setDeleteDialogOpen(false)}
        migrations={[migration]}
        onSuccess={onBack}
      />
    </Box>
  )
}
