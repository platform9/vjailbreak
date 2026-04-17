import {
  Box,
  IconButton,
  Tooltip,
  Chip,
  TextField,
  MenuItem,
  Select,
  FormControl,
  Alert,
  Typography,
  Autocomplete
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutlined'
import LockIcon from '@mui/icons-material/Lock'
import { useState } from 'react'
import { useQueryClient, useMutation } from '@tanstack/react-query'
import {
  StyledDrawer,
  DrawerContent,
  Header,
  Footer
} from 'src/shared/components/forms'
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
import TuneIcon from '@mui/icons-material/Tune'

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
  { label: 'Windows', value: 'windows' },
  { label: 'Linux', value: 'linux' },
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

export default function VolumeImageProfileDrawer({
  open,
  onClose,
  editProfile
}: VolumeImageProfileDrawerProps) {
  const isEdit = Boolean(editProfile)
  const isDefaultProfile =
    editProfile && DEFAULT_PROFILE_NAMES.includes(editProfile.metadata.name)

  const [name, setName] = useState(editProfile?.metadata.name ?? '')
  const [osFamily, setOsFamily] = useState<VolumeImageProfileSpec['osFamily']>(
    editProfile?.spec.osFamily ?? 'any'
  )
  const [description, setDescription] = useState(editProfile?.spec.description ?? '')
  const [rows, setRows] = useState<KeyValueRow[]>(
    editProfile?.spec.properties
      ? propertiesToRows(editProfile.spec.properties)
      : [{ key: '', value: '' }]
  )
  const [nameError, setNameError] = useState('')
  const [rowErrors, setRowErrors] = useState<string[]>([])
  const [submitError, setSubmitError] = useState('')

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

  const handleAddRow = () => {
    setRows((prev) => [...prev, { key: '', value: '' }])
  }

  const handleRemoveRow = (index: number) => {
    setRows((prev) => prev.filter((_, i) => i !== index))
  }

  const handleRowChange = (index: number, field: 'key' | 'value', val: string) => {
    setRows((prev) => prev.map((row, i) => (i === index ? { ...row, [field]: val } : row)))
  }

  const validate = (): boolean => {
    let valid = true
    setNameError('')
    setRowErrors([])
    setSubmitError('')

    if (!isEdit && !name.trim()) {
      setNameError('Profile name is required')
      valid = false
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
    const hasDuplicate = keys.length !== new Set(keys).size
    if (hasDuplicate) {
      setSubmitError('Duplicate property keys are not allowed')
      valid = false
    }

    return valid
  }

  const handleSubmit = async () => {
    if (!validate()) return
    await mutateAsync()
  }

  const hintForKey = (key: string) =>
    KNOWN_IMAGE_PROPERTIES.find((p) => p.key === key)?.hint ?? ''

  return (
    <StyledDrawer anchor="right" open={open} onClose={onClose}>
      <Header
        title={isEdit ? 'Edit Image Profile' : 'Add Image Profile'}
        icon={<TuneIcon color="primary" />}
      />

      <DrawerContent>
        {isDefaultProfile && (
          <Alert severity="info" icon={<LockIcon fontSize="small" />} sx={{ mb: 3 }}>
            This is a system default profile. Changes here override the built-in defaults.
          </Alert>
        )}

        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {/* Name */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <Typography variant="body2" fontWeight={600}>
              Profile Name <span style={{ color: 'red' }}>*</span>
            </Typography>
            <TextField
              size="small"
              fullWidth
              placeholder="e.g. windows-uefi-q35"
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={isEdit}
              error={Boolean(nameError)}
              helperText={nameError || (isEdit ? 'Name cannot be changed after creation' : '')}
            />
          </Box>

          {/* OS Family */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <Typography variant="body2" fontWeight={600}>
              OS Family <span style={{ color: 'red' }}>*</span>
            </Typography>
            <FormControl size="small" fullWidth>
              <Select
                value={osFamily}
                onChange={(e) =>
                  setOsFamily(e.target.value as VolumeImageProfileSpec['osFamily'])
                }
              >
                {OS_FAMILY_OPTIONS.map((opt) => (
                  <MenuItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </MenuItem>
                ))}
              </Select>
              {/* <FormHelperText>
                Profile is auto-applied in migration form for matching OS type
              </FormHelperText> */}
            </FormControl>
          </Box>

          {/* Description */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            <Typography variant="body2" fontWeight={600}>
              Description
            </Typography>
            <TextField
              size="small"
              fullWidth
              placeholder="Optional"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </Box>

          {/* Properties key-value editor */}
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <Typography variant="body2" fontWeight={600}>
                Image Properties
              </Typography>
              <Tooltip title="Add property">
                <IconButton size="small" onClick={handleAddRow} color="primary">
                  <AddIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            </Box>

            {/* Header row */}
            <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr 36px', gap: 1, px: 0.5 }}>
              <Typography variant="caption" color="text.secondary" fontWeight={600}>
                Key
              </Typography>
              <Typography variant="caption" color="text.secondary" fontWeight={600}>
                Value
              </Typography>
            </Box>

            {rows.map((row, index) => {
              const hint = hintForKey(row.key)
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
                    onInputChange={(_e, val) => handleRowChange(index, 'key', val)}
                    renderInput={(params) => (
                      <TextField
                        {...params}
                        placeholder="e.g. hw_firmware_type"
                        error={Boolean(rowErrors[index])}
                        helperText={rowErrors[index]}
                        size="small"
                      />
                    )}
                  />
                  <TextField
                    size="small"
                    fullWidth
                    placeholder={hint || 'value'}
                    value={row.value}
                    onChange={(e) => handleRowChange(index, 'value', e.target.value)}
                  />
                  <Tooltip title="Remove">
                    <span>
                      <IconButton
                        size="small"
                        onClick={() => handleRemoveRow(index)}
                        disabled={rows.length === 1}
                      >
                        <DeleteOutlineIcon fontSize="small" />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Box>
              )
            })}

            {/* Known properties reference chips */}
            <Box sx={{ mt: 1, display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
              {KNOWN_IMAGE_PROPERTIES.map((p) => (
                <Tooltip key={p.key} title={p.hint} placement="top">
                  <Chip
                    label={p.key}
                    size="small"
                    variant="outlined"
                    onClick={() => {
                      const emptyRow = rows.findIndex((r) => !r.key)
                      if (emptyRow >= 0) {
                        handleRowChange(emptyRow, 'key', p.key)
                      } else {
                        setRows((prev) => [...prev, { key: p.key, value: '' }])
                      }
                    }}
                    sx={{ fontSize: '0.7rem', cursor: 'pointer' }}
                  />
                </Tooltip>
              ))}
            </Box>
            <Typography variant="caption" color="text.secondary">
              Click a chip to add a known property. Hover for accepted values.
            </Typography>
          </Box>

          {submitError && <Alert severity="error">{submitError}</Alert>}
        </Box>
      </DrawerContent>

      <Footer
        submitButtonLabel={isEdit ? 'Save Changes' : 'Create Profile'}
        onClose={onClose}
        onSubmit={handleSubmit}
        submitting={isPending}
      />
    </StyledDrawer>
  )
}
