import { useState } from 'react'
import {
  Box,
  Chip,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography
} from '@mui/material'
import CloudUploadOutlinedIcon from '@mui/icons-material/CloudUploadOutlined'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import EditOutlinedIcon from '@mui/icons-material/EditOutlined'
import ArrowRightAltIcon from '@mui/icons-material/ArrowRightAlt'
import type { SavedTemplate } from '../../api/migration-blueprints/types'
import { dataCopyMethodChipSx, DATA_COPY_METHOD_LABEL } from '../../utils/templateLabels'
import { useCloneTemplate, useDeleteTemplate } from '../../hooks/useTemplateLifecycle'
import { useTemplateTenantLookup } from '../../hooks/useTemplateTenantLookup'
import DeleteTemplateDialog from './DeleteTemplateDialog'
import TemplateTypeAvatar from './TemplateTypeAvatar'

export interface TemplatesTableProps {
  templates: SavedTemplate[]
  onOpenDetail: (template: SavedTemplate) => void
  onUse: (template: SavedTemplate) => void
  onEdit: (template: SavedTemplate) => void
}

export default function TemplatesTable({
  templates,
  onOpenDetail,
  onUse,
  onEdit
}: TemplatesTableProps) {
  const [deleteTarget, setDeleteTarget] = useState<SavedTemplate | null>(null)
  const cloneMutation = useCloneTemplate()
  const deleteMutation = useDeleteTemplate()
  const tenantByDestination = useTemplateTenantLookup()

  const handleDeleteConfirmed = async () => {
    if (!deleteTarget) return
    await deleteMutation.mutateAsync(deleteTarget.name)
    setDeleteTarget(null)
  }

  return (
    <>
      <TableContainer sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
        <Table size="small" data-testid="templates-table">
          <TableHead>
            <TableRow>
              <TableCell>Template</TableCell>
              <TableCell>Source → Destination</TableCell>
              <TableCell>Copy</TableCell>
              <TableCell>Mappings</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {templates.map((template) => {
              const mappingCount = template.networkMappings.length + template.storageMappings.length
              const tenantProject = tenantByDestination[template.destination]
              const subtitleLine = [tenantProject, template.targetCluster].filter(Boolean).join(' · ')
              return (
                <TableRow
                  key={template.name}
                  hover
                  sx={{ cursor: 'pointer' }}
                  onClick={() => onOpenDetail(template)}
                  data-testid={`templates-table-row-${template.name}`}
                >
                  <TableCell sx={{ maxWidth: 320 }}>
                    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.25, minWidth: 0 }}>
                      <TemplateTypeAvatar dataCopyMethod={template.dataCopyMethod} size={30} />
                      <Box sx={{ minWidth: 0 }}>
                        <Typography variant="body2" sx={{ fontWeight: 600 }} noWrap>
                          {template.displayName}
                        </Typography>
                        {template.description && (
                          <Typography variant="caption" color="text.secondary" noWrap component="div">
                            {template.description}
                          </Typography>
                        )}
                      </Box>
                    </Box>
                  </TableCell>
                  <TableCell sx={{ maxWidth: 260 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, minWidth: 0 }}>
                      <Typography variant="body2" noWrap>
                        {template.sourceVCenter}
                      </Typography>
                      <ArrowRightAltIcon fontSize="small" sx={{ color: 'text.secondary' }} />
                      <Typography variant="body2" noWrap>
                        {template.destination}
                      </Typography>
                    </Box>
                    {subtitleLine && (
                      <Typography variant="caption" color="text.secondary" noWrap component="div">
                        {subtitleLine}
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      label={DATA_COPY_METHOD_LABEL[template.dataCopyMethod]}
                      sx={dataCopyMethodChipSx(template.dataCopyMethod)}
                    />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">
                      {mappingCount} mapping{mappingCount === 1 ? '' : 's'}
                    </Typography>
                  </TableCell>
                  <TableCell align="right" onClick={(event) => event.stopPropagation()}>
                    <Tooltip title="Use template">
                      <IconButton
                        size="small"
                        onClick={() => onUse(template)}
                        data-testid={`templates-table-use-${template.name}`}
                      >
                        <CloudUploadOutlinedIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Edit template">
                      <IconButton
                        size="small"
                        onClick={() => onEdit(template)}
                        data-testid={`templates-table-edit-${template.name}`}
                      >
                        <EditOutlinedIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Clone template">
                      <IconButton
                        size="small"
                        onClick={() => cloneMutation.mutate(template)}
                        disabled={cloneMutation.isPending}
                        data-testid={`templates-table-clone-${template.name}`}
                      >
                        <ContentCopyIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Delete template">
                      <IconButton
                        size="small"
                        onClick={() => setDeleteTarget(template)}
                        data-testid={`templates-table-delete-${template.name}`}
                      >
                        <DeleteOutlineIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </TableContainer>

      <DeleteTemplateDialog
        open={Boolean(deleteTarget)}
        template={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={handleDeleteConfirmed}
        isDeleting={deleteMutation.isPending}
      />
    </>
  )
}
