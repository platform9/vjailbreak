import React from 'react'
import {
  Box,
  FormControl,
  InputAdornment,
  MenuItem,
  Select,
  TextField,
  Typography
} from '@mui/material'
import { styled } from '@mui/material/styles'
import { FieldLabel } from 'src/components/design-system/ui'
import '@cds/core/icon/register.js'
import { ClarityIcons, clusterIcon, searchIcon } from '@cds/core/icon'

ClarityIcons.addIcons(clusterIcon, searchIcon)

const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center'
})

interface PcdClusterItem {
  id: string
  name?: string
  openstackCredName?: string
  tenantName?: string
}

interface RetrySourceDestinationSummaryProps {
  vmwareCredName?: string
  sourceCluster?: string
  openstackCredName?: string
  pcdClusters: PcdClusterItem[]
  selectedPcdClusterId: string
  onPcdClusterChange: (id: string) => void
  disabled?: boolean
}

// Read-only source environment + editable target cluster for retry mode.
// Credentials and source cluster are locked; only target cluster can change
// (which cascades a mapping reset in the parent form).
export function RetrySourceDestinationSummary({
  vmwareCredName,
  sourceCluster,
  openstackCredName,
  pcdClusters,
  selectedPcdClusterId,
  onPcdClusterChange,
  disabled
}: RetrySourceDestinationSummaryProps) {
  const [pcdSearchTerm, setPcdSearchTerm] = React.useState('')
  const [pcdDropdownOpen, setPcdDropdownOpen] = React.useState(false)

  const resolvedClusterId =
    pcdClusters.find((c) => c.id === selectedPcdClusterId)?.id ||
    pcdClusters.find((c) => c.name === selectedPcdClusterId)?.id ||
    selectedPcdClusterId

  const filteredPcdClusters = React.useMemo(() => {
    if (!pcdSearchTerm) return pcdClusters
    const term = pcdSearchTerm.toLowerCase().trim()
    return pcdClusters.filter(
      (c) =>
        (c.name || '').toLowerCase().includes(term) ||
        (c.openstackCredName || '').toLowerCase().includes(term) ||
        (c.tenantName || '').toLowerCase().includes(term)
    )
  }, [pcdClusters, pcdSearchTerm])

  const clusterDropdown = (
    <FormControl fullWidth size="small" disabled={disabled}>
      <Select
        value={resolvedClusterId}
        onChange={(e) => onPcdClusterChange(e.target.value)}
        onOpen={() => setPcdDropdownOpen(true)}
        onClose={() => {
          setPcdDropdownOpen(false)
          setPcdSearchTerm('')
        }}
        open={pcdDropdownOpen}
        displayEmpty
        inputProps={{ 'data-testid': 'retry-target-cluster-select' }}
        renderValue={(selected) => {
          if (!selected) return <em>Select PCD Cluster</em>
          const pcd = pcdClusters.find((c) => c.id === selected)
          return (
            <Box sx={{ display: 'flex', flexDirection: 'row', alignItems: 'center' }}>
              <Typography variant="body2" noWrap>
                {pcd?.name || selected}
              </Typography>
              {pcd?.tenantName && (
                <Typography variant="caption" color="text.secondary" noWrap sx={{ ml: 1 }}>
                  | Tenant: {pcd.tenantName}
                </Typography>
              )}
            </Box>
          )
        }}
        MenuProps={{
          PaperProps: { style: { maxHeight: 300 } },
          MenuListProps: { autoFocus: false }
        }}
      >
        <Box sx={{ p: 1, position: 'sticky', top: 0, bgcolor: 'background.paper', zIndex: 1 }}>
          <TextField
            size="small"
            placeholder="Search by cluster, credential, or tenant"
            fullWidth
            value={pcdSearchTerm}
            onChange={(e) => {
              e.stopPropagation()
              setPcdSearchTerm(e.target.value)
            }}
            onClick={(e) => {
              e.stopPropagation()
              if (!pcdDropdownOpen) setPcdDropdownOpen(true)
            }}
            onKeyDown={(e) => {
              e.stopPropagation()
              if (e.key === 'Backspace') e.nativeEvent.stopImmediatePropagation()
            }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                  {/* @ts-ignore */}
                  <cds-icon shape="search" size="sm"></cds-icon>
                </InputAdornment>
              )
            }}
          />
        </Box>
        <MenuItem value="" disabled>
          <em>Select PCD Cluster</em>
        </MenuItem>
        {pcdClusters.length === 0 ? (
          <MenuItem disabled>No PCD clusters found</MenuItem>
        ) : filteredPcdClusters.length === 0 ? (
          <MenuItem disabled>No matching clusters found</MenuItem>
        ) : (
          filteredPcdClusters.map((pcd) => (
            <MenuItem key={pcd.id} value={pcd.id}>
              <Box sx={{ display: 'flex', alignItems: 'center' }}>
                <CdsIconWrapper>
                  {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                  {/* @ts-ignore */}
                  <cds-icon shape="cluster" size="md"></cds-icon>
                </CdsIconWrapper>
                <Box sx={{ display: 'flex', flexDirection: 'column' }}>
                  <Typography variant="body1">{pcd.name}</Typography>
                  <Typography variant="caption" color="text.secondary">
                    Credential: {pcd.openstackCredName} | Tenant: {pcd.tenantName}
                  </Typography>
                </Box>
              </Box>
            </MenuItem>
          ))
        )}
      </Select>
    </FormControl>
  )

  const ROW = { display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 3 } as const
  const CELL = { display: 'grid', gridTemplateColumns: '160px 1fr', columnGap: 2, alignItems: 'center' } as const

  return (
    <Box data-testid="retry-source-destination-summary" sx={{ display: 'grid', gap: 1.5 }}>
      {/* Headers */}
      <Box sx={ROW}>
        <Typography variant="subtitle2" color="text.secondary">
          VMware
        </Typography>
        <Typography variant="subtitle2" color="text.secondary">
          PCD
        </Typography>
      </Box>

      {/* Credentials row */}
      <Box sx={ROW}>
        <Box sx={CELL}>
          <FieldLabel label="Source credential" />
          <Typography variant="body2">{vmwareCredName || '—'}</Typography>
        </Box>
        <Box sx={CELL}>
          <FieldLabel label="Destination credential" />
          <Typography variant="body2">{openstackCredName || '—'}</Typography>
        </Box>
      </Box>

      {/* Cluster row */}
      <Box sx={ROW}>
        <Box sx={CELL}>
          <FieldLabel label="Source cluster" />
          <Typography variant="body2">{sourceCluster || '—'}</Typography>
        </Box>
        <Box sx={CELL}>
          <FieldLabel label="Target cluster" />
          {clusterDropdown}
        </Box>
      </Box>

      <Typography variant="caption" color="text.secondary">
        Credentials and source cluster are locked while retrying a migration. Changing the target cluster will reset network and storage mappings.
      </Typography>
    </Box>
  )
}
