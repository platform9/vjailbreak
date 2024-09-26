import CheckCircleOutlineIcon from "@mui/icons-material/CheckCircleOutline"
import ErrorOutlineIcon from "@mui/icons-material/ErrorOutline"
import { Box, CircularProgress, Paper, styled, Typography } from "@mui/material"
import { DataGrid, GridColDef } from "@mui/x-data-grid"
import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import ApiClient from "src/api/ApiClient"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import { Migration } from "src/data/migrations/model"
import { useInterval } from "src/hooks/useInterval"

const DashboardContainer = styled("div")({
  // display: "flex",
  // justifyContent: "center",
  // alignItems: "center",
  // height: "100%",
  // width: "100%",
  marginTop: "40px",
})

const columns: GridColDef[] = [
  {
    field: "name",
    headerName: "Name",
    valueGetter: (_, row) => row.metadata?.name,
    flex: 2,
  },
  {
    field: "status",
    headerName: "Status",
    valueGetter: (_, row) => row.status?.phase,
    flex: 1,
    renderCell: (params) => {
      const phase = params.row?.status?.phase
      let icon

      if (phase === "Succeeded") {
        icon = <CheckCircleOutlineIcon style={{ color: "green" }} />
      } else if (phase === "Running") {
        icon = <CircularProgress size={20} style={{ marginRight: 3 }} />
      } else if (phase === "Failed") {
        icon = <ErrorOutlineIcon style={{ color: "red" }} />
      }

      return (
        <>
          {phase ? (
            <Box height={52} display={"flex"} alignItems={"center"}>
              {icon}
              <Typography variant="body2" style={{ marginLeft: 8 }}>
                {phase}
              </Typography>
            </Box>
          ) : null}
        </>
      )
    },
  },
]

const paginationModel = { page: 0, pageSize: 25 }

const { vjailbreak } = ApiClient.getInstance()

export default function Dashboard() {
  const navigate = useNavigate()
  const [migrations, setMigrations] = useState<Migration[] | null>(null)

  const getMigrations = async () => {
    try {
      const data = await vjailbreak.getMigrationList()
      setMigrations(data?.items || [])
    } catch (error) {
      console.error("Error getting MigrationsList", { error })
      return []
    }
  }

  useEffect(() => {
    getMigrations()
  }, [])

  useEffect(() => {
    if (migrations !== null && migrations.length === 0) {
      navigate("/onboarding")
    }
  }, [migrations])

  useInterval(() => {
    getMigrations()
  }, 1000 * 20)

  return (
    <DashboardContainer>
      <Paper sx={{ margin: 4 }}>
        <DataGrid
          rows={migrations || []}
          columns={columns}
          initialState={{ pagination: { paginationModel } }}
          pageSizeOptions={[25, 50, 100]}
          localeText={{ noRowsLabel: "No Migrations Available" }}
          getRowId={(row) => row.metadata?.name}
          slots={{
            toolbar: () => <CustomSearchToolbar title="Migrations" />,
          }}
        />
      </Paper>
    </DashboardContainer>
  )
}
