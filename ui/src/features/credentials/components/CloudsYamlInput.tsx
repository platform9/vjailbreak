import {
  Alert,
  Box,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from '@mui/material'
import { ChangeEvent, useMemo, useRef, useState } from 'react'
import {
  parseCloudsYAML,
  detectAuthMethod,
  ParseResult,
} from 'src/utils/cloudsYamlParser'

export interface CloudsYamlInputValue {
  /** Raw clouds.yaml content (the string that will be stored in the Secret). */
  cloudsYaml: string
  /** Selected cloud entry name; required when the YAML has >1 entry. */
  cloudName: string
  /** Parse status; emitted so parent forms can disable submit on errors. */
  isValid: boolean
}

export interface CloudsYamlInputProps {
  initialValue?: string
  onChange: (value: CloudsYamlInputValue) => void
  disabled?: boolean
}

const PLACEHOLDER = `clouds:
  destination:
    auth_type: v3applicationcredential
    auth:
      auth_url: https://keystone.example.com:5000/v3
      application_credential_id: <id>
      application_credential_secret: <secret>
    region_name: RegionOne
    interface: public
`

/**
 * Credential input for the OpenStack clouds.yaml format.
 *
 * Operators paste their clouds.yaml content or upload a file; the component
 * parses client-side via {@link parseCloudsYAML}, surfaces parse errors
 * inline, populates a cloud-name selector when the YAML carries multiple
 * cloud entries, and shows an auth-method badge (password vs Application
 * Credential) so the operator can confirm before submission.
 *
 * The component is intentionally stateless about persistence — the parent
 * form decides when to submit and is responsible for calling the API helper
 * that writes the Secret + OpenstackCreds resource.
 */
export default function CloudsYamlInput({
  initialValue = '',
  onChange,
  disabled,
}: CloudsYamlInputProps) {
  const [raw, setRaw] = useState<string>(initialValue)
  const [selectedCloud, setSelectedCloud] = useState<string>('')
  const fileInputRef = useRef<HTMLInputElement | null>(null)

  const parseResult: ParseResult = useMemo(() => parseCloudsYAML(raw), [raw])

  const cloudNames = parseResult.ok ? parseResult.cloudNames : []
  const effectiveCloud =
    cloudNames.length === 1 ? cloudNames[0] : selectedCloud
  const selectedEntry =
    parseResult.ok && effectiveCloud
      ? parseResult.parsed.clouds?.[effectiveCloud]
      : undefined
  const authMethod = detectAuthMethod(selectedEntry)

  const isValid =
    parseResult.ok &&
    effectiveCloud !== '' &&
    authMethod !== 'unsupported'

  // Emit upstream whenever effective state changes.
  useMemoOnChange(() => {
    onChange({
      cloudsYaml: raw,
      cloudName: effectiveCloud,
      isValid,
    })
  }, [raw, effectiveCloud, isValid])

  const onTextChange = (e: ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => {
    setRaw(e.target.value)
    setSelectedCloud('')
  }

  const onFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) {
      return
    }
    const reader = new FileReader()
    reader.onload = (event) => {
      const text = String(event.target?.result ?? '')
      setRaw(text)
      setSelectedCloud('')
    }
    reader.readAsText(file)
  }

  return (
    <Box display="flex" flexDirection="column" gap={2}>
      <Typography variant="body2" color="text.secondary">
        Paste your <code>clouds.yaml</code> content below, or upload a file.
        The content is parsed in your browser; nothing is sent until you
        submit.
      </Typography>

      <Box display="flex" gap={1} alignItems="center">
        <input
          type="file"
          accept=".yaml,.yml,application/x-yaml,text/yaml"
          ref={fileInputRef}
          style={{ display: 'none' }}
          onChange={onFileChange}
        />
        <button
          type="button"
          onClick={() => fileInputRef.current?.click()}
          disabled={disabled}
        >
          Upload clouds.yaml
        </button>
        <Typography variant="caption" color="text.secondary">
          or paste below
        </Typography>
      </Box>

      <TextField
        label="clouds.yaml"
        multiline
        minRows={10}
        maxRows={20}
        fullWidth
        value={raw}
        onChange={onTextChange}
        placeholder={PLACEHOLDER}
        disabled={disabled}
        spellCheck={false}
        slotProps={{
          input: {
            sx: { fontFamily: 'monospace', fontSize: '0.85rem' },
          },
        }}
      />

      {!parseResult.ok && raw !== '' && (
        <Alert severity="error" variant="outlined">
          {parseResult.error}
          {parseResult.line !== undefined && (
            <> (line {parseResult.line + 1}
            {parseResult.column !== undefined && `, column ${parseResult.column + 1}`}
            )</>
          )}
        </Alert>
      )}

      {parseResult.ok && cloudNames.length > 1 && (
        <FormControl size="small" fullWidth>
          <InputLabel id="clouds-yaml-cloud-name-label">Cloud entry</InputLabel>
          <Select
            labelId="clouds-yaml-cloud-name-label"
            label="Cloud entry"
            value={selectedCloud}
            onChange={(e) => setSelectedCloud(e.target.value)}
            disabled={disabled}
          >
            {cloudNames.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      )}

      {parseResult.ok && effectiveCloud && (
        <Box display="flex" gap={1} alignItems="center" flexWrap="wrap">
          <Typography variant="body2">Auth method:</Typography>
          {authMethod === 'applicationCredential' && (
            <Chip
              label="Application Credential"
              color="success"
              size="small"
              variant="outlined"
            />
          )}
          {authMethod === 'password' && (
            <Chip label="Password" size="small" variant="outlined" />
          )}
          {authMethod === 'unsupported' && (
            <Chip
              label={`Unsupported auth_type: ${selectedEntry?.auth_type ?? '(none)'}`}
              color="error"
              size="small"
              variant="outlined"
            />
          )}
        </Box>
      )}
    </Box>
  )
}

/**
 * Tiny effect helper: call `fn` whenever any of `deps` changes. Avoids
 * pulling React useEffect's lint dependency check noise into this file by
 * keeping the dep list explicit.
 */
function useMemoOnChange(fn: () => void, deps: unknown[]) {
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useMemo(() => {
    fn()
  }, deps)
}
