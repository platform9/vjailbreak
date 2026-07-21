import { useEffect, useRef } from 'react'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import type { FormValues, SelectedMigrationOptionsType } from '../types'
import type { SourceDataItem } from './useClusterData'

interface UseApplyTemplatePrefillParams {
  open: boolean
  templatePrefill: SavedTemplate | undefined
  pcdData: Array<{ id: string; name?: string }>
  sourceData: SourceDataItem[]
  currentPcdCluster?: string
  currentVmwareCluster?: string
  updateParams: (values: Partial<FormValues>) => void
  updateSelectedOptions: (values: Partial<SelectedMigrationOptionsType>) => void
}

// Maps a saved template's fields onto the New Migration form's FormValues, mirroring
// useRetryPrefill.ts's plan/template → FormValues mapping. A blueprint only stores the
// target PCD cluster's name, so pcdData resolves it to the id the form expects — the
// same name→id lookup useRetryPrefill does for retries. The source VMware cluster is
// stored the same way (name, not the dropdown's composite id) and needs the matching
// credName + cluster name → id lookup against sourceData before it can be applied.
export function useApplyTemplatePrefill({
  open,
  templatePrefill,
  pcdData,
  sourceData,
  currentPcdCluster,
  currentVmwareCluster,
  updateParams,
  updateSelectedOptions
}: UseApplyTemplatePrefillParams) {
  const appliedRef = useRef<string | null>(null)

  useEffect(() => {
    if (!open || !templatePrefill) {
      appliedRef.current = null
      return
    }
    if (appliedRef.current === templatePrefill.name) return
    appliedRef.current = templatePrefill.name

    const pcd = pcdData.find((p) => p.name === templatePrefill.targetCluster)

    const sourceItem = sourceData.find((item) => item.credName === templatePrefill.sourceVCenter)
    const vmwareClusterId = sourceItem?.clusters.find(
      (cluster) => cluster.name === templatePrefill.sourceCluster
    )?.id

    updateParams({
      vmwareCreds: { existingCredName: templatePrefill.sourceVCenter } as FormValues['vmwareCreds'],
      // pcdData/sourceData may still be loading when the dialog opens — fall back to
      // the raw name (mirrors pcdCluster below); the resync effect swaps in the real
      // id once the matching data finishes loading.
      vmwareCluster: vmwareClusterId || templatePrefill.sourceCluster || '',
      openstackCreds: {
        existingCredName: templatePrefill.destination
      } as FormValues['openstackCreds'],
      pcdCluster: pcd?.id || templatePrefill.targetCluster || '',
      networkMappings: templatePrefill.networkMappings,
      storageMappings: templatePrefill.storageMappings,
      arrayCredsMappings: templatePrefill.arrayCredsMappings,
      dataCopyMethod: templatePrefill.dataCopyMethod,
      dataCopyStartTime: templatePrefill.dataCopyStartTime,
      storageCopyMethod: templatePrefill.storageCopyMethod,
      proxyVMRef: templatePrefill.proxyVMRef,
      cutoverOption: templatePrefill.cutoverOption,
      disconnectSourceNetwork: templatePrefill.disconnectSourceNetwork,
      fallbackToDHCP: templatePrefill.fallbackToDHCP,
      securityGroups: templatePrefill.securityGroups,
      serverGroup: templatePrefill.serverGroup,
      postMigrationScript: templatePrefill.firstBootScript,
      networkPersistence: templatePrefill.networkPersistence,
      removeVMwareTools: templatePrefill.removeVMwareTools,
      imageProfiles: templatePrefill.imageProfiles,
      periodicSyncInterval: templatePrefill.periodicSyncInterval,
      acknowledgeNetworkConflictRisk: templatePrefill.acknowledgeNetworkConflictRisk,
      postMigrationAction: templatePrefill.postMigrationAction,
      osFamily: templatePrefill.osFamily,
      useGPU: templatePrefill.useGPU || false
    })

    updateSelectedOptions({
      dataCopyMethod: true,
      cutoverOption: true,
      dataCopyStartTime: Boolean(templatePrefill.dataCopyStartTime),
      useGPU: templatePrefill.useGPU || false,
      periodicSyncEnabled: templatePrefill.periodicSyncEnabled,
      postMigrationScript: Boolean(templatePrefill.firstBootScript),
      postMigrationAction: templatePrefill.postMigrationAction
        ? {
            suffix: Boolean(templatePrefill.postMigrationAction.suffix),
            folderName: Boolean(templatePrefill.postMigrationAction.folderName),
            renameVm: Boolean(templatePrefill.postMigrationAction.renameVm),
            moveToFolder: Boolean(templatePrefill.postMigrationAction.moveToFolder)
          }
        : undefined
    })
  }, [open, templatePrefill, pcdData, sourceData, updateParams, updateSelectedOptions])

  // pcdData/sourceData often finish loading after the effect above already ran with
  // the raw name fallback. Once they load, swap the raw name for the real id so the
  // cluster dropdowns actually show a selection — without this, "Use template" looked
  // like it worked (no error) but silently left both cluster dropdowns empty.
  useEffect(() => {
    if (!open || !templatePrefill) return
    if (appliedRef.current !== templatePrefill.name) return

    if (
      templatePrefill.targetCluster &&
      currentPcdCluster === templatePrefill.targetCluster &&
      !pcdData.some((p) => p.id === currentPcdCluster)
    ) {
      const match = pcdData.find((p) => p.name === templatePrefill.targetCluster)
      if (match) updateParams({ pcdCluster: match.id })
    }

    if (
      templatePrefill.sourceCluster &&
      currentVmwareCluster === templatePrefill.sourceCluster
    ) {
      const sourceItem = sourceData.find(
        (item) => item.credName === templatePrefill.sourceVCenter
      )
      const match = sourceItem?.clusters.find(
        (cluster) => cluster.name === templatePrefill.sourceCluster
      )
      if (match) updateParams({ vmwareCluster: match.id })
    }
  }, [
    open,
    templatePrefill,
    pcdData,
    sourceData,
    currentPcdCluster,
    currentVmwareCluster,
    updateParams
  ])

  return { appliedTemplateName: templatePrefill?.name }
}
