import { describe, expect, it } from 'vitest'
import {
  getRollingAreSelectedMigrationOptionsConfigured,
  getRollingIsSubmitDisabled,
  getRollingStep6Complete,
  getRollingStep6HasErrors
} from 'src/features/migration/utils/rollingMigrationValidation'
import { CUTOVER_TYPES } from 'src/features/migration/constants'

describe('rollingMigrationValidation', () => {
  it('getRollingStep6HasErrors returns false when not touched', () => {
    expect(
      getRollingStep6HasErrors({
        isTouched: false,
        selectedMigrationOptions: {
          dataCopyMethod: false,
          dataCopyStartTime: true,
          cutoverOption: false,
          postMigrationScript: false,
          osFamily: false
        },
        params: {},
        fieldErrors: { dataCopyStartTime: 'bad' }
      })
    ).toBe(false)
  })

  it('getRollingAreSelectedMigrationOptionsConfigured requires values for enabled options', () => {
    const selectedMigrationOptions = {
      dataCopyMethod: false,
      dataCopyStartTime: true,
      cutoverOption: true,
      postMigrationScript: true,
      osFamily: false,
      useGPU: true,
      useFlavorless: true,
      postMigrationAction: { renameVm: true, suffix: true }
    }

    const params = {
      dataCopyStartTime: '2026-01-01T00:00:00Z',
      cutoverOption: CUTOVER_TYPES.TIME_WINDOW,
      cutoverStartTime: '2026-01-02T00:00:00Z',
      cutoverEndTime: '2026-01-02T01:00:00Z',
      postMigrationScript: 'echo ok',
      useGPU: false,
      useFlavorless: false,
      postMigrationAction: { suffix: 'new' }
    }

    expect(
      getRollingAreSelectedMigrationOptionsConfigured({
        selectedMigrationOptions: selectedMigrationOptions as any,
        params: params as any,
        fieldErrors: {}
      })
    ).toBe(true)
  })

  it('getRollingStep6Complete matches legacy rule (touched + configured OR toggles + no errors)', () => {
    const params = {
      disconnectSourceNetwork: false,
      fallbackToDHCP: true,
      networkPersistence: false
    }

    expect(
      getRollingStep6Complete({
        isTouched: true,
        areSelectedMigrationOptionsConfigured: false,
        params: params as any,
        step6HasErrors: false
      })
    ).toBe(true)

    expect(
      getRollingStep6Complete({
        isTouched: true,
        areSelectedMigrationOptionsConfigured: false,
        params: params as any,
        step6HasErrors: true
      })
    ).toBe(false)
  })

  it('getRollingIsSubmitDisabled returns true when basic requirements missing', () => {
    expect(
      getRollingIsSubmitDisabled({
        sourceCluster: '',
        destinationPCD: 'pcd',
        selectedMaasConfig: {},
        selectedVMsLength: 1,
        submitting: false,
        params: { storageCopyMethod: 'normal' } as any,
        selectedMigrationOptions: {
          dataCopyMethod: false,
          dataCopyStartTime: false,
          cutoverOption: false,
          postMigrationScript: false,
          osFamily: false
        } as any,
        fieldErrors: {},
        availableVmwareNetworks: ['net1'],
        availableVmwareDatastores: ['ds1'],
        networkMappings: [{ source: 'net1', target: 'pcd-net1' }],
        storageMappings: [{ source: 'ds1', target: 'pcd-ds1' }],
        arrayCredsMappings: []
      })
    ).toBe(true)
  })
})
