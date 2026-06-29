import { Box, Text } from '@mantine/core'

interface Props {
  status: string
}

export default function StatusBar({ status }: Props) {
  return (
    <Box
      bg="gray.1"
      px="sm"
      style={{
        height: 28,
        borderTop: '1px solid var(--mantine-color-gray-3)',
        display: 'flex',
        alignItems: 'center',
        flexShrink: 0,
      }}
    >
      <Text size="xs" c="dimmed" truncate>
        {status || 'Nincs Excel fájl megnyitva'}
      </Text>
    </Box>
  )
}
