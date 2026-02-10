import {
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  IconButton,
  Box,
  Chip,
  Tooltip,
  Divider,
  Collapse,
  Popper,
  Paper,
  ClickAwayListener,
  styled,
  alpha
} from '@mui/material'
import { ChevronLeft, ChevronRight, OpenInNew, ExpandLess, ExpandMore } from '@mui/icons-material'
import { useState, useEffect, useMemo, useId, type MouseEvent as ReactMouseEvent } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { NavigationItem, SidenavProps } from 'src/types/navigation'
import { useVersionQuery } from 'src/hooks/api/useVersionQuery'
import Platform9Logo from '../Platform9Logo'
import { UpgradeModal } from 'src/features/migration/components'
import UpgradeIcon from '@mui/icons-material/Upgrade'
import Button from '@mui/material/Button'
import Typography from '@mui/material/Typography'
import { useTheme } from '@mui/material/styles'
import { useMigrationsQuery } from 'src/hooks/api/useMigrationsQuery'
import { Phase } from 'src/api/migrations/model'
import { FIVE_SECONDS, THIRTY_SECONDS } from 'src/constants'

const DRAWER_WIDTH = 280
const DRAWER_WIDTH_COLLAPSED = 72

const SUBMENU_CONNECTOR_GREY = '#e6e6ea'
const SUBMENU_CONNECTOR_BLUE = '#0089c7'
const SUBMENU_ROW_HEIGHT_PX = 32
const FIRST_LEVEL_ROW_HEIGHT_PX = 44
const SUBMENU_SPINE_X_PX = 22
const SUBMENU_CONNECTOR_SVG_LEFT_PX = SUBMENU_SPINE_X_PX - 4
const SUBMENU_TEXT_INDENT_PX = 36

function SubmenuConnectorIcon({ color }: { color: string }) {
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

const StyledDrawer = styled(Drawer, {
  shouldForwardProp: (prop) => prop !== 'collapsed'
})<{ collapsed: boolean }>(({ theme, collapsed }) => ({
  width: collapsed ? DRAWER_WIDTH_COLLAPSED : DRAWER_WIDTH,
  flexShrink: 0,
  whiteSpace: 'nowrap',
  boxSizing: 'border-box',
  '& .MuiDrawer-paper': {
    width: collapsed ? DRAWER_WIDTH_COLLAPSED : DRAWER_WIDTH,
    transition: theme.transitions.create('width', {
      easing: theme.transitions.easing.sharp,
      duration: theme.transitions.duration.enteringScreen
    }),
    overflow: 'visible',
    borderRight: `1px solid ${theme.palette.divider}`,
    backgroundColor: theme.palette.background.paper,
    '&::-webkit-scrollbar': {
      display: 'none'
    },
    scrollbarWidth: 'none',
    msOverflowStyle: 'none'
  }
}))

const DrawerHeader = styled('div')(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: theme.spacing(1.5, 2),
  ...theme.mixins.toolbar,
  minHeight: '64px !important',
  borderBottom: `1px solid ${alpha(theme.palette.divider, 0.12)}`,
  marginBottom: theme.spacing(1),
  position: 'relative'
}))

const BrandContainer = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'collapsed'
})<{ collapsed?: boolean }>(({ theme, collapsed }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: collapsed ? 'center' : 'flex-start',
  width: '100%',
  paddingLeft: collapsed ? 0 : theme.spacing(2),
  transition: theme.transitions.create(['justify-content', 'padding-left'], {
    easing: theme.transitions.easing.sharp,
    duration: theme.transitions.duration.enteringScreen
  })
}))

const VersionBadge = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'collapsed'
})<{ collapsed?: boolean }>(({ theme, collapsed }) => ({
  fontSize: '0.8rem',
  color: alpha(theme.palette.text.secondary, 0.6),
  fontWeight: 400,
  width: collapsed ? '60px' : 'auto',
  textAlign: 'center',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  padding: collapsed ? theme.spacing(0.5, 1) : 0
}))

