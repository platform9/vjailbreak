import {
  Chip,
  FormControl,
  FormHelperText,
  Paper,
  styled,
  Tooltip,
} from "@mui/material";
import { DataGrid, GridColDef, GridRow, GridRowSelectionModel } from "@mui/x-data-grid";
import { VmData } from "src/api/migration-templates/model";
import CustomLoadingOverlay from "src/components/grid/CustomLoadingOverlay";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import Step from "../../components/forms/Step";
import { useEffect, useRef, useState } from "react";

const VmsSelectionStepContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridGap: theme.spacing(1),
  "& .disabled-row": {
    opacity: 0.6,
    cursor: "not-allowed",
  }
}));

const FieldsContainer = styled("div")(({ theme }) => ({
  display: "grid",
  marginLeft: theme.spacing(6),
}));

// const TruncatedCell = ({ value }: { value: string }) => (
//   <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', width: '100%' }}>
//     {value}
//   </div>
// );

const TruncatedCell = ({ value }: { value: string }) => {
  const [isTruncated, setIsTruncated] = useState(false);
  const cellRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const checkTruncation = () => {
      if (cellRef.current) {
        setIsTruncated(
          cellRef.current.scrollWidth > cellRef.current.clientWidth
        );
      }
    };
    checkTruncation();
  }, [value]);

  const cellContent = (
    <div
      ref={cellRef}
      style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', width: '100%' }}
    >
      {value}
    </div>
  );

  return isTruncated ? (
    <Tooltip title={value}>
      {cellContent}
    </Tooltip>
  ) : cellContent;
};

const columns: GridColDef[] = [
  {
    field: "name",
    headerName: "VM Name",
    flex: 2,
    renderCell: (params) => <TruncatedCell value={params.value} />,
  },
  {
    field: "vmState",
    headerName: "Status",
    flex: 1,
    valueGetter: (value) => value === "running" ? "running" : "stopped", // needed for search to work.
    sortComparator: (v1, v2) => {
      if (v1 === "running" && v2 === "stopped") return -1;
      if (v1 === "stopped" && v2 === "running") return 1;
      return 0;
    },
    renderCell: (params) => (
      <Chip
        variant="outlined"
        label={params.value === "running" ? "Running" : "Stopped"}
        color={params.value === "running" ? "success" : "error"}
        size="small"
      />
    ),
  },
  {
    field: "ipAddress",
    headerName: "Current IP",
    flex: 1,
    valueGetter: (value) => value || " -",
  },
  {
    field: "networks",
    headerName: "Network Interface(s)",
    flex: 1.2,
    valueGetter: (value: string[]) => value?.join(", "),
    renderCell: (params) => <TruncatedCell value={params.value} />,
  },
  {
    field: "osType",
    headerName: "OS",
    valueGetter: (value) => {
      if (value === "linuxGuest") return "Linux";
      if (value === "windowsGuest") return "Windows";
      if (value === "otherGuestFamily") return "Other";
      return "";
    },
    flex: 1,
  },
  // { field: "version", headerName: "Version", flex: 1 },
];

const paginationModel = { page: 0, pageSize: 5 };

const DISABLED_TOOLTIP_MESSAGE = "Turn on the VM to enable migration.";

interface VmsSelectionStepProps {
  vms: VmData[];
  onChange: (id: string) => (value: unknown) => void;
  error: string;
  loadingVms?: boolean;
  onRefresh?: () => void;
}

export default function VmsSelectionStep({
  vms = [],
  onChange,
  error,
  loadingVms = false,
  onRefresh,
}: VmsSelectionStepProps) {
  const handleVmSelection = (selectedRowIds: GridRowSelectionModel) => {
    const selectedVms = vms.filter((vm) => selectedRowIds.includes(vm.name));
    onChange("vms")(selectedVms);
  };

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber="2" label="Select Virtual Machines to Migrate" />
      <FieldsContainer>
        <FormControl error={!!error} required>
          <Paper sx={{ width: "100%", height: 500 }}>
            <DataGrid
              rows={vms}
              columns={columns}
              initialState={{
                pagination: { paginationModel },
                sorting: {
                  sortModel: [{ field: 'vmState', sort: 'asc' }],
                },
              }}
              pageSizeOptions={[5, 10, 25]}
              localeText={{ noRowsLabel: "No VMs discovered" }}
              rowHeight={90}
              onRowSelectionModelChange={handleVmSelection}
              getRowId={(row) => row.name}
              isRowSelectable={(params) => params.row.vmState === "running"}
              slots={{
                toolbar: (props) => (
                  <CustomSearchToolbar
                    {...props}
                    onRefresh={onRefresh}
                    disableRefresh={loadingVms}
                  />
                ),
                loadingOverlay: () => (
                  <CustomLoadingOverlay loadingMessage="Scanning for VMs" />
                ),
                row: (props) => {
                  const isDisabled = props.row.vmState !== "running";
                  return (
                    <Tooltip
                      title={isDisabled ? DISABLED_TOOLTIP_MESSAGE : ""}
                      followCursor
                    >
                      <span style={{ display: 'contents' }}>
                        <GridRow {...props} />
                      </span>
                    </Tooltip>
                  );
                },
              }}
              loading={loadingVms}
              checkboxSelection
              disableColumnMenu
              disableColumnResize
              getRowClassName={(params) =>
                params.row.vmState !== "running" ? "disabled-row" : ""
              }
            />
          </Paper>
        </FormControl>
        {error && <FormHelperText error>{error}</FormHelperText>}
      </FieldsContainer>
    </VmsSelectionStepContainer>
  );
}
