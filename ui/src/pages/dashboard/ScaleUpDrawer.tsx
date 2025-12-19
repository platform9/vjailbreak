import {
  Box,
  FormControl,
  FormLabel,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
  IconButton,
  Tooltip,
  CircularProgress,
  InputAdornment,
  Checkbox,
  ListItemText
} from '@mui/material'
import React, { useState, useCallback, useEffect } from 'react'
import Step from 'src/components/forms/Step'
import { StyledDrawer, DrawerContent } from 'src/components/forms/StyledDrawer'
import Header from 'src/components/forms/Header'
import Footer from 'src/components/forms/Footer'
import OpenstackCredentialsForm from 'src/components/forms/OpenstackCredentialsForm'
import InfoIcon from '@mui/icons-material/Info'
import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { OpenstackCreds } from 'src/api/openstack-creds/model'
import { createNodes } from 'src/api/nodes/nodeMappings'
import { ArrowDropDownIcon } from '@mui/x-date-pickers/icons'
import { OpenstackFlavor } from 'src/api/openstack-creds/model'
import SearchIcon from '@mui/icons-material/Search'
import { NodeItem } from 'src/api/nodes/model'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'
import axios from 'axios'
import { useKeyboardSubmit } from 'src/hooks/ui/useKeyboardSubmit'

// Mock data - replace with actual data from API

interface ScaleUpDrawerProps {
  open: boolean
  onClose: () => void
  masterNode: NodeItem | null
}

const StepHeader = ({
  number,
  label,
  tooltip
}: {
  number: string
  label: string
  tooltip: string
}) => (
  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
    <Step stepNumber={number} label={label} sx={{ mb: 0 }} />
    <Tooltip title={tooltip} arrow>
      <IconButton size="small" color="info">
        <InfoIcon />
      </IconButton>
    </Tooltip>
  </Box>
)