const VersionDisplay = ({ collapsed }: { collapsed?: boolean }) => {
  const { data: versionInfo, isLoading, error } = useVersionQuery()

  if (isLoading) {
    const content = (
      <VersionBadge collapsed={collapsed}>{collapsed ? '...' : 'Loading version...'}</VersionBadge>
    )

    if (collapsed) {
      return (
        <Tooltip title="Loading version..." placement="right" arrow>
          {content}
        </Tooltip>
      )
    }
    return content
  }

  if (error) {
    const content = (
      <VersionBadge collapsed={collapsed}>
        {collapsed ? 'v?' : 'Version: Unable to load'}
      </VersionBadge>
    )

    if (collapsed) {
      return (
        <Tooltip title="Version: Unable to load" placement="right" arrow>
          {content}
        </Tooltip>
      )
    }
    return content
  }

  const content = (
    <VersionBadge collapsed={collapsed}>
      {collapsed ? `${versionInfo?.version || '?'}` : `Version: ${versionInfo?.version}`}
    </VersionBadge>
  )

  if (collapsed) {
    return (
      <Tooltip
        title={
          <Box>
            Version: {versionInfo?.version}
            {versionInfo?.upgradeAvailable && versionInfo?.upgradeVersion && (
              <Box component="span" sx={{ display: 'block', fontSize: '0.85rem', mt: 0.5 }}>
                Update available: {versionInfo.upgradeVersion}
              </Box>
            )}
          </Box>
        }
        placement="right"
        arrow
      >
        {content}
      </Tooltip>
    )
  }

  return content
}

const StyledListItemButton = styled(ListItemButton, {
  shouldForwardProp: (prop) =>
    prop !== 'active' && prop !== 'collapsed' && prop !== 'depth' && prop !== 'groupActive'
})<{ active?: boolean; collapsed?: boolean; depth?: number; groupActive?: boolean }>(
  ({ theme, active, collapsed, depth = 0, groupActive }) => ({
    minHeight: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
    height: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
    maxHeight: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
    margin: collapsed ? theme.spacing(0.5, 1) : theme.spacing(0.5, 1),
    borderRadius: theme.spacing(1),
    paddingLeft: collapsed ? 'auto' : theme.spacing(2),
    paddingRight: collapsed ? 'auto' : theme.spacing(2),
    justifyContent: collapsed ? 'center' : 'initial',
    position: 'relative',
    overflow: 'hidden',
    cursor: 'pointer',
    transition: theme.transitions.create(['background-color', 'box-shadow'], {
      duration: theme.transitions.duration.shorter
    }),
    ...(active
      ? depth > 0
        ? {
            backgroundColor: 'transparent',
            color: theme.palette.text.primary,
            '& .MuiTypography-root': {
              fontWeight: 600
            },
            '&::before': {
              content: '""',
              position: 'absolute',
              right: 0,
              top: 6,
              bottom: 6,
              width: 3,
              borderRadius: 2,
              backgroundColor: theme.palette.primary.main
            },
            '& .MuiListItemIcon-root': {
              color: theme.palette.text.primary
            }
          }
        : {
            backgroundColor: 'transparent',
            color: theme.palette.text.primary,
            '& .MuiTypography-root': {
              fontWeight: 600
            },
            '&::before': {
              content: '""',
              position: 'absolute',
              right: 0,
              top: 5,
              bottom: 5,
              width: 3,
              backgroundColor: theme.palette.primary.main
            },
            '& .MuiListItemIcon-root': {
              color: theme.palette.common.white,
              backgroundColor: theme.palette.primary.main,
              borderRadius: theme.spacing(1),
              width: 32,
              height: 32,
              minWidth: 32,
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center'
            }
          }
      : null),
    ...(!active && groupActive && depth === 0
      ? {
          color: theme.palette.text.primary,
          '& .MuiListItemIcon-root': {
            color: theme.palette.common.white,
            backgroundColor: theme.palette.primary.main,
            borderRadius: theme.spacing(1),
            width: 32,
            height: 32,
            minWidth: 32,
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center'
          },
          '& .MuiTypography-root': {
            fontWeight: 600
          }
        }
      : null),
    '&:hover':
      depth > 0
        ? {
            backgroundColor: 'transparent',
            '& .MuiTypography-root': {
              color: theme.palette.text.primary
            }
          }
        : {
            backgroundColor: alpha(theme.palette.primary.main, 0.05)
          },
    '&.Mui-focusVisible': {
      boxShadow: `0 0 0 2px ${alpha(theme.palette.primary.main, 0.25)}`
    }
  })
)

