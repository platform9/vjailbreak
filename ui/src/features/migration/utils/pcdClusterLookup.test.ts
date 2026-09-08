import { describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { findPcdClusterByName } from './pcdClusterLookup'
import { useApplyTemplatePrefill } from '../hooks/useApplyTemplatePrefill'
import type { SavedTemplate } from '../api/migration-blueprints/types'
import type { SourceDataItem } from '../hooks/useClusterData'

// PCD cluster names are unique per credential, not globally, so every
// name -> id resolution must be scoped to the owning credential.
//
// This file owns the two things a browser cannot reach: the helper's branches (absent
// credential, missing label, empty/undefined inputs) and the template-apply hook. The
// retry path is asserted once, in the browser, by RET-010 in e2e/migration/retry.spec.ts
// — not repeated here.

// Two credentials/tenants each exposing a cluster called "shared-cluster". Tenant A is
// listed first, so a name-only match returns it and any cred-scoped match must not.
const PCD_DATA = [
  { id: 'id-tenant-a', name: 'shared-cluster', openstackCredName: 'creds-a' },
  { id: 'id-tenant-b', name: 'shared-cluster', openstackCredName: 'creds-b' },
  { id: 'id-unique', name: 'only-on-b', openstackCredName: 'creds-b' }
]

describe('findPcdClusterByName', () => {
  it('picks the cluster belonging to the given credential, not the first name match', () => {
    expect(findPcdClusterByName(PCD_DATA, 'shared-cluster', 'creds-b')?.id).toBe('id-tenant-b')
  })

  it('still resolves when only one credential exposes the name', () => {
    expect(findPcdClusterByName(PCD_DATA, 'only-on-b', 'creds-b')?.id).toBe('id-unique')
  })

  it('falls back to a name match when the credential owns no cluster by that name', () => {
    // Credential was renamed or the migration references creds that no longer exist:
    // selecting *a* cluster beats leaving the dropdown empty.
    expect(findPcdClusterByName(PCD_DATA, 'shared-cluster', 'creds-gone')?.id).toBe('id-tenant-a')
  })

  it('falls back to a name match when no credential is known', () => {
    expect(findPcdClusterByName(PCD_DATA, 'shared-cluster', undefined)?.id).toBe('id-tenant-a')
  })

  it('resolves clusters missing the openstackcreds label via the fallback', () => {
    const unlabelled = [{ id: 'id-unlabelled', name: 'legacy-cluster' }]
    expect(findPcdClusterByName(unlabelled, 'legacy-cluster', 'creds-a')?.id).toBe('id-unlabelled')
  })

  it('returns undefined for an unknown cluster name', () => {
    expect(findPcdClusterByName(PCD_DATA, 'does-not-exist', 'creds-a')).toBeUndefined()
  })

  it('returns undefined when no cluster name is given', () => {
    expect(findPcdClusterByName(PCD_DATA, undefined, 'creds-a')).toBeUndefined()
    expect(findPcdClusterByName(PCD_DATA, '', 'creds-a')).toBeUndefined()
  })

  it('returns undefined when there are no clusters to search', () => {
    expect(findPcdClusterByName([], 'shared-cluster', 'creds-a')).toBeUndefined()
  })
})

// ─── Call site: template apply ────────────────────────────────────────────────

const makeTemplate = (overrides: Partial<SavedTemplate> = {}): SavedTemplate =>
  ({
    name: 'template-1',
    displayName: 'Template 1',
    targetCluster: 'shared-cluster',
    destination: 'creds-b',
    sourceVCenter: 'vmware-1',
    sourceCluster: 'source-cluster',
    networkMappings: [],
    storageMappings: [],
    arrayCredsMappings: [],
    customMetadata: [],
    securityGroups: [],
    imageProfiles: [],
    ...overrides
  }) as unknown as SavedTemplate

const applyTemplate = (templatePrefill: SavedTemplate, pcdData = PCD_DATA) => {
  const updateParams = vi.fn()
  renderHook(() =>
    useApplyTemplatePrefill({
      open: true,
      templatePrefill,
      pcdData,
      sourceData: [] as SourceDataItem[],
      updateParams,
      updateSelectedOptions: vi.fn()
    })
  )
  return updateParams.mock.calls[0]?.[0] ?? {}
}

describe('useApplyTemplatePrefill — target PCD cluster', () => {
  it("selects the cluster owned by the template's destination credential", () => {
    expect(applyTemplate(makeTemplate()).pcdCluster).toBe('id-tenant-b')
  })

  it("selects the other credential's cluster when the template targets it", () => {
    expect(applyTemplate(makeTemplate({ destination: 'creds-a' })).pcdCluster).toBe('id-tenant-a')
  })

  it('falls back to the raw cluster name when pcdData has not loaded yet', () => {
    // The hook's second effect swaps in the real id once pcdData arrives.
    expect(applyTemplate(makeTemplate(), []).pcdCluster).toBe('shared-cluster')
  })
})
