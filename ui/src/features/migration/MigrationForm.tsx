import { Box, Drawer, styled } from "@mui/material"
import { useQueryClient } from "@tanstack/react-query"
import axios from "axios"
import { useEffect, useMemo, useState, useCallback } from "react"
import { useNavigate } from "react-router-dom"
import { createMigrationPlanJson } from "src/api/migration-plans/helpers"
import { postMigrationPlan } from "src/api/migration-plans/migrationPlans"
import { MigrationPlan } from "src/api/migration-plans/model"
import { createMigrationTemplateJson } from "src/api/migration-templates/helpers"
import {
  getMigrationTemplate,
  patchMigrationTemplate,
  postMigrationTemplate,
  deleteMigrationTemplate,
} from "src/api/migration-templates/migrationTemplates"
import { MigrationTemplate, VmData } from "src/api/migration-templates/model"
import { getMigrations } from "src/api/migrations/migrations"
import { Migration } from "src/api/migrations/model"
import { createNetworkMappingJson } from "src/api/network-mapping/helpers"
import { postNetworkMapping } from "src/api/network-mapping/networkMappings"
import { OpenstackCreds } from "src/api/openstack-creds/model"
import {
  getOpenstackCredentials,
  deleteOpenstackCredentials,
} from "src/api/openstack-creds/openstackCreds"
import { createStorageMappingJson } from "src/api/storage-mappings/helpers"
import { postStorageMapping } from "src/api/storage-mappings/storageMappings"
import { VMwareCreds } from "src/api/vmware-creds/model"
import {
  getVmwareCredentials,
  deleteVmwareCredentials,
} from "src/api/vmware-creds/vmwareCreds"
import { THREE_SECONDS } from "src/constants"
import { MIGRATIONS_QUERY_KEY } from "src/hooks/api/useMigrationsQuery"
import { VMWARE_MACHINES_BASE_KEY } from "src/hooks/api/useVMwareMachinesQuery"
import { useInterval } from "src/hooks/useInterval"
import useParams from "src/hooks/useParams"
import { isNilOrEmpty } from "src/utils"
import Footer from "../../components/forms/Footer"
import Header from "../../components/forms/Header"
import MigrationOptions from "./MigrationOptionsAlt"
import NetworkAndStorageMappingStep from "./NetworkAndStorageMappingStep"
import SourceAndDestinationEnvStep from "./SourceAndDestinationEnvStep"
import VmsSelectionStep from "./VmsSelectionStep"
import { CUTOVER_TYPES, OS_TYPES } from "./constants"
import { uniq } from "ramda"
import { flatten } from "ramda"
import { useKeyboardSubmit } from "src/hooks/ui/useKeyboardSubmit"

const stringsCompareFn = (a, b) =>
  a.toLowerCase().localeCompare(b.toLowerCase())

