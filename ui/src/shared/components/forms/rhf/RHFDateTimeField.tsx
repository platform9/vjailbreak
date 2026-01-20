import { Box } from '@mui/material'
import { DateTimePicker, DateTimePickerProps } from '@mui/x-date-pickers/DateTimePicker'
import dayjs, { Dayjs } from 'dayjs'
import { Controller, ControllerProps, FieldValues, useFormContext } from 'react-hook-form'
import { FieldLabel, FieldLabelProps } from 'src/components'

import TextField from '../TextField'

type ControllerRules = ControllerProps<FieldValues>['rules']

type FieldLabelCustomProps = Omit<FieldLabelProps, 'label' | 'helperText'>

type PickerProps = Omit<DateTimePickerProps<Dayjs>, 'value' | 'onChange' | 'slotProps' | 'slots'>

export type RHFDateTimeFieldProps = PickerProps & {
  name: ControllerProps<FieldValues>['name']
  rules?: ControllerRules
  label?: string
  required?: boolean
  disabled?: boolean
  helperText?: string
  error?: boolean
  labelProps?: FieldLabelCustomProps
  labelHelperText?: FieldLabelProps['helperText']
  placeholder?: string
  onPickerError?: (error: unknown) => void
}

export default function RHFDateTimeField({
  name,
  rules,
  label,
  required,
  disabled,
  helperText,
  error: errorProp,
  labelProps,
  labelHelperText,
  placeholder,
  onPickerError,
  ...rest
}: RHFDateTimeFieldProps) {
  const { control } = useFormContext()

  return (
    <Controller
      name={name}
      control={control}
      rules={rules}
      render={({ field, fieldState }) => {
        const parsed = field.value ? dayjs(String(field.value)) : null
        const value = parsed && parsed.isValid() ? parsed : null

        return (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
            {label ? (
              <FieldLabel
                label={label}
                helperText={labelHelperText}
                required={required}
                align="flex-start"
                {...labelProps}
              />
            ) : null}
            <DateTimePicker
              {...rest}
              ampm={false}
              value={value}
              onChange={(newValue) => {
                field.onChange(newValue ? newValue.format() : '')
              }}
              onError={(error) => {
                onPickerError?.(error)
              }}
              slotProps={{
                popper: {
                  disablePortal: true,
                  placement: 'bottom-start'
                },
                textField: {
                  variant: 'outlined',
                  sx: {
                    width: '100%'
                  },
                  size: 'small',
                  required,
                  placeholder,
                  error: fieldState.invalid || errorProp,
                  helperText: fieldState.error?.message ?? helperText,
                  disabled
                }
              }}
              slots={{
                textField: TextField
              }}
            />
          </Box>
        )
      }}
    />
  )
}
