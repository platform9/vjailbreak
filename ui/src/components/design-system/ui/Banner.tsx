import {
  Alert,
  AlertColor,
  AlertProps,
  Box,
  Button,
  Typography,
  buttonClasses
} from '@mui/material'
import { ReactNode } from 'react'

export type BannerVariant = AlertColor

export interface BannerProps {
  variant?: BannerVariant
  title?: ReactNode
  message: ReactNode
  actionLabel?: string
  onAction?: () => void
  actionProps?: Omit<React.ComponentProps<typeof Button>, 'onClick' | 'children'>
  sx?: AlertProps['sx']
}

export default function Banner({
  variant = 'info',
  title,
  message,
  actionLabel,
  onAction,
  actionProps,
  sx
}: BannerProps) {
  return (
    <Alert
      severity={variant}
      sx={{
        width: '100%',
        '& .MuiAlert-message': { width: '100%' },
        '& .MuiAlert-icon': { alignSelf: 'flex-start', mt: '2px' },
        [`& .${buttonClasses.root}`]: { whiteSpace: 'nowrap' },
        ...sx
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 2,
          width: '100%'
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          {title ? (
            <Typography variant="body2" sx={{ fontWeight: 600, mb: 0.5 }}>
              {title}
            </Typography>
          ) : null}
          <Typography variant="body2">{message}</Typography>
        </Box>

        {actionLabel && onAction ? (
          <Button
            variant="contained"
            color={variant}
            size="small"
            onClick={onAction}
            {...actionProps}
          >
            {actionLabel}
          </Button>
        ) : null}
      </Box>
    </Alert>
  )
}
