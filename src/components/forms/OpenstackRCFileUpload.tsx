import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  styled,
  TextField,
} from "@mui/material"
import { parse } from "dotenv"
import React, { useState } from "react"

const requiredFields = [
  "OS_AUTH_URL",
  "OS_DOMAIN_NAME",
  "OS_USERNAME",
  "OS_PASSWORD",
  "OS_REGION_NAME",
  "OS_TENANT_NAME",
]

const FileUploadFieldContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridTemplateColumns: "1fr auto",
  alignItems: "center",
  gap: theme.spacing(2),
  marginTop: theme.spacing(2),
}))

interface OpenstackRCFileUploaderProps {
  onChange: (values: unknown) => void
}

export default function OpenstackRCFileUploader({
  onChange,
}: OpenstackRCFileUploaderProps) {
  const [fileName, setFileName] = useState<string>("")
  const [error, setError] = useState<string | null>(null)

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (file) {
      setFileName(file.name)
      parseRCFile(file)
    }
  }

  const parseRCFile = (file: File) => {
    const reader = new FileReader()
    reader.onload = (e) => {
      try {
        setError(null) // Clear any previous errors
        const content = e.target?.result as string
        const parsedFields = parseFields(content)
        const isValid = validateFields(parsedFields)
        if (isValid) {
          onChange(parsedFields)
        }
      } catch {
        setError("Failed to parse the file. Please check the file format.")
      }
    }
    reader.onerror = () => {
      setError("Failed to read the file. Please try again.")
    }
    reader.readAsText(file)
  }

  const parseFields = (content: string) => {
    // Remove 'export' from each line before parsing with dotenv
    const cleanedContent = content.replace(/^export\s+/gm, "")
    return parse(cleanedContent)
  }

  const validateFields = (fields: Record<string, string>) => {
    const missingFields = requiredFields.filter(
      (field) => !fields[field] || fields[field].trim() === ""
    )
    if (missingFields.length > 0) {
      setError(`Missing required fields: ${missingFields.join(", ")}`)
      return false
    }
    return true
  }

  return (
    <FileUploadFieldContainer>
      <FormControl error={!!error} fullWidth>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "1fr auto",
            gridGap: (theme) => theme.spacing(2),
          }}
        >
          <TextField
            label="OpenStack RC File"
            value={fileName}
            variant="outlined"
            component="label"
            color="primary"
            aria-readonly
            size="small"
            required
          />
          <Button variant="contained" component="label" color="primary">
            Choose File
            <input
              type="file"
              accept=".sh"
              hidden
              onChange={handleFileChange}
            />
          </Button>
        </Box>
        <FormHelperText error={!!error}>{error}</FormHelperText>
      </FormControl>
    </FileUploadFieldContainer>
  )
}
