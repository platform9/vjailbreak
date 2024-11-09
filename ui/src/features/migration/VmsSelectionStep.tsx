import { FormControl, FormHelperText, Paper, styled } from "@mui/material"
import { DataGrid, GridColDef, GridRowSelectionModel } from "@mui/x-data-grid"
import { VmData } from "src/api/migration-templates/model"
import CustomLoadingOverlay from "src/components/grid/CustomLoadingOverlay"
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar"
import Step from "../../components/forms/Step"

const VmsSelectionStepContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridGap: theme.spacing(1),
}))

const FieldsContainer = styled("div")(({ theme }) => ({
  display: "grid",
  marginLeft: theme.spacing(6),
}))

const columns: GridColDef[] = [
  { field: "name", headerName: "VM Name", flex: 2 },
]

const paginationModel = { page: 0, pageSize: 5 }

interface VmsSelectionStepProps {
  vms: VmData[]
  onChange: (id: string) => (value: unknown) => void
  error: string
  loadingVms?: boolean
}

export default function VmsSelectionStep({
  vms = [],
  onChange,
  error,
  loadingVms = false,
}: VmsSelectionStepProps) {
  const handleVmSelection = (selectedRowIds: GridRowSelectionModel) => {
    const selectedVms = vms.filter((vm) => selectedRowIds.includes(vm.name))
    onChange("vms")(selectedVms)
  }

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber="2" label="Select Virtual Machines to Migrate" />
      <FieldsContainer>
        <FormControl error={!!error} required>
          <Paper sx={{ width: "100%", height: 338 }}>
            <DataGrid
              rows={vms}
              columns={columns}
              initialState={{ pagination: { paginationModel } }}
              pageSizeOptions={[5, 10, 25]}
              localeText={{ noRowsLabel: "No VMs discovered" }}
              rowHeight={35}
              onRowSelectionModelChange={handleVmSelection}
              getRowId={(row) => row.name}
              slots={{
                toolbar: CustomSearchToolbar,
                loadingOverlay: () => (
                  <CustomLoadingOverlay loadingMessage="Scanning for VMs" />
                ),
              }}
              loading={loadingVms}
              checkboxSelection
              disableColumnMenu
              disableColumnResize
            />
          </Paper>
        </FormControl>
        {error && <FormHelperText error>{error}</FormHelperText>}
      </FieldsContainer>
    </VmsSelectionStepContainer>
  )
}
