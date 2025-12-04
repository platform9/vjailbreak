import { Box, Typography, Grid } from '@mui/material'
import Step from '../../components/forms/Step'
import Autocomplete from '@mui/material/Autocomplete'
import TextField from '@mui/material/TextField'
import Checkbox from '@mui/material/Checkbox'
import { OpenstackCreds, SecurityGroupOption, ServerGroupOption } from 'src/api/openstack-creds/model'

interface SecurityGroupAndServerGroupProps {
  params: {
    vms?: any[]
    securityGroups?: string[]
    serverGroup?: string
  }
  onChange: (key: string) => (value: any) => void
  openstackCredentials?: OpenstackCreds
  stepNumber?: string
}

export default function SecurityGroupAndServerGroup({
  params,
  onChange,
  openstackCredentials,
  stepNumber = '4'
}: SecurityGroupAndServerGroupProps) {
  const securityGroupOptions: SecurityGroupOption[] =
    openstackCredentials?.status?.openstack?.securityGroups || []
  
  const serverGroupOptions: ServerGroupOption[] =
    openstackCredentials?.status?.openstack?.serverGroups || []

  return (
    <Box>
      <Step stepNumber={stepNumber} label="Security Groups & Server Group (Optional)" />
      <Box sx={{ ml: 6 }}>
        <Grid container spacing={3}>
          {/* Left side: Security Groups */}
          <Grid item xs={6}>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Security Groups
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Assign security groups to the selected VMs.
            </Typography>
            <Autocomplete
            multiple
            options={securityGroupOptions}
            getOptionLabel={(option) =>
              option.requiresIdDisplay
                ? `${option.name} (${option.id.substring(0, 8)}...)`
                : option.name
            }
            isOptionEqualToValue={(option, value) => option.id === value.id}
            value={securityGroupOptions.filter((option) =>
              (params.securityGroups || []).includes(option.id)
            )}
            onChange={(_, value) => {
              const selectedIds = value.map((option) => option.id)
              onChange('securityGroups')(selectedIds)
            }}
            renderInput={(inputParams) => (
              <TextField
                {...inputParams}
                label="Security Groups"
                placeholder={
                  params.securityGroups && params.securityGroups.length > 0
                    ? ''
                    : 'Select Security Groups'
                }
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
                    padding: '0 8px'
                  }}
                >
                  {option.requiresIdDisplay
                    ? `${option.name} (${option.id.substring(0, 8)}...)`
                    : option.name}
                  <span
                    style={{ marginLeft: 4, cursor: 'pointer' }}
                    onClick={() => {
                      const currentIds = value.map((v) => v.id)
                      currentIds.splice(index, 1)
                      onChange('securityGroups')(currentIds)
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
          </Grid>

          {/* Right side: Server Group */}
          <Grid item xs={6}>
            <Typography variant="subtitle2" sx={{ mb: 1 }}>
              Server Group
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Control VM affinity/anti-affinity placement.
            </Typography>
            <Autocomplete
              options={serverGroupOptions}
              getOptionLabel={(option) => 
                `${option.name} (${option.policy})`
              }
              isOptionEqualToValue={(option, value) => option.id === value.id}
              value={serverGroupOptions.find(opt => opt.id === params.serverGroup) || null}
              onChange={(_, value) => {
                onChange('serverGroup')(value?.id || '')
              }}
              renderInput={(inputParams) => (
                <TextField
                  {...inputParams}
                  label="Server Group"
                  placeholder="Select Server Group"
                  size="small"
                />
              )}
              renderOption={(props, option) => (
                <li {...props}>
                  {option.name} ({option.policy})
                </li>
              )}
              size="small"
              sx={{ width: '100%' }}
            />
          </Grid>
        </Grid>
      </Box>
    </Box>
  )
}
