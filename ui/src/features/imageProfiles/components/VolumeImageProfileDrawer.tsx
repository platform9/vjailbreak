import {
  Box,
  IconButton,
  Tooltip,
  TextField,
  MenuItem,
  Select,
  FormControl,
  Alert,
  Typography,
  Autocomplete
} from '@mui/material'
import { FieldLabel } from 'src/components'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlined'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import ProfileIcon from '@mui/icons-material/Tune'
import { useState, useEffect } from 'react'
import { useQueryClient, useMutation } from '@tanstack/react-query'
import { ConfirmationDialog } from 'src/components/dialogs'
import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell } from 'src/components/design-system'
import {
  createVolumeImageProfile,
  updateVolumeImageProfile
} from 'src/api/volume-image-profiles/volumeImageProfiles'
import {
  VolumeImageProfile,
  VolumeImageProfileSpec,
  KNOWN_IMAGE_PROPERTIES,
  DEFAULT_PROFILE_NAMES
} from 'src/api/volume-image-profiles/model'
import { VOLUME_IMAGE_PROFILES_QUERY_KEY } from 'src/hooks/api/useVolumeImageProfilesQuery'

interface KeyValueRow {
  // Stable id used as the React key so row add/delete/reorder doesn't
  // cause MUI text fields to re-associate with the wrong row (cursor jumps,
  // stale error text, value "shifting" after a delete).
  id: string
  key: string
  value: string
}

