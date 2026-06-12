export const isValidNtpServer = (value: string): boolean => {
  const v = value.trim()
  if (!v) return false
  if (v.includes('://') || v.includes('/')) return false

  const ipv4Match = v.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/)
  if (ipv4Match) {
    const octets = ipv4Match.slice(1).map((part) => Number(part))
    return octets.every((octet) => Number.isInteger(octet) && octet >= 0 && octet <= 255)
  }

  if (!/^[a-zA-Z0-9.-]+$/.test(v)) return false
  if (v.startsWith('.') || v.endsWith('.') || v.includes('..')) return false

  return v.split('.').every((label) => {
    if (!label) return false
    if (label.length > 63) return false
    if (!/^[a-zA-Z0-9-]+$/.test(label)) return false
    if (label.startsWith('-') || label.endsWith('-')) return false
    return true
  })
}
