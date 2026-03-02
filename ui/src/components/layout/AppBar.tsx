import Box from '@mui/material/Box'
import Paper from '@mui/material/Paper'
import Button from '@mui/material/Button'
import { cleanupAllResources } from 'src/api/helpers'
import ThemeToggle from './ThemeToggle'

export default function ButtonAppBar({ hide = false }) {
  return (
    <Paper
      elevation={0}
      square
      sx={{
        visibility: hide ? 'hidden' : 'visible',
        position: 'sticky',
        top: 0,
        zIndex: (theme) => theme.zIndex.appBar,
        borderBottom: (theme) => `1px solid ${theme.palette.divider}`,
        backgroundColor: 'background.paper'
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'flex-end',
          gap: 1.5,
          height: 64,
          px: { xs: 2, sm: 3 },
          maxWidth: { lg: '1600px' },
          mx: 'auto',
          width: '100%'
        }}
      >
        <ThemeToggle />
        {import.meta.env.MODE === 'development' && (
          <Button
            size="medium"
            onClick={() => cleanupAllResources()}
            color="error"
            variant="outlined"
          >
            DEV ONLY: Clean Up Resources
          </Button>
        )}
      </Box>
    </Paper>
  )
}
