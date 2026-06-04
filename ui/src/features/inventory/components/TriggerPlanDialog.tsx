import {
  Box,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Radio,
  RadioGroup,
  Typography
} from '@mui/material'
import { ActionButton, Banner, InlineHelp } from 'src/components'
import { DEFAULT_BUCKET_LABEL } from '../constants'
import type { AgentRecommendation } from '../utils/agentRecommendation'
import type { MigrationBucket } from '../types'
import AgentCountStepper from './AgentCountStepper'

export type TriggerScheduleMode = 'now' | 'scheduled'

export interface TriggerPlanDialogProps {
  open: boolean
  onClose: () => void
  selectedCount: number
  totalVms: number
  recommendation: AgentRecommendation
  agentCount: number
  onAgentCountChange: (value: number) => void
  /** Buckets in the recommended execution order (success-first). */
  orderedBuckets: MigrationBucket[]
  scheduleMode: TriggerScheduleMode
  onScheduleModeChange: (mode: TriggerScheduleMode) => void
  confirming?: boolean
  /** Launch error to surface in the dialog (null/undefined = none). */
  error?: string | null
  onConfirm: () => void
}

/**
 * Pre-launch plan dialog (FR-019/FR-020). Phase 6: recommended (editable) agent count + the
 * explainable derivation. Phase 7 (T032/T033) adds the recommended bucket order and the
 * Trigger-now vs Schedule controls within this same dialog.
 */
export default function TriggerPlanDialog({
  open,
  onClose,
  selectedCount,
  totalVms,
  recommendation,
  agentCount,
  onAgentCountChange,
  orderedBuckets,
  scheduleMode,
  onScheduleModeChange,
  confirming = false,
  error,
  onConfirm
}: TriggerPlanDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Plan migration</DialogTitle>
      <DialogContent>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
          <Typography variant="body2" color="text.secondary">
            {selectedCount} bucket{selectedCount === 1 ? '' : 's'} · {totalVms} VM
            {totalVms === 1 ? '' : 's'} selected
          </Typography>

          <Box>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Migration agents
            </Typography>
            <AgentCountStepper
              value={agentCount}
              max={recommendation.maxAgents}
              onChange={onAgentCountChange}
              disabled={confirming}
            />
          </Box>

          <InlineHelp tone="default" icon="info">
            {recommendation.derivation}
          </InlineHelp>

          {recommendation.exceedsCapacity ? (
            <Banner
              variant="warning"
              message={`The selected workload needs ${recommendation.rawValue} agents but the maximum is ${recommendation.maxAgents}; migrations will run in waves.`}
            />
          ) : null}

          <Box>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Execution order
            </Typography>
            <Box component="ol" sx={{ m: 0, pl: 3, display: 'flex', flexDirection: 'column', gap: 0.5 }}>
              {orderedBuckets.map((bucket) => (
                <Typography key={bucket.metadata.name} component="li" variant="body2">
                  {bucket.spec.isDefault ? DEFAULT_BUCKET_LABEL : bucket.metadata.name}{' '}
                  <Typography component="span" variant="caption" color="text.secondary">
                    ({bucket.spec.vms.length} VM{bucket.spec.vms.length === 1 ? '' : 's'})
                  </Typography>
                </Typography>
              ))}
            </Box>
          </Box>

          <Box>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              When to run
            </Typography>
            <RadioGroup
              value={scheduleMode}
              onChange={(e) => onScheduleModeChange(e.target.value as TriggerScheduleMode)}
            >
              <FormControlLabel value="now" control={<Radio size="small" />} label="Trigger now" />
              <FormControlLabel
                value="scheduled"
                control={<Radio size="small" />}
                label="Use each bucket's schedule"
              />
            </RadioGroup>
            {scheduleMode === 'now' ? (
              <InlineHelp tone="default" icon="info">
                Triggering now ignores per-bucket schedules for the selected buckets.
              </InlineHelp>
            ) : null}
          </Box>

          {error ? <Banner variant="error" message={error} /> : null}
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <ActionButton tone="secondary" onClick={onClose} disabled={confirming}>
          Cancel
        </ActionButton>
        <ActionButton tone="primary" onClick={onConfirm} loading={confirming}>
          Trigger
        </ActionButton>
      </DialogActions>
    </Dialog>
  )
}
