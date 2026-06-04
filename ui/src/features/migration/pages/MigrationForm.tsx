import MigrationIcon from '@mui/icons-material/SwapHoriz'
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell } from 'src/components'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { useMigrationFormSubmit } from '../hooks/useMigrationFormSubmit'
import MigrationConfigForm, {
  defaultMigrationOptions,
  MigrationConfigState
} from '../components/MigrationConfigForm'
import type { MigrationFormDrawerProps } from '../types'

const drawerWidth = 1400
const noop = () => {}

/**
 * "Start Migration" drawer. Thin wrapper around the shared <MigrationConfigForm>: it owns the
 * drawer chrome and the submit action (create the Migration objects). The form body + wiring
 * live in MigrationConfigForm so the bucket editor can reuse them verbatim.
 */
export default function MigrationFormDrawer({ open, onClose, onSuccess }: MigrationFormDrawerProps) {
  const navigate = useNavigate()
  const { reportError } = useErrorHandler({ component: 'MigrationForm' })
  const { track } = useAmplitude({ component: 'MigrationForm' })
  const queryClient = useQueryClient()
  const [sessionId] = useState(() => `form-session-${Date.now()}`)
  const [cfg, setCfg] = useState<MigrationConfigState | null>(null)

  const { submitting, handleSubmit, handleClose } = useMigrationFormSubmit({
    params: cfg?.params ?? {},
    selectedMigrationOptions: cfg?.selectedMigrationOptions ?? defaultMigrationOptions,
    migrationTemplate: cfg?.migrationTemplate,
    vmwareCredentials: cfg?.vmwareCredentials,
    openstackCredentials: cfg?.openstackCredentials,
    setMigrationTemplate: cfg?.setMigrationTemplate ?? noop,
    setVmwareCredentials: cfg?.setVmwareCredentials ?? noop,
    setOpenstackCredentials: cfg?.setOpenstackCredentials ?? noop,
    getFieldErrorsUpdater: cfg?.getFieldErrorsUpdater ?? (() => noop),
    reportError,
    track,
    queryClient,
    navigate,
    onClose,
    onSuccess,
    sessionId,
    networkMappingRequired: cfg?.networkMappingRequired ?? false
  })

  const submitDisabled = !cfg || cfg.disableSubmit || submitting

  return (
    <MigrationConfigForm
      open={open}
      sessionId={sessionId}
      onStateChange={setCfg}
      onSubmit={handleSubmit}
      onClose={handleClose}
      submitDisabled={submitDisabled}
    >
      {(content) => (
        <DrawerShell
          data-testid="migration-form-drawer"
          open={open}
          onClose={handleClose}
          width={drawerWidth}
          ModalProps={{ keepMounted: false, style: { zIndex: 1300 } }}
          header={
            <DrawerHeader
              data-testid="migration-form-header"
              closeButtonTestId="migration-form-close"
              title="Start Migration"
              subtitle="Configure source/destination, select VMs, and map resources before starting"
              icon={<MigrationIcon />}
              onClose={handleClose}
            />
          }
          footer={
            <DrawerFooter data-testid="migration-form-footer">
              <ActionButton
                tone="secondary"
                onClick={handleClose}
                data-testid="migration-form-cancel"
              >
                Cancel
              </ActionButton>
              <ActionButton
                tone="primary"
                onClick={handleSubmit}
                disabled={submitDisabled}
                loading={submitting}
                data-testid="migration-form-submit"
              >
                Start Migration
              </ActionButton>
            </DrawerFooter>
          }
        >
          {content}
        </DrawerShell>
      )}
    </MigrationConfigForm>
  )
}
