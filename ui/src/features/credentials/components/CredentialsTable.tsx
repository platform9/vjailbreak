import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import { Button, Box, IconButton, Tooltip, Chip } from '@mui/material'
import { keyframes } from '@mui/material/styles'
import DeleteIcon from '@mui/icons-material/DeleteOutlined'
import WarningIcon from '@mui/icons-material/Warning'
import AddIcon from '@mui/icons-material/Add'
import CredentialsIcon from '@mui/icons-material/VpnKey'
import SyncIcon from '@mui/icons-material/Sync'
import { CustomSearchToolbar, ListingToolbar } from 'src/components/grid'
import { CommonDataGrid } from 'src/components/grid'
import { useState, useCallback, useEffect, useRef } from 'react'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'
import { VmwareCredential } from './VmwareCredentialsForm'
import { OpenstackCredential } from './OpenstackCredentialsForm'
import { ConfirmationDialog } from 'src/components/dialogs'
import { useQueryClient, useMutation } from '@tanstack/react-query'
import { VMWARE_CREDS_QUERY_KEY } from 'src/hooks/api/useVmwareCredentialsQuery'
import { OPENSTACK_CREDS_QUERY_KEY } from 'src/hooks/api/useOpenstackCredentialsQuery'
import {
  deleteVMwareCredsWithSecretFlow,
  deleteOpenStackCredsWithSecretFlow,
  revalidateCredentials
} from 'src/api/helpers'
import VMwareCredentialsDrawer from './VMwareCredentialsDrawer'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import OpenstackCredentialsDrawer from './OpenstackCredentialsDrawer'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { useAmplitude } from 'src/hooks/useAmplitude'
import { AMPLITUDE_EVENTS } from 'src/types/amplitude'

interface CredentialItem {
  id: string
  name: string
  type: 'VMware' | 'OpenStack'
  status: string
  credObject: VmwareCredential | OpenstackCredential
}

const REVALIDATION_TIMEOUT_MS = 31 * 60 * 1000

const syncIconSpin = keyframes`
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
`

const normalizeCredentialStatus = (status?: string) => {
  if (!status) return 'Unknown'
  return status === 'Validating' ? 'Revalidating' : status
}

const getStatusColor = (status: string): 'success' | 'error' | 'warning' | 'info' | 'default' => {
  if (!status) return 'default'
  const normalizedStatus = status.toLowerCase()
  if (normalizedStatus === 'succeeded') {
    return 'success'
  }
  if (normalizedStatus === 'failed') {
    return 'error'
  }
  if (normalizedStatus === 'revalidating') {
    return 'info'
  }
  return 'default'
}

const getColumns = (
  onDeleteClick: (id: string, type: 'VMware' | 'OpenStack') => void,
  onRevalidateClick: (row: CredentialItem) => void,
  revalidatingId: string | null,
  timedOutRevalidatingId: string | null
): GridColDef[] => [
  {
    field: 'name',
    headerName: 'Name',
    flex: 1
  },
  {
    field: 'type',
    headerName: 'Type',
    flex: 1,
    renderCell: (params) => (
      <Chip
        label={params.value === 'OpenStack' ? 'PCD' : params.value}
        color={params.value === 'VMware' ? 'primary' : 'secondary'}
        variant="outlined"
        size="small"
      />
    )
  },
  {
    field: 'status',
    headerName: 'Status',
    flex: 1,
    renderCell: (params) => {
      const displayStatus =
        revalidatingId === params.row.id ? 'Revalidating' : normalizeCredentialStatus(params.value)
      return (
        <Chip
          label={displayStatus}
          variant="outlined"
          color={getStatusColor(displayStatus)}
          size="small"
        />
      )
    }
  },
  {
    field: 'actions',
    headerName: 'Actions',
    flex: 1,
    width: 100,
    sortable: false,
    renderCell: (params) => {
      const backendRevalidating = normalizeCredentialStatus(params.row.status) === 'Revalidating'
      const isRowLoading =
        revalidatingId === params.row.id ||
        (backendRevalidating && timedOutRevalidatingId !== params.row.id)

      return (
        <Box>
          <Tooltip title="Revalidate Credentials">
            <span>
              <IconButton
                onClick={(e) => {
                  e.stopPropagation()
                  onRevalidateClick(params.row)
                }}
                disabled={isRowLoading}
                aria-label="revalidate credential"
              >
                <SyncIcon
                  sx={
                    isRowLoading
                      ? {
                          animation: `${syncIconSpin} 1s linear infinite`
                        }
                      : undefined
                  }
                />
              </IconButton>
            </span>
          </Tooltip>

          <Tooltip title="Delete credential">
            <span>
              <IconButton
                onClick={(e) => {
                  e.stopPropagation()
                  onDeleteClick(params.row.id, params.row.type)
                }}
                aria-label="delete credential"
                disabled={isRowLoading}
              >
                <DeleteIcon />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      )
    }
  }
]

