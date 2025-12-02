import { Box, Select, MenuItem, FormControl, InputLabel } from '@mui/material'
import { Controller, ControllerProps, FieldValues, useFormContext } from 'react-hook-form'
import { FieldLabel, FieldLabelProps } from 'src/design-system'

type ControllerRules = ControllerProps<FieldValues>['rules']

type FieldLabelCustomProps = Omit<FieldLabelProps, 'label' | 'helperText'>

export interface Option {
  label: string
  value: any
}

export type RHFSelectProps = {
  name: ControllerProps<FieldValues>['name']
  options: Option[]
  rules?: ControllerRules
  labelHelperText?: FieldLabelProps['helperText']
  labelProps?: FieldLabelCustomProps
  placeholder?: string
  label?: string
  disabled?: boolean
  helperText?: string
  error?: boolean
}

export default function RHFSelect({
  name,
  options,
  rules,
  helperText,
  error: errorProp,
  label,
  labelHelperText,
  labelProps,
  placeholder,
  disabled,
  ...rest
}: RHFSelectProps) {
  const { control } = useFormContext()

  return (
    <Controller
      name={name}
      control={control}
      rules={rules}
      render={({ field, fieldState }) => (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0 }}>
          {label ? (
            <FieldLabel
              label={label}
              helperText={labelHelperText}
              required={(rest as any).required}
              align="flex-start"
              {...labelProps}
            />
          ) : null}
          <FormControl
            variant="outlined"
            size="small"
            disabled={disabled}
            error={fieldState.invalid || errorProp}
          >
            {label && <InputLabel id={`${name}-label`}>{label}</InputLabel>}
            <Select
              {...rest}
              {...field}
              labelId={label ? `${name}-label` : undefined}
              label={label}
              value={field.value ?? ''}
              displayEmpty
            >
              {placeholder && (
                <MenuItem value="" disabled>
                  {placeholder}
                </MenuItem>
              )}
              {options.map((option) => (
                <MenuItem key={String(option.value)} value={option.value}>
                  {option.label}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
          {fieldState.error?.message && (
            <Box sx={{ color: 'error.main', fontSize: '0.75rem', mt: 0.5 }}>
              {fieldState.error.message}
            </Box>
          )}
          {!fieldState.error && helperText && (
            <Box sx={{ color: 'text.secondary', fontSize: '0.75rem', mt: 0.5 }}>{helperText}</Box>
          )}
        </Box>
      )}
    />
  )
}
