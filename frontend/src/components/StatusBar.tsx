import { Badge, Box, Group, Text } from '@mantine/core'

interface Props {
  status: string
  errorCount?: number
  warningCount?: number
  onCounterClick?: () => void
}

export default function StatusBar({ status, errorCount = 0, warningCount = 0, onCounterClick }: Props) {
  const badgeStyle = onCounterClick ? { cursor: 'pointer' } : undefined
  return (
    <Box
      bg="gray.1"
      px="sm"
      style={{
        height: 28,
        borderTop: '1px solid var(--mantine-color-gray-3)',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        flexShrink: 0,
      }}
    >
      <Group gap={6} wrap="nowrap">
        {errorCount > 0 && (
          <Badge color="red" variant="filled" size="sm" style={badgeStyle} onClick={onCounterClick}>{errorCount} hiba</Badge>
        )}
        {warningCount > 0 && (
          <Badge color="orange" variant="light" size="sm" style={badgeStyle} onClick={onCounterClick}>{warningCount} figyelmeztetés</Badge>
        )}
      </Group>
      <Text size="xs" c="dimmed" truncate>
        {status || 'Nincs Excel fájl megnyitva'}
      </Text>
    </Box>
  )
}
