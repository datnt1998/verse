import { cloneElement, isValidElement, useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from './tokens.stylex'
import { Kbd } from './Button'
import { Icon } from './Icon'
import { Tooltip, useEscapeLayer } from './Overlays'
import './kit.css'
import { getKitLabels } from './labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const FG4 = color['--color-fg-4']

const sx = stylex.create({
  list: { outline: 'none' },
  iconTick: { color: color['--color-accent'] },
  iconWrap: { display: 'inline-flex', flex: '0 0 auto' },
  iconDanger: { color: kit['--kit-err-bright'] },
  iconNormal: { color: FG4 },
  optHeight: (minHeight: string) => ({ minHeight }),
  labelCol: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  label: { whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  desc: { fontSize: 11.5, color: FG4, overflowWrap: 'anywhere' },
  meta: { fontSize: 11, color: FG4, whiteSpace: 'nowrap' },
  subIcon: { color: FG4 },
  row: { position: 'relative' },
  menuPos: (left: string, top: string, width: number) => ({ position: 'fixed', width, left, top }),
  trigger: {
    width: 30, height: 30, borderRadius: 8, borderWidth: 0, borderStyle: 'none', color: color['--color-fg-2'],
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
  },
  triggerOpen: { backgroundColor: line['--line-3'] },
  triggerIdle: { backgroundColor: 'transparent' },
  context: { display: 'flex', alignItems: 'center', gap: 10 },
})

export type ActionMenuItem = { id: string; label: string; icon?: ReactNode; kbd?: ReactNode; /** số/ghi chú nhỏ bên phải (vd “24 ch”) */ meta?: ReactNode; description?: string; tooltip?: string; danger?: boolean; disabled?: boolean; /** khoá nhưng vẫn focus được (aria-disabled): lý do thay dòng phụ và là aria-describedby; bấm không làm gì */ lockedReason?: string; selected?: boolean; children?: ActionMenuItem[]; onSelect?: () => void }
export type MenuPoint = { x: number; y: number }
/** Icon của mục menu: 16 px (15 px ở mục chọn một), màu fg-3 — mục nguy hiểm --kit-err-bright. */
function itemIcon(item: ActionMenuItem) {
  if (item.selected) return <Icon name="tick-circle" size={15} {...stylex.props(sx.iconTick)} />
  if (!item.icon) return null
  const size = item.selected === undefined ? 16 : 15
  const wrap = stylex.props(sx.iconWrap, item.danger ? sx.iconDanger : sx.iconNormal)
  return <span className={wrap.className} style={wrap.style}>{isValidElement<{ size?: number }>(item.icon) && item.icon.type === Icon ? cloneElement(item.icon, { size }) : item.icon}</span>
}
function MenuList({ items, close, nested = false, onBack, initialSubmenu = null }: { items: ActionMenuItem[]; close: () => void; nested?: boolean; onBack?: () => void; initialSubmenu?: string | null }) {
  const list = useRef<HTMLDivElement>(null)
  const uid = useId()
  const [submenu, setSubmenu] = useState<string | null>(initialSubmenu)
  const [flip, setFlip] = useState(false)
  // Không có deps: vị trí của menu cha được đặt sau lần render đầu (ActionMenu), nên phải đo lại ở mỗi lần render.
  useLayoutEffect(() => { const r = list.current?.getBoundingClientRect(); if (r) setFlip(r.right + 245 > window.innerWidth) })
  return <div ref={list} role="menu" tabIndex={-1} {...stylex.props(sx.list)} onKeyDown={(event) => {
    if ((event.target as HTMLElement).closest('[role=menu]') !== list.current) return
    const buttons = Array.from(list.current?.querySelectorAll<HTMLButtonElement>(':scope > [data-menu-row] > button:not(:disabled), :scope > [data-menu-row] > span > button:not(:disabled)') ?? [])
    const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
    const enabledItems = items.filter((item) => !item.disabled)
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault(); event.stopPropagation()
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? buttons.length - 1 : event.key === 'ArrowUp' && index < 0 ? buttons.length - 1 : (index + (event.key === 'ArrowDown' ? 1 : buttons.length - 1)) % buttons.length
      buttons[next]?.focus()
    }
    if (event.key === 'ArrowRight' && enabledItems[index]?.children) {
      event.preventDefault(); setSubmenu(enabledItems[index].id)
      requestAnimationFrame(() => buttons[index]?.closest('[data-menu-row]')?.querySelector<HTMLElement>('[role=menu] button:not(:disabled)')?.focus())
    }
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close() }
  }}>{items.map((item, index) => {
    const note = item.lockedReason ?? item.description
    const opt = stylex.props(sx.optHeight(note ? '48px' : '34px'))
    const ids = item.lockedReason ? { label: `${uid}-${item.id}-l`, note: `${uid}-${item.id}-r` } : null
    const button = <button type="button" role={item.selected !== undefined ? 'menuitemradio' : 'menuitem'} aria-checked={item.selected} aria-haspopup={item.children ? 'menu' : undefined} aria-expanded={item.children ? submenu === item.id : undefined} disabled={item.disabled} aria-disabled={ids ? true : undefined} aria-labelledby={ids?.label} aria-describedby={ids?.note} className={[`kit-option${item.danger ? ' kit-danger' : ''}`, opt.className].filter(Boolean).join(' ')} onClick={() => { if (ids) return; if (item.children) setSubmenu(item.id); else { item.onSelect?.(); close() } }} style={opt.style}>
      {itemIcon(item)}<span {...stylex.props(sx.labelCol)}><span id={ids?.label} {...stylex.props(sx.label)}>{item.label}</span>{note && <small id={ids?.note} {...stylex.props(sx.desc)}>{note}</small>}</span>{item.meta != null && <span className={`num ${stylex.props(sx.meta).className}`}>{item.meta}</span>}{item.kbd && <Kbd>{item.kbd}</Kbd>}{item.children && <Icon name="arrow-right-2" size={14} {...stylex.props(sx.subIcon)} />}
    </button>
    return <div key={item.id} data-menu-row {...stylex.props(sx.row)} onPointerEnter={() => setSubmenu(item.children ? item.id : null)}>
      {item.danger && !items[index - 1]?.danger && <div role="separator" className="kit-separator" />}
      {item.tooltip ? <Tooltip content={item.tooltip} block>{button}</Tooltip> : button}
      {item.children && submenu === item.id && <div className="kit-pop kit-fly" data-flip={flip}><MenuList items={item.children} close={close} nested onBack={() => { setSubmenu(null); list.current?.children[index]?.querySelector<HTMLElement>('button')?.focus() }} /></div>}
    </div>
  })}</div>
}

