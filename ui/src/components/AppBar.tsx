import Box from "@mui/material/Box"
import Button from "@mui/material/Button"
import IconButton from "@mui/material/IconButton"
import Tooltip from "@mui/material/Tooltip"
import { cleanupAllResources } from "src/api/helpers"
import MenuItem from "@mui/material/MenuItem"
import Menu from "@mui/material/Menu"
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown"
import MigrationIcon from "@mui/icons-material/SwapHoriz"
import ClusterIcon from "@mui/icons-material/Hub"
import LogoutIcon from "@mui/icons-material/Logout"
import { useState } from "react"
import { useLocation } from "react-router-dom"
import ThemeToggle from "./ThemeToggle"
import { authService } from "../api/auth/authService"

export default function ButtonAppBar({ setOpenMigrationForm, hide = false }) {
  const location = useLocation();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const isAuthPage = location.pathname === '/login' || location.pathname === '/change-password';


  const handleMenuClick = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const handleMigrationSelect = (type: string) => {
    setOpenMigrationForm(true, type);
    handleMenuClose();
  };

  const handleLogout = () => {
    authService.logout();
  };

  return (
    <Box sx={{ visibility: hide ? "hidden" : "visible", display: "flex", gap: 2, alignItems: "center", justifyContent: "flex-end", mr: 10, height: 80 }}>
      <ThemeToggle />
      {!isAuthPage && (
        <Tooltip title="Logout">
          <IconButton
            onClick={handleLogout}
            color="inherit"
            size="large"
          >
            <LogoutIcon />
          </IconButton>
        </Tooltip>
      )}
      {
        import.meta.env.MODE === "development" && (
          <Button
            size="large"
            onClick={() => cleanupAllResources()}
            color="error"
            variant="outlined"
          >
            DEV ONLY: Clean Up Resources
          </Button>
        )
      }
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
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <MigrationIcon fontSize="small" />
            <span>Start Migration</span>
          </Box>
        </MenuItem>
        <MenuItem onClick={() => handleMigrationSelect('rolling')}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <ClusterIcon fontSize="small" />
            <span>Start Cluster Conversion</span>
          </Box>
        </MenuItem>
      </Menu>
    </Box>
  )
}
