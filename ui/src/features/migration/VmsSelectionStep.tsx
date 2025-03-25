import {
  Chip,
  FormControl,
  FormHelperText,
  Paper,
  styled,
  Tooltip,
  Box,
  Button,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  FormLabel,
  MenuItem,
  Select,
  Typography,
} from "@mui/material";
import { DataGrid, GridColDef, GridRow, GridRowSelectionModel } from "@mui/x-data-grid";
import { VmData } from "src/api/migration-templates/model";
import CustomLoadingOverlay from "src/components/grid/CustomLoadingOverlay";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import Step from "../../components/forms/Step";
import { useEffect, useState } from "react";
import { getMigrationPlans } from "src/api/migration-plans/migrationPlans";

// Example flavor data structure - replace with your actual flavor data source
interface FlavorOption {
  id: string;
  name: string;
  vcpu: number;
  ram: number;
  storage: string;
}

const EXAMPLE_FLAVORS: FlavorOption[] = [
  { id: "t2.micro", name: "t2.micro", vcpu: 1, ram: 1, storage: "10GB" },
  { id: "t2.small", name: "t2.small", vcpu: 1, ram: 2, storage: "20GB" },
  { id: "t2.medium", name: "t2.medium", vcpu: 2, ram: 4, storage: "50GB" },
  { id: "m4.large", name: "m4.large", vcpu: 2, ram: 8, storage: "100GB" },
  { id: "m4.xlarge", name: "m4.xlarge", vcpu: 4, ram: 16, storage: "200GB" },
];

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

// Custom toolbar with assign flavors button
const CustomToolbarWithActions = (props) => {
  const { rowSelectionModel, onAssignFlavor, ...toolbarProps } = props;

  return (
    <Box sx={{ display: 'flex', justifyContent: 'flex-end', width: '100%', padding: '4px  8px' }}>
      {rowSelectionModel.length > 0 && (
        <Button
          variant="text"
          color="primary"
          onClick={onAssignFlavor}
          size="small"
          sx={{ ml: 1 }}
        >
          Assign Flavor ({rowSelectionModel.length})
        </Button>
      )}
      <CustomSearchToolbar {...toolbarProps} />
    </Box>
  );
};

// Modify VmData interface to include flavor
interface VmDataWithFlavor extends VmData {
  flavor?: string;
  isMigrated?: boolean;
}

// Update columns to include flavor
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
  {
    field: "flavor",
    headerName: "Flavor",
    flex: 1,
    valueGetter: (value) => value || "-",
  },
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
  const [flavorDialogOpen, setFlavorDialogOpen] = useState(false);
  const [selectedFlavor, setSelectedFlavor] = useState<string>("");
  const [rowSelectionModel, setRowSelectionModel] = useState<GridRowSelectionModel>([]);
  const [vmsWithFlavor, setVmsWithFlavor] = useState<VmDataWithFlavor[]>([]);

  useEffect(() => {
    const fetchMigratedVms = async () => {
      if (!open) return;

      setLoadingMigratedVms(true);
      try {
        const plans = await getMigrationPlans();
        const migratedVmSet = new Set<string>();

        plans.forEach(plan => {
          plan.spec.virtualMachines.forEach(vmList => {
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

  useEffect(() => {
    // Initialize VMs with flavor data
    const initialVmsWithFlavor = vms.map(vm => ({
      ...vm,
      isMigrated: migratedVms.has(vm.name),
      flavor: undefined
    }));
    setVmsWithFlavor(initialVmsWithFlavor);
  }, [vms, migratedVms]);

  const handleVmSelection = (selectedRowIds: GridRowSelectionModel) => {
    setRowSelectionModel(selectedRowIds);
    const selectedVms = vmsWithFlavor.filter((vm) => selectedRowIds.includes(vm.name));
    onChange("vms")(selectedVms);
  };

  const handleOpenFlavorDialog = () => {
    if (rowSelectionModel.length === 0) return;
    setFlavorDialogOpen(true);
  };

  const handleCloseFlavorDialog = () => {
    setFlavorDialogOpen(false);
    setSelectedFlavor("");
  };

  const handleFlavorChange = (event) => {
    setSelectedFlavor(event.target.value);
  };

  const handleApplyFlavor = () => {
    if (!selectedFlavor) {
      handleCloseFlavorDialog();
      return;
    }

    // Update VMs with the selected flavor
    const updatedVms = vmsWithFlavor.map(vm => {
      if (rowSelectionModel.includes(vm.name)) {
        return { ...vm, flavor: selectedFlavor };
      }
      return vm;
    });

    setVmsWithFlavor(updatedVms);
    onChange("vms")(updatedVms.filter((vm) => rowSelectionModel.includes(vm.name)));
    handleCloseFlavorDialog();
  };

  const selectedFlavorDetails = EXAMPLE_FLAVORS.find(f => f.id === selectedFlavor);

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
              rows={vmsWithFlavor}
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
              rowSelectionModel={rowSelectionModel}
              getRowId={(row) => row.name}
              isRowSelectable={isRowSelectable}
              slots={{
                toolbar: (props) => (
                  <CustomToolbarWithActions
                    {...props}
                    onRefresh={onRefresh}
                    disableRefresh={loadingVms || loadingMigratedVms}
                    placeholder="Search by Name, Status, IP Address, or Network Interface(s)"
                    rowSelectionModel={rowSelectionModel}
                    onAssignFlavor={handleOpenFlavorDialog}
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

      {/* Flavor Assignment Dialog */}
      <Dialog
        open={flavorDialogOpen}
        onClose={handleCloseFlavorDialog}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>
          Assign Flavor to {rowSelectionModel.length} {rowSelectionModel.length === 1 ? 'VM' : 'VMs'}
        </DialogTitle>
        <DialogContent>
          <Box sx={{ my: 2 }}>
            <FormLabel>Select Flavor</FormLabel>
            <Select
              fullWidth
              value={selectedFlavor}
              onChange={handleFlavorChange}
              size="small"
              sx={{ mt: 1 }}
            >
              {EXAMPLE_FLAVORS.map((flavor) => (
                <MenuItem key={flavor.id} value={flavor.id}>
                  {flavor.name}
                </MenuItem>
              ))}
            </Select>
          </Box>

          {selectedFlavorDetails && (
            <Box sx={{ mt: 2, p: 2, bgcolor: 'background.paper', borderRadius: 1 }}>
              <Typography variant="subtitle2">Flavor details:</Typography>
              <Typography variant="body2">
                {selectedFlavorDetails.vcpu} vCPU, {selectedFlavorDetails.ram}GB RAM, {selectedFlavorDetails.storage} Storage
              </Typography>
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseFlavorDialog}>Cancel</Button>
          <Button
            onClick={handleApplyFlavor}
            variant="contained"
            color="primary"
            disabled={!selectedFlavor}
          >
            Apply to selected VMs
          </Button>
        </DialogActions>
      </Dialog>
    </VmsSelectionStepContainer>
  );
}
