interface Props {
  sheets: string[]
  selected: string | null
  onChange: (sheet: string) => void
}

export default function SheetTabs({ sheets, selected, onChange }: Props) {
  if (sheets.length === 0) return null

  return (
    <div style={{
      display: 'flex',
      overflowX: 'auto',
      borderTop: '1px solid var(--mantine-color-default-border)',
      background: 'var(--mantine-color-default)',
      flexShrink: 0,
    }}>
      {sheets.map(sheet => {
        const isActive = sheet === selected
        return (
          <button
            key={sheet}
            onClick={() => onChange(sheet)}
            style={{
              padding: '4px 16px',
              border: 'none',
              borderRight: '1px solid var(--mantine-color-default-border)',
              borderTop: isActive
                ? '2px solid var(--mantine-primary-color-filled)'
                : '2px solid transparent',
              background: isActive
                ? 'var(--mantine-color-body)'
                : 'transparent',
              color: 'var(--mantine-color-text)',
              fontWeight: isActive ? 600 : 400,
              fontSize: 12,
              whiteSpace: 'nowrap',
              cursor: 'pointer',
              outline: 'none',
              fontFamily: 'inherit',
            }}
          >
            {sheet}
          </button>
        )
      })}
    </div>
  )
}
