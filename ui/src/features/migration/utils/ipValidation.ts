// Pure IP validation and parsing utilities shared across migration form components.
// No React imports, no side effects.

export const IPV4_MATCH_REGEX =
  /(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)/g

export const IPV4_FULL_REGEX =
  /^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$/

/** Returns the first IPv4 address found in a string, or '' if none. */
export const extractFirstIPv4 = (value: string): string => {
  if (!value) return ''
  const matches = value.match(IPV4_MATCH_REGEX)
  return matches?.[0] || ''
}

/** Returns true when the string contains more than one IPv4 address. */
export const hasMultipleIPv4 = (value: string): boolean => {
  if (!value) return false
  const matches = value.match(IPV4_MATCH_REGEX)
  return (matches?.length || 0) > 1
}

/** Splits a comma-separated IP string into a trimmed, filtered array. */
export const parseIpList = (value: string): string[] => {
  const trimmed = value.trim()
  if (!trimmed) return []
  return trimmed.split(/\s*,\s*/).filter((v) => v !== '')
}

/** Returns true when the string contains more than one comma-separated entry. */
export const hasMultipleIpEntries = (value: string): boolean => parseIpList(value).length > 1

/** Returns true when every comma-separated entry is a valid IPv4 address. */
export const isValidIPAddressList = (value: string): boolean => {
  const ips = parseIpList(value)
  if (ips.length === 0) return false
  return ips.every((ip) => IPV4_FULL_REGEX.test(ip))
}

/** Returns true when value is a single valid IPv4 address. */
export const isValidIPAddress = (value: string): boolean => IPV4_FULL_REGEX.test(value)
