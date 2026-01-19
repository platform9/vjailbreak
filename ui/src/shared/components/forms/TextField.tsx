import { TextField as BaseTextField, TextFieldProps as BaseTextFieldProps } from '@mui/material'
import { forwardRef } from 'react'

export type TextFieldProps = BaseTextFieldProps

const TextField = forwardRef<HTMLDivElement, TextFieldProps>((props, ref) => {
  return (
    <BaseTextField
      ref={ref}
      variant="outlined"
      size="small"
      sx={{
        width: '100%',
        ...(props.sx || {})
      }}
      {...props}
    />
  )
})

TextField.displayName = 'TextField'

export default TextField
