import { TextField as BaseTextField, TextFieldProps as BaseTextFieldProps } from '@mui/material'

export type TextFieldProps = BaseTextFieldProps

export default function TextField(props: TextFieldProps) {
  return (
    <BaseTextField
      variant="outlined"
      size="small"
      sx={{
        width: '100%',
        ...(props.sx || {})
      }}
      {...props}
    />
  )
}
