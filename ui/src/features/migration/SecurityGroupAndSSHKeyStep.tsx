import { Box, Typography } from "@mui/material";
import Step from "../../components/forms/Step";
import Autocomplete from '@mui/material/Autocomplete';
import TextField from '@mui/material/TextField';
import Checkbox from '@mui/material/Checkbox';
import { OpenstackCreds, SecurityGroupOption } from "src/api/openstack-creds/model";

interface SecurityGroupAndSSHKeyStepProps {
  params: {
    vms?: any[];
    securityGroups?: string[];
  };
  onChange: (key: string) => (value: any) => void;
  openstackCredentials?: OpenstackCreds;
  stepNumber?: string;
}

export default function SecurityGroupAndSSHKeyStep({
  params,
  onChange,
  openstackCredentials,
  stepNumber = "4",
}: SecurityGroupAndSSHKeyStepProps) {
  const securityGroupOptions: SecurityGroupOption[] =
    openstackCredentials?.status?.openstack?.securityGroups || [];

  return (
    <Box>
      <Step stepNumber={stepNumber} label="Security Groups (Optional)" />
      <Box sx={{ ml: 6 }}>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Assign security groups to the selected VMs.
        </Typography>
        <Box>
          <Autocomplete
            multiple
            options={securityGroupOptions}
            getOptionLabel={(option) =>
              option.requiresIdDisplay
                ? `${option.name} (${option.id.substring(0, 8)}...)`
                : option.name
            }
            isOptionEqualToValue={(option, value) => option.id === value.id}
            value={
              securityGroupOptions.filter(option =>
                (params.securityGroups || []).includes(option.id)
              )
            }
            onChange={(_, value) => {
              const selectedIds = value.map(option => option.id);
              onChange("securityGroups")(selectedIds);
            }}
            renderInput={(inputParams) => (
              <TextField
                {...inputParams}
                label="Security Groups"
                placeholder={params.securityGroups && params.securityGroups.length > 0 ? "" : "Select Security Groups"}
                size="small"
              />
            )}
            renderTags={(value, getTagProps) =>
              value.map((option, index) => (
                <span
                  {...getTagProps({ index })}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    background: '#444',
                    color: '#fff',
                    borderRadius: 12,
                    fontSize: 13,
                    height: 24,
                    marginRight: 4,
                    padding: '0 8px',
                  }}
                >
                  {option.requiresIdDisplay
                    ? `${option.name} (${option.id.substring(0, 8)}...)`
                    : option.name}
                  <span
                    style={{ marginLeft: 4, cursor: 'pointer' }}
                    onClick={() => {
                      const currentIds = value.map(v => v.id);
                      currentIds.splice(index, 1);
                      onChange("securityGroups")(currentIds);
                    }}
                  >
                    ×
                  </span>
                </span>
              ))
            }
            renderOption={(props, option, { selected }) => (
              <li {...props}>
                <Checkbox style={{ marginRight: 8 }} checked={selected} size="small" />
                {option.requiresIdDisplay
                  ? `${option.name} (${option.id.substring(0, 8)}...)`
                  : option.name}
              </li>
            )}
            disableCloseOnSelect
            size="small"
            sx={{ width: '100%' }}
          />
        </Box>
      </Box>
    </Box>
  );
}