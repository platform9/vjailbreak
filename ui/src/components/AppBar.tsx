import { Typography } from "@mui/material"
import AppBar from "@mui/material/AppBar"
import Box from "@mui/material/Box"
import Button from "@mui/material/Button"
import Toolbar from "@mui/material/Toolbar"
import { cleanupAllResources } from "src/api/helpers"
import ThemeToggle from "./ThemeToggle"
import { useThemeContext } from "src/theme/ThemeContext"

export default function ButtonAppBar({ setOpenMigrationForm, hide = false }) {
  const { mode } = useThemeContext();

  const openGrafanaDashboard = () => {
    window.open(`https://${window.location.host}/grafana`, "_blank")
  }

  const openMigrationForm = () => {
    setOpenMigrationForm(true)
  }

  return (
    <Box sx={{ visibility: hide ? "hidden" : "visible" }}>
      <AppBar
        position="static"
        color="default"
        sx={{
          backgroundColor: mode === 'dark'
            ? `rgba(30, 30, 30, 0.9)`
            : `rgba(255, 255, 255, 0.9)`,
        }}
        elevation={1}
      >
        <Toolbar>
          <Typography variant="h3">vJailbreak</Typography>
          <Box sx={{ display: "flex", gap: 2, marginLeft: "auto", alignItems: "center" }}>
            <ThemeToggle />
            {import.meta.env.MODE === "development" && (
              <Button
                size="large"
                onClick={() => cleanupAllResources()}
                color="error"
                variant="outlined"
              >
                DEV ONLY: Clean Up Resources
              </Button>
            )}
            <Button
              size="large"
              onClick={openGrafanaDashboard}
              color="primary"
              variant="outlined"
            >
              Open Grafana
            </Button>
            <Button
              size="large"
              onClick={openMigrationForm}
              color="primary"
              variant="contained"
            >
              Start Migration
            </Button>
          </Box>
        </Toolbar>
      </AppBar>
    </Box>
  )
}
