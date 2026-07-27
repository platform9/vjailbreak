import { describe, expect, it } from 'vitest'
import { renderHook } from '@testing-library/react'
import { useFormValidation } from './useFormValidation'
import type { FormValues, SelectedMigrationOptionsType } from '../types'

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
