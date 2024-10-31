import { Paper, styled } from "@mui/material"
import { DataGrid, GridColDef } from "@mui/x-data-grid"
import { useEffect, useState } from "react"
import { useNavigate } from "react-router-dom"
import ApiClient from "src/api/ApiClient"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import { Migration } from "src/data/migrations/model"
import { useInterval } from "src/hooks/useInterval"
import MigrationProgressWithPopover from "./MigrationProgressWithPopover"
// import MigrationProgress from "./MigrationProgress"

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
    valueGetter: (_, row) => row?.status?.phase,
    flex: 1,
  },
  {
    field: "status.conditions",
    headerName: "Progress",
    valueGetter: (_, row) => row.status?.phase,
    flex: 2,
    renderCell: (params) => {
      const phase = params.row?.status?.phase
      const conditions = params.row?.status?.conditions
      return conditions ? (
        <MigrationProgressWithPopover
          phase={phase}
          conditions={params.row?.status?.conditions}
        />
      ) : null
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
      setMigrations([])
    }
  }

  useEffect(() => {
    getMigrations()
  }, [])

  useEffect(() => {
    if (migrations !== null && migrations.length === 0) {
      navigate("/onboarding")
      window.location.reload()
    }
  }, [migrations, navigate])

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
