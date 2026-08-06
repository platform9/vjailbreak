import { describe, expect, it } from 'vitest'
import { createMigrationPlanJson } from './helpers'

type MigrationStrategyShape = {
  type?: string
  dataOnly?: boolean
  copyOnly?: boolean
}

const strategyOf = (body: ReturnType<typeof createMigrationPlanJson>): MigrationStrategyShape =>
  (body.spec as { migrationStrategy: MigrationStrategyShape }).migrationStrategy

const baseParams = {
  name: 'test-plan',
  migrationTemplateName: 'test-template',
  virtualMachines: ['vm-a'],
  type: 'cold'
}

describe('createMigrationPlanJson migration mode flags', () => {
  it('omits copyOnly when it is not requested', () => {
    const strategy = strategyOf(createMigrationPlanJson(baseParams))
    expect(strategy).not.toHaveProperty('copyOnly')
  })

  it('omits copyOnly when it is explicitly false', () => {
    const strategy = strategyOf(createMigrationPlanJson({ ...baseParams, copyOnly: false }))
    expect(strategy).not.toHaveProperty('copyOnly')
  })

  it('sets copyOnly on the migration strategy when requested', () => {
    const strategy = strategyOf(createMigrationPlanJson({ ...baseParams, copyOnly: true }))
    expect(strategy.copyOnly).toBe(true)
  })

  // copyOnly (skip conversion) and dataOnly (skip VM creation) are independent, so both must be
  // able to travel together in a single plan.
  it('carries copyOnly and dataOnly together', () => {
    const strategy = strategyOf(
      createMigrationPlanJson({ ...baseParams, copyOnly: true, dataOnly: true })
    )
    expect(strategy.copyOnly).toBe(true)
    expect(strategy.dataOnly).toBe(true)
  })

  it('keeps copyOnly out of the strategy when only dataOnly is set', () => {
    const strategy = strategyOf(createMigrationPlanJson({ ...baseParams, dataOnly: true }))
    expect(strategy.dataOnly).toBe(true)
    expect(strategy).not.toHaveProperty('copyOnly')
  })
})