// Tiny id generator — crypto.randomUUID is widely available, but fall back
// to a timestamp+random for older browsers without requiring a dependency.
function newRowId(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

function makeEmptyRow(): KeyValueRow {
  return { id: newRowId(), key: '', value: '' }
}

interface VolumeImageProfileDrawerProps {
  open: boolean
  onClose: () => void
  editProfile?: VolumeImageProfile | null
}

const OS_FAMILY_OPTIONS: { label: string; value: VolumeImageProfileSpec['osFamily'] }[] = [
  { label: 'Windows', value: 'windowsGuest' },
  { label: 'Linux', value: 'linuxGuest' },
  { label: 'Any (applies to all VMs)', value: 'any' }
]

const knownKeys = KNOWN_IMAGE_PROPERTIES.map((p) => p.key)

function propertiesToRows(props: Record<string, string>): KeyValueRow[] {
  return Object.entries(props).map(([key, value]) => ({ id: newRowId(), key, value }))
}

function rowsToProperties(rows: KeyValueRow[]): Record<string, string> {
  const result: Record<string, string> = {}
  rows.forEach(({ key, value }) => {
    const k = key.trim()
    const v = value.trim()
    // Skip half-filled rows entirely — validate() already surfaces an error
    // before this runs, but belt-and-braces in case of a future caller.
    if (k && v) result[k] = v
  })
  return result
}

// Ensures the rows list always ends with exactly one empty row — the bottom
// input slot where users type their next entry. When they fill the empty row,
// a fresh empty row is appended below so typing can continue without any
// "add" button.
function ensureTrailingEmpty(rows: KeyValueRow[]): KeyValueRow[] {
  if (rows.length === 0) return [makeEmptyRow()]
  const last = rows[rows.length - 1]
  if (last.key || last.value) return [...rows, makeEmptyRow()]
  return rows
}

export default function VolumeImageProfileDrawer({
  open,
  onClose,
  editProfile
}: VolumeImageProfileDrawerProps) {
  const isEdit = Boolean(editProfile)
  const isDefaultProfile =
    editProfile && DEFAULT_PROFILE_NAMES.includes(editProfile.metadata.name)

  const [name, setName] = useState('')
  const [osFamily, setOsFamily] = useState<VolumeImageProfileSpec['osFamily']>('any')
  const [description, setDescription] = useState('')
  const [rows, setRows] = useState<KeyValueRow[]>(() => [makeEmptyRow()])
  const [nameError, setNameError] = useState('')
  const [rowErrors, setRowErrors] = useState<string[]>([])
  const [rowValueErrors, setRowValueErrors] = useState<string[]>([])
  const [submitError, setSubmitError] = useState('')
  const [showDiscardDialog, setShowDiscardDialog] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [hasUserInteracted, setHasUserInteracted] = useState(false)

  useEffect(() => {
    if (open) {
      setName(editProfile?.metadata.name ?? '')
      setOsFamily(editProfile?.spec.osFamily ?? 'any')
      setDescription(editProfile?.spec.description ?? '')
      setRows(
        ensureTrailingEmpty(
          editProfile?.spec.properties ? propertiesToRows(editProfile.spec.properties) : []
        )
      )
      setNameError('')
      setRowErrors([])
      setRowValueErrors([])
      setSubmitError('')
      setShowDiscardDialog(false)
      setHasUserInteracted(false)
    }
  }, [open, editProfile])

  const queryClient = useQueryClient()

  const { mutateAsync, isPending } = useMutation({
    mutationFn: async () => {
      const spec: VolumeImageProfileSpec = {
        osFamily,
        properties: rowsToProperties(rows),
        description: description.trim() || undefined
      }
      if (isEdit && editProfile) {
        return updateVolumeImageProfile({ ...editProfile, spec })
      }
      return createVolumeImageProfile(name.trim(), spec)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: VOLUME_IMAGE_PROFILES_QUERY_KEY })
      onClose()
    },
    onError: (error: Error) => {
      setSubmitError(error.message || 'Failed to save profile')
    }
  })

  const handleClose = () => {
    if (hasUserInteracted && !isSubmitting) {
      setShowDiscardDialog(true)
      return
    }
    setSubmitError('')
    onClose()
  }

  const handleDiscardChanges = async () => {
    setShowDiscardDialog(false)
    setSubmitError('')
    onClose()
  }

  const handleRemoveRow = (index: number) => {
    setRows((prev) => ensureTrailingEmpty(prev.filter((_, i) => i !== index)))
    setHasUserInteracted(true)
  }

  const handleRowChange = (index: number, field: 'key' | 'value', val: string) => {
    setRows((prev) => {
      const updated = prev.map((row, i) => (i === index ? { ...row, [field]: val } : row))
      return ensureTrailingEmpty(updated)
    })
    setHasUserInteracted(true)
  }

  const validate = (): boolean => {
    let valid = true
    setNameError('')
    setRowErrors([])
    setRowValueErrors([])
    setSubmitError('')

    if (!isEdit) {
      const trimmed = name.trim()
      if (!trimmed) {
        setNameError('Profile name is required')
        valid = false
      } else if (trimmed.length > 63) {
        setNameError('Name must be 63 characters or fewer')
        valid = false
      } else if (!/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(trimmed)) {
        // Enforces a Kubernetes DNS label (RFC 1123): lowercase alphanumerics
        // and hyphens, must start and end with alphanumeric.
        setNameError(
          'Name must be lowercase alphanumerics or hyphens, and start/end with an alphanumeric.'
        )
        valid = false
      }
    }

    const keyErrs = rows.map((row) => {
      if (row.value && !row.key.trim()) return 'Key is required when value is set'
      return ''
    })
    const valueErrs = rows.map((row) => {
      if (row.key.trim() && !row.value.trim()) return 'Value is required when key is set'
      return ''
    })
    if (keyErrs.some(Boolean) || valueErrs.some(Boolean)) {
      setRowErrors(keyErrs)
      setRowValueErrors(valueErrs)
      valid = false
    }

    const keys = rows.map((r) => r.key.trim()).filter(Boolean)
    if (keys.length !== new Set(keys).size) {
      setSubmitError(
        'This profile already has that property. Each key must be unique within a profile.'
      )
      valid = false
    }

    return valid
  }

  const handleSubmit = async () => {
    if (!validate()) return
    setIsSubmitting(true)
    try {
      await mutateAsync()
    } finally {
      setIsSubmitting(false)
    }
  }

  const hintForKey = (key: string) =>
    KNOWN_IMAGE_PROPERTIES.find((p) => p.key === key)?.hint ?? ''

  return (
    <>
      <DrawerShell
        open={open}
        onClose={handleClose}
        requireCloseConfirmation={false}
        header={
          <DrawerHeader
            title={isEdit ? 'Edit Profile' : 'Add Profile'}
            icon={<ProfileIcon color="primary" />}
            onClose={handleClose}
          />
        }
        footer={
          <DrawerFooter>
            <ActionButton tone="secondary" onClick={handleClose} disabled={isSubmitting || isPending}>
              Cancel
            </ActionButton>
            <ActionButton loading={isSubmitting || isPending} onClick={handleSubmit}>
              {isEdit ? 'Save Changes' : 'Create Profile'}
            </ActionButton>
          </DrawerFooter>
        }
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {isDefaultProfile && (
            <Alert severity="info" icon={<AutoAwesomeIcon fontSize="small" />}>
              This is a system default profile.
            </Alert>
          )}

          {/* Name */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <FieldLabel label="Profile Name" required align="flex-start" />
            <TextField
              size="small"
              fullWidth
              placeholder="e.g. windows-uefi-q35"
              value={name}
              onChange={(e) => {
                setName(e.target.value)
                setHasUserInteracted(true)
              }}
              disabled={isEdit}
              error={Boolean(nameError)}
              helperText={nameError || (isEdit ? 'Name cannot be changed after creation' : '')}
            />
          </Box>

          {/* OS Family */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <FieldLabel label="OS Family" required align="flex-start" />
            <FormControl size="small" fullWidth>
              <Select
                value={osFamily}
                onChange={(e) => {
                  setOsFamily(e.target.value as VolumeImageProfileSpec['osFamily'])
                  setHasUserInteracted(true)
                }}
              >
                {OS_FAMILY_OPTIONS.map((opt) => (
                  <MenuItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>

          {/* Description */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <FieldLabel label="Description" align="flex-start" />
            <TextField
              size="small"
              fullWidth
              placeholder="Optional"
              value={description}
              onChange={(e) => {
                setDescription(e.target.value)
                setHasUserInteracted(true)
              }}
            />
          </Box>

          {/* Properties key-value editor */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <FieldLabel label="Image Properties" align="flex-start" />

            <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr 36px', gap: 1, px: 0.5 }}>
              <Typography variant="caption" color="text.secondary" fontWeight={600}>
                Key
              </Typography>
              <Typography variant="caption" color="text.secondary" fontWeight={600}>
                Value
              </Typography>
            </Box>

            {rows.map((row, index) => {
              const isEmpty = !row.key && !row.value
              return (
                <Box
                  key={row.id}
                  sx={{
                    display: 'grid',
                    gridTemplateColumns: '1fr 1fr 36px',
                    gap: 1,
                    alignItems: 'flex-start'
                  }}
                >
                  <Autocomplete
                    freeSolo
                    size="small"
                    options={knownKeys}
                    value={row.key}
                    onInputChange={(_e, val, reason) => {
                      if (reason === 'reset') return
                      handleRowChange(index, 'key', val)
                    }}
                    renderInput={(params) => (
                      <TextField
                        {...params}
                        placeholder="Select or type a key"
                        error={Boolean(rowErrors[index])}
                        helperText={rowErrors[index]}
                        size="small"
                      />
                    )}
                  />
                  <TextField
                    size="small"
                    fullWidth
                    placeholder={hintForKey(row.key) || 'value'}
                    value={row.value}
                    onChange={(e) => handleRowChange(index, 'value', e.target.value)}
                    error={Boolean(rowValueErrors[index])}
                    helperText={rowValueErrors[index]}
                  />
                  {isEmpty ? (
                    <Box />
                  ) : (
                    <Tooltip title="Remove">
                      <IconButton size="small" onClick={() => handleRemoveRow(index)}>
                        <DeleteOutlineIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  )}
                </Box>
              )
            })}
          </Box>

          {submitError && <Alert severity="error">{submitError}</Alert>}
        </Box>
      </DrawerShell>

      <ConfirmationDialog
        open={showDiscardDialog}
        onClose={() => setShowDiscardDialog(false)}
        title="Discard Changes?"
        message="Are you sure you want to leave? Any unsaved changes will be lost."
        actionLabel="Leave"
        actionColor="warning"
        actionVariant="outlined"
        onConfirm={handleDiscardChanges}
      />
    </>
  )
}