import { styled, Snackbar, Alert } from '@mui/material'
import { useState } from 'react'
import { Route, Routes, useLocation, Navigate } from 'react-router-dom'
import './assets/reset.css'
import { AppBar, DashboardLayout } from './components/layout'
import { RouteCompatibility } from './components/providers'
import MigrationFormDrawer from './features/migration/MigrationForm'
import RollingMigrationFormDrawer from './features/migration/RollingMigrationForm'
import MigrationsPage from './features/migration/pages/MigrationsPage'
import AgentsPage from './features/agents/pages/AgentsPage'
import CredentialsPage from './features/credentials/pages/CredentialsPage'
import ClusterConversionsPage from './features/clusterConversions/pages/ClusterConversionsPage'
import MaasConfigPage from './features/baremetalConfig/pages/MaasConfigPage'
import Onboarding from './features/onboarding/pages/Onboarding'
import GlobalSettingsPage from './features/globalSettings/pages/GlobalSettingsPage'

const AppFrame = styled('div')(() => ({
  position: 'relative',
  display: 'grid',
  gridTemplateRows: 'auto 1fr',
  height: '100vh',
  overflow: 'hidden'
}))

const AppContent = styled('div')(({ theme }) => ({
  overflow: 'hidden',
  display: 'flex',
  flexDirection: 'column',
  flex: 1,
  [theme.breakpoints.up('lg')]: {
    maxWidth: '1600px',
    margin: '0 auto',
    width: '100%'
  }
}))

function App() {
  const location = useLocation()
  const [openMigrationForm, setOpenMigrationForm] = useState(false)
  const [migrationType, setMigrationType] = useState('standard')
  const [notification, setNotification] = useState({
    open: false,
    message: '',
    severity: 'success' as 'error' | 'info' | 'success' | 'warning'
  })
  const hideAppbar = location.pathname === '/onboarding' || location.pathname === '/'

  const handleOpenMigrationForm = (open, type = 'standard') => {
    setOpenMigrationForm(open)
    setMigrationType(type)
  }

  const handleSuccess = (message: string) => {
    setNotification({
      open: true,
      message,
      severity: 'success'
    })
  }

  return (
    <AppFrame>
      <RouteCompatibility />
      <AppBar setOpenMigrationForm={handleOpenMigrationForm} hide={hideAppbar} />
      <AppContent>
        {openMigrationForm && migrationType === 'standard' && (
          <MigrationFormDrawer
            open
            onClose={() => setOpenMigrationForm(false)}
            onSuccess={handleSuccess}
          />
        )}
        {openMigrationForm && migrationType === 'rolling' && (
          <RollingMigrationFormDrawer open onClose={() => setOpenMigrationForm(false)} />
        )}
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard/migrations" replace />} />
          <Route path="/dashboard" element={<DashboardLayout />}>
            <Route path="migrations" element={<MigrationsPage />} />
            <Route path="agents" element={<AgentsPage />} />
            <Route path="credentials" element={<CredentialsPage />} />
            <Route path="cluster-conversions" element={<ClusterConversionsPage />} />
            <Route path="baremetal-config" element={<MaasConfigPage />} />
            <Route path="global-settings" element={<GlobalSettingsPage />} />
          </Route>
          <Route path="/onboarding" element={<Onboarding />} />
        </Routes>
      </AppContent>
      <Snackbar
        open={notification.open}
        autoHideDuration={6000}
        onClose={() => setNotification((prev) => ({ ...prev, open: false }))}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
        sx={{
          top: { xs: 80, sm: 90 },
          right: { xs: 16, sm: 24 }
        }}
      >
        <Alert
          onClose={() => setNotification((prev) => ({ ...prev, open: false }))}
          severity={notification.severity}
          variant="filled"
          sx={{
            minWidth: '350px',
            fontSize: '1rem',
            fontWeight: 600,
            boxShadow: '0 8px 24px rgba(0, 0, 0, 0.25)',
            '& .MuiAlert-icon': {
              fontSize: '28px'
            },
            '& .MuiAlert-message': {
              fontSize: '1rem',
              fontWeight: 600
            }
          }}
        >
          {notification.message}
        </Alert>
      </Snackbar>
    </AppFrame>
  )
}

export default App
