import { useState, useEffect, useMemo } from 'react'
import { Box, Tab, Tabs, Typography, Button } from '@mui/material'
import Snackbar from '@mui/material/Snackbar'
import Alert from '@mui/material/Alert'
import Tooltip from '@mui/material/Tooltip'
import RefreshRoundedIcon from '@mui/icons-material/RefreshRounded'
import AddIcon from '@mui/icons-material/Add'
import { FIVE_SECONDS, THIRTY_SECONDS } from 'src/constants'
import { useMigrationsQuery } from '../hooks/useMigrationsQuery'
import { Migration } from '../api/migrations'
import MigrationsTable from '../components/MigrationsTable'
import MigrationsSummaryStats from '../components/MigrationsSummaryStats'
import DeleteMigrationDialog from '../components/DeleteMigrationDialog'
import { useMigrationStatusMonitor } from '../hooks/useMigrationStatusMonitor'
import { useMigrationTemplatesQuery } from '../hooks/useMigrationTemplatesQuery'
import { useMigrationFormActions } from '../context/MigrationFormContext'
import TemplatesTabPanel from '../components/templates/TemplatesTabPanel'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import { getMigrationStatusCategory } from '../utils/migrationTableUtils'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'

type MigrationsPageTab = 'migrations' | 'templates'

