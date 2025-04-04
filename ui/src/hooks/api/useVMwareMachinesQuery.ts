import { useQuery } from "@tanstack/react-query"
import {
  getVMwareMachines,
  mapToVmData,
} from "src/api/vmware-machines/vmwareMachines"
import { VmData } from "src/api/migration-templates/model"

export const VMWARE_MACHINES_BASE_KEY = "vmwaremachines"

interface UseVMwareMachinesQueryProps {
  vmwareCredsValidated?: boolean
  openstackCredsValidated?: boolean
  enabled?: boolean
  sessionId?: string
  vmwareCredName?: string
}

export const useVMwareMachinesQuery = ({
  vmwareCredsValidated = false,
  openstackCredsValidated = false,
  enabled = true,
  sessionId = "default",
  vmwareCredName,
}: UseVMwareMachinesQueryProps = {}) => {
  const areCredsValidated = vmwareCredsValidated && openstackCredsValidated
  const queryEnabled = enabled && areCredsValidated

  const queryKey = [
    VMWARE_MACHINES_BASE_KEY,
    sessionId,
    vmwareCredsValidated,
    openstackCredsValidated,
    vmwareCredName,
  ]

  return useQuery({
    queryKey,
    queryFn: async (): Promise<VmData[]> => {
      if (!areCredsValidated) {
        return []
      }
      const response = await getVMwareMachines(undefined, vmwareCredName)
      return mapToVmData(response.items)
    },
    enabled: queryEnabled,
    refetchOnWindowFocus: false,
    staleTime: 0, // Consider data immediately stale to ensure fresh fetch on new sessions
    // Don't keep previous data when credentials change or form reopens
    placeholderData: [],
  })
}
