import { useState } from 'react'
import { Box, Chip, IconButton, Stack, Tooltip, Typography } from '@mui/material'
import ArrowRightAltIcon from '@mui/icons-material/ArrowRightAlt'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import EditOutlinedIcon from '@mui/icons-material/EditOutlined'
import { ActionButton, SurfaceCard } from 'src/components'
import type { SavedTemplate } from '../../api/migration-blueprints/types'
import {
  cutoverOptionLabel,
  dataCopyMethodChipSx,
  DATA_COPY_METHOD_LABEL
} from '../../utils/templateLabels'
import { useCloneTemplate, useDeleteTemplate } from '../../hooks/useTemplateLifecycle'
import { useTemplateTenantLookup } from '../../hooks/useTemplateTenantLookup'
import DeleteTemplateDialog from './DeleteTemplateDialog'
import TemplateTypeAvatar from './TemplateTypeAvatar'

export interface TemplateCardProps {
  template: SavedTemplate
  onOpenDetail: (template: SavedTemplate) => void
  onUse: (template: SavedTemplate) => void
  onEdit: (template: SavedTemplate) => void
}

export default function TemplateCard({ template, onOpenDetail, onUse, onEdit }: TemplateCardProps) {
  const mappingCount = template.networkMappings.length + template.storageMappings.length
  const [confirmDeleteOpen, setConfirmDeleteOpen] = useState(false)
  const cloneMutation = useCloneTemplate()
  const deleteMutation = useDeleteTemplate()
  const tenantByDestination = useTemplateTenantLookup()
  const tenantProject = tenantByDestination[template.destination]
  const subtitleLine = [tenantProject, template.targetCluster].filter(Boolean).join(' · ')

  const handleDeleteConfirmed = async () => {
    await deleteMutation.mutateAsync(template.name)
    setConfirmDeleteOpen(false)
  }

  return (
    <>
      <SurfaceCard
        variant="card"
        data-testid={`template-card-${template.name}`}
        sx={{
          cursor: 'pointer',
          height: '100%',
          '&:hover .template-card-hover-actions': { opacity: 1 }
        }}
        onClick={() => onOpenDetail(template)}
      >
        <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
          <TemplateTypeAvatar dataCopyMethod={template.dataCopyMethod} />
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
              <Typography variant="subtitle1" component="h3" sx={{ fontWeight: 600 }}>
                {template.displayName}
              </Typography>
            </Box>
            {template.description && (
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{
                  display: '-webkit-box',
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: 'vertical',
                  overflow: 'hidden'
                }}
              >
                {template.description}
              </Typography>
            )}
          </Box>
          <Box
            className="template-card-hover-actions"
            sx={{
              display: 'flex',
              gap: 0.5,
              opacity: 0,
              transition: 'opacity 0.1s ease',
              flexShrink: 0
            }}
          >
            <Tooltip title="Edit template">
              <IconButton
                size="small"
                onClick={(event) => {
                  event.stopPropagation()
                  onEdit(template)
                }}
                data-testid={`template-card-edit-${template.name}`}
              >
                <EditOutlinedIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="Clone template">
              <IconButton
                size="small"
                onClick={(event) => {
                  event.stopPropagation()
                  cloneMutation.mutate(template)
                }}
                disabled={cloneMutation.isPending}
                data-testid={`template-card-clone-${template.name}`}
              >
                <ContentCopyIcon fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title="Delete template">
              <IconButton
                size="small"
                onClick={(event) => {
                  event.stopPropagation()
                  setConfirmDeleteOpen(true)
                }}
                data-testid={`template-card-delete-${template.name}`}
              >
                <DeleteOutlineIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Box>
        </Box>

        <Box
          sx={{
            bgcolor: 'background.default',
            borderRadius: 1,
            px: 1.5,
            py: 1
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, minWidth: 0 }}>
            <Typography variant="body2" noWrap sx={{ fontWeight: 500 }}>
              {template.sourceVCenter}
            </Typography>
            <ArrowRightAltIcon fontSize="small" sx={{ color: 'text.secondary' }} />
            <Typography variant="body2" noWrap sx={{ fontWeight: 500 }}>
              {template.destination}
            </Typography>
          </Box>
          {subtitleLine && (
            <Typography variant="caption" color="text.secondary" noWrap component="div">
              {subtitleLine}
            </Typography>
          )}
        </Box>

        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 1.5,
            mt: 'auto'
          }}
        >
          <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap" sx={{ minWidth: 0 }}>
            <Chip
              size="small"
              label={DATA_COPY_METHOD_LABEL[template.dataCopyMethod]}
              sx={dataCopyMethodChipSx(template.dataCopyMethod)}
            />
            <Chip size="small" label={cutoverOptionLabel(template.cutoverOption)} />
            <Chip size="small" label={`${mappingCount} mapping${mappingCount === 1 ? '' : 's'}`} />
          </Stack>
          <ActionButton
            tone="primary"
            size="small"
            sx={{ flexShrink: 0 }}
            data-testid={`template-use-${template.name}`}
            onClick={(event) => {
              event.stopPropagation()
              onUse(template)
            }}
          >
            Use
          </ActionButton>
        </Box>
      </SurfaceCard>
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
