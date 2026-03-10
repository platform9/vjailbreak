import { Box, Alert } from '@mui/material'
import { Step, RHFAutocomplete } from 'src/shared/components/forms'
import { FormGrid } from 'src/components'
import {
  OpenstackCreds,
  SecurityGroupOption,
  ServerGroupOption,
  PCDNetworkInfo
} from 'src/api/openstack-creds/model'
import { useMemo } from 'react'
import { ResourceMap } from './NetworkAndStorageMappingStep'

interface SecurityGroupAndServerGroupProps {
  params: {
    vms?: any[]
    securityGroups?: string[]
    serverGroup?: string
    networkMappings?: ResourceMap[]
  }
  onChange: (key: string) => (value: any) => void
  openstackCredentials?: OpenstackCreds
  openstackNetworks?: PCDNetworkInfo[]
  stepNumber?: string
  showHeader?: boolean
}

export default function SecurityGroupAndServerGroup({
  params,
  onChange,
  openstackCredentials,
  openstackNetworks = [],
  stepNumber = '4',
  showHeader = true
}: SecurityGroupAndServerGroupProps) {
  const securityGroupOptions: SecurityGroupOption[] =
    openstackCredentials?.status?.openstack?.securityGroups || []

  const serverGroupOptions: ServerGroupOption[] =
    openstackCredentials?.status?.openstack?.serverGroups || []

  // Check if any selected network has the "simple_network" tag (L2 network)
  const hasL2Network = useMemo(() => {
    if (!params.networkMappings || params.networkMappings.length === 0) {
      return false
    }

    // Get all target network names from mappings
    const targetNetworkNames = params.networkMappings.map((mapping) => mapping.target)

    // Check if any of these networks has the "simple_network" tag
    return openstackNetworks.some(
      (network) =>
        targetNetworkNames.includes(network.name) &&
        network.tags &&
        network.tags.includes('simple_network')
    )
  }, [params.networkMappings, openstackNetworks])

  return (
    <Box>
      {showHeader ? (
        <Step stepNumber={stepNumber} label="Security Groups & Server Group (Optional)" />
      ) : null}
      <Box>
        {hasL2Network && (
          <Alert severity="info" sx={{ mb: 2 }}>
            Security Groups are not available when using L2 networks (networks with "simple_network" tag).
            L2 networks do not support security group assignment.
          </Alert>
        )}
        <FormGrid minWidth={320} gap={3}>
          <Box>
            <RHFAutocomplete<SecurityGroupOption>
              name="securityGroups"
              multiple
              options={securityGroupOptions}
              label="Security Groups"
              placeholder={
                params.securityGroups && params.securityGroups.length > 0
                  ? ''
                  : 'Select Security Groups'
              }
              getOptionLabel={(option) =>
                option.requiresIdDisplay
                  ? `${option.name} (${option.id.substring(0, 8)}...)`
                  : option.name
              }
              getOptionValue={(option) => option.id}
              renderOptionLabel={(option) =>
                option.requiresIdDisplay
                  ? `${option.name} (${option.id.substring(0, 8)}...)`
                  : option.name
              }
              showCheckboxes
              onValueChange={(value) => onChange('securityGroups')(value)}
              data-testid="security-groups-autocomplete"
              labelProps={{ tooltip: 'Assign security groups to the selected VMs.' }}
              disabled={hasL2Network}
            />
          </Box>

          {/* Right side: Server Group */}
          <Box>
            <RHFAutocomplete<ServerGroupOption>
              name="serverGroup"
              options={serverGroupOptions}
              label="Server Group"
              placeholder="Select Server Group"
              getOptionLabel={(option) => `${option.name} (${option.policy})`}
              getOptionValue={(option) => option.id}
              renderOptionLabel={(option) => `${option.name} (${option.policy})`}
              onValueChange={(value) => onChange('serverGroup')(value)}
              data-testid="server-group-autocomplete"
              labelProps={{ tooltip: 'Control VM affinity/anti-affinity placement.' }}
            />
          </Box>
        </FormGrid>
      </Box>
    </Box>
  )
}
