import { Phase } from 'src/api/migrations/model'
import { PRIMARY_MAIN } from 'src/theme/theme'

export const DRAWER_WIDTH = 280
export const DRAWER_WIDTH_COLLAPSED = 72

export const SUBMENU_CONNECTOR_GREY = '#e6e6ea'
export const SUBMENU_CONNECTOR_BLUE = PRIMARY_MAIN
export const SUBMENU_ROW_HEIGHT_PX = 32
export const FIRST_LEVEL_ROW_HEIGHT_PX = 44
export const SUBMENU_SPINE_X_PX = 22
export const SUBMENU_CONNECTOR_SVG_LEFT_PX = SUBMENU_SPINE_X_PX - 4
export const SUBMENU_TEXT_INDENT_PX = 36

export const ACTIVE_MIGRATION_PHASES = new Set<Phase>([
  Phase.Pending,
  Phase.Validating,
  Phase.AwaitingDataCopyStart,
  Phase.CopyingBlocks,
  Phase.CopyingChangedBlocks,
  Phase.ConvertingDisk,
  Phase.AwaitingCutOverStartTime,
  Phase.AwaitingAdminCutOver,
  Phase.Unknown
])

export const isPathActive = (navPath: string, currentFullPath: string): boolean => {
  if (navPath === currentFullPath) return true

  const [navBase, navQuery] = navPath.split('?')
  const [curBase, curQuery] = currentFullPath.split('?')

  if (navQuery) {
    return navBase === curBase && curQuery === navQuery
  }

  if (navBase && navBase !== '/dashboard') {
    return curBase.startsWith(navBase)
  }
  return false
}
