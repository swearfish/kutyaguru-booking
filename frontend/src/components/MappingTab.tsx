import { useState } from 'react'
import { ActionIcon, Button, Group, ScrollArea, Table, Text, TextInput } from '@mantine/core'

interface Props {
  charMapping: Record<string, string>
  unmappedChars: string[]
  onChange: (m: Record<string, string>) => void
}

export default function MappingTab({ charMapping, unmappedChars, onChange }: Props) {
  const [editValues, setEditValues] = useState<Record<string, string>>({})

  function getDisplayValue(char: string): string {
    return char in editValues ? editValues[char] : charMapping[char]
  }

  function commitEdit(char: string) {
    if (!(char in editValues)) return
    const val = editValues[char]
    const updated = { ...charMapping, [char]: val }
    setEditValues(prev => { const n = { ...prev }; delete n[char]; return n })
    onChange(updated)
  }

  function handleDelete(char: string) {
    const updated = { ...charMapping }
    delete updated[char]
    onChange(updated)
  }

  function handleAddAll() {
    const toAdd = unmappedChars.filter(c => !(c in charMapping))
    if (toAdd.length === 0) return
    const updated = { ...charMapping }
    for (const c of toAdd) updated[c] = '-'
    onChange(updated)
  }

  const entries = Object.keys(charMapping).sort()

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', padding: 16, gap: 12 }}>
      <Group>
        <Text size="sm" fw={600}>Karakter térkép — Latin-2 helyettesítők</Text>
        <Button
          size="xs"
          variant="light"
          color="orange"
          disabled={unmappedChars.filter(c => !(c in charMapping)).length === 0}
          onClick={handleAddAll}
        >
          Összes ismeretlen hozzáadása ({unmappedChars.filter(c => !(c in charMapping)).length})
        </Button>
      </Group>

      {entries.length === 0 ? (
        <Text size="sm" c="dimmed">
          Nincs bejegyzés. Kattints egy piros cellán a ➕ gombra, vagy használd az "Összes ismeretlen hozzáadása" gombot.
        </Text>
      ) : (
        <ScrollArea style={{ flex: 1 }}>
          <Table withTableBorder withColumnBorders highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={90}>Karakter</Table.Th>
                <Table.Th w={90}>Unicode</Table.Th>
                <Table.Th>Helyettesítő</Table.Th>
                <Table.Th w={50}></Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {entries.map(char => (
                <Table.Tr key={char}>
                  <Table.Td>
                    <Text ff="monospace" size="sm">{char}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="xs" c="dimmed">U+{char.codePointAt(0)!.toString(16).toUpperCase().padStart(4, '0')}</Text>
                  </Table.Td>
                  <Table.Td>
                    <TextInput
                      size="xs"
                      value={getDisplayValue(char)}
                      onChange={e => setEditValues(prev => ({ ...prev, [char]: e.target.value }))}
                      onBlur={() => commitEdit(char)}
                      onKeyDown={e => { if (e.key === 'Enter') commitEdit(char) }}
                      styles={{ input: { fontFamily: 'monospace' } }}
                    />
                  </Table.Td>
                  <Table.Td>
                    <ActionIcon variant="subtle" color="red" size="sm" onClick={() => handleDelete(char)}>
                      🗑
                    </ActionIcon>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      )}
    </div>
  )
}
