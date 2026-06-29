import { useState, useCallback, useEffect } from 'react'
import { AppShell, useMantineColorScheme } from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { main } from '../wailsjs/go/models'
import {
  OpenBookedFile,
  LoadSheet,
  GetFields,
  UpdateFieldValue,
  ReapplyFields,
  UpdateCell,
  ExportToExcel,
  ImportFromExcel,
  SaveCSV,
  GetStatus,
  GetSettings,
  SaveSettings,
  SetEncoding,
  SetCharMapping,
} from '../wailsjs/go/main/Booking'
import Toolbar from './components/Toolbar'
import DataTab from './components/DataTab'
import FieldsTab from './components/FieldsTab'
import MappingTab from './components/MappingTab'
import StatusBar from './components/StatusBar'
import NavSidebar from './components/NavSidebar'
import SheetTabs from './components/SheetTabs'

const emptyTable: main.TableDataResult = new main.TableDataResult({ columns: [], rows: [], cellErrors: [] })

type View = 'table' | 'fields' | 'mapping'
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

  // Load persisted settings on mount.
  useEffect(() => {
    GetSettings().then(s => {
      const scheme = (s.colorScheme as ColorScheme) || 'auto'
      setColorScheme(scheme)
      mantineSetColorScheme(scheme)
      setEncoding(s.encoding || 'ISO-8859-2')
      setCharMapping(s.charMapping || {})
    })
  }, [])

  const currentSettings = useCallback((): main.Settings => {
    const s = new main.Settings({})
    s.colorScheme = colorScheme
    s.encoding = encoding
    return s
  }, [colorScheme, encoding])

  const handleOpenFile = useCallback(async () => {
    try {
      const sheets = await OpenBookedFile()
      if (!sheets || sheets.length === 0) return
      setSheetNames(sheets)
      setSelectedSheet(sheets[0])
      const st = await GetStatus()
      setStatus(st)
      const result = await LoadSheet(sheets[0])
      setTableData(result)
      const f = await GetFields()
      setFields(f)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [])

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
      await ExportToExcel()
      notifications.show({ color: 'green', message: 'Excel fájl sikeresen mentve.' })
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
      await SaveCSV()
      notifications.show({ color: 'green', message: 'CSV sikeresen mentve.' })
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

  const handleColorSchemeChange = useCallback(async (scheme: string) => {
    const s = scheme as ColorScheme
    setColorScheme(s)
    mantineSetColorScheme(s)
    try {
      const settings = new main.Settings({
        colorScheme: s,
        encoding,
        charMapping,
        windowX: 0, windowY: 0, windowW: 0, windowH: 0,
      })
      await SaveSettings(settings)
    } catch { /* non-critical */ }
  }, [encoding, charMapping])

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

  const handleEncodingChange = useCallback(async (enc: string) => {
    setEncoding(enc)
    try {
      const result = await SetEncoding(enc)
      setTableData(result)
      const settings = new main.Settings({
        colorScheme,
        encoding: enc,
        charMapping,
        windowX: 0, windowY: 0, windowW: 0, windowH: 0,
      })
      await SaveSettings(settings)
    } catch (err: any) {
      notifications.show({ color: 'red', title: 'Hiba', message: String(err) })
    }
  }, [colorScheme, charMapping])

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
          onExportExcel={handleExportExcel}
          onImportExcel={handleImportExcel}
          onSaveCSV={handleSaveCSV}
          hasData={sheetNames.length > 0}
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
              <DataTab tableData={tableData} onCellChange={handleCellChange} onAddToMapping={handleAddToMapping} charMapping={charMapping} />
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
                tableData.cellErrors.filter(ce => !ce.mapped).map(ce => ce.invalidChar)
              )]}
              onChange={handleSetCharMapping}
            />
          </div>
        )}
      </AppShell.Main>

      <AppShell.Footer>
        <StatusBar status={status} />
      </AppShell.Footer>
    </AppShell>
  )
}
