import { useState, useEffect } from 'react'
import Snackbar from '@mui/material/Snackbar'
import Alert from '@mui/material/Alert'
import { FIVE_SECONDS, THIRTY_SECONDS } from 'src/constants'
import { useMigrationsQuery } from '../hooks/useMigrationsQuery'
import { Migration } from '../api/migrations'
import MigrationsTable from '../components/MigrationsTable'
import DeleteMigrationDialog from '../components/DeleteMigrationDialog'
import { useMigrationStatusMonitor } from '../hooks/useMigrationStatusMonitor'

export default function MigrationsPage() {
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedMigrations, setSelectedMigrations] = useState<Migration[]>([])
  const [openSnackbar, setOpenSnackbar] = useState(false)

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

  return (
    <>
      <MigrationsTable
        refetchMigrations={refetchMigrations}
        migrations={migrations || []}
        onDeleteMigration={handleDeleteClick}
        onDeleteSelected={handleDeleteSelected}
        loading={isMigrationsLoading}
      />

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
