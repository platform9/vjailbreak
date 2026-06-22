import { Autocomplete, Box, TextField, Typography } from '@mui/material'
import FiberManualRecordIcon from '@mui/icons-material/FiberManualRecord'
import { FieldLabel } from 'src/components'
import type { VMOption } from './types'

interface VMAutocompleteProps {
  options: VMOption[]
  loading: boolean
  disabled: boolean
  value: VMOption | null
  onChange: (vm: VMOption | null) => void
  credSelected: boolean
}

function VMInfoText({ vm }: { vm: VMOption }) {
  const parts = [vm.ipAddress, vm.cpu ? `${vm.cpu} vCPU` : null].filter(Boolean)
  return (
    <Typography variant="caption" color="text.secondary" sx={{ flexShrink: 0 }}>
      {parts.join('  ·  ')}
    </Typography>
  )
}

export default function VMAutocomplete({
  options,
  loading,
  disabled,
  value,
  onChange,
  credSelected
}: VMAutocompleteProps) {
  return (
    <Box sx={{ display: 'grid', gap: 1 }}>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
        <FieldLabel label="Proxy VM" align="flex-start" />
        <Autocomplete<VMOption>
          options={options}
          loading={loading}
          disabled={disabled}
          value={value}
          onChange={(_, v) => onChange(v)}
          getOptionLabel={(o) => o.name}
          isOptionEqualToValue={(a, b) => a.name === b.name}
          filterOptions={(opts, { inputValue }) => {
            const q = inputValue.toLowerCase()
            return opts.filter(
              (o) =>
                o.name.toLowerCase().includes(q) ||
                (o.ipAddress?.toLowerCase().includes(q) ?? false)
            )
          }}
          renderInput={(params) => (
            <TextField
              {...params}
              size="small"
              placeholder={
                !credSelected
                  ? 'Select credentials first'
                  : loading
                    ? 'Loading VMs…'
                    : 'Filter by name or IP...'
              }
            />
          )}
          renderOption={(props, option) => (
            <Box component="li" {...props} key={option.name}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%' }}>
                <FiberManualRecordIcon
                  sx={{
                    fontSize: 10,
                    color: option.powerState === 'running' ? 'success.main' : 'text.disabled',
                    flexShrink: 0
                  }}
                />
                <Typography variant="body2" sx={{ flex: 1 }}>
                  {option.name}
                </Typography>
                <VMInfoText vm={option} />
              </Box>
            </Box>
          )}
          noOptionsText={!credSelected ? 'Select credentials first' : 'No powered-on VMs found'}
          slotProps={{ paper: { sx: { mt: 0.5 } } }}
        />
      </Box>

      {!value && credSelected && (
        <Typography variant="caption" color="text.secondary">
          Only powered-on VMs in the selected vCenter are listed — pick one instead of typing a
          name.
        </Typography>
      )}

      {value && (
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 1.5,
            py: 1,
            border: '1px solid',
            borderColor: 'divider',
            borderRadius: 1,
            bgcolor: 'background.default'
          }}
        >
          <FiberManualRecordIcon sx={{ fontSize: 10, color: 'success.main', flexShrink: 0 }} />
          <Typography variant="body2" fontWeight={500} sx={{ flex: 1 }}>
            {value.name}
          </Typography>
          <VMInfoText vm={value} />
        </Box>
      )}
    </Box>
  )
}
