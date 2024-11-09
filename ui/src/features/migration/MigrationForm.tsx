import { Box, Drawer, styled } from "@mui/material"
import { flatten, uniq } from "ramda"
import { useEffect, useMemo, useState } from "react"
import { useNavigate } from "react-router-dom"
import { MigrationPlan } from "src/api/migration-plans/model"
import { VmData } from "src/api/migration-templates/model"
import { OpenstackCreds } from "src/api/openstack-creds/model"
import {
  getOpenstackCredentials,
  postOpenstackCredentials,
} from "src/api/openstack-creds/openstackCreds"
import { createVmwareCredsJson } from "src/api/vmware-creds/helpers"
import {
  getVmwareCredentials,
  postVmwareCredentials,
} from "src/api/vmware-creds/vmwareCreds"

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { createMigrationTemplateJson } from "src/api/migration-templates/helpers"
import {
  createMigrationTemplate,
  getMigrationTemplate,
} from "src/api/migration-templates/migrationTemplates"
import { createOpenstackCredsJson } from "src/api/openstack-creds/helpers"
import useParams from "src/hooks/useParams"
import { isNilOrEmpty } from "src/utils"
import Footer from "../../components/forms/Footer"
import Header from "../../components/forms/Header"
import MigrationOptions from "./MigrationOptionsAlt"
import NetworkAndStorageMappingStep from "./NetworkAndStorageMappingStep"
import SourceAndDestinationEnvStep from "./SourceAndDestinationEnvStep"
import VmsSelectionStep from "./VmsSelectionStep"
import { CUTOVER_TYPES } from "./constants"

const StyledDrawer = styled(Drawer)(() => ({
  "& .MuiDrawer-paper": {
    display: "grid",
    gridTemplateRows: "max-content 1fr max-content",
    width: "1034px",
  },
}))

const DrawerContent = styled("div")(({ theme }) => ({
  overflow: "auto",
  padding: theme.spacing(4, 6, 4, 4),
}))

export interface FormValues extends Record<string, unknown> {
  vmwareCreds?: {
    vcenterHost: string
    datacenter: string
    username: string
    password: string
  }
  openstackCreds?: OpenstackCreds
  vms?: VmData[]
  networkMappings?: { source: string; target: string }[]
  storageMappings?: { source: string; target: string }[]
  // Optional Params
  dataCopyMethod?: string
  dataCopyStartTime?: string
  cutoverOption?: string
  cutoverStartTime?: string
  cutoverEndTime?: string
  postMigrationScript?: string
  retryOnFailure?: boolean
  osType?: string
}

export interface SelectedMigrationOptionsType extends Record<string, unknown> {
  dataCopyMethod: boolean
  dataCopyStartTime: boolean
  cutoverOption: boolean
  cutoverStartTime: boolean
  cutoverEndTime: boolean
  postMigrationScript: boolean
  osType: boolean
}

// Default state for checkboxes
const defaultMigrationOptions = {
  dataCopyMethod: false,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false,
  osType: false,
}

const defaultValues: Partial<FormValues> = {}

export type Errors = { [formId: string]: string }

interface MigrationFormDrawerProps {
  open: boolean
  onClose: () => void
  reloadMigrations?: () => void
}

