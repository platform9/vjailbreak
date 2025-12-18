import { Box, Chip } from '@mui/material'
import { Step } from 'src/shared/components/forms'
import { SectionHeader, Row } from 'src/components'
import Autocomplete from '@mui/material/Autocomplete'
import TextField from '@mui/material/TextField'
import Checkbox from '@mui/material/Checkbox'
import {
  OpenstackCreds,
  SecurityGroupOption,
  ServerGroupOption
} from 'src/api/openstack-creds/model'

interface SecurityGroupAndServerGroupProps {
  params: {
    vms?: any[]
    securityGroups?: string[]
    serverGroup?: string
  }
  onChange: (key: string) => (value: any) => void
  openstackCredentials?: OpenstackCreds
  stepNumber?: string
  showHeader?: boolean
}

export default function SecurityGroupAndServerGroup({
  params,
  onChange,
  openstackCredentials,
  stepNumber = '4',
  showHeader = true
}: SecurityGroupAndServerGroupProps) {
  const securityGroupOptions: SecurityGroupOption[] =
    openstackCredentials?.status?.openstack?.securityGroups || []

  const serverGroupOptions: ServerGroupOption[] =
    openstackCredentials?.status?.openstack?.serverGroups || []

  return (
    <Box>
      {showHeader ? (
        <Step stepNumber={stepNumber} label="Security Groups & Server Group (Optional)" />
      ) : null}
      <Box>
        <Row gap={3} flexWrap="wrap">
          {/* Left side: Security Groups */}
          <Box sx={{ flex: 1, minWidth: 300 }}>
            <SectionHeader
              title="Security Groups"
              subtitle="Assign security groups to the selected VMs."
            />
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
              renderTags={(value) =>
                value.map((option, index) => (
                  <Chip
                    key={index}
                    label={
                      option.requiresIdDisplay
                        ? `${option.name} (${option.id.substring(0, 8)}...)`
                        : option.name
                    }
                    size="small"
                    onDelete={() => {
                      const currentIds = value.map((v) => v.id)
                      currentIds.splice(index, 1)
                      onChange('securityGroups')(currentIds)
                    }}
                  />
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

          {/* Right side: Server Group */}
          <Box sx={{ flex: 1, minWidth: 300 }}>
            <SectionHeader
              title="Server Group"
              subtitle="Control VM affinity/anti-affinity placement."
            />
            <Autocomplete
              options={serverGroupOptions}
              getOptionLabel={(option) => `${option.name} (${option.policy})`}
              isOptionEqualToValue={(option, value) => option.id === value.id}
              value={serverGroupOptions.find((opt) => opt.id === params.serverGroup) || null}
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
          </Box>
        </Row>
      </Box>
    </Box>
  )
}
