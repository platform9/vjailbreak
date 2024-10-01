import { Visibility, VisibilityOff } from "@mui/icons-material"
import CheckIcon from "@mui/icons-material/Check"
import {
  Box,
  CircularProgress,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  styled,
  Typography,
} from "@mui/material"
import { useCallback, useEffect, useState } from "react"
import { debounce } from "src/utils"
import OpenstackRCFileUpload from "../../components/forms/OpenstackRCFileUpload"
import Step from "../../components/forms/Step"
import TextField from "../../components/forms/TextField"

const SourceAndDestinationStepContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridGap: theme.spacing(1),
}))

const FieldsContainer = styled("div")(({ theme }) => ({
  display: "grid",
  marginLeft: theme.spacing(6),
}))

const Fields = styled("div")(() => ({
  display: "grid",
  gridTemplateColumns: "1fr 1fr", // Ensures equal column width for both input fields
  gridGap: "16px", // Adds spacing between the columns
}))

interface SourceAndDestinationEnvStepProps {
  params: { [key: string]: unknown }
  onChange: (id: string) => (value: unknown) => void
  errors: { [fieldId: string]: string }
  vmwareCredsValidated?: boolean | null
  validatingVmwareCreds?: boolean
  validatingOpenstackCreds?: boolean
  openstackCredsValidated?: boolean | null
}

export default function SourceAndDestinationEnvStep({
  params,
  onChange,
  errors,
  validatingVmwareCreds = false,
  validatingOpenstackCreds = false,
  vmwareCredsValidated = null,
  openstackCredsValidated = null,
}: SourceAndDestinationEnvStepProps) {
  const [showPassword, setShowPassword] = useState(false)
  const [vmwareCreds, setVmwareCreds] = useState({
    vcenterHost: "",
    datacenter: "",
    username: "",
    password: "",
  })

  const handleClickShowPassword = () => setShowPassword((show) => !show)

  const handleMouseDownPassword = (
    event: React.MouseEvent<HTMLButtonElement>
  ) => {
    event.preventDefault()
  }

  const handleVmwareCredsChange = (value) => {
    setVmwareCreds({ ...vmwareCreds, ...value })
  }

  const handleOpenstackCredsChange = (values) => {
    onChange("openstackCreds")(values)
  }

  const debouncedOnChange = useCallback(
    debounce((creds) => {
      onChange("vmwareCreds")(creds) // Pass the validated creds to the parent component
    }, 1000 * 3), // Debounce for 3 seconds
    [onChange]
  )

  useEffect(() => {
    // Only call debouncedOnChange when all required fields are filled out
    if (
      vmwareCreds.vcenterHost &&
      vmwareCreds.datacenter &&
      vmwareCreds.username &&
      vmwareCreds.password
    ) {
      debouncedOnChange(vmwareCreds)
    }

    // Cleanup debounced function on unmount or when creds change
    return () => {
      debouncedOnChange.cancel() // Ensure debounce is cleared
    }
  }, [vmwareCreds, debouncedOnChange])

  return (
    <SourceAndDestinationStepContainer>
      <Step stepNumber="1" label="Source and Destination Environments" />
      <FieldsContainer>
        <FormControl fullWidth error={!!errors["vmwareCreds"]} required>
          <Box
            sx={{
              display: "grid",
            }}
          >
            <Typography variant="body1">Source VMWare</Typography>
            <Fields>
              <TextField
                id="vcenterHost"
                label="vCenter Server"
                variant="outlined"
                value={params["datacenter"]}
                onChange={(e) =>
                  handleVmwareCredsChange({ vcenterHost: e.target.value })
                }
                error={!!errors.sourceEnv}
                required
              />
              <TextField
                id="datacenter"
                label="Datacenter Name"
                variant="outlined"
                value={params["datacenter"]}
                onChange={(e) =>
                  handleVmwareCredsChange({ datacenter: e.target.value })
                }
                error={!!errors.sourceEnv}
                required
              />
            </Fields>

            <Fields>
              <TextField
                id="username"
                label="Username"
                variant="outlined"
                value={params["username"]}
                onChange={(e) =>
                  handleVmwareCredsChange({ username: e.target.value })
                }
                error={!!errors.sourceEnv}
                fullWidth
                required
              />
              <TextField
                label="Password"
                type={showPassword ? "text" : "password"}
                variant="outlined"
                slotProps={{
                  input: {
                    endAdornment: (
                      <InputAdornment position="end">
                        <IconButton
                          onClick={handleClickShowPassword}
                          onMouseDown={handleMouseDownPassword}
                          edge="end"
                        >
                          {showPassword ? <VisibilityOff /> : <Visibility />}
                        </IconButton>
                      </InputAdornment>
                    ),
                  },
                }}
                onChange={(e) =>
                  handleVmwareCredsChange({ password: e.target.value })
                }
                fullWidth
                required
              />
            </Fields>
          </Box>
          <Box sx={{ display: "flex", gap: 2, mt: 1 }}>
            {validatingVmwareCreds && (
              <>
                <CircularProgress size={24} />
                <FormLabel sx={{ mb: 1 }}>Validating VMWare Creds...</FormLabel>
              </>
            )}
            {vmwareCredsValidated && (
              <>
                <CheckIcon color="success" fontSize="small" />
                <FormLabel sx={{ mb: 1 }}>VMWare Creds Validated</FormLabel>
              </>
            )}
            {!!errors["vmwareCreds"] && (
              <FormLabel error sx={{ mb: 1 }}>
                {errors["vmwareCreds"]}
              </FormLabel>
            )}
          </Box>
        </FormControl>
      </FieldsContainer>
      <FieldsContainer>
        <Typography variant="body1">Destination Platform</Typography>
        <FormControl fullWidth error={!!errors["openstackCreds"]} required>
          <OpenstackRCFileUpload onChange={handleOpenstackCredsChange} />
          <Box sx={{ display: "flex", gap: 2, mt: 1 }}>
            {validatingOpenstackCreds && (
              <>
                <CircularProgress size={24} />
                <FormLabel sx={{ mb: 1 }}>
                  Validating Openstack Creds...
                </FormLabel>
              </>
            )}
            {openstackCredsValidated && (
              <>
                <CheckIcon color="success" fontSize="small" />
                <FormLabel sx={{ mb: 1 }}>Openstack Creds Validated</FormLabel>
              </>
            )}
            {!!errors["openstackCreds"] && (
              <FormLabel error sx={{ mb: 1 }}>
                {errors["openstackCreds"]}
              </FormLabel>
            )}
          </Box>
        </FormControl>
      </FieldsContainer>
    </SourceAndDestinationStepContainer>
  )
}
