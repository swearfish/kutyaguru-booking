import { useEffect, useMemo, useRef, useState } from 'react'
import { DataGrid, renderTextEditor } from 'react-data-grid'
import type { Column, DataGridHandle, RenderCellProps, RowsChangeData, SortColumn } from 'react-data-grid'
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
  filterText: string
  scrollTarget: { rowIndex: number; nonce: number } | null
}

function applyMapping(value: string, mapping: Record<string, string>): string {
  if (Object.keys(mapping).length === 0) return value
  return [...value].map(ch => mapping[ch] ?? ch).join('')
}

// Fold case and strip diacritics so a free-text search matches accented text
// (Hungarian: "arvi" matches "árví"). Sorting uses the same normalization via
// localeCompare's 'base' sensitivity.
function normalize(value: string): string {
  return value.toLowerCase().normalize('NFD').replace(/\p{Diacritic}/gu, '')
}

export default function DataTab({ tableData, onCellChange, onAddToMapping, onToggleRow, onToggleAll, charMapping, filterText, scrollTarget }: Props) {
  const computedScheme = useComputedColorScheme('light')
  const [sortColumns, setSortColumns] = useState<readonly SortColumn[]>([])
  const gridRef = useRef<DataGridHandle>(null)
  const [flashRow, setFlashRow] = useState<number | null>(null)

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

  // The grid's columns split into two concerns with different triggers, kept as
  // separate memos so each rebuilds only for its own inputs (and so the deps are
  // honest). NOTE: the backend hands back freshly-allocated arrays on every
  // mutation, so in practice both still recompute on each edit — the split is for
  // clarity and correct dependencies, not a rebuild-count optimization.

  // enabledColumn: the frozen row-toggle checkbox. Driven by row on/off state —
  // the header tri-state (all/some/none) and each row's own flag.
  const enabledColumn = useMemo<Column<GridRow>>(() => {
    const total = (tableData.rows ?? []).length
    let onCount = 0
    for (let i = 0; i < total; i++) {
      if (tableData.rowEnabled[i] ?? true) onCount++
    }
    const allOn = total > 0 && onCount === total
    const someOn = onCount > 0 && onCount < total
    return {
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
        const checked = tableData.rowEnabled[idx] ?? true
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
  }, [tableData.rows, tableData.rowEnabled, onToggleRow, onToggleAll])

  // dataColumns: one editable column per output field. Driven by the schema and
  // the validation maps (cell tinting + tooltips) — not by row on/off state.
  const dataColumns = useMemo<Column<GridRow>[]>(() => {
    return tableData.columns.map(colName => ({
      key: colName,
      name: colName,
      editable: true,
      resizable: true,
      sortable: true,
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
  }, [tableData.columns, errorMap, mappedMap, warnMap, charMapping, onAddToMapping])

  const columns = useMemo<Column<GridRow>[]>(
    () => [enabledColumn, ...dataColumns],
    [enabledColumn, dataColumns],
  )

  const rows = useMemo<GridRow[]>(() => {
    return (tableData.rows ?? []).map((row, rowIndex) => {
      const obj: GridRow = { __rowIndex: rowIndex }
      for (let ci = 0; ci < tableData.columns.length; ci++) {
        obj[tableData.columns[ci]] = row[ci] ?? ''
      }
      return obj
    })
  }, [tableData])

  // displayRows is the view: a free-text filter (row kept if any column contains
  // the query) followed by an optional single-column sort. An empty sortColumns
  // (the default, and what a third header click restores) keeps natural import
  // order — i.e. the sequential record number the CSV is keyed on. Filtering and
  // sorting are view-only; export reads the backend's original rows.
  const displayRows = useMemo<GridRow[]>(() => {
    const q = normalize(filterText.trim())
    let view = rows
    if (q) {
      view = view.filter(row =>
        tableData.columns.some(col => normalize(String(row[col] ?? '')).includes(q)),
      )
    }
    const sort = sortColumns[0]
    if (sort) {
      const dir = sort.direction === 'DESC' ? -1 : 1
      view = [...view].sort((a, b) =>
        String(a[sort.columnKey] ?? '').localeCompare(
          String(b[sort.columnKey] ?? ''),
          'hu',
          { numeric: true, sensitivity: 'base' },
        ) * dir,
      )
    }
    return view
  }, [rows, tableData.columns, filterText, sortColumns])

  // Feature 3: scroll to and briefly highlight a row requested from the problem
  // list. The nonce on scrollTarget makes repeated clicks on the same row re-fire.
  useEffect(() => {
    if (!scrollTarget) return
    const displayIdx = displayRows.findIndex(r => r.__rowIndex === scrollTarget.rowIndex)
    if (displayIdx < 0) return
    gridRef.current?.scrollToCell({ rowIdx: displayIdx, idx: 1 }) // idx 1 = first data column
    setFlashRow(scrollTarget.rowIndex)
    const t = setTimeout(() => setFlashRow(null), 2000)
    return () => clearTimeout(t)
    // displayRows intentionally omitted: the target object identity (nonce) is the
    // trigger; re-running when the filtered view changes would re-flash spuriously.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scrollTarget])

  function handleRowsChange(newRows: GridRow[], data: RowsChangeData<GridRow>) {
    const colName = data.column.key
    if (colName === '__enabled') return
    // newRows is in display order (filtered/sorted); translate each changed
    // position back to its backend row index before updating the model.
    for (const rowIdx of data.indexes) {
      const r = newRows[rowIdx]
      onCellChange(r.__rowIndex, colName, String(r[colName] ?? ''))
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
      ref={gridRef}
      className={computedScheme === 'dark' ? 'rdg-dark' : 'rdg-light'}
      columns={columns}
      rows={displayRows}
      rowKeyGetter={(row: GridRow) => row.__rowIndex}
      rowClass={(row: GridRow) => {
        const classes: string[] = []
        if (tableData.rowEnabled[row.__rowIndex] === false) classes.push('disabledRow')
        if (row.__rowIndex === flashRow) classes.push('highlightRow')
        return classes.length > 0 ? classes.join(' ') : undefined
      }}
      sortColumns={sortColumns}
      onSortColumnsChange={cols => setSortColumns(cols.slice(-1))}
      onRowsChange={handleRowsChange}
      style={{ height: '100%', blockSize: '100%' }}
    />
  )
}
