import { useState, useEffect, useCallback, useMemo } from 'react'
import { Box, Tab, Tabs, Typography, Button } from '@mui/material'
import Snackbar from '@mui/material/Snackbar'
import Alert from '@mui/material/Alert'
import Tooltip from '@mui/material/Tooltip'
import AddIcon from '@mui/icons-material/Add'
import { CustomSearchToolbar } from 'src/components/grid'
import { FIVE_SECONDS, THIRTY_SECONDS } from 'src/constants'
import { useMigrationsQuery } from '../hooks/useMigrationsQuery'
import { Migration } from '../api/migrations'
import MigrationsTable from '../components/MigrationsTable'
import DeleteMigrationDialog from '../components/DeleteMigrationDialog'
import { useMigrationStatusMonitor } from '../hooks/useMigrationStatusMonitor'
import { useMigrationTemplatesQuery } from '../hooks/useMigrationTemplatesQuery'
import { useMigrationFormActions } from '../context/MigrationFormContext'
import TemplatesTabPanel from '../components/templates/TemplatesTabPanel'
import TemplatesToolbar from '../components/templates/TemplatesToolbar'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import { getMigrationStatusCategory, STATUS_FILTER_OPTIONS } from '../utils/migrationTableUtils'
import type { TemplateCopyMethodFilter, TemplateSortKey } from '../utils/templateFilters'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'

type MigrationsPageTab = 'migrations' | 'templates'

// Tab label with a small count "pill" — filled primary when its tab is active,
// muted/outlined when inactive, matching the Tab's own selected/unselected color.
function TabLabelWithCount({
  label,
  count,
  active
}: {
  label: string
  count: number
  active: boolean
}) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
      <span>{label}</span>
      <Box
        component="span"
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          minWidth: 20,
          height: 20,
          px: 0.5,
          borderRadius: 10,
          fontSize: '0.7rem',
          fontWeight: 700,
          lineHeight: 1,
          bgcolor: active ? 'primary.main' : 'action.selected',
          color: active ? 'primary.contrastText' : 'text.secondary'
        }}
      >
        {count}
      </Box>
    </Box>
  )
}

export default function MigrationsPage() {
  const [activeTab, setActiveTab] = useState<MigrationsPageTab>('migrations')
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedMigrations, setSelectedMigrations] = useState<Migration[]>([])
  const [openSnackbar, setOpenSnackbar] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const [statusFilter, setStatusFilter] = useState('All')
  const [dateFilter, setDateFilter] = useState('All Time')
  const [isRefreshing, setIsRefreshing] = useState(false)
  const [bulkActionsContainer, setBulkActionsContainer] = useState<HTMLDivElement | null>(null)
  const bulkActionsSlotRef = useCallback((node: HTMLDivElement | null) => {
    setBulkActionsContainer(node)
  }, [])
  const [templateQuery, setTemplateQuery] = useState('')
  const [templateCopyMethodFilter, setTemplateCopyMethodFilter] =
    useState<TemplateCopyMethodFilter>('all')
  const [templateSortKey, setTemplateSortKey] = useState<TemplateSortKey>('created')
  const [templateView, setTemplateView] = useState<'grid' | 'list'>('grid')
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

  const handleCreateTemplate = () => {
    openMigrationForm('standard', undefined, undefined, 'create')
  }

  const handleEditTemplate = (template: SavedTemplate) => {
    openMigrationForm('standard', undefined, template, 'edit')
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

  const handleRefresh = useCallback(async () => {
    setIsRefreshing(true)
    try {
      await refetchMigrations()
    } finally {
      setIsRefreshing(false)
    }
  }, [refetchMigrations])

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
          <Typography variant="h4" component="h1" data-testid="migrations-page-title">
            {activeTab === 'migrations' ? 'Migrations' : 'Migration Templates'}
          </Typography>
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
              Save and reuse migration configurations.
            </Typography>
          )}
        </Box>
        <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'center' }}>
          {activeTab === 'migrations' ? (
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
          ) : (
            <Tooltip title={startMigrationDisabled ? startMigrationDisabledReason : ''} arrow>
              <span>
                <Button
                  variant="contained"
                  color="primary"
                  startIcon={<AddIcon />}
                  onClick={handleCreateTemplate}
                  disabled={startMigrationDisabled}
                  data-testid="create-template-button"
                >
                  Create New Template
                </Button>
              </span>
            </Tooltip>
          )}
        </Box>
      </Box>

      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 2,
          borderBottom: 1,
          borderColor: 'divider',
          mb: 2
        }}
      >
        <Tabs
          value={activeTab}
          onChange={(_event, value: MigrationsPageTab) => setActiveTab(value)}
          data-testid="migrations-page-tabs"
        >
          <Tab
            label={
              <TabLabelWithCount
                label="Migrations"
                count={migrationsCount}
                active={activeTab === 'migrations'}
              />
            }
            value="migrations"
            data-testid="migrations-page-tab-migrations"
          />
          <Tab
            label={
              <TabLabelWithCount
                label="Templates"
                count={templates.length}
                active={activeTab === 'templates'}
              />
            }
            value="templates"
            data-testid="migrations-page-tab-templates"
          />
        </Tabs>

        {activeTab === 'migrations' ? (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap' }}>
            {/* Portal target for MigrationsTable's bulk-actions bar (Delete/Cutover/Retry
                Selected) — keeps that state and logic owned by the table while letting it
                render here, before the search bar, instead of on its own row above the grid. */}
            <Box ref={bulkActionsSlotRef} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }} />
            <CustomSearchToolbar
              placeholder="Search VM, tenant, OS..."
              searchValue={searchValue}
              onSearchChange={setSearchValue}
              onRefresh={handleRefresh}
              isRefreshing={isRefreshing}
              currentDateFilter={dateFilter}
              onDateFilterChange={setDateFilter}
              currentStatusFilter={statusFilter}
              onStatusFilterChange={setStatusFilter}
              statusFilterOptions={[...STATUS_FILTER_OPTIONS]}
              maxSearchWidth={220}
            />
          </Box>
        ) : (
          <TemplatesToolbar
            query={templateQuery}
            onQueryChange={setTemplateQuery}
            copyMethodFilter={templateCopyMethodFilter}
            onCopyMethodFilterChange={setTemplateCopyMethodFilter}
            sortKey={templateSortKey}
            onSortKeyChange={setTemplateSortKey}
            view={templateView}
            onViewChange={setTemplateView}
          />
        )}
      </Box>

      {activeTab === 'migrations' ? (
        <MigrationsTable
          refetchMigrations={refetchMigrations}
          migrations={migrations || []}
          onDeleteMigration={handleDeleteClick}
          onDeleteSelected={handleDeleteSelected}
          bulkActionsContainer={bulkActionsContainer}
          loading={isMigrationsLoading}
          searchValue={searchValue}
          statusFilter={statusFilter}
          dateFilter={dateFilter}
        />
      ) : (
        <TemplatesTabPanel
          onUseTemplate={handleUseTemplate}
          onEditTemplate={handleEditTemplate}
          query={templateQuery}
          copyMethodFilter={templateCopyMethodFilter}
          sortKey={templateSortKey}
          view={templateView}
        />
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
