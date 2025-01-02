import {
  Chip,
  FormControl,
  FormHelperText,
  IconButton,
  MenuItem,
  Paper,
  Select,
  styled,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import { DataGrid, GridColDef, GridRowSelectionModel } from "@mui/x-data-grid";
import { VmData } from "src/api/migration-templates/model";
import CustomLoadingOverlay from "src/components/grid/CustomLoadingOverlay";
import CustomSearchToolbar from "src/components/grid/CustomSearchToolbar";
import Step from "../../components/forms/Step";
import { useState } from "react";
import { Edit, EditNote, EditNoteRounded, EditNoteTwoTone, Warning } from "@mui/icons-material";

const VmsSelectionStepContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridGap: theme.spacing(1),
}));

const IP_REGEX =
  /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)$/;

const FormControlContainer = styled("div")(({ theme }) => ({
  display: "flex",
  flexDirection: "column",
  height: "100%",
  justifyContent: "center",
}));

const FieldsContainer = styled("div")(({ theme }) => ({
  display: "grid",
  marginLeft: theme.spacing(6),
}));

const CellContainer = styled("div")(({ theme }) => ({
  display: "flex",
  flexDirection: "column",
  height: "100%",
  justifyContent: "center",
}));

const IpCell = ({ params }) => {
  const [isEditing, setEditing] = useState(false);
  const [customIP, setCustomIP] = useState(params.row.customIP || "");
  const [error, setError] = useState("");

  const handleDoubleClick = () => {
    setEditing(true);
  };

  const handleBlur = () => {
    if (!customIP) {
      setEditing(false);
    }
    if (!IP_REGEX.test(customIP)) {
      setError("Invalid IP address. ");
    } else {
      setError("");
      setEditing(false);
    }
  };

  return (
    <CellContainer>
      {params?.value ? (
        <span>{params.value}</span>
      ) : (
        <div style={{ display: "flex", alignItems: "center" }}>
          {isEditing ? (
            <TextField
              size="small"
              value={customIP}
              onChange={(e) => setCustomIP(e.target.value)}
              onBlur={handleBlur}
              error={!!error}
              helperText={error}
              placeholder="Enter custom IP"
              autoFocus
            />
          ) : (
            <Tooltip
              title={
                customIP
                  ? `Custom IP: ${customIP}. Double-click to edit.`
                  : "IP is not assigned to your VM. It will auto-assign using DHCP or you can give a custom IP by double-clicking the icon."
              }
              arrow
              sx={{
                cursor: "pointer",
                color: customIP ? "inherit" : "gray",
              }}
            >
              {/* {!customIP && <IconButton>-</IconButton>} */}
              <IconButton
                onClick={handleDoubleClick}
                sx={{
                  cursor: "pointer",
                  color: customIP ? "inherit" : "grey",
                }}
              >
                <EditNoteTwoTone />
              </IconButton>
            </Tooltip>
          )}
          {customIP && !isEditing && (
            <Typography
              variant="body1"
              sx={{
                color: "text.secondary",
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              }}
            >
              {customIP}
            </Typography>
          )}
        </div>
      )
      }
    </CellContainer >
  );
};

