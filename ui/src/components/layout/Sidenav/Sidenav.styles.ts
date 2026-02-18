import {
  Drawer,
  ListItemButton,
  ListItemIcon,
  IconButton,
  Box,
  Chip,
  styled,
  alpha
} from '@mui/material'

import {
  DRAWER_WIDTH,
  DRAWER_WIDTH_COLLAPSED,
  SUBMENU_ROW_HEIGHT_PX,
  FIRST_LEVEL_ROW_HEIGHT_PX,
  SUBMENU_TEXT_INDENT_PX
} from './Sidenav.constants'

export const ExpandToggleIconRoot = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'expanded'
})<{ expanded: boolean }>(({ theme, expanded }) => ({
  width: 16,
  height: 16,
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  position: 'relative',
  transition: theme.transitions.create(['transform'], {
    duration: theme.transitions.duration.shorter,
    easing: theme.transitions.easing.easeInOut
  }),
  transform: expanded ? 'rotate(180deg)' : 'rotate(0deg)'
}))

export const ExpandToggleBar = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'vertical' && prop !== 'expanded'
})<{ vertical?: boolean; expanded?: boolean }>(({ theme, vertical, expanded }) => ({
  position: 'absolute',
  backgroundColor: expanded ? theme.palette.text.secondary : theme.palette.text.primary,
  borderRadius: 2,
  ...(vertical
    ? {
        width: 2,
        height: 10,
        transition: theme.transitions.create(['transform', 'opacity'], {
          duration: theme.transitions.duration.shorter,
          easing: theme.transitions.easing.easeInOut
        }),
        transform: expanded ? 'scaleY(0)' : 'scaleY(1)',
        opacity: expanded ? 0 : 1
      }
    : {
        height: 2,
        width: 10
      })
}))

export const StyledDrawer = styled(Drawer, {
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

export const DrawerHeader = styled('div')(({ theme }) => ({
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

export const BrandContainer = styled(Box, {
  shouldForwardProp: (prop) => prop !== 'collapsed'
})<{ collapsed?: boolean }>(({ theme, collapsed }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: collapsed ? 'center' : 'flex-start',
  width: '100%',
  paddingLeft: collapsed ? 0 : theme.spacing(1),
  transition: theme.transitions.create(['justify-content', 'padding-left'], {
    easing: theme.transitions.easing.sharp,
    duration: theme.transitions.duration.enteringScreen
  })
}))

export const VersionBadge = styled(Box, {
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

export const StyledListItemButton = styled(ListItemButton, {
  shouldForwardProp: (prop) =>
    prop !== 'active' && prop !== 'collapsed' && prop !== 'depth' && prop !== 'groupActive'
})<{ active?: boolean; collapsed?: boolean; depth?: number; groupActive?: boolean }>(
  ({ theme, active, collapsed, depth = 0, groupActive }) => ({
    minHeight: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
    height: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
    maxHeight: depth > 0 ? SUBMENU_ROW_HEIGHT_PX : FIRST_LEVEL_ROW_HEIGHT_PX,
    margin: theme.spacing(0.5, 1),
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
    color:
      depth > 0
        ? alpha(theme.palette.text.secondary, 0.75)
        : alpha(theme.palette.text.secondary, 0.8),
    '& .MuiTypography-root': {
      fontSize: depth > 0 ? '0.875rem' : '0.95rem',
      fontWeight: depth > 0 ? 400 : 400,
      letterSpacing: '0.01em',
      lineHeight: 1.25
    },
    '& .MuiListItemText-root': {
      margin: 0
    },
    ...(active
      ? depth > 0
        ? {
            backgroundColor: 'transparent',
            color: theme.palette.text.primary,
            '& .MuiTypography-root': {
              fontWeight: 400
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
    '&.Mui-focusVisible': {
      boxShadow: 'none'
    }
  })
)

export const ChildGroupContainer = styled(Box)(({ theme }) => ({
  marginLeft: theme.spacing(2),
  position: 'relative'
}))

export const ChildNavItemWrapper = styled(Box)(() => ({
  position: 'relative',
  paddingLeft: `${SUBMENU_TEXT_INDENT_PX}px`
}))

export const StyledListItemIcon = styled(ListItemIcon, {
  shouldForwardProp: (prop) => prop !== 'collapsed'
})<{ collapsed?: boolean }>(({ collapsed }) => ({
  minWidth: collapsed ? 44 : 36,
  marginRight: collapsed ? 6 : 10,
  justifyContent: 'center',
  '& svg': {
    fontSize: 20
  }
}))

export const NavigationBadge = styled(Chip)(({ theme }) => ({
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

export const CornerToggleButton = styled(IconButton)(({ theme }) => ({
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
