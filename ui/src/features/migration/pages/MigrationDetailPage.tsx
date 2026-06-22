import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Tab,
  Tabs,
  Typography,
} from '@mui/material'
import { useMigrationDetailQuery } from '../hooks/useMigrationDetailQuery'
import { useMigrationDetailResourcesQuery } from 'src/hooks/api/useMigrationDetailResourcesQuery'
import { isMigrationFailed } from '../utils/phaseUtils'
import {
  MigrationDetailHeader,
  MigrationKpiStrip,
  MigrationNextActionBanner,
  MigrationPhaseStepper,
  MigrationPhaseDetail,
  MigrationErrorCard,
  MigrationActivityTimeline,
  MigrationSpecCard,
  MigrationDetailDebugLogs,
} from '../components/detail'

type TabId = 'overview' | 'logs'

export default function MigrationDetailPage() {
  const { migrationName } = useParams<{ migrationName: string }>()
  const navigate = useNavigate()
  const [tab, setTab] = useState<TabId>('overview')

  const {
    data: migration,
    isLoading,
    error,
    refetch,
  } = useMigrationDetailQuery(migrationName ?? '')

  const { data: resources } = useMigrationDetailResourcesQuery({
    open: true,
    migration: migration ?? null,
  })

  if (!migrationName) {
    return <Alert severity="error">No migration name provided.</Alert>
  }

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, p: 4 }}>
        <CircularProgress size={24} />
        <Typography variant="body2" color="text.secondary">
          Loading migration…
        </Typography>
      </Box>
    )
  }

  if (error || !migration) {
    return (
      <Box sx={{ p: 4 }}>
        <Alert severity="error">Failed to load migration "{migrationName}".</Alert>
        <Button onClick={() => navigate('/dashboard/migrations')} sx={{ mt: 2 }}>
          Back to Migrations
        </Button>
      </Box>
    )
  }

  const failed = isMigrationFailed(migration)

  return (
    <Box sx={{ maxWidth: '100%', px: 3, py: 3 }}>
      {/* Header: breadcrumb, title, action buttons */}
      <MigrationDetailHeader
        migration={migration}
        onBack={() => navigate('/dashboard/migrations')}
        onCutoverSuccess={() => refetch()}
        resources={resources}
      />

      {/* KPI strip */}
      <MigrationKpiStrip migration={migration} resources={resources} />

      {/* Contextual banner */}
      <MigrationNextActionBanner migration={migration} />

      {/* Tabs */}
      <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
        <Tabs value={tab} onChange={(_, v) => setTab(v as TabId)}>
          <Tab label="Overview" value="overview" />
          <Tab label="Pod logs" value="logs" />
          <Tab label="Events" disabled sx={{ opacity: 0.4 }} />
          <Tab label="Resources" disabled sx={{ opacity: 0.4 }} />
        </Tabs>
      </Box>

      {/* Overview tab */}
      {tab === 'overview' && (
        <Box>
          {/* Phase stepper */}
          <MigrationPhaseStepper migration={migration} />

          {/* Error card (failed) or phase detail (active/done) */}
          {failed
            ? <MigrationErrorCard migration={migration} />
            : <MigrationPhaseDetail migration={migration} onCutoverSuccess={() => refetch()} />
          }

          {/* Two-column meta row */}
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
              gap: 2,
              mt: 2,
            }}
          >
            <MigrationActivityTimeline migration={migration} />
            <MigrationSpecCard migration={migration} resources={resources} />
          </Box>
        </Box>
      )}

      {/* Logs tab */}
      {tab === 'logs' && (
        <MigrationDetailDebugLogs migration={migration} />
      )}
    </Box>
  )
}
