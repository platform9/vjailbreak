import { useState } from 'react'
import { Box, Typography } from '@mui/material'
import CloudSyncIcon from '@mui/icons-material/CloudSync'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import { formatDistanceToNowStrict } from 'date-fns'
import {
  ActionButton,
  DrawerFooter,
  DrawerHeader,
  DrawerShell,
  KeyValueGrid,
  SurfaceCard
} from 'src/components'
import type { SavedTemplate } from '../../mock-templates/types'
import { useCloneTemplate, useDeleteTemplate } from '../../hooks/useTemplateLifecycle'
import DeleteTemplateDialog from './DeleteTemplateDialog'

export interface TemplateDetailDrawerProps {
  open: boolean
  template: SavedTemplate | null
  onClose: () => void
  onUse: (template: SavedTemplate) => void
}

export default function TemplateDetailDrawer({
  open,
  template,
  onClose,
  onUse
}: TemplateDetailDrawerProps) {
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false)
  const deleteMutation = useDeleteTemplate()
  const cloneMutation = useCloneTemplate()

  if (!template) return null

  const infoItems = [
    {
      label: 'Times used',
      value: `${template.timesUsed} migration${template.timesUsed === 1 ? '' : 's'}`
    },
    {
      label: 'Last used',
      value: template.lastUsedAt
        ? formatDistanceToNowStrict(new Date(template.lastUsedAt), { addSuffix: true })
        : 'Never'
    },
    { label: 'Created', value: new Date(template.createdAt).toLocaleDateString() }
  ]

  const sourceDestinationItems = [
    { label: 'Source vCenter', value: template.sourceVCenter },
    { label: 'Destination', value: template.destination },
    { label: 'Tenant / project', value: template.tenantProject },
    { label: 'Target cluster', value: template.targetCluster }
  ]

  const mappings = [
    ...template.networkMappings.map((m) => ({ ...m, kind: 'Network' })),
    ...template.storageMappings.map((m) => ({ ...m, kind: 'Storage' }))
  ]

  const handleClone = async () => {
    await cloneMutation.mutateAsync(template.name)
    onClose()
  }

  const handleDeleteConfirmed = async () => {
    await deleteMutation.mutateAsync(template.name)
    setConfirmDeleteOpen(false)
    onClose()
  }

  return (
    <>
      <DrawerShell
        open={open}
        onClose={onClose}
        width={640}
        requireCloseConfirmation={false}
        data-testid="template-detail-drawer"
        header={
          <DrawerHeader
            icon={<CloudSyncIcon color="primary" />}
            title={template.displayName}
            subtitle={template.description}
            onClose={onClose}
          />
        }
        footer={
          <DrawerFooter data-testid="template-detail-footer">
            <ActionButton
              tone="danger"
              variant="outlined"
              startIcon={<DeleteOutlineIcon />}
              onClick={() => setConfirmDeleteOpen(true)}
              sx={{ mr: 'auto' }}
              data-testid="template-delete-button"
            >
              Delete
            </ActionButton>
            <ActionButton
              tone="secondary"
              startIcon={<ContentCopyIcon />}
              onClick={handleClone}
              loading={cloneMutation.isPending}
              data-testid="template-clone-button"
            >
              Clone
            </ActionButton>
            <ActionButton
              tone="primary"
              onClick={() => onUse(template)}
              data-testid="template-use-button"
            >
              Use template
            </ActionButton>
          </DrawerFooter>
        }
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
          <SurfaceCard variant="card">
            <KeyValueGrid items={infoItems} mdGrids={2} />
          </SurfaceCard>

          <SurfaceCard variant="card" title="Source & destination">
            <KeyValueGrid items={sourceDestinationItems} mdGrids={1} />
          </SurfaceCard>

          <SurfaceCard variant="card" title="Network & storage mappings">
            {mappings.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No mappings saved with this template.
              </Typography>
            ) : (
              <KeyValueGrid
                items={mappings.map((mapping) => ({
                  label: mapping.kind,
                  value: `${mapping.source} → ${mapping.target}`
                }))}
                mdGrids={1}
              />
            )}
          </SurfaceCard>
        </Box>
      </DrawerShell>

      <DeleteTemplateDialog
        open={confirmDeleteOpen}
        template={template}
        onClose={() => setConfirmDeleteOpen(false)}
        onConfirm={handleDeleteConfirmed}
        isDeleting={deleteMutation.isPending}
      />
    </>
  )
}
