import { useEffect, useState } from 'react'
import {
  Alert,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  TextField
} from '@mui/material'
import { ActionButton } from 'src/components'
import { useSaveAsTemplate } from '../../hooks/useSaveAsTemplate'
import { useUpdateTemplate } from '../../hooks/useTemplateLifecycle'
import type { SavedTemplate, SaveAsTemplateInput } from '../../api/migration-blueprints/types'

export interface SaveAsTemplateDialogProps {
  open: boolean
  onClose: () => void
  onSaved?: () => void
  // Everything except displayName/description is derived from the current
  // New Migration form state by the caller (MigrationForm.tsx).
  buildTemplateInput: (fields: {
    displayName: string
    description?: string
  }) => SaveAsTemplateInput
  // When set, the dialog updates this existing blueprint in place (Edit Template)
  // instead of creating a new one — name/description prefill from it.
  editingTemplate?: SavedTemplate
}

export default function SaveAsTemplateDialog({
  open,
  onClose,
  onSaved,
  buildTemplateInput,
  editingTemplate
}: SaveAsTemplateDialogProps) {
  const [displayName, setDisplayName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState<string | null>(null)
  const saveMutation = useSaveAsTemplate()
  const updateMutation = useUpdateTemplate()
  const isEditing = Boolean(editingTemplate)
  const isPending = saveMutation.isPending || updateMutation.isPending

  useEffect(() => {
    if (!open) return
    setDisplayName(editingTemplate?.displayName || '')
    setDescription(editingTemplate?.description || '')
    setError(null)
  }, [open, editingTemplate])

  const handleClose = () => {
    if (isPending) return
    setError(null)
    saveMutation.reset()
    updateMutation.reset()
    onClose()
  }

  const handleSave = async () => {
    if (!displayName.trim()) {
      setError('Template name is required.')
      return
    }
    setError(null)

    try {
      const input = buildTemplateInput({
        displayName: displayName.trim(),
        description: description.trim() || undefined
      })
      if (editingTemplate) {
        await updateMutation.mutateAsync({ name: editingTemplate.name, input })
      } else {
        await saveMutation.mutateAsync(input)
      }
      onSaved?.()
      handleClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Error saving template.')
    }
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{isEditing ? 'Save changes' : 'Save as template'}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        <DialogContentText>
          {isEditing
            ? 'Update this template’s name, description, and configuration.'
            : 'Save the current source, destination, mappings, and migration options as a reusable template. VM selection is not saved — you’ll choose VMs fresh each time you use this template.'}
        </DialogContentText>
        <TextField
          label="Template name"
          value={displayName}
          onChange={(event) => setDisplayName(event.target.value)}
          autoFocus
          fullWidth
          required
          data-testid="save-template-name"
        />
        <TextField
          label="Description"
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          fullWidth
          multiline
          minRows={2}
          placeholder="Optional — what is this template for?"
          data-testid="save-template-description"
        />
        {error && <Alert severity="error">{error}</Alert>}
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 3 }}>
        <ActionButton tone="secondary" onClick={handleClose} disabled={isPending}>
          Cancel
        </ActionButton>
        <ActionButton
          tone="primary"
          onClick={handleSave}
          loading={isPending}
          data-testid="save-template-confirm"
        >
          {isEditing ? 'Save changes' : 'Save template'}
        </ActionButton>
      </DialogActions>
    </Dialog>
  )
}
