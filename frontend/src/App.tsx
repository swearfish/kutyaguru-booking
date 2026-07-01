import { useState, useCallback, useEffect, useMemo } from 'react'
import { AppShell, Code, Modal, ScrollArea, SegmentedControl, Table, useMantineColorScheme } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { notifications } from '@mantine/notifications'
import * as main from '../bindings/kutyaguru'
// Wails v3 namespaces service methods under `Booking`; destructure them so the
// existing call sites stay unchanged, and keep the `main.*` alias for models
// (TableDataResult, Field, …) which the root index re-exports.
const {
  OpenBookedFile,
  LoadRecentFile,
  LoadSheet,
  GetFields,
  UpdateFieldValue,
  ReapplyFields,
  UpdateCell,
  SetRowEnabled,
  SetAllRowsEnabled,
  ExportToExcel,
  ImportFromExcel,
  SaveCSV,
  PreviewCSV,
  GetStatus,
  GetSettings,
  GetColumnRoles,
  GetRecentFiles,
  SetColorScheme,
  SetEncoding,
  SetCharMapping,
  SetServicePrices,
} = main.Booking
import Toolbar from './components/Toolbar'
import DataTab from './components/DataTab'
import FieldsTab from './components/FieldsTab'
import MappingTab from './components/MappingTab'
import PricesTab from './components/PricesTab'
import StatusBar from './components/StatusBar'
import NavSidebar from './components/NavSidebar'
import SheetTabs from './components/SheetTabs'

const emptyTable: main.TableDataResult = new main.TableDataResult({ columns: [], rows: [], rowEnabled: [], cellErrors: [] })

type View = 'table' | 'fields' | 'mapping' | 'prices' | 'manual'
type ColorScheme = 'light' | 'dark' | 'auto'

