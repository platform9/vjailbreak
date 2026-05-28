import {
  Alert,
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography
} from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { useState } from 'react'
import { useForm, Controller } from 'react-hook-form'
import { useQueryClient } from '@tanstack/react-query'
import axios from 'axios'
import { postProxyVM } from 'src/api/proxyvms/proxyVMs'
import { PROXY_VMS_QUERY_KEY } from 'src/hooks/api/useProxyVMsQuery'
import { useVmwareCredentialsQuery } from 'src/hooks/api/useVmwareCredentialsQuery'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'

interface FormData {
  vmName: string
  vmwareCredsRef: string
}

interface AddProxyVMDialogProps {
  open: boolean
  onClose: () => void
}

function toK8sName(input: string): string {
  return input
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
    .slice(0, 63)
}

const PREREQUISITES = [
  'lsblk — block device lister must be installed',
  'nbdkit — network block device kit must be installed',
  'qemu-nbd — QEMU NBD server must be installed',
  'sshd — SSH daemon must be running and accessible',
  'disk.EnableUUID — VMware disk UUID must be enabled on the VM',
  'SSH key authorization — vJailbreak public key must be added to authorized_keys'
]

export default function AddProxyVMDialog({ open, onClose }: AddProxyVMDialogProps) {
  const queryClient = useQueryClient()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const { data: vmwareCreds = [] } = useVmwareCredentialsQuery()
  const succeededCreds = vmwareCreds.filter(
    (c) => c.status?.vmwareValidationStatus?.toLowerCase() === 'succeeded'
  )

  const {
    control,
    handleSubmit,
    watch,
    reset,
    formState: { errors }
  } = useForm<FormData>({
    defaultValues: { vmName: '', vmwareCredsRef: '' }
  })

  const vmName = watch('vmName')
  const derivedName = vmName ? toK8sName(vmName) : ''

  const handleClose = () => {
    reset()
    setSubmitError(null)
    onClose()
  }

  const onSubmit = async (data: FormData) => {
    setIsSubmitting(true)
    setSubmitError(null)
    try {
      await postProxyVM({
        apiVersion: 'vjailbreak.k8s.pf9.io/v1alpha1',
        kind: 'ProxyVM',
        metadata: {
          name: derivedName,
          namespace: VJAILBREAK_DEFAULT_NAMESPACE,
          creationTimestamp: '',
          uid: ''
        },
        spec: {
          vmName: data.vmName,
          vmwareCredsRef: { name: data.vmwareCredsRef }
        }
      })
      queryClient.invalidateQueries({ queryKey: PROXY_VMS_QUERY_KEY })
      handleClose()
    } catch (err) {
      if (axios.isAxiosError(err) && err.response?.status === 409) {
        setSubmitError(`A Proxy VM with the name "${derivedName}" already exists.`)
      } else if (axios.isAxiosError(err)) {
        setSubmitError(err.response?.data?.message || 'Failed to create Proxy VM.')
      } else {
        setSubmitError('Failed to create Proxy VM.')
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Add Proxy VM</DialogTitle>
      <DialogContent>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2, mt: 1 }}>
          {submitError && <Alert severity="error">{submitError}</Alert>}

          <Controller
            name="vmName"
            control={control}
            rules={{ required: 'VM name is required' }}
            render={({ field }) => (
              <TextField
                {...field}
                label="VM Name"
                required
                error={!!errors.vmName}
                helperText={errors.vmName?.message}
                fullWidth
              />
            )}
          />

          {derivedName && (
            <TextField
              label="Kubernetes Resource Name"
              value={derivedName}
              InputProps={{ readOnly: true }}
              size="small"
              helperText="Auto-derived from VM name"
              fullWidth
            />
          )}

          <Controller
            name="vmwareCredsRef"
            control={control}
            rules={{ required: 'VMware credentials are required' }}
            render={({ field }) => (
              <FormControl fullWidth required error={!!errors.vmwareCredsRef}>
                <InputLabel>VMware Credentials</InputLabel>
                <Select {...field} label="VMware Credentials">
                  {succeededCreds.length === 0 && (
                    <MenuItem disabled value="">
                      No validated VMware credentials found
                    </MenuItem>
                  )}
                  {succeededCreds.map((cred) => (
                    <MenuItem key={cred.metadata.name} value={cred.metadata.name}>
                      {cred.metadata.name}
                    </MenuItem>
                  ))}
                </Select>
                {errors.vmwareCredsRef && (
                  <FormHelperText>{errors.vmwareCredsRef.message}</FormHelperText>
                )}
              </FormControl>
            )}
          />

          <Accordion disableGutters elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
            <AccordionSummary expandIcon={<ExpandMoreIcon />}>
              <Typography variant="body2" fontWeight={500}>
                Setup Prerequisites
              </Typography>
            </AccordionSummary>
            <AccordionDetails>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                Ensure the following are configured on the proxy VM before adding:
              </Typography>
              <Box component="ul" sx={{ m: 0, pl: 2 }}>
                {PREREQUISITES.map((item) => (
                  <Box component="li" key={item} sx={{ mb: 0.5 }}>
                    <Typography variant="body2">{item}</Typography>
                  </Box>
                ))}
              </Box>
            </AccordionDetails>
          </Accordion>
        </Box>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={handleClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit(onSubmit)}
          disabled={isSubmitting}
          startIcon={isSubmitting ? <CircularProgress size={16} /> : null}
        >
          {isSubmitting ? 'Adding...' : 'Add Proxy VM'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