const ChildGroupContainer = styled(Box)(({ theme }) => ({
  marginLeft: theme.spacing(2),
  position: 'relative'
}))

const ChildNavItemWrapper = styled(Box)(() => ({
  position: 'relative',
  paddingLeft: `${SUBMENU_TEXT_INDENT_PX}px`
}))

const StyledListItemIcon = styled(ListItemIcon, {
  shouldForwardProp: (prop) => prop !== 'collapsed'
})<{ collapsed?: boolean }>(({ collapsed }) => ({
  minWidth: collapsed ? 44 : 36,
  marginRight: collapsed ? 6 : 12,
  justifyContent: 'center',
  '& svg': {
    fontSize: 20
  }
}))

const NavigationBadge = styled(Chip)(({ theme }) => ({
  fontSize: '0.6rem',
  height: '16px',
  fontWeight: 600,
  marginLeft: theme.spacing(1),
  transform: 'translateY(-6px)',
  px: 0.75,
  lineHeight: '16px',
  display: 'flex',
  alignItems: 'center'
}))

const CornerToggleButton = styled(IconButton)(({ theme }) => ({
  position: 'absolute',
  right: -18,
  bottom: -18,
  width: 32,
  height: 32,
  borderRadius: '50%',
  backgroundColor: theme.palette.background.paper,
  border: `1px solid ${theme.palette.divider}`,
  color: theme.palette.text.secondary,
  boxShadow: theme.shadows[1],
  transition: theme.transitions.create(
    ['background-color', 'border-color', 'box-shadow', 'color'],
    {
      duration: theme.transitions.duration.shorter
    }
  ),
  '&:hover': {
    backgroundColor: alpha(theme.palette.primary.main, theme.palette.mode === 'dark' ? 0.18 : 0.12),
    borderColor: alpha(theme.palette.primary.main, theme.palette.mode === 'dark' ? 0.55 : 0.4),
    color: theme.palette.primary.main,
    boxShadow: theme.shadows[3]
  },
  '&:active': {
    backgroundColor: alpha(theme.palette.primary.main, theme.palette.mode === 'dark' ? 0.24 : 0.16),
    borderColor: alpha(theme.palette.primary.main, theme.palette.mode === 'dark' ? 0.65 : 0.5),
    boxShadow: theme.shadows[2]
  },
  '&.Mui-focusVisible': {
    boxShadow: `0 0 0 2px ${alpha(theme.palette.primary.main, 0.25)}`
  }
}))

interface NavigationItemProps {
  item: NavigationItem
  isActive: boolean
  isGroupActive?: boolean
  isCollapsed: boolean
  onClick: (item: NavigationItem) => void
  onOpenFlyout?: (item: NavigationItem, anchorEl: HTMLElement) => void
  isExpanded?: boolean
  onToggleExpand?: (itemId: string) => void
  depth?: number
}

