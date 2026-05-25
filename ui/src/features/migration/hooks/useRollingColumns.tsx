import * as React from 'react'
import { Box, Tooltip } from '@mui/material'
import { styled } from '@mui/material/styles'
import { GridColDef, GridRowSelectionModel } from '@mui/x-data-grid'
import type { OpenStackFlavor } from 'src/api/openstack-creds/model'
import { OsFamilyCell, RollingIpAddressCell, RollingFlavorCell } from '../components/cells'
import type { VM } from '../types'

const CdsIconWrapper = styled('div')({
  marginRight: 8,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
})

interface UseRollingColumnsParams {
  selectedVMs: GridRowSelectionModel
  vmOSAssignments: Record<string, string>
  openstackFlavors: OpenStackFlavor[]
  handleOSAssignment: (vmId: string, os: string) => void
  handleFlavorChange: (vmId: string, flavorId: string) => void
}

export function useRollingColumns({
  selectedVMs,
  vmOSAssignments,
  openstackFlavors,
  handleOSAssignment,
  handleFlavorChange,
}: UseRollingColumnsParams): GridColDef[] {
  return React.useMemo(
    () => [
      {
        field: 'name',
        headerName: 'VM Name',
        flex: 1.3,
        minWidth: 150,
        hideable: false,
        renderCell: (params) => (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Tooltip title={params.row.powerState === 'powered-on' ? 'Powered On' : 'Powered Off'}>
              <CdsIconWrapper>
                {/* eslint-disable-next-line @typescript-eslint/ban-ts-comment */}
                {/* @ts-ignore */}
                <cds-icon
                  shape="vm"
                  size="md"
                  badge={params.row.powerState === 'powered-on' ? 'success' : 'danger'}
                ></cds-icon>
              </CdsIconWrapper>
            </Tooltip>
            <Box sx={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{params.value}</Box>
          </Box>
        ),
      },
      {
        field: 'ip',
        headerName: 'IP Address(es)',
        flex: 1,
        hideable: true,
        renderCell: (params) => (
          <RollingIpAddressCell
            vm={params.row as VM}
            isSelected={selectedVMs.includes(params.row.id)}
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
            showSelectWhenSelected={
              selectedVMs.includes(params.row.id) && params.row?.powerState === 'powered-off'
            }
            unknownFallbackLabel="Other"
            showWarningForPoweredOffOnly={true}
            onOSAssignment={handleOSAssignment}
          />
        ),
      },
      {
        field: 'networks',
        headerName: 'Network Interface(s)',
        flex: 1,
        hideable: true,
        valueGetter: (value) => value || '—',
      },
      {
        field: 'cpu',
        headerName: 'CPU',
        flex: 0.3,
        hideable: true,
        valueGetter: (value) => value || '- ',
      },
      {
        field: 'memory',
        headerName: 'Memory (MB)',
        flex: 0.8,
        hideable: true,
        valueGetter: (value) => value || '—',
      },
      {
        field: 'esxHost',
        headerName: 'ESX Host',
        flex: 1,
        hideable: true,
        valueGetter: (value) => value || '—',
      },
      {
        field: 'flavor',
        headerName: 'Flavor',
        flex: 1,
        hideable: true,
        renderCell: (params) => (
          <RollingFlavorCell
            vmId={params.row.id}
            currentFlavor={params.value || 'auto-assign'}
            isSelected={selectedVMs.includes(params.row.id)}
            openstackFlavors={openstackFlavors}
            onFlavorChange={handleFlavorChange}
          />
        ),
      },
      {
        field: 'powerState',
        headerName: 'Power State',
        hideable: true,
        flex: 0.8,
        valueGetter: (value) => value || '—',
      },
    ],
    [selectedVMs, vmOSAssignments, openstackFlavors, handleOSAssignment, handleFlavorChange]
  )
}
