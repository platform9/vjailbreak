import { describe, expect, it } from 'vitest'

import { getDisableSubmit, getStepCompletion } from './validation'

describe('migration validation utils', () => {
  it('disables submit when creds are not validated', () => {
    expect(
      getDisableSubmit({
        vmwareCredsValidated: false,
        openstackCredsValidated: true,
        params: { vms: [], networkMappings: [], vmwareCluster: 'c1', pcdCluster: 'c2' },
        availableVmwareNetworks: [],
        availableVmwareDatastores: [],
        fieldErrors: {},
        migrationOptionValidated: true,
        vmValidation: { hasError: false },
        rdmValidation: { hasValidationError: false, hasConfigError: false }
      })
    ).toBe(true)
  })

  it('disables submit when mappings are incomplete', () => {
    expect(
      getDisableSubmit({
        vmwareCredsValidated: true,
        openstackCredsValidated: true,
        params: {
          vms: [{}],
          networkMappings: [{ source: 'net1', target: 't1' }],
          vmwareCluster: 'c1',
          pcdCluster: 'c2',
          storageCopyMethod: 'normal',
          storageMappings: [{ source: 'ds1', target: 'pool1' }]
        },
        availableVmwareNetworks: ['net1', 'net2'],
        availableVmwareDatastores: ['ds1'],
        fieldErrors: {},
        migrationOptionValidated: true,
        vmValidation: { hasError: false },
        rdmValidation: { hasValidationError: false, hasConfigError: false }
      })
    ).toBe(true)
  })

  it('enables submit when all requirements are satisfied', () => {
    expect(
      getDisableSubmit({
        vmwareCredsValidated: true,
        openstackCredsValidated: true,
        params: {
          vms: [{}],
          networkMappings: [
            { source: 'net1', target: 't1' },
            { source: 'net2', target: 't2' }
          ],
          vmwareCluster: 'c1',
          pcdCluster: 'c2',
          storageCopyMethod: 'normal',
          storageMappings: [{ source: 'ds1', target: 'pool1' }]
        },
        availableVmwareNetworks: ['net1', 'net2'],
        availableVmwareDatastores: ['ds1'],
        fieldErrors: {},
        migrationOptionValidated: true,
        vmValidation: { hasError: false },
        rdmValidation: { hasValidationError: false, hasConfigError: false }
      })
    ).toBe(false)
  })

  it('computes step completion consistently', () => {
    const result = getStepCompletion({
      params: {
        vmwareCluster: 'cred:dc:cluster',
        pcdCluster: 'pcd',
        vms: [{}],
        networkMappings: [{ source: 'net1', target: 't1' }],
        storageCopyMethod: 'normal',
        storageMappings: [{ source: 'ds1', target: 'pool1' }],
        securityGroups: ['default']
      },
      fieldErrors: {},
      availableVmwareNetworks: ['net1'],
      availableVmwareDatastores: ['ds1'],
      vmValidation: { hasError: false },
      rdmValidation: { hasConfigError: false }
    })

    expect(result.isStep1Complete).toBe(true)
    expect(result.isStep2Complete).toBe(true)
    expect(result.isStep3Complete).toBe(true)
    expect(result.step4Complete).toBe(true)
  })
})
