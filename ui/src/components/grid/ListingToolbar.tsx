import { Box, Typography } from '@mui/material'
import { GridToolbarContainer } from '@mui/x-data-grid'
import type { ReactNode } from 'react'

interface ListingToolbarProps {
  title: string
  icon?: ReactNode
  subtitle?: ReactNode
  search?: ReactNode
  actions?: ReactNode
}

export default function ListingToolbar({
  title,
  icon,
  subtitle,
  search,
  actions
}: ListingToolbarProps) {
  return (
    <GridToolbarContainer
      sx={{
        p: 2,
        display: 'flex',
        flexWrap: { xs: 'wrap', md: 'nowrap' },
        gap: 2,
        alignItems: { xs: 'flex-start', md: 'center' },
        justifyContent: 'space-between'
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25 }}>
        {icon}
        <Box>
          <Typography variant="h6" component="h2">
            {title}
          </Typography>
          {subtitle ? (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          ) : null}
        </Box>
      </Box>
      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: 2,
          justifyContent: { xs: 'flex-start', md: 'flex-end' },
          width: { xs: '100%', md: 'auto' }
        }}
      >
        {search ? <Box sx={{ display: 'flex', alignItems: 'center' }}>{search}</Box> : null}
        {actions ? (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>{actions}</Box>
        ) : null}
      </Box>
    </GridToolbarContainer>
  )
}
