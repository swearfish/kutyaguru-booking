import { Button, Group, Paper, Tooltip } from '@mantine/core'

interface Props {
  onOpenFile: () => void
  onExportExcel: () => void
  onImportExcel: () => void
  onSaveCSV: () => void
  hasData: boolean
}

export default function Toolbar({ onOpenFile, onExportExcel, onImportExcel, onSaveCSV, hasData }: Props) {
  return (
    <Paper h="100%" px="xs" radius={0} style={{ borderBottom: '1px solid var(--mantine-color-default-border)', display: 'flex', alignItems: 'center' }}>
      <Group gap="xs">
        <Tooltip label="Booked4us Excel megnyitása" position="bottom" withArrow>
          <Button variant="subtle" size="sm" leftSection="📂" onClick={onOpenFile}>
            Megnyitás
          </Button>
        </Tooltip>
        <Tooltip label="Munkaadatok exportálása Excel fájlba" position="bottom" withArrow>
          <Button variant="subtle" size="sm" leftSection="📤" onClick={onExportExcel} disabled={!hasData}>
            Excel export
          </Button>
        </Tooltip>
        <Tooltip label="Munkaadatok importálása Excel fájlból" position="bottom" withArrow>
          <Button variant="subtle" size="sm" leftSection="📥" onClick={onImportExcel}>
            Excel import
          </Button>
        </Tooltip>
        <Tooltip label="CSV mentése Számlázz.hu formátumban" position="bottom" withArrow>
          <Button variant="filled" color="blue" size="sm" leftSection="💾" onClick={onSaveCSV} disabled={!hasData}>
            CSV mentése
          </Button>
        </Tooltip>
      </Group>
    </Paper>
  )
}
