import { AdapterDayjs } from '@mui/x-date-pickers/AdapterDayjs'
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider'
import { RHFDateTimeField } from 'src/shared/components/forms'

export interface BucketScheduleFieldProps {
  /** RHF field name (defaults to "schedule"). Must be inside a FormProvider. */
  name?: string
  label?: string
  disabled?: boolean
}

/**
 * Optional per-bucket schedule time (FR-016). Future-only: `disablePast` disables past dates
 * and times. Wrapped in LocalizationProvider (AdapterDayjs) like the migration form, since the
 * app has no global date-pickers provider.
 */
export default function BucketScheduleField({
  name = 'schedule',
  label = 'Schedule (optional)',
  disabled
}: BucketScheduleFieldProps) {
  return (
    <LocalizationProvider dateAdapter={AdapterDayjs}>
      <RHFDateTimeField
        name={name}
        label={label}
        disabled={disabled}
        disablePast
        labelHelperText="Migration starts no earlier than this time. Past times are disabled."
        placeholder="Select a future date & time"
      />
    </LocalizationProvider>
  )
}
