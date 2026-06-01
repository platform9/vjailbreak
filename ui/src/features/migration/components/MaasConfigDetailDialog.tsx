import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Typography
} from '@mui/material'
import { ActionButton } from 'src/components'
import { styled } from '@mui/material/styles'
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism'
import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { VJAILBREAK_DEFAULT_NAMESPACE } from 'src/api/constants'
import { BMConfig } from 'src/api/bmconfig/model'

const MaasConfigDialog = styled(Dialog)({
  '& .MuiDialog-paper': {
    maxWidth: '900px',
    width: '100%'
  }
})

const ConfigSection = styled(Box)(({ theme }) => ({
  marginBottom: theme.spacing(3)
}))

const ConfigField = styled(Box)(({ theme }) => ({
  display: 'flex',
  gap: theme.spacing(1),
  marginBottom: theme.spacing(1.5)
}))

const FieldLabel = styled(Typography)(({ theme }) => ({
  fontWeight: 500,
  minWidth: '140px',
  color: theme.palette.text.secondary
}))

const FieldValue = styled(Typography)(({ theme }) => ({
  fontWeight: 400,
  color: theme.palette.text.primary
}))

const CodeEditorContainer = styled(Box)(({ theme }) => ({
  border: `1px solid ${theme.palette.grey[300]}`,
  borderRadius: theme.shape.borderRadius,
  overflow: 'auto',
  position: 'relative',
  resize: 'vertical',
  minHeight: '250px',
  maxHeight: '400px',
  backgroundColor: theme.palette.common.white,
  '& pre': {
    margin: 0,
    borderRadius: 0,
    height: '100%',
    overflow: 'auto',
    fontSize: '14px'
  },
  '&::-webkit-scrollbar': {
    width: '8px',
    height: '8px'
  },
  '&::-webkit-scrollbar-thumb': {
    backgroundColor: theme.palette.grey[300],
    borderRadius: '4px'
  },
  '&::-webkit-scrollbar-track': {
    backgroundColor: theme.palette.grey[100]
  }
}))

interface MaasConfigDetailDialogProps {
  open: boolean
  onClose: () => void
  selectedMaasConfig: BMConfig | null
  loadingMaasConfig: boolean
}

export default function MaasConfigDetailDialog({
  open,
  onClose,
  selectedMaasConfig,
  loadingMaasConfig
}: MaasConfigDetailDialogProps) {
  return (
    <MaasConfigDialog
      open={open}
      onClose={onClose}
      aria-labelledby="baremetal-config-dialog-title"
      data-testid="maas-config-detail-dialog"
    >
      <DialogTitle id="baremetal-config-dialog-title">
        <Typography variant="h6">ESXi - Bare Metal Configuration</Typography>
      </DialogTitle>
      <DialogContent dividers>
        {loadingMaasConfig ? (
          <Typography>Loading configuration details...</Typography>
        ) : !selectedMaasConfig ? (
          <Typography>No configuration available</Typography>
        ) : (
          <>
            <ConfigSection>
              <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>
                Provider Configuration
              </Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, ml: 1 }}>
                <ConfigField>
                  <FieldLabel>Provider Type:</FieldLabel>
                  <FieldValue>{selectedMaasConfig!.spec.providerType}</FieldValue>
                </ConfigField>
                <ConfigField>
                  <FieldLabel>Bare Metal Provider URL:</FieldLabel>
                  <FieldValue>{selectedMaasConfig!.spec.apiUrl}</FieldValue>
                </ConfigField>
                <ConfigField>
                  <FieldLabel>Insecure:</FieldLabel>
                  <FieldValue>{selectedMaasConfig!.spec.insecure ? 'Yes' : 'No'}</FieldValue>
                </ConfigField>
                {selectedMaasConfig!.spec.os && (
                  <ConfigField>
                    <FieldLabel>OS:</FieldLabel>
                    <FieldValue>{selectedMaasConfig!.spec.os}</FieldValue>
                  </ConfigField>
                )}
                <ConfigField>
                  <FieldLabel>Status:</FieldLabel>
                  <FieldValue>
                    {selectedMaasConfig!.status?.validationStatus || 'Pending validation'}
                  </FieldValue>
                </ConfigField>
                {selectedMaasConfig!.status?.validationMessage && (
                  <ConfigField>
                    <FieldLabel>Validation Message:</FieldLabel>
                    <FieldValue>{selectedMaasConfig!.status!.validationMessage}</FieldValue>
                  </ConfigField>
                )}
              </Box>
            </ConfigSection>

            {selectedMaasConfig!.spec.userDataSecretRef && (
              <ConfigSection>
                <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>
                  Cloud-Init Configuration
                </Typography>
                <Typography
                  variant="caption"
                  sx={{ mb: 1, display: 'block', color: 'text.secondary' }}
                >
                  User data is stored in a secret:{' '}
                  {selectedMaasConfig!.spec.userDataSecretRef.name}
                </Typography>
                <CodeEditorContainer>
                  <SyntaxHighlighter
                    language="yaml"
                    style={oneLight}
                    showLineNumbers
                    wrapLongLines
                    customStyle={{
                      margin: 0,
                      maxHeight: '100%'
                    }}
                  >
                    {`# Cloud-init configuration is stored in Kubernetes Secret:
# ${selectedMaasConfig!.spec.userDataSecretRef.name}
# in namespace: ${selectedMaasConfig!.spec.userDataSecretRef.namespace || VJAILBREAK_DEFAULT_NAMESPACE}

# The cloud-init configuration includes:
# - package updates and installations
# - configuration files
# - commands to run on startup
# - network configuration
# - and other system setup parameters

# This will be used when provisioning ESXi hosts in the bare metal environment.`}
                  </SyntaxHighlighter>
                </CodeEditorContainer>
              </ConfigSection>
            )}

            <ConfigSection>
              <Typography variant="subtitle1" gutterBottom sx={{ fontWeight: 500 }}>
                Resource Information
              </Typography>
              <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3, ml: 1 }}>
                <ConfigField>
                  <FieldLabel>Name:</FieldLabel>
                  <FieldValue>{selectedMaasConfig.metadata.name}</FieldValue>
                </ConfigField>
                <ConfigField>
                  <FieldLabel>Namespace:</FieldLabel>
                  <FieldValue>{selectedMaasConfig.metadata.namespace}</FieldValue>
                </ConfigField>
                <ConfigField>
                  <FieldLabel>Created:</FieldLabel>
                  <FieldValue>
                    {new Date(selectedMaasConfig.metadata.creationTimestamp).toLocaleString()}
                  </FieldValue>
                </ConfigField>
              </Box>
            </ConfigSection>
          </>
        )}
      </DialogContent>
      <DialogActions sx={{ gap: 1, p: 2 }}>
        <ActionButton
          tone="primary"
          onClick={onClose}
          data-testid="rolling-migration-form-baremetal-dialog-close"
        >
          Close
        </ActionButton>
      </DialogActions>
    </MaasConfigDialog>
  )
}
