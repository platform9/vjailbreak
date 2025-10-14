import {
  styled,
  Typography,
  Box,
} from "@mui/material"
import { useState } from "react"
import Step from "../../components/forms/Step"
import { useVmwareCredentialsQuery } from "src/hooks/api/useVmwareCredentialsQuery"
import { useOpenstackCredentialsQuery } from "src/hooks/api/useOpenstackCredentialsQuery"
import VmwareCredentialsForm from "../../components/forms/VmwareCredentialsForm"
import OpenstackCredentialsForm from "../../components/forms/OpenstackCredentialsForm"

const SourceAndDestinationStepContainer = styled("div")(({ theme }) => ({
  display: "grid",
  gridGap: theme.spacing(1),
}))


const SideBySideContainer = styled(Box)(({ theme }) => ({
  display: "grid",
  gridTemplateColumns: "1fr 1fr",
  gap: theme.spacing(3),
  marginLeft: theme.spacing(6),
}))

interface SourceAndDestinationEnvStepProps {
  onChange: (id: string) => (value: unknown) => void
  errors: { [fieldId: string]: string }
}

export default function SourceAndDestinationEnvStep({
  onChange,
  errors,
}: SourceAndDestinationEnvStepProps) {
  const [selectedVmwareCred, setSelectedVmwareCred] = useState<string | null>(null)
  const [selectedOpenstackCred, setSelectedOpenstackCred] = useState<string | null>(null)

  const { data: vmwareCredsList = [], isLoading: loadingVmwareCreds } = useVmwareCredentialsQuery()
  const { data: openstackCredsList = [], isLoading: loadingOpenstackCreds } = useOpenstackCredentialsQuery()

  const handleVmwareCredSelect = async (credId: string | null) => {
    setSelectedVmwareCred(credId)
    if (credId) {
      const selectedCred = vmwareCredsList.find(cred => cred.metadata.name === credId)
      if (selectedCred) {
        try {
          const mappedCreds = {
            existingCredName: selectedCred.metadata.name,
            secretRef: selectedCred.spec.secretRef,
            datacenter: selectedCred.spec.datacenter,
            hostName: selectedCred.spec.hostName,
          }

          onChange("vmwareCreds")(mappedCreds)
        } catch (error) {
          console.error("Error processing VMware credential:", error)
        }
      }
    } else {
      // Clear the selection
      onChange("vmwareCreds")({})
    }
  }

  const handleOpenstackCredSelect = async (credId: string | null) => {
    setSelectedOpenstackCred(credId)
    if (credId) {
      const selectedCred = openstackCredsList.find(cred => cred.metadata.name === credId)
      if (selectedCred) {
        try {
          // Use data directly from credential spec instead of fetching from secret
          const mappedCreds = {
            existingCredName: selectedCred.metadata.name,
            projectName: selectedCred.spec.projectName,
          }

          onChange("openstackCreds")(mappedCreds)
        } catch (error) {
          console.error("Error processing OpenStack credential:", error)
        }
      }
    } else {
      // Clear the selection
      onChange("openstackCreds")({})
    }
  }
  return (
    <SourceAndDestinationStepContainer>
      <Step stepNumber="1" label="Source and Destination Environments" />
      <SideBySideContainer>
        <Box>
          <Typography variant="body1" sx={{ mb: 1 }}>Source VMware</Typography>
          <VmwareCredentialsForm
            credentialsList={vmwareCredsList}
            loadingCredentials={loadingVmwareCreds}
            error={errors["vmwareCreds"]}
            onCredentialSelect={handleVmwareCredSelect}
            selectedCredential={selectedVmwareCred}
            showCredentialSelector={true}
            showCredentialNameField={false}
          />
        </Box>

        <Box>
          <Typography variant="body1" sx={{ mb: 1 }}>Destination Platform</Typography>
          <OpenstackCredentialsForm
            credentialsList={openstackCredsList}
            loadingCredentials={loadingOpenstackCreds}
            error={errors["openstackCreds"]}
            onCredentialSelect={handleOpenstackCredSelect}
            selectedCredential={selectedOpenstackCred}
            showCredentialSelector={true}
            showCredentialNameField={false}
          />
        </Box>
      </SideBySideContainer>
    </SourceAndDestinationStepContainer>
  )
}
