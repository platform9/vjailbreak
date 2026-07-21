import { Avatar } from '@mui/material'
import { alpha, type Theme } from '@mui/material/styles'
import LocalFireDepartmentOutlinedIcon from '@mui/icons-material/LocalFireDepartmentOutlined'
import AcUnitOutlinedIcon from '@mui/icons-material/AcUnitOutlined'
import ScienceOutlinedIcon from '@mui/icons-material/ScienceOutlined'
import type { DataCopyMethod } from '../../api/migration-blueprints/types'
import { DATA_COPY_METHOD_CHIP_COLOR } from '../../utils/templateLabels'

export interface TemplateTypeAvatarProps {
  dataCopyMethod: DataCopyMethod
  size?: number
}

// Icon + color mirror the copy-method chip (warning/info/error) so the avatar
// communicates "hot/cold/mock" at a glance instead of a generic template glyph.
const ICON_BY_METHOD: Record<DataCopyMethod, typeof LocalFireDepartmentOutlinedIcon> = {
  hot: LocalFireDepartmentOutlinedIcon,
  cold: AcUnitOutlinedIcon,
  mock: ScienceOutlinedIcon
}

export default function TemplateTypeAvatar({ dataCopyMethod, size = 36 }: TemplateTypeAvatarProps) {
  const Icon = ICON_BY_METHOD[dataCopyMethod] ?? ICON_BY_METHOD.hot
  const colorKey = DATA_COPY_METHOD_CHIP_COLOR[dataCopyMethod] ?? DATA_COPY_METHOD_CHIP_COLOR.hot

  return (
    <Avatar
      variant="rounded"
      sx={(theme: Theme) => ({
        width: size,
        height: size,
        bgcolor: alpha(theme.palette[colorKey].main, theme.palette.mode === 'dark' ? 0.24 : 0.12),
        color:
          theme.palette.mode === 'dark'
            ? theme.palette[colorKey].light
            : theme.palette[colorKey].dark
      })}
    >
      <Icon fontSize="small" />
    </Avatar>
  )
}
