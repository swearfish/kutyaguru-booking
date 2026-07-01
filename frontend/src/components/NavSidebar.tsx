import { ActionIcon, Popover, SegmentedControl, Select, Stack, Text, Tooltip } from '@mantine/core'

type View = 'table' | 'fields' | 'mapping' | 'prices' | 'manual'

// How a changed field default / service price propagates to already-loaded rows.
// Shared by the Fields and Prices workflows.
const applyModeData = [
  { value: 'never', label: 'Soha' },
  { value: 'match', label: 'Csak az egyezőkre' },
  { value: 'ask', label: 'Rákérdez' },
  { value: 'always', label: 'Mindig' },
]

interface Props {
  view: View
  onViewChange: (v: View) => void
  colorScheme: string
  encoding: string
  applyMode: string
  onColorSchemeChange: (s: string) => void
  onEncodingChange: (e: string) => void
  onApplyModeChange: (mode: string) => void
}

export default function NavSidebar({ view, onViewChange, colorScheme, encoding, applyMode, onColorSchemeChange, onEncodingChange, onApplyModeChange }: Props) {
  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      height: '100%',
      paddingTop: 6,
      paddingBottom: 6,
      borderRight: '1px solid var(--mantine-color-default-border)',
    }}>
      {/* View switchers — top */}
      <Tooltip label="Táblázat" position="right" withArrow>
        <ActionIcon
          variant={view === 'table' ? 'filled' : 'subtle'}
          size="xl"
          onClick={() => onViewChange('table')}
          mb={4}
        >
          📊
        </ActionIcon>
      </Tooltip>

      <Tooltip label="Mezők" position="right" withArrow>
        <ActionIcon
          variant={view === 'fields' ? 'filled' : 'subtle'}
          size="xl"
          onClick={() => onViewChange('fields')}
          mb={4}
        >
          🗂
        </ActionIcon>
      </Tooltip>

      <Tooltip label="Karakter térkép" position="right" withArrow>
        <ActionIcon
          variant={view === 'mapping' ? 'filled' : 'subtle'}
          size="xl"
          onClick={() => onViewChange('mapping')}
          mb={4}
        >
          🔤
        </ActionIcon>
      </Tooltip>

      <Tooltip label="Árak" position="right" withArrow>
        <ActionIcon
          variant={view === 'prices' ? 'filled' : 'subtle'}
          size="xl"
          onClick={() => onViewChange('prices')}
        >
          💰
        </ActionIcon>
      </Tooltip>

      {/* Manual + Settings — bottom */}
      <div style={{ marginTop: 'auto', display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
        <Tooltip label="Kézikönyv" position="right" withArrow>
          <ActionIcon
            variant={view === 'manual' ? 'filled' : 'subtle'}
            size="xl"
            onClick={() => onViewChange('manual')}
            mb={4}
          >
            📖
          </ActionIcon>
        </Tooltip>

        <Popover position="right-end" withArrow offset={8}>
          <Popover.Target>
            <Tooltip label="Beállítások" position="right" withArrow>
              <ActionIcon variant="subtle" size="xl">
                ⚙️
              </ActionIcon>
            </Tooltip>
          </Popover.Target>
          <Popover.Dropdown>
            <Stack gap="sm" w={210}>
              <Text size="xs" fw={600}>Téma</Text>
              <SegmentedControl
                size="xs"
                fullWidth
                data={[
                  { label: 'Auto', value: 'auto' },
                  { label: 'Világos', value: 'light' },
                  { label: 'Sötét', value: 'dark' },
                ]}
                value={colorScheme}
                onChange={onColorSchemeChange}
              />
              <Text size="xs" fw={600} mt={4}>CSV kódolás</Text>
              <SegmentedControl
                size="xs"
                fullWidth
                data={['ISO-8859-2', 'UTF-8']}
                value={encoding}
                onChange={onEncodingChange}
              />
              <Text size="xs" fw={600} mt={4}>Változtatás a betöltött sorokra</Text>
              <Select
                size="xs"
                data={applyModeData}
                value={applyMode}
                onChange={(v) => v && onApplyModeChange(v)}
                allowDeselect={false}
                comboboxProps={{ withinPortal: false }}
                description="Mezők és árak alapértékének módosításakor"
              />
            </Stack>
          </Popover.Dropdown>
        </Popover>
      </div>
    </div>
  )
}