export default function App() {
  const { setColorScheme: mantineSetColorScheme } = useMantineColorScheme()

  const [view, setView] = useState<View>('table')
  const [sheetNames, setSheetNames] = useState<string[]>([])
  const [selectedSheet, setSelectedSheet] = useState<string | null>(null)
  const [fields, setFields] = useState<main.Field[]>([])
  const [tableData, setTableData] = useState<main.TableDataResult>(emptyTable)
  const [status, setStatus] = useState<string>('')
  const [colorScheme, setColorScheme] = useState<ColorScheme>('auto')
  const [encoding, setEncoding] = useState<string>('ISO-8859-2')
  const [charMapping, setCharMapping] = useState<Record<string, string>>({})
  const [servicePrices, setServicePrices] = useState<Record<string, string>>({})
  const [recentFiles, setRecentFiles] = useState<string[]>([])
  const [columnRoles, setColumnRoles] = useState<main.ColumnRoles | null>(null)
  const [previewText, setPreviewText] = useState<string>('')
  const [previewMode, setPreviewMode] = useState<'table' | 'raw'>('table')
  const [filterText, setFilterText] = useState<string>('')
  const [previewOpened, previewHandlers] = useDisclosure(false)

  // Hydrate state from persisted settings once on mount. mantineSetColorScheme is
  // listed for exhaustive-deps honesty; it's a stable ref from useMantineColorScheme,
  // so the effect still runs exactly once (the state setters are stable by React
  // contract and need no listing).
  useEffect(() => {
    GetSettings().then(s => {
      const scheme = (s.colorScheme as ColorScheme) || 'auto'
      setColorScheme(scheme)
      mantineSetColorScheme(scheme)
      setEncoding(s.encoding || 'ISO-8859-2')
      // v3 binds Go map[string]string as { [k: string]?: string }; the backend
      // never emits undefined values, so narrow to Record<string, string>.
      setCharMapping((s.charMapping ?? {}) as Record<string, string>)
      setServicePrices((s.servicePrices ?? {}) as Record<string, string>)
      setRecentFiles(s.recentFiles || [])
    })
  }, [mantineSetColorScheme])

  // The service/price column names come from the backend (which owns the schema)
  // rather than being re-typed here. They are constant for the app's lifetime.
  useEffect(() => {
    GetColumnRoles().then(setColumnRoles)
  }, [])

  // Distinct service values found in the current sheet, plus the price default.
  const services = useMemo(() => {
    if (!columnRoles) return []
    const idx = tableData.columns.indexOf(columnRoles.service)
    if (idx < 0) return []
    return [...new Set(tableData.rows.map(r => r[idx]).filter(v => v && v.trim() !== ''))]
  }, [tableData, columnRoles])
  const defaultPrice = useMemo(
    () => (columnRoles ? fields.find(f => f.name === columnRoles.price)?.value ?? '' : ''),
    [fields, columnRoles],
  )

  // Error / warning counts for the status bar.
  const { errorCount, warningCount } = useMemo(() => {
    let errorCount = 0, warningCount = 0
    for (const ce of (tableData.cellErrors ?? [])) {
      // Disabled rows aren't exported, so their issues don't count toward the
      // status bar (and no longer block CSV export — keep the two consistent).
      if (tableData.rowEnabled[ce.rowIndex] === false) continue
      if (ce.severity === 'error') errorCount++
      else if (ce.severity === 'warning') warningCount++
    }
    return { errorCount, warningCount }
  }, [tableData.cellErrors, tableData.rowEnabled])

  const loadFirstSheet = useCallback(async (sheets: string[]) => {
    setSheetNames(sheets)
    setSelectedSheet(sheets[0])
    const st = await GetStatus()
    setStatus(st)
    const result = await LoadSheet(sheets[0])
    setTableData(result)
    const f = await GetFields()
    setFields(f)
    setRecentFiles(await GetRecentFiles())
  }, [])

  const handleOpenFile = useCallback(async () => {
    try {
      const sheets = await OpenBookedFile()
      if (!sheets || sheets.length === 0) return
      await loadFirstSheet(sheets)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [loadFirstSheet])

  const handleOpenRecent = useCallback(async (path: string) => {
    try {
      const sheets = await LoadRecentFile(path)
      if (!sheets || sheets.length === 0) return
      await loadFirstSheet(sheets)
    } catch (err: any) {
      setRecentFiles(await GetRecentFiles()) // a missing file is dropped server-side
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [loadFirstSheet])

  const handleSheetChange = useCallback(async (sheet: string) => {
    setSelectedSheet(sheet)
    try {
      const result = await LoadSheet(sheet)
      setTableData(result)
      const f = await GetFields()
      setFields(f)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleExportExcel = useCallback(async () => {
    try {
      const saved = await ExportToExcel()
      if (saved) notifications.show({ color: 'green', message: 'Excel fájl sikeresen mentve.' })
    } catch (err: any) {
      if (err) notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleImportExcel = useCallback(async () => {
    try {
      const result = await ImportFromExcel()
      setTableData(result)
    } catch (err: any) {
      if (err) notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleSaveCSV = useCallback(async () => {
    try {
      const saved = await SaveCSV()
      if (saved) notifications.show({ color: 'green', message: 'CSV sikeresen mentve.' })
    } catch (err: any) {
      if (err) notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleCellChange = useCallback(async (rowIndex: number, colName: string, value: string) => {
    try {
      const result = await UpdateCell(rowIndex, colName, value)
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleToggleRow = useCallback(async (rowIndex: number, enabled: boolean) => {
    try {
      const result = await SetRowEnabled(rowIndex, enabled)
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleToggleAll = useCallback(async (enabled: boolean) => {
    try {
      const result = await SetAllRowsEnabled(enabled)
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleFieldChange = useCallback(async (fieldName: string, value: string) => {
    try {
      await UpdateFieldValue(fieldName, value)
      setFields(prev => prev.map(f => f.name === fieldName ? { ...f, value } : f))
      const result = await ReapplyFields()
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  // Settings are persisted via single-field backend mutators (SetColorScheme,
  // SetEncoding, SetCharMapping, SetServicePrices, UpdateFieldValue) so the
  // frontend never reconstructs a full Settings object — that previously wiped
  // server-managed fields like the char map / prices / recent files.
  const handleColorSchemeChange = useCallback(async (scheme: string) => {
    const s = scheme as ColorScheme
    setColorScheme(s)
    mantineSetColorScheme(s)
    try {
      await SetColorScheme(s)
    } catch { /* non-critical */ }
  }, [])

  const handleSetCharMapping = useCallback(async (m: Record<string, string>) => {
    setCharMapping(m)
    try {
      const result = await SetCharMapping(m)
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handleAddToMapping = useCallback((char: string) => {
    const m = { ...charMapping, [char]: '-' }
    handleSetCharMapping(m)
  }, [charMapping, handleSetCharMapping])

  const handleSetServicePrices = useCallback(async (m: Record<string, string>) => {
    setServicePrices(m)
    try {
      const result = await SetServicePrices(m)
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  const handlePreview = useCallback(async () => {
    try {
      const text = await PreviewCSV()
      setPreviewText(text)
      previewHandlers.open()
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [previewHandlers])

  // Parse the raw CSV preview into a grid mirroring the Számlázz.hu layout:
  // a 3-row header (grouping line + partner-level + item-level column names),
  // then two output lines per invoice record. Cells align only within each
  // matching line type — the same shape the saved file (and Excel) uses.
  const previewRows = useMemo(() => {
    const lines = previewText.replace(/\n+$/, '').split('\n')
    if (lines.length === 0 || (lines.length === 1 && lines[0] === '')) return []
    const rows = lines.map(l => l.split(';'))
    let maxCol = 0
    for (const r of rows) {
      for (let i = r.length - 1; i >= 0; i--) {
        if (r[i].trim() !== '') { maxCol = Math.max(maxCol, i + 1); break }
      }
    }
    return rows.map(r => {
      const padded = r.slice(0, maxCol)
      while (padded.length < maxCol) padded.push('')
      return padded
    })
  }, [previewText])

  const handleEncodingChange = useCallback(async (enc: string) => {
    setEncoding(enc)
    try {
      const result = await SetEncoding(enc)
      setTableData(result)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  return (
    <AppShell
      header={{ height: 44 }}
      navbar={{ width: 50, breakpoint: 0 }}
      footer={{ height: 26 }}
      padding={0}
    >
      <AppShell.Header>
        <Toolbar
          onOpenFile={handleOpenFile}
          onOpenRecent={handleOpenRecent}
          recentFiles={recentFiles}
          onExportExcel={handleExportExcel}
          onImportExcel={handleImportExcel}
          onSaveCSV={handleSaveCSV}
          onPreview={handlePreview}
          hasData={sheetNames.length > 0}
          filterText={filterText}
          onFilterChange={setFilterText}
        />
      </AppShell.Header>

      <AppShell.Navbar>
        <NavSidebar
          view={view}
          onViewChange={setView}
          colorScheme={colorScheme}
          encoding={encoding}
          onColorSchemeChange={handleColorSchemeChange}
          onEncodingChange={handleEncodingChange}
        />
      </AppShell.Navbar>

      <AppShell.Main style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {view === 'table' && (
          <div style={{ flex: 1, overflow: 'hidden', minHeight: 0, display: 'flex', flexDirection: 'column' }}>
            <div style={{ flex: 1, overflow: 'hidden', minHeight: 0 }}>
              <DataTab tableData={tableData} onCellChange={handleCellChange} onAddToMapping={handleAddToMapping} onToggleRow={handleToggleRow} onToggleAll={handleToggleAll} charMapping={charMapping} filterText={filterText} />
            </div>
            <SheetTabs sheets={sheetNames} selected={selectedSheet} onChange={handleSheetChange} />
          </div>
        )}
        {view === 'fields' && (
          <div style={{ flex: 1, overflow: 'hidden', minHeight: 0 }}>
            <FieldsTab fields={fields} onFieldChange={handleFieldChange} />
          </div>
        )}
        {view === 'mapping' && (
          <div style={{ flex: 1, overflow: 'hidden', minHeight: 0 }}>
            <MappingTab
              charMapping={charMapping}
              unmappedChars={[...new Set(
                tableData.cellErrors.filter(ce => ce.severity === 'error').map(ce => ce.invalidChar)
              )]}
              onChange={handleSetCharMapping}
            />
          </div>
        )}
        {view === 'prices' && (
          <div style={{ flex: 1, overflow: 'hidden', minHeight: 0 }}>
            <PricesTab
              servicePrices={servicePrices}
              services={services}
              defaultPrice={defaultPrice}
              onChange={handleSetServicePrices}
            />
          </div>
        )}
        {view === 'manual' && (
          <div style={{ flex: 1, overflow: 'hidden', minHeight: 0, display: 'flex' }}>
            {/* Static Hungarian manual bundled under frontend/public/manual/ →
                embedded into the binary via //go:embed all:frontend/dist.
                flex:1 (not height:100%) so the iframe fills the flex column —
                a percentage height won't resolve against a flex-item parent. */}
            <iframe
              src="./manual/index.html"
              title="Kézikönyv"
              style={{ flex: 1, width: '100%', border: 0 }}
            />
          </div>
        )}
      </AppShell.Main>

      <AppShell.Footer>
        <StatusBar status={status} errorCount={errorCount} warningCount={warningCount} />
      </AppShell.Footer>

      <Modal opened={previewOpened} onClose={previewHandlers.close} title="CSV előnézet" size="xl">
        <SegmentedControl
          mb="sm"
          value={previewMode}
          onChange={v => setPreviewMode(v as 'table' | 'raw')}
          data={[{ label: 'Táblázat', value: 'table' }, { label: 'Nyers szöveg', value: 'raw' }]}
        />
        <ScrollArea h="60vh">
          {previewMode === 'raw' ? (
            <Code block style={{ whiteSpace: 'pre', fontSize: 12 }}>{previewText}</Code>
          ) : (
            <Table withTableBorder withColumnBorders stickyHeader fz="xs" style={{ whiteSpace: 'nowrap' }}>
              <Table.Thead>
                {previewRows.slice(0, 3).map((row, ri) => (
                  <Table.Tr key={ri}>
                    {row.map((cell, ci) => <Table.Th key={ci}>{cell}</Table.Th>)}
                  </Table.Tr>
                ))}
              </Table.Thead>
              <Table.Tbody>
                {previewRows.slice(3).map((row, ri) => (
                  <Table.Tr
                    key={ri}
                    style={ri % 2 === 0 ? { borderTop: '2px solid var(--mantine-color-default-border)' } : undefined}
                  >
                    {row.map((cell, ci) => <Table.Td key={ci}>{cell}</Table.Td>)}
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
          )}
        </ScrollArea>
      </Modal>
    </AppShell>
  )
}