/**
 * Menu thao tác ⋯. Đặt `data-menu-anchor` lên dòng chứa nút để menu mở ngay dưới dòng (4 px) như artboard;
 * không có thì mở dưới nút (6 px). `defaultOpen`/`defaultSubmenu` cho gallery mở sẵn; `onOpen` báo khi người dùng mở.
 */
export function ActionMenu({ items, label = getKitLabels().menus.actions, trigger, point, onClose, onOpen, alwaysVisible = false, defaultOpen = false, defaultSubmenu = null, width = 264 }: { items: ActionMenuItem[]; label?: string; trigger?: ReactNode; point?: MenuPoint | null; onClose?: () => void; onOpen?: () => void; alwaysVisible?: boolean; defaultOpen?: boolean; defaultSubmenu?: string | null; /** bề rộng popover (mặc định 264); menu có dòng phụ dài dùng rộng hơn. */ width?: number }) {
  const [open, setOpen] = useState(defaultOpen)
  const [position, setPosition] = useState<MenuPoint>({ x: 0, y: 0 })
  const button = useRef<HTMLButtonElement>(null)
  const menu = useRef<HTMLDivElement>(null)
  const keyboard = useRef(false)
  const initialSubmenu = useRef(defaultSubmenu)
  const isOpen = open || !!point
  const close = () => { setOpen(false); keyboard.current = false; initialSubmenu.current = null; onClose?.(); button.current?.focus() }
  const openMenu = (byKeyboard: boolean) => { keyboard.current = byKeyboard; initialSubmenu.current = null; setOpen(true); onOpen?.() }
  useLayoutEffect(() => {
    if (!isOpen) return
    const place = () => {
      const rect = button.current?.getBoundingClientRect()
      const row = button.current?.closest<HTMLElement>('[data-menu-anchor]')?.getBoundingClientRect()
      const origin = point ?? { x: (rect?.right ?? 0) + 4, y: row ? row.bottom + 4 : (rect?.bottom ?? 0) + 6 }
      // offsetWidth/Height (không bị scale của animation mở menu làm lệch).
      const w = menu.current?.offsetWidth ?? width
      const height = menu.current?.offsetHeight ?? 250
      setPosition({ x: Math.max(8, Math.min(point ? origin.x : origin.x - w, window.innerWidth - w - 8)), y: Math.max(8, Math.min(origin.y, window.innerHeight - height - 8)) })
    }
    place()
    // Bố cục có thể còn dịch sau lần vẽ đầu (font, thanh cuộn) — đo lại một lần nữa ở khung hình kế.
    const frame = requestAnimationFrame(place)
    // Mở bằng chuột: focus vào menu (không tô mục đầu); mở bằng phím: focus mục đầu như artboard (↓ mở → mục 1 sáng).
    if (keyboard.current) menu.current?.querySelector<HTMLElement>('button:not(:disabled)')?.focus()
    else menu.current?.querySelector<HTMLElement>('[role=menu]')?.focus({ preventScroll: true })
    return () => cancelAnimationFrame(frame)
  }, [isOpen, point, width])
  useEffect(() => {
    if (!point) return
    const outside = (event: PointerEvent) => { if (!menu.current?.contains(event.target as Node)) close() }
    document.addEventListener('pointerdown', outside)
    return () => { document.removeEventListener('pointerdown', outside) }
  }, [point])
  // Menu là một lớp: Esc đóng menu trước, không đóng hộp thoại bên dưới (17-kit, chốt 25/10).
  useEscapeLayer(isOpen, close)
  const trig = stylex.props(sx.trigger, isOpen ? sx.triggerOpen : sx.triggerIdle)
  const trigger_ = <button ref={button} type="button" className={`hov-ghost${alwaysVisible ? '' : ' kit-action-trigger'} ${trig.className}`} aria-label={label} aria-haspopup="menu" aria-expanded={isOpen} onClick={(event) => { if (open) close(); else openMenu(event.detail === 0) }} onKeyDown={(event) => { if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); openMenu(true) } }} style={trig.style}>{trigger ?? <Icon name="more" size={17} />}</button>
  const pos = stylex.props(sx.menuPos(`${position.x}px`, `${position.y}px`, width))
  return <><Tooltip content={label} asChild>{trigger_}</Tooltip>{isOpen && createPortal(<div ref={menu} className={`kit-pop ${pos.className}`} style={pos.style}><MenuList items={items} close={close} initialSubmenu={initialSubmenu.current} /></div>, document.body)}</>
}

type ActionMenuOptions = Pick<Parameters<typeof ActionMenu>[0], 'label' | 'defaultOpen' | 'defaultSubmenu' | 'onOpen' | 'alwaysVisible'>

/** Dòng có menu thao tác: chuột phải mở tại con trỏ, nút ⋯ hiện khi rê chuột. `anchorToRow`: menu ⋯ mở ngay dưới dòng (artboard). */
export function ContextMenu({ items, children, style, className, anchorToRow = false, actionProps }: { items: ActionMenuItem[]; children: ReactNode; style?: CSSProperties; className?: string; anchorToRow?: boolean; actionProps?: ActionMenuOptions }) {
  const [point, setPoint] = useState<MenuPoint | null>(null)
  const wrap = stylex.props(sx.context)
  return <div className={['kit-context', wrap.className, className].filter(Boolean).join(' ')} data-menu-anchor={anchorToRow || undefined} style={{ ...wrap.style, ...style }} onContextMenu={(event) => { event.preventDefault(); setPoint({ x: event.clientX, y: event.clientY }) }}>{children}<ActionMenu {...actionProps} items={items} point={point} onClose={() => setPoint(null)} /></div>
}
