// Tabs (tablist) + Segmented (radiogroup) — nền surface-1, mục chọn surface-6.
// Segmented: khối nền trượt .kit-seg-thumb (kit.css › motion:controls) trượt 320ms
// cubic-bezier(.3,.7,.2,1) — cả vị trí lẫn bề rộng (04-components › Nhóm chọn);
// vị trí đo từ bố cục nút nên cả nhóm chia đều lẫn pill bề rộng tự do đều trượt đúng.
import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
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
  /* Tab gạch chân `.vv-tabs`/`.vv-tab` của kit (Library.dc.html): chữ và gạch đổi
     240 ms, tôn trọng giảm chuyển động. Nhóm cuộn ngang khi hẹp hơn nội dung. */
  underlineRoot: {
    display: 'flex', gap: 4, minWidth: 0, overflowX: 'auto', scrollbarWidth: 'none',
    borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: line['--line-2'],
  },
  underlineBtn: {
    position: 'relative', display: 'inline-flex', alignItems: 'center', gap: 6, flex: '0 0 auto',
    height: 40, paddingInline: 12, borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent',
    borderBottomWidth: 2, borderBottomStyle: 'solid', borderBottomColor: 'transparent',
    fontSize: 14, whiteSpace: 'nowrap', cursor: 'pointer',
    transition: { default: 'color .24s cubic-bezier(.2,.9,.3,1), border-color .24s cubic-bezier(.2,.9,.3,1)', [NO_MOTION]: 'none' },
  },
  underlineOn: { color: FG, borderBottomColor: color['--color-accent'], fontWeight: 500 },
  underlineOff: { color: { default: FG3, ':hover': FG }, fontWeight: 400 },
  underlineDisabled: { opacity: .42, cursor: 'not-allowed' },
  iconWrap: { display: 'inline-flex' },
  iconOn: { color: color['--color-accent'] },
  segmentRoot: { position: 'relative', display: 'flex', gap: 3, padding: 3, borderRadius: 10, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'] },
  segmentGrid: (columns: number) => ({ display: 'grid', gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))` }),
  segmentHintRoot: { gap: 2, backgroundColor: color['--color-surface-1'], borderColor: line['--line-1'] },
  segmentButton: (radius: number) => ({ position: 'relative', zIndex: 1, borderWidth: 0, borderStyle: 'none', borderRadius: radius, backgroundColor: 'transparent', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6 }),
  segmentSimpleButton: { fontSize: 12.5, whiteSpace: 'nowrap' },
  // 'equal': chia đều cứng (Noverse, lưới tab). 'fill': chia đều nhưng nhãn
  // dài vẫn giữ đủ chữ (min-width: max-content) — `.vv-seg--fill`.
  segmentFill: { flex: 1, minWidth: 0 },
  segmentFillContent: { flex: '1 1 0', minWidth: 'max-content' },
  // 'content': mục rộng đúng nhãn (`.vv-seg`, đợt 34) — không cắt “…”.
  segmentContent: { flex: '0 0 auto', paddingInline: 12 },
  segmentHintButton: { minWidth: 0, minHeight: 40, padding: '6px 8px' },
  segmentHeight: (height: number) => ({ height }),
  /* Cỡ điện thoại (`.vv-seg--m` artboard): nhóm 48px, mục 40px, chữ 14, góc 12/9. */
  segmentXl: { display: 'grid', gridAutoFlow: 'column', gridAutoColumns: 'minmax(max-content, 1fr)', borderRadius: 12 },
  segmentXlButton: { fontSize: 14 },
  segmentOn: { color: color['--color-fg'] },
  segmentOff: { color: color['--color-fg-3'] },
})

export type TabItem = { id: string; label: ReactNode; icon?: ReactNode; dot?: ReactNode; disabled?: boolean }

export function Tabs({ items, value, onChange, label, style, columns, variant = 'panel', tabs }: {
  items: TabItem[]; value: string; onChange: (id: string) => void
  label: string; style?: CSSProperties; columns?: number
  /** 'panel' = tablist lưới 30 px của Main/Reader; 'bar' = thanh nhóm của Kit (32 px, bo 11, icon tím khi chọn); 'underline' = `.vv-tabs` gạch chân 40 px (Library, Settings). */
  variant?: 'panel' | 'bar' | 'underline'
  /** id tab + id panel để `aria-controls`/`aria-labelledby` trỏ đúng cặp (dùng với 'underline'). */
  tabs?: { id: string; panel: string }
}) {
  const bar = variant === 'bar'
  const underline = variant === 'underline'
  const group = stylex.props(bar ? sx.bar : underline ? sx.underlineRoot : sx.panel(String(columns ?? items.length)))
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
        const btn = stylex.props(
          underline ? [sx.underlineBtn, on ? sx.underlineOn : sx.underlineOff, item.disabled && sx.underlineDisabled]
            : bar ? [sx.barBtn, on ? sx.barOn : sx.barOff] : [sx.panelBtn, on ? sx.panelOn : sx.panelOff],
        )
        return (
          <button key={item.id} type="button" role="tab" aria-selected={on} tabIndex={on ? 0 : -1} disabled={item.disabled}
            id={tabs ? `${tabs.id}-${item.id}` : undefined}
            aria-controls={tabs?.panel}
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
   * 'equal' (mặc định của kit) chia đều cứng bề rộng nhóm — Noverse giữ cách này.
   * 'content' để mỗi mục rộng đúng nhãn, không bao giờ cắt “…” (Viderse mặc
   * định, 04-components › SegmentedTabs đợt 34); nhóm hẹp hơn nội dung thì cuộn
   * ngang. 'fill' (`.vv-seg--fill`) chia đều nhưng nhãn dài vẫn giữ đủ chữ
   * (`min-width: max-content`).
   * Chỉ áp dụng cho nhóm một dòng; mục có `hint` vẫn là lưới chia đều.
   * Đệm ngang của mục khi đó là 12 px — đặt lại qua `options[].style.paddingInline`.
   */
  fit?: 'equal' | 'content' | 'fill'
  /**
   * Hai vai trò của cùng một control (22-brief-2510.md › Câu 14): mặc định `radiogroup` cho lọc dữ liệu;
   * `tabs` cho chuyển màn trong cùng trang → `tablist`/`tab` + `aria-selected`, `id` = `<id>-<mục>`,
   * `aria-controls` = `panel` để `tabpanel` trỏ ngược lại bằng `aria-labelledby`.
   */
  tabs?: { id: string; panel: string }
  /** Cao nút: 'sm' 28 (Reader/Settings .segc) · 'md' 30 (Kit) · 'lg' 36 (MNotify) · 'xl' 40 trong nhóm 48 (điện thoại `.vv-seg--m`) · số tuỳ ý. */
  size?: 'sm' | 'md' | 'lg' | 'xl' | number
  /** Bo góc nút và khối trượt (mặc định 7; Reader 6, mobile 10–11). */
  radius?: number
  thumbStyle?: CSSProperties
}) {
  // Không có dòng phụ → "Nhóm chọn" của Kit (nền #0F0F11, mục chọn #2A2A2E); có dòng phụ → thẻ 2 dòng.
  const simple = !options.some((option) => option.hint)
  /** Nhóm một dòng rộng theo nhãn: mục không co, không chia đều. */
  const contentFit = simple && fit === 'content'
  /** Chia đều nhưng nhãn dài vẫn giữ đủ chữ (`.vv-seg--fill`). */
  const fillFit = simple && fit === 'fill'
  const xl = size === 'xl'
  const btnH = typeof size === 'number' ? size : { sm: 28, md: 30, lg: 36, xl: 40 }[size]
  const btnR = radius ?? (xl ? 9 : 7)
  const itemRole = tabs ? 'tab' : 'radio'
  const tabStop = options.find(option => option.id === value && !option.disabled)?.id ?? options.find(option => !option.disabled)?.id
  const groupRef = useRef<HTMLDivElement>(null)
  const [thumb, setThumb] = useState<SegRect | null>(null)
  // Lần đo ĐẦU đặt viên vào đúng mục rồi mới bật transition, nếu không viên "mọc"
  // từ bề rộng 0 trong 320 ms ngay khi mở màn.
  const [anim, setAnim] = useState(false)
  useEffect(() => {
    if (!thumb) return
    const frame = typeof requestAnimationFrame === 'function' ? requestAnimationFrame(() => setAnim(true)) : null
    if (frame === null) { setAnim(true); return }
    return () => cancelAnimationFrame(frame)
  }, [thumb])
  useLayoutEffect(() => {
    const group = groupRef.current
    if (!group) return
    // Viên trượt đo theo hộp THẬT của mục (không phải 100%/n) và cộng
    // `scrollLeft` — nhóm hẹp hơn nội dung thì cuộn ngang, viên trượt đi theo.
    const measure = () => {
      const index = options.findIndex((option) => option.id === value)
      const btn = index >= 0 ? group.querySelectorAll<HTMLElement>(`[role=${itemRole}]`)[index] : undefined
      const bounds = btn?.getBoundingClientRect()
      const container = group.getBoundingClientRect()
      const rect = bounds ? {
        x: bounds.left - container.left - group.clientLeft + group.scrollLeft,
        y: bounds.top - container.top - group.clientTop,
        w: bounds.width,
        h: bounds.height,
      } : null
      setThumb((old) => old?.x === rect?.x && old?.y === rect?.y && old?.w === rect?.w && old?.h === rect?.h ? old : rect)
    }
    // Mục đang chọn tự cuộn vào tầm nhìn khi nhóm phải cuộn ngang (#80).
    const reveal = () => {
      if (group.scrollWidth <= group.clientWidth) return
      const index = options.findIndex((option) => option.id === value)
      const btn = index >= 0 ? group.querySelectorAll<HTMLElement>(`[role=${itemRole}]`)[index] : undefined
      if (!btn) return
      const bounds = btn.getBoundingClientRect()
      const container = group.getBoundingClientRect()
      if (bounds.left < container.left) group.scrollLeft -= container.left - bounds.left
      else if (bounds.right > container.right) group.scrollLeft += bounds.right - container.right
    }
    reveal()
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(group)
    group.querySelectorAll(`[role=${itemRole}]`).forEach((button) => observer.observe(button))
    group.addEventListener('scroll', measure, { passive: true })
    // Font tải xong thì bề rộng nhãn đổi — đo lại viên trượt.
    if (document.fonts) void document.fonts.ready.then(measure)
    return () => {
      observer.disconnect()
      group.removeEventListener('scroll', measure)
    }
  }, [value, options, columns, itemRole])
  const root = stylex.props(sx.segmentRoot, xl && sx.segmentXl, !simple && sx.segmentHintRoot, !simple ? sx.segmentGrid(columns ?? options.length) : columns && !contentFit && !fillFit ? sx.segmentGrid(columns) : null, xstyle)
  return (
    <div ref={groupRef} role={tabs ? 'tablist' : 'radiogroup'} aria-label={label} onKeyDown={(event) => {
      if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const enabled = options.filter((option) => !option.disabled)
      const selectedIndex = enabled.findIndex((option) => option.id === value)
      const focusedIndex = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')).indexOf(document.activeElement as HTMLButtonElement)
      const index = selectedIndex >= 0 ? selectedIndex : focusedIndex
      const next = enabled[event.key === 'Home' ? 0 : event.key === 'End' ? enabled.length - 1 : (index + (event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : enabled.length - 1)) % enabled.length]
      if (next) { onChange(next.id); event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')[enabled.indexOf(next)]?.focus() }
    }} className={`kit-seg ${root.className}`} style={{ ...root.style, ...style }}>
      <span aria-hidden="true" className="kit-seg-thumb" style={{
        width: thumb?.w ?? 0, height: thumb?.h ?? 0, borderRadius: btnR, visibility: thumb ? 'visible' : 'hidden',
        background: simple ? 'var(--color-surface-7)' : 'var(--color-surface-6)',
        boxShadow: 'var(--kit-shadow-segment)',
        transition: anim ? undefined : 'none',
        ...thumbStyle,
        transform: `translate(${thumb?.x ?? 0}px, ${thumb?.y ?? 0}px)`,
      }} />
      {options.map((option) => {
        const on = option.id === value
        const button = stylex.props(sx.segmentButton(btnR), simple ? [sx.segmentSimpleButton, xl && sx.segmentXlButton, contentFit ? sx.segmentContent : fillFit ? sx.segmentFillContent : sx.segmentFill] : sx.segmentHintButton, (simple || typeof size === 'number' || size !== 'md') && sx.segmentHeight(btnH), on ? sx.segmentOn : sx.segmentOff, buttonStyle)
        return (
          <button key={option.id} type="button" role={itemRole}
            {...(tabs
              ? { id: `${tabs.id}-${option.id}`, 'aria-selected': on, 'aria-controls': tabs.panel }
              : { 'aria-checked': on })}
            tabIndex={option.id === tabStop ? 0 : -1} disabled={option.disabled}
            className={button.className} onClick={() => onChange(option.id)} style={{ ...button.style, ...option.style }}>
            {option.icon && <span style={{ display: 'inline-flex', flex: '0 0 auto' }}>{option.icon}</span>}
            {simple
              ? <span style={{ whiteSpace: 'nowrap', flex: contentFit ? '0 0 auto' : undefined }}>{option.label}</span>
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
