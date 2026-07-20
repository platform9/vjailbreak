import { Avatar } from '@mui/material'
import WhatshotIcon from '@mui/icons-material/Whatshot'
import AcUnitIcon from '@mui/icons-material/AcUnit'
import ScienceIcon from '@mui/icons-material/Science'
import type { DataCopyMethod } from '../../api/migration-blueprints/types'

export interface TemplateTypeAvatarProps {
  dataCopyMethod: DataCopyMethod
  size?: number
}

// Icon + color mirror the copy-method chip (warning/info/error) so the avatar
// communicates "hot/cold/mock" at a glance instead of a generic template glyph.
const ICON_BY_METHOD: Record<DataCopyMethod, typeof WhatshotIcon> = {
  hot: WhatshotIcon,
  cold: AcUnitIcon,
  mock: ScienceIcon
}

const COLOR_BY_METHOD: Record<DataCopyMethod, { bgcolor: string; color: string }> = {
  hot: { bgcolor: 'warning.light', color: 'warning.dark' },
  cold: { bgcolor: 'info.light', color: 'info.dark' },
  mock: { bgcolor: 'error.light', color: 'error.dark' }
}

export default function TemplateTypeAvatar({ dataCopyMethod, size = 36 }: TemplateTypeAvatarProps) {
  const Icon = ICON_BY_METHOD[dataCopyMethod] ?? WhatshotIcon
  const colors = COLOR_BY_METHOD[dataCopyMethod] ?? COLOR_BY_METHOD.hot

  return (
    <Avatar variant="rounded" sx={{ ...colors, width: size, height: size }}>
      <Icon fontSize="small" />
    </Avatar>
  )
}
