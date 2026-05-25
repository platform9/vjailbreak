import {
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  InputAdornment,
  Switch,
  TextField,
  Tooltip,
  Typography
} from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import WarningIcon from '@mui/icons-material/Warning'
import { styled } from '@mui/material/styles'
import '@cds/core/icon/register.js'
import { ClarityIcons, vmIcon } from '@cds/core/icon'
import { ActionButton } from 'src/components'
import type { CanonicalVM } from '../types'
import { extractFirstIPv4, hasMultipleIPv4 } from '../utils/ipValidation'

ClarityIcons.addIcons(vmIcon)

const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

export interface BulkIPEditDialogProps {
  open: boolean
  selectedVMCount: number
  vms: CanonicalVM[]
  bulkEditIPs: Record<string, Record<number, string>>
  bulkPreserveIp: Record<string, Record<number, boolean>>
  bulkPreserveMac: Record<string, Record<number, boolean>>
  bulkExistingIPs: Record<string, Record<number, string>>
  bulkCurrentIPs?: Record<string, Record<number, string>>
  bulkValidationStatus: Record<string, Record<number, string>>
  bulkValidationMessages: Record<string, Record<number, string>>
  assigningIPs: boolean
  hasBulkIpsToApply: boolean
  hasBulkIpValidationErrors: boolean
  duplicateNames?: Set<string>
  onClose: () => void
  onApply: () => void
  onClearAll: () => void
  onPreserveIpChange: (vmId: string, ifIdx: number, val: boolean) => void
  onPreserveMacChange: (vmId: string, ifIdx: number, val: boolean) => void
  onIpChange: (vmId: string, ifIdx: number, val: string) => void
}

function renderValidationAdornment(status?: string) {
  if (!status || status === 'empty') return null
  if (status === 'validating') {
    return (
      <InputAdornment position="end" sx={{ alignItems: 'center' }}>
        <CircularProgress size={16} />
      </InputAdornment>
    )
  }
  if (status === 'valid') {
    return (
      <InputAdornment position="end" sx={{ alignItems: 'center' }}>
        <CheckCircleIcon color="success" fontSize="small" />
      </InputAdornment>
    )
  }
  if (status === 'invalid') {
    return (
      <InputAdornment position="end" sx={{ alignItems: 'center' }}>
        <ErrorIcon color="error" fontSize="small" />
      </InputAdornment>
    )
  }
  return null
}

