import { Box } from "@mui/material"
import { GridToolbarQuickFilter } from "@mui/x-data-grid"

const CustomSearchToolbar = () => {
  return (
    <Box
      sx={{
        p: 1,
        display: "flex",
        justifyContent: "flex-end",
        alignItems: "center",
      }}
    >
      <GridToolbarQuickFilter />
    </Box>
  )
}

export default CustomSearchToolbar
