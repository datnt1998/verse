// Adapter mỏng cho chọn/menu của Viderse: desktop dùng đúng primitive nổi của
// kit, điện thoại (<768px) dùng Sheet/ActionSheet. Một nguồn options/actions,
// cùng nhãn/mặc định/quyền/disabled; lớp phủ mobile giữ 44px chạm, scrim, Esc và
// khoá cuộn của `Sheet`. Không dựng listbox/popover thứ hai.
//
// Callback của một lựa chọn chỉ chạy trong `Sheet.onExited`: sau khi lớp đã gỡ,
// khoá cuộn/focus đã trả về trigger. Nhờ vậy action mở `confirm()`/Dialog không
// chồng lên sheet đang fade, focus được trả về trigger trước khi lớp mới mở và
// callback vẫn chỉ chạy một lần. Đổi breakpoint khi sheet đang mở giữ nguyên
// component cho hết hoạt cảnh đóng; unmount do route/account đổi thì không chạy
// action đã chọn dở (stale mutation).
import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionMenu, ChipSelect, Icon, KitSelect, Sheet, getKitLabels, PHONE_MAX_VIDERSE, phoneQuery,
  type ActionMenuItem, type ChipSelectProps, type KitSelectProps, type Option,
} from '@noverse/ui'
import { color, kit, line } from '@noverse/ui/tokens.stylex'
import { useMediaQuery } from '../lib/useMediaQuery'
import { controlMotion, corners, space, typography } from '../styles/viderse.stylex'

const MOBILE = phoneQuery(PHONE_MAX_VIDERSE)
const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

