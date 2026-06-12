export interface TimezoneOption {
  label: string
  value: string
  offset: string
}

type SupportedValuesOf = (key: 'timeZone') => string[]

const getOffsetValue = (timeZone: string): { label: string; offset: string } => {
  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone,
      timeZoneName: 'shortOffset',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false
    }).formatToParts(new Date())

    const tzName = parts.find((part) => part.type === 'timeZoneName')?.value ?? 'GMT'

    if (tzName === 'GMT' || tzName === 'UTC') {
      return { label: 'UTC+00:00', offset: '+00:00' }
    }

    const match = tzName.match(/^GMT(?<sign>[+-])(?<hours>\d{1,2})(?::?(?<minutes>\d{2}))?$/)
    if (!match?.groups?.sign || !match.groups.hours) {
      return { label: 'UTC+00:00', offset: '+00:00' }
    }

    const sign = match.groups.sign
    const hours = match.groups.hours.padStart(2, '0')
    const minutes = (match.groups.minutes ?? '00').padStart(2, '0')
    return { label: `UTC${sign}${hours}:${minutes}`, offset: `${sign}${hours}:${minutes}` }
  } catch {
    return { label: 'UTC+00:00', offset: '+00:00' }
  }
}

const buildTimezoneOptions = (): TimezoneOption[] => {
  const supportedValuesOf = (Intl as typeof Intl & { supportedValuesOf?: SupportedValuesOf })
    .supportedValuesOf

  const values = supportedValuesOf ? supportedValuesOf('timeZone') : []

  return [...new Set([...values, 'UTC'])]
    .sort((a, b) => a.localeCompare(b))
    .map((value) => {
      const { label, offset } = getOffsetValue(value)
      return {
        value,
        offset,
        label: `(${label}) ${value}`
      }
    })
}

export const POPULAR_TIMEZONES: TimezoneOption[] = buildTimezoneOptions()
