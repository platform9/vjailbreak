import {
  Chip,
  FormControl,
  FormHelperText,
  Paper,
  styled,
  Tooltip,
  Box,
} from "@mui/material";
import { DataGrid, GridColDef, GridRow, GridRowSelectionModel } from "@mui/x-data-grid";
import { VmData } from "src/api/migration-templates/model";
import CustomLoadingOverlay from "src/components/grid/CustomLoadingOverlay";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import Step from "../../components/forms/Step";
import { useEffect, useState } from "react";
import { getMigrationPlans } from "src/api/migration-plans/migrationPlans";

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


const columns: GridColDef[] = [
  {
    field: "name",
    headerName: "VM Name",
    flex: 2,
    renderCell: (params) => (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        {params.value}
        {params.row.isMigrated && (
          <Chip
            variant="outlined"
            label="Migrated"
            color="info"
            size="small"
          />
        )}
      </Box>
    ),
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

const MIGRATED_TOOLTIP_MESSAGE = "This VM is migrating or  already has been migrated.";
const DISABLED_TOOLTIP_MESSAGE = "Turn on the VM to enable migration.";
const NO_IP_TOOLTIP_MESSAGE = "VM has not been assigned an IP address yet. Please refresh again.";

interface VmsSelectionStepProps {
  vms: VmData[];
  onChange: (id: string) => (value: unknown) => void;
  error: string;
  loadingVms?: boolean;
  onRefresh?: () => void;
  open?: boolean;
}

export default function VmsSelectionStep({
  vms = [],
  onChange,
  error,
  loadingVms = false,
  onRefresh,
  open = false,
}: VmsSelectionStepProps) {
  const [migratedVms, setMigratedVms] = useState<Set<string>>(new Set());
  const [loadingMigratedVms, setLoadingMigratedVms] = useState(false);

  useEffect(() => {
    const fetchMigratedVms = async () => {
      if (!open) return;

      setLoadingMigratedVms(true);
      try {
        const plans = await getMigrationPlans();
        const migratedVmSet = new Set<string>();

        plans.forEach(plan => {
          plan.spec.virtualmachines.forEach(vmList => {
            vmList.forEach(vm => migratedVmSet.add(vm));
          });
        });

        setMigratedVms(migratedVmSet);
      } catch (error) {
        console.error("Error fetching migrated VMs:", error);
      } finally {
        setLoadingMigratedVms(false);
      }
    };

    fetchMigratedVms();
  }, [open]);

  const vmsWithMigrationStatus = vms.map(vm => ({
    ...vm,
    isMigrated: migratedVms.has(vm.name)
  }));

  const handleVmSelection = (selectedRowIds: GridRowSelectionModel) => {
    const selectedVms = vmsWithMigrationStatus.filter((vm) => selectedRowIds.includes(vm.name));
    onChange("vms")(selectedVms);
  };

  const isRowSelectable = (params) => {
    if (params.row.isMigrated) return false;
    return params.row.vmState === "running" && !!params.row.ipAddress;
  };

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber="2" label="Select Virtual Machines to Migrate" />
      <FieldsContainer>
        <FormControl error={!!error} required>
          <Paper sx={{ width: "100%", height: 389 }}>
            <DataGrid
              rows={vmsWithMigrationStatus}
              columns={columns}
              initialState={{
                pagination: { paginationModel },
                sorting: {
                  sortModel: [{ field: 'vmState', sort: 'asc' }],
                },
              }}
              pageSizeOptions={[5, 10, 25]}
              localeText={{ noRowsLabel: "No VMs discovered" }}
              rowHeight={45}
              onRowSelectionModelChange={handleVmSelection}
              getRowId={(row) => row.name}
              isRowSelectable={isRowSelectable}
              slots={{
                toolbar: (props) => (
                  <CustomSearchToolbar
                    {...props}
                    onRefresh={onRefresh}
                    disableRefresh={loadingVms || loadingMigratedVms}
                    placeholder="Search by  Name, Status, IP Address, or Network Interface(s)"
                  />
                ),
                loadingOverlay: () => (
                  <CustomLoadingOverlay loadingMessage="Scanning for VMs" />
                ),
                row: (props) => {
                  const isVmStopped = props.row.vmState !== "running";
                  const runningButNoIp = props.row.vmState === "running" && !props.row.ipAddress;
                  const isMigrated = props.row.isMigrated;

                  let tooltipMessage = "";
                  if (isMigrated) {
                    tooltipMessage = MIGRATED_TOOLTIP_MESSAGE;
                  } else if (isVmStopped) {
                    tooltipMessage = DISABLED_TOOLTIP_MESSAGE;
                  } else if (runningButNoIp) {
                    tooltipMessage = NO_IP_TOOLTIP_MESSAGE;
                  }

                  return (
                    <Tooltip
                      title={tooltipMessage}
                      followCursor
                    >
                      <span style={{ display: 'contents' }}>
                        <GridRow {...props} />
                      </span>
                    </Tooltip>
                  );
                },
              }}
              loading={loadingVms || loadingMigratedVms}
              checkboxSelection
              disableColumnMenu
              disableColumnResize
              getRowClassName={(params) =>
                (params.row.vmState !== "running" || !params.row.ipAddress || params.row.isMigrated)
                  ? "disabled-row"
                  : ""
              }
            />
          </Paper>
        </FormControl>
        {error && <FormHelperText error>{error}</FormHelperText>}
      </FieldsContainer>
    </VmsSelectionStepContainer>
  );
}
