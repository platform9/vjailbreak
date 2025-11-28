import { TextField as BaseTextField, TextFieldProps as BaseTextFieldProps } from '@mui/material'

export type TextFieldProps = BaseTextFieldProps

export default function TextField(props: TextFieldProps) {
  return (
    <BaseTextField
      variant="outlined"
      size="small"
      sx={{
        maxWidth: '400px',
        margin: '8px 0'
      }}
      {...props}
    />
  )
}
