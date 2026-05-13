import React, { useCallback, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  IconButton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import EditOutlinedIcon from '@mui/icons-material/EditOutlined'
import CheckIcon from '@mui/icons-material/Check'
import CloseIcon from '@mui/icons-material/Close'

export interface HostEntry {
  ip: string
  hostnames: string[]
}

export interface HostEntriesTabProps {
  value: string
  onChange: (v: string) => void
  disabled?: boolean
}

const isValidIP = (ip: string): boolean =>
  /^(\d{1,3}\.){3}\d{1,3}$/.test(ip.trim()) || /^[0-9a-fA-F:]+$/.test(ip.trim())

const isValidHostname = (h: string): boolean =>
  /^[a-zA-Z0-9]([a-zA-Z0-9\-.]*[a-zA-Z0-9])?$/.test(h) || /^[a-zA-Z0-9]$/.test(h)

const parseEntries = (value: string): HostEntry[] => {
  if (!value || value.trim() === '' || value.trim() === '[]') return []
  try {
    return JSON.parse(value) as HostEntry[]
  } catch {
    return []
  }
}

const serializeEntries = (entries: HostEntry[]): string => JSON.stringify(entries)

interface RowState {
  ip: string
  hostnamesRaw: string
}

const EMPTY_ROW: RowState = { ip: '', hostnamesRaw: '' }

function validateRow(row: RowState, entries: HostEntry[], editingIdx?: number): string | null {
  if (!row.ip.trim()) return 'IP is required'
  if (!isValidIP(row.ip.trim())) return 'Invalid IP address'

  const hostnames = row.hostnamesRaw
    .split(',')
    .map((h) => h.trim())
    .filter(Boolean)
  if (hostnames.length === 0) return 'At least one hostname is required'
  const bad = hostnames.find((h) => !isValidHostname(h))
  if (bad) return `Invalid hostname: "${bad}"`

  const duplicate = entries.find(
    (e, i) => e.ip === row.ip.trim() && i !== editingIdx
  )
  if (duplicate) return `Duplicate IP: ${row.ip.trim()}`

  return null
}

export function HostEntriesTab({ value, onChange, disabled }: HostEntriesTabProps) {
  const entries = useMemo(() => parseEntries(value), [value])

  const [adding, setAdding] = useState(false)
  const [addRow, setAddRow] = useState<RowState>(EMPTY_ROW)
  const [addError, setAddError] = useState<string | null>(null)

  const [editingIdx, setEditingIdx] = useState<number | null>(null)
  const [editRow, setEditRow] = useState<RowState>(EMPTY_ROW)
  const [editError, setEditError] = useState<string | null>(null)

  const commit = useCallback(
    (newEntries: HostEntry[]) => onChange(serializeEntries(newEntries)),
    [onChange]
  )

  const handleAddStart = () => {
    setAdding(true)
    setAddRow(EMPTY_ROW)
    setAddError(null)
  }

  const handleAddConfirm = () => {
    const err = validateRow(addRow, entries)
    if (err) {
      setAddError(err)
      return
    }
    const hostnames = addRow.hostnamesRaw
      .split(',')
      .map((h) => h.trim())
      .filter(Boolean)
    commit([...entries, { ip: addRow.ip.trim(), hostnames }])
    setAdding(false)
    setAddRow(EMPTY_ROW)
    setAddError(null)
  }

  const handleAddCancel = () => {
    setAdding(false)
    setAddRow(EMPTY_ROW)
    setAddError(null)
  }

  const handleEditStart = (idx: number) => {
    setEditingIdx(idx)
    setEditRow({ ip: entries[idx].ip, hostnamesRaw: entries[idx].hostnames.join(', ') })
    setEditError(null)
  }

  const handleEditConfirm = () => {
    if (editingIdx === null) return
    const err = validateRow(editRow, entries, editingIdx)
    if (err) {
      setEditError(err)
      return
    }
    const hostnames = editRow.hostnamesRaw
      .split(',')
      .map((h) => h.trim())
      .filter(Boolean)
    const updated = entries.map((e, i) =>
      i === editingIdx ? { ip: editRow.ip.trim(), hostnames } : e
    )
    commit(updated)
    setEditingIdx(null)
    setEditRow(EMPTY_ROW)
    setEditError(null)
  }

  const handleEditCancel = () => {
    setEditingIdx(null)
    setEditRow(EMPTY_ROW)
    setEditError(null)
  }

  const handleDelete = (idx: number) => {
    commit(entries.filter((_, i) => i !== idx))
  }

  return (
    <Box>
      <Box display="flex" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="body2" color="text.secondary">
          Custom hostname-to-IP mappings injected into agent nodes at provisioning time.
          Supports ESXi hosts, vCenter, PCD, and OpenStack endpoints.
        </Typography>
        <Button
          size="small"
          startIcon={<AddIcon />}
          onClick={handleAddStart}
          disabled={disabled || adding || editingIdx !== null}
          data-testid="host-entries-add-btn"
        >
          Add Entry
        </Button>
      </Box>

      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell sx={{ fontWeight: 600, width: '180px' }}>IP Address</TableCell>
              <TableCell sx={{ fontWeight: 600 }}>Hostnames (comma-separated)</TableCell>
              <TableCell sx={{ width: '88px' }} />
            </TableRow>
          </TableHead>
          <TableBody>
            {entries.map((entry, idx) =>
              editingIdx === idx ? (
                <TableRow key={idx}>
                  <TableCell>
                    <TextField
                      size="small"
                      value={editRow.ip}
                      onChange={(e) => setEditRow((r) => ({ ...r, ip: e.target.value }))}
                      disabled={disabled}
                      inputProps={{ 'data-testid': 'host-entry-edit-ip' }}
                    />
                  </TableCell>
                  <TableCell>
                    <TextField
                      size="small"
                      fullWidth
                      value={editRow.hostnamesRaw}
                      onChange={(e) => setEditRow((r) => ({ ...r, hostnamesRaw: e.target.value }))}
                      disabled={disabled}
                      inputProps={{ 'data-testid': 'host-entry-edit-hostnames' }}
                      helperText={editError}
                      error={Boolean(editError)}
                    />
                  </TableCell>
                  <TableCell>
                    <Tooltip title="Save">
                      <IconButton size="small" onClick={handleEditConfirm} data-testid="host-entry-edit-save">
                        <CheckIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Cancel">
                      <IconButton size="small" onClick={handleEditCancel} data-testid="host-entry-edit-cancel">
                        <CloseIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ) : (
                <TableRow key={idx}>
                  <TableCell>
                    <Typography variant="body2" fontFamily="monospace">
                      {entry.ip}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">{entry.hostnames.join(', ')}</Typography>
                  </TableCell>
                  <TableCell>
                    <Tooltip title="Edit">
                      <IconButton
                        size="small"
                        onClick={() => handleEditStart(idx)}
                        disabled={disabled || adding || editingIdx !== null}
                        data-testid={`host-entry-edit-${idx}`}
                      >
                        <EditOutlinedIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                    <Tooltip title="Delete">
                      <IconButton
                        size="small"
                        onClick={() => handleDelete(idx)}
                        disabled={disabled || adding || editingIdx !== null}
                        data-testid={`host-entry-delete-${idx}`}
                      >
                        <DeleteOutlineIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              )
            )}

            {adding && (
              <TableRow>
                <TableCell>
                  <TextField
                    size="small"
                    placeholder="192.168.1.10"
                    value={addRow.ip}
                    onChange={(e) => setAddRow((r) => ({ ...r, ip: e.target.value }))}
                    disabled={disabled}
                    inputProps={{ 'data-testid': 'host-entry-new-ip' }}
                  />
                </TableCell>
                <TableCell>
                  <TextField
                    size="small"
                    fullWidth
                    placeholder="vcenter.corp.local, vcenter"
                    value={addRow.hostnamesRaw}
                    onChange={(e) => setAddRow((r) => ({ ...r, hostnamesRaw: e.target.value }))}
                    disabled={disabled}
                    inputProps={{ 'data-testid': 'host-entry-new-hostnames' }}
                    helperText={addError}
                    error={Boolean(addError)}
                  />
                </TableCell>
                <TableCell>
                  <Tooltip title="Add">
                    <IconButton size="small" onClick={handleAddConfirm} data-testid="host-entry-add-confirm">
                      <CheckIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title="Cancel">
                    <IconButton size="small" onClick={handleAddCancel} data-testid="host-entry-add-cancel">
                      <CloseIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </TableCell>
              </TableRow>
            )}

            {entries.length === 0 && !adding && (
              <TableRow>
                <TableCell colSpan={3}>
                  <Typography variant="body2" color="text.secondary" sx={{ py: 2, textAlign: 'center' }}>
                    No host entries configured. Click "Add Entry" to add the first one.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {entries.length > 0 && (
        <Alert severity="info" sx={{ mt: 2 }}>
          Changes apply to newly provisioned agent nodes only. Use "Reprovision Node" in the
          Nodes table to apply the updated config to an existing idle node.
        </Alert>
      )}
    </Box>
  )
}