export default function MigrationsPage() {
  const [activeTab, setActiveTab] = useState<MigrationsPageTab>('migrations')
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedMigrations, setSelectedMigrations] = useState<Migration[]>([])
  const [openSnackbar, setOpenSnackbar] = useState(false)
  const [statusFilter, setStatusFilter] = useState('All')
  const { data: templates = [] } = useMigrationTemplatesQuery()
  const { openMigrationForm } = useMigrationFormActions()

  const { data: vmwareCreds } = useVmwareCredentialsQuery(undefined, {
    staleTime: 0,
    refetchOnMount: true
  })
  const { data: openstackCreds } = useOpenstackCredentialsQuery(undefined, {
    staleTime: 0,
    refetchOnMount: true
  })

  const hasVmwareCredentials = (vmwareCreds || []).length > 0
  const hasPcdCredentials = useMemo(() => {
    const openstack = Array.isArray(openstackCreds) ? openstackCreds : []
    return openstack.some(
      (cred) => cred?.metadata?.labels?.['vjailbreak.k8s.pf9.io/is-pcd'] === 'true'
    )
  }, [openstackCreds])

  const startMigrationDisabled = !hasVmwareCredentials || !hasPcdCredentials
  const startMigrationDisabledReason = 'Add VMware and PCD credentials before starting a migration.'

  const handleUseTemplate = (template: SavedTemplate) => {
    openMigrationForm('standard', undefined, template)
  }

  const {
    data: migrations,
    refetch: refetchMigrations,
    isLoading: isMigrationsLoading
  } = useMigrationsQuery(undefined, {
    refetchInterval: (query) => {
      const migrations = query?.state?.data || []
      const hasPendingMigration = !!migrations.find((m) => m.status === undefined)
      return hasPendingMigration ? FIVE_SECONDS : THIRTY_SECONDS
    },
    staleTime: 0,
    refetchOnMount: true
  })

  useMigrationStatusMonitor(migrations)

  const summaryCounts = useMemo(() => {
    const counts = { inProgress: 0, awaitingAction: 0, pending: 0, succeeded: 0, failed: 0 }
    for (const migration of migrations || []) {
      counts[getMigrationStatusCategory(migration.status?.phase)]++
    }
    return counts
  }, [migrations])

  const activeAgentCount = useMemo(() => {
    const agents = new Set(
      (migrations || [])
        .filter((m) => getMigrationStatusCategory(m.status?.phase) === 'inProgress')
        .map((m) => m.status?.agentName)
        .filter(Boolean)
    )
    return agents.size
  }, [migrations])

  useEffect(() => {
    const showSuccess = sessionStorage.getItem('showUpgradeSuccess')
    if (showSuccess === 'true') {
      setOpenSnackbar(true)
      sessionStorage.removeItem('showUpgradeSuccess')
    }
  }, [])

  const handleDeleteClick = (migrationName: string) => {
    const migration = migrations?.find((m) => m.metadata.name === migrationName)
    if (migration) {
      setSelectedMigrations([migration])
      setDeleteDialogOpen(true)
    }
  }

  const handleDeleteSelected = (migrations: Migration[]) => {
    setSelectedMigrations(migrations)
    setDeleteDialogOpen(true)
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedMigrations([])
  }

  const handleCloseSnackbar = (_event?: React.SyntheticEvent | Event, reason?: string) => {
    if (reason === 'clickaway') return
    setOpenSnackbar(false)
  }

  const migrationsCount = migrations?.length || 0

  return (
    <>
      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 2,
          alignItems: { xs: 'flex-start', md: 'center' },
          justifyContent: 'space-between',
          mb: 2
        }}
      >
        <Box>
          <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
            <Typography variant="h4" component="h1" data-testid="migrations-page-title">
              {activeTab === 'migrations' ? 'Migrations' : 'Migration Templates'}
            </Typography>
            <Typography variant="h6" color="text.secondary">
              {activeTab === 'migrations' ? migrationsCount : templates.length}
            </Typography>
          </Box>
          {activeTab === 'migrations' ? (
            <Typography variant="body2" color="text.secondary">
              Lift VMware VMs into Private Cloud Director.{' '}
              {summaryCounts.inProgress > 0 && (
                <Box component="span" sx={{ fontWeight: 600, color: 'text.primary' }}>
                  {summaryCounts.inProgress} migrating
                </Box>
              )}{' '}
              {summaryCounts.inProgress > 0 && activeAgentCount > 0
                ? `right now across ${activeAgentCount} agent${activeAgentCount !== 1 ? 's' : ''}.`
                : null}
            </Typography>
          ) : (
            <Typography variant="body2" color="text.secondary">
              Save and reuse migration configurations across the team.
            </Typography>
          )}
        </Box>
        <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center' }}>
          {activeTab === 'migrations' ? (
            <>
              <Tooltip title="Refresh">
                <Button
                  variant="outlined"
                  startIcon={<RefreshRoundedIcon />}
                  onClick={() => refetchMigrations()}
                  data-testid="migrations-page-refresh-button"
                >
                  Refresh
                </Button>
              </Tooltip>
              <Tooltip title={startMigrationDisabled ? startMigrationDisabledReason : ''} arrow>
                <span>
                  <Button
                    variant="contained"
                    color="primary"
                    startIcon={<AddIcon />}
                    onClick={() => openMigrationForm('standard')}
                    disabled={startMigrationDisabled}
                    data-testid="start-migration-button"
                  >
                    Start Migration
                  </Button>
                </span>
              </Tooltip>
            </>
          ) : (
            <Tooltip title={startMigrationDisabled ? startMigrationDisabledReason : ''} arrow>
              <span>
                <Button
                  variant="contained"
                  color="primary"
                  startIcon={<AddIcon />}
                  onClick={() => openMigrationForm('standard')}
                  disabled={startMigrationDisabled}
                  data-testid="new-migration-button"
                >
                  New Migration
                </Button>
              </span>
            </Tooltip>
          )}
        </Box>
      </Box>

      <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
        <Tabs
          value={activeTab}
          onChange={(_event, value: MigrationsPageTab) => setActiveTab(value)}
          data-testid="migrations-page-tabs"
        >
          <Tab label="Migrations" value="migrations" data-testid="migrations-page-tab-migrations" />
          <Tab
            label={`Templates ${templates.length}`}
            value="templates"
            data-testid="migrations-page-tab-templates"
          />
        </Tabs>
      </Box>

      {activeTab === 'migrations' ? (
        <>
          <MigrationsSummaryStats
            counts={summaryCounts}
            activeFilter={statusFilter}
            onFilterChange={setStatusFilter}
          />
          <MigrationsTable
            refetchMigrations={refetchMigrations}
            migrations={migrations || []}
            onDeleteMigration={handleDeleteClick}
            onDeleteSelected={handleDeleteSelected}
            loading={isMigrationsLoading}
            statusFilter={statusFilter}
          />
        </>
      ) : (
        <TemplatesTabPanel onUseTemplate={handleUseTemplate} />
      )}

      <DeleteMigrationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        migrations={selectedMigrations}
      />

      <Snackbar
        open={openSnackbar}
        autoHideDuration={6000}
        onClose={handleCloseSnackbar}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Alert onClose={handleCloseSnackbar} severity="success" sx={{ width: '100%' }}>
          Successfully upgraded to {sessionStorage.getItem('upgradedVersion')}!
        </Alert>
      </Snackbar>
    </>
  )
}
