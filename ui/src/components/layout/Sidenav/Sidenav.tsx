import { List, Box, Tooltip, Divider, Collapse } from '@mui/material'
import { ChevronLeft, ChevronRight } from '@mui/icons-material'
import { useState, useEffect, useMemo, useCallback } from 'react'
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
import {
  ACTIVE_MIGRATION_PHASES,
  SUBMENU_CONNECTOR_BLUE,
  SUBMENU_CONNECTOR_GREY,
  SUBMENU_ROW_HEIGHT_PX,
  SUBMENU_SPINE_X_PX,
  SUBMENU_CONNECTOR_SVG_LEFT_PX,
  isPathActive
} from './Sidenav.constants'
import {
  StyledDrawer,
  DrawerHeader,
  BrandContainer,
  ChildGroupContainer,
  ChildNavItemWrapper,
  CornerToggleButton
} from './Sidenav.styles'
import { VersionDisplay } from './VersionDisplay'
import { NavigationItemComponent } from './NavigationItem'
import { SubmenuConnectorIcon } from './SubmenuConnectorIcon'
import { SidenavFlyout } from './SidenavFlyout'

declare global {
  interface Window {
    __VDDK_UPLOAD_IN_PROGRESS__?: boolean
  }
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

  const getFirstChildItem = useCallback((item: NavigationItem) => {
    const visibleChildren = item.children?.filter((child) => !child.hidden && !child.disabled) ?? []
    return visibleChildren.length ? visibleChildren[0] : undefined
  }, [])

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

  const versionQuery = useVersionQuery()
  const versionInfo = versionQuery.data
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

  const hasActiveMigrations = useMemo(() => {
    return Array.isArray(migrations)
      ? migrations.some((m) => ACTIVE_MIGRATION_PHASES.has(m.status?.phase as Phase))
      : false
  }, [migrations])

  useEffect(() => {
    if (controlledCollapsed === undefined) {
      localStorage.setItem('sidenav-collapsed', JSON.stringify(internalCollapsed))
    }
  }, [internalCollapsed, controlledCollapsed])

  useEffect(() => {
    localStorage.setItem('sidenav-expanded', JSON.stringify(expandedItem))
  }, [expandedItem])

  const handleToggleCollapse = useCallback(() => {
    if (onToggleCollapse) {
      onToggleCollapse()
    } else {
      setInternalCollapsed((prev: boolean) => !prev)
    }
  }, [onToggleCollapse])

  useEffect(() => {
    if (!isCollapsed) {
      setFlyoutAnchorEl(null)
      setFlyoutItemId(null)
    }
  }, [isCollapsed])

  const closeFlyout = useCallback(() => {
    setFlyoutAnchorEl(null)
    setFlyoutItemId(null)
  }, [])

  const handleOpenFlyout = useCallback(
    (item: NavigationItem, anchorEl: HTMLElement) => {
      if (!isCollapsed) return
      if (!item.children?.length) return
      if (item.external) return

      const firstChild = getFirstChildItem(item)
      if (firstChild && !firstChild.external) {
        navigate(firstChild.path)
      }
      setFlyoutItemId(item.id)
      setFlyoutAnchorEl(anchorEl)
    },
    [getFirstChildItem, isCollapsed, navigate]
  )

  const handleItemClick = useCallback(
    (item: NavigationItem) => {
      const itemToNavigate =
        item.children?.length && !item.external ? (getFirstChildItem(item) ?? item) : item

      if (item.children?.length && !isCollapsed && !item.external) {
        if (expandedItem === item.id) {
          setExpandedItem(null)
          return
        }

        setExpandedItem(item.id)
      }

      if (window.__VDDK_UPLOAD_IN_PROGRESS__) {
        const confirmed = window.confirm(
          'File upload is in progress. If you leave this page, the upload may be interrupted. Do you want to continue?'
        )
        if (!confirmed) return
      }

      if (isCollapsed && flyoutItemId) {
        window.setTimeout(() => {
          closeFlyout()
        }, 150)
      } else {
        closeFlyout()
      }

      if (onItemClick) {
        onItemClick(itemToNavigate)
      } else if (itemToNavigate.external) {
        const url =
          itemToNavigate.externalUrl && itemToNavigate.externalUrl.trim()
            ? itemToNavigate.externalUrl
            : `https://${window.location.host}${itemToNavigate.path}`
        window.open(url, '_blank', 'noopener,noreferrer')
      } else {
        navigate(itemToNavigate.path)
      }
    },
    [closeFlyout, expandedItem, flyoutItemId, getFirstChildItem, isCollapsed, navigate, onItemClick]
  )

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

  const currentActiveItem = useMemo(() => {
    const exact = flattenItems.find((it) => it.path === activeItem)
    if (exact) return exact.path
    const match = flattenItems.find((it) => isPathActive(it.path, activeItem))
    return match?.path || activeItem
  }, [activeItem, flattenItems])

  const handleToggleExpand = useCallback(
    (item: NavigationItem) => {
      const willExpand = expandedItem !== item.id
      setExpandedItem(willExpand ? item.id : null)

      if (!willExpand) return

      const firstChild = getFirstChildItem(item)
      if (firstChild && !firstChild.external) {
        navigate(firstChild.path)
      }
    },
    [expandedItem, getFirstChildItem, navigate]
  )

  const visibleItems = useMemo(() => items.filter((item) => !item.hidden), [items])

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
        {visibleItems.map((item) => {
          const isItemActive = currentActiveItem === item.path
          const isChildActive = !!item.children?.some((c) => currentActiveItem === c.path)
          const isItemVisuallyActive = isCollapsed ? isItemActive || isChildActive : isItemActive
          const isExpanded = expandedItem === item.id
          const visibleChildren = item.children?.filter((child) => !child.hidden) ?? []
          const activeChildIndex = visibleChildren.findIndex((c) => currentActiveItem === c.path)

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

              {!isCollapsed && visibleChildren.length ? (
                <Collapse in={isExpanded} timeout="auto" unmountOnExit>
                  <ChildGroupContainer
                    sx={{
                      pl: 0.5,
                      '&::before': {
                        content: '""',
                        position: 'absolute',
                        left: `${SUBMENU_SPINE_X_PX}px`,
                        top: '-8px',
                        height: `${Math.max(visibleChildren.length * SUBMENU_ROW_HEIGHT_PX - SUBMENU_ROW_HEIGHT_PX / 2, 0)}px`,
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
                      {visibleChildren.map((child) => {
                        const isSelected = currentActiveItem === child.path
                        const lineColor = isSelected
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
                                visibility: 'visible'
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

      <SidenavFlyout
        open={Boolean(isCollapsed && flyoutAnchorEl && flyoutItemId)}
        anchorEl={flyoutAnchorEl}
        flyoutItemId={flyoutItemId}
        items={items}
        currentActiveItem={currentActiveItem}
        onClose={closeFlyout}
        onItemClick={handleItemClick}
        zIndex={theme.zIndex.drawer + 2}
      />

      <Box sx={{ mt: 'auto', px: 2, pb: 1.5, pt: 1.5, position: 'relative' }}>
        <Divider sx={{ mb: 1.5, opacity: 0.6 }} />
        <VersionDisplay
          collapsed={isCollapsed}
          versionInfo={versionQuery.data}
          isLoading={versionQuery.isLoading}
          error={versionQuery.error}
        />
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
