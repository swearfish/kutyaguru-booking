import { useState, useCallback } from 'react'
import { Stack, Tabs } from '@mantine/core'
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
} from '../wailsjs/go/main/Booking'
import Toolbar from './components/Toolbar'
import DataTab from './components/DataTab'
import FieldsTab from './components/FieldsTab'
import StatusBar from './components/StatusBar'

const emptyTable: main.TableDataResult = new main.TableDataResult({ columns: [], rows: [], cellErrors: [] })

export default function App() {
  const [sheetNames, setSheetNames] = useState<string[]>([])
  const [selectedSheet, setSelectedSheet] = useState<string | null>(null)
  const [fields, setFields] = useState<main.Field[]>([])
  const [tableData, setTableData] = useState<main.TableDataResult>(emptyTable)
  const [status, setStatus] = useState<string>('')

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

  const handleSaveCSV = useCallback(async (encoding: string) => {
    try {
      await SaveCSV(encoding)
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

  return (
    <Stack h="100vh" gap={0}>
      <Toolbar
        sheetNames={sheetNames}
        selectedSheet={selectedSheet}
        onOpenFile={handleOpenFile}
        onSheetChange={handleSheetChange}
        onExportExcel={handleExportExcel}
        onImportExcel={handleImportExcel}
        onSaveCSV={handleSaveCSV}
      />
      <Tabs defaultValue="table" flex={1} style={{ display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <Tabs.List>
          <Tabs.Tab value="table">Táblázat</Tabs.Tab>
          <Tabs.Tab value="fields">Mezők</Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="table" style={{ flex: 1, overflow: 'hidden' }}>
          <DataTab tableData={tableData} onCellChange={handleCellChange} />
        </Tabs.Panel>
        <Tabs.Panel value="fields" style={{ flex: 1, overflow: 'auto' }}>
          <FieldsTab fields={fields} onFieldChange={handleFieldChange} />
        </Tabs.Panel>
      </Tabs>
      <StatusBar status={status} />
    </Stack>
  )
}
