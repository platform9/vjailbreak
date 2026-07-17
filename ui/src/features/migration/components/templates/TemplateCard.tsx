import { Avatar, Box, Chip, Stack, Typography } from '@mui/material'
import CloudSyncIcon from '@mui/icons-material/CloudSync'
import ArrowRightAltIcon from '@mui/icons-material/ArrowRightAlt'
import { formatDistanceToNowStrict } from 'date-fns'
import { ActionButton, SurfaceCard } from 'src/components'
import type { SavedTemplate } from '../../mock-templates/types'
import { cutoverOptionLabel, DATA_COPY_METHOD_LABEL } from '../../utils/templateLabels'

export interface TemplateCardProps {
  template: SavedTemplate
  onOpenDetail: (template: SavedTemplate) => void
  onUse: (template: SavedTemplate) => void
}

export default function TemplateCard({ template, onOpenDetail, onUse }: TemplateCardProps) {
  const mappingCount = template.networkMappings.length + template.storageMappings.length

  return (
    <SurfaceCard
      variant="card"
      data-testid={`template-card-${template.name}`}
      sx={{ cursor: 'pointer', height: '100%' }}
      onClick={() => onOpenDetail(template)}
    >
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1.5 }}>
        <Avatar
          variant="rounded"
          sx={{
            bgcolor: 'primary.light',
            color: 'primary.dark',
            width: 36,
            height: 36
          }}
        >
          <CloudSyncIcon fontSize="small" />
        </Avatar>
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
        <Typography variant="caption" color="text.secondary" noWrap component="div">
          {template.tenantProject} · {template.targetCluster}
        </Typography>
      </Box>

      <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap">
        <Chip size="small" label={DATA_COPY_METHOD_LABEL[template.dataCopyMethod]} />
        <Chip size="small" label={cutoverOptionLabel(template.cutoverOption)} />
        <Chip size="small" label={`${mappingCount} mapping${mappingCount === 1 ? '' : 's'}`} />
      </Stack>

      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
        <Typography variant="caption" color="text.secondary" noWrap component="div">
          {template.lastUsedAt
            ? `${formatDistanceToNowStrict(new Date(template.lastUsedAt), { addSuffix: true })}`
            : 'Never used'}{' '}
          · {template.timesUsed} run{template.timesUsed === 1 ? '' : 's'}
        </Typography>
        <ActionButton
          tone="primary"
          size="small"
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
  )
}