function NavigationItemComponent({
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
              '& .MuiTypography-root': {
                fontSize: depth > 0 ? '0.875rem' : '0.95rem',
                fontWeight:
                  depth === 0 ? (isActive || isGroupActive ? 600 : 500) : isActive ? 600 : 500,
                color: depth > 0 ? (isActive ? 'text.primary' : 'text.secondary') : undefined
              },
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
                    aria-label={
                      isExpanded ? 'collapse navigation group' : 'expand navigation group'
                    }
                    sx={{ color: 'inherit' }}
                  >
                    {isExpanded ? <ExpandLess fontSize="small" /> : <ExpandMore fontSize="small" />}
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
}

export default function Sidenav({
  items,
  isCollapsed: controlledCollapsed,
  onToggleCollapse,
  onItemClick,
  activeItem: controlledActiveItem
}: SidenavProps) {
  const navigate = useNavigate()
  const location = useLocation()
  const theme = useTheme()

  const [flyoutAnchorEl, setFlyoutAnchorEl] = useState<HTMLElement | null>(null)
  const [flyoutItemId, setFlyoutItemId] = useState<string | null>(null)

  const [internalCollapsed, setInternalCollapsed] = useState(() => {
    const saved = localStorage.getItem('sidenav-collapsed')
    return saved ? JSON.parse(saved) : false
  })

  const isCollapsed = controlledCollapsed ?? internalCollapsed

  const activeItem = controlledActiveItem ?? `${location.pathname}${location.search}`

  const [expandedItem, setExpandedItem] = useState<string | null>(() => {
    const saved = localStorage.getItem('sidenav-expanded')
    return saved ? JSON.parse(saved) : null
  })

  const { data: versionInfo } = useVersionQuery()
  const [isUpgradeModalOpen, setIsUpgradeModalOpen] = useState(false)
  const { data: migrations } = useMigrationsQuery(undefined, {
    refetchInterval: (query) => {
      // if we're not on the migrations page, don't refetch
      if (activeItem !== '/dashboard/migrations') return Infinity
      const migrations = query?.state?.data || []
      const hasPendingMigration = !!migrations.find((m) => m.status === undefined)
      return hasPendingMigration ? FIVE_SECONDS : THIRTY_SECONDS
    },
    refetchOnMount: true
  })

  // checking if there's any active migrations
  const activePhases = new Set<Phase>([
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

  const hasActiveMigrations = Array.isArray(migrations)
    ? migrations.some((m) => activePhases.has(m.status?.phase as Phase))
    : false

  useEffect(() => {
    if (controlledCollapsed === undefined) {
      localStorage.setItem('sidenav-collapsed', JSON.stringify(internalCollapsed))
    }
  }, [internalCollapsed, controlledCollapsed])

  useEffect(() => {
    localStorage.setItem('sidenav-expanded', JSON.stringify(expandedItem))
  }, [expandedItem])

  const handleToggleCollapse = () => {
    if (onToggleCollapse) {
      onToggleCollapse()
    } else {
      setInternalCollapsed((prev: boolean) => !prev)
    }
  }

  useEffect(() => {
    if (!isCollapsed) {
      setFlyoutAnchorEl(null)
      setFlyoutItemId(null)
    }
  }, [isCollapsed])

  const closeFlyout = () => {
    setFlyoutAnchorEl(null)
    setFlyoutItemId(null)
  }

  const handleOpenFlyout = (item: NavigationItem, anchorEl: HTMLElement) => {
    if (!isCollapsed) return
    if (!item.children?.length) return
    if (item.external) return
    setFlyoutItemId(item.id)
    setFlyoutAnchorEl(anchorEl)
  }

  const handleItemClick = (item: NavigationItem) => {
    if ((window as any).__VDDK_UPLOAD_IN_PROGRESS__) {
      const confirmed = window.confirm(
        'File upload is in progress. If you leave this page, the upload may be interrupted. Do you want to continue?'
      )
      if (!confirmed) return
    }

    if (item.children?.length && !isCollapsed && !item.external) {
      setExpandedItem((prev) => (prev === item.id ? null : item.id))
      return
    }

    if (isCollapsed && flyoutItemId) {
      window.setTimeout(() => {
        closeFlyout()
      }, 150)
    } else {
      closeFlyout()
    }

    if (onItemClick) {
      onItemClick(item)
    } else if (item.external) {
      const url =
        item.externalUrl && item.externalUrl.trim()
          ? item.externalUrl
          : `https://${window.location.host}${item.path}`
      window.open(url, '_blank', 'noopener,noreferrer')
    } else {
      navigate(item.path)
    }
  }

  const flattenItems = useMemo(() => {
    const out: NavigationItem[] = []
    const walk = (list: NavigationItem[]) => {
      for (const it of list) {
        out.push(it)
        if (it.children?.length) walk(it.children)
      }
    }
    walk(items)
    return out
  }, [items])

  const isPathActive = (navPath: string, currentFullPath: string): boolean => {
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

  const currentActiveItem = useMemo(() => {
    const exact = flattenItems.find((it) => it.path === activeItem)
    if (exact) return exact.path
    const match = flattenItems.find((it) => isPathActive(it.path, activeItem))
    return match?.path || activeItem
  }, [activeItem, flattenItems])

  useEffect(() => {
    // Intentionally do not auto-expand on route change so the user can collapse all groups.
  }, [activeItem, items])

  const handleToggleExpand = (itemId: string) => {
    setExpandedItem((prev) => (prev === itemId ? null : itemId))
  }

  const drawerContent = (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      <DrawerHeader>
        <BrandContainer collapsed={isCollapsed}>
          <Platform9Logo collapsed={isCollapsed} />
        </BrandContainer>
        <Tooltip
          title={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          placement="right"
          arrow
        >
          <CornerToggleButton onClick={handleToggleCollapse} aria-label="toggle sidebar">
            {isCollapsed ? <ChevronRight fontSize="small" /> : <ChevronLeft fontSize="small" />}
          </CornerToggleButton>
        </Tooltip>
      </DrawerHeader>

      <List
        sx={{
          pt: 1,
          flex: 1,
          overflowY: 'auto',
          overscrollBehavior: 'contain',
          '&::-webkit-scrollbar': {
            display: 'none'
          },
          scrollbarWidth: 'none',
          msOverflowStyle: 'none'
        }}
      >
        {items
          .filter((item) => !item.hidden)
          .map((item) => {
            const isItemActive = currentActiveItem === item.path
            const isChildActive = !!item.children?.some((c) => currentActiveItem === c.path)
            const isItemVisuallyActive = isCollapsed ? isItemActive || isChildActive : isItemActive
            const isExpanded = expandedItem === item.id
            const activeChildIndex =
              item.children?.findIndex((c) => currentActiveItem === c.path) ?? -1

            return (
              <Box key={item.id}>
                {item.id === 'monitoring' && <Divider sx={{ my: 1, mx: 2 }} />}
                <NavigationItemComponent
                  item={item}
                  isActive={isItemVisuallyActive}
                  isGroupActive={isChildActive}
                  isCollapsed={isCollapsed}
                  onClick={handleItemClick}
                  onOpenFlyout={handleOpenFlyout}
                  isExpanded={isExpanded}
                  onToggleExpand={handleToggleExpand}
                  depth={0}
                />

                {!isCollapsed && item.children?.length ? (
                  <Collapse in={isExpanded} timeout="auto" unmountOnExit>
                    <ChildGroupContainer
                      sx={{
                        pl: 0.5,
                        '&::before': {
                          content: '""',
                          position: 'absolute',
                          left: `${SUBMENU_SPINE_X_PX}px`,
                          top: '-8px',
                          bottom: 0,
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
                        {item.children
                          .filter((child) => !child.hidden)
                          .map((child, idx) => {
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
                                  isCollapsed={isCollapsed}
                                  onClick={handleItemClick}
                                  depth={1}
                                />
                              </ChildNavItemWrapper>
                            )
                          })}
                      </List>
                    </ChildGroupContainer>
                  </Collapse>
                ) : null}
              </Box>
            )
          })}
      </List>

      <Popper
        open={Boolean(isCollapsed && flyoutAnchorEl && flyoutItemId)}
        anchorEl={flyoutAnchorEl}
        placement="right-start"
        modifiers={[{ name: 'offset', options: { offset: [8, 16] } }]}
        sx={{ zIndex: theme.zIndex.drawer + 2 }}
      >
        <ClickAwayListener onClickAway={closeFlyout}>
          <Paper
            elevation={6}
            onMouseLeave={closeFlyout}
            onMouseEnter={() => {
              /* keep open */
            }}
            sx={{
              width: 280,
              borderRadius: 1.5,
              border: `1px solid ${alpha(theme.palette.divider, 0.18)}`,
              backgroundColor: theme.palette.background.paper,
              boxShadow: theme.shadows[6],
              py: 1,
              px: 0.5
            }}
          >
            {(() => {
              const flyoutItem = items.find((it) => it.id === flyoutItemId)
              if (!flyoutItem?.children?.length) return null

              const activeChildIndex =
                flyoutItem.children?.findIndex((c) => currentActiveItem === c.path) ?? -1

              return (
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
                        bottom: 0,
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
                      {flyoutItem.children
                        .filter((child) => !child.hidden)
                        .map((child, idx) => {
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
                                onClick={handleItemClick}
                                depth={1}
                              />
                            </ChildNavItemWrapper>
                          )
                        })}
                    </List>
                  </Box>
                </Box>
              )
            })()}
          </Paper>
        </ClickAwayListener>
      </Popper>

      <Box sx={{ mt: 'auto', px: 2, pb: 1.5, pt: 1.5, position: 'relative' }}>
        <Divider sx={{ mb: 1.5, opacity: 0.6 }} />
        <VersionDisplay collapsed={isCollapsed} />
        {versionInfo?.upgradeAvailable &&
          versionInfo?.upgradeVersion &&
          (isCollapsed ? (
            <Tooltip
              title={
                <Typography variant="body2">
                  {hasActiveMigrations
                    ? "Migrations are in progress, can't upgrade"
                    : 'Upgrade Available'}
                </Typography>
              }
              placement="right"
              arrow
              slotProps={{ tooltip: { sx: { ...theme.typography.body2 } } }}
            >
              <span>
                <Button
                  color="primary"
                  variant="contained"
                  sx={{ mt: 2, minWidth: 0, width: 40, height: 40, borderRadius: '50%' }}
                  onClick={() => setIsUpgradeModalOpen(true)}
                  disabled={hasActiveMigrations}
                >
                  <UpgradeIcon />
                </Button>
              </span>
            </Tooltip>
          ) : (
            <Tooltip
              title={
                hasActiveMigrations ? (
                  <Typography variant="body2">Migrations are in progress, can't upgrade</Typography>
                ) : (
                  ''
                )
              }
              placement="top"
              arrow
              disableHoverListener={!hasActiveMigrations}
              slotProps={{ tooltip: { sx: { ...theme.typography.body2 } } }}
            >
              <span>
                <Button
                  color="primary"
                  variant="contained"
                  fullWidth
                  sx={{ mt: 1, fontWeight: 600 }}
                  startIcon={<UpgradeIcon />}
                  onClick={() => setIsUpgradeModalOpen(true)}
                  disabled={hasActiveMigrations}
                >
                  Upgrade Available
                </Button>
              </span>
            </Tooltip>
          ))}
        <UpgradeModal show={isUpgradeModalOpen} onClose={() => setIsUpgradeModalOpen(false)} />
      </Box>
    </Box>
  )

  return (
    <StyledDrawer variant="permanent" collapsed={isCollapsed}>
      {drawerContent}
    </StyledDrawer>
  )
}
