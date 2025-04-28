import { styled } from "@mui/material"
import { useEffect, useState } from "react"
import { Route, Routes, useLocation, useNavigate } from "react-router-dom"
import "./assets/reset.css"
import AppBar from "./components/AppBar"
import MigrationFormDrawer from "./features/migration/MigrationForm"
import RollingMigrationFormDrawer from "./features/migration/RollingMigrationForm"
import { useMigrationsQuery } from "./hooks/api/useMigrationsQuery"
import Dashboard from "./pages/dashboard/Dashboard"
import Onboarding from "./pages/onboarding/Onboarding"
import { useNodesQuery } from "./hooks/api/useNodesQuery"

const AppFrame = styled("div")(() => ({
  position: "relative",
  display: "grid",
  gridTemplateRows: "auto 1fr",
  height: "100vh",
  overflow: "hidden"
}))

const AppContent = styled("div")(({ theme }) => ({
  overflow: "auto",
  [theme.breakpoints.up("lg")]: {
    maxWidth: "1600px",
    margin: "0 auto",
    width: "100%",
  },
}))

function App() {
  const navigate = useNavigate()
  const location = useLocation()
  const [openMigrationForm, setOpenMigrationForm] = useState(false)
  const [migrationType, setMigrationType] = useState('standard')

  const { data: migrations } = useMigrationsQuery()
  const { data: nodes } = useNodesQuery()

  useEffect(() => {
    if (!migrations || !nodes) {
      return
    } else if (migrations.length === 0 && nodes.length === 0) {
      navigate("/onboarding")
    } else {
      navigate("/dashboard")
    }
  }, [migrations, nodes, navigate])

  const hideAppbar =
    location.pathname === "/onboarding" || location.pathname === "/"

  const handleOpenMigrationForm = (open, type = 'standard') => {
    setOpenMigrationForm(open);
    setMigrationType(type);
  };

  return (
    <AppFrame>
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
          <Route path="/" element={<div></div>} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/onboarding" element={<Onboarding />} />
        </Routes>
      </AppContent>
    </AppFrame>
  )
}

export default App
