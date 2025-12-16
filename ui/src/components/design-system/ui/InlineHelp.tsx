import { Box, Typography, BoxProps, Theme } from '@mui/material'
import { forwardRef } from 'react'

export type InlineHelpTone = 'default' | 'positive' | 'critical' | 'warning'

export interface InlineHelpProps extends BoxProps {
  children: React.ReactNode
  tone?: InlineHelpTone
}

const getToneColors = (tone: InlineHelpTone, theme: Theme) => {
  switch (tone) {
    case 'positive':
      return {
        color: theme.palette.common.white,
        backgroundColor: theme.palette.success.light,
        borderColor: theme.palette.success.main
      }
    case 'critical':
      return {
        color: theme.palette.error.main,
        backgroundColor: theme.palette.error.light,
        borderColor: theme.palette.error.main
      }
    case 'warning':
      return {
        color: theme.palette.common.white,
        backgroundColor: theme.palette.warning.light,
        borderColor: theme.palette.warning.main
      }
    default:
      return {
        color: theme.palette.text.secondary,
        backgroundColor: theme.palette.grey[100],
        borderColor: theme.palette.grey[300]
      }
  }
}

const InlineHelp = forwardRef<HTMLDivElement, InlineHelpProps>(function InlineHelp(
  { children, tone = 'default', ...rest },
  ref
) {
  return (
    <Box
      ref={ref}
      sx={(theme) => ({
        p: 1.5,
        borderRadius: 1,
        border: 1,
        fontSize: '0.875rem',
        ...getToneColors(tone, theme)
      })}
      {...rest}
    >
      <Typography variant="body2">{children}</Typography>
    </Box>
  )
})

export default InlineHelp
