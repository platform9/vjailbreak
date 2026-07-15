import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Chip,
  Divider,
  FormControlLabel,
  IconButton,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography
} from '@mui/material'
import AddCircleIcon from '@mui/icons-material/AddCircle'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import RemoveCircleIcon from '@mui/icons-material/RemoveCircle'
import type { VmData } from '../api/migration-templates/model'
import type { KeyValuePair } from '../types'
import { METADATA_MAX_LENGTH, countVmSourceEntries, summarizeSourceEntries } from '../utils/metadataUtils'

interface TagsAndMetadataSectionProps {
  // Selected VMs with tag data; undefined when the caller can't provide a preview
  // (e.g. the rolling migration form) — the preview accordion is hidden then.
  vms?: VmData[]
  preserveSourceTags: boolean
  customMetadata: KeyValuePair[]
  onChange: (key: string) => (value: unknown) => void
}

const renderEntryChips = (entries?: Record<string, string>) => {
  const keys = Object.keys(entries || {})
  if (keys.length === 0) {
    return (
      <Typography variant="caption" color="text.disabled" fontStyle="italic">
        None
      </Typography>
    )
  }
  return (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
      {keys.map((key) => (
        <Chip
          key={key}
          size="small"
          variant="outlined"
          label={`${key}: ${entries?.[key]}`}
          sx={{ fontFamily: 'monospace', fontSize: 11 }}
        />
      ))}
    </Box>
  )
}

export default function TagsAndMetadataSection({
  vms,
  preserveSourceTags,
  customMetadata,
  onChange
}: TagsAndMetadataSectionProps) {
  const { vmCount, entryCount } = summarizeSourceEntries(vms || [])

  const updateRow = (index: number, field: keyof KeyValuePair, value: string) => {
    const next = customMetadata.map((row, i) => (i === index ? { ...row, [field]: value } : row))
    onChange('customMetadata')(next)
  }

  const removeRow = (index: number) => {
    onChange('customMetadata')(customMetadata.filter((_, i) => i !== index))
  }

  const addRow = () => {
    onChange('customMetadata')([...customMetadata, { key: '', value: '' }])
  }

  return (
    <Box sx={{ display: 'grid', gap: 1.25 }}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'baseline',
          justifyContent: 'space-between',
          gap: 2,
          mb: 1
        }}
      >
        <Typography variant="subtitle2">Tags &amp; Metadata</Typography>
        <Typography variant="caption" color="text.secondary">
          Carry organizational context from VMware to the migrated VMs
        </Typography>
      </Box>
      <Divider />

      <Box sx={{ py: 1 }}>
        <FormControlLabel
          control={
            <Switch
              data-testid="preserve-source-tags-toggle"
              checked={preserveSourceTags}
              onChange={(e) => onChange('preserveSourceTags')(e.target.checked)}
            />
          }
          label="Preserve VMware tags and custom attributes"
        />
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', ml: 6 }}>
          {preserveSourceTags
            ? "Copies each VM's vSphere tags and custom attributes to the migrated VM as metadata. Applies to all VMs in this plan."
            : 'Source tags and attributes will not be copied.'}
        </Typography>
      </Box>

      {preserveSourceTags && vms && (
        <Accordion disableGutters variant="outlined" data-testid="source-tags-preview">
          <AccordionSummary expandIcon={<ExpandMoreIcon />}>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                width: '100%',
                pr: 1
              }}
            >
              <Typography variant="body2" fontWeight={600}>
                Preview source tags &amp; attributes
              </Typography>
              <Chip
                size="small"
                color="primary"
                variant="outlined"
                label={`${vmCount} VM${vmCount === 1 ? '' : 's'} · ${entryCount} entr${entryCount === 1 ? 'y' : 'ies'}`}
              />
            </Box>
          </AccordionSummary>
          <AccordionDetails sx={{ p: 0, overflowX: 'auto' }}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Source VM</TableCell>
                  <TableCell>vSphere tags (category: tag)</TableCell>
                  <TableCell>Custom attributes</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {vms.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={3}>
                      <Typography variant="caption" color="text.secondary">
                        No VMs selected yet.
                      </Typography>
                    </TableCell>
                  </TableRow>
                ) : (
                  vms.map((vm) => (
                    <TableRow key={vm.id}>
                      <TableCell sx={{ whiteSpace: 'nowrap', fontWeight: 600 }}>
                        {vm.name}
                        {countVmSourceEntries(vm) === 0 && (
                          <Typography
                            variant="caption"
                            color="text.disabled"
                            sx={{ display: 'block', fontWeight: 400 }}
                          >
                            Nothing to copy
                          </Typography>
                        )}
                      </TableCell>
                      <TableCell>{renderEntryChips(vm.tags)}</TableCell>
                      <TableCell>{renderEntryChips(vm.customAttributes)}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </AccordionDetails>
        </Accordion>
      )}

      <Box sx={{ mt: 1 }}>
        <Typography variant="body2" fontWeight={600}>
          Custom Metadata{' '}
          <Typography component="span" variant="caption" color="text.secondary">
            (optional)
          </Typography>
        </Typography>
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1.5 }}>
          Extra key–value pairs added to every migrated VM in this plan. A custom key overrides a
          preserved source key with the same name. Keys and values are limited to{' '}
          {METADATA_MAX_LENGTH} characters.
        </Typography>

        {customMetadata.map((row, index) => (
          <Box
            key={index}
            sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 1.5 }}
            data-testid={`custom-metadata-row-${index}`}
          >
            <TextField
              size="small"
              placeholder="Key"
              value={row.key}
              onChange={(e) => updateRow(index, 'key', e.target.value)}
              slotProps={{ htmlInput: { maxLength: METADATA_MAX_LENGTH } }}
              sx={{ flex: 1 }}
            />
            <Typography color="text.secondary">–</Typography>
            <TextField
              size="small"
              placeholder="Value"
              value={row.value}
              onChange={(e) => updateRow(index, 'value', e.target.value)}
              slotProps={{ htmlInput: { maxLength: METADATA_MAX_LENGTH } }}
              sx={{ flex: 1 }}
            />
            <IconButton
              size="small"
              color="primary"
              aria-label="Remove metadata row"
              onClick={() => removeRow(index)}
            >
              <RemoveCircleIcon />
            </IconButton>
          </Box>
        ))}

        <Button
          startIcon={<AddCircleIcon />}
          onClick={addRow}
          size="small"
          data-testid="add-custom-metadata"
        >
          Add Metadata
        </Button>
      </Box>
    </Box>
  )
}
