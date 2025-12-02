import { Box, Typography, BoxProps } from '@mui/material'
import { forwardRef } from 'react'

export interface SectionHeaderProps extends BoxProps {
  title?: string
  subtitle?: string
}

const SectionHeader = forwardRef<HTMLDivElement, SectionHeaderProps>(function SectionHeader(
  { title, subtitle, ...rest },
  ref
) {
  return (
    <Box ref={ref} sx={{ mb: 2 }} {...rest}>
      {title && (
        <Typography variant="h6" component="h2" sx={{ mb: subtitle ? 1 : 0 }}>
          {title}
        </Typography>
      )}
      {subtitle && (
        <Typography variant="body2" color="text.secondary">
          {subtitle}
        </Typography>
      )}
    </Box>
  )
})

export default SectionHeader