const sx = stylex.create({
  col: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'], minWidth: 0 },
  chipCol: { alignItems: 'flex-start', width: 'auto' },
  label: { fontSize: typography['--vv-fs-small'] },
  trigger: {
    display: 'flex', alignItems: 'center', gap: space['--vv-s-8'], width: '100%', minHeight: space['--vv-s-44'],
    paddingTop: 0, paddingBottom: 0, paddingLeft: space['--vv-s-12'], paddingRight: space['--vv-s-10'],
    borderRadius: corners['--vv-r-12'], backgroundColor: color['--color-surface-0'],
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'],
    color: color['--color-fg'], fontSize: typography['--vv-fs-ui'], textAlign: 'left', cursor: 'pointer',
    ':focus-visible': { borderColor: kit['--kit-acc-55'], boxShadow: kit['--kit-ring-acc'], outline: 'none' },
  },
  triggerOpen: { borderColor: kit['--kit-acc-55'], boxShadow: kit['--kit-ring-acc'] },
  triggerInvalid: { borderColor: kit['--kit-errb-55'], boxShadow: kit['--kit-ring-err'] },
  triggerDisabled: { opacity: .45, cursor: 'not-allowed' },
  chipTrigger: {
    width: 'auto', minHeight: space['--vv-s-44'], gap: space['--vv-s-6'], paddingLeft: space['--vv-s-12'], paddingRight: space['--vv-s-10'],
    borderRadius: corners['--vv-r-12'], fontSize: typography['--vv-fs-meta'], color: color['--color-fg-2'],
    backgroundColor: color['--color-surface-1'], borderColor: line['--line-4'],
  },
  chipCustom: { borderColor: kit['--kit-warn-30'] },
  iconLead: { display: 'flex', flexShrink: 0, color: color['--color-fg-3'] },
  prefix: { flex: '0 0 auto', whiteSpace: 'nowrap' },
  value: { flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  valueChip: { fontWeight: 500, color: color['--color-fg'] },
  placeholder: { flex: '1 1 auto', minWidth: 0, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  valueCol: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: space['--vv-s-1'] },
  description: { fontSize: typography['--vv-fs-small'], lineHeight: 1.35, color: kit['--kit-fg-icon'], whiteSpace: 'normal', overflowWrap: 'anywhere' },
  chev: { flexShrink: 0, color: color['--color-fg-4'], transitionProperty: 'transform', transitionDuration: { default: controlMotion.selectChevron, [NO_MOTION]: '0s' } },
  chevOpen: { transform: 'rotate(180deg)' },
  error: { fontSize: typography['--vv-fs-small'], color: kit['--kit-err-soft'] },
  sheetBody: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-2'], paddingInline: space['--vv-s-8'], paddingBottom: space['--vv-s-14'] },
  groupLabel: { display: 'block', fontSize: typography['--vv-fs-group'], paddingTop: space['--vv-s-8'], paddingInline: space['--vv-s-10'], paddingBottom: space['--vv-s-4'], color: color['--color-fg-4'] },
  row: {
    display: 'flex', alignItems: 'center', gap: space['--vv-s-10'], width: '100%', minHeight: space['--vv-s-44'],
    paddingTop: space['--vv-s-8'], paddingBottom: space['--vv-s-8'], paddingInline: space['--vv-s-10'], borderWidth: 0, borderStyle: 'none',
    borderRadius: corners['--vv-r-9'], backgroundColor: 'transparent', color: color['--color-fg'],
    fontSize: typography['--vv-fs-ui'], textAlign: 'left', cursor: 'pointer',
    ':active': { backgroundColor: line['--line-2'] },
    ':focus-visible': { outlineWidth: space['--vv-s-2'], outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: `calc(-1 * ${space['--vv-s-2']})` },
  },
  rowOn: { backgroundColor: kit['--kit-wash-75'] },
  rowDisabled: { color: kit['--kit-fg-sub'], cursor: 'not-allowed', opacity: .55, ':active': { backgroundColor: 'transparent' } },
  rowDanger: { color: kit['--kit-err-ghost-ink'] },
  rowText: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: space['--vv-s-1'] },
  rowLabel: { fontSize: typography['--vv-fs-ui'], lineHeight: 1.35, overflowWrap: 'anywhere' },
  rowHint: { fontSize: typography['--vv-fs-small'], lineHeight: 1.4, color: kit['--kit-fg-sub'], whiteSpace: 'normal', overflowWrap: 'anywhere' },
  meta: { fontSize: typography['--vv-fs-group'], color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  tick: { flex: '0 0 auto', color: color['--color-accent'] },
  separator: { height: space['--vv-s-1'], marginTop: space['--vv-s-6'], marginBottom: space['--vv-s-6'], marginInline: space['--vv-s-4'], backgroundColor: line['--line-2'] },
  sub: { paddingLeft: space['--vv-s-16'] },
  footer: { borderTopWidth: space['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-2'], marginTop: space['--vv-s-6'], paddingTop: space['--vv-s-6'] },
  menuTrigger: {
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
    minWidth: space['--vv-s-44'], minHeight: space['--vv-s-44'], padding: 0, borderRadius: corners['--vv-r-10'], borderWidth: 0, borderStyle: 'none',
    backgroundColor: 'transparent', color: color['--color-fg-2'],
    ':active': { backgroundColor: line['--line-2'] },
    ':focus-visible': { outlineWidth: space['--vv-s-2'], outlineStyle: 'solid', outlineColor: line['--line-focus'], outlineOffset: `calc(-1 * ${space['--vv-s-2']})` },
  },
})

type SolidSelectProps = KitSelectProps

type MobileSelectProps = SolidSelectProps & {
  mobile: boolean
  onBusy: (busy: boolean) => void
  chip?: boolean
  custom?: boolean
  size?: ChipSelectProps['size']
}

function MobileSelect({ mobile, onBusy, chip = false, custom, size = 'sm', label, ...props }: MobileSelectProps) {
  const {
    options, value, onChange, id: suppliedId, name, 'aria-label': ariaLabel, placeholder, disabled, error,
    inlineDescription, optionDescriptions, footer, defaultOpen = false, autoFocus, icon, variant, xstyle, triggerStyle,
    chevronSize, prefix, valueStyle, style,
  } = props
  const uid = useId()
  const id = suppliedId ?? uid
  const L = getKitLabels().selects
  const ph = placeholder ?? L.choose
  const selected = options.find((option) => option.value === value)
  const [open, setOpen] = useState(defaultOpen && !disabled)
  const [pending, setPending] = useState<Option | null>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const list = useRef<HTMLDivElement>(null)
  const busyRef = useRef(onBusy)
  busyRef.current = onBusy
  useEffect(() => { if (open || pending) busyRef.current(true) }, [open, pending])
  useEffect(() => { if (!mobile && open) setOpen(false) }, [mobile, open])
  useEffect(() => { if (autoFocus && !defaultOpen) trigger.current?.focus({ preventScroll: true }) }, [autoFocus, defaultOpen])
  // Sheet tự focus khung; đưa focus vào mục đang chọn/mục đầu để Arrow/Tab dùng
  // được ngay trên bàn phím, vẫn trong trap focus của Sheet.
  useEffect(() => {
    if (!open || !mobile) return
    const frame = requestAnimationFrame(() => {
      const rows = list.current?.querySelectorAll<HTMLButtonElement>('button[role="option"]:not(:disabled)')
      if (!rows?.length) return
      const current = [...rows].findIndex((row) => row.getAttribute('aria-selected') === 'true')
      ;(rows[current >= 0 ? current : 0]).focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(frame)
  }, [open, mobile])
  const listKeys = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const rows = [...(list.current?.querySelectorAll<HTMLButtonElement>('button[role="option"]:not(:disabled)') ?? [])]
    if (!rows.length) return
    event.preventDefault()
    const at = rows.indexOf(document.activeElement as HTMLButtonElement)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? rows.length - 1
      : at < 0 ? (event.key === 'ArrowUp' ? rows.length - 1 : 0)
        : (at + (event.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length
    rows[next].focus()
  }

  const close = () => { if (open) setOpen(false) }
  const choose = (option: Option) => {
    if (!open || disabled || option.disabled || pending) return
    setPending(option)
    setOpen(false)
  }
  // `onExited` chạy sau khi Sheet đã gỡ focus/scroll lock; focus đã về trigger.
  const exited = () => {
    const option = pending
    setPending(null)
    if (option) onChange(option.value)
    busyRef.current(false)
  }
  const sheetTitle = label ?? ariaLabel ?? L.options
  const triggerProps = stylex.props(
    sx.trigger,
    chip && sx.chipTrigger,
    chip && custom && sx.chipCustom,
    disabled ? sx.triggerDisabled : error ? sx.triggerInvalid : open ? sx.triggerOpen : undefined,
    triggerStyle,
  )
  const outer = stylex.props(sx.col, chip && sx.chipCol, xstyle)
  const chev = stylex.props(sx.chev, open && sx.chevOpen)
  const shown = selected
  const showDescription = !!shown && (inlineDescription || variant === 'project-role')
  return <div className={[outer.className, props.className].filter(Boolean).join(' ')} style={{ ...outer.style, ...style }}>
    {!chip && label && <label className={`lbl ${stylex.props(sx.label).className}`} htmlFor={id}>{label}</label>}
    {name && <input type="hidden" name={name} value={value} disabled={disabled} />}
    <button
      ref={trigger}
      id={id}
      type="button"
      className={triggerProps.className}
      style={{ ...triggerProps.style, ...(props.height ? { height: Math.max(44, props.height) } : {}), ...(size === 'm' ? { minHeight: 44 } : {}) }}
      aria-label={ariaLabel ?? label ?? L.chooseItem}
      role="combobox"
      aria-haspopup="dialog"
      aria-expanded={open}
      aria-invalid={!!error || undefined}
      aria-describedby={error ? `${id}-error` : undefined}
      disabled={disabled}
      title={shown ? [shown.label, showDescription && shown.description].filter(Boolean).join(' · ') : ph}
      onClick={() => { if (!open) busyRef.current(true); setOpen((next) => !next) }}
    >
      {icon && <span aria-hidden="true" className={stylex.props(sx.iconLead).className}>{icon}</span>}
      {prefix != null && <span className={stylex.props(sx.prefix).className}>{prefix}</span>}
      {shown
        ? <span className={stylex.props(sx.valueCol).className}>
          <span className={stylex.props(chip ? sx.valueChip : sx.value, valueStyle).className}>{shown.label}</span>
          {showDescription && shown.description && <span className={stylex.props(sx.description).className}>{shown.description}</span>}
        </span>
        : <span className={stylex.props(sx.placeholder).className}>{ph}</span>}
      <Icon name="arrow-down-2" size={chevronSize ?? 15} className={chev.className} style={chev.style} />
    </button>
    {error && <span id={`${id}-error`} role="alert" className={stylex.props(sx.error).className}>{error}</span>}
    <Sheet open={mobile && open} onClose={close} onExited={exited} title={sheetTitle} size="md" height="auto" homeIndicator>
      <div ref={list} role="listbox" aria-label={ariaLabel ?? label ?? L.options} className={stylex.props(sx.sheetBody).className} onKeyDown={listKeys}>
        {options.length === 0 && <span className={stylex.props(sx.rowHint).className}>{L.empty}</span>}
        {options.map((option, index) => {
          const on = option.value === value
          const row = stylex.props(sx.row, on && sx.rowOn, option.disabled && sx.rowDisabled)
          const showMeta = option.meta != null || option.code != null
          const description = option.description && (inlineDescription || optionDescriptions || variant === 'project-role') ? option.description : null
          return <div key={option.value}>
            {option.group && (index === 0 || options[index - 1].group !== option.group) && <span className={stylex.props(sx.groupLabel).className}>{option.group}</span>}
            <button
              type="button"
              role="option"
              aria-selected={on}
              disabled={option.disabled}
              className={row.className}
              style={row.style}
              onClick={() => choose(option)}
            >
              {option.icon && <span aria-hidden="true" className={stylex.props(sx.iconLead).className}>{option.icon}</span>}
              <span className={stylex.props(sx.rowText).className}>
                <span className={stylex.props(sx.rowLabel).className}>{option.label}</span>
                {description && <span className={stylex.props(sx.rowHint).className}>{description}</span>}
              </span>
              {showMeta && <span className={stylex.props(sx.meta).className}>{option.code ?? option.meta}</span>}
              {option.meta != null && option.code != null && <span className={stylex.props(sx.meta).className}>{option.meta}</span>}
              {on && <Icon name="tick-circle" size={17} className={stylex.props(sx.tick).className} />}
            </button>
          </div>
        })}
        {footer && <div className={stylex.props(sx.footer).className}>{footer}</div>}
      </div>
    </Sheet>
  </div>
}

/** Chọn một của Viderse: KitSelect nổi trên desktop, Sheet 44px dưới 768px. */
export function ResponsiveSelect(props: SolidSelectProps) {
  const mobile = useMediaQuery(MOBILE)
  const [busy, setBusy] = useState(false)
  const onBusy = useCallback((next: boolean) => setBusy(next), [])
  return mobile || busy
    ? <MobileSelect {...props} mobile={mobile} onBusy={onBusy} />
    : <KitSelect {...props} />
}

/** Chip lọc của Viderse: giữ nguyên ChipSelect desktop, sheet mobile cùng options. */
export function ResponsiveChipSelect({ label, size = 'sm', custom, ...props }: ChipSelectProps) {
  const mobile = useMediaQuery(MOBILE)
  const [busy, setBusy] = useState(false)
  const onBusy = useCallback((next: boolean) => setBusy(next), [])
  return mobile || busy
    ? <MobileSelect {...props} label={label} chip size={size === 'm' ? 'm' : 'sm'} custom={custom} mobile={mobile} onBusy={onBusy} />
    : <ChipSelect {...props} label={label} size={size} custom={custom} />
}

type ActionMenuProps = Parameters<typeof ActionMenu>[0]

function MenuRow({ item, onChoose, depth = 0 }: { item: ActionMenuItem; onChoose: (item: ActionMenuItem) => void; depth?: number }) {
  const [expanded, setExpanded] = useState(false)
  const off = item.disabled
  const locked = !off && !!item.lockedReason
  const row = stylex.props(sx.row, depth > 0 && sx.sub, off && sx.rowDisabled, item.danger && !off && sx.rowDanger)
  const note = item.lockedReason ?? item.description
  const content = <>
    {item.icon && <span aria-hidden="true" className={stylex.props(sx.iconLead).className}>{item.icon}</span>}
    <span className={stylex.props(sx.rowText).className}>
      <span className={stylex.props(sx.rowLabel).className}>{item.label}</span>
      {note && <span className={stylex.props(sx.rowHint).className}>{note}</span>}
    </span>
    {item.meta != null && <span className={`num ${stylex.props(sx.meta).className}`}>{item.meta}</span>}
    {item.kbd != null && <span className={stylex.props(sx.meta).className}>{item.kbd}</span>}
    {item.selected && <Icon name="tick-circle" size={17} className={stylex.props(sx.tick).className} />}
    {item.children && <Icon name={expanded ? 'arrow-up' : 'arrow-down-2'} size={15} />}
  </>
  const onClick = () => {
    if (off || locked) return
    if (item.children) { setExpanded((next) => !next); return }
    onChoose(item)
  }
  return <>
    {item.separatorBefore && <div role="separator" className={stylex.props(sx.separator).className} />}
    {item.href && !off && !locked && !item.children
      ? <a
        role="menuitem"
        href={item.href}
        target={item.target}
        rel={item.rel}
        className={row.className}
        style={row.style}
        onClick={() => onChoose(item)}
      >{content}</a>
      : <button
        type="button"
        role={item.selected !== undefined ? 'menuitemradio' : 'menuitem'}
        aria-checked={item.selected}
        aria-haspopup={item.children ? 'menu' : undefined}
        aria-expanded={item.children ? expanded : undefined}
        aria-disabled={locked || undefined}
        disabled={off}
        className={row.className}
        style={row.style}
        onClick={onClick}
      >{content}</button>}
    {item.children && expanded && item.children.map((child) => <MenuRow key={child.id} item={child} onChoose={onChoose} depth={depth + 1} />)}
  </>
}

function MobileActionMenu({ mobile, onBusy, items, label = getKitLabels().menus.actions, trigger, triggerStyle, onClose, onOpen, defaultOpen = false }: ActionMenuProps & { mobile: boolean; onBusy: (busy: boolean) => void }) {
  const [open, setOpen] = useState(defaultOpen)
  const [pending, setPending] = useState<ActionMenuItem | null>(null)
  const menu = useRef<HTMLDivElement>(null)
  const busyRef = useRef(onBusy)
  busyRef.current = onBusy
  useEffect(() => { if (open || pending) busyRef.current(true) }, [open, pending])
  useEffect(() => { if (!mobile && open) setOpen(false) }, [mobile, open])
  useEffect(() => {
    if (!open || !mobile) return
    const frame = requestAnimationFrame(() => {
      menu.current?.querySelector<HTMLElement>('button[role^="menuitem"]:not(:disabled), a[role="menuitem"]')?.focus({ preventScroll: true })
    })
    return () => cancelAnimationFrame(frame)
  }, [open, mobile])
  const menuKeys = (event: KeyboardEvent<HTMLDivElement>) => {
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    const rows = [...(menu.current?.querySelectorAll<HTMLElement>('button[role^="menuitem"]:not(:disabled), a[role="menuitem"]') ?? [])]
    if (!rows.length) return
    event.preventDefault()
    const at = rows.indexOf(document.activeElement as HTMLElement)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? rows.length - 1
      : at < 0 ? (event.key === 'ArrowUp' ? rows.length - 1 : 0)
        : (at + (event.key === 'ArrowDown' ? 1 : -1) + rows.length) % rows.length
    rows[next].focus()
  }
  const close = () => { if (open) setOpen(false) }
  const choose = (item: ActionMenuItem) => {
    if (!open || item.disabled || item.lockedReason || pending) return
    setPending(item)
    setOpen(false)
  }
  const exited = () => {
    const item = pending
    setPending(null)
    if (item) item.onSelect?.()
    onClose?.()
    busyRef.current(false)
  }
  const trig = stylex.props(sx.menuTrigger, triggerStyle)
  return <>
    <button
      type="button"
      className={`hov-ghost ${trig.className}`}
      style={trig.style}
      aria-label={label}
      aria-haspopup="dialog"
      aria-expanded={open}
      onClick={() => { if (open) close(); else { busyRef.current(true); setOpen(true); onOpen?.() } }}
    >{trigger ?? <Icon name="more" size={17} />}</button>
    <Sheet open={mobile && open} onClose={close} onExited={exited} title={label} size="md" height="auto" homeIndicator>
      <div ref={menu} role="menu" aria-label={label} className={stylex.props(sx.sheetBody).className} onKeyDown={menuKeys}>
        {items.map((item) => <MenuRow key={item.id} item={item} onChoose={choose} />)}
      </div>
    </Sheet>
  </>
}

/** Menu thao tác: ActionMenu nổi trên desktop, Sheet 44px dưới 768px. */
export function ResponsiveActionMenu(props: ActionMenuProps) {
  const mobile = useMediaQuery(MOBILE)
  const [busy, setBusy] = useState(false)
  const onBusy = useCallback((next: boolean) => setBusy(next), [])
  return mobile || busy
    ? <MobileActionMenu {...props} mobile={mobile} onBusy={onBusy} />
    : <ActionMenu {...props} />
}

export type { ActionMenuProps, SolidSelectProps }
