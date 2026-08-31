import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import { deriveSourceCatalog, useFormValidation } from './useFormValidation'
import type { FormValues, SelectedMigrationOptionsType } from '../types'
import type { VmData } from '../api/migration-templates/model'

const baseSelectedMigrationOptions: SelectedMigrationOptionsType = {
  dataCopyMethod: false,
  dataCopyStartTime: false,
  cutoverOption: false,
  cutoverStartTime: false,
  cutoverEndTime: false,
  postMigrationScript: false
}

const renderStep5 = (params: Partial<FormValues>, touchedTagsMetadata: boolean) =>
  renderHook(() =>
    useFormValidation({
      params,
      fieldErrors: {},
      selectedMigrationOptions: baseSelectedMigrationOptions,
      vmwareCredsValidated: true,
      openstackCredsValidated: true,
      rdmDisks: [],
      openstackCredentials: undefined,
      touchedSections: { options: false, tagsMetadata: touchedTagsMetadata }
    })
  )

describe('useFormValidation - Tags & Metadata step (step5Complete)', () => {
  it('is incomplete when untouched and nothing set', () => {
    const { result } = renderStep5({}, false)
    expect(result.current.step5Complete).toBe(false)
  })

  it('goes complete once preserveSourceTags is enabled', () => {
    const { result } = renderStep5({ preserveSourceTags: true }, true)
    expect(result.current.step5Complete).toBe(true)
  })

  it('goes back to incomplete when preserveSourceTags is disabled again, even though the section was touched', () => {
    // Regression test for GH-2209: toggling the option off with no custom metadata
    // rows set must clear the step checkmark, not stay "complete" from having
    // been touched.
    const { result } = renderStep5({ preserveSourceTags: false, customMetadata: [] }, true)
    expect(result.current.step5Complete).toBe(false)
  })

  it('stays complete when custom metadata rows are filled even if preserveSourceTags is off', () => {
    const { result } = renderStep5(
      { preserveSourceTags: false, customMetadata: [{ key: 'env', value: 'prod' }] },
      true
    )
    expect(result.current.step5Complete).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Template authoring mode
// ---------------------------------------------------------------------------

const vm = (name: string, networks: string[], datastores: string[]): VmData => ({
  id: name,
  name,
  networks,
  datastores
})

describe('deriveSourceCatalog', () => {
  it('returns an empty list when there is no VM inventory', () => {
    expect(deriveSourceCatalog(undefined, 'networks')).toEqual([])
    expect(deriveSourceCatalog([], 'datastores')).toEqual([])
  })

  it('unions the requested field across every VM', () => {
    const vms = [
      vm('web-01', ['VM Network'], ['datastore1']),
      vm('db-01', ['Mgmt Network'], ['datastore-nfs'])
    ]

    expect(deriveSourceCatalog(vms, 'networks')).toEqual(['Mgmt Network', 'VM Network'])
    expect(deriveSourceCatalog(vms, 'datastores')).toEqual(['datastore-nfs', 'datastore1'])
  })

  it('de-duplicates names shared by several VMs', () => {
    const vms = [
      vm('web-01', ['VM Network', 'Mgmt Network'], ['datastore1']),
      vm('web-02', ['VM Network'], ['datastore1'])
    ]

    expect(deriveSourceCatalog(vms, 'networks')).toEqual(['Mgmt Network', 'VM Network'])
    expect(deriveSourceCatalog(vms, 'datastores')).toEqual(['datastore1'])
  })

  it('sorts case-insensitively so dropdown order is stable', () => {
    const vms = [vm('web-01', ['zeta-net', 'Alpha-net', 'mid-net'], [])]
    expect(deriveSourceCatalog(vms, 'networks')).toEqual(['Alpha-net', 'mid-net', 'zeta-net'])
  })

  it('tolerates VMs with the field absent', () => {
    const vms: VmData[] = [{ id: 'a', name: 'a', datastores: ['datastore1'] }, vm('b', ['VM Network'], [])]

    expect(deriveSourceCatalog(vms, 'networks')).toEqual(['VM Network'])
    expect(deriveSourceCatalog(vms, 'datastores')).toEqual(['datastore1'])
  })

  it('yields a catalog that is a superset of any VM subset — the template-mode guarantee', () => {
    const clusterVms = [
      vm('web-01', ['VM Network'], ['datastore1']),
      vm('db-01', ['Mgmt Network'], ['datastore-nfs']),
      vm('app-01', ['vMotion'], ['datastore2'])
    ]
    const clusterCatalog = deriveSourceCatalog(clusterVms, 'networks')
    const subsetCatalog = deriveSourceCatalog([clusterVms[1]], 'networks')

    expect(subsetCatalog.every((name) => clusterCatalog.includes(name))).toBe(true)
  })
})

const renderTemplateMode = (
  params: Partial<FormValues>,
  clusterVms: VmData[] | undefined,
  templateMode = true
) =>
  renderHook(() =>
    useFormValidation({
      params,
      fieldErrors: {},
      selectedMigrationOptions: baseSelectedMigrationOptions,
      vmwareCredsValidated: true,
      openstackCredsValidated: true,
      rdmDisks: [],
      openstackCredentials: undefined,
      touchedSections: { options: false, tagsMetadata: false },
      templateMode,
      clusterVms
    })
  )

describe('useFormValidation - template authoring mode', () => {
  const clusterVms = [
    vm('web-01', ['VM Network'], ['datastore1']),
    vm('db-01', ['Mgmt Network'], ['datastore-nfs'])
  ]

  it('sources the mappable catalog from the cluster inventory, not from selected VMs', () => {
    // No VMs are ever selected while authoring a template.
    const { result } = renderTemplateMode({ vms: undefined }, clusterVms)

    expect(result.current.availableVmwareNetworks).toEqual(['Mgmt Network', 'VM Network'])
    expect(result.current.availableVmwareDatastores).toEqual(['datastore-nfs', 'datastore1'])
  })

  it('still sources from selected VMs when not in template mode', () => {
    const { result } = renderTemplateMode(
      { vms: [vm('only-this', ['VM Network'], ['datastore1'])] },
      clusterVms,
      false
    )

    expect(result.current.availableVmwareNetworks).toEqual(['VM Network'])
    expect(result.current.availableVmwareDatastores).toEqual(['datastore1'])
  })

  it('drops the Select VMs step from the rail', () => {
    const { result } = renderTemplateMode({}, clusterVms)

    expect(result.current.sectionNavItems.map((i) => i.id)).not.toContain('select-vms')
  })

  it('keeps the Select VMs step in the rail for a real migration', () => {
    const { result } = renderTemplateMode({}, clusterVms, false)

    expect(result.current.sectionNavItems.map((i) => i.id)).toContain('select-vms')
  })

  it('treats a partial mapping as a complete step — a template need not map everything', () => {
    const { result } = renderTemplateMode(
      { networkMappings: [{ source: 'VM Network', target: 'Physnet1' }] },
      clusterVms
    )

    // 'Mgmt Network' and both datastores are deliberately left unmapped.
    expect(result.current.unmappedNetworksCount).toBe(1)
    expect(result.current.isStep3Complete).toBe(true)
  })

  it('reports the step incomplete only when nothing at all has been mapped', () => {
    const { result } = renderTemplateMode({}, clusterVms)
    expect(result.current.isStep3Complete).toBe(false)
  })

  it('counts an array-creds-only mapping as configured', () => {
    const { result } = renderTemplateMode(
      {
        storageCopyMethod: 'StorageAcceleratedCopy',
        arrayCredsMappings: [{ source: 'datastore1', target: 'pure-array-1' }]
      },
      clusterVms
    )

    expect(result.current.isStep3Complete).toBe(true)
  })

  it('requires every source mapped for a real migration, unlike template mode', () => {
    const selected = [vm('web-01', ['VM Network', 'Mgmt Network'], ['datastore1'])]
    const { result } = renderTemplateMode(
      { vms: selected, networkMappings: [{ source: 'VM Network', target: 'Physnet1' }] },
      undefined,
      false
    )

    expect(result.current.isStep3Complete).toBe(false)
  })
})

// Only the "normal" copy path opens VDDK, so the other methods must not be blocked on it.
describe('useFormValidation - VDDK requirement by copy method', () => {
  const renderWithVddk = (
    storageCopyMethod: FormValues['storageCopyMethod'],
    vddkUploaded: boolean | undefined
  ) =>
    renderHook(() =>
      useFormValidation({
        params: { storageCopyMethod },
        fieldErrors: {},
        selectedMigrationOptions: baseSelectedMigrationOptions,
        vmwareCredsValidated: true,
        openstackCredsValidated: true,
        rdmDisks: [],
        openstackCredentials: undefined,
        touchedSections: { options: false, tagsMetadata: false },
        vddkUploaded
      })
    )

  it('raises the requirement for the normal copy method when VDDK is missing', () => {
    const { result } = renderWithVddk('normal', false)
    expect(result.current.vddkRequirementError).not.toBe('')
    expect(result.current.disableSubmit).toBe(true)
    expect(result.current.step3HasErrors).toBe(true)
  })

  it('does not raise it for StorageAcceleratedCopy when VDDK is missing', () => {
    const { result } = renderWithVddk('StorageAcceleratedCopy', false)
    expect(result.current.vddkRequirementError).toBe('')
    expect(result.current.step3HasErrors).toBe(false)
  })

  it('does not raise it for HotAdd when VDDK is missing', () => {
    const { result } = renderWithVddk('HotAdd', false)
    expect(result.current.vddkRequirementError).toBe('')
    expect(result.current.step3HasErrors).toBe(false)
  })

  it('does not raise it for the normal method once VDDK is uploaded', () => {
    const { result } = renderWithVddk('normal', true)
    expect(result.current.vddkRequirementError).toBe('')
  })

  it('fails open while the VDDK status is unknown', () => {
    const { result } = renderWithVddk('normal', undefined)
    expect(result.current.vddkRequirementError).toBe('')
  })
})
