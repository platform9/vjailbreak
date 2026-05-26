import * as React from 'react'
import { Box, Tooltip, Chip } from '@mui/material'
import { styled } from '@mui/material/styles'
import { GridColDef } from '@mui/x-data-grid'
import WarningIcon from '@mui/icons-material/Warning'
import InfoIcon from '@mui/icons-material/Info'
import { OsFamilyCell, StandardIpAddressCell } from '../components/cells'
import type { VmDataWithFlavor } from '../types'

const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
})

interface UseStandardColumnsParams {
  selectedVMs: Set<string>
  duplicateNames: Set<string>
  vmOSAssignments: Record<string, string>
  originalIPsPerVM: Record<string, Record<number, string>>
  handleOSAssignment: (vmId: string, os: string) => void
}

export function useStandardColumns({
  selectedVMs,
  duplicateNames,
  vmOSAssignments,
  originalIPsPerVM,
  handleOSAssignment,
}: UseStandardColumnsParams): GridColDef[] {
  return React.useMemo(
    () => [
      {
        field: 'name',
        headerName: 'VM Name',
        flex: 2.5,
        renderCell: (params) => {
          const displayName = duplicateNames.has(params.row.name)
            ? params.row.vmKey || params.value
            : params.value
          return (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Tooltip title={params.row.vmState === 'running' ? 'Running' : 'Stopped'}>
                <CdsIconWrapper>
                  {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                  {/* @ts-ignore */}
                  <cds-icon
                    shape="vm"
                    size="md"
                    badge={params.row.vmState === 'running' ? 'success' : 'danger'}
                  >
                    {/* @ts-ignore */}
                  </cds-icon>
                </CdsIconWrapper>
              </Tooltip>
              <Box>{displayName}</Box>
              {params.row.isMigrated && (
                <Chip variant="outlined" label="Migrated" color="info" size="small" />
              )}
              {params.row.flavorNotFound && (
                <Box display="flex" alignItems="center" gap={0.5}>
                  <WarningIcon color="warning" fontSize="small" />
                </Box>
              )}
              {params.row.hasSharedRdm && (
                <Tooltip title="This VM has shared RDM disks">
                  <Chip
                    variant="outlined"
                    label="RDM"
                    color="secondary"
                    size="small"
                    sx={{ fontSize: '0.7rem', height: '20px' }}
                  />
                </Tooltip>
              )}
            </Box>
          )
        },
      },
      {
        field: 'ipAddress',
        headerName: 'IP Address(es)',
        flex: 0.8,
        minWidth: 190,
        hideable: true,
        renderCell: (params) => (
          <StandardIpAddressCell
            vm={params.row as VmDataWithFlavor}
            isSelected={selectedVMs.has(params.row.id)}
            originalIPsPerVM={originalIPsPerVM}
          />
        ),
      },
      {
        field: 'osFamily',
        headerName: 'Operating System',
        flex: 1,
        hideable: true,
        renderCell: (params) => (
          <OsFamilyCell
            vmId={params.row.id}
            powerState={params.row?.powerState}
            detectedOsFamily={params.row?.osFamily}
            assignedOsFamily={vmOSAssignments[params.row.id]}
            showSelectWhenSelected={selectedVMs.has(params.row.id)}
            unknownFallbackLabel="Unknown"
            showWarningForPoweredOffOnly={false}
            keepSelectMenuMounted={true}
            onOSAssignment={handleOSAssignment}
          />
        ),
      },
      {
        field: 'networks',
        headerName: 'Network Interface(s)',
        flex: 1.2,
        valueGetter: (value: string[]) => value?.join(', ') || '- ',
      },
      {
        field: 'cpuCount',
        headerName: 'CPU',
        flex: 0.7,
        valueGetter: (value) => value || '- ',
      },
      {
        field: 'memory',
        headerName: 'Memory (MB)',
        flex: 0.9,
        valueGetter: (value) => value || '- ',
      },
      {
        field: 'esxHost',
        headerName: 'ESX Host',
        flex: 1,
        valueGetter: (value) => value || '—',
      },
      {
        field: 'flavor',
        headerName: 'Flavor',
        flex: 1,
        getApplyQuickFilterFn: () => null,
        valueGetter: (value) => value || 'auto-assign',
        renderHeader: () => (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <div style={{ fontWeight: 500 }}>Flavor</div>
            <Tooltip title="Target PCD flavor to be assigned to this VM after migration.">
              <InfoIcon fontSize="small" sx={{ color: 'info.info', opacity: 0.7, cursor: 'help' }} />
            </Tooltip>
          </Box>
        ),
      },
      {
        field: 'rdmDisks',
        headerName: 'RDM Disks',
        flex: 1.2,
        hideable: true,
        valueGetter: (value: string[]) => value?.join(', ') || '—',
        renderHeader: () => (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <div style={{ fontWeight: 500 }}>RDM Disks</div>
            <Tooltip title="Raw Device Mapping disks associated with this VM.">
              <InfoIcon fontSize="small" sx={{ color: 'info.info', opacity: 0.7, cursor: 'help' }} />
            </Tooltip>
          </Box>
        ),
      },
      {
        field: 'vmState',
        headerName: 'Status',
        flex: 1,
        sortable: true,
        sortComparator: (v1, v2) => {
          if (v1 === 'running' && v2 === 'stopped') return -1
          if (v1 === 'stopped' && v2 === 'running') return 1
          return 0
        },
      },
    ],
    [selectedVMs, duplicateNames, vmOSAssignments, originalIPsPerVM, handleOSAssignment]
  )
}
