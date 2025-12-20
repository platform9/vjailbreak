import { Alert, Box, CircularProgress } from '@mui/material'
import { useState, useCallback, useEffect, useMemo } from 'react'
import { useForm, SubmitHandler } from 'react-hook-form'
import {
  DrawerShell,
  DrawerHeader,
  DrawerFooter,
  ActionButton,
  Section,
  SectionHeader,
  InlineHelp,
  FormGrid,
  Row
} from 'src/components'

import { DesignSystemForm, RHFTextField, RHFSelect } from 'src/shared/components/forms'
import { OpenstackCredentialsForm } from 'src/features/credentials/components'

import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { createNodes } from 'src/api/nodes/nodeMappings'
import axios from 'axios'
import { OpenstackCreds, OpenstackFlavor } from 'src/api/openstack-creds/model'
import { NodeItem } from 'src/api/nodes/model'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'

interface ScaleUpDrawerProps {
  open: boolean
  onClose: () => void
  masterNode: NodeItem | null
}

interface ScaleUpFormValues {
  openstackCredential: string
  flavor: string
  nodeCount: number
}

export default function ScaleUpDrawer({ open, onClose, masterNode }: ScaleUpDrawerProps) {
  const form = useForm<ScaleUpFormValues>({
    defaultValues: {
      openstackCredential: '',
      flavor: '',
      nodeCount: 1
    }
  })

  const {
    watch,
    setValue,
    reset,
    formState: { errors }
  } = form

  const [openstackCredentials, setOpenstackCredentials] = useState<OpenstackCreds | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [openstackError, setOpenstackError] = useState<string | null>(null)

  const [flavors, setFlavors] = useState<Array<OpenstackFlavor>>([])
  const [loadingFlavors, setLoadingFlavors] = useState(false)
  const [flavorsError, setFlavorsError] = useState<string | null>(null)

  const watchedValues = watch()
  const selectedOpenstackCred = watchedValues.openstackCredential

  const { data: openstackCredsList = [], isLoading: loadingOpenstackCreds } =
    useOpenstackCredentialsQuery()

  const openstackCredsValidated =
    openstackCredentials?.status?.openstackValidationStatus === 'Succeeded'

  const flavorOptions = useMemo(() => {
    return flavors.map((flavor) => ({
      label: `${flavor.name} — ${flavor.vcpus} vCPU · ${flavor.ram / 1024}GB RAM · ${flavor.disk}GB disk`,
      value: flavor.id
    }))
  }, [flavors])

  const clearStates = useCallback(() => {
    reset({
      openstackCredential: '',
      flavor: '',
      nodeCount: 1
    })
    setOpenstackCredentials(null)
    setOpenstackError(null)
    setError(null)
    setFlavors([])
    setLoadingFlavors(false)
    setFlavorsError(null)
  }, [reset])

  const handleClose = useCallback(() => {
    clearStates()
    onClose()
  }, [clearStates, onClose])

  const handleOpenstackCredSelect = useCallback(
    async (credId: string | null) => {
      setValue('openstackCredential', credId || '')
      setValue('flavor', '')

      if (credId) {
        try {
          const response = await getOpenstackCredentials(credId)
          setOpenstackCredentials(response)
          setOpenstackError(null)
        } catch (err) {
          console.error('Error fetching OpenStack credentials:', err)
          setOpenstackError(
            'Error fetching PCD credentials: ' +
              (axios.isAxiosError(err) ? err?.response?.data?.message : String(err))
          )
        }
      } else {
        setOpenstackCredentials(null)
        setOpenstackError(null)
      }
    },
    [setValue]
  )

  useEffect(() => {
    const fetchFlavours = async () => {
      if (openstackCredsValidated && openstackCredentials) {
        setLoadingFlavors(true)
        setFlavorsError(null)
        try {
          const flavours = openstackCredentials?.spec.flavors
          setFlavors(flavours || [])
        } catch (err) {
          console.error('Failed to fetch flavors:', err)
          setFlavorsError('Failed to fetch PCD flavors')
        } finally {
          setLoadingFlavors(false)
        }
      } else {
        setFlavors([])
        setValue('flavor', '')
      }
    }
    fetchFlavours()
  }, [openstackCredsValidated, openstackCredentials, setValue])

  const handleSubmit: SubmitHandler<ScaleUpFormValues> = useCallback(
    async (values) => {
      const nodeCountNum = Number(values.nodeCount)
      if (
        !masterNode?.spec.openstackImageID ||
        !values.flavor ||
        !nodeCountNum ||
        isNaN(nodeCountNum) ||
        !openstackCredentials?.metadata?.name
      ) {
        setError('Please fill in all required fields')
        return
      }

      try {
        setLoading(true)
        setError(null)
        await createNodes({
          imageId: masterNode.spec.openstackImageID,
          openstackCreds: {
            kind: 'openstackcreds' as const,
            name: openstackCredentials.metadata.name,
            namespace: 'migration-system'
          },
          count: nodeCountNum,
          flavorId: values.flavor
        })

        handleClose()
      } catch (err) {
        console.error('Error scaling up nodes:', err)
        setError(err instanceof Error ? err.message : 'Failed to scale up nodes')
      } finally {
        setLoading(false)
      }
    },
    [masterNode, openstackCredentials, handleClose]
  )

  const isSubmitDisabled =
    !masterNode ||
    !watchedValues.flavor ||
    loading ||
    !openstackCredsValidated ||
    !!errors.openstackCredential ||
    !!errors.flavor ||
    !!errors.nodeCount

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      header={<DrawerHeader title="Scale Up Agents" onClose={handleClose} />}
      footer={
        <DrawerFooter>
          <ActionButton tone="secondary" onClick={handleClose} data-testid="scaleup-cancel">
            Cancel
          </ActionButton>
          <ActionButton
            tone="primary"
            type="submit"
            form="scaleup-form"
            loading={loading}
            disabled={isSubmitDisabled}
            data-testid="scaleup-submit"
          >
            Scale Up
          </ActionButton>
        </DrawerFooter>
      }
    >
      <DesignSystemForm
        form={form}
        id="scaleup-form"
        onSubmit={handleSubmit}
        keyboardSubmitProps={{
          open,
          onClose: handleClose,
          isSubmitDisabled
        }}
      >
        <Box sx={{ display: 'grid', gap: 2, py: 1.5 }} data-testid="scaleup-form">
          <Section sx={{ mb: 1 }}>
            <SectionHeader title="PCD Credentials" sx={{ mb: 0 }} />

            <OpenstackCredentialsForm
              fullWidth
              size="small"
              credentialsList={openstackCredsList}
              loadingCredentials={loadingOpenstackCreds}
              error={openstackError ?? undefined}
              onCredentialSelect={handleOpenstackCredSelect}
              selectedCredential={selectedOpenstackCred}
              showCredentialSelector
            />
          </Section>

          <Section>
            <SectionHeader
              title="Agent Template"
              subtitle="Select a flavor to define the CPU, memory and disk for new agents."
            />

            <FormGrid minWidth={360} gap={1.5}>
              <RHFTextField
                name="masterAgentImage"
                label="Master Agent Image"
                value={
                  masterNode?.spec.openstackImageID
                    ? 'Image selected from master node'
                    : 'No image found on master node'
                }
              />

              <RHFSelect
                name="flavor"
                label="Flavor"
                options={flavorOptions}
                placeholder={loadingFlavors ? 'Loading flavors...' : 'Select a flavor'}
                disabled={loadingFlavors || !openstackCredsValidated || !openstackCredentials}
                searchable
                searchPlaceholder="Search flavors by name, vCPU, RAM or disk"
                rules={{ required: 'Flavor selection is required' }}
                helperText={flavorsError ?? undefined}
                error={!!flavorsError}
              />
            </FormGrid>

            <Alert severity="warning" variant="outlined" sx={{ mt: 1.5 }}>
              Select a flavor with disk &gt; 16GB for production workloads.
            </Alert>

            {loadingFlavors && (
              <Row gap={1} alignItems="center">
                <CircularProgress size={16} />
                <span>Loading available flavors…</span>
              </Row>
            )}
          </Section>

          <Section sx={{ mb: 1 }}>
            <SectionHeader
              title="Agent Count"
              subtitle="Specify how many agents to create with the selected template."
            />

            <FormGrid minWidth={360} gap={2}>
              <RHFTextField
                name="nodeCount"
                label="Number of Agents"
                type="number"
                rules={{
                  required: 'Number of agents is required',
                  validate: (value) => {
                    const num = Number(value)
                    if (isNaN(num)) return 'Please enter a valid number'
                    if (num < 1) return 'Minimum 1 agent required'
                    if (num > 5) return 'Maximum 5 agents allowed'
                    return true
                  }
                }}
                inputProps={{ min: 1, max: 5 }}
                fullWidth
                labelHelperText="Min: 1 · Max: 5"
                required
              />
            </FormGrid>
          </Section>

          {error && <InlineHelp tone="critical">{error}</InlineHelp>}
        </Box>
      </DesignSystemForm>
    </DrawerShell>
  )
}
