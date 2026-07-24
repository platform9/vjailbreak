import { describe, expect, it } from 'vitest'
import { GRID_CHECKBOX_SELECTION_COL_DEF } from '@mui/x-data-grid'
import { defaultGetTogglableColumns } from './CommonDataGrid'

describe('defaultGetTogglableColumns', () => {
  it('excludes the row-selection checkbox column', () => {
    const columns = [
      { field: 'name' },
      { field: GRID_CHECKBOX_SELECTION_COL_DEF.field },
      { field: 'status' }
    ]

    expect(defaultGetTogglableColumns(columns)).toEqual(['name', 'status'])
  })

  it('returns all fields when no checkbox column is present', () => {
    const columns = [{ field: 'name' }, { field: 'status' }]

    expect(defaultGetTogglableColumns(columns)).toEqual(['name', 'status'])
  })
})
