import { Box, CircularProgress } from '@mui/material'
import { useState, useCallback, useEffect, useMemo } from 'react'
import { useForm, SubmitHandler } from 'react-hook-form'
import {
  DrawerShell,
  DrawerHeader,
  DrawerFooter,
  ActionButton,
  InlineHelp,
  FormGrid,
  Row,
  SurfaceCard
} from 'src/components'

import { DesignSystemForm, RHFTextField, RHFSelect } from 'src/shared/components/forms'
import { OpenstackCredentialsForm } from 'src/features/credentials/components'

import { getOpenstackCredentials } from 'src/api/openstack-creds/openstackCreds'
import { createNodes } from 'src/api/nodes/nodeMappings'
import axios from 'axios'
import { OpenstackCreds, OpenstackFlavor } from 'src/api/openstack-creds/model'
import { NodeItem } from 'src/api/nodes/model'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'
import { useErrorHandler } from 'src/hooks/useErrorHandler'
import { useAmplitude } from 'src/hooks/useAmplitude'

interface ScaleUpDrawerProps {
  open: boolean
  onClose: () => void
  masterNode: NodeItem | null
}

interface ScaleUpFormValues {
  openstackCredential: string
  flavor: string
  securityGroup: string
  nodeCount: number
}

export default function ScaleUpDrawer({ open, onClose, masterNode }: ScaleUpDrawerProps) {
  const { reportError } = useErrorHandler({ component: 'ScaleUpDrawer' })
  const { track } = useAmplitude({ component: 'ScaleUpDrawer' })

  const form = useForm<ScaleUpFormValues>({
    defaultValues: {
      openstackCredential: '',
      flavor: '',
      securityGroup: '',
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
  const [success, setSuccess] = useState(false)

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

  const securityGroupOptions = useMemo(() => {
    const groups = openstackCredentials?.status?.openstack?.securityGroups || []
    return groups.map((group) => ({
      label: group.requiresIdDisplay ? `${group.name} (${group.id})` : group.name,
      value: group.id
    }))
  }, [openstackCredentials])

  const clearStates = useCallback(() => {
    reset({
      openstackCredential: '',
      flavor: '',
      securityGroup: '',
      nodeCount: 1
    })
    setOpenstackCredentials(null)
    setOpenstackError(null)
    setError(null)
    setSuccess(false)
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
      setValue('securityGroup', '')

      if (credId) {
        try {
          const response = await getOpenstackCredentials(credId)
          setOpenstackCredentials(response)
          setOpenstackError(null)
        } catch (err) {
          console.error('Error fetching OpenStack credentials:', err)
          reportError(err as Error, {
            context: 'scaleup-fetch-openstack-credential',
            metadata: {
              credentialName: credId,
              action: 'get-openstack-credential'
            }
          })
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
    [reportError, setValue]
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

        track('Agents Scale Up', {
          stage: 'start',
          nodeCount: nodeCountNum,
          flavorId: values.flavor,
          credentialName: openstackCredentials?.metadata?.name
        })

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

        track('Agents Scale Up', {
          stage: 'success',
          nodeCount: nodeCountNum,
          flavorId: values.flavor,
          credentialName: openstackCredentials?.metadata?.name
        })

        setSuccess(true)
        setTimeout(() => {
          handleClose()
        }, 1500)
      } catch (err) {
        console.error('Error scaling up nodes:', err)

        track('Agents Scale Up', {
          stage: 'failure',
          nodeCount: nodeCountNum,
          flavorId: values.flavor,
          credentialName: openstackCredentials?.metadata?.name,
          errorMessage: err instanceof Error ? err.message : String(err)
        })

        reportError(err as Error, {
          context: 'scaleup-create-nodes',
          metadata: {
            nodeCount: nodeCountNum,
            flavorId: values.flavor,
            credentialName: openstackCredentials?.metadata?.name,
            action: 'create-nodes'
          }
        })

        setError(err instanceof Error ? err.message : 'Failed to scale up nodes')
      } finally {
        setLoading(false)
      }
    },
    [handleClose, masterNode, openstackCredentials, reportError, track]
  )

  const isSubmitDisabled =
    !masterNode ||
    !watchedValues.flavor ||
    !watchedValues.securityGroup ||
    loading ||
    !openstackCredsValidated ||
    !!errors.openstackCredential ||
    !!errors.flavor ||
    !!errors.securityGroup ||
    !!errors.nodeCount

  return (
    <DrawerShell
      open={open}
      onClose={handleClose}
      header={
        <DrawerHeader
          title="Scale Up Agents"
          subtitle="Create additional worker agents using a PCD credential and flavor"
          onClose={handleClose}
        />
      }
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
        <Box sx={{ display: 'grid', gap: 2 }} data-testid="scaleup-form">
          <SurfaceCard
            title="1. PCD Credentials"
            subtitle="Select an existing PCD credential. The credential must be validated."
          >
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
          </SurfaceCard>

          <SurfaceCard
            title="2. Agent Template"
            subtitle="Choose the flavor and security group for newly created agents."
          >
            <FormGrid minWidth={260} gap={2}>
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

              <RHFSelect
                name="securityGroup"
                label="Security Group"
                options={securityGroupOptions}
                placeholder={
                  !openstackCredsValidated
                    ? 'Select a validated credential first'
                    : securityGroupOptions.length === 0
                      ? 'No security groups available'
                      : 'Select a security group'
                }
                disabled={!openstackCredsValidated || securityGroupOptions.length === 0}
                searchable
                searchPlaceholder="Search security groups"
                rules={{ required: 'Security group is required' }}
              />
            </FormGrid>

            <Box sx={{ display: 'grid', gap: 1.5 }}>
              <InlineHelp tone="warning">
                Select a flavor with disk &gt; 16GB for production workloads.
              </InlineHelp>

              {loadingFlavors && (
                <InlineHelp tone="warning">
                  <Row gap={1}>
                    <CircularProgress size={16} />
                    <span>Loading available flavors…</span>
                  </Row>
                </InlineHelp>
              )}
            </Box>
          </SurfaceCard>

          <SurfaceCard
            title="3. Agent Count & Review"
            subtitle="Set the number of agents to create and review your selections before scaling up."
          >
            <FormGrid minWidth={260} gap={2}>
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

            <Box sx={{ display: 'grid', gap: 2 }}>
              {loading && (
                <InlineHelp tone="warning">
                  <Row gap={1}>
                    <CircularProgress size={16} />
                    <span>Scaling up agents…</span>
                  </Row>
                </InlineHelp>
              )}

              {success && <InlineHelp tone="positive">Agents successfully created.</InlineHelp>}

              {error && <InlineHelp tone="critical">{error}</InlineHelp>}
            </Box>
          </SurfaceCard>
        </Box>
      </DesignSystemForm>
    </DrawerShell>
  )
}
