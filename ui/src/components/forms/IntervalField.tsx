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

    // Allow composite formats like 1h30m, 5m30s, etc.
    const regex = /^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/
    const match = trimmedVal.match(regex)

    if (!match || match[0] === '') {
      return 'Use duration format like 30s, 5m, 1h, 1h30m, 5m30s (units: h,m,s).'
    }

    const hours = match[1] ? Number(match[1]) : 0
    const minutes = match[2] ? Number(match[2]) : 0
    const seconds = match[3] ? Number(match[3]) : 0

    // Convert total duration to minutes
    const totalMinutes = hours * 60 + minutes + seconds / 60

    if (isNaN(totalMinutes) || totalMinutes < 5) {
      return 'Interval must be at least 5 minutes'
    }

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
