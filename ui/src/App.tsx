import { styled, Snackbar, Alert } from '@mui/material'
import { useState } from 'react'
import { Route, Routes, useLocation, Navigate } from 'react-router-dom'
import './assets/reset.css'
import AppBar from './components/AppBar'
import RouteCompatibility from './components/RouteCompatibility'
import MigrationFormDrawer from './features/migration/MigrationForm'
import RollingMigrationFormDrawer from './features/migration/RollingMigrationForm'
import DashboardLayout from './pages/dashboard/DashboardLayout'
import MigrationsPage from './pages/dashboard/MigrationsPage'
import AgentsPage from './pages/dashboard/AgentsPage'
import CredentialsPage from './pages/dashboard/CredentialsPage'
import ClusterConversionsPage from './pages/dashboard/ClusterConversionsPage'
import MaasConfigPage from './pages/dashboard/MaasConfigPage'
import Onboarding from './pages/onboarding/Onboarding'
import GlobalSettingsPage from './pages/dashboard/GlobalSettingsPage'

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
