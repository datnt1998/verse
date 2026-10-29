// Segmented compact 28px của Trình đọc (Reader.dc.html).
import type { ReactNode } from 'react'
import { Segmented } from '@noverse/ui'

export type CompactOption = { id: string; label: ReactNode; mono?: boolean }

/** Mặc định = thanh Nguồn văn bản của Reader (.segc 28px, nền surface-1, viền line-1);
 *  khối Định dạng trong Xuất truyện cao 30px, nền surface-0, viền line-3, chữ đậm nhạt như nhau. */
export function CompactSegmented({ options, value, onChange, label, columns, height = 28, activeWeight = 500, surface = 'var(--color-surface-1)', border = 'var(--line-1)' }: {
  options: CompactOption[]
  value: string
  onChange: (id: string) => void
  label: string
  columns?: number
  height?: number
  activeWeight?: number
  surface?: string
  border?: string
}) {
  return (
    <Segmented label={label} size={height} radius={6} columns={columns}
      value={value} onChange={onChange}
      options={options.map((option) => ({
        id: option.id,
        label: option.mono ? <span className="mono">{option.label}</span> : option.label,
        style: { flex: columns ? 1 : '0 1 auto', padding: '0 10px', fontSize: 12, fontWeight: option.id === value ? activeWeight : 400 },
      }))}
      style={{
        display: columns ? 'grid' : 'flex',
        gridTemplateColumns: columns ? `repeat(${columns}, minmax(0, 1fr))` : undefined,
        gap: 2, borderRadius: 9, background: surface, border: `1px solid ${border}`,
      }} />
  )
}
