import { useEffect, useRef } from 'react'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

interface UseApplyTemplatePrefillParams {
  open: boolean
  templatePrefill: SavedTemplate | undefined
  pcdData: Array<{ id: string; name?: string }>
  updateParams: (values: Partial<FormValues>) => void
  updateSelectedOptions: (values: Partial<SelectedMigrationOptionsType>) => void
}

// Maps a saved template's fields onto the New Migration form's FormValues, mirroring
// useRetryPrefill.ts's plan/template → FormValues mapping. A blueprint only stores the
// target PCD cluster's name, so pcdData resolves it to the id the form expects — the
// same name→id lookup useRetryPrefill does for retries.
export function useApplyTemplatePrefill({
  open,
  templatePrefill,
  pcdData,
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

    updateParams({
      vmwareCreds: { existingCredName: templatePrefill.sourceVCenter } as FormValues['vmwareCreds'],
      openstackCreds: {
        existingCredName: templatePrefill.destination
      } as FormValues['openstackCreds'],
      pcdCluster: pcd?.id || templatePrefill.targetCluster || '',
      networkMappings: templatePrefill.networkMappings,
      storageMappings: templatePrefill.storageMappings,
      dataCopyMethod: templatePrefill.dataCopyMethod,
      cutoverOption: templatePrefill.cutoverOption,
      osFamily: templatePrefill.osFamily,
      useGPU: templatePrefill.useGPU || false
    })

    updateSelectedOptions({
      dataCopyMethod: true,
      cutoverOption: true,
      useGPU: templatePrefill.useGPU || false
    })
  }, [open, templatePrefill, pcdData, updateParams, updateSelectedOptions])

  return { appliedTemplateName: templatePrefill?.name }
}
