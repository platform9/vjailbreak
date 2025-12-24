import { Box, List, ListItemButton, ListItemText, Typography } from '@mui/material'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import { ReactNode } from 'react'

export type SectionNavStatus = 'complete' | 'attention' | 'incomplete' | 'optional'

export interface SectionNavItem {
  id: string
  title: ReactNode
  description?: ReactNode
  status?: SectionNavStatus
}

export interface SectionNavProps {
  items: SectionNavItem[]
  activeId?: string
  onSelect: (id: string) => void
  dense?: boolean
  showDescriptions?: boolean
  'data-testid'?: string
}

const statusIcon = (status: SectionNavStatus | undefined) => {
  switch (status) {
    case 'complete':
      return <CheckCircleOutlineIcon fontSize="small" color="success" />
    case 'attention':
      return <ErrorOutlineIcon fontSize="small" color="error" />
    case 'optional':
      return <InfoOutlinedIcon fontSize="small" color="action" />
    default:
      return <RadioButtonUncheckedIcon fontSize="small" color="action" />
  }
}

export default function SectionNav({
  items,
  activeId,
  onSelect,
  dense = false,
  showDescriptions = true,
  'data-testid': dataTestId = 'section-nav'
}: SectionNavProps) {
  return (
    <Box
      data-testid={dataTestId}
      sx={{
        position: 'sticky',
        top: 0,
        alignSelf: 'start',
        border: (theme) => `1px solid ${theme.palette.divider}`,
        borderRadius: (theme) => theme.shape.borderRadius * 2,
        backgroundColor: (theme) => theme.palette.background.paper,
        overflow: 'hidden'
      }}
    >
      <Box
        sx={{
          px: dense ? 1.5 : 2,
          py: dense ? 1 : 1.5,
          borderBottom: (theme) => `1px solid ${theme.palette.divider}`
        }}
      >
        <Typography variant="subtitle2" sx={{ lineHeight: 1.2 }}>
          Steps
        </Typography>
        {!dense ? (
          <Typography variant="caption" color="text.secondary">
            Jump to any section
          </Typography>
        ) : null}
      </Box>
      <List dense disablePadding>
        {items.map((item) => {
          const selected = item.id === activeId
          return (
            <ListItemButton
              key={item.id}
              selected={selected}
              onClick={() => onSelect(item.id)}
              sx={{
                alignItems: 'center',
                gap: 1.5,
                py: dense ? 0.75 : 1.25,
                px: dense ? 1.5 : 2,
                '&.Mui-selected': {
                  backgroundColor: (theme) => theme.palette.action.selected
                }
              }}
            >
              <Box
                sx={{
                  width: 20,
                  display: 'grid',
                  placeItems: 'center',
                  flexShrink: 0
                }}
              >
                {statusIcon(item.status)}
              </Box>
              <ListItemText
                primary={
                  <Typography
                    variant={dense ? 'caption' : 'body2'}
                    sx={{ fontWeight: selected ? 600 : 500 }}
                  >
                    {item.title}
                  </Typography>
                }
                secondary={
                  showDescriptions && item.description ? (
                    <Typography variant="caption" color="text.secondary">
                      {item.description}
                    </Typography>
                  ) : null
                }
              />
            </ListItemButton>
          )
        })}
      </List>
    </Box>
  )
}