const columns: GridColDef[] = [
  { field: "name", headerName: "VM Name", flex: 1 },
  {
    field: "status",
    headerName: "Status",
    flex: 1,
    renderCell: (params) => (
      <Chip
        variant="outlined"
        label={params.value}
        color={params.value === "Running" ? "success" : "error"}
        size="small"
      />
    ),
  },
  {
    field: "ip",
    headerName: "IP",
    flex: 1.5,
    renderCell: (params) => <IpCell params={params} />
  },
  // {
  //   field: "ip",
  //   headerName: "IP",
  //   flex: 2,
  //   renderCell: (params) =>
  //     params.value ? (
  //       <span>{params.value}</span>
  //     ) : (
  //       <FormControlContainer>
  //         <FormControl fullWidth size="small">
  //           <Select
  //             defaultValue=""
  //             displayEmpty
  //             size="small"
  //             renderValue={(selected) => {
  //               if (selected === "custom") {
  //                 return (
  //                   <TextField
  //                     placeholder="Custom IP"
  //                     size="small"
  //                     onClick={(e) => e.stopPropagation()}
  //                   />
  //                 );
  //               }
  //               return selected || <em>Select IP Option</em>;
  //             }}
  //           >
  //             <MenuItem value="">
  //               <em>Select IP Option</em>
  //             </MenuItem>
  //             <MenuItem value="Auto Assign using DHCP">
  //               Auto Assign using DHCP
  //             </MenuItem>
  //             <MenuItem value="custom">Custom IP</MenuItem>
  //           </Select>
  //         </FormControl>
  //       </FormControlContainer>
  //     ),
  // },
  {
    field: "network_details",
    headerName: "Network Details",
    flex: 2.5,
    renderCell: (params) => (
      <CellContainer>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            rowGap: "2px",
          }}
        >
          <Typography variant="subtitle2" component="span" fontWeight="bold">
            Network
          </Typography>
          <Typography variant="body2" component="span">
            {params.row.network_details?.network || ""}
          </Typography>
          <Typography variant="subtitle2" component="span" fontWeight="bold">
            IP Address
          </Typography>
          <Typography variant="body2" component="span">
            {params.row.network_details?.ip_address || ""}
          </Typography>
          <Typography variant="subtitle2" component="span" fontWeight="bold">
            MAC Address
          </Typography>
          <Typography variant="body2" component="span">
            {params.row.network_details?.mac_address || ""}
          </Typography>
        </div>
      </CellContainer>
    ),
  },
  {
    field: "os",
    headerName: "OS",
    flex: 1,
    renderCell: (params) => (
      <CellContainer>
        <Typography variant="body2">
          {params.value.name}
          <Typography variant="body2">({params.value.edition})</Typography>
        </Typography>
      </CellContainer>
    ),
  },
  // { field: "version", headerName: "Version", flex: 1 },
];

const paginationModel = { page: 0, pageSize: 5 };

interface VmsSelectionStepProps {
  vms: VmData[];
  onChange: (id: string) => (value: unknown) => void;
  error: string;
  loadingVms?: boolean;
}

export default function VmsSelectionStep({
  vms = [],
  onChange,
  error,
  loadingVms = false,
}: VmsSelectionStepProps) {
  const handleVmSelection = (selectedRowIds: GridRowSelectionModel) => {
    const selectedVms = vms.filter((vm) => selectedRowIds.includes(vm.name));
    onChange("vms")(selectedVms);
  };

  const vmsMockData = [
    {
      id: 1,
      name: "VM 1",
      status: "Running",
      ip: "89.207.132.170",
      network_details: {
        network: "POD Network",
        ip_address: "89.207.132.170",
        mac_address: "00:00:00:00:00:00",
      },
      os: {
        name: "Windows",
        edition: "Enterprise",
      },
      version: "kubevert-c7",
    },
    {
      id: 2,
      name: "VM 2",
      status: "Stopped",
      ip: "",
      network_details: {
        network: "POD Network",
        ip_address: "89.207.132.170",
        mac_address: "00:00:00:00:00:00",
      },
      os: {
        name: "Windows",
        edition: "Enterprise",
      },
      version: "kubevert-c7",
    },
    {
      id: 3,
      name: "VM 3",
      status: "Running",
      ip: "89.207.132.170",
      network_details: {
        network: "POD Network",
        ip_address: "89.207.132.170",
        mac_address: "00:00:00:00:00:00",
      },
      os: {
        name: "Windows",
        edition: "Enterprise",
      },
      version: "kubevert-c7",
    },
  ];

  return (
    <VmsSelectionStepContainer>
      <Step stepNumber="2" label="Select Virtual Machines to Migrate" />
      <FieldsContainer>
        <FormControl error={!!error} required>
          <Paper sx={{ width: "100%", height: 500 }}>
            <DataGrid
              rows={vmsMockData}
              columns={columns}
              initialState={{ pagination: { paginationModel } }}
              pageSizeOptions={[5, 10, 25]}
              localeText={{ noRowsLabel: "No VMs discovered" }}
              rowHeight={90}
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
  );
}
