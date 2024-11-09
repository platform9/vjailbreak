import { styled } from "@mui/material"
import { useEffect, useState } from "react"
import { Route, Routes, useLocation, useNavigate } from "react-router-dom"
import "./assets/reset.css"
import AppBar from "./components/AppBar"
import MigrationFormDrawer from "./features/migration/MigrationForm"
import { useMigrationsQuery } from "./hooks/api/useMigrationsQuery"
import Dashboard from "./pages/dashboard/Dashboard"
import Onboarding from "./pages/onboarding/Onboarding"

const AppFrame = styled("div")(() => ({
  position: "relative",
  display: "grid",
  width: "100vw",
  height: "100vh",
  maxWidth: "100vw",
  maxHeight: "100vh",
  gridTemplateRows: "auto 1fr",
}))

const AppContent = styled("div")(({ theme }) => ({
  width: "100%",
  height: "100%",
  [theme.breakpoints.up("lg")]: {
    maxWidth: "1600px",
    margin: "0 auto",
  },
}))

function App() {
  const navigate = useNavigate()
  const location = useLocation()
  const [openMigrationForm, setOpenMigrationForm] = useState(false)

  const { data: migrations } = useMigrationsQuery()

  useEffect(() => {
    if (!migrations) {
      return
    } else if (migrations.length === 0) {
      navigate("/onboarding")
    } else {
      navigate("/dashboard")
    }
  }, [migrations, navigate])

  const hideAppbar =
    location.pathname === "/onboarding" || location.pathname === "/"

  return (
    <AppFrame>
      <AppBar setOpenMigrationForm={setOpenMigrationForm} hide={hideAppbar} />
      <AppContent>
        {openMigrationForm && (
          <MigrationFormDrawer
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
