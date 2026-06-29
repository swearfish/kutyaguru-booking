import { useMemo } from 'react'
import { DataGrid, renderTextEditor } from 'react-data-grid'
import type { Column, RenderCellProps, RowsChangeData } from 'react-data-grid'
import { Tooltip, useComputedColorScheme } from '@mantine/core'
import { main } from '../../wailsjs/go/models'
import './DataTab.css'

type GridRow = Record<string, string | number> & { __rowIndex: number }

interface Props {
  tableData: main.TableDataResult
  onCellChange: (rowIndex: number, colName: string, value: string) => void
}

export default function DataTab({ tableData, onCellChange }: Props) {
  const computedScheme = useComputedColorScheme('light')

  const errorMap = useMemo(() => {
    const map = new Map<string, main.CellError>()
    for (const ce of (tableData.cellErrors ?? [])) {
      map.set(`${ce.rowIndex}:${ce.colName}`, ce)
    }
    return map
  }, [tableData.cellErrors])

  const columns = useMemo<Column<GridRow>[]>(() => {
    return tableData.columns.map(colName => ({
      key: colName,
      name: colName,
      editable: true,
      resizable: true,
      width: 150,
      renderEditCell: renderTextEditor,
      cellClass: (row: GridRow) =>
        errorMap.has(`${row.__rowIndex}:${colName}`) ? 'errorCell' : undefined,
      renderCell: (props: RenderCellProps<GridRow>) => {
        const err = errorMap.get(`${props.row.__rowIndex}:${colName}`)
        const cellValue = String(props.row[colName] ?? '')
        if (err) {
          return (
            <Tooltip
              label={`Nem kódolható karakter: "${err.invalidChar}" (pozíció: ${err.charPos})`}
              withArrow
            >
              <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center' }}>
                {cellValue}
              </div>
            </Tooltip>
          )
        }
        return <>{cellValue}</>
      },
    }))
  }, [tableData.columns, errorMap])

  const rows = useMemo<GridRow[]>(() => {
    return (tableData.rows ?? []).map((row, rowIndex) => {
      const obj: GridRow = { __rowIndex: rowIndex }
      for (let ci = 0; ci < tableData.columns.length; ci++) {
        obj[tableData.columns[ci]] = row[ci] ?? ''
      }
      return obj
    })
  }, [tableData])

  function handleRowsChange(newRows: GridRow[], data: RowsChangeData<GridRow>) {
    const colName = data.column.key
    for (const rowIdx of data.indexes) {
      onCellChange(rowIdx, colName, String(newRows[rowIdx][colName] ?? ''))
    }
  }

  if (tableData.columns.length === 0) {
    return (
      <div style={{ padding: 16, color: '#888' }}>
        Nyiss meg egy Excel fájlt a munkalap betöltéséhez.
      </div>
    )
  }

  return (
    <DataGrid
      className={computedScheme === 'dark' ? 'rdg-dark' : 'rdg-light'}
      columns={columns}
      rows={rows}
      onRowsChange={handleRowsChange}
      style={{ height: '100%', blockSize: '100%' }}
    />
  )
}
