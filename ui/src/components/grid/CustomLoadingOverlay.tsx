import { Box, CircularProgress, styled } from '@mui/material'
import { GridLoadingOverlayProps } from '@mui/x-data-grid'

const StyledGridOverlay = styled('div')(({ theme }) => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  justifyContent: 'center',
  height: '100%',
  backgroundColor:
    theme.palette.mode === 'dark' ? 'rgba(18, 18, 18, 0.9)' : 'rgba(255, 255, 255, 0.9)',
  color: theme.palette.text.primary
}))

export interface CustomLoadingOverlayProps extends GridLoadingOverlayProps {
  loadingMessage?: string
}

const CustomLoadingOverlay = (props: CustomLoadingOverlayProps) => {
  const { loadingMessage = "Loading..." } = props
  return (
    <StyledGridOverlay>
      <CircularProgress />
      <Box sx={{ mt: 2 }}>{loadingMessage}</Box>
    </StyledGridOverlay>
  )
}

export default CustomLoadingOverlay
