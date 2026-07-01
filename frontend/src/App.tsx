import { useState, useCallback, useEffect, useMemo } from 'react'
import { AppShell, Box, Button, Checkbox, Code, Group, List, Modal, ScrollArea, SegmentedControl, Stack, Table, Text, useMantineColorScheme } from '@mantine/core'
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
  ApplyFieldToRows,
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
  SetApplyMode,
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
// How a field-default change propagates to already-loaded rows.
type ApplyMode = 'never' | 'match' | 'ask' | 'always'

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
  // Feature 5: after a field default changes while a sheet is loaded, propagate it to
  // the loaded rows per applyMode ('never' | 'match' | 'ask' | 'always'). In 'ask' mode
  // pendingApply holds the field name + its previous value (needed for a "match" apply),
  // awaiting the user's choice (replace-latest if another commit arrives while open).
  const [applyMode, setApplyMode] = useState<ApplyMode>('ask')
  const [pendingApply, setPendingApply] = useState<{ name: string; prevValue: string } | null>(null)
  const [applyOpened, applyHandlers] = useDisclosure(false)
  // The same "apply to loaded rows" workflow for service-price EDITS. pendingPrice holds
  // the new price map awaiting the user's 'ask'-mode choice.
  const [pendingPrice, setPendingPrice] = useState<Record<string, string> | null>(null)
  const [priceApplyOpened, priceApplyHandlers] = useDisclosure(false)
  // "Ne kérdezd újra": when ticked, the button the user clicks in either apply modal is
  // promoted to the global applyMode default (Nem alkalmazom→never, Csak az egyezőkre→match,
  // Mindegyikre→always). Shared by both modals — only one is ever open at a time.
  const [rememberApply, setRememberApply] = useState(false)
  // Feature 3: the status-bar counter opens a list of problem rows; clicking one
  // scrolls the table to it and briefly highlights it. scrollTarget carries a nonce
  // so clicking the same row twice re-fires the effect (object identity changes).
  const [problemsOpened, problemsHandlers] = useDisclosure(false)
  const [scrollTarget, setScrollTarget] = useState<{ rowIndex: number; nonce: number } | null>(null)
  // Feature 2: when a CSV export is blocked by non-encodable characters, offer to add
  // them (blank) to the character map and jump to the Mapping view to fill in real
  // replacements.
  const [mapFixOpened, mapFixHandlers] = useDisclosure(false)

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
      setApplyMode((s.applyMode as ApplyMode) || 'ask')
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

  // Fields (defaults for prices, dates, etc.) are independent of any loaded sheet —
  // they come from the embedded field schema with persisted values. Fetch them on
  // mount so the Mezők editor is usable before an Excel file is opened. Loading a
  // sheet re-fetches them (loadFirstSheet / handleSheetChange), so this is idempotent.
  useEffect(() => {
    GetFields().then(setFields)
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

  // Problem rows for the status-bar list: enabled rows that carry an error or
  // warning, grouped by row and labelled with the partner name (falling back to
  // the record number when no partner column / value is present).
  const problemRows = useMemo(() => {
    const pIdx = columnRoles ? tableData.columns.indexOf(columnRoles.partner) : -1
    const byRow = new Map<number, { rowIndex: number; name: string; messages: string[] }>()
    for (const ce of (tableData.cellErrors ?? [])) {
      if (tableData.rowEnabled[ce.rowIndex] === false) continue
      if (ce.severity !== 'error' && ce.severity !== 'warning') continue
      let g = byRow.get(ce.rowIndex)
      if (!g) {
        const partner = pIdx >= 0 ? (tableData.rows[ce.rowIndex]?.[pIdx] ?? '').trim() : ''
        g = { rowIndex: ce.rowIndex, name: partner || `${ce.rowIndex + 1}. sor`, messages: [] }
        byRow.set(ce.rowIndex, g)
      }
      g.messages.push(ce.message || `${ce.colName}: ${ce.value}`)
    }
    return [...byRow.values()].sort((a, b) => a.rowIndex - b.rowIndex)
  }, [tableData, columnRoles])

  const goToRow = useCallback((rowIndex: number) => {
    setFilterText('')       // clear any active search so the row isn't filtered out
    setView('table')        // the grid only exists in the table view
    setScrollTarget(prev => ({ rowIndex, nonce: (prev?.nonce ?? 0) + 1 }))
    problemsHandlers.close()
  }, [problemsHandlers])

  // Non-encodable characters that actually block export: severity 'error' in an
  // enabled row (disabled rows aren't exported, so their chars don't block).
  const blockingChars = useMemo(() => [...new Set(
    (tableData.cellErrors ?? [])
      .filter(ce => ce.severity === 'error' && tableData.rowEnabled[ce.rowIndex] !== false)
      .map(ce => ce.invalidChar)
      .filter(Boolean)
  )], [tableData.cellErrors, tableData.rowEnabled])

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
      // If the export was blocked by non-encodable characters, offer the guided fix
      // (add them to the char map + open the Mapping view) instead of a bare toast.
      // Any other failure (e.g. a disk error) still shows the plain red notification.
      if (blockingChars.length > 0) {
        mapFixHandlers.open()
      } else if (err) {
        notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
      }
    }
  }, [blockingChars, mapFixHandlers])

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

  // applyToRows pushes the field's (already-persisted) new default onto loaded rows.
  // mode 'all' rewrites every row; mode 'match' rewrites only rows still carrying the
  // old default (prevValue) — leaving manually-edited rows alone.
  const applyToRows = useCallback(async (name: string, mode: 'all' | 'match', prevValue: string) => {
    try {
      setTableData(await ApplyFieldToRows(name, mode, prevValue))
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  // A field commit always persists the new default. If a sheet is loaded, applyMode
  // decides how it reaches the loaded rows: 'never' leaves them, 'always' rewrites all,
  // 'match' rewrites only rows still on the old default, 'ask' prompts. With no sheet
  // loaded there's nothing to apply, so we skip regardless of mode.
  const handleFieldChange = useCallback(async (fieldName: string, value: string) => {
    const prevValue = fields.find(f => f.name === fieldName)?.value ?? ''
    try {
      await UpdateFieldValue(fieldName, value)
      setFields(prev => prev.map(f => f.name === fieldName ? { ...f, value } : f))
      if (sheetNames.length === 0 || applyMode === 'never') return
      if (applyMode === 'always') { await applyToRows(fieldName, 'all', prevValue); return }
      if (applyMode === 'match') { await applyToRows(fieldName, 'match', prevValue); return }
      setPendingApply({ name: fieldName, prevValue }) // 'ask'
      applyHandlers.open()
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [fields, sheetNames, applyMode, applyToRows, applyHandlers])

  const handleApplyModeChange = useCallback(async (mode: string) => {
    setApplyMode(mode as ApplyMode)
    try {
      await SetApplyMode(mode)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  // Shared teardown for both apply modals: if "Ne kérdezd újra" is ticked, promote the
  // clicked action to the global default, then always reset the checkbox.
  const rememberIfAsked = useCallback((mode: 'never' | 'match' | 'always') => {
    if (rememberApply) handleApplyModeChange(mode)
    setRememberApply(false)
  }, [rememberApply, handleApplyModeChange])

  // The three "ask" modal choices. dismissApply just closes; the two apply variants
  // route to applyToRows with the pending field's previous value.
  const dismissApply = useCallback(() => {
    applyHandlers.close()
    setPendingApply(null)
    rememberIfAsked('never')
  }, [applyHandlers, rememberIfAsked])

  const chooseApply = useCallback(async (mode: 'all' | 'match') => {
    const p = pendingApply
    applyHandlers.close()
    setPendingApply(null)
    rememberIfAsked(mode === 'all' ? 'always' : 'match')
    if (p) await applyToRows(p.name, mode, p.prevValue)
  }, [pendingApply, applyHandlers, applyToRows, rememberIfAsked])

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

  // Feature 2 confirm: add each blocking character as a blank mapping entry (which
  // flips its cells from 'error' to 'mapped', unblocking export) and switch to the
  // Mapping view so the user can type real replacements. No auto-retry of the save.
  const handleConfirmMapFix = useCallback(async () => {
    const additions = Object.fromEntries(blockingChars.map(c => [c, '']))
    await handleSetCharMapping({ ...charMapping, ...additions })
    mapFixHandlers.close()
    setView('mapping')
  }, [blockingChars, charMapping, handleSetCharMapping, mapFixHandlers])

  // applyPrices persists the price lookup and pushes it to loaded rows per backendMode
  // ('never' | 'match' | 'all'). Optimistic setServicePrices already ran in the caller.
  const applyPrices = useCallback(async (m: Record<string, string>, backendMode: 'never' | 'match' | 'all') => {
    try {
      setTableData(await SetServicePrices(m, backendMode))
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

  // A price change always persists the lookup. The "apply to loaded rows" workflow (shared
  // with fields, via applyMode) governs only single price EDITS: 'never' leaves rows,
  // 'always' rewrites all, 'match' rewrites only rows still on the service's old price,
  // 'ask' prompts. Bulk-add / delete never push a new price onto existing rows, and with
  // no sheet loaded there's nothing to apply — both just persist ('all' is a no-op there).
  const handleSetServicePrices = useCallback(async (m: Record<string, string>, kind: 'edit' | 'delete' | 'addAll') => {
    setServicePrices(m)
    if (kind !== 'edit' || sheetNames.length === 0) { await applyPrices(m, 'all'); return }
    if (applyMode === 'never') { await applyPrices(m, 'never'); return }
    if (applyMode === 'always') { await applyPrices(m, 'all'); return }
    if (applyMode === 'match') { await applyPrices(m, 'match'); return }
    setPendingPrice(m) // 'ask'
    priceApplyHandlers.open()
  }, [sheetNames, applyMode, applyPrices, priceApplyHandlers])

  // "Nem alkalmazom" / modal-close: persist the new price but leave loaded rows as they are.
  const dismissPriceApply = useCallback(async () => {
    const m = pendingPrice
    priceApplyHandlers.close()
    setPendingPrice(null)
    rememberIfAsked('never')
    if (m) await applyPrices(m, 'never')
  }, [pendingPrice, priceApplyHandlers, applyPrices, rememberIfAsked])

  const choosePriceApply = useCallback(async (backendMode: 'match' | 'all') => {
    const m = pendingPrice
    priceApplyHandlers.close()
    setPendingPrice(null)
    rememberIfAsked(backendMode === 'all' ? 'always' : 'match')
    if (m) await applyPrices(m, backendMode)
  }, [pendingPrice, priceApplyHandlers, applyPrices, rememberIfAsked])

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
          applyMode={applyMode}
          onColorSchemeChange={handleColorSchemeChange}
          onEncodingChange={handleEncodingChange}
          onApplyModeChange={handleApplyModeChange}
        />
      </AppShell.Navbar>

      <AppShell.Main style={{ display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        {view === 'table' && (
          <div style={{ flex: 1, overflow: 'hidden', minHeight: 0, display: 'flex', flexDirection: 'column' }}>
            <div style={{ flex: 1, overflow: 'hidden', minHeight: 0 }}>
              <DataTab tableData={tableData} onCellChange={handleCellChange} onAddToMapping={handleAddToMapping} onToggleRow={handleToggleRow} onToggleAll={handleToggleAll} charMapping={charMapping} filterText={filterText} scrollTarget={scrollTarget} />
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
        <StatusBar
          status={status}
          errorCount={errorCount}
          warningCount={warningCount}
          onCounterClick={problemRows.length > 0 ? problemsHandlers.open : undefined}
        />
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

      <Modal opened={applyOpened} onClose={dismissApply} title="Alkalmazod a betöltött sorokra is?" size="lg">
        <Stack gap="sm">
          <Text size="sm">
            A(z) <b>{pendingApply?.name}</b> mező alapértéke frissült. Alkalmazod a már betöltött
            sorokra is?
          </Text>
          <Text size="xs" c="dimmed">
            A „Csak az egyezőkre” csak azokat a sorokat írja át, amelyek még a régi
            alapértéket tartalmazzák — a kézzel módosított sorok érintetlenek maradnak.
          </Text>
          <Checkbox
            size="xs"
            label="Ne kérdezd újra — a választott művelet lesz az alapértelmezett"
            checked={rememberApply}
            onChange={(e) => setRememberApply(e.currentTarget.checked)}
          />
          <Group justify="flex-end" gap="sm" wrap="nowrap">
            <Button variant="default" onClick={dismissApply}>Nem alkalmazom</Button>
            <Button variant="light" onClick={() => chooseApply('match')}>Csak az egyezőkre</Button>
            <Button onClick={() => chooseApply('all')}>Mindegyikre</Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={priceApplyOpened} onClose={dismissPriceApply} title="Alkalmazod a betöltött sorokra is?" size="lg">
        <Stack gap="sm">
          <Text size="sm">
            A szolgáltatás ára frissült. Alkalmazod a már betöltött sorokra is?
          </Text>
          <Text size="xs" c="dimmed">
            A „Csak az egyezőkre” csak azokat a sorokat írja át, amelyek még a régi
            árat tartalmazzák — a kézzel módosított sorok érintetlenek maradnak.
          </Text>
          <Checkbox
            size="xs"
            label="Ne kérdezd újra — a választott művelet lesz az alapértelmezett"
            checked={rememberApply}
            onChange={(e) => setRememberApply(e.currentTarget.checked)}
          />
          <Group justify="flex-end" gap="sm" wrap="nowrap">
            <Button variant="default" onClick={dismissPriceApply}>Nem alkalmazom</Button>
            <Button variant="light" onClick={() => choosePriceApply('match')}>Csak az egyezőkre</Button>
            <Button onClick={() => choosePriceApply('all')}>Mindegyikre</Button>
          </Group>
        </Stack>
      </Modal>

      <Modal opened={problemsOpened} onClose={problemsHandlers.close} title="Hibás sorok" size="lg">
        <Text size="sm" c="dimmed" mb="sm">
          Kattints egy sorra, hogy a táblázatban odaugorj.
        </Text>
        <ScrollArea.Autosize mah="60vh">
          <Stack gap="xs">
            {problemRows.map(p => (
              <Box
                key={p.rowIndex}
                onClick={() => goToRow(p.rowIndex)}
                style={{
                  cursor: 'pointer',
                  padding: '8px 10px',
                  borderRadius: 6,
                  border: '1px solid var(--mantine-color-default-border)',
                }}
              >
                <Text size="sm" fw={600}>{p.name}</Text>
                <List size="xs" c="dimmed" spacing={2} withPadding>
                  {p.messages.map((m, mi) => <List.Item key={mi}>{m}</List.Item>)}
                </List>
              </Box>
            ))}
          </Stack>
        </ScrollArea.Autosize>
      </Modal>

      <Modal opened={mapFixOpened} onClose={mapFixHandlers.close} title="Hiányzó karakter-leképezések" size="md">
        <Stack gap="sm">
          <Text size="sm">
            A CSV export nem sikerült, mert az alábbi karakterek nem kódolhatók a kiválasztott
            kódolással, és nincs hozzájuk leképezés. Hozzáadod őket a karakter térképhez? Üresen
            kerülnek be — a Karakter térkép nézetben tudod megadni a helyettesítő karaktereket.
          </Text>
          <Group gap={6}>
            {blockingChars.map(c => <Code key={c} style={{ fontSize: 14 }}>{c}</Code>)}
          </Group>
          <Group justify="flex-end" gap="sm">
            <Button variant="default" onClick={mapFixHandlers.close}>Mégse</Button>
            <Button onClick={handleConfirmMapFix}>Hozzáadás és megnyitás</Button>
          </Group>
        </Stack>
      </Modal>
    </AppShell>
  )
}
