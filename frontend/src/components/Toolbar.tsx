import { ActionIcon, Group, Paper } from '@mantine/core'

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
        <ActionIcon variant="subtle" size="lg" title="Booked4us Excel megnyitása" onClick={onOpenFile}>
          📂
        </ActionIcon>
        <ActionIcon variant="subtle" size="lg" title="Munkaadatok exportálása Excel fájlba" onClick={onExportExcel} disabled={!hasData}>
          📤
        </ActionIcon>
        <ActionIcon variant="subtle" size="lg" title="Munkaadatok importálása Excel fájlból" onClick={onImportExcel}>
          📥
        </ActionIcon>
        <ActionIcon variant="filled" color="blue" size="lg" title="CSV mentése (Számlázz.hu)" onClick={onSaveCSV} disabled={!hasData}>
          💾
        </ActionIcon>
      </Group>
    </Paper>
  )
}
