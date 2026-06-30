import { useMemo } from 'react'
import { DataGrid, renderTextEditor } from 'react-data-grid'
import type { Column, RenderCellProps, RowsChangeData } from 'react-data-grid'
import { ActionIcon, Tooltip, useComputedColorScheme } from '@mantine/core'
import * as main from '../../bindings/kutyaguru'
import './DataTab.css'

type GridRow = Record<string, string | number> & { __rowIndex: number }

interface Props {
  tableData: main.TableDataResult
  onCellChange: (rowIndex: number, colName: string, value: string) => void
  onAddToMapping: (char: string) => void
  onToggleRow: (rowIndex: number, enabled: boolean) => void
  onToggleAll: (enabled: boolean) => void
  charMapping: Record<string, string>
}

function applyMapping(value: string, mapping: Record<string, string>): string {
  if (Object.keys(mapping).length === 0) return value
  return [...value].map(ch => mapping[ch] ?? ch).join('')
}

export default function DataTab({ tableData, onCellChange, onAddToMapping, onToggleRow, onToggleAll, charMapping }: Props) {
  const computedScheme = useComputedColorScheme('light')

  const { errorMap, mappedMap, warnMap } = useMemo(() => {
    const errorMap = new Map<string, main.CellError>()
    const mappedMap = new Map<string, main.CellError>()
    const warnMap = new Map<string, main.CellError>()
    for (const ce of (tableData.cellErrors ?? [])) {
      const key = `${ce.rowIndex}:${ce.colName}`
      if (ce.severity === 'error') {
        errorMap.set(key, ce)
      } else if (ce.severity === 'warning') {
        warnMap.set(key, ce)
      } else {
        mappedMap.set(key, ce)
      }
    }
    return { errorMap, mappedMap, warnMap }
  }, [tableData.cellErrors])

  const columns = useMemo<Column<GridRow>[]>(() => {
    const total = (tableData.rows ?? []).length
    let onCount = 0
    for (let i = 0; i < total; i++) {
      if (tableData.rowEnabled?.[i] ?? true) onCount++
    }
    const allOn = total > 0 && onCount === total
    const someOn = onCount > 0 && onCount < total
    const enabledColumn: Column<GridRow> = {
      key: '__enabled',
      name: '',
      width: 40,
      minWidth: 40,
      maxWidth: 40,
      frozen: true,
      editable: false,
      resizable: false,
      cellClass: undefined,
      renderHeaderCell: () => (
        <input
          type="checkbox"
          aria-label="Összes sor ki/be kapcsolása"
          title="Összes sor ki/be"
          checked={allOn}
          ref={el => { if (el) el.indeterminate = someOn }}
          onChange={e => onToggleAll(e.target.checked)}
        />
      ),
      renderCell: (props: RenderCellProps<GridRow>) => {
        const idx = props.row.__rowIndex
        const checked = tableData.rowEnabled?.[idx] ?? true
        return (
          <input
            type="checkbox"
            aria-label="Sor ki/be kapcsolása"
            checked={checked}
            onClick={e => e.stopPropagation()}
            onChange={e => onToggleRow(idx, e.target.checked)}
          />
        )
      },
    }
    const dataCols = tableData.columns.map(colName => ({
      key: colName,
      name: colName,
      editable: true,
      resizable: true,
      width: 150,
      renderEditCell: renderTextEditor,
      cellClass: (row: GridRow) => {
        const key = `${row.__rowIndex}:${colName}`
        if (errorMap.has(key)) return 'errorCell'
        if (warnMap.has(key)) return 'warnCell'
        if (mappedMap.has(key)) return 'mappedCell'
        return undefined
      },
      renderCell: (props: RenderCellProps<GridRow>) => {
        const key = `${props.row.__rowIndex}:${colName}`
        const cellValue = String(props.row[colName] ?? '')
        const err = errorMap.get(key)
        const warn = warnMap.get(key)
        const mapped = mappedMap.get(key)
        if (err) {
          return (
            <Tooltip
              label={err.message || `Nem kódolható: "${err.invalidChar}"`}
              withArrow
            >
              <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center', gap: 4 }}>
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis' }}>{cellValue}</span>
                <ActionIcon
                  size="xs"
                  variant="subtle"
                  color="red"
                  title="Hozzáadás a karakter térképhez"
                  onClick={e => { e.stopPropagation(); onAddToMapping(err.invalidChar) }}
                >
                  ➕
                </ActionIcon>
              </div>
            </Tooltip>
          )
        }
        if (warn) {
          return (
            <Tooltip label={warn.message} withArrow>
              <div style={{ width: '100%', height: '100%', display: 'flex', alignItems: 'center' }}>
                {cellValue}
              </div>
            </Tooltip>
          )
        }
        if (mapped) {
          const exported = applyMapping(cellValue, charMapping)
          return (
            <Tooltip
              label={`Exportáláskor: "${exported}"`}
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
    return [enabledColumn, ...dataCols]
  }, [tableData.columns, tableData.rows, tableData.rowEnabled, errorMap, mappedMap, warnMap, charMapping, onToggleRow, onToggleAll])

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
    if (colName === '__enabled') return
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
      rowKeyGetter={(row: GridRow) => row.__rowIndex}
      rowClass={(row: GridRow) => (tableData.rowEnabled?.[row.__rowIndex] === false ? 'disabledRow' : undefined)}
      onRowsChange={handleRowsChange}
      style={{ height: '100%', blockSize: '100%' }}
    />
  )
}
