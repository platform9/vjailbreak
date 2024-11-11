import { Paper, styled } from "@mui/material"
import { DataGrid, GridColDef } from "@mui/x-data-grid"
import { useEffect } from "react"
import { useNavigate } from "react-router-dom"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import { FIVE_SECONDS, THIRTY_SECONDS } from "src/constants"
import { useMigrationsQuery } from "src/hooks/api/useMigrationsQuery"
import MigrationProgressWithPopover from "./MigrationProgressWithPopover"

const DashboardContainer = styled("div")({
  display: "flex",
  justifyContent: "center",
  width: "100%",
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
    valueGetter: (_, row) => row?.status?.phase || "Pending",
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

export default function Dashboard() {
  const navigate = useNavigate()

  const { data: migrations } = useMigrationsQuery(undefined, {
    refetchInterval: (query) => {
      const migrations = query?.state?.data || []
      const hasPendingMigration = !!migrations.find(
        (m) => m.status === undefined
      )
      return hasPendingMigration ? FIVE_SECONDS : THIRTY_SECONDS
    },
  })

  useEffect(() => {
    if (!!migrations && migrations.length === 0) {
      navigate("/onboarding")
    }
  }, [migrations, navigate])

  return (
    <DashboardContainer>
      <Paper sx={{ width: "95%", margin: "auto" }}>
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
