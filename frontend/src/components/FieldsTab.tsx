import { Table, TextInput, Select, Text, Badge, Box } from '@mantine/core'
import * as main from '../../bindings/kutyaguru'

interface Props {
  fields: main.Field[]
  onFieldChange: (fieldName: string, value: string) => void
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
        <Text c="dimmed" size="sm">Nyiss meg egy Excel fájlt a mezők megjelenítéséhez.</Text>
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
                {(field.type === 'MAPPING' || field.type === 'CONST') ? (
                  <Text size="sm" c="dimmed">{field.value || field.mapping}</Text>
                ) : field.options && field.options.length > 0 ? (
                  <Select
                    data={field.options}
                    value={field.value}
                    onChange={(v) => v && onFieldChange(field.name, v)}
                    size="xs"
                    w={180}
                  />
                ) : (
                  <TextInput
                    value={field.value}
                    onChange={(e) => onFieldChange(field.name, e.currentTarget.value)}
                    size="xs"
                    w={180}
                    placeholder={field.type === 'DATE' ? 'éééé.hh.nn' : ''}
                  />
                )}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Box>
  )
}
