import {
  styled,
  Typography,
} from "@mui/material"
import { useEffect, useState } from "react"
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

const FieldsContainer = styled("div")(({ theme }) => ({
  display: "grid",
  marginLeft: theme.spacing(6),
}))

interface SourceAndDestinationEnvStepProps {
  onChange: (id: string) => (value: unknown) => void
  errors: { [fieldId: string]: string }
  vmwareCredsValidated?: boolean | null
  validatingVmwareCreds?: boolean
  validatingOpenstackCreds?: boolean
  openstackCredsValidated?: boolean | null
}

export default function SourceAndDestinationEnvStep({
  onChange,
  errors,
  validatingVmwareCreds = false,
  validatingOpenstackCreds = false,
  vmwareCredsValidated = null,
  openstackCredsValidated = null,
}: SourceAndDestinationEnvStepProps) {
  const [selectedVmwareCred, setSelectedVmwareCred] = useState<string | null>(null)
  const [selectedOpenstackCred, setSelectedOpenstackCred] = useState<string | null>(null)

  // Fetch credentials
  const { data: vmwareCredsList = [], isLoading: loadingVmwareCreds, refetch: refetchVmwareCreds } = useVmwareCredentialsQuery()
  const { data: openstackCredsList = [], isLoading: loadingOpenstackCreds, refetch: refetchOpenstackCreds } = useOpenstackCredentialsQuery()

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
    }
  }

  // Auto-select newly created VMware credential when validated
  useEffect(() => {
    if (vmwareCredsValidated === true && selectedVmwareCred === null) {
      refetchVmwareCreds()
    }
  }, [vmwareCredsValidated, selectedVmwareCred, refetchVmwareCreds])

  // Auto-select newly created OpenStack credential when validated
  useEffect(() => {
    if (openstackCredsValidated === true && selectedOpenstackCred === null) {
      refetchOpenstackCreds()
    }
  }, [openstackCredsValidated, selectedOpenstackCred, refetchOpenstackCreds])
  console.log("Errors are ", errors)

  return (
    <SourceAndDestinationStepContainer>
      <Step stepNumber="1" label="Source and Destination Environments" />
      <FieldsContainer>
        <Typography variant="body1">Source VMware</Typography>

        <VmwareCredentialsForm
          credentialsList={vmwareCredsList}
          loadingCredentials={loadingVmwareCreds}
          refetchCredentials={refetchVmwareCreds}
          validatingCredentials={validatingVmwareCreds}
          credentialsValidated={vmwareCredsValidated}
          error={errors["vmwareCreds"]}
          onChange={onChange("vmwareCreds")}
          onCredentialSelect={handleVmwareCredSelect}
          selectedCredential={selectedVmwareCred}
        />
      </FieldsContainer>

      <FieldsContainer>
        <Typography variant="body1">Destination Platform</Typography>

        <OpenstackCredentialsForm
          credentialsList={openstackCredsList}
          loadingCredentials={loadingOpenstackCreds}
          refetchCredentials={refetchOpenstackCreds}
          validatingCredentials={validatingOpenstackCreds}
          credentialsValidated={openstackCredsValidated}
          error={errors["openstackCreds"]}
          onChange={onChange("openstackCreds")}
          onCredentialSelect={handleOpenstackCredSelect}
          selectedCredential={selectedOpenstackCred}
        />
      </FieldsContainer>
    </SourceAndDestinationStepContainer>
  )
}
