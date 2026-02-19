import { Box, List, Popper, Paper, ClickAwayListener, alpha } from '@mui/material'
import { useMemo } from 'react'

import { NavigationItem } from 'src/types/navigation'

import {
  SUBMENU_SPINE_X_PX,
  SUBMENU_ROW_HEIGHT_PX,
  SUBMENU_CONNECTOR_BLUE,
  SUBMENU_CONNECTOR_GREY,
  SUBMENU_CONNECTOR_SVG_LEFT_PX
} from './Sidenav.constants'
import { ChildNavItemWrapper } from './Sidenav.styles'
import { SubmenuConnectorIcon } from './SubmenuConnectorIcon'
import { NavigationItemComponent } from './NavigationItem'

export function SidenavFlyout({
  open,
  anchorEl,
  flyoutItemId,
  items,
  currentActiveItem,
  onClose,
  onItemClick,
  zIndex
}: {
  open: boolean
  anchorEl: HTMLElement | null
  flyoutItemId: string | null
  items: NavigationItem[]
  currentActiveItem: string
  onClose: () => void
  onItemClick: (item: NavigationItem) => void
  zIndex: number
}) {
  const flyoutItem = useMemo(() => {
    if (!flyoutItemId) return null
    return items.find((it) => it.id === flyoutItemId) ?? null
  }, [flyoutItemId, items])

  const visibleFlyoutChildren = useMemo(() => {
    return flyoutItem?.children?.filter((child) => !child.hidden) ?? []
  }, [flyoutItem])

  const activeChildIndex = useMemo(() => {
    return visibleFlyoutChildren.findIndex((c) => currentActiveItem === c.path)
  }, [currentActiveItem, visibleFlyoutChildren])

  return (
    <Popper
      open={open}
      anchorEl={anchorEl}
      placement="right-start"
      modifiers={[{ name: 'offset', options: { offset: [8, 16] } }]}
      sx={{ zIndex }}
    >
      <ClickAwayListener onClickAway={onClose}>
        <Paper
          elevation={6}
          onMouseLeave={onClose}
          onMouseEnter={() => {
            /* keep open */
          }}
          sx={(theme) => ({
            width: 280,
            borderRadius: 1.5,
            border: `1px solid ${alpha(theme.palette.divider, 0.18)}`,
            backgroundColor: theme.palette.background.paper,
            boxShadow: theme.shadows[6],
            py: 1,
            px: 0.5
          })}
        >
          {flyoutItem?.children?.length ? (
            <Box>
              <Box
                sx={{
                  px: 2,
                  py: 1,
                  fontWeight: 700,
                  color: 'text.primary'
                }}
              >
                {flyoutItem.label}
              </Box>

              <Box
                sx={{
                  position: 'relative',
                  px: 0.5,
                  '&::before': {
                    content: '""',
                    position: 'absolute',
                    left: `${SUBMENU_SPINE_X_PX}px`,
                    top: '-8px',
                    height: `${Math.max(visibleFlyoutChildren.length * SUBMENU_ROW_HEIGHT_PX - SUBMENU_ROW_HEIGHT_PX / 2, 0)}px`,
                    width: '1px',
                    backgroundColor: SUBMENU_CONNECTOR_GREY,
                    borderRadius: 1
                  },
                  '&::after': {
                    content: '""',
                    position: 'absolute',
                    left: `${SUBMENU_SPINE_X_PX}px`,
                    top: '-8px',
                    width: '1px',
                    height:
                      activeChildIndex >= 0
                        ? `${activeChildIndex * SUBMENU_ROW_HEIGHT_PX + SUBMENU_ROW_HEIGHT_PX / 2}px`
                        : '0px',
                    backgroundColor: SUBMENU_CONNECTOR_BLUE,
                    borderRadius: 1
                  }
                }}
              >
                <List disablePadding>
                  {visibleFlyoutChildren.map((child, idx) => {
                    const isSelected = currentActiveItem === child.path
                    const isActiveBranch = activeChildIndex >= 0 && idx <= activeChildIndex
                    const lineColor = isActiveBranch
                      ? SUBMENU_CONNECTOR_BLUE
                      : SUBMENU_CONNECTOR_GREY

                    return (
                      <ChildNavItemWrapper key={child.id}>
                        <Box
                          sx={{
                            position: 'absolute',
                            left: `${SUBMENU_CONNECTOR_SVG_LEFT_PX}px`,
                            top: '50%',
                            transform: 'translateY(-65%)',
                            visibility: isSelected ? 'visible' : 'hidden'
                          }}
                        >
                          <SubmenuConnectorIcon color={lineColor} />
                        </Box>
                        <NavigationItemComponent
                          item={child}
                          isActive={isSelected}
                          isCollapsed={false}
                          onClick={onItemClick}
                          depth={1}
                        />
                      </ChildNavItemWrapper>
                    )
                  })}
                </List>
              </Box>
            </Box>
          ) : null}
        </Paper>
      </ClickAwayListener>
    </Popper>
  )
}
