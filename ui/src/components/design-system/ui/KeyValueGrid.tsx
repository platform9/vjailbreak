import { Box, BoxProps, Typography } from '@mui/material'
import { ReactNode } from 'react'

import FieldLabel from './FieldLabel'

export type KeyValueItem = {
  label: string
  value?: ReactNode
}

export interface KeyValueGridProps extends Omit<BoxProps, 'children'> {
  items: KeyValueItem[]
  labelWidth?: number
  mdGrids?: number
}

export default function KeyValueGrid({
  items,
  labelWidth = 150,
  mdGrids = 2,
  ...boxProps
}: KeyValueGridProps) {
  const normalizedMdGrids = Math.max(1, Math.floor(mdGrids))
  const mdGridTemplateColumns =
    normalizedMdGrids === 1
      ? `${labelWidth}px 1fr`
      : `${labelWidth}px 1.5fr ${Array.from({ length: normalizedMdGrids - 1 })
          .map(() => `${labelWidth}px 1fr`)
          .join(' ')}`

  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: {
          xs: '1fr',
          sm: `${labelWidth}px 1fr`,
          md: mdGridTemplateColumns
        },
        columnGap: 2,
        rowGap: 1.5,
        alignItems: 'center',
        justifyContent: 'right'
      }}
      {...boxProps}
    >
      {items.map((item) => {
        const value =
          item.value === undefined || item.value === null || item.value === '' ? '—' : item.value

        return (
          <Box
            key={item.label}
            sx={{
              display: 'contents'
            }}
          >
            <FieldLabel label={item.label} />
            <Box
              sx={{
                display: 'flex',
                alignItems: 'flex-start',
                justifyContent: 'space-between',
                gap: 1,
                minWidth: 0
              }}
            >
              {typeof value === 'string' || typeof value === 'number' ? (
                <Typography variant="body2" sx={{ wordBreak: 'break-word', flex: 1 }}>
                  {value}
                </Typography>
              ) : (
                <Box sx={{ flex: 1, minWidth: 0 }}>{value}</Box>
              )}
            </Box>
          </Box>
        )
      })}
    </Box>
  )
}
