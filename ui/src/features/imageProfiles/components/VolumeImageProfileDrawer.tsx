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
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlined'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import ProfileIcon from '@mui/icons-material/Tune'
import { useState, useEffect } from 'react'
import { useForm, FormProvider } from 'react-hook-form'
import { useQueryClient, useMutation } from '@tanstack/react-query'
import { ConfirmationDialog } from 'src/components/dialogs'
import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell } from 'src/components/design-system'
import { RHFTextField } from 'src/shared/components/forms'
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

interface DrawerFormValues {
  name: string
  description: string
}

interface KeyValueRow {
  key: string
  value: string
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
  return Object.entries(props).map(([key, value]) => ({ key, value }))
}

function rowsToProperties(rows: KeyValueRow[]): Record<string, string> {
  const result: Record<string, string> = {}
  rows.forEach(({ key, value }) => {
    if (key.trim()) result[key.trim()] = value
  })
  return result
}

// Ensures the rows list always ends with exactly one empty row — the bottom
// input slot where users type their next entry. When they fill the empty row,
// a fresh empty row is appended below so typing can continue without any
// "add" button.
function ensureTrailingEmpty(rows: KeyValueRow[]): KeyValueRow[] {
  if (rows.length === 0) return [{ key: '', value: '' }]
  const last = rows[rows.length - 1]
  if (last.key || last.value) return [...rows, { key: '', value: '' }]
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

  const methods = useForm<DrawerFormValues>({
    defaultValues: { name: '', description: '' }
  })
  const { reset, trigger, getValues } = methods

  const [osFamily, setOsFamily] = useState<VolumeImageProfileSpec['osFamily']>('any')
  const [rows, setRows] = useState<KeyValueRow[]>([{ key: '', value: '' }])
  const [rowErrors, setRowErrors] = useState<string[]>([])
  const [submitError, setSubmitError] = useState('')
  const [showDiscardDialog, setShowDiscardDialog] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [hasUserInteracted, setHasUserInteracted] = useState(false)

  useEffect(() => {
    if (open) {
      reset({
        name: editProfile?.metadata.name ?? '',
        description: editProfile?.spec.description ?? ''
      })
      setOsFamily(editProfile?.spec.osFamily ?? 'any')
      setRows(
        ensureTrailingEmpty(
          editProfile?.spec.properties ? propertiesToRows(editProfile.spec.properties) : []
        )
      )
      setRowErrors([])
      setSubmitError('')
      setShowDiscardDialog(false)
      setHasUserInteracted(false)
    }
  }, [open, editProfile, reset])

  const queryClient = useQueryClient()

  const { mutateAsync, isPending } = useMutation({
    mutationFn: async () => {
      const { name: nameVal, description: descVal } = getValues()
      const spec: VolumeImageProfileSpec = {
        osFamily,
        properties: rowsToProperties(rows),
        description: descVal.trim() || undefined
      }
      if (isEdit && editProfile) {
        return updateVolumeImageProfile({ ...editProfile, spec })
      }
      return createVolumeImageProfile(nameVal.trim(), spec)
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

  const validate = async (): Promise<boolean> => {
    let valid = true
    setRowErrors([])
    setSubmitError('')

    if (!isEdit) {
      const nameValid = await trigger('name')
      if (!nameValid) valid = false
    }

    const errs = rows.map((row) => {
      if (row.value && !row.key.trim()) return 'Key is required when value is set'
      return ''
    })
    if (errs.some(Boolean)) {
      setRowErrors(errs)
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
    if (!(await validate())) return
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
    <FormProvider {...methods}>
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
          <RHFTextField
            name="name"
            label="Profile Name"
            required
            size="small"
            fullWidth
            placeholder="e.g. windows-uefi-q35"
            disabled={isEdit}
            helperText={isEdit ? 'Name cannot be changed after creation' : undefined}
            rules={!isEdit ? {
              required: 'Profile name is required',
              maxLength: { value: 253, message: 'Name must be 253 characters or fewer' },
              pattern: {
                value: /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/,
                message:
                  'Use only lowercase letters, numbers, or hyphens; cannot start or end with a hyphen'
              }
            } : undefined}
            onValueChange={() => setHasUserInteracted(true)}
          />

          {/* OS Family */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <Typography variant="body2" fontWeight={600}>
              OS Family <span style={{ color: 'red' }}>*</span>
            </Typography>
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
          <RHFTextField
            name="description"
            label="Description"
            size="small"
            fullWidth
            placeholder="Optional"
            onValueChange={() => setHasUserInteracted(true)}
          />

          {/* Properties key-value editor */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Typography variant="body2" fontWeight={600}>
              Image Properties
            </Typography>

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
                  key={index}
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
    </FormProvider>
  )
}