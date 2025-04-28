import { Typography } from "@mui/material"
import AppBar from "@mui/material/AppBar"
import Box from "@mui/material/Box"
import Button from "@mui/material/Button"
import Toolbar from "@mui/material/Toolbar"
import { cleanupAllResources } from "src/api/helpers"
import MenuItem from "@mui/material/MenuItem"
import Menu from "@mui/material/Menu"
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown"
import { useState } from "react"
import { useThemeContext } from "src/theme/ThemeContext"
import ThemeToggle from "./ThemeToggle"

export default function ButtonAppBar({ setOpenMigrationForm, hide = false }) {
  const { mode } = useThemeContext();
  const [anchorEl, setAnchorEl] = useState(null);

  const openGrafanaDashboard = () => {
    window.open(`https://${window.location.host}/grafana`, "_blank")
  }

  const handleMenuClick = (event) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const handleMigrationSelect = (type) => {
    setOpenMigrationForm(true, type);
    handleMenuClose();
  };

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
              onClick={handleMenuClick}
              color="primary"
              variant="contained"
              endIcon={<KeyboardArrowDownIcon />}
            >
              Start Migration
            </Button>
            <Menu
              anchorEl={anchorEl}
              open={Boolean(anchorEl)}
              onClose={handleMenuClose}
            >
              <MenuItem onClick={() => handleMigrationSelect('standard')}>
                Start Migration
              </MenuItem>
              <MenuItem onClick={() => handleMigrationSelect('rolling')}>
                Start Rolling Migration
              </MenuItem>
            </Menu>
          </Box>
        </Toolbar>
      </AppBar>
    </Box>
  )
}
