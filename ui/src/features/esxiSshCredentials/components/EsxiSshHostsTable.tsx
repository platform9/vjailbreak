import { GridColDef } from '@mui/x-data-grid'
import {
  Button,
  Box,
  Tooltip,
  Chip,
  Typography,
  MenuItem,
  Select,
  FormControl,
  InputLabel,
  SelectChangeEvent
} from '@mui/material'
import KeyIcon from '@mui/icons-material/Key'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import HourglassEmptyIcon from '@mui/icons-material/HourglassEmpty'
import ComputerIcon from '@mui/icons-material/Computer'
import { CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { CommonDataGrid } from 'src/components/grid'
import { useState, useCallback, useMemo } from 'react'
import { useForm } from 'react-hook-form'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { getSecret, upsertSecret } from 'src/api/secrets/secrets'
import { upsertESXiSSHCreds, getESXiSSHCreds } from 'src/api/esxi-ssh-creds'
import { getVMwareHosts } from 'src/api/vmware-hosts/vmwareHosts'
import { VMwareHost } from 'src/api/vmware-hosts/model'
import { ActionButton, DrawerFooter, DrawerHeader, DrawerShell } from 'src/components/design-system'
import { DesignSystemForm, RHFTextField } from 'src/shared/components/forms'
import { Alert, Snackbar } from '@mui/material'

interface HostRow {
  id: string
  name: string
  hostname: string
  clusterName: string
  sshStatus: string
  sshMessage: string
  esxiVersion: string
  lastChecked: string
  hostObject: VMwareHost
}

const getStatusColor = (status: string): 'success' | 'error' | 'warning' | 'default' => {
  if (!status) return 'default'
  const normalizedStatus = status.toLowerCase()
  if (normalizedStatus === 'succeeded') return 'success'
  if (normalizedStatus === 'failed') return 'error'
  if (normalizedStatus === 'pending' || normalizedStatus === 'validating') return 'warning'
  return 'default'
}

const getStatusIcon = (status: string) => {
  const normalizedStatus = (status || '').toLowerCase()
  if (normalizedStatus === 'succeeded') return <CheckCircleIcon fontSize="small" />
  if (normalizedStatus === 'failed') return <ErrorIcon fontSize="small" />
  return <HourglassEmptyIcon fontSize="small" />
}

const getColumns = (): GridColDef[] => [
  {
    field: 'hostname',
    headerName: 'Host',
    flex: 1.5,
    minWidth: 150
  },
  {
    field: 'clusterName',
    headerName: 'Cluster',
    flex: 1,
    minWidth: 120
  },
  {
    field: 'esxiVersion',
    headerName: 'ESXi Version',
    flex: 1,
    minWidth: 120,
    renderCell: (params) => params.value || '-'
  },
  {
    field: 'sshStatus',
    headerName: 'SSH Status',
    flex: 1,
    minWidth: 130,
    renderCell: (params) => (
      <Chip
        icon={getStatusIcon(params.value)}
        label={params.value || 'Pending'}
        variant="outlined"
        color={getStatusColor(params.value)}
        size="small"
        sx={{ borderRadius: '4px' }}
      />
    )
  },
  {
    field: 'sshMessage',
    headerName: 'Message',
    flex: 2,
    minWidth: 200,
    renderCell: (params) => {
      const message = params.value || ''
      const status = params.row.sshStatus?.toLowerCase()
      if (!message) return '-'
      return (
        <Tooltip title={message}>
          <Typography
            variant="body2"
            color={status === 'failed' ? 'error' : 'text.secondary'}
            sx={{
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
              maxWidth: '100%'
            }}
          >
            {message}
          </Typography>
        </Tooltip>
      )
    }
  },
  {
    field: 'lastChecked',
    headerName: 'Last Checked',
    flex: 1,
    minWidth: 150,
    renderCell: (params) => {
      if (!params.value) return '-'
      try {
        const date = new Date(params.value)
        return date.toLocaleString()
      } catch {
        return params.value
      }
    }
  }
]

interface CustomToolbarProps {
  onRefresh: () => void
  onConfigureKey: () => void
  isKeyConfigured: boolean
  statusFilter: string
  onStatusFilterChange: (status: string) => void
  overallStatus?: string
  totalHosts?: number
  successfulHosts?: number
  failedHosts?: number
}

const CustomToolbar = ({
  onRefresh,
  onConfigureKey,
  isKeyConfigured,
  statusFilter,
  onStatusFilterChange,
  overallStatus,
  totalHosts,
  successfulHosts
}: CustomToolbarProps) => {
  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      <FormControl size="small" sx={{ minWidth: 150 }}>
        <InputLabel id="status-filter-label">Status Filter</InputLabel>
        <Select
          labelId="status-filter-label"
          value={statusFilter}
          label="Status Filter"
          onChange={(e: SelectChangeEvent) => onStatusFilterChange(e.target.value)}
        >
          <MenuItem value="all">All</MenuItem>
          <MenuItem value="succeeded">Succeeded</MenuItem>
          <MenuItem value="failed">Failed</MenuItem>
          <MenuItem value="pending">Pending</MenuItem>
        </Select>
      </FormControl>
      <CustomSearchToolbar placeholder="Search by hostname or cluster" onRefresh={onRefresh} />
    </Box>
  )

  const actions = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      {overallStatus && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Chip
            icon={getStatusIcon(overallStatus)}
            label={`${successfulHosts || 0}/${totalHosts || 0} Validated`}
            variant="outlined"
            color={getStatusColor(overallStatus)}
            size="small"
            sx={{ borderRadius: '4px' }}
          />
        </Box>
      )}
      <Button
        variant={isKeyConfigured ? 'outlined' : 'contained'}
        color="primary"
        startIcon={<KeyIcon />}
        onClick={onConfigureKey}
        sx={{ height: 40 }}
      >
        {isKeyConfigured ? 'Edit SSH Key' : 'Configure SSH Key'}
      </Button>
    </Box>
  )

  return (
    <ListingToolbar title="ESXi SSH Credentials" icon={<ComputerIcon />} search={search} actions={actions} />
  )
}

