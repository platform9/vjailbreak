import { PCDNetworkInfo } from 'src/api/openstack-creds/model'

interface NetworkMappingTarget {
  target?: string
}

const SIMPLE_NETWORK_TAG = 'simple_network'

const hasSimpleNetworkTag = (network?: PCDNetworkInfo): boolean =>
  Array.isArray(network?.tags) && network.tags.includes(SIMPLE_NETWORK_TAG)

export const hasAnyLayer2Network = (openstackNetworks?: PCDNetworkInfo[]): boolean => {
  if (!Array.isArray(openstackNetworks) || openstackNetworks.length === 0) {
    return false
  }

  return openstackNetworks.some(hasSimpleNetworkTag)
}

export const hasSelectedLayer2Network = (
  networkMappings?: NetworkMappingTarget[],
  openstackNetworks?: PCDNetworkInfo[]
): boolean => {
  if (!Array.isArray(networkMappings) || networkMappings.length === 0) {
    return false
  }

  if (!Array.isArray(openstackNetworks) || openstackNetworks.length === 0) {
    return false
  }

  const targetNetworkNames = new Set(
    networkMappings.map((mapping) => mapping.target).filter((target): target is string => Boolean(target))
  )

  if (targetNetworkNames.size === 0) {
    return false
  }

  return openstackNetworks.some(
    (network) => targetNetworkNames.has(network.name) && hasSimpleNetworkTag(network)
  )
}
