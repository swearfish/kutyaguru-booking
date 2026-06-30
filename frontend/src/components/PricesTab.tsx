import { useState } from 'react'
import { ActionIcon, Button, Group, ScrollArea, Table, Text, TextInput } from '@mantine/core'

interface Props {
  servicePrices: Record<string, string>
  services: string[] // distinct service values found in the loaded sheet
  defaultPrice: string // current "Nettó egységár" default for new entries
  onChange: (m: Record<string, string>) => void
}

export default function PricesTab({ servicePrices, services, defaultPrice, onChange }: Props) {
  const [editValues, setEditValues] = useState<Record<string, string>>({})

  function getDisplayValue(svc: string): string {
    return svc in editValues ? editValues[svc] : servicePrices[svc]
  }

  function commitEdit(svc: string) {
    if (!(svc in editValues)) return
    const val = editValues[svc]
    const updated = { ...servicePrices, [svc]: val }
    setEditValues(prev => { const n = { ...prev }; delete n[svc]; return n })
    onChange(updated)
  }

  function handleDelete(svc: string) {
    const updated = { ...servicePrices }
    delete updated[svc]
    onChange(updated)
  }

  function handleAddAll() {
    const toAdd = services.filter(s => !(s in servicePrices))
    if (toAdd.length === 0) return
    const updated = { ...servicePrices }
    for (const s of toAdd) updated[s] = defaultPrice
    onChange(updated)
  }

  const missingCount = services.filter(s => !(s in servicePrices)).length
  const entries = Object.keys(servicePrices).sort()

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', padding: 16, gap: 12 }}>
      <Group>
        <Text size="sm" fw={600}>Szolgáltatás árak — Nettó egységár</Text>
        <Button
          size="xs"
          variant="light"
          color="orange"
          disabled={missingCount === 0}
          onClick={handleAddAll}
        >
          Összes szolgáltatás hozzáadása ({missingCount})
        </Button>
      </Group>

      {entries.length === 0 ? (
        <Text size="sm" c="dimmed">
          Nincs bejegyzés. Tölts be egy munkalapot, majd használd az "Összes szolgáltatás hozzáadása" gombot,
          vagy add meg az árakat egyesével.
        </Text>
      ) : (
        <ScrollArea style={{ flex: 1 }}>
          <Table withTableBorder withColumnBorders highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Szolgáltatás</Table.Th>
                <Table.Th w={180}>Nettó egységár</Table.Th>
                <Table.Th w={50}></Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {entries.map(svc => (
                <Table.Tr key={svc}>
                  <Table.Td>
                    <Text size="sm">{svc}</Text>
                  </Table.Td>
                  <Table.Td>
                    <TextInput
                      size="xs"
                      value={getDisplayValue(svc)}
                      onChange={e => setEditValues(prev => ({ ...prev, [svc]: e.target.value }))}
                      onBlur={() => commitEdit(svc)}
                      onKeyDown={e => { if (e.key === 'Enter') commitEdit(svc) }}
                    />
                  </Table.Td>
                  <Table.Td>
                    <ActionIcon variant="subtle" color="red" size="sm" onClick={() => handleDelete(svc)}>
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
