import { Box, Paper, Radio, Typography } from '@mui/material'
import type { ReactNode } from 'react'

export interface MethodCardProps {
  selected: boolean
  onClick: () => void
  icon: ReactNode
  title: string
  description: string
  recommended?: boolean
}

export default function MethodCard({
  selected,
  onClick,
  icon,
  title,
  description,
  recommended
}: MethodCardProps) {
  return (
    <Paper
      variant="outlined"
      onClick={onClick}
      sx={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 1.5,
        p: 1.5,
        cursor: 'pointer',
        borderColor: selected ? 'primary.main' : 'divider',
        borderWidth: selected ? 2 : 1,
        bgcolor: selected ? 'primary.50' : 'background.paper',
        transition: 'border-color 0.15s, background-color 0.15s',
        '&:hover': { borderColor: selected ? 'primary.main' : 'action.hover' }
      }}
    >
      <Radio checked={selected} size="small" sx={{ mt: 0.2, p: 0 }} disableRipple />
      <Box sx={{ color: selected ? 'primary.main' : 'text.secondary', mt: 0.25 }}>{icon}</Box>
      <Box sx={{ flex: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography variant="body2" fontWeight={600}>
            {title}
          </Typography>
          {recommended && (
            <Box
              sx={{
                bgcolor: 'success.main',
                color: 'success.contrastText',
                fontSize: '0.65rem',
                fontWeight: 700,
                px: 0.75,
                py: 0.25,
                borderRadius: 0.5,
                letterSpacing: 0.3,
                lineHeight: 1.4
              }}
            >
              RECOMMENDED
            </Box>
          )}
        </Box>
        <Typography variant="caption" color="text.secondary" sx={{ lineHeight: 1.4 }}>
          {description}
        </Typography>
      </Box>
    </Paper>
  )
}
