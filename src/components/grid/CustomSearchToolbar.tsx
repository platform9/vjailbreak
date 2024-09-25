import { Box, Typography } from "@mui/material"
import { GridToolbarQuickFilter } from "@mui/x-data-grid"

const CustomSearchToolbar = ({ title }) => {
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
      <GridToolbarQuickFilter sx={{ marginLeft: "auto" }} />
    </Box>
  )
}

export default CustomSearchToolbar
