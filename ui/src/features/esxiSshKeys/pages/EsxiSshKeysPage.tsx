import { GridColDef } from '@mui/x-data-grid'
import { Box, Button, Chip, Tooltip, Typography } from '@mui/material'
import KeyIcon from '@mui/icons-material/Key'
import ComputerIcon from '@mui/icons-material/Computer'
import { useCallback, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getSecret } from 'src/api/secrets/secrets'
import { getVMwareHosts } from 'src/api/vmware-hosts/vmwareHosts'
import { VMwareHost } from 'src/api/vmware-hosts/model'
import { CommonDataGrid, CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import AddEsxiSshKeyDrawer from '../components/AddEsxiSshKeyDrawer'

interface HostRow {
  id: string
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

export default function EsxiSshKeysPage() {
  const { reportError } = useErrorHandler({ component: 'EsxiSshKeysPage' })
  const [drawerState, setDrawerState] = useState<
    | { open: false }
    | { open: true; mode: 'add' | 'edit'; initialValues?: { name: string; sshPrivateKey: string } }
  >({ open: false })
  const [statusFilter, setStatusFilter] = useState<string>('All')

  const SECRET_NAMESPACE = 'migration-system'
  const ESXI_SSH_KEY_SECRET_NAME = 'esxi-ssh-key'

  const {
    data: esxiSshKeySecret,
    isLoading: isEsxiSshKeyLoading,
    refetch: refetchEsxiSshKey
  } = useQuery({
    queryKey: ['secret', SECRET_NAMESPACE, ESXI_SSH_KEY_SECRET_NAME],
    queryFn: async () => {
      try {
        return await getSecret(ESXI_SSH_KEY_SECRET_NAME, SECRET_NAMESPACE)
      } catch (error: any) {
        if (error?.response?.status === 404) {
          return null
        }
        reportError(error as Error, { context: 'get-esxi-ssh-key-secret' })
        throw error
      }
    },
    retry: false
  })

  const {
    data: vmwareHosts,
    isLoading,
    refetch
  } = useQuery({
    queryKey: ['vmwarehosts', SECRET_NAMESPACE],
    queryFn: async () => {
      try {
        const result = await getVMwareHosts(SECRET_NAMESPACE)
        return result.items || []
      } catch (error: any) {
        if (error?.response?.status === 404) {
          return []
        }
        reportError(error as Error, { context: 'list-vmware-hosts' })
        throw error
      }
    },
    retry: false,
    refetchInterval: 30000
  })

  const rows: HostRow[] = useMemo(() => {
    const items = Array.isArray(vmwareHosts) ? vmwareHosts : []
    return items.map((host: VMwareHost) => ({
      id: host.metadata.name,
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
    switch (statusFilter) {
      case 'Succeeded':
        return rows.filter((row) => row.sshStatus.toLowerCase() === 'succeeded')
      case 'Failed':
        return rows.filter((row) => row.sshStatus.toLowerCase() === 'failed')
      case 'In Progress':
        return rows.filter((row) => {
          const normalized = row.sshStatus.toLowerCase()
          return normalized === 'pending' || normalized === 'validating'
        })
      case 'All':
      default:
        return rows
    }
  }, [rows, statusFilter])

  const handleRefresh = useCallback(() => {
    refetch()
    refetchEsxiSshKey()
  }, [refetch, refetchEsxiSshKey])

  const isKeyConfigured = !!esxiSshKeySecret?.metadata?.name

  const handleOpenConfigure = () => {
    const existingKeyContent = (esxiSshKeySecret as any)?.data?.['ssh-privatekey']
    setDrawerState({
      open: true,
      mode: isKeyConfigured ? 'edit' : 'add',
      initialValues: {
        name: ESXI_SSH_KEY_SECRET_NAME,
        sshPrivateKey: typeof existingKeyContent === 'string' ? existingKeyContent : ''
      }
    })
  }

  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      <CustomSearchToolbar
        placeholder="Search by hostname or cluster"
        onRefresh={handleRefresh}
        disableRefresh={isLoading}
        onStatusFilterChange={setStatusFilter}
        currentStatusFilter={statusFilter}
      />
    </Box>
  )

  const actions = (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
      <Button
        variant={isKeyConfigured ? 'outlined' : 'contained'}
        color="primary"
        startIcon={<KeyIcon />}
        onClick={handleOpenConfigure}
        sx={{ height: 40 }}
      >
        {isKeyConfigured ? 'Edit SSH Key' : 'Configure SSH Key'}
      </Button>
    </Box>
  )

  const toolbar = (
    <ListingToolbar
      title="ESXi SSH Credentials"
      icon={<ComputerIcon />}
      search={search}
      actions={actions}
    />
  )

  const columns = useMemo(() => getColumns(), [])

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={filteredRows}
        columns={columns}
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
          toolbar: () => toolbar
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        loading={isLoading || isEsxiSshKeyLoading}
        emptyMessage="No ESXi hosts discovered"
      />

      {drawerState.open && (
        <AddEsxiSshKeyDrawer
          open
          mode={drawerState.mode}
          fixedName={ESXI_SSH_KEY_SECRET_NAME}
          initialValues={drawerState.initialValues}
          onClose={() => {
            setDrawerState({ open: false })
            refetch()
            refetchEsxiSshKey()
          }}
        />
      )}
    </div>
  )
}
