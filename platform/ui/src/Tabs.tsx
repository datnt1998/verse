// Tabs (tablist) + Segmented (radiogroup) — nền surface-1, mục chọn surface-6.
// Segmented: khối nền trượt .kit-seg-thumb (kit.css › motion:controls) trượt 300ms
// cubic-bezier(.34,1.25,.5,1) giữa các nút; vị trí đo từ bố cục nút nên
// cả nhóm bằng nhau lẫn pill bề rộng tự do đều trượt đúng.
import { useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from './tokens.stylex'
import './kit.css'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const FG = color['--color-fg']
const FG3 = color['--color-fg-3']

const sx = stylex.create({
  bar: {
    flex: '1 1 auto', display: 'flex', justifyContent: 'center', gap: 2, padding: 3, borderRadius: 11,
    backgroundColor: color['--color-surface-1'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
  },
  panel: (columns: string) => ({
    display: 'grid', gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
    gap: 2, padding: 3, borderRadius: 10, backgroundColor: color['--color-surface-1'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
  }),
  barBtn: {
    flex: '1 1 0', height: 32, paddingInline: 12, borderWidth: 0, borderStyle: 'none', borderRadius: 8,
    fontSize: 13, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 7, whiteSpace: 'nowrap',
  },
  barOn: { backgroundColor: color['--color-surface-6'], fontWeight: 500, color: { default: FG, ':hover': FG } },
  barOff: { backgroundColor: 'transparent', fontWeight: 400, color: { default: FG3, ':hover': FG } },
  panelBtn: {
    height: 30, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6,
    borderRadius: 7, borderWidth: 0, borderStyle: 'none', fontSize: 13,
    // Nhãn tab luôn một dòng và co được: nhãn dài cắt “…” thay vì đẩy tablist rộng hơn panel.
    minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden',
  },
  label: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  panelOn: { backgroundColor: color['--color-surface-6'], color: FG, fontWeight: 500 },
  panelOff: { backgroundColor: 'transparent', color: FG3, fontWeight: 400 },
  iconWrap: { display: 'inline-flex' },
  iconOn: { color: color['--color-accent'] },
  segmentRoot: { position: 'relative', display: 'flex', gap: 3, padding: 3, borderRadius: 10, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'] },
  segmentGrid: (columns: number) => ({ display: 'grid', gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }),
  segmentHintRoot: { gap: 2, backgroundColor: color['--color-surface-1'], borderColor: line['--line-1'] },
  segmentButton: (radius: number) => ({ position: 'relative', zIndex: 1, borderWidth: 0, borderStyle: 'none', borderRadius: radius, backgroundColor: 'transparent', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6 }),
  segmentSimpleButton: { fontSize: 12.5, whiteSpace: 'nowrap' },
  segmentFill: { flex: 1, minWidth: 0 },
  segmentContent: { flex: '0 0 auto', paddingInline: 12 },
  segmentHintButton: { minWidth: 0, minHeight: 40, padding: '6px 8px' },
  segmentHeight: (height: number) => ({ height }),
  segmentOn: { color: color['--color-fg'] },
  segmentOff: { color: color['--color-fg-3'] },
})

export type TabItem = { id: string; label: ReactNode; icon?: ReactNode; dot?: ReactNode; disabled?: boolean }

export function Tabs({ items, value, onChange, label, style, columns, variant = 'panel' }: {
  items: TabItem[]; value: string; onChange: (id: string) => void
  label: string; style?: CSSProperties; columns?: number
  /** 'panel' = tablist lưới 30 px của Main/Reader; 'bar' = thanh nhóm của Kit (32 px, bo 11, icon tím khi chọn). */
  variant?: 'panel' | 'bar'
}) {
  const bar = variant === 'bar'
  const group = stylex.props(bar ? sx.bar : sx.panel(String(columns ?? items.length)))
  return (
    <div role="tablist" aria-label={label} onKeyDown={(event) => {
      if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const enabled = items.filter((item) => !item.disabled)
      const index = enabled.findIndex((item) => item.id === value)
      const next = enabled[event.key === 'Home' ? 0 : event.key === 'End' ? enabled.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : enabled.length - 1)) % enabled.length]
      if (next) { onChange(next.id); event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')[enabled.indexOf(next)]?.focus() }
    }} className={group.className} style={{ ...group.style, ...style }}>
      {items.map((item) => {
        const on = item.id === value
        const btn = stylex.props(bar ? [sx.barBtn, on ? sx.barOn : sx.barOff] : [sx.panelBtn, on ? sx.panelOn : sx.panelOff])
        return (
          <button key={item.id} type="button" role="tab" aria-selected={on} tabIndex={on ? 0 : -1} disabled={item.disabled}
            className={btn.className}
            onClick={() => onChange(item.id)} style={btn.style}>
            {bar && item.icon ? <span {...stylex.props(sx.iconWrap, on && sx.iconOn)}>{item.icon}</span> : item.icon}
            <span {...stylex.props(sx.label)}>{item.label}</span>{item.dot}
          </button>
        )
      })}
    </div>
  )
}

export type Segment = { id: string; label: ReactNode; icon?: ReactNode; hint?: ReactNode; disabled?: boolean; style?: CSSProperties }

type SegRect = { x: number; y: number; w: number; h: number }

export function Segmented({ options, value, onChange, label, style, xstyle, buttonStyle, columns, size = 'md', radius, thumbStyle, tabs, fit = 'equal' }: {
  options: Segment[]; value: string; onChange: (id: string) => void
  label: string; style?: CSSProperties; columns?: number
  xstyle?: stylex.StyleXStyles
  buttonStyle?: stylex.StyleXStyles
  /**
   * 'equal' (mặc định) chia đều bề rộng nhóm; 'content' để mỗi mục rộng đúng nhãn
   * (nhóm lọc ngắn cỡ nút "Sắp xếp" của artboard — không cắt bằng dấu ba chấm).
   * Chỉ áp dụng cho nhóm một dòng; mục có `hint` vẫn là lưới chia đều.
   * Đệm ngang của mục khi đó là 12 px — đặt lại qua `options[].style.paddingInline`.
   */
  fit?: 'equal' | 'content'
  /**
   * Hai vai trò của cùng một control (22-brief-2510.md › Câu 14): mặc định `radiogroup` cho lọc dữ liệu;
   * `tabs` cho chuyển màn trong cùng trang → `tablist`/`tab` + `aria-selected`, `id` = `<id>-<mục>`,
   * `aria-controls` = `panel` để `tabpanel` trỏ ngược lại bằng `aria-labelledby`.
   */
  tabs?: { id: string; panel: string }
  /** Cao nút: 'sm' 28 (Reader/Settings .segc) · 'md' 30 (Kit) · 'lg' 36 (MNotify) · số tuỳ ý. */
  size?: 'sm' | 'md' | 'lg' | number
  /** Bo góc nút và khối trượt (mặc định 7; Reader 6, mobile 10–11). */
  radius?: number
  thumbStyle?: CSSProperties
}) {
  // Không có dòng phụ → "Nhóm chọn" của Kit (nền #0F0F11, mục chọn #2A2A2E); có dòng phụ → thẻ 2 dòng.
  const simple = !options.some((option) => option.hint)
  /** Nhóm một dòng rộng theo nhãn: mục không co, không chia đều. */
  const contentFit = simple && fit === 'content'
  const btnH = typeof size === 'number' ? size : { sm: 28, md: 30, lg: 36 }[size]
  const btnR = radius ?? 7
  const itemRole = tabs ? 'tab' : 'radio'
  const groupRef = useRef<HTMLDivElement>(null)
  const [thumb, setThumb] = useState<SegRect | null>(null)
  useLayoutEffect(() => {
    const group = groupRef.current
    if (!group) return
    const measure = () => {
      const index = options.findIndex((option) => option.id === value)
      const btn = index >= 0 ? group.querySelectorAll<HTMLElement>(`[role=${itemRole}]`)[index] : undefined
      const bounds = btn?.getBoundingClientRect()
      const container = group.getBoundingClientRect()
      const rect = bounds ? {
        x: bounds.left - container.left - group.clientLeft,
        y: bounds.top - container.top - group.clientTop,
        w: bounds.width,
        h: bounds.height,
      } : null
      setThumb((old) => old?.x === rect?.x && old?.y === rect?.y && old?.w === rect?.w && old?.h === rect?.h ? old : rect)
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(group)
    group.querySelectorAll(`[role=${itemRole}]`).forEach((button) => observer.observe(button))
    return () => observer.disconnect()
  }, [value, options, columns, itemRole])
  const root = stylex.props(sx.segmentRoot, !simple && sx.segmentHintRoot, !simple ? sx.segmentGrid(columns ?? options.length) : columns && !contentFit ? sx.segmentGrid(columns) : null, xstyle)
  return (
    <div ref={groupRef} role={tabs ? 'tablist' : 'radiogroup'} aria-label={label} onKeyDown={(event) => {
      if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const enabled = options.filter((option) => !option.disabled)
      const index = enabled.findIndex((option) => option.id === value)
      const next = enabled[event.key === 'Home' ? 0 : event.key === 'End' ? enabled.length - 1 : (index + (event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : enabled.length - 1)) % enabled.length]
      if (next) { onChange(next.id); event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')[enabled.indexOf(next)]?.focus() }
    }} className={root.className} style={{ ...root.style, ...style }}>
      <span aria-hidden="true" className="kit-seg-thumb" style={{
        width: thumb?.w ?? 0, height: thumb?.h ?? 0, borderRadius: btnR, visibility: thumb ? 'visible' : 'hidden',
        background: simple ? 'var(--color-surface-7)' : 'var(--color-surface-6)',
        boxShadow: 'var(--kit-shadow-segment)',
        ...thumbStyle,
        transform: `translate(${thumb?.x ?? 0}px, ${thumb?.y ?? 0}px)`,
      }} />
      {options.map((option) => {
        const on = option.id === value
        const button = stylex.props(sx.segmentButton(btnR), simple ? [sx.segmentSimpleButton, contentFit ? sx.segmentContent : sx.segmentFill] : sx.segmentHintButton, (simple || typeof size === 'number' || size !== 'md') && sx.segmentHeight(btnH), on ? sx.segmentOn : sx.segmentOff, buttonStyle)
        return (
          <button key={option.id} type="button" role={itemRole}
            {...(tabs
              ? { id: `${tabs.id}-${option.id}`, 'aria-selected': on, 'aria-controls': tabs.panel }
              : { 'aria-checked': on })}
            tabIndex={on ? 0 : -1} disabled={option.disabled}
            className={button.className} onClick={() => onChange(option.id)} style={{ ...button.style, ...option.style }}>
            {option.icon && <span style={{ display: 'inline-flex', flex: '0 0 auto' }}>{option.icon}</span>}
            {simple
              ? <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 0, maxWidth: '100%' }}>{option.label}</span>
              : <span style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2, minWidth: 0, lineHeight: 1.2 }}>
                  <span style={{ fontSize: 13, fontWeight: on ? 500 : 400, display: 'flex', alignItems: 'center', gap: 5 }}>{option.label}</span>
                  {option.hint && <span style={{ fontSize: 11, color: 'var(--color-fg-4)' }}>{option.hint}</span>}
                </span>}
          </button>
        )
      })}
    </div>
  )
}
