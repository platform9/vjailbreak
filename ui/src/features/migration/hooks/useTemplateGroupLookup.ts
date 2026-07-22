import { useMemo } from 'react'
import { useOpenstackCredentialsQuery } from 'src/hooks/api/useOpenstackCredentialsQuery'

export interface TemplateGroupNameLookup {
  securityGroups: Record<string, string>
  serverGroups: Record<string, string>
}

// Maps an OpenStack creds object name (SavedTemplate.destination) to id->name lookups
// for its security groups and server groups, so the template detail drawer can show
// names instead of raw ids (templates only persist ids). React Query dedups the
// underlying request, so calling this from many cards/rows costs one network call.
export function useTemplateGroupLookup(): Record<string, TemplateGroupNameLookup> {
  const { data: openstackCreds } = useOpenstackCredentialsQuery()

  return useMemo(() => {
    const lookup: Record<string, TemplateGroupNameLookup> = {}
    for (const cred of openstackCreds || []) {
      const name = cred.metadata?.name
      if (!name) continue
      const securityGroups: Record<string, string> = {}
      for (const group of cred.status?.openstack?.securityGroups || []) {
        securityGroups[group.id] = group.name
      }
      const serverGroups: Record<string, string> = {}
      for (const group of cred.status?.openstack?.serverGroups || []) {
        serverGroups[group.id] = group.name
      }
      lookup[name] = { securityGroups, serverGroups }
    }
    return lookup
  }, [openstackCreds])
}
