import { Box, TextField } from '@mui/material'
import { useCallback } from 'react'

type IntervalFieldProps = {
  label: string
  name: string
  value: string
  helper?: string
  error?: string
  disabled?: boolean
  required?: boolean
  getErrorsUpdater?: any
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void
}

const IntervalField = ({
  label,
  name,
  value,
  helper,
  error,
  disabled,
  required,
  onChange,
  getErrorsUpdater
}: IntervalFieldProps) => {
  const validate = useCallback((val: string): string | undefined => {
    const trimmedVal = val?.trim()
    if (!trimmedVal) return undefined

    const regex = /^([0-9]+)(s|m|h)$/
    const match = trimmedVal.match(regex)
    if (!match) return 'Use duration format like 30s, 5m, 1h (units: s,m,h).'

    const num = Number(match[1])
    const unit = match[2].toLowerCase()
    const minutes = unit === 's' ? num / 60 : unit === 'm' ? num : unit === 'h' ? num * 60 : NaN

    if (isNaN(minutes) || minutes < 5) return 'Interval must be at least 5 minutes'
    return undefined
  }, [])

  const validationError = validate(value)
  const hasError = !!error || !!validationError?.trim()

  return (
    <Box display="flex" flexDirection="column" gap={0.5}>
      {/* <Typography variant="body2" fontWeight={500}>
        {label} {required && '*'}
      </Typography> */}
      <TextField
        fullWidth
        size="small"
        name={String(name)}
        placeholder={`${label}${required ? ' *' : ''}`}
        value={value}
        onChange={onChange}
        disabled={disabled}
        required={required}
        error={hasError}
        onBlur={() => getErrorsUpdater?.(name)(validationError || '')}
        helperText={error || helper || validationError || 'e.g. 30s, 5m, 1h30m'}
      />
    </Box>
  )
}

export default IntervalField
