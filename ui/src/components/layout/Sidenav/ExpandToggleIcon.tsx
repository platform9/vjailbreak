import { ExpandToggleBar, ExpandToggleIconRoot } from './Sidenav.styles'

export function ExpandToggleIcon({ expanded }: { expanded: boolean }) {
  return (
    <ExpandToggleIconRoot expanded={expanded} aria-hidden>
      <ExpandToggleBar expanded={expanded} />
      <ExpandToggleBar vertical expanded={expanded} />
    </ExpandToggleIconRoot>
  )
}
