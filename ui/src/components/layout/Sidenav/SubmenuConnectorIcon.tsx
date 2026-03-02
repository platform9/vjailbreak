import { useId } from 'react'

export function SubmenuConnectorIcon({ color }: { color: string }) {
  const maskId = useId()
  return (
    <svg width="13" height="12" viewBox="0 0 13 12" fill="none" xmlns="http://www.w3.org/2000/svg">
      <mask id={maskId} fill="white">
        <path d="M0 0H13V12H8C3.58172 12 0 8.41828 0 4V0Z" />
      </mask>
      <path
        d="M0 0H13H0ZM13 13H8C3.02944 13 -1 8.97056 -1 4H1C1 7.86599 4.13401 11 8 11H13V13ZM8 13C3.02944 13 -1 8.97056 -1 4V0H1V4C1 7.86599 4.13401 11 8 11V13ZM13 0V12V0Z"
        fill={color}
        mask={`url(#${maskId})`}
      />
    </svg>
  )
}
