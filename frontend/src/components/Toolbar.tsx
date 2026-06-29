import { useState } from 'react'
import { ActionIcon, Group, Select, Modal, Stack, Button, Text, Paper } from '@mantine/core'

interface Props {
  sheetNames: string[]
  selectedSheet: string | null
  onOpenFile: () => void
  onSheetChange: (sheet: string) => void
  onExportExcel: () => void
  onImportExcel: () => void
  onSaveCSV: (encoding: string) => void
}

export default function Toolbar({ sheetNames, selectedSheet, onOpenFile, onSheetChange, onExportExcel, onImportExcel, onSaveCSV }: Props) {
  const [csvModalOpen, setCsvModalOpen] = useState(false)
  const [encoding, setEncoding] = useState<string | null>('ISO-8859-2')

  function handleSave() {
    setCsvModalOpen(false)
    onSaveCSV(encoding ?? 'ISO-8859-2')
  }

  return (
    <>
      <Paper shadow="xs" p="xs" radius={0} style={{ borderBottom: '1px solid var(--mantine-color-gray-3)' }}>
        <Group gap="sm">
          <ActionIcon
            variant="light"
            size="lg"
            title="Booked4us Excel megnyitása"
            onClick={onOpenFile}
          >
            📂
          </ActionIcon>

          {sheetNames.length > 0 && (
            <Select
              placeholder="Munkalap kiválasztása"
              data={sheetNames}
              value={selectedSheet}
              onChange={(v) => v && onSheetChange(v)}
              w={220}
              size="sm"
            />
          )}

          <ActionIcon
            variant="light"
            size="lg"
            title="Munkaadatok exportálása Excel fájlba"
            onClick={onExportExcel}
            disabled={sheetNames.length === 0}
          >
            📤
          </ActionIcon>

          <ActionIcon
            variant="light"
            size="lg"
            title="Munkaadatok importálása Excel fájlból"
            onClick={onImportExcel}
          >
            📥
          </ActionIcon>

          <ActionIcon
            variant="filled"
            color="blue"
            size="lg"
            title="CSV mentése (Számlázz.hu)"
            onClick={() => setCsvModalOpen(true)}
          >
            💾
          </ActionIcon>
        </Group>
      </Paper>

      <Modal opened={csvModalOpen} onClose={() => setCsvModalOpen(false)} title="CSV mentése" size="sm">
        <Stack>
          <Text size="sm">Válassz kódolást:</Text>
          <Select
            data={['ISO-8859-2', 'UTF-8']}
            value={encoding}
            onChange={setEncoding}
          />
          <Text size="xs" c="dimmed">
            ISO-8859-2 a Számlázz.hu ajánlott formátuma.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setCsvModalOpen(false)}>Mégse</Button>
            <Button onClick={handleSave}>Mentés</Button>
          </Group>
        </Stack>
      </Modal>
    </>
  )
}
