import { describe, expect, it } from 'vitest'
import { CUTOVER_TYPES } from '../constants'
import { cutoverOptionLabel, DATA_COPY_METHOD_LABEL } from './templateLabels'

describe('DATA_COPY_METHOD_LABEL', () => {
  it('has a label for every data copy method', () => {
    expect(DATA_COPY_METHOD_LABEL.hot).toBe('Hot copy')
    expect(DATA_COPY_METHOD_LABEL.cold).toBe('Cold copy')
    expect(DATA_COPY_METHOD_LABEL.mock).toBe('Mock copy')
  })
})

describe('cutoverOptionLabel', () => {
  it('labels immediate cutover', () => {
    expect(cutoverOptionLabel(CUTOVER_TYPES.IMMEDIATE)).toBe('Immediate cutover')
  })

  it('labels admin-initiated cutover', () => {
    expect(cutoverOptionLabel(CUTOVER_TYPES.ADMIN_INITIATED)).toBe('Admin cutover')
  })

  it('labels time-window cutover', () => {
    expect(cutoverOptionLabel(CUTOVER_TYPES.TIME_WINDOW)).toBe('Time window cutover')
  })

  it('defaults to immediate cutover for undefined input', () => {
    expect(cutoverOptionLabel(undefined)).toBe('Immediate cutover')
  })

  it('defaults to immediate cutover for an unrecognized value', () => {
    expect(cutoverOptionLabel('nonsense')).toBe('Immediate cutover')
  })
})
