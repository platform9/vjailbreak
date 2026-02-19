import { Box, Tooltip } from '@mui/material'
import { memo } from 'react'

import { useVersionQuery } from 'src/hooks/api/useVersionQuery'

import { VersionBadge } from './Sidenav.styles'

export const VersionDisplay = memo(function VersionDisplay({
  collapsed,
  versionInfo,
  isLoading,
  error
}: {
  collapsed?: boolean
  versionInfo: ReturnType<typeof useVersionQuery>['data']
  isLoading: boolean
  error: ReturnType<typeof useVersionQuery>['error']
}) {
  if (isLoading) {
    const content = (
      <VersionBadge collapsed={collapsed}>{collapsed ? '...' : 'Loading version...'}</VersionBadge>
    )

    if (collapsed) {
      return (
        <Tooltip title="Loading version..." placement="right" arrow>
          {content}
        </Tooltip>
      )
    }
    return content
  }

  if (error) {
    const content = (
      <VersionBadge collapsed={collapsed}>{collapsed ? 'v?' : 'Version: Unable to load'}</VersionBadge>
    )

    if (collapsed) {
      return (
        <Tooltip title="Version: Unable to load" placement="right" arrow>
          {content}
        </Tooltip>
      )
    }
    return content
  }

  const content = (
    <VersionBadge collapsed={collapsed}>
      {collapsed ? `${versionInfo?.version || '?'}` : `Version: ${versionInfo?.version}`}
    </VersionBadge>
  )

  if (collapsed) {
    return (
      <Tooltip
        title={
          <Box>
            Version: {versionInfo?.version}
            {versionInfo?.upgradeAvailable && versionInfo?.upgradeVersion && (
              <Box component="span" sx={{ display: 'block', fontSize: '0.85rem', mt: 0.5 }}>
                Update available: {versionInfo.upgradeVersion}
              </Box>
            )}
          </Box>
        }
        placement="right"
        arrow
      >
        {content}
      </Tooltip>
    )
  }

  return content
})
