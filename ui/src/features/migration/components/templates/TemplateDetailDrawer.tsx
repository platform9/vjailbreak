import { useState, type ReactNode } from 'react'
import { Box, Chip, IconButton, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import DnsOutlinedIcon from '@mui/icons-material/DnsOutlined'
import LanOutlinedIcon from '@mui/icons-material/LanOutlined'
import TuneOutlinedIcon from '@mui/icons-material/TuneOutlined'
import { ActionButton, DrawerShell, KeyValueGrid, SurfaceCard } from 'src/components'
import type { SavedTemplate } from '../../api/migration-blueprints/types'
import { useCloneTemplate, useDeleteTemplate } from '../../hooks/useTemplateLifecycle'
import { useTemplateTenantLookup } from '../../hooks/useTemplateTenantLookup'
import {
  cutoverOptionLabel,
  dataCopyMethodChipSx,
  DATA_COPY_METHOD_LABEL,
  deriveAdvancedOptionsSummary,
  guestOsLabel,
  storageCopyMethodLabel
} from '../../utils/templateLabels'
import DeleteTemplateDialog from './DeleteTemplateDialog'
import TemplateTypeAvatar from './TemplateTypeAvatar'

export interface TemplateDetailDrawerProps {
  open: boolean
  template: SavedTemplate | null
  onClose: () => void
  onUse: (template: SavedTemplate) => void
}

function SectionBlock({ icon, title, children }: { icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75, mb: 1 }}>
        {icon}
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ fontWeight: 700, letterSpacing: 0.6, textTransform: 'uppercase' }}
        >
          {title}
        </Typography>
      </Box>
      <Box sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>{children}</Box>
    </Box>
  )
}

function DetailRow({
  label,
  value,
  isLast = false
}: {
  label: string
  value: ReactNode
  isLast?: boolean
}) {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 2,
        px: 2,
        py: 1.25,
        borderBottom: isLast ? 'none' : '1px solid',
        borderColor: 'divider'
      }}
    >
      <Typography variant="body2" color="text.secondary" sx={{ flexShrink: 0 }}>
        {label}
      </Typography>
      {typeof value === 'string' ? (
        <Typography
          variant="body2"
          sx={{ fontWeight: 600, textAlign: 'right', wordBreak: 'break-word' }}
        >
          {value}
        </Typography>
      ) : (
        value
      )}
    </Box>
  )
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
  const tenantByDestination = useTemplateTenantLookup()

  if (!template) return null

  const tenantProject = tenantByDestination[template.destination] || ''

  const infoItems = [{ label: 'Created', value: new Date(template.createdAt).toLocaleDateString() }]

  const mappings = [
    ...template.networkMappings.map((m) => ({ ...m, kind: 'Network' })),
    ...template.storageMappings.map((m) => ({ ...m, kind: 'Storage' }))
  ]

  const copyMethodLabel = storageCopyMethodLabel(template.spec.storageCopyMethod)

  const handleClone = async () => {
    await cloneMutation.mutateAsync(template)
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
        width={460}
        requireCloseConfirmation={false}
        data-testid="template-detail-drawer"
        header={
          <Box
            sx={{
              display: 'flex',
              alignItems: 'flex-start',
              justifyContent: 'space-between',
              gap: 2,
              px: 3,
              py: 2.5,
              borderBottom: '1px solid',
              borderColor: 'divider'
            }}
          >
            <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5, minWidth: 0 }}>
              <TemplateTypeAvatar dataCopyMethod={template.dataCopyMethod} />
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 700 }}>
                  {template.displayName}
                </Typography>
                {template.description && (
                  <Typography variant="body2" color="text.secondary">
                    {template.description}
                  </Typography>
                )}
              </Box>
            </Box>
            <IconButton
              aria-label="Close drawer"
              onClick={onClose}
              size="small"
              data-testid="template-detail-drawer-close"
            >
              <CloseIcon fontSize="small" />
            </IconButton>
          </Box>
        }
        footer={
          <Box
            sx={{
              borderTop: '1px solid',
              borderColor: 'divider',
              px: 3,
              py: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 1.5
            }}
          >
            <ActionButton
              tone="danger"
              variant="text"
              startIcon={<DeleteOutlineIcon />}
              onClick={() => setConfirmDeleteOpen(true)}
              data-testid="template-delete-button"
            >
              Delete
            </ActionButton>
            <Box sx={{ display: 'flex', gap: 1.5 }}>
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
            </Box>
          </Box>
        }
      >
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2.5 }}>
          <SurfaceCard variant="card">
            <KeyValueGrid items={infoItems} mdGrids={1} />
          </SurfaceCard>

          <SectionBlock icon={<DnsOutlinedIcon fontSize="small" color="action" />} title="Source & destination">
            <DetailRow label="Source vCenter" value={template.sourceVCenter} />
            <DetailRow label="Destination" value={template.destination} />
            <DetailRow label="Tenant / project" value={tenantProject || '—'} />
            <DetailRow label="Target cluster" value={template.targetCluster} isLast />
          </SectionBlock>

          <SectionBlock
            icon={<LanOutlinedIcon fontSize="small" color="action" />}
            title="Network & storage mappings"
          >
            {mappings.length === 0 && !copyMethodLabel ? (
              <Box sx={{ px: 2, py: 1.25 }}>
                <Typography variant="body2" color="text.secondary">
                  No mappings saved with this template.
                </Typography>
              </Box>
            ) : (
              <>
                {mappings.map((mapping, index) => (
                  <DetailRow
                    key={`${mapping.kind}-${mapping.source}-${index}`}
                    label={mapping.kind}
                    value={`${mapping.source} → ${mapping.target}`}
                    isLast={index === mappings.length - 1 && !copyMethodLabel}
                  />
                ))}
                {copyMethodLabel && (
                  <DetailRow label="Copy method" value={copyMethodLabel} isLast />
                )}
              </>
            )}
          </SectionBlock>

          <SectionBlock
            icon={<TuneOutlinedIcon fontSize="small" color="action" />}
            title="Migration options"
          >
            <DetailRow
              label="Copy mode"
              value={
                <Chip
                  size="small"
                  label={DATA_COPY_METHOD_LABEL[template.dataCopyMethod]}
                  sx={dataCopyMethodChipSx(template.dataCopyMethod)}
                />
              }
            />
            <DetailRow label="Cutover" value={cutoverOptionLabel(template.cutoverOption)} />
            <DetailRow label="Guest OS" value={guestOsLabel(template.osFamily)} />
            <DetailRow label="Advanced" value={deriveAdvancedOptionsSummary(template.spec)} isLast />
          </SectionBlock>
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
