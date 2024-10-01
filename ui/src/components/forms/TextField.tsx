import {
  TextField as BaseTextField,
  TextFieldProps as BaseTextFieldProps,
} from "@mui/material"

export default function TextField(props: BaseTextFieldProps) {
  return (
    <BaseTextField
      variant="outlined"
      size="small"
      sx={{
        maxWidth: "400px",
        margin: "12px 0",
      }}
      {...props}
    />
  )
}
