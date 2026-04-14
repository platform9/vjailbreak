import { describe, expect, it } from 'vitest'
import {
  getAreSelectedMigrationOptionsConfigured,
  getHasAnyMigrationOptionSelected,
  getStep5Complete
} from 'src/features/migration/utils/migrationOptionsValidation'
import type { FieldErrors, FormValues, SelectedMigrationOptionsType } from 'src/features/migration/types'
import { CUTOVER_TYPES } from 'src/features/migration/constants'

describe('migrationOptionsValidation', () => {
  it('getHasAnyMigrationOptionSelected returns false when nothing is selected', () => {
    const selectedMigrationOptions = {
      dataCopyMethod: false,
      dataCopyStartTime: false,
      cutoverOption: false,
      cutoverStartTime: false,
      cutoverEndTime: false,
      postMigrationScript: false,
      postMigrationAction: {
        suffix: false,
        folderName: false,
        renameVm: false,
        moveToFolder: false
      },
      useGPU: false,
      useFlavorless: false,
      periodicSyncEnabled: false
    } as SelectedMigrationOptionsType

    expect(
      getHasAnyMigrationOptionSelected({
        selectedMigrationOptions,
        removeVMwareTools: false
      })
    ).toBe(false)
  })

  it('getHasAnyMigrationOptionSelected returns true when removeVMwareTools is set', () => {
    const selectedMigrationOptions = {
      dataCopyMethod: false,
      dataCopyStartTime: false,
      cutoverOption: false,
      cutoverStartTime: false,
      cutoverEndTime: false,
      postMigrationScript: false,
      postMigrationAction: {
        suffix: false,
        folderName: false,
        renameVm: false,
        moveToFolder: false
      }
    } as SelectedMigrationOptionsType

    expect(
      getHasAnyMigrationOptionSelected({
        selectedMigrationOptions,
        removeVMwareTools: true
      })
    ).toBe(true)
  })

  it('getAreSelectedMigrationOptionsConfigured requires required values for enabled options', () => {
    const selectedMigrationOptions = {
      dataCopyMethod: false,
      dataCopyStartTime: true,
      cutoverOption: true,
      postMigrationScript: true,
      periodicSyncEnabled: true
    } as SelectedMigrationOptionsType

    const params = {
      dataCopyStartTime: '2026-01-01T00:00:00Z',
      cutoverOption: CUTOVER_TYPES.TIME_WINDOW,
      cutoverStartTime: '2026-01-02T00:00:00Z',
      cutoverEndTime: '2026-01-02T01:00:00Z',
      periodicSyncInterval: '10m',
      postMigrationScript: 'echo hello',
      useGPU: false,
      useFlavorless: false,
      postMigrationAction: {}
    } as unknown as FormValues

    const fieldErrors = {} as FieldErrors

    expect(
      getAreSelectedMigrationOptionsConfigured({
        hasAnyMigrationOptionSelected: true,
        selectedMigrationOptions,
        params,
        fieldErrors
      })
    ).toBe(true)
  })

  it('getStep5Complete matches legacy behavior (touched + configured OR toggles + no errors)', () => {
    const params = {
      disconnectSourceNetwork: false,
      fallbackToDHCP: true,
      networkPersistence: false,
      removeVMwareTools: false
    } as unknown as FormValues

    expect(
      getStep5Complete({
        isTouched: true,
        areSelectedMigrationOptionsConfigured: false,
        params,
        step5HasErrors: false
      })
    ).toBe(true)

    expect(
      getStep5Complete({
        isTouched: true,
        areSelectedMigrationOptionsConfigured: false,
        params,
        step5HasErrors: true
      })
    ).toBe(false)
  })
})
