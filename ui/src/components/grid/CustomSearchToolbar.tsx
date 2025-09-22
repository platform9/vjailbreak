import { Box, IconButton, Tooltip, Typography, Menu, MenuItem } from "@mui/material"
import { GridToolbarQuickFilter } from "@mui/x-data-grid"
import { RefreshRounded, FilterList as FilterListIcon } from "@mui/icons-material"
import { useState } from "react"

interface CustomSearchToolbarProps {
  title?: string;
  onRefresh?: () => void;
  disableRefresh?: boolean;
  placeholder?: string;
  onFilterChange?: (filter: string) => void;
  currentFilter?: string;
}

const CustomSearchToolbar = ({
  title,
  onRefresh,
  disableRefresh = false,
  placeholder = "Search",
  onFilterChange,
  currentFilter = 'All'
}: CustomSearchToolbarProps) => {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleFilterClick = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleFilterClose = () => {
    setAnchorEl(null);
  };

  const handleFilterSelect = (filter: string) => {
    onFilterChange?.(filter);
    handleFilterClose();
  };

  const filterOptions = ['All', 'In Progress', 'Succeeded', 'Failed'];

  return (
    <Box
      sx={{
        p: 1,
        display: "flex",
        alignItems: "center",
        marginLeft: 2,
        marginRight: 2,
      }}
    >
      {title && <Typography variant="h6">{title}</Typography>}
      <Box sx={{ marginLeft: "auto", display: "flex", gap: 1, alignItems: "center" }}>
        <Box sx={{ display: "flex", alignItems: "center" }}>
          {onRefresh && (
            <Tooltip title="Refresh">
              <span>
                <IconButton
                  onClick={onRefresh}
                  disabled={disableRefresh}
                  size="small"
                >
                  <RefreshRounded />
                </IconButton>
              </span>
            </Tooltip>
          )}
          {onFilterChange && (
            <>
              <Tooltip title="Filter">
                <IconButton onClick={handleFilterClick} size="small">
                  <FilterListIcon />
                </IconButton>
              </Tooltip>
              <Menu
                anchorEl={anchorEl}
                open={open}
                onClose={handleFilterClose}
              >
                {filterOptions.map((option) => (
                  <MenuItem
                    key={option}
                    selected={option === currentFilter}
                    onClick={() => handleFilterSelect(option)}
                  >
                    {option}
                  </MenuItem>
                ))}
              </Menu>
            </>
          )}
        </Box>
        <Box sx={{ maxWidth: "300px" }}>
          <div>
            <GridToolbarQuickFilter
              placeholder={placeholder}
              sx={{
                "& .MuiInputBase-input": {
                  textOverflow: "ellipsis",
                }
              }}
            />
          </div>
        </Box>
      </Box>
    </Box>
  )
}

export default CustomSearchToolbar
