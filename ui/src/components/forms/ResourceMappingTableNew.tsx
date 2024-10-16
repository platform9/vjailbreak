import {
  FormControl,
  FormHelperText,
  InputLabel,
  MenuItem,
  Select,
  Typography,
} from "@mui/material"
import { useMemo, useState, useCallback } from "react"
import AddCircleOutlineIcon from "@mui/icons-material/AddCircleOutline"

import DeleteIcon from "@mui/icons-material/Delete"
import {
  IconButton,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
} from "@mui/material"

export interface ResourceMap {
  source: string
  target: string
}

interface ResourceMappingProps {
  label?: string
  sourceItems: string[]
  targetItems: string[]
  sourceLabel: string // Label for the source dropdown
  targetLabel: string // Label for the target dropdown
  values: ResourceMap[]
  onChange: (mappings: ResourceMap[]) => void
  error?: string
}

export default function ResourceMappingTable({
  label,
  sourceItems,
  targetItems,
  sourceLabel,
  targetLabel,
  values = [],
  onChange,
  error,
}: ResourceMappingProps) {
  const [selectedSourceItem, setSelectedSourceItem] = useState("")
  const [selectedTargetItem, setSelectedTargetItem] = useState("")

  const handleAddMapping = useCallback(() => {
    if (selectedSourceItem && selectedTargetItem) {
      const updatedMappings = [
        ...values,
        {
          source: selectedSourceItem,
          target: selectedTargetItem,
        },
      ]

      onChange(updatedMappings)
      setSelectedSourceItem("")
      setSelectedTargetItem("")
    }
  }, [values, selectedSourceItem, selectedTargetItem, onChange])

  const handleDeleteMapping = useCallback(
    (mapping: ResourceMap) => {
      const updatedMappings = values.filter(
        ({ source, target }) =>
          mapping.source !== source || mapping.target !== target
      )

      onChange(updatedMappings)
    },
    [values, onChange]
  )

  // Filter out already mapped source and target items
  const availableSourceItems = useMemo(
    () =>
      sourceItems.filter(
        (item) => !values.some((mapping) => mapping.source === item)
      ),
    [sourceItems, values]
  )

  const availableTargetItems = useMemo(
    () =>
      targetItems.filter(
        (item) => !values.some((mapping) => mapping.target === item)
      ),
    [targetItems, values]
  )

  const renderValues = useCallback(() => {
    return values.length ? (
      values?.map((mapping, index) => (
        <TableRow key={index}>
          <TableCell>{mapping.source}</TableCell>
          <TableCell>{mapping.target}</TableCell>
          <TableCell align="right">
            <IconButton
              color="error"
              size="small"
              onClick={() => handleDeleteMapping(mapping)}
              aria-label="delete-mapping"
            >
              <DeleteIcon />
            </IconButton>
          </TableCell>
        </TableRow>
      ))
    ) : (
      <TableRow>
        <TableCell colSpan={3}>No Mappings Created</TableCell>
      </TableRow>
    )
  }, [values, handleDeleteMapping])

  return (
    <div>
      {label && <Typography variant="body1">{label}</Typography>}
      <TableContainer component={Paper} sx={{ mt: 2, mb: 4 }}>
        <Table size="small" aria-label="resource-mapping">
          <TableHead>
            <TableRow sx={{ height: "50px" }}>
              <TableCell>{sourceLabel}</TableCell>
              <TableCell>{targetLabel}</TableCell>
              <TableCell align="right">Action</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {renderValues()}
            <TableRow sx={{ height: "60px" }}>
              <TableCell width={400}>
                <FormControl
                  fullWidth
                  size="small"
                  disabled={availableSourceItems.length === 0}
                >
                  <InputLabel id="source-item-label">{sourceLabel}</InputLabel>
                  <Select
                    labelId="source-item-label"
                    value={selectedSourceItem}
                    onChange={(e) => setSelectedSourceItem(e.target.value)}
                    label={sourceLabel}
                    fullWidth
                  >
                    {availableSourceItems.map((item) => (
                      <MenuItem key={item} value={item}>
                        {item}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </TableCell>
              <TableCell width={400}>
                <FormControl
                  fullWidth
                  size="small"
                  disabled={availableTargetItems.length === 0}
                >
                  <InputLabel id="target-item-label">{targetLabel}</InputLabel>
                  <Select
                    labelId="target-item-label"
                    value={selectedTargetItem}
                    onChange={(e) => setSelectedTargetItem(e.target.value)}
                    label={targetLabel}
                  >
                    {availableTargetItems.map((item) => (
                      <MenuItem key={item} value={item}>
                        {item}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              </TableCell>
              <TableCell align="right">
                <IconButton
                  size="small"
                  color="primary"
                  onClick={handleAddMapping}
                  disabled={!selectedSourceItem || !selectedTargetItem}
                  aria-label="add-mapping"
                >
                  <AddCircleOutlineIcon />
                </IconButton>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>
      {!!error && <FormHelperText error>{error}</FormHelperText>}
    </div>
  )
}
