import { styled, Typography } from '@mui/material'
import cubeIcon from '../assets/platform9-cube.svg'

const LogoContainer = styled('div')<{ size?: 'small' | 'medium' | 'large' }>(({ size = 'medium' }) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: size === 'large' ? '16px' : size === 'medium' ? '8px' : '6px',
}))

const CubeIcon = styled('img')<{ collapsed?: boolean; size?: 'small' | 'medium' | 'large' }>(({ collapsed, size = 'medium' }) => {
  const getHeight = () => {
    if (collapsed) return '20px'
    if (size === 'large') return '40px'
    if (size === 'medium') return '28px'
    return '24px'
  }

  return {
    height: getHeight(),
    width: 'auto',
    transition: 'all 0.3s ease',
    cursor: collapsed ? 'pointer' : 'default',
    transformOrigin: 'center',

    ...(collapsed && {
      '&:hover': {
        transform: 'scale(1.1) rotate(5deg)',
        transition: 'transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1)',
      },
      '&:active': {
        transform: 'scale(0.95) rotate(-2deg)',
        transition: 'transform 0.1s ease-out',
      }
    }),
  }
})

const BrandText = styled(Typography)<{ size?: 'small' | 'medium' | 'large' }>(({ theme, size = 'medium' }) => {
  const getFontSize = () => {
    if (size === 'large') return '2.5rem'
    if (size === 'medium') return '1.5rem'
    return '1.25rem'
  }

  return {
    fontWeight: 700,
    fontSize: getFontSize(),
    background: theme.palette.primary.main,
    backgroundClip: 'text',
    WebkitBackgroundClip: 'text',
    WebkitTextFillColor: 'transparent',
    transition: 'all 0.3s ease',
  }
})

interface Platform9LogoProps {
  collapsed?: boolean
  size?: 'small' | 'medium' | 'large'
}

export default function Platform9Logo({ collapsed = false, size = 'medium' }: Platform9LogoProps) {
  return (
    <LogoContainer size={size}>
      <CubeIcon
        src={cubeIcon}
        alt="Platform9"
        collapsed={collapsed}
        size={size}
      />
      {!collapsed && (
        <BrandText variant={size === 'large' ? 'h2' : 'h6'} size={size}>
          vJailbreak
        </BrandText>
      )}
    </LogoContainer>
  )
}