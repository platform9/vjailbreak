import { useEffect, useRef } from 'react'
import type { SavedTemplate } from '../mock-templates/types'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

interface UseApplyTemplatePrefillParams {
  open: boolean
  templatePrefill: SavedTemplate | undefined
  updateParams: (values: Partial<FormValues>) => void
  updateSelectedOptions: (values: Partial<SelectedMigrationOptionsType>) => void
}

// Maps a saved template's fields onto the New Migration form's FormValues, mirroring
// the shape of useRetryPrefill.ts's template → FormValues mapping. Runs synchronously
// (no credential/mapping existence lookups) while templates are mock-data-backed;
// swap in stale-reference resolution once the real API lands (plan.md FR-009).
export function useApplyTemplatePrefill({
  open,
  templatePrefill,
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

    updateParams({
      vmwareCreds: { existingCredName: templatePrefill.sourceVCenter } as FormValues['vmwareCreds'],
      openstackCreds: {
        existingCredName: templatePrefill.destination
      } as FormValues['openstackCreds'],
      vmwareCluster: templatePrefill.vmwareCluster || '',
      pcdCluster: templatePrefill.pcdCluster || '',
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
  }, [open, templatePrefill, updateParams, updateSelectedOptions])

  return { appliedTemplateName: templatePrefill?.name }
}
