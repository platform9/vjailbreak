import { Typography } from "@mui/material"
import AppBar from "@mui/material/AppBar"
import Box from "@mui/material/Box"
import Button from "@mui/material/Button"
import Toolbar from "@mui/material/Toolbar"

export default function ButtonAppBar({ setOpenMigrationForm, hide = false }) {
  return (
    <Box sx={{ visibility: hide ? "hidden" : "visible" }}>
      <AppBar position="static" color="transparent">
        <Toolbar>
          <Typography variant="h3">vJailbreak</Typography>
          <Button
            size="large"
            onClick={() => setOpenMigrationForm(true)}
            color="primary"
            variant="contained"
            sx={{
              marginLeft: "auto",
            }}
          >
            Start Migration
          </Button>
        </Toolbar>
      </AppBar>
    </Box>
  )
}
