import { styled } from "@mui/material"
import { useEffect, useState } from "react"
import { Route, Routes, useNavigate } from "react-router-dom"
import ApiClient from "./api/ApiClient"
import "./App.css"
import "./assets/reset.css"
import AppBar from "./components/AppBar"
import { Migration } from "./data/migrations/model"
import MigrationFormDrawer from "./features/migration/MigrationForm"
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

const { vjailbreak } = ApiClient.getInstance()

function App() {
  const navigate = useNavigate()
  const [migrations, setMigrations] = useState<Migration[] | null>(null)
  const [openMigrationForm, setOpenMigrationForm] = useState(false)

  const getMigrations = async () => {
    try {
      const data = await vjailbreak.getMigrationList()
      setMigrations(data?.items || [])
    } catch (error) {
      console.error("Error getting MigrationsList", { error })
      return setMigrations([])
    }
  }

  useEffect(() => {
    getMigrations()
  }, [])

  useEffect(() => {
    if (migrations === null) {
      return
    } else if (migrations.length === 0) {
      navigate("/onboarding")
    } else {
      navigate("/dashboard")
    }
  }, [migrations, navigate])

  const isOnboardingPage = window.location.pathname === "/onboarding"

  return (
    <AppFrame>
      <AppBar
        setOpenMigrationForm={setOpenMigrationForm}
        hide={isOnboardingPage}
      />
      <AppContent>
        {openMigrationForm && (
          <MigrationFormDrawer
            open
            onClose={() => setOpenMigrationForm(false)}
            reloadMigrations={() => getMigrations()}
          />
        )}
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/onboarding" element={<Onboarding />} />
        </Routes>
      </AppContent>
    </AppFrame>
  )
}

export default App