type CredentialType = 'vmware' | 'pcd'

interface CustomToolbarProps {
  title: string
  numSelected: number
  onDeleteSelected: () => void
  loading: boolean
  onRefresh: () => void
  onAddCredential: () => void
  actionLabel: string
  actionDataTour?: string
}

const CustomToolbar = ({
  title,
  numSelected,
  onDeleteSelected,
  loading,
  onRefresh,
  onAddCredential,
  actionLabel,
  actionDataTour
}: CustomToolbarProps) => {
  const search = (
    <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 2 }}>
      {numSelected > 0 && (
        <Button
          variant="outlined"
          color="error"
          startIcon={<DeleteIcon />}
          onClick={onDeleteSelected}
          disabled={loading}
          sx={{ height: 40 }}
        >
          Delete Selected ({numSelected})
        </Button>
      )}
      <CustomSearchToolbar placeholder="Search by Name or Type" onRefresh={onRefresh} />
    </Box>
  )

  const actions = (
    <Button
      variant="contained"
      color="primary"
      startIcon={<AddIcon />}
      onClick={onAddCredential}
      sx={{ height: 40 }}
      data-tour={actionDataTour}
    >
      {actionLabel}
    </Button>
  )

  return (
    <ListingToolbar title={title} icon={<CredentialsIcon />} search={search} actions={actions} />
  )
}

interface CredentialsTableProps {
  credentialType: CredentialType
}

