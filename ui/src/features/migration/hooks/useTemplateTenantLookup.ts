import { useMemo } from 'react'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'

// Maps an OpenStack creds object name (SavedTemplate.destination / spec.pcdRef) to its
// project/tenant name, for the "Tenant" row and the templates list's grey
// source/destination summary box. React Query dedups the underlying request, so calling
// this from many cards/rows costs one network call, not one per caller.
export function useTemplateTenantLookup(): Record<string, string> {
  const { data: openstackCreds } = useOpenstackCredentialsQuery()

  return useMemo(() => {
    const lookup: Record<string, string> = {}
    for (const cred of openstackCreds || []) {
      const name = cred.metadata?.name
      const projectName = cred.spec?.projectName
      if (name && projectName) lookup[name] = projectName
    }
    return lookup
  }, [openstackCreds])
}
