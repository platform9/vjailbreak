import { styled } from "@mui/material"
import { useState } from "react"
import { Route, Routes, useLocation, Navigate } from "react-router-dom"
import "./assets/reset.css"
import AppBar from "./components/AppBar"
import AuthGuard from "./components/AuthGuard"
import RouteCompatibility from "./components/RouteCompatibility"
import MigrationFormDrawer from "./features/migration/MigrationForm"
import RollingMigrationFormDrawer from "./features/migration/RollingMigrationForm"
import DashboardLayout from "./pages/dashboard/DashboardLayout"
import MigrationsPage from "./pages/dashboard/MigrationsPage"
import AgentsPage from "./pages/dashboard/AgentsPage"
import CredentialsPage from "./pages/dashboard/CredentialsPage"
import ClusterConversionsPage from "./pages/dashboard/ClusterConversionsPage"
import MaasConfigPage from "./pages/dashboard/MaasConfigPage"
import Onboarding from "./pages/onboarding/Onboarding"
import LoginPage from "./pages/auth/LoginPage"
import ChangePasswordPage from "./pages/auth/ChangePasswordPage"
import IdentityProvidersPage from "./pages/settings/IdentityProvidersPage"

const AppFrame = styled("div")(() => ({
  position: "relative",
  display: "grid",
  gridTemplateRows: "auto 1fr",
  height: "100vh",
  overflow: "hidden"
}))

const AppContent = styled("div")(({ theme }) => ({
  overflow: "hidden",
  display: "flex",
  flexDirection: "column",
  flex: 1,
  [theme.breakpoints.up("lg")]: {
    maxWidth: "1600px",
    margin: "0 auto",
    width: "100%",
  },
}))

function App() {
  const location = useLocation()
  const [openMigrationForm, setOpenMigrationForm] = useState(false)
  const [migrationType, setMigrationType] = useState('standard')
  const hideAppbar =
    location.pathname === "/onboarding" || location.pathname === "/"

  const handleOpenMigrationForm = (open, type = 'standard') => {
    setOpenMigrationForm(open);
    setMigrationType(type);
  };

  return (
    <AppFrame>
      <RouteCompatibility />
      <AppBar setOpenMigrationForm={handleOpenMigrationForm} hide={hideAppbar} />
      <AppContent>
        {openMigrationForm && migrationType === 'standard' && (
          <MigrationFormDrawer
            open
            onClose={() => setOpenMigrationForm(false)}
          />
        )}
        {openMigrationForm && migrationType === 'rolling' && (
          <RollingMigrationFormDrawer
            open
            onClose={() => setOpenMigrationForm(false)}
          />
        )}
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<LoginPage />} />
          <Route path="/change-password" element={<ChangePasswordPage />} />
          
          {/* Protected routes */}
          <Route path="/" element={<Navigate to="/dashboard/migrations" replace />} />
          <Route 
            path="/dashboard" 
            element={
              <AuthGuard>
                <DashboardLayout />
              </AuthGuard>
            }
          >
            <Route path="migrations" element={<MigrationsPage />} />
            <Route path="agents" element={<AgentsPage />} />
            <Route 
              path="credentials" 
              element={
                <AuthGuard requiredRole="credential-manager">
                  <CredentialsPage />
                </AuthGuard>
              } 
            />
            <Route path="cluster-conversions" element={<ClusterConversionsPage />} />
            <Route path="baremetal-config" element={<MaasConfigPage />} />
            <Route 
              path="identity-providers" 
              element={
                <AuthGuard requiredRole="admin">
                  <IdentityProvidersPage />
                </AuthGuard>
              } 
            />
          </Route>
          <Route 
            path="/onboarding" 
            element={
              <AuthGuard>
                <Onboarding />
              </AuthGuard>
            } 
          />
        </Routes>
      </AppContent>
    </AppFrame>
  )
}

export default App