export default function CredentialsTable({ credentialType }: CredentialsTableProps) {
  const { reportError } = useErrorHandler({ component: 'CredentialsTable' })
  const { track } = useAmplitude({ component: 'CredentialsTable' })
  const queryClient = useQueryClient()
  const [revalidatingId, setRevalidatingId] = useState<string | null>(null)
  const [timedOutRevalidatingId, setTimedOutRevalidatingId] = useState<string | null>(null)
  const revalidationStartDataUpdatedAtRef = useRef(0)
  const revalidationTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const isVmware = credentialType === 'vmware'

  const clearRevalidationTimeout = useCallback(() => {
    if (revalidationTimeoutRef.current) {
      clearTimeout(revalidationTimeoutRef.current)
      revalidationTimeoutRef.current = null
    }
  }, [])

  const clearActiveRevalidation = useCallback(() => {
    clearRevalidationTimeout()
    setRevalidatingId(null)
  }, [clearRevalidationTimeout])

  const {
    data: vmwareCredentials,
    isLoading: loadingVmware,
    refetch: refetchVmware,
    dataUpdatedAt: vmwareDataUpdatedAt
  } = useVmwareCredentialsQuery(undefined, {
    enabled: isVmware,
    staleTime: 0,
    refetchOnMount: true,
    refetchInterval: (query) => {
      const data = query.state.data as VmwareCredential[] | undefined
      const hasRevalidationInProgress = data?.some((cred) => {
        const credentialId = `vmware-${cred.metadata.name}`
        return (
          credentialId !== timedOutRevalidatingId &&
          normalizeCredentialStatus(cred.status?.vmwareValidationStatus) === 'Revalidating'
        )
      })
      return revalidatingId || hasRevalidationInProgress ? 5000 : false
    }
  })

  const {
    data: openstackCredentials,
    isLoading: loadingOpenstack,
    refetch: refetchOpenstack,
    dataUpdatedAt: openstackDataUpdatedAt
  } = useOpenstackCredentialsQuery(undefined, {
    enabled: !isVmware,
    staleTime: 0,
    refetchOnMount: true,
    refetchInterval: (query) => {
      const data = query.state.data as OpenstackCredential[] | undefined
      const hasRevalidationInProgress = data?.some((cred) => {
        const credentialId = `openstack-${cred.metadata.name}`
        return (
          credentialId !== timedOutRevalidatingId &&
          normalizeCredentialStatus(cred.status?.openstackValidationStatus) === 'Revalidating'
        )
      })
      return revalidatingId || hasRevalidationInProgress ? 5000 : false
    }
  })

  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false)
  const [selectedForDeletion, setSelectedForDeletion] = useState<CredentialItem[]>([])
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [vmwareCredDrawerOpen, setVmwareCredDrawerOpen] = useState(false)
  const [openstackCredDrawerOpen, setOpenstackCredDrawerOpen] = useState(false)

  const { mutate: revalidate, isPending: isRevalidationApiPending } = useMutation({
    mutationFn: revalidateCredentials,
    onSuccess: () => {
      setTimeout(() => {
        queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY })
        queryClient.invalidateQueries({ queryKey: OPENSTACK_CREDS_QUERY_KEY })
      }, 500)
    },
    onError: (error: unknown, variables) => {
      reportError(error instanceof Error ? error : new Error(String(error)), {
        context: 'credentials-revalidation',
        metadata: {
          credentialName: variables.name,
          credentialKind: variables.kind
        }
      })
      clearActiveRevalidation()
    }
  })

  const vmwareItems: CredentialItem[] =
    vmwareCredentials?.map((cred: VmwareCredential) => ({
      id: `vmware-${cred.metadata.name}`,
      name: cred.metadata.name,
      type: 'VMware' as const,
      status: cred.status?.vmwareValidationStatus || 'Unknown',
      credObject: cred
    })) || []

  const openstackItems: CredentialItem[] =
    openstackCredentials?.map((cred: OpenstackCredential) => ({
      id: `openstack-${cred.metadata.name}`,
      name: cred.metadata.name,
      type: 'OpenStack' as const,
      status: cred.status?.openstackValidationStatus || 'Unknown',
      credObject: cred
    })) || []

  const allCredentials = isVmware ? vmwareItems : openstackItems

  useEffect(() => {
    if (revalidatingId) {
      const revalidatingItem = allCredentials.find((cred) => cred.id === revalidatingId)
      const currentDataUpdatedAt = isVmware ? vmwareDataUpdatedAt : openstackDataUpdatedAt

      if (revalidatingItem) {
        const status = normalizeCredentialStatus(revalidatingItem.status)
        const hasFreshStatus = currentDataUpdatedAt > revalidationStartDataUpdatedAtRef.current

        if (!isRevalidationApiPending && hasFreshStatus && status !== 'Revalidating') {
          clearActiveRevalidation()
          setTimedOutRevalidatingId((current) => (current === revalidatingId ? null : current))
        }
      } else {
        clearActiveRevalidation()
        setTimedOutRevalidatingId((current) => (current === revalidatingId ? null : current))
      }
    }
  }, [
    allCredentials,
    clearActiveRevalidation,
    isRevalidationApiPending,
    isVmware,
    openstackDataUpdatedAt,
    revalidatingId,
    vmwareDataUpdatedAt
  ])

  useEffect(() => clearRevalidationTimeout, [clearRevalidationTimeout])

  useEffect(() => {
    if (isVmware) {
      refetchVmware()
    } else {
      refetchOpenstack()
    }
  }, [isVmware, refetchOpenstack, refetchVmware])

  const handleRefresh = useCallback(() => {
    if (isVmware) {
      refetchVmware()
    } else {
      refetchOpenstack()
    }
  }, [isVmware, refetchOpenstack, refetchVmware])

  const handleDeleteCredential = (id: string, type: 'VMware' | 'OpenStack') => {
    const credentialName = id.startsWith('vmware-')
      ? id.replace('vmware-', '')
      : id.replace('openstack-', '')

    const credential =
      type === 'VMware'
        ? vmwareCredentials?.find((cred) => cred.metadata.name === credentialName)
        : openstackCredentials?.find((cred) => cred.metadata.name === credentialName)

    if (credential) {
      const credItem: CredentialItem = {
        id,
        name: credential.metadata.name,
        type,
        status:
          type === 'VMware'
            ? (credential as VmwareCredential).status?.vmwareValidationStatus || 'Unknown'
            : (credential as OpenstackCredential).status?.openstackValidationStatus || 'Unknown',
        credObject: credential
      }
      setSelectedForDeletion([credItem])
      setDeleteDialogOpen(true)
    }
  }

  const handleRevalidateClick = (row: CredentialItem) => {
    const { credObject } = row
    if (!credObject) {
      reportError(new Error('Cannot revalidate: Missing credential object data.'), {
        context: 'credentials-revalidation',
        metadata: { credentialName: row.name }
      })
      return
    }

    revalidationStartDataUpdatedAtRef.current = isVmware
      ? vmwareDataUpdatedAt
      : openstackDataUpdatedAt
    clearRevalidationTimeout()
    setTimedOutRevalidatingId(null)
    setRevalidatingId(row.id)
    revalidationTimeoutRef.current = setTimeout(() => {
      revalidationTimeoutRef.current = null
      setTimedOutRevalidatingId(row.id)
      setRevalidatingId((current) => (current === row.id ? null : current))
      reportError(
        new Error('Credential revalidation is still in progress after 31 minutes. You can retry.'),
        {
          context: 'credentials-revalidation-timeout',
          metadata: {
            credentialName: credObject.metadata.name,
            credentialKind: row.type === 'VMware' ? 'VmwareCreds' : 'OpenstackCreds'
          }
        }
      )
      queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: OPENSTACK_CREDS_QUERY_KEY })
    }, REVALIDATION_TIMEOUT_MS)

    revalidate({
      name: credObject.metadata.name,
      namespace: credObject.metadata.namespace || VJAILBREAK_DEFAULT_NAMESPACE,
      kind: row.type === 'VMware' ? 'VmwareCreds' : 'OpenstackCreds'
    })
  }

  const handleSelectionChange = (rowSelectionModel: GridRowSelectionModel) => {
    setSelectedIds(rowSelectionModel as string[])
  }

  const handleDeleteSelected = () => {
    const selectedCreds = allCredentials.filter((cred) => selectedIds.includes(cred.id))
    setSelectedForDeletion(selectedCreds)
    setDeleteDialogOpen(true)
  }

  const handleDeleteClose = () => {
    setDeleteDialogOpen(false)
    setSelectedForDeletion([])
    setDeleteError(null)
  }

  const handleConfirmDelete = async () => {
    setDeleting(true)
    try {
      const vmwareCreds = selectedForDeletion.filter((cred) => cred.type === 'VMware')
      const openstackCreds = selectedForDeletion.filter((cred) => cred.type === 'OpenStack')

      await Promise.all(
        vmwareCreds.map((cred) => {
          const credName = cred.id.replace('vmware-', '')
          return deleteVMwareCredsWithSecretFlow(credName)
        })
      )

      vmwareCreds.forEach((cred) => {
        const credName = cred.id.replace('vmware-', '')
        track(AMPLITUDE_EVENTS.VMWARE_CREDENTIALS_DELETED, {
          credentialName: credName,
          namespace: cred.credObject.metadata.namespace
        })
      })

      await Promise.all(
        openstackCreds.map((cred) => {
          const credName = cred.id.replace('openstack-', '')
          return deleteOpenStackCredsWithSecretFlow(credName)
        })
      )

      openstackCreds.forEach((cred) => {
        const credName = cred.id.replace('openstack-', '')
        track(AMPLITUDE_EVENTS.PCD_CREDENTIALS_DELETED, {
          credentialName: credName,
          isPcd: true,
          namespace: cred.credObject.metadata.namespace
        })
      })

      queryClient.invalidateQueries({ queryKey: VMWARE_CREDS_QUERY_KEY })
      queryClient.invalidateQueries({ queryKey: OPENSTACK_CREDS_QUERY_KEY })

      setSelectedIds([])
      handleDeleteClose()
    } catch (error) {
      console.error('Error deleting credentials:', error)

      const errorMessage = error instanceof Error ? error.message : String(error)
      const vmwareCreds = selectedForDeletion.filter((cred) => cred.type === 'VMware')
      const openstackCreds = selectedForDeletion.filter((cred) => cred.type === 'OpenStack')

      vmwareCreds.forEach((cred) => {
        const credName = cred.id.replace('vmware-', '')
        track(AMPLITUDE_EVENTS.VMWARE_CREDENTIALS_DELETE_FAILED, {
          credentialName: credName,
          namespace: cred.credObject.metadata.namespace,
          errorMessage
        })
      })

      openstackCreds.forEach((cred) => {
        const credName = cred.id.replace('openstack-', '')
        track(AMPLITUDE_EVENTS.PCD_CREDENTIALS_DELETE_FAILED, {
          credentialName: credName,
          isPcd: true,
          namespace: cred.credObject.metadata.namespace,
          errorMessage
        })
      })

      reportError(error as Error, {
        context: 'credentials-deletion',
        metadata: {
          selectedIds: selectedIds,
          credentialsCount: selectedIds.length,
          action: 'delete-credentials'
        }
      })
      setDeleteError(error instanceof Error ? error.message : 'Unknown error occurred')
    } finally {
      setDeleting(false)
    }
  }

  const getCustomErrorMessage = useCallback((error: Error | string) => {
    const baseMessage = 'Failed to delete credentials'
    if (error instanceof Error) {
      return `${baseMessage}: ${error.message}`
    }
    return `${baseMessage}: ${error}`
  }, [])

  const tableColumns = getColumns(
    handleDeleteCredential,
    handleRevalidateClick,
    revalidatingId,
    timedOutRevalidatingId
  )

  const isLoading = (isVmware ? loadingVmware : loadingOpenstack) || deleting

  const handleOpenVMwareCredDrawer = () => {
    setVmwareCredDrawerOpen(true)
  }

  const handleCloseVMwareCredDrawer = () => {
    setVmwareCredDrawerOpen(false)
    refetchVmware()
  }

  const handleOpenOpenstackCredDrawer = () => {
    setOpenstackCredDrawerOpen(true)
  }

  const handleCloseOpenstackCredDrawer = () => {
    setOpenstackCredDrawerOpen(false)
    refetchOpenstack()
  }

  const title = isVmware ? 'VMware Credentials' : 'PCD Credentials'
  const actionLabel = isVmware ? 'Add VMware Credentials' : 'Add PCD Credentials'
  const actionDataTour = isVmware ? 'add-vmware-creds' : 'add-pcd-creds'
  const handleOpenDrawer = isVmware ? handleOpenVMwareCredDrawer : handleOpenOpenstackCredDrawer

  return (
    <div style={{ height: '100%', width: '100%', overflow: 'hidden' }}>
      <CommonDataGrid
        rows={allCredentials}
        columns={tableColumns}
        disableRowSelectionOnClick
        checkboxSelection
        rowSelectionModel={selectedIds}
        onRowSelectionModelChange={handleSelectionChange}
        initialState={{
          sorting: {
            sortModel: [{ field: 'name', sort: 'asc' }]
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
              title={title}
              numSelected={selectedIds.length}
              onDeleteSelected={handleDeleteSelected}
              loading={isLoading}
              onRefresh={handleRefresh}
              onAddCredential={handleOpenDrawer}
              actionLabel={actionLabel}
              actionDataTour={actionDataTour}
            />
          )
        }}
        pageSizeOptions={[10, 25, 50, 100]}
        loading={isLoading}
        emptyMessage="No credentials available"
        sx={{
          '& .MuiDataGrid-cell:focus': {
            outline: 'none'
          }
        }}
      />

      <ConfirmationDialog
        open={deleteDialogOpen}
        onClose={handleDeleteClose}
        title="Confirm Delete"
        icon={<WarningIcon color="warning" />}
        message={
          selectedForDeletion.length > 1
            ? 'Are you sure you want to delete these credentials?'
            : `Are you sure you want to delete ${selectedForDeletion[0]?.type} credential "${selectedForDeletion[0]?.name}"?`
        }
        items={selectedForDeletion.map((cred) => ({
          id: cred.id,
          name: `${cred.name} (${cred.type})`
        }))}
        actionLabel="Delete"
        actionColor="error"
        actionVariant="outlined"
        onConfirm={handleConfirmDelete}
        customErrorMessage={getCustomErrorMessage}
        errorMessage={deleteError}
        onErrorChange={setDeleteError}
      />

      {vmwareCredDrawerOpen && (
        <VMwareCredentialsDrawer
          open={vmwareCredDrawerOpen}
          onClose={handleCloseVMwareCredDrawer}
        />
      )}

      {openstackCredDrawerOpen && (
        <OpenstackCredentialsDrawer
          open={openstackCredDrawerOpen}
          onClose={handleCloseOpenstackCredDrawer}
        />
      )}
    </div>
  )
}