export default function EsxiSshHostsTable() {
  const { reportError } = useErrorHandler({ component: 'EsxiSshHostsTable' })

  const [esxiKeyDrawerOpen, setEsxiKeyDrawerOpen] = useState(false)
  const [esxiKeyError, setEsxiKeyError] = useState<string | null>(null)
  const [esxiKeyToastOpen, setEsxiKeyToastOpen] = useState(false)
  const [esxiKeyToastMessage, setEsxiKeyToastMessage] = useState<string>('')
  const [esxiKeyToastSeverity, setEsxiKeyToastSeverity] = useState<'success' | 'error'>('success')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  type EsxiKeyFormData = {
    sshPrivateKey: string
  }

  const esxiKeyForm = useForm<EsxiKeyFormData>({
    defaultValues: {
      sshPrivateKey: ''
    }
  })

  const {
    reset: resetEsxiKeyForm,
    setValue: setEsxiKeyValue
  } = esxiKeyForm

  const {
    data: esxiKeySecret,
    isLoading: isEsxiKeyLoading,
    refetch: refetchEsxiKey
  } = useQuery({
    queryKey: ['secret', 'migration-system', 'esxi-ssh-key'],
    queryFn: async () => {
      try {
        return await getSecret('esxi-ssh-key', 'migration-system')
      } catch (error: any) {
        if (error?.response?.status === 404) {
          return null
        }
        throw error
      }
    },
    retry: false
  })

  const {
    data: esxiSSHCredsData,
    isLoading: isEsxiSSHCredsLoading,
    refetch: refetchEsxiSSHCreds
  } = useQuery({
    queryKey: ['esxisshcreds', 'migration-system'],
    queryFn: async () => {
      try {
        const result = await getESXiSSHCreds('migration-system')
        return result.items?.[0] || null
      } catch (error: any) {
        if (error?.response?.status === 404) {
          return null
        }
        throw error
      }
    },
    retry: false,
    refetchInterval: 30000
  })

  const {
    data: vmwareHosts,
    isLoading: isVMwareHostsLoading,
    refetch: refetchVMwareHosts
  } = useQuery({
    queryKey: ['vmwarehosts', 'migration-system'],
    queryFn: async () => {
      try {
        const result = await getVMwareHosts('migration-system')
        return result.items || []
      } catch (error: any) {
        if (error?.response?.status === 404) {
          return []
        }
        throw error
      }
    },
    retry: false,
    refetchInterval: 30000
  })

  const { mutateAsync: saveEsxiKey, isPending: isSavingEsxiKey } = useMutation({
    mutationFn: async (keyContent: string) => {
      await upsertSecret('esxi-ssh-key', { 'ssh-privatekey': keyContent }, 'migration-system')
      await upsertESXiSSHCreds('esxi-ssh-creds', 'esxi-ssh-key', 'root', 'migration-system')
    },
    onSuccess: () => {
      refetchEsxiKey()
      refetchEsxiSSHCreds()
      refetchVMwareHosts()
    }
  })

  const isEsxiKeyConfigured = !!esxiKeySecret?.metadata?.name

  const rows: HostRow[] = useMemo(() => {
    if (!vmwareHosts) return []
    return vmwareHosts.map((host: VMwareHost) => ({
      id: host.metadata.name,
      name: host.metadata.name,
      hostname: host.spec.name,
      clusterName: host.spec.clusterName || '-',
      sshStatus: host.status?.sshStatus || 'Pending',
      sshMessage: host.status?.sshMessage || '',
      esxiVersion: host.status?.esxiVersion || '',
      lastChecked: host.status?.sshLastChecked || '',
      hostObject: host
    }))
  }, [vmwareHosts])

  const filteredRows = useMemo(() => {
    if (statusFilter === 'all') return rows
    return rows.filter((row) => row.sshStatus.toLowerCase() === statusFilter.toLowerCase())
  }, [rows, statusFilter])

  const handleRefresh = useCallback(() => {
    refetchEsxiKey()
    refetchEsxiSSHCreds()
    refetchVMwareHosts()
  }, [refetchEsxiKey, refetchEsxiSSHCreds, refetchVMwareHosts])

  const validateOpenSshPrivateKey = (value: string): string | null => {
    const trimmed = value.trim()
    if (!trimmed) return 'SSH private key is required'
    if (/^ssh-privatekey\s*:/m.test(trimmed)) {
      return 'Paste only the key content (do not include "ssh-privatekey:")'
    }
    const hasBegin = /-----BEGIN (OPENSSH |RSA )?PRIVATE KEY-----/.test(trimmed)
    const hasEnd = /-----END (OPENSSH |RSA )?PRIVATE KEY-----/.test(trimmed)
    if (!hasBegin || !hasEnd) {
      return 'Invalid key format. Expected OpenSSH or RSA private key'
    }
    return null
  }

  const handleOpenEsxiKeyDrawer = () => {
    setEsxiKeyError(null)
    const existing = (esxiKeySecret as any)?.data?.['ssh-privatekey']
    resetEsxiKeyForm({ sshPrivateKey: typeof existing === 'string' ? existing : '' })
    setEsxiKeyDrawerOpen(true)
  }

  const handleCloseEsxiKeyDrawer = () => {
    if (isSavingEsxiKey) return
    setEsxiKeyDrawerOpen(false)
    setEsxiKeyError(null)
  }

  const handleCloseEsxiKeyToast = () => {
    setEsxiKeyToastOpen(false)
  }

  const handleEsxiKeyFileChange = async (file: File | null) => {
    if (!file) return
    const MAX_KEY_FILE_SIZE = 1024 * 1024
    if (file.size > MAX_KEY_FILE_SIZE) {
      setEsxiKeyError('File is too large. SSH private key files should be less than 1 MB.')
      return
    }
    try {
      const text = await file.text()
      setEsxiKeyValue('sshPrivateKey', text, { shouldDirty: true })
      setEsxiKeyError(null)
    } catch (error) {
      setEsxiKeyError('Failed to read file')
    }
  }

  const onSubmitEsxiKey = async (data: EsxiKeyFormData) => {
    const validationError = validateOpenSshPrivateKey(data.sshPrivateKey)
    if (validationError) {
      setEsxiKeyError(validationError)
      return
    }

    try {
      setEsxiKeyError(null)
      await saveEsxiKey(data.sshPrivateKey.trim())
      setEsxiKeyToastSeverity('success')
      setEsxiKeyToastMessage('ESXi SSH key saved successfully.')
      setEsxiKeyToastOpen(true)
      setEsxiKeyDrawerOpen(false)
    } catch (error: any) {
      const errorMessage =
        error?.response?.data?.message || error?.message || 'Failed to save ESXi SSH key'
      setEsxiKeyError(errorMessage)
      setEsxiKeyToastSeverity('error')
      setEsxiKeyToastMessage(errorMessage)
      setEsxiKeyToastOpen(true)
      reportError(error, { context: 'save-esxi-ssh-key' })
    }
  }

  const isLoading = isEsxiKeyLoading || isEsxiSSHCredsLoading || isVMwareHostsLoading

  const tableColumns = getColumns()

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      {/* SSH Key Status Banner */}
      {!isEsxiKeyConfigured && !isEsxiKeyLoading && (
        <Box
          sx={{
            mx: 2,
            mt: 2,
            mb: 1,
            px: 2,
            py: 1.5,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 2,
            border: '1px solid',
            borderColor: 'warning.main',
            borderRadius: 1,
            backgroundColor: 'warning.light',
            opacity: 0.9
          }}
        >
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <KeyIcon color="warning" />
            <Typography variant="body2" fontWeight={500}>
              SSH key not configured. Configure an SSH private key to validate ESXi host connectivity.
            </Typography>
          </Box>
          <Button
            variant="contained"
            color="warning"
            size="small"
            onClick={handleOpenEsxiKeyDrawer}
          >
            Configure Now
          </Button>
        </Box>
      )}

      <CommonDataGrid
        rows={filteredRows}
        columns={tableColumns}
        disableRowSelectionOnClick
        initialState={{
          sorting: {
            sortModel: [{ field: 'hostname', sort: 'asc' }]
          },
          pagination: {
            paginationModel: {
              pageSize: 25
            }
          }
        }}
        slots={{
          toolbar: () => (
            <CustomToolbar
              onRefresh={handleRefresh}
              onConfigureKey={handleOpenEsxiKeyDrawer}
              isKeyConfigured={isEsxiKeyConfigured}
              statusFilter={statusFilter}
              onStatusFilterChange={setStatusFilter}
              overallStatus={esxiSSHCredsData?.status?.validationStatus}
              totalHosts={esxiSSHCredsData?.status?.totalHosts || rows.length}
              successfulHosts={esxiSSHCredsData?.status?.successfulHosts}
              failedHosts={esxiSSHCredsData?.status?.failedHosts}
            />
          )
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        loading={isLoading}
        emptyMessage={
          isEsxiKeyConfigured
            ? 'No ESXi hosts discovered. Hosts will appear once VMware credentials are configured.'
            : 'Configure an SSH key to start validating ESXi hosts.'
        }
        sx={{
          '& .MuiDataGrid-main': {
            overflow: 'auto'
          },
          '& .MuiDataGrid-cell:focus': {
            outline: 'none'
          }
        }}
      />

      {/* SSH Key Configuration Drawer */}
      <DrawerShell open={esxiKeyDrawerOpen} onClose={handleCloseEsxiKeyDrawer}>
        <DrawerHeader title={isEsxiKeyConfigured ? 'Edit ESXi SSH Key' : 'Configure ESXi SSH Key'} />
        <DesignSystemForm form={esxiKeyForm} onSubmit={onSubmitEsxiKey}>
          <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
            <Typography variant="body2" color="text.secondary">
              Provide the SSH private key used to connect to ESXi hosts. This key will be used to
              validate SSH connectivity and retrieve ESXi version information.
            </Typography>
            <RHFTextField
              name="sshPrivateKey"
              label="SSH Private Key"
              multiline
              minRows={8}
              maxRows={16}
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
            />
            {esxiKeyError && (
              <Alert severity="error" onClose={() => setEsxiKeyError(null)}>
                {esxiKeyError}
              </Alert>
            )}
            <Box>
              <Button variant="outlined" component="label" size="small">
                Upload Key File
                <input
                  type="file"
                  hidden
                  accept=".pem,.key,*"
                  onChange={(e) => handleEsxiKeyFileChange(e.target.files?.[0] || null)}
                />
              </Button>
            </Box>
          </Box>
          <DrawerFooter>
            <ActionButton variant="outlined" onClick={handleCloseEsxiKeyDrawer} disabled={isSavingEsxiKey}>
              Cancel
            </ActionButton>
            <ActionButton type="submit" variant="contained" loading={isSavingEsxiKey}>
              {isEsxiKeyConfigured ? 'Update Key' : 'Save Key'}
            </ActionButton>
          </DrawerFooter>
        </DesignSystemForm>
      </DrawerShell>

      {/* Toast Notification */}
      <Snackbar
        open={esxiKeyToastOpen}
        autoHideDuration={6000}
        onClose={handleCloseEsxiKeyToast}
        anchorOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <Alert
          onClose={handleCloseEsxiKeyToast}
          severity={esxiKeyToastSeverity}
          variant="filled"
        >
          {esxiKeyToastMessage}
        </Alert>
      </Snackbar>
    </div>
  )
}
