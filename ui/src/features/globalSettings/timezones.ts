export interface TimezoneOption {
  label: string
  value: string
  offset: string
}

export const POPULAR_TIMEZONES: TimezoneOption[] = [
  { value: 'Pacific/Pago_Pago', label: '(UTC-11:00) Pago Pago (Pacific/Pago_Pago)', offset: '-11:00' },
  { value: 'Pacific/Honolulu', label: '(UTC-10:00) Honolulu (Pacific/Honolulu)', offset: '-10:00' },
  { value: 'America/Anchorage', label: '(UTC-09:00) Anchorage (America/Anchorage)', offset: '-09:00' },
  { value: 'America/Vancouver', label: '(UTC-08:00) Vancouver (America/Vancouver)', offset: '-08:00' },
  {
    value: 'America/Los_Angeles',
    label: '(UTC-08:00) Los Angeles (America/Los_Angeles)',
    offset: '-08:00'
  },
  { value: 'America/Tijuana', label: '(UTC-08:00) Tijuana (America/Tijuana)', offset: '-08:00' },
  { value: 'America/Phoenix', label: '(UTC-07:00) Phoenix (America/Phoenix)', offset: '-07:00' },
  { value: 'America/Denver', label: '(UTC-07:00) Denver (America/Denver)', offset: '-07:00' },
  { value: 'America/Edmonton', label: '(UTC-07:00) Edmonton (America/Edmonton)', offset: '-07:00' },
  { value: 'America/Chihuahua', label: '(UTC-07:00) Chihuahua (America/Chihuahua)', offset: '-07:00' },
  { value: 'America/Chicago', label: '(UTC-06:00) Chicago (America/Chicago)', offset: '-06:00' },
  { value: 'America/Winnipeg', label: '(UTC-06:00) Winnipeg (America/Winnipeg)', offset: '-06:00' },
  {
    value: 'America/Mexico_City',
    label: '(UTC-06:00) Mexico City (America/Mexico_City)',
    offset: '-06:00'
  },
  { value: 'America/Guatemala', label: '(UTC-06:00) Guatemala City (America/Guatemala)', offset: '-06:00' },
  {
    value: 'America/New_York',
    label: '(UTC-05:00) New York (America/New_York)',
    offset: '-05:00'
  },
  { value: 'America/Toronto', label: '(UTC-05:00) Toronto (America/Toronto)', offset: '-05:00' },
  {
    value: 'America/Indianapolis',
    label: '(UTC-05:00) Indianapolis (America/Indianapolis)',
    offset: '-05:00'
  },
  { value: 'America/Bogota', label: '(UTC-05:00) Bogotá (America/Bogota)', offset: '-05:00' },
  { value: 'America/Lima', label: '(UTC-05:00) Lima (America/Lima)', offset: '-05:00' },
  { value: 'America/Caracas', label: '(UTC-04:00) Caracas (America/Caracas)', offset: '-04:00' },
  { value: 'America/Halifax', label: '(UTC-04:00) Halifax (America/Halifax)', offset: '-04:00' },
  { value: 'America/Puerto_Rico', label: '(UTC-04:00) Puerto Rico (America/Puerto_Rico)', offset: '-04:00' },
  { value: 'America/Santiago', label: '(UTC-04:00) Santiago (America/Santiago)', offset: '-04:00' },
  {
    value: 'America/Argentina/Buenos_Aires',
    label: '(UTC-03:00) Buenos Aires (America/Argentina/Buenos_Aires)',
    offset: '-03:00'
  },
  {
    value: 'America/Sao_Paulo',
    label: '(UTC-03:00) São Paulo (America/Sao_Paulo)',
    offset: '-03:00'
  },
  { value: 'America/Montevideo', label: '(UTC-03:00) Montevideo (America/Montevideo)', offset: '-03:00' },
  { value: 'Atlantic/Azores', label: '(UTC-01:00) Azores (Atlantic/Azores)', offset: '-01:00' },
  {
    value: 'UTC',
    label: '(UTC+00:00) Coordinated Universal Time (UTC)',
    offset: '+00:00'
  },
  { value: 'Europe/London', label: '(UTC+00:00) London (Europe/London)', offset: '+00:00' },
  { value: 'Europe/Dublin', label: '(UTC+00:00) Dublin (Europe/Dublin)', offset: '+00:00' },
  { value: 'Africa/Casablanca', label: '(UTC+00:00) Casablanca (Africa/Casablanca)', offset: '+00:00' },
  { value: 'Europe/Lisbon', label: '(UTC+00:00) Lisbon (Europe/Lisbon)', offset: '+00:00' },
  { value: 'Africa/Lagos', label: '(UTC+01:00) Lagos (Africa/Lagos)', offset: '+01:00' },
  { value: 'Europe/Paris', label: '(UTC+01:00) Paris (Europe/Paris)', offset: '+01:00' },
  { value: 'Europe/Berlin', label: '(UTC+01:00) Berlin (Europe/Berlin)', offset: '+01:00' },
  { value: 'Europe/Madrid', label: '(UTC+01:00) Madrid (Europe/Madrid)', offset: '+01:00' },
  { value: 'Europe/Rome', label: '(UTC+01:00) Rome (Europe/Rome)', offset: '+01:00' },
  { value: 'Europe/Warsaw', label: '(UTC+01:00) Warsaw (Europe/Warsaw)', offset: '+01:00' },
  { value: 'Africa/Johannesburg', label: '(UTC+02:00) Johannesburg (Africa/Johannesburg)', offset: '+02:00' },
  { value: 'Europe/Athens', label: '(UTC+02:00) Athens (Europe/Athens)', offset: '+02:00' },
  { value: 'Europe/Kyiv', label: '(UTC+02:00) Kyiv (Europe/Kyiv)', offset: '+02:00' },
  { value: 'Asia/Jerusalem', label: '(UTC+02:00) Jerusalem (Asia/Jerusalem)', offset: '+02:00' },
  { value: 'Europe/Istanbul', label: '(UTC+03:00) Istanbul (Europe/Istanbul)', offset: '+03:00' },
  { value: 'Europe/Moscow', label: '(UTC+03:00) Moscow (Europe/Moscow)', offset: '+03:00' },
  { value: 'Africa/Nairobi', label: '(UTC+03:00) Nairobi (Africa/Nairobi)', offset: '+03:00' },
  { value: 'Asia/Riyadh', label: '(UTC+03:00) Riyadh (Asia/Riyadh)', offset: '+03:00' },
  { value: 'Asia/Baghdad', label: '(UTC+03:00) Baghdad (Asia/Baghdad)', offset: '+03:00' },
  { value: 'Asia/Tehran', label: '(UTC+03:30) Tehran (Asia/Tehran)', offset: '+03:30' },
  { value: 'Asia/Dubai', label: '(UTC+04:00) Dubai (Asia/Dubai)', offset: '+04:00' },
  { value: 'Asia/Baku', label: '(UTC+04:00) Baku (Asia/Baku)', offset: '+04:00' },
  { value: 'Asia/Tbilisi', label: '(UTC+04:00) Tbilisi (Asia/Tbilisi)', offset: '+04:00' },
  { value: 'Asia/Kabul', label: '(UTC+04:30) Kabul (Asia/Kabul)', offset: '+04:30' },
  { value: 'Asia/Karachi', label: '(UTC+05:00) Karachi (Asia/Karachi)', offset: '+05:00' },
  { value: 'Asia/Tashkent', label: '(UTC+05:00) Tashkent (Asia/Tashkent)', offset: '+05:00' },
  { value: 'Asia/Kolkata', label: '(UTC+05:30) Kolkata (Asia/Kolkata)', offset: '+05:30' },
  { value: 'Asia/Colombo', label: '(UTC+05:30) Colombo (Asia/Colombo)', offset: '+05:30' },
  { value: 'Asia/Kathmandu', label: '(UTC+05:45) Kathmandu (Asia/Kathmandu)', offset: '+05:45' },
  { value: 'Asia/Dhaka', label: '(UTC+06:00) Dhaka (Asia/Dhaka)', offset: '+06:00' },
  { value: 'Asia/Almaty', label: '(UTC+06:00) Almaty (Asia/Almaty)', offset: '+06:00' },
  { value: 'Asia/Yangon', label: '(UTC+06:30) Yangon (Asia/Yangon)', offset: '+06:30' },
  { value: 'Asia/Bangkok', label: '(UTC+07:00) Bangkok (Asia/Bangkok)', offset: '+07:00' },
  { value: 'Asia/Jakarta', label: '(UTC+07:00) Jakarta (Asia/Jakarta)', offset: '+07:00' },
  {
    value: 'Asia/Ho_Chi_Minh',
    label: '(UTC+07:00) Ho Chi Minh City (Asia/Ho_Chi_Minh)',
    offset: '+07:00'
  },
  { value: 'Asia/Singapore', label: '(UTC+08:00) Singapore (Asia/Singapore)', offset: '+08:00' },
  { value: 'Asia/Shanghai', label: '(UTC+08:00) Shanghai (Asia/Shanghai)', offset: '+08:00' },
  { value: 'Asia/Hong_Kong', label: '(UTC+08:00) Hong Kong (Asia/Hong_Kong)', offset: '+08:00' },
  { value: 'Asia/Taipei', label: '(UTC+08:00) Taipei (Asia/Taipei)', offset: '+08:00' },
  { value: 'Australia/Perth', label: '(UTC+08:00) Perth (Australia/Perth)', offset: '+08:00' },
  { value: 'Asia/Seoul', label: '(UTC+09:00) Seoul (Asia/Seoul)', offset: '+09:00' },
  { value: 'Asia/Tokyo', label: '(UTC+09:00) Tokyo (Asia/Tokyo)', offset: '+09:00' },
  { value: 'Australia/Darwin', label: '(UTC+09:30) Darwin (Australia/Darwin)', offset: '+09:30' },
  { value: 'Australia/Adelaide', label: '(UTC+09:30) Adelaide (Australia/Adelaide)', offset: '+09:30' },
  { value: 'Australia/Brisbane', label: '(UTC+10:00) Brisbane (Australia/Brisbane)', offset: '+10:00' },
  { value: 'Australia/Sydney', label: '(UTC+10:00) Sydney (Australia/Sydney)', offset: '+10:00' },
  { value: 'Australia/Melbourne', label: '(UTC+10:00) Melbourne (Australia/Melbourne)', offset: '+10:00' },
  { value: 'Pacific/Guam', label: '(UTC+10:00) Guam (Pacific/Guam)', offset: '+10:00' },
  { value: 'Pacific/Noumea', label: '(UTC+11:00) Nouméa (Pacific/Noumea)', offset: '+11:00' },
  {
    value: 'Pacific/Auckland',
    label: '(UTC+12:00) Auckland (Pacific/Auckland)',
    offset: '+12:00'
  },
  { value: 'Pacific/Fiji', label: '(UTC+12:00) Fiji (Pacific/Fiji)', offset: '+12:00' },
  { value: 'Pacific/Tongatapu', label: '(UTC+13:00) Tonga (Pacific/Tongatapu)', offset: '+13:00' }
]
