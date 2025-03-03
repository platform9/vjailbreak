import {
    Autocomplete,
    Box,
    Button,
    CircularProgress,
    FormControl,
    FormHelperText,
    TextField,
    Typography,
} from "@mui/material"
import AddIcon from "@mui/icons-material/Add"
import { useState } from "react"

interface CredentialOption {
    label: string
    value: string
    metadata: {
        name: string
        namespace?: string
    }
    status?: {
        validationStatus?: string
        validationMessage?: string
    }
}

interface CredentialSelectorProps {
    label?: string
    placeholder?: string
    options: CredentialOption[]
    value: string | null
    onChange: (value: string | null) => void
    onAddNew: () => void
    loading: boolean
    error?: string
    emptyMessage?: string
    size?: "small" | "medium"
}

export default function CredentialSelector({
    label,
    placeholder,
    options,
    value,
    onChange,
    onAddNew,
    loading,
    error,
    size = "small",
    emptyMessage = "No credentials found",
}: CredentialSelectorProps) {
    const [inputValue, setInputValue] = useState("")

    const selectedOption = options.find(option => option.value === value) || null

    return (
        <FormControl fullWidth error={!!error}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
                <Typography variant="body1">{label}</Typography>
            </Box>
            <Box sx={{ display: "flex", gap: 1 }}>
                <Autocomplete
                    fullWidth
                    options={options}
                    loading={loading}
                    size={size}
                    value={selectedOption}
                    inputValue={inputValue}
                    onInputChange={(_, newInputValue) => {
                        setInputValue(newInputValue)
                    }}
                    onChange={(_, newValue) => {
                        onChange(newValue?.value || null)
                    }}
                    getOptionLabel={(option) => option.label}
                    noOptionsText={emptyMessage}
                    renderInput={(params) => (
                        <TextField
                            {...params}
                            label={label}
                            placeholder={placeholder}
                            variant="outlined"
                            size={size}
                            InputProps={{
                                ...params.InputProps,
                                endAdornment: (
                                    <>
                                        {loading ? <CircularProgress color="inherit" size={20} /> : null}
                                        {params.InputProps.endAdornment}
                                    </>
                                ),
                            }}
                        />
                    )}
                />
                <Button
                    color="primary"
                    onClick={onAddNew}
                    startIcon={<AddIcon />}
                    sx={{ minWidth: "120px" }}
                >
                    Add New
                </Button>
            </Box>
            {!!error && (
                <Box sx={{ mt: 1 }}>
                    <FormHelperText error>
                        {error}
                    </FormHelperText>
                </Box>
            )}
        </FormControl>
    )
} 