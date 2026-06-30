import { Button, CloseButton, Group, Menu, Paper, Text, TextInput, Tooltip } from '@mantine/core'

interface Props {
  onOpenFile: () => void
  onOpenRecent: (path: string) => void
  recentFiles: string[]
  onExportExcel: () => void
  onImportExcel: () => void
  onSaveCSV: () => void
  onPreview: () => void
  hasData: boolean
  filterText: string
  onFilterChange: (value: string) => void
}

// baseName returns the file name portion of a path for compact display.
function baseName(path: string): string {
  const parts = path.split(/[/\\]/)
  return parts[parts.length - 1] || path
}

export default function Toolbar({
  onOpenFile, onOpenRecent, recentFiles, onExportExcel, onImportExcel, onSaveCSV, onPreview, hasData,
  filterText, onFilterChange,
}: Props) {
  return (
    <Paper h="100%" px="xs" radius={0} style={{ borderBottom: '1px solid var(--mantine-color-default-border)', display: 'flex', alignItems: 'center' }}>
      <Group gap="xs">
        <Group gap={0} wrap="nowrap">
          <Tooltip label="Booked4us Excel megnyitása" position="bottom" withArrow>
            <Button
              variant="subtle"
              size="sm"
              leftSection="📂"
              onClick={onOpenFile}
              styles={{ root: { borderTopRightRadius: 0, borderBottomRightRadius: 0 } }}
            >
              Megnyitás
            </Button>
          </Tooltip>
          <Menu position="bottom-start" withArrow shadow="md" width={320}>
            <Menu.Target>
              <Button
                variant="subtle"
                size="sm"
                px={6}
                aria-label="Legutóbbi fájlok"
                styles={{ root: { borderTopLeftRadius: 0, borderBottomLeftRadius: 0 } }}
              >
                ▾
              </Button>
            </Menu.Target>
            <Menu.Dropdown>
              <Menu.Item leftSection="📂" onClick={onOpenFile}>Tallózás…</Menu.Item>
              <Menu.Divider />
              <Menu.Label>Legutóbbi fájlok</Menu.Label>
              {recentFiles.length > 0 ? (
                recentFiles.map(path => (
                  <Menu.Item key={path} onClick={() => onOpenRecent(path)}>
                    <Text size="sm" truncate title={path}>{baseName(path)}</Text>
                  </Menu.Item>
                ))
              ) : (
                <Menu.Item disabled>Nincsenek legutóbbi fájlok</Menu.Item>
              )}
            </Menu.Dropdown>
          </Menu>
        </Group>

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
        <Tooltip label="A CSV előnézete mentés előtt" position="bottom" withArrow>
          <Button variant="subtle" size="sm" leftSection="👁" onClick={onPreview} disabled={!hasData}>
            Előnézet
          </Button>
        </Tooltip>
        <Tooltip label="CSV mentése Számlázz.hu formátumban" position="bottom" withArrow>
          <Button variant="filled" color="blue" size="sm" leftSection="💾" onClick={onSaveCSV} disabled={!hasData}>
            CSV mentése
          </Button>
        </Tooltip>
      </Group>

      <TextInput
        ml="auto"
        size="sm"
        w={240}
        leftSection="🔍"
        placeholder="Keresés minden oszlopban…"
        aria-label="Szabadszavas keresés"
        value={filterText}
        onChange={e => onFilterChange(e.currentTarget.value)}
        disabled={!hasData}
        rightSectionPointerEvents="auto"
        rightSection={
          filterText ? (
            <CloseButton size="sm" aria-label="Keresés törlése" onClick={() => onFilterChange('')} />
          ) : null
        }
      />
    </Paper>
  )
}
