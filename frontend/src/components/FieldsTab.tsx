import { useEffect, useState } from 'react'
import { ActionIcon, Table, TextInput, Select, Text, Badge, Box, Modal } from '@mantine/core'
import { useDisclosure } from '@mantine/hooks'
import { DatePicker } from '@mantine/dates'
import * as main from '../../bindings/kutyaguru'

interface Props {
  fields: main.Field[]
  onFieldChange: (fieldName: string, value: string) => void
}

// Stored dates use Hungarian dotted "YYYY.MM.DD"; Mantine's DatePicker speaks ISO
// "YYYY-MM-DD" strings. These convert between the two (toDash returns null when the
// draft isn't a complete valid dotted date, so a half-typed value seeds no selection).
const toDash = (s: string) => /^\d{4}\.\d{2}\.\d{2}$/.test(s) ? s.split('.').join('-') : null
const toDot = (s: string) => s.split('-').join('.')

// DateFieldInput: a free-text date input (draft-committed on blur/Enter like the other
// editable fields) with a 📅 button that opens a calendar modal. Picking a date commits
// immediately in the dotted format.
function DateFieldInput({ field, onFieldChange }: { field: main.Field; onFieldChange: (name: string, value: string) => void }) {
  const [draft, setDraft] = useState(field.value)
  const [opened, handlers] = useDisclosure(false)
  useEffect(() => setDraft(field.value), [field.value])

  const commit = () => { if (draft !== field.value) onFieldChange(field.name, draft) }
  return (
    <>
      <TextInput
        value={draft}
        onChange={(e) => setDraft(e.currentTarget.value)}
        onBlur={commit}
        onKeyDown={(e) => { if (e.key === 'Enter') e.currentTarget.blur() }}
        size="xs"
        w={180}
        placeholder="éééé.hh.nn"
        rightSection={
          <ActionIcon variant="subtle" size="sm" onClick={handlers.open} title="Naptár">
            📅
          </ActionIcon>
        }
      />
      <Modal opened={opened} onClose={handlers.close} title="Dátum kiválasztása" size="auto" centered>
        <DatePicker
          value={toDash(draft)}
          onChange={(v) => { if (v) { onFieldChange(field.name, toDot(v)); handlers.close() } }}
        />
      </Modal>
    </>
  )
}

// FieldValueCell renders the editable value for one field. Free-text edits are held
// in a local draft and committed only on blur / Enter — never per keystroke — so the
// parent's per-commit "apply to loaded rows?" prompt can't fire on every character.
// Selects commit immediately (atomic). MAPPING/CONST are read-only.
function FieldValueCell({ field, onFieldChange }: { field: main.Field; onFieldChange: (name: string, value: string) => void }) {
  const [draft, setDraft] = useState(field.value)
  useEffect(() => setDraft(field.value), [field.value])

  if (field.type === 'MAPPING' || field.type === 'CONST') {
    return <Text size="sm" c="dimmed">{field.value || field.mapping}</Text>
  }
  if (field.type === 'DATE') {
    return <DateFieldInput field={field} onFieldChange={onFieldChange} />
  }
  if (field.options && field.options.length > 0) {
    return (
      <Select
        data={field.options}
        value={field.value}
        onChange={(v) => v && onFieldChange(field.name, v)}
        size="xs"
        w={180}
      />
    )
  }
  const commit = () => { if (draft !== field.value) onFieldChange(field.name, draft) }
  return (
    <TextInput
      value={draft}
      onChange={(e) => setDraft(e.currentTarget.value)}
      onBlur={commit}
      onKeyDown={(e) => { if (e.key === 'Enter') e.currentTarget.blur() }}
      size="xs"
      w={180}
    />
  )
}

const typeLabel: Record<string, string> = {
  MAPPING: 'Leképezés',
  CONST: 'Állandó',
  TEXT: 'Szöveg',
  DATE: 'Dátum',
}

const typeColor: Record<string, string> = {
  MAPPING: 'blue',
  CONST: 'gray',
  TEXT: 'green',
  DATE: 'orange',
}

export default function FieldsTab({ fields, onFieldChange }: Props) {
  if (fields.length === 0) {
    return (
      <Box p="md">
        <Text c="dimmed" size="sm">Nincsenek betölthető mezők.</Text>
      </Box>
    )
  }

  return (
    <Box p="md">
      <Table striped withTableBorder withColumnBorders>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Mező</Table.Th>
            <Table.Th>Típus</Table.Th>
            <Table.Th>Érték</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {fields.map(field => (
            <Table.Tr key={field.name}>
              <Table.Td>
                <Text size="sm" fw={500}>{field.name}</Text>
                {field.mapping && (
                  <Text size="xs" c="dimmed">← {field.mapping}</Text>
                )}
              </Table.Td>
              <Table.Td>
                <Badge color={typeColor[field.type] ?? 'gray'} variant="light" size="sm">
                  {typeLabel[field.type] ?? field.type}
                </Badge>
              </Table.Td>
              <Table.Td>
                <FieldValueCell field={field} onFieldChange={onFieldChange} />
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Box>
  )
}
