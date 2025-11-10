import { useQuery } from "@tanstack/react-query"
import { getVMwareMachines } from "src/api/vmware-machines/vmwareMachines"
import { VmData } from "src/api/migration-templates/model"
import {
  fetchRdmDisksMap,
  mapToVmDataWithRdm,
} from "src/api/rdm-disks/rdmDiskUtils"
import { VMwareMachine } from "src/api/vmware-machines/model"

export const VMWARE_MACHINES_BASE_KEY = "vmwaremachines"

interface UseVMwareMachinesQueryProps {
  vmwareCredsValidated?: boolean
  openstackCredsValidated?: boolean
  enabled?: boolean
  sessionId?: string
  vmwareCredName?: string
  clusterName?: string
  vmwareClusterDisplayName?: string
}

export const useVMwareMachinesQuery = ({
  vmwareCredsValidated = false,
  openstackCredsValidated = false,
  enabled = true,
  sessionId = "default",
  vmwareCredName,
  clusterName,
  vmwareClusterDisplayName,
}: UseVMwareMachinesQueryProps = {}) => {
  const areCredsValidated = vmwareCredsValidated && openstackCredsValidated
  const queryEnabled = enabled && areCredsValidated

  const queryKey = [
    VMWARE_MACHINES_BASE_KEY,
    sessionId,
    vmwareCredsValidated,
    openstackCredsValidated,
    vmwareCredName,
    clusterName,
  ]

  return useQuery({
    queryKey,
    queryFn: async (): Promise<VmData[]> => {
      if (!areCredsValidated) {
        return []
      }
      const [vmResponse, rdmDisksMap] = await Promise.all([
        getVMwareMachines(undefined, vmwareCredName),
        fetchRdmDisksMap(),
      ])

      let filteredItems: VMwareMachine[] = vmResponse.items;
      
      if (vmwareClusterDisplayName && vmwareClusterDisplayName !== "NO CLUSTER") {
        filteredItems = vmResponse.items.filter(vm => 
          vm.spec.vms.clusterName === vmwareClusterDisplayName
        );
      }

      // Use RDM-aware mapping function
      return mapToVmDataWithRdm(filteredItems, rdmDisksMap)
    },
    enabled: queryEnabled,
    refetchOnWindowFocus: false,
    staleTime: 0, // Consider data immediately stale to ensure fresh fetch on new sessions
    // Don't keep previous data when credentials change or form reopens
    placeholderData: [],
  })
}