export default function ScaleUpDrawer({ open, onClose, masterNode }: ScaleUpDrawerProps) {
  const [openstackCredentials, setOpenstackCredentials] = useState<OpenstackCreds | null>(null)
  const [nodeCount, setNodeCount] = useState(1)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selectedOpenstackCred, setSelectedOpenstackCred] = useState<string | null>(null)
  const [openstackError, setOpenstackError] = useState<string | null>(null)

  const [flavors, setFlavors] = useState<Array<OpenstackFlavor>>([])
  const [selectedFlavor, setSelectedFlavor] = useState('')
  const [loadingFlavors, setLoadingFlavors] = useState(false)
  const [flavorsError, setFlavorsError] = useState<string | null>(null)
  const [flavorSearchTerm, setFlavorSearchTerm] = useState('')

  const [volumeTypes, setVolumeTypes] = useState<Array<string>>([])
  const [selectedVolumeType, setSelectedVolumeType] = useState('')

  const [securityGroups, setSecurityGroups] = useState<Array<{ name: string; id: string }>>([])
  const [selectedSecurityGroups, setSelectedSecurityGroups] = useState<string[]>([])
  const [useMasterSecurityGroups, setUseMasterSecurityGroups] = useState(true)

  // Filter flavors based on search term
  const filteredFlavors = React.useMemo(() => {
    return flavors.filter(
      (flavor) =>
        flavor.name.toLowerCase().includes(flavorSearchTerm.toLowerCase()) ||
        `${flavor.vcpus} vCPU`.toLowerCase().includes(flavorSearchTerm.toLowerCase()) ||
        `${flavor.ram / 1024}GB RAM`.toLowerCase().includes(flavorSearchTerm.toLowerCase()) ||
        `${flavor.disk}GB disk`.toLowerCase().includes(flavorSearchTerm.toLowerCase())
    )
  }, [flavors, flavorSearchTerm])

  // Fetch credentials list
  const { data: openstackCredsList = [], isLoading: loadingOpenstackCreds } =
    useOpenstackCredentialsQuery()

  const openstackCredsValidated =
    openstackCredentials?.status?.openstackValidationStatus === 'Succeeded'

  const clearStates = () => {
    setOpenstackCredentials(null)
    setSelectedOpenstackCred(null)
    setOpenstackError(null)
    setNodeCount(1)
    setError(null)
    setSelectedFlavor('')
    setFlavors([])
    setLoadingFlavors(false)
    setFlavorsError(null)
    setFlavorSearchTerm('')
    setVolumeTypes([])
    setSelectedVolumeType('')
    setSecurityGroups([])
    setSelectedSecurityGroups([])
    setUseMasterSecurityGroups(true)
  }

  // Reset state when drawer closes
  const handleClose = useCallback(() => {
    clearStates()
    onClose()
  }, [onClose])

  const handleOpenstackCredSelect = async (credId: string | null) => {
    setSelectedOpenstackCred(credId)

    if (credId) {
      try {
        const response = await getOpenstackCredentials(credId)
        setOpenstackCredentials(response)
      } catch (error) {
        console.error('Error fetching OpenStack credentials:', error)
        setOpenstackError(
          'Error fetching OpenStack credentials: ' +
            (axios.isAxiosError(error) ? error?.response?.data?.message : error)
        )
      }
    } else {
      setOpenstackCredentials(null)
    }
  }

  useEffect(() => {
    const fetchFlavours = async () => {
      if (openstackCredsValidated || openstackCredentials) {
        setLoadingFlavors(true)
        try {
          const flavours = openstackCredentials?.spec.flavors
          console.log(flavours)
          setFlavors(flavours || [])
        } catch (error) {
          console.error('Failed to fetch flavors:', error)
          setFlavorsError('Failed to fetch OpenStack flavors')
        } finally {
          setLoadingFlavors(false)
        }
      }
    }
    fetchFlavours()
  }, [openstackCredsValidated, openstackCredentials])

  useEffect(() => {
    if (openstackCredsValidated && openstackCredentials) {
      const types = openstackCredentials?.status?.openstack?.volumeTypes || []
      setVolumeTypes(types)
      // Set default to use master's volume type
      setSelectedVolumeType('USE_MASTER')

      const sgs = openstackCredentials?.status?.openstack?.securityGroups || []
      setSecurityGroups(sgs)
      // Default to using master's security groups
      setUseMasterSecurityGroups(true)
      setSelectedSecurityGroups([])
    } else {
      setVolumeTypes([])
      setSelectedVolumeType('')
      setSecurityGroups([])
      setSelectedSecurityGroups([])
      setUseMasterSecurityGroups(true)
    }
  }, [openstackCredsValidated, openstackCredentials])

  const handleSubmit = async () => {
    if (
      !masterNode?.spec.openstackImageID ||
      !selectedFlavor ||
      !nodeCount ||
      !openstackCredentials?.metadata?.name
    ) {
      setError('Please fill in all required fields')
      return
    }

    try {
      setLoading(true)
      await createNodes({
        imageId: masterNode.spec.openstackImageID,
        openstackCreds: {
          kind: 'openstackcreds' as const,
          name: openstackCredentials.metadata.name,
          namespace: 'migration-system'
        },
        count: nodeCount,
        flavorId: selectedFlavor,
        volumeType: selectedVolumeType === 'USE_MASTER' ? undefined : selectedVolumeType,
        securityGroups: useMasterSecurityGroups ? undefined : selectedSecurityGroups
      })

      handleClose()
    } catch (error) {
      console.error('Error scaling up nodes:', error)
      setError(error instanceof Error ? error.message : 'Failed to scale up nodes')
    } finally {
      setLoading(false)
    }
  }

  useKeyboardSubmit({
    open,
    isSubmitDisabled: !masterNode || !selectedFlavor || loading || !openstackCredsValidated,
    onSubmit: handleSubmit,
    onClose: handleClose
  })

  return (
    <StyledDrawer anchor="right" open={open} onClose={handleClose}>
      <Header title="Scale Up Agents" />
      <DrawerContent>
        <Box sx={{ display: 'grid', gap: 4 }}>
          {/* Step 1: OpenStack Credentials */}
          <div>
            <StepHeader
              number="1"
              label="OpenStack Credentials"
              tooltip="Select existing OpenStack credentials to authenticate with the OpenStack platform where new nodes will be created."
            />
            <Box sx={{ ml: 6, mt: 2 }}>
              <FormControl fullWidth error={!!openstackError} required>
                <OpenstackCredentialsForm
                  fullWidth={true}
                  size="small"
                  credentialsList={openstackCredsList}
                  loadingCredentials={loadingOpenstackCreds}
                  error={openstackError || ''}
                  onCredentialSelect={handleOpenstackCredSelect}
                  selectedCredential={selectedOpenstackCred}
                  showCredentialSelector={true}
                />
              </FormControl>
            </Box>
          </div>

          {/* Step 2: Agent Template */}
          <div>
            <StepHeader
              number="2"
              label="Agent Template"
              tooltip="Configure the specification for the new nodes."
            />
            <Box sx={{ ml: 6, mt: 2, display: 'grid', gap: 3 }}>
              <FormControl fullWidth>
                <TextField
                  label="Master Agent Image"
                  value={'Image selected from the first vjailbreak node'}
                  disabled
                  fullWidth
                  size="small"
                />
              </FormControl>
              <FormControl error={!!flavorsError} fullWidth>
                <InputLabel size="small">
                  {loadingFlavors ? 'Loading Flavors...' : 'Flavor'}
                </InputLabel>
                <Select
                  value={selectedFlavor}
                  label="Flavor"
                  onChange={(e) => setSelectedFlavor(e.target.value)}
                  required
                  size="small"
                  disabled={loadingFlavors || !openstackCredsValidated || !openstackCredentials}
                  IconComponent={
                    loadingFlavors
                      ? () => (
                          <CircularProgress
                            size={24}
                            sx={{ marginRight: 2, display: 'flex', alignItems: 'center' }}
                          />
                        )
                      : ArrowDropDownIcon
                  }
                  MenuProps={{
                    PaperProps: {
                      style: {
                        maxHeight: 300
                      }
                    }
                  }}
                >
                  <Box
                    sx={{
                      p: 1,
                      position: 'sticky',
                      top: 0,
                      bgcolor: 'background.paper',
                      zIndex: 1
                    }}
                  >
                    <TextField
                      size="small"
                      placeholder="Search flavors"
                      fullWidth
                      value={flavorSearchTerm}
                      onChange={(e) => {
                        e.stopPropagation()
                        setFlavorSearchTerm(e.target.value)
                      }}
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => e.stopPropagation()}
                      autoFocus
                      InputProps={{
                        startAdornment: (
                          <InputAdornment position="start">
                            <SearchIcon fontSize="small" />
                          </InputAdornment>
                        )
                      }}
                    />
                  </Box>
                  {flavors.length === 0 ? (
                    <MenuItem disabled>No flavors available</MenuItem>
                  ) : filteredFlavors.length === 0 ? (
                    <MenuItem disabled>No matching flavors found</MenuItem>
                  ) : (
                    filteredFlavors.map((flavor) => {
                      const isDisabled = flavor.disk < 60
                      return (
                        <MenuItem key={flavor.id} value={flavor.id} disabled={isDisabled}>
                          {`${flavor.name} (${flavor.vcpus} vCPU, ${flavor.ram / 1024}GB RAM, ${flavor.disk}GB disk)`}
                          {isDisabled && ' - Insufficient disk size'}
                        </MenuItem>
                      )
                    })
                  )}
                </Select>
                {flavorsError && (
                  <FormLabel error sx={{ mt: 1, fontSize: '0.75rem' }}>
                    {flavorsError}
                  </FormLabel>
                )}
              </FormControl>
              <FormControl fullWidth>
                <InputLabel size="small">Volume Type</InputLabel>
                <Select
                  value={selectedVolumeType}
                  label="Volume Type"
                  onChange={(e) => setSelectedVolumeType(e.target.value)}
                  size="small"
                  disabled={!openstackCredsValidated || !openstackCredentials}
                >
                  <MenuItem value="USE_MASTER">
                    Use volume type of primary VJB instance
                  </MenuItem>
                  {volumeTypes.map((volumeType) => (
                    <MenuItem key={volumeType} value={volumeType}>
                      {volumeType}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <FormControl fullWidth>
                <InputLabel size="small">Security Groups</InputLabel>
                <Select
                  multiple
                  value={useMasterSecurityGroups ? ['USE_MASTER'] : selectedSecurityGroups}
                  label="Security Groups"
                  onChange={(e) => {
                    const value = e.target.value as string[]
                    if (value.includes('USE_MASTER')) {
                      setUseMasterSecurityGroups(true)
                      setSelectedSecurityGroups([])
                    } else {
                      setUseMasterSecurityGroups(false)
                      setSelectedSecurityGroups(value)
                    }
                  }}
                  size="small"
                  disabled={!openstackCredsValidated || !openstackCredentials}
                  renderValue={(selected) => {
                    if (useMasterSecurityGroups) {
                      return 'Use security groups of primary VJB instance'
                    }
                    return (selected as string[])
                      .map((id) => securityGroups.find((sg) => sg.id === id)?.name || id)
                      .join(', ')
                  }}
                >
                  <MenuItem value="USE_MASTER">
                    <Checkbox checked={useMasterSecurityGroups} />
                    <ListItemText primary="Use security groups of primary VJB instance" />
                  </MenuItem>
                  {securityGroups.map((sg) => (
                    <MenuItem key={sg.id} value={sg.id} disabled={useMasterSecurityGroups}>
                      <Checkbox checked={selectedSecurityGroups.includes(sg.id)} />
                      <ListItemText primary={sg.name} />
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
            </Box>
          </div>

          {/* Step 3: Node Count */}
          <div>
            <StepHeader
              number="3"
              label="Agent Count"
              tooltip="Specify how many new nodes to create based on the above node template."
            />
            <Box sx={{ ml: 6, mt: 2 }}>
              <TextField
                type="number"
                label="Number of Agents"
                value={nodeCount}
                onChange={(e) => {
                  const value = parseInt(e.target.value)
                  if (value >= 1 && value <= 5) {
                    setNodeCount(value)
                  }
                }}
                inputProps={{ min: 1, max: 5 }}
                fullWidth
                size="small"
                helperText="Min: 1, Max: 5 nodes"
              />
            </Box>
          </div>

          {error && (
            <Typography color="error" sx={{ ml: 6 }}>
              {error}
            </Typography>
          )}
        </Box>
      </DrawerContent>
      <Footer
        submitButtonLabel="Scale Up"
        onClose={handleClose}
        onSubmit={handleSubmit}
        disableSubmit={!masterNode || !selectedFlavor || loading || !openstackCredsValidated}
        submitting={loading}
      />
    </StyledDrawer>
  )
}