const StyledDrawer = styled(Drawer)(() => ({
  "& .MuiDrawer-paper": {
    display: "grid",
    gridTemplateRows: "max-content 1fr max-content",
    width: "1200px",
    maxWidth: "90vw", // For responsiveness on smaller screens
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
    existingCredName?: string
    credentialName?: string
  }
  openstackCreds?: {
    OS_AUTH_URL: string
    OS_DOMAIN_NAME: string
    OS_USERNAME: string
    OS_PASSWORD: string
    OS_REGION_NAME: string
    OS_TENANT_NAME: string
    existingCredName?: string
    credentialName?: string
    OS_INSECURE?: boolean
  }
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

export type FieldErrors = { [formId: string]: string }

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
  const [error, setError] = useState<{ title: string; message: string } | null>(
    null
  )
  // Theses are the errors that will be displayed on the form
  const { params: fieldErrors, getParamsUpdater: getFieldErrorsUpdater } =
    useParams<FieldErrors>({})
  const queryClient = useQueryClient()

  // Migration Options - Checked or Unchecked state
  const {
    params: selectedMigrationOptions,
    getParamsUpdater: updateSelectedMigrationOptions,
  } = useParams<SelectedMigrationOptionsType>(defaultMigrationOptions)

  // Form Statuses
  const [submitting, setSubmitting] = useState(false)

  // Migration Resources
  const [vmwareCredentials, setVmwareCredentials] = useState<
    VMwareCreds | undefined
  >(undefined)
  const [openstackCredentials, setOpenstackCredentials] = useState<
    OpenstackCreds | undefined
  >(undefined)
  const [migrationTemplate, setMigrationTemplate] = useState<
    MigrationTemplate | undefined
  >(undefined)
  const [migrationPlan, setMigrationPlan] = useState<MigrationPlan | undefined>(
    undefined
  )
  const [migrations, setMigrations] = useState<Migration[] | undefined>(
    undefined
  )

  // Generate a unique session ID for this form instance
  const [sessionId] = useState(() => `form-session-${Date.now()}`);

  const vmwareCredsValidated =
    vmwareCredentials?.status?.vmwareValidationStatus === "Succeeded"

  const openstackCredsValidated =
    openstackCredentials?.status?.openstackValidationStatus === "Succeeded"

  // Polling Conditions
  const shouldPollMigrationTemplate = !migrationTemplate?.metadata?.name

  const shouldPollMigrationPlan =
    !!migrationPlan?.metadata?.name && migrationPlan?.status === undefined

  // Update this effect to only handle existing credential selection
  useEffect(() => {
    const fetchCredentials = async () => {
      if (!params.vmwareCreds || !params.vmwareCreds.existingCredName) return;

      try {
        const existingCredName = params.vmwareCreds.existingCredName;
        const response = await getVmwareCredentials(existingCredName);
        setVmwareCredentials(response);
      } catch (error) {
        console.error("Error fetching existing VMware credentials:", error);
        getFieldErrorsUpdater("vmwareCreds")(
          "Error fetching VMware credentials: " + (axios.isAxiosError(error) ? error?.response?.data?.message : error),
        )
      }
    }

    if (isNilOrEmpty(params.vmwareCreds)) return
    setVmwareCredentials(undefined)
    getFieldErrorsUpdater("vmwareCreds")("")
    fetchCredentials()
  }, [params.vmwareCreds, getFieldErrorsUpdater])

  // Update this effect to only handle existing credential selection
  useEffect(() => {
    const fetchCredentials = async () => {
      if (!params.openstackCreds || !params.openstackCreds.existingCredName) return;

      try {
        const existingCredName = params.openstackCreds.existingCredName;
        const response = await getOpenstackCredentials(existingCredName);
        setOpenstackCredentials(response);
      } catch (error) {
        console.error("Error fetching existing OpenStack credentials:", error);
        getFieldErrorsUpdater("openstackCreds")(
          "Error fetching OpenStack credentials: " + (axios.isAxiosError(error) ? error?.response?.data?.message : error),
        )
      }
    }

    if (isNilOrEmpty(params.openstackCreds)) return
    // Reset the OpenstackCreds object if the user changes the credentials
    setOpenstackCredentials(undefined)
    getFieldErrorsUpdater("openstackCreds")("")
    fetchCredentials()
  }, [params.openstackCreds, getFieldErrorsUpdater])

  useEffect(() => {
    const createMigrationTemplate = async () => {
      const body = createMigrationTemplateJson({
        datacenter: params.vmwareCreds?.datacenter,
        vmwareRef: vmwareCredentials?.metadata.name,
        openstackRef: openstackCredentials?.metadata.name,
      })
      const response = await postMigrationTemplate(body)
      setMigrationTemplate(response)
    }

    if (!vmwareCredsValidated || !openstackCredsValidated) return
    createMigrationTemplate()
  }, [
    vmwareCredsValidated,
    openstackCredsValidated,
    params.vmwareCreds?.datacenter,
    vmwareCredentials?.metadata.name,
    openstackCredentials?.metadata.name,
  ])

  // Keep original fetchMigrationTemplate for fetching OpenStack networks and volume types
  const fetchMigrationTemplate = async () => {
    try {
      const updatedMigrationTemplate = await getMigrationTemplate(
        migrationTemplate?.metadata?.name
      )
      setMigrationTemplate(updatedMigrationTemplate)
    } catch (err) {
      console.error("Error retrieving migration templates", err)
      getFieldErrorsUpdater("migrationTemplate")(
        "Error retrieving migration templates"
      )
    }
  }

  useInterval(
    async () => {
      console.log("migrationTemplate", migrationTemplate?.metadata?.name)
      if (shouldPollMigrationTemplate) {
        try {
          fetchMigrationTemplate()
        } catch (err) {
          console.error("Error retrieving migration templates", err)
          getFieldErrorsUpdater("migrationTemplate")(
            "Error retrieving migration templates"
          )
        }
      }
    },
    THREE_SECONDS,
    shouldPollMigrationTemplate
  )

  useEffect(() => {
    if (vmwareCredsValidated && openstackCredsValidated) return
    // Reset all the migration resources if the user changes the credentials
    setMigrationTemplate(undefined)
  }, [vmwareCredsValidated, openstackCredsValidated])

  const availableVmwareNetworks = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.networks || []))).sort(
      stringsCompareFn
    )
  }, [params.vms])

  const availableVmwareDatastores = useMemo(() => {
    if (params.vms === undefined) return []
    return uniq(flatten(params.vms.map((vm) => vm.datastores || []))).sort(
      stringsCompareFn
    )
  }, [params.vms])

  const createNetworkMapping = async (networkMappingParams) => {
    const body = createNetworkMappingJson({
      networkMappings: networkMappingParams,
    })

    try {
      const data = postNetworkMapping(body)
      return data
    } catch (err) {
      setError({
        title: "Error creating network mapping",
        message: axios.isAxiosError(err) ? err?.response?.data?.message : "",
      })
      getFieldErrorsUpdater("networksMapping")(
        "Error creating network mapping : " + (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
    }
  }

  const createStorageMapping = async (storageMappingsParams) => {
    const body = createStorageMappingJson({
      storageMappings: storageMappingsParams,
    })
    try {
      const data = postStorageMapping(body)
      return data
    } catch (err) {
      console.error("Error creating storage mapping", err)
      setError({
        title: "Error creating storage mapping",
        message: axios.isAxiosError(err) ? err?.response?.data?.message : "",
      })
      getFieldErrorsUpdater("storageMapping")(
        "Error creating storage mapping : " + (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )
    }
  }

  const updateMigrationTemplate = async (
    migrationTemplate,
    networkMappings,
    storageMappings
  ) => {
    const migrationTemplateName = migrationTemplate?.metadata?.name
    const updatedMigrationTemplateFields = {
      spec: {
        networkMapping: networkMappings.metadata.name,
        storageMapping: storageMappings.metadata.name,
        ...(selectedMigrationOptions.osType &&
          params.osType !== OS_TYPES.AUTO_DETECT && {
          osType: params.osType,
        }),
      },
    }
    try {
      const data = await patchMigrationTemplate(
        migrationTemplateName,
        updatedMigrationTemplateFields
      )
      return data
    } catch (err) {
      setError({
        title: "Error updating migration template",
        message: axios.isAxiosError(err) ? err?.response?.data?.message : "",
      })
    }
  }

  const createMigrationPlan = async (updatedMigrationTemplate) => {
    const vmsToMigrate = (params.vms || []).map((vm) => vm.name)
    const migrationFields = {
      migrationTemplateName: updatedMigrationTemplate?.metadata?.name,
      virtualMachines: vmsToMigrate,
      type:
        selectedMigrationOptions.dataCopyMethod && params.dataCopyMethod
          ? params.dataCopyMethod
          : "hot",
      ...(selectedMigrationOptions.dataCopyStartTime &&
        params?.dataCopyStartTime && {
        dataCopyStart: params.dataCopyStartTime,
      }),
      ...(selectedMigrationOptions.cutoverOption &&
        params.cutoverOption === CUTOVER_TYPES.ADMIN_INITIATED && { adminInitiatedCutOver: true }),

      ...(selectedMigrationOptions.cutoverOption &&
        params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
        params.cutoverStartTime && { vmCutoverStart: params.cutoverStartTime }),
      ...(selectedMigrationOptions.cutoverOption &&
        params.cutoverOption === CUTOVER_TYPES.TIME_WINDOW &&
        params.cutoverEndTime && { vmCutoverEnd: params.cutoverEndTime }),
      retry: params.retryOnFailure,
    }
    const body = createMigrationPlanJson(migrationFields)
    try {
      const data = await postMigrationPlan(body)
      return data
    } catch (err) {
      // Handle error
      console.error("Error creating migration plan", err)
      setError({
        title: "Error creating migration plan",
        message: axios.isAxiosError(err) ? err?.response?.data?.message : "",
      })
      getFieldErrorsUpdater("migrationPlan")(
        "Error creating migration plan : " + (axios.isAxiosError(err) ? err?.response?.data?.message : err)
      )

    }
  }

  const handleSubmit = useCallback(async () => {
    setSubmitting(true)
    setError(null)

    // Create NetworkMapping
    const networkMappings = await createNetworkMapping(params.networkMappings)

    // Create StorageMapping
    const storageMappings = await createStorageMapping(params.storageMappings)

    if (!networkMappings || !storageMappings) {
      setSubmitting(false)
      return
    }

    // Update MigrationTemplate with NetworkMapping and StorageMapping resource names
    const updatedMigrationTemplate = await updateMigrationTemplate(
      migrationTemplate,
      networkMappings,
      storageMappings
    )

    // Create MigrationPlan
    const migrationPlanResource = await createMigrationPlan(
      updatedMigrationTemplate
    )
    setMigrationPlan(migrationPlanResource)
  }, [
    params.networkMappings,
    params.storageMappings,
    migrationTemplate,
    createNetworkMapping,
    createStorageMapping,
    updateMigrationTemplate,
    createMigrationPlan
  ]);

  useInterval(
    async () => {
      if (shouldPollMigrationPlan) {
        try {
          const response = await getMigrations(migrationPlan?.metadata?.name)
          setMigrations(response)
        } catch (error) {
          console.error("Error getting MigrationPlan", { error })
          setSubmitting(false)
        }
      }
    },
    THREE_SECONDS,
    shouldPollMigrationPlan
  )

  useEffect(() => {
    if (migrations && migrations.length > 0 && !error) {
      setSubmitting(false)
      queryClient.invalidateQueries({ queryKey: MIGRATIONS_QUERY_KEY })
      onClose()
      navigate("/dashboard?tab=migrations")
    }
  }, [migrations, error, onClose, navigate, queryClient])

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
              !fieldErrors["cutoverStartTime"] &&
              !fieldErrors["cutoverEndTime"]
            )
          }
          return params?.[key] && !fieldErrors[key]
        }
        return true
      }),
    [selectedMigrationOptions, params, fieldErrors]
  )

  const disableSubmit =
    !vmwareCredsValidated ||
    !openstackCredsValidated ||
    isNilOrEmpty(params.vms) ||
    isNilOrEmpty(params.networkMappings) ||
    isNilOrEmpty(params.storageMappings) ||
    // Check if all networks are mapped
    availableVmwareNetworks.some(network =>
      !params.networkMappings?.some(mapping => mapping.source === network)) ||
    // Check if all datastores are mapped
    availableVmwareDatastores.some(datastore =>
      !params.storageMappings?.some(mapping => mapping.source === datastore)) ||
    !migrationOptionValidated

  const sortedOpenstackNetworks = useMemo(
    () =>
      (openstackCredentials?.status?.openstack?.networks || []).sort(
        stringsCompareFn
      ),
    [openstackCredentials?.status?.openstack?.networks]
  )
  const sortedOpenstackVolumeTypes = useMemo(
    () =>
      (openstackCredentials?.status?.openstack?.volumeTypes || []).sort(
        stringsCompareFn
      ),
    [openstackCredentials?.status?.openstack?.volumeTypes]
  )

  const handleClose = useCallback(async () => {
    try {
      setMigrationTemplate(undefined)
      setVmwareCredentials(undefined)
      setOpenstackCredentials(undefined)
      setError(null)

      // Invalidate and remove queries when form closes
      queryClient.invalidateQueries({ queryKey: [VMWARE_MACHINES_BASE_KEY, sessionId] })
      queryClient.removeQueries({ queryKey: [VMWARE_MACHINES_BASE_KEY, sessionId] })

      onClose()
      // Delete migration template if it exists
      if (migrationTemplate?.metadata?.name) {
        await deleteMigrationTemplate(migrationTemplate.metadata.name)
      }

      if (vmwareCredentials?.metadata?.name && !params.vmwareCreds?.existingCredName) {
        await deleteVmwareCredentials(vmwareCredentials.metadata.name)
      }

      if (openstackCredentials?.metadata?.name && !params.openstackCreds?.existingCredName) {
        await deleteOpenstackCredentials(openstackCredentials.metadata.name)
      }

    } catch (err) {
      console.error("Error cleaning up resources", err)
      onClose()
    }
  }, [migrationTemplate, vmwareCredentials, openstackCredentials, queryClient, sessionId, onClose, params.vmwareCreds, params.openstackCreds])

  // Handle keyboard events
  useKeyboardSubmit({
    open,
    isSubmitDisabled: disableSubmit || submitting,
    onSubmit: handleSubmit,
    onClose: handleClose
  });

  return (
    <StyledDrawer
      anchor="right"
      open={open}
      onClose={handleClose}
      ModalProps={{ keepMounted: false }}
    >
      <Header title="Migration Form" />
      <DrawerContent>
        <Box sx={{ display: "grid", gap: 4 }}>
          {/* Step 1 */}
          <SourceAndDestinationEnvStep
            onChange={getParamsUpdater}
            errors={fieldErrors}
          />
          {/* Step 2 - VM selection now manages its own data fetching with unique session ID */}
          <VmsSelectionStep
            onChange={getParamsUpdater}
            error={fieldErrors["vms"]}
            open={open}
            vmwareCredsValidated={vmwareCredsValidated}
            openstackCredsValidated={openstackCredsValidated}
            sessionId={sessionId}
            openstackFlavors={openstackCredentials?.spec?.flavors}
            vmwareCredName={params.vmwareCreds?.existingCredName}
            openstackCredName={params.openstackCreds?.existingCredName}
          />
          {/* Step 3 */}
          <NetworkAndStorageMappingStep
            vmwareNetworks={availableVmwareNetworks}
            vmWareStorage={availableVmwareDatastores}
            openstackNetworks={sortedOpenstackNetworks}
            openstackStorage={sortedOpenstackVolumeTypes}
            params={params}
            onChange={getParamsUpdater}
            networkMappingError={fieldErrors["networksMapping"]}
            storageMappingError={fieldErrors["storageMapping"]}
          />
          {/* Step 4 */}
          <MigrationOptions
            params={params}
            onChange={getParamsUpdater}
            selectedMigrationOptions={selectedMigrationOptions}
            updateSelectedMigrationOptions={updateSelectedMigrationOptions}
            errors={fieldErrors}
            getErrorsUpdater={getFieldErrorsUpdater}
            stepNumber="4"
          />
        </Box>
      </DrawerContent>
      <Footer
        submitButtonLabel={"Start Migration"}
        onClose={handleClose}
        onSubmit={handleSubmit}
        disableSubmit={disableSubmit || submitting}
        submitting={submitting}
      />
    </StyledDrawer>
  )
}
