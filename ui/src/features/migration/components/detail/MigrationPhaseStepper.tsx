import { Box, CircularProgress, Typography } from '@mui/material'
import CheckIcon from '@mui/icons-material/Check'
import CloseIcon from '@mui/icons-material/Close'
import PauseIcon from '@mui/icons-material/Pause'
import { Migration } from '../../api/migrations'
import {
  DESIGN_PHASE_DEFS,
  PhaseState,
  PhaseStatus,
  derivePhaseStates,
} from '../../utils/phaseUtils'

const STATUS_BG_COLOR: Record<PhaseStatus, string> = {
  done: 'success.main',
  active: 'primary.main',
  paused: 'warning.main',
  failed: 'error.main',
  pending: 'grey.300',
}

function StepCircle({ status }: { status: PhaseStatus }) {
  const isPending = status === 'pending'
  return (
    <Box
      sx={{
        width: 40,
        height: 40,
        borderRadius: '50%',
        bgcolor: isPending ? 'transparent' : STATUS_BG_COLOR[status],
        border: isPending ? '2px solid' : 'none',
        borderColor: isPending ? 'grey.300' : undefined,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        flexShrink: 0,
      }}
    >
      {status === 'done'   && <CheckIcon sx={{ color: 'white', fontSize: 18 }} />}
      {status === 'active' && <CircularProgress size={16} sx={{ color: 'white' }} />}
      {status === 'paused' && <PauseIcon sx={{ color: 'white', fontSize: 18 }} />}
      {status === 'failed' && <CloseIcon sx={{ color: 'white', fontSize: 18 }} />}
      {status === 'pending' && (
        <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: 'grey.400' }} />
      )}
    </Box>
  )
}

function connectorBgColor(status: PhaseStatus): string {
  switch (status) {
    case 'done':   return 'success.main'
    case 'failed': return 'error.main'
    case 'active': return 'primary.main'
    case 'paused': return 'warning.main'
    default:       return 'grey.300'
  }
}

function metaText(state: PhaseState): string {
  switch (state.status) {
    case 'done':    return state.elapsed ?? '—'
    case 'active': {
      const parts = [
        state.elapsed ? `${state.elapsed} elapsed` : null,
        state.eta ?? null,
      ].filter(Boolean)
      return parts.join(' · ') || '—'
    }
    case 'paused': return state.elapsed ? `${state.elapsed} elapsed` : 'Awaiting admin'
    case 'pending': return state.eta ? `est. ${state.eta}` : 'Pending'
    case 'failed':  return `Halted · ${state.elapsed ?? '—'}`
  }
}

interface MigrationPhaseStepperProps {
  migration: Migration
  cutoverTriggered?: boolean
}

export default function MigrationPhaseStepper({ migration, cutoverTriggered }: MigrationPhaseStepperProps) {
  const phaseStates = derivePhaseStates(migration, {
    minDesignIndex: cutoverTriggered ? 3 : undefined,
    cutoverTriggered,
  })

  return (
    <Box
      sx={{
        p: 3,
        bgcolor: 'background.paper',
        borderRadius: 2,
        border: '1px solid',
        borderColor: 'divider',
        mb: 2,
      }}
    >
      <Box sx={{ display: 'flex' }}>
        {DESIGN_PHASE_DEFS.map((phaseDef, idx) => {
          const state = phaseStates[idx]
          const isLast = idx === DESIGN_PHASE_DEFS.length - 1
          return (
            <Box
              key={phaseDef.key}
              sx={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}
            >
              {/* Rail: circle + connector to next step */}
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <StepCircle status={state.status} />
                {!isLast && (
                  <Box
                    sx={{
                      flex: 1,
                      height: 2,
                      bgcolor: connectorBgColor(state.status),
                      ml: 0.5,
                    }}
                  />
                )}
              </Box>
              {/* Body text */}
              <Box sx={{ mt: 1, pr: isLast ? 0 : 2 }}>
                <Typography
                  variant="caption"
                  color={
                    state.status === 'active' ? 'primary.main' :
                    state.status === 'paused' ? 'warning.main' :
                    state.status === 'failed' ? 'error.main' :
                    'text.disabled'
                  }
                  sx={{
                    display: 'block',
                    fontWeight: state.status === 'active' || state.status === 'paused' || state.status === 'failed' ? 700 : 400,
                    letterSpacing: 0.8,
                    textTransform: 'uppercase',
                    fontSize: '0.65rem',
                  }}
                >
                  {phaseDef.stepLabel}
                </Typography>
                <Typography
                  variant="body2"
                  fontWeight={state.status === 'active' || state.status === 'paused' || state.status === 'failed' ? 700 : 500}
                  color={
                    state.status === 'active' ? 'primary.main' :
                    state.status === 'paused' ? 'warning.main' :
                    state.status === 'failed' ? 'error.main' :
                    state.status === 'done'   ? 'text.primary' :
                    'text.disabled'
                  }
                  sx={{ display: 'block' }}
                >
                  {phaseDef.label}
                </Typography>
                <Typography
                  variant="caption"
                  color={state.status === 'failed' ? 'error.main' : 'text.secondary'}
                  sx={{ display: 'block' }}
                >
                  {metaText(state)}
                </Typography>
                <Typography
                  variant="caption"
                  color="text.secondary"
                  sx={{ display: 'block', mt: 0.25, opacity: 0.75 }}
                >
                  {state.detail}
                </Typography>
              </Box>
            </Box>
          )
        })}
      </Box>
    </Box>
  )
}
