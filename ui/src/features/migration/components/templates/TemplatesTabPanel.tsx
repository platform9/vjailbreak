import { useMemo, useState } from 'react'
import { Box, CircularProgress, Grid, Typography } from '@mui/material'
import CloudSyncIcon from '@mui/icons-material/CloudSync'
import { useMigrationTemplatesQuery } from '../../hooks/useMigrationTemplatesQuery'
import {
  filterTemplates,
  sortTemplates,
  type TemplateCopyMethodFilter,
  type TemplateSortKey
} from '../../utils/templateFilters'
import type { SavedTemplate } from '../../api/migration-blueprints/types'
import TemplateCard from './TemplateCard'
import TemplatesTable from './TemplatesTable'
import TemplateDetailDrawer from './TemplateDetailDrawer'

export interface TemplatesTabPanelProps {
  onUseTemplate: (template: SavedTemplate) => void
  onEditTemplate: (template: SavedTemplate) => void
  // Search/filter/sort/view controls live inline with the page's tabs (see
  // MigrationsPage + TemplatesToolbar) rather than in this panel.
  query: string
  copyMethodFilter: TemplateCopyMethodFilter
  sortKey: TemplateSortKey
  view: 'grid' | 'list'
}

export default function TemplatesTabPanel({
  onUseTemplate,
  onEditTemplate,
  query,
  copyMethodFilter,
  sortKey,
  view
}: TemplatesTabPanelProps) {
  const { data: templates = [], isLoading } = useMigrationTemplatesQuery()
  const [selectedTemplate, setSelectedTemplate] = useState<SavedTemplate | null>(null)

  const visibleTemplates = useMemo(
    () => sortTemplates(filterTemplates(templates, query, copyMethodFilter), sortKey),
    [templates, query, copyMethodFilter, sortKey]
  )

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
        <CircularProgress size={28} />
      </Box>
    )
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      {templates.length === 0 ? (
        <Box sx={{ textAlign: 'center', py: 8 }}>
          <CloudSyncIcon sx={{ fontSize: 40, color: 'text.secondary', mb: 1 }} />
          <Typography variant="h6" component="p">
            No templates yet
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Configure a migration, then use &ldquo;Save as template&rdquo; in the New Migration
            drawer to create your first one.
          </Typography>
        </Box>
      ) : visibleTemplates.length === 0 ? (
        <Box sx={{ textAlign: 'center', py: 8 }}>
          <Typography variant="h6" component="p">
            No templates match
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Try a different search term or filter.
          </Typography>
        </Box>
      ) : view === 'list' ? (
        <TemplatesTable
          templates={visibleTemplates}
          onOpenDetail={setSelectedTemplate}
          onUse={onUseTemplate}
          onEdit={onEditTemplate}
        />
      ) : (
        <Grid container spacing={2}>
          {visibleTemplates.map((template) => (
            <Grid item xs={12} sm={6} md={4} key={template.name}>
              <TemplateCard
                template={template}
                onOpenDetail={setSelectedTemplate}
                onUse={onUseTemplate}
                onEdit={onEditTemplate}
              />
            </Grid>
          ))}
        </Grid>
      )}

      <TemplateDetailDrawer
        open={Boolean(selectedTemplate)}
        template={selectedTemplate}
        onClose={() => setSelectedTemplate(null)}
        onUse={onUseTemplate}
        onEdit={onEditTemplate}
      />
    </Box>
  )
}
