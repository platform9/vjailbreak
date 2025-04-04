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
import { getSecret } from "src/api/secrets/secrets"
import { VJAILBREAK_DEFAULT_NAMESPACE } from "src/api/constants"

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
      let mappedCreds = {}
      const selectedCred = vmwareCredsList.find(cred => cred.metadata.name === credId)
      if (selectedCred) {
        try {
          if (selectedCred.spec.secretRef?.name) {
            // Fetch the secret data
            const secretName = selectedCred.spec.secretRef.name
            const secret = await getSecret(secretName, selectedCred.metadata.namespace || VJAILBREAK_DEFAULT_NAMESPACE)

            if (secret?.data) {
              // Add secret data to mappedCreds
              mappedCreds = {
                existingCredName: selectedCred.metadata.name,
                secretRef: selectedCred.spec.secretRef,
                datacenter: secret.data.VCENTER_DATACENTER,
              }
            }
          }

          onChange("vmwareCreds")(mappedCreds)
        } catch (error) {
          console.error("Error fetching VMware credential secret:", error)
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
          const mappedCreds = {
            existingCredName: selectedCred.metadata.name,
          }

          onChange("openstackCreds")(mappedCreds)
        } catch (error) {
          console.error("Error fetching OpenStack credential secret:", error)
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
