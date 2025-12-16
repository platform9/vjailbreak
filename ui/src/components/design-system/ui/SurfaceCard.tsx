import { Box, Divider, Paper, PaperProps, Typography } from '@mui/material'
import { ReactNode } from 'react'

export interface SurfaceCardProps extends Omit<PaperProps, 'title'> {
  title?: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
  footer?: ReactNode
  'data-testid'?: string
}

export default function SurfaceCard({
  title,
  subtitle,
  actions,
  footer,
  children,
  sx,
  'data-testid': dataTestId = 'surface-card',
  ...paperProps
}: SurfaceCardProps) {
  const hasHeader = !!(title || subtitle || actions)

  return (
    <Paper
      elevation={0}
      {...paperProps}
      sx={{
        border: (theme) => `1px solid ${theme.palette.divider}`,
        borderRadius: (theme) => theme.shape.borderRadius * 2,
        backgroundColor: (theme) => theme.palette.background.paper,
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        p: 3,
        ...sx
      }}
      data-testid={dataTestId}
    >
      {hasHeader ? (
        <Box data-testid={`${dataTestId}-header`} sx={{ display: 'flex', alignItems: 'center' }}>
          <Box sx={{ flex: 1 }}>
            {title ? (
              <Typography variant="h6" component="h2">
                {title}
              </Typography>
            ) : null}
            {subtitle ? (
              <Typography variant="body2" color="text.secondary">
                {subtitle}
              </Typography>
            ) : null}
          </Box>
          {actions ? <Box sx={{ ml: 2 }}>{actions}</Box> : null}
        </Box>
      ) : null}

      {hasHeader && children ? <Divider /> : null}

      {children ? (
        <Box
          data-testid={`${dataTestId}-body`}
          sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}
        >
          {children}
        </Box>
      ) : null}

      {footer ? (
        <Box data-testid={`${dataTestId}-footer`} sx={{ mt: hasHeader || children ? 1 : 0 }}>
          <Divider sx={{ mb: 2 }} />
          {footer}
        </Box>
      ) : null}
    </Paper>
  )
}
