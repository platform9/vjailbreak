import { grey } from '@mui/material/colors'
import { createTheme } from '@mui/material/styles'
import customTypography from './typography'

export const PRIMARY_MAIN = '#0089c7'

const theme = createTheme({
  spacing: 8,
  palette: {
    primary: {
      main: PRIMARY_MAIN
    },
    secondary: {
      main: '#444f5f'
    },
    background: {
      default: grey[50]
    }
  },
  typography: {
    ...customTypography
  },
  components: {
    MuiTableCell: {
      styleOverrides: {
        head: {
          backgroundColor: grey[200]
        }
      }
    }
  }
})

export default theme