export default function MigrationFormDrawer({
  open,
  onClose,
}: MigrationFormDrawerProps) {
  const navigate = useNavigate()
  const { params, getParamsUpdater } = useParams<FormValues>(defaultValues)
  const { params: errors, getParamsUpdater: getErrorsUpdater } =
    useParams<Errors>({})
  const [validatingVmwareCreds, setValidatingVmwareCreds] = useState(false)
  const [validatingOpenstackCreds, setValidatingOpenstackCreds] =
    useState(false)
  const [submitting, setSubmitting] = useState(false)
  // Migration Options - Checked or Unchecked state
  const {
    params: selectedMigrationOptions,
    getParamsUpdater: updateSelectedMigrationOptions,
  } = useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  // Migration JSON Objects
  const [vmwareCredName, setVmwareCredName] = useState<string | null>(null)
  const [openstackCredName, setOpenstackCredName] = useState<string | null>(
    null
  )
  const [migrationTemplateName, setMigrationTemplateName] = useState<
    string | null
  >(null)
  const [migrationPlanResource, setMigrationPlanResource] =
    useState<MigrationPlan>({} as MigrationPlan)

  // Queries
  const queryClient = useQueryClient()

  const { data: vmWareCredentials } = useQuery({
    queryKey: ["migrationForm", "vmwareCreds"],
    queryFn: async () => {
      // Reset error
      getErrorsUpdater("vmwareCreds")("")

      const data = await getVmwareCredentials(vmwareCredName)
      if (data?.status?.vmwareValidationStatus === "Succeeded") {
        getErrorsUpdater("vmwareCreds")("")
        setValidatingVmwareCreds(false)
      } else if (data?.status?.vmwareValidationStatus === "Failed") {
        getErrorsUpdater("vmwareCreds")(data.status?.vmwareValidationMessage)
        setValidatingVmwareCreds(false)
      }
      return data
    },
    enabled: (query) =>
      !!query?.state?.data?.metadata?.name &&
      !query?.state?.data?.status?.vmwareValidationStatus,
    refetchInterval: 3000,
    notifyOnChangeProps: ["data"],
  })

  const { data: openstackCredentials } = useQuery({
    queryKey: ["migrationForm", "openstackCreds"],
    queryFn: async () => {
      // Reset error
      getErrorsUpdater("openstackCreds")("")

      const data = await getOpenstackCredentials(openstackCredName)
      if (data?.status?.openstackValidationStatus === "Succeeded") {
        getErrorsUpdater("openstackCreds")("")
        setValidatingOpenstackCreds(false)
      } else if (data?.status?.openstackValidationStatus === "Failed") {
        getErrorsUpdater("openstackCreds")(
          data.status?.openstackValidationMessage
        )
        setValidatingOpenstackCreds(false)
      }
      return data
    },
    enabled: (query) =>
      !!query?.state?.data?.metadata?.name &&
      !query?.state?.data?.status?.openstackValidationStatus,
    refetchInterval: 3000,
    notifyOnChangeProps: ["data"],
  })

  const { data: migrationTemplateResource } = useQuery({
    queryKey: ["migrationForm", "migrationTemplate"],
    queryFn: async () => {
      const data = await getMigrationTemplate(migrationTemplateName)
      return data
    },
    enabled: (query) =>
      !!query?.state?.data?.metadata?.name &&
      query?.state?.data?.status === undefined,
    refetchInterval: 3000,
    notifyOnChangeProps: ["data"],
  })

  const openstackCredsMutation = useMutation({
    mutationFn: (params: unknown) => {
      setValidatingOpenstackCreds(true)
      return postOpenstackCredentials(params)
    },
    onSuccess: (response) => {
      setOpenstackCredName(response?.metadata?.name)
      // Invalidate and refetch
      queryClient.setQueryData(["migrationForm", "openstackCreds"], () => {
        return response
      })
    },
    onError: () => {
      setValidatingOpenstackCreds(false)
      getErrorsUpdater("openstackCreds")(
        "Error validating Openstack credentials"
      )
    },
  })

  const vmwareCredsMutation = useMutation({
    mutationFn: (params: unknown) => {
      setValidatingVmwareCreds(true)
      return postVmwareCredentials(params)
    },
    onSuccess: (response) => {
      setVmwareCredName(response?.metadata?.name)
      // Directly update the cache
      queryClient.setQueryData(["migrationForm", "vmwareCreds"], () => {
        return response
      })
    },
    onError: () => {
      setValidatingOpenstackCreds(false)
      getErrorsUpdater("vmwareCreds")("Error validating VMware credentials")
    },
  })

  const migrationTemplateMutation = useMutation({
    mutationFn: (params: unknown) => {
      return createMigrationTemplate(params)
    },
    onSuccess: (response) => {
      setMigrationTemplateName(response?.metadata?.name)
      // Directly update the cache
      queryClient.setQueryData(["migrationForm", "migrationTemplate"], () => {
        return response
      })
    },
    onError: () => {
      getErrorsUpdater("vms")("Error creating Migration Template")
    },
  })

  useEffect(() => {
    if (isNilOrEmpty(params.vmwareCreds)) return
    // Reset the VMwareCreds object if the user changes the credentials
    setVmwareCredName(null)
    const body = createVmwareCredsJson(params.vmwareCreds)
    vmwareCredsMutation.mutate(body)
  }, [params.vmwareCreds])

  useEffect(() => {
    if (isNilOrEmpty(params.openstackCreds)) return
    // Reset the OpenstackCreds object if the user changes the credentials
    setOpenstackCredName(null)
    const body = createOpenstackCredsJson(params.openstackCreds)
    openstackCredsMutation.mutate(body)
    // createOpenstackCredsResource()
  }, [params.openstackCreds])

  useEffect(() => {
    // Once the Openstack and VMware creds are validated, create the migration template
    const credsValidated =
      vmwareCredName === vmWareCredentials?.metadata?.name &&
      vmWareCredentials?.status?.vmwareValidationStatus === "Succeeded" &&
      openstackCredName === openstackCredentials?.metadata?.name &&
      openstackCredentials?.status?.openstackValidationStatus === "Succeeded"
    if (!credsValidated) return
    const body = createMigrationTemplateJson({
      datacenter: params.vmwareCreds?.datacenter,
      vmwareRef: vmWareCredentials.metadata.name,
      openstackRef: openstackCredentials.metadata.name,
    })
    migrationTemplateMutation.mutate(body)
  }, [
    vmwareCredName,
    openstackCredName,
    vmWareCredentials,
    openstackCredentials,
  ])

  const availableVmwareNetworks = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.networks || [])))
  }, [params.vms])

  const availableVmwareDatastores = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.datastores || [])))
  }, [params.vms])

  const handleSubmit = async () => {
    // setSubmitting(true)
    // // Create NetworkMapping Resource
    // const networkMappingsResource = await createNetworkMapping({
    //   networkMappings: params.networkMappings,
    // })
    // // Create StorageMapping Resource
    // const storageMappingsResource = await createStorageMapping({
    //   storageMappings: params.storageMappings,
    // })
    // // Update MigrationTemplate with NetworkMapping and StorageMapping resource names
    // const templateName = migrationTemplateResource?.metadata?.name
    // const updatedMigrationTemplateResource = await updateMigrationTemplate(
    //   templateName,
    //   {
    //     spec: {
    //       networkMapping: networkMappingsResource.metadata.name,
    //       storageMapping: storageMappingsResource.metadata.name,
    //       ...(selectedMigrationOptions.osType &&
    //         params.osType !== OS_TYPES.AUTO_DETECT && {
    //           osType: params.osType,
    //         }),
    //     },
    //   }
    // )
    // // Create MigrationPlan Resource
    // const vmsToMigrate = (params.vms || []).map((vm) => vm.name)
    // const migrationPlanResource = await createMigrationPlan({
    //   migrationTemplateName: updatedMigrationTemplateResource?.metadata?.name,
    //   virtualmachines: vmsToMigrate,
    //   // Optional Migration Params
    //   type:
    //     selectedMigrationOptions.dataCopyMethod && params.dataCopyMethod
    //       ? params.dataCopyMethod
    //       : "hot",
    //   ...(selectedMigrationOptions.dataCopyStartTime &&
    //     params?.dataCopyStartTime && {
    //       dataCopyStart: params.dataCopyStartTime,
    //     }),
    //   ...(selectedMigrationOptions.cutoverOption &&
    //     params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
    //     params.cutoverStartTime && { vmCutoverStart: params.cutoverStartTime }),
    //   ...(selectedMigrationOptions.cutoverOption &&
    //     params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
    //     params.cutoverEndTime && { vmCutoverEnd: params.cutoverEndTime }),
    //   retry: params.retryOnFailure,
    // })
    // setMigrationPlanResource(migrationPlanResource)
  }

  // const closeAndRedirectToDashboard = useCallback(() => {
  //   setSubmitting(false)
  //   navigate("/dashboard")
  //   window.location.reload()
  //   onClose()
  // }, [navigate, onClose])

  // useEffect(() => {
  //   if (
  //     isNilOrEmpty(migrationPlanResource) ||
  //     !migrationPlanResource.metadata?.name
  //   )
  //     return

  //   let pollingTimeout: NodeJS.Timeout // Declare a variable to store the timeout ID

  //   const pollForMigrations = async () => {
  //     console.log("Polling for migrations")
  //     const migrations = await getMigrationsList(
  //       migrationPlanResource.metadata.name
  //     )
  //     if (migrations.length > 0) {
  //       console.log("Migrations detected. Polling stopped.")
  //       // If migrations are detected, stop polling and trigger the next steps
  //       closeAndRedirectToDashboard()
  //     } else {
  //       // If no migrations are found, continue polling
  //       pollingTimeout = setTimeout(pollForMigrations, 5000)
  //     }
  //   }

  //   // Start polling for migrations
  //   pollForMigrations()

  //   // Cleanup function to stop polling if the component unmounts
  //   return () => {
  //     console.log("Clearing polling", pollingTimeout)
  //     clearTimeout(pollingTimeout) // Properly clear the timeout using the ID
  //     setSubmitting(false)
  //   }
  // }, [
  //   migrationPlanResource,
  //   reloadMigrations,
  //   navigate,
  //   onClose,
  //   closeAndRedirectToDashboard,
  // ])

  const vmwareCredsValidated =
    vmWareCredentials?.status?.vmwareValidationStatus === "Succeeded"
  const openstackCredsValidated =
    openstackCredentials?.status?.openstackValidationStatus === "Succeeded"

  // Validate Selected Migration Options
  const migrationOptionValidated = useMemo(
    () =>
      Object.keys(selectedMigrationOptions).every((key) => {
        if (selectedMigrationOptions[key]) {
          if (
            key === "cutoverOption" &&
            params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW
          ) {
            return (
              params.cutoverStartTime &&
              params.cutoverEndTime &&
              !errors["cutoverStartTime"] &&
              !errors["cutoverEndTime"]
            )
          }
          return params?.[key] && !errors[key]
        }
        return true
      }),
    [selectedMigrationOptions, params, errors]
  )

  const disableSubmit =
    !vmwareCredsValidated ||
    !openstackCredsValidated ||
    isNilOrEmpty(params.vms) ||
    isNilOrEmpty(params.networkMappings) ||
    isNilOrEmpty(params.storageMappings) ||
    !migrationOptionValidated

  const handleOnClose = () => {
    queryClient.removeQueries({ queryKey: ["migrationForm"], exact: false })
    onClose()
  }

  return (
    <StyledDrawer
      anchor="right"
      open={open}
      onClose={handleOnClose}
      ModalProps={{ keepMounted: false }}
    >
      <Header title="Migration Form" />
      <DrawerContent>
        <Box sx={{ display: "grid", gap: 4 }}>
          {/* Step 1 */}
          <SourceAndDestinationEnvStep
            params={params}
            onChange={getParamsUpdater}
            errors={errors}
            validatingVmwareCreds={validatingVmwareCreds}
            validatingOpenstackCreds={validatingOpenstackCreds}
            vmwareCredsValidated={
              vmWareCredentials?.status?.vmwareValidationStatus === "Succeeded"
            }
            openstackCredsValidated={
              openstackCredentials?.status?.openstackValidationStatus ===
              "Succeeded"
            }
          />
          {/* Step 2 */}
          <VmsSelectionStep
            vms={migrationTemplateResource?.status?.vmware}
            onChange={getParamsUpdater}
            error={errors["vms"]}
            loadingVms={
              !isNilOrEmpty(migrationTemplateResource) &&
              migrationTemplateResource?.status === undefined &&
              !errors["vms"]
            }
          />
          {/* Step 3 */}
          <NetworkAndStorageMappingStep
            vmwareNetworks={availableVmwareNetworks}
            vmWareStorage={availableVmwareDatastores}
            openstackNetworks={
              migrationTemplateResource?.status?.openstack?.networks
            }
            openstackStorage={
              migrationTemplateResource?.status?.openstack?.volumeTypes
            }
            params={params}
            onChange={getParamsUpdater}
            networkMappingError={errors["networksMapping"]}
            storageMappingError={errors["storageMapping"]}
          />
          {/* Step 4 */}
          <MigrationOptions
            params={params}
            onChange={getParamsUpdater}
            selectedMigrationOptions={selectedMigrationOptions}
            updateSelectedMigrationOptions={updateSelectedMigrationOptions}
            errors={errors}
            getErrorsUpdater={getErrorsUpdater}
          />
        </Box>
      </DrawerContent>
      <Footer
        submitButtonLabel={"Start Migration"}
        onClose={handleOnClose}
        onSubmit={handleSubmit}
        disableSubmit={disableSubmit || submitting}
        submitting={submitting}
      />
    </StyledDrawer>
  )
}