export function BulkIPEditDialog({
  open,
  selectedVMCount,
  vms,
  bulkEditIPs,
  bulkPreserveIp,
  bulkPreserveMac,
  bulkExistingIPs,
  bulkCurrentIPs,
  bulkValidationStatus,
  bulkValidationMessages,
  assigningIPs,
  hasBulkIpsToApply,
  hasBulkIpValidationErrors,
  duplicateNames,
  onClose,
  onApply,
  onClearAll,
  onPreserveIpChange,
  onPreserveMacChange,
  onIpChange
}: BulkIPEditDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>
        Edit IP Addresses for {selectedVMCount} {selectedVMCount === 1 ? 'VM' : 'VMs'}
      </DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          <Box sx={{ display: 'flex', justifyContent: { xs: 'flex-start', sm: 'flex-end' } }}>
            <Button size="small" variant="outlined" onClick={onClearAll}>
              Clear All
            </Button>
          </Box>
          <Box
            sx={{
              maxHeight: 420,
              overflowY: 'auto',
              pr: 1,
              display: 'flex',
              flexDirection: 'column',
              gap: 2
            }}
          >
            {Object.entries(bulkEditIPs).map(([vmId, interfaces]) => {
              const vm = vms.find((v) => v.id === vmId)
              if (!vm) return null
              const displayName = duplicateNames?.has(vm.name) ? (vm.vmKey || vm.name) : vm.name
              return (
                <Box
                  key={vmId}
                  sx={{
                    p: 2,
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: 2,
                    bgcolor: 'background.paper',
                    boxShadow: 1,
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 2
                  }}
                >
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <Tooltip title={vm.powerState === 'powered-on' ? 'Running' : 'Stopped'}>
                      <CdsIconWrapper>
                        {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                        {/* @ts-ignore */}
                        <cds-icon
                          shape="vm"
                          size="md"
                          badge={vm.powerState === 'powered-on' ? 'success' : 'danger'}
                        >
                          {/* @ts-ignore */}
                        </cds-icon>
                      </CdsIconWrapper>
                    </Tooltip>
                    <Typography variant="body2" sx={{ fontWeight: 700 }}>
                      {displayName}
                    </Typography>
                  </Box>

                  {Object.entries(interfaces).map(([interfaceIndexStr, ip]) => {
                    const interfaceIndex = parseInt(interfaceIndexStr)
                    const networkInterface = vm.networkInterfaces?.[interfaceIndex]
                    const status = bulkValidationStatus[vmId]?.[interfaceIndex]
                    const message = bulkValidationMessages[vmId]?.[interfaceIndex]
                    const isPoweredOff = vm.powerState !== 'powered-on'
                    const preserveIp =
                      !isPoweredOff && bulkPreserveIp?.[vmId]?.[interfaceIndex] !== false
                    const preserveMac = bulkPreserveMac?.[vmId]?.[interfaceIndex] !== false
                    const discoveredIp = bulkExistingIPs?.[vmId]?.[interfaceIndex] || ''
                    const interfaceIp = Array.isArray(networkInterface?.ipAddress)
                      ? networkInterface.ipAddress
                          .filter((v: string) => v && v.trim() !== '')
                          .join(', ')
                      : ''
                    const currentIp = bulkCurrentIPs?.[vmId]?.[interfaceIndex] || interfaceIp || ''
                    const displayIp = preserveIp ? discoveredIp : currentIp
                    const existingIpForSlot = bulkExistingIPs?.[vmId]?.[interfaceIndex]
                    return (
                      <Box
                        key={interfaceIndex}
                        sx={{
                          display: 'grid',
                          gridTemplateColumns: { xs: '1fr', sm: '240px 150px 1fr' },
                          columnGap: { xs: 1.5, sm: 2 },
                          rowGap: 1,
                          alignItems: 'flex-start'
                        }}
                      >
                        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75 }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Typography
                              variant="body2"
                              color="text.secondary"
                              sx={{ minWidth: 36 }}
                            >
                              IP:
                            </Typography>
                            <Box
                              component="span"
                              sx={{
                                px: 1,
                                py: 0.25,
                                borderRadius: 1,
                                bgcolor: (theme) =>
                                  theme.palette.mode === 'dark'
                                    ? 'rgba(255, 255, 255, 0.08)'
                                    : theme.palette.grey[100],
                                border: '1px solid',
                                borderColor: 'divider',
                                color: 'text.primary',
                                fontFamily: 'monospace'
                              }}
                            >
                              {displayIp.trim() !== ''
                                ? displayIp
                                : !preserveIp &&
                                    !networkInterface &&
                                    interfaceIndex === 0 &&
                                    !hasMultipleIPv4(vm.ip || '')
                                  ? extractFirstIPv4(vm.ip || '')
                                  : '—'}
                            </Box>
                          </Box>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Typography
                              variant="body2"
                              color="text.secondary"
                              sx={{ minWidth: 36 }}
                            >
                              MAC:
                            </Typography>
                            <Box
                              sx={{
                                display: 'flex',
                                alignItems: 'center',
                                gap: 0.75,
                                minWidth: 0
                              }}
                            >
                              <Box
                                component="span"
                                sx={{
                                  px: 1,
                                  py: 0.25,
                                  borderRadius: 1,
                                  bgcolor: (theme) =>
                                    theme.palette.mode === 'dark'
                                      ? 'rgba(255, 255, 255, 0.08)'
                                      : theme.palette.grey[100],
                                  border: '1px solid',
                                  borderColor: 'divider',
                                  color: 'text.primary',
                                  fontFamily: 'monospace'
                                }}
                              >
                                {networkInterface?.mac || '—'}
                              </Box>
                              {!preserveMac ? (
                                <Tooltip
                                  title="A new MAC address will be assigned in the destination"
                                  placement="right"
                                >
                                  <WarningIcon sx={{ fontSize: 16, color: 'warning.main' }} />
                                </Tooltip>
                              ) : null}
                            </Box>
                          </Box>
                        </Box>
                        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, pt: 0.25 }}>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Switch
                              size="small"
                              checked={preserveIp}
                              disabled={isPoweredOff}
                              onChange={(e) =>
                                onPreserveIpChange(vmId, interfaceIndex, e.target.checked)
                              }
                            />
                            <Typography variant="body2" sx={{ fontWeight: 600 }}>
                              Preserve IP
                            </Typography>
                          </Box>
                          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Switch
                              size="small"
                              checked={preserveMac}
                              onChange={(e) =>
                                onPreserveMacChange(vmId, interfaceIndex, e.target.checked)
                              }
                            />
                            <Typography variant="body2" sx={{ fontWeight: 600 }}>
                              Preserve MAC
                            </Typography>
                          </Box>
                        </Box>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <TextField
                            value={ip}
                            onChange={(e) => onIpChange(vmId, interfaceIndex, e.target.value)}
                            placeholder={
                              preserveIp ? 'Enter IP address' : 'Enter new IP (optional)'
                            }
                            size="small"
                            fullWidth
                            disabled={preserveIp && Boolean(existingIpForSlot?.trim())}
                            InputProps={{
                              endAdornment: renderValidationAdornment(status)
                            }}
                            error={status === 'invalid'}
                            helperText={
                              status === 'invalid'
                                ? message || 'Invalid IP'
                                : preserveIp && !existingIpForSlot?.trim()
                                  ? message || ''
                                  : ''
                            }
                          />
                        </Box>
                      </Box>
                    )
                  })}
                </Box>
              )
            })}
          </Box>
        </Box>
      </DialogContent>
      <DialogActions
        sx={{ justifyContent: 'flex-end', alignItems: 'center', gap: 1, px: 3, py: 2 }}
      >
        <ActionButton tone="secondary" onClick={onClose} disabled={assigningIPs}>
          Cancel
        </ActionButton>
        <ActionButton
          tone="primary"
          onClick={onApply}
          disabled={!hasBulkIpsToApply || assigningIPs || hasBulkIpValidationErrors}
          loading={assigningIPs}
        >
          Apply Changes
        </ActionButton>
      </DialogActions>
    </Dialog>
  )
}
