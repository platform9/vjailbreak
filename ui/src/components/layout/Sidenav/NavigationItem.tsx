import {
  ListItem,
  ListItemText,
  IconButton,
  Box
} from '@mui/material'
import { OpenInNew } from '@mui/icons-material'
import { memo, type MouseEvent as ReactMouseEvent } from 'react'

import { NavigationItem as NavigationItemType } from 'src/types/navigation'

import {
  StyledListItemButton,
  StyledListItemIcon,
  NavigationBadge
} from './Sidenav.styles'
import { ExpandToggleIcon } from './ExpandToggleIcon'
import {
  SUBMENU_ROW_HEIGHT_PX,
  FIRST_LEVEL_ROW_HEIGHT_PX
} from './Sidenav.constants'

export interface NavigationItemProps {
  item: NavigationItemType
  isActive: boolean
  isGroupActive?: boolean
  isCollapsed: boolean
  onClick: (item: NavigationItemType) => void
  onOpenFlyout?: (item: NavigationItemType, anchorEl: HTMLElement) => void
  isExpanded?: boolean
  onToggleExpand?: (itemId: string) => void
  depth?: number
}

export const NavigationItemComponent = memo(function NavigationItemComponent({
  item,
  isActive,
  isGroupActive,
  isCollapsed,
  onClick,
  onOpenFlyout,
  isExpanded,
  onToggleExpand,
  depth = 0
}: NavigationItemProps) {
  const handleClick = (e: any) => {
    if (item.disabled) return

    if (isCollapsed && item.children?.length && !item.external) {
      const anchorEl = e?.currentTarget as HTMLElement | undefined
      if (anchorEl) {
        onOpenFlyout?.(item, anchorEl)
        return
      }
    }

    onClick(item)
  }

  const handleToggleExpand = (e: ReactMouseEvent) => {
    e.stopPropagation()
    onToggleExpand?.(item.id)
  }

  const listItemContent = (
    <StyledListItemButton
      active={isActive}
      collapsed={isCollapsed}
      depth={depth}
      groupActive={isGroupActive}
      onClick={handleClick}
      disabled={item.disabled}
      disableRipple
      disableTouchRipple
      data-tour={`nav-${item.id}`}
      sx={
        !isCollapsed
          ? {
              pl: depth > 0 ? 0 : 2,
              minHeight: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
              height: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
              maxHeight: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
              ...(depth > 0 ? { margin: '0 8px' } : null),
              marginRight: depth > 0 ? 0 : undefined,
              borderRadius: depth > 0 ? 1 : undefined,
              ...(depth === 0 && isGroupActive && !isActive ? { color: 'text.primary' } : null)
            }
          : undefined
      }
    >
      {depth === 0 && item.icon && (
        <StyledListItemIcon collapsed={isCollapsed}>{item.icon}</StyledListItemIcon>
      )}
      {!isCollapsed && (
        <ListItemText
          primary={
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                {item.label}
                {item.badge && (
                  <NavigationBadge
                    label={item.badge.label}
                    size="small"
                    color={item.badge.color}
                    variant={item.badge.variant}
                  />
                )}
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
                {item.children?.length ? (
                  <IconButton
                    size="small"
                    onClick={handleToggleExpand}
                    disabled={item.disabled}
                    aria-label={isExpanded ? 'collapse navigation group' : 'expand navigation group'}
                    disableRipple
                    disableTouchRipple
                    sx={{
                      color: 'inherit',
                      p: 0.5,
                      '&:hover': { backgroundColor: 'transparent' },
                      '&:active': { backgroundColor: 'transparent' }
                    }}
                  >
                    <ExpandToggleIcon expanded={Boolean(isExpanded)} />
                  </IconButton>
                ) : null}
                {item.external && <OpenInNew sx={{ fontSize: '0.875rem', opacity: 0.7 }} />}
              </Box>
            </Box>
          }
          sx={{
            opacity: isCollapsed ? 0 : 1,
            '& .MuiTypography-root': {
              fontSize: '0.875rem'
            }
          }}
        />
      )}
    </StyledListItemButton>
  )

  if (isCollapsed && (item.label || item.badge)) {
    return (
      <ListItem disablePadding sx={{ display: 'block' }}>
        {listItemContent}
      </ListItem>
    )
  }

  return (
    <ListItem disablePadding sx={{ display: 'block' }}>
      {listItemContent}
    </ListItem>
  )
})
