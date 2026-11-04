import { Fragment, useEffect, useId, useLayoutEffect, useRef, useState, type CSSProperties, type KeyboardEvent, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from './tokens.stylex'
import { Kbd } from './Button'
import { Icon } from './Icon'
import { useEscapeLayer } from './Overlays'
import './kit.css'
import { getKitLabels } from './labels'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const L3 = line['--line-3']
const L4 = line['--line-4']
const ACC = color['--color-accent']
const ACC55 = kit['--kit-acc-55']
const ERRB55 = kit['--kit-errb-55']
const FG_OFF = kit['--kit-fg-off']
const BG = color['--color-bg']
const TRANSITION = { default: 'border-color .15s, box-shadow .15s', [NO_MOTION]: 'none' }
const ACC_FOCUS = { borderColor: ACC55, boxShadow: kit['--kit-ring-acc'] }
const ERR_FOCUS = { borderColor: ERRB55, boxShadow: kit['--kit-ring-err'] }

export type Option = {
  value: string
  label: string
  code?: string
  description?: string
  icon?: ReactNode
  meta?: ReactNode
  group?: string
  avatar?: ReactNode
  avatarColor?: string
  disabled?: boolean
}

export type OptionGroup = { label: string; options: Option[] }

const normalize = (value: string) => value.normalize('NFD').replace(/\p{M}/gu, '').replace(/đ/g, 'd').replace(/Đ/g, 'D').toLocaleLowerCase('vi')

/** Counts and group order always describe the filtered options, not the source list. */
export function filterOptionGroups(options: readonly Option[], query: string): OptionGroup[] {
  const needle = normalize(query.trim())
  const groups = new Map<string, Option[]>()
  for (const option of options) {
    if (needle && !normalize(option.label).includes(needle)) continue
    const label = option.group ?? ''
    const list = groups.get(label)
    if (list) list.push(option)
    else groups.set(label, [option])
  }
  return [...groups].map(([label, list]) => ({ label, options: list }))
}

/** Selected options remain removable at the limit, including after a caller lowers it. */
export function toggleMultiValue(values: readonly string[], option: Option, limit = 5): string[] {
  if (option.disabled) return [...values]
  if (values.includes(option.value)) return values.filter((value) => value !== option.value)
  if (values.length >= limit) return [...values]
  return [...values, option.value]
}

/** Gõ nhanh chữ đầu để nhảy tới lựa chọn: lặp một chữ thì xoay vòng, gõ cụm thì ở lại mục đang sáng nếu còn khớp. */
export function typeAheadIndex(options: readonly Option[], active: number, text: string): number {
  const chars = normalize(text)
  if (!chars) return -1
  const count = options.length
  // Lặp một ký tự (nn…) = gõ nhanh cùng một phím → xoay vòng qua các mục bắt đầu bằng chữ đó.
  const repeat = new Set(chars.split('')).size === 1
  const needle = repeat ? chars[0] : chars
  const match = (option: Option) => normalize(option.label).startsWith(needle)
  if (repeat) {
    for (let step = 1; step <= count; step++) {
      const index = (active + step) % count
      if (!options[index].disabled && match(options[index])) return index
    }
    return -1
  }
  if (active >= 0 && active < count && !options[active].disabled && match(options[active])) return active
  for (let index = 0; index < count; index++) {
    if (!options[index].disabled && match(options[index])) return index
  }
  return -1
}

export type KitSelectProps = {
  options: readonly Option[]
  value: string
  onChange: (value: string) => void
  id?: string
  name?: string
  label?: string
  'aria-label'?: string
  placeholder?: string
  disabled?: boolean
  error?: string
  /** Model picker: keep the full name, truncate only its secondary description. */
  inlineDescription?: boolean
  footer?: ReactNode
  defaultOpen?: boolean
  /** Biểu tượng nhỏ đứng trước giá trị ở ô chọn đơn (kit: language-square cho “Ngôn ngữ truyện”). */
  icon?: ReactNode
  /** Dense 32 px control for rows that sit beside other 32 px boxes (per-agent model pickers). */
  compact?: boolean
  /** Members.dc.html: 32px role trigger with a 300px descriptive permission menu. */
  variant?: 'project-role'
  /** Chiều cao ô chọn khi phải khớp một ô nhập bên cạnh (mặc định 36, compact 32). */
  height?: number
  /** Tự focus ô chọn khi vừa hiện (dùng cho ô chọn xuất hiện sau một nút “+ Thêm”). */
  autoFocus?: boolean
  className?: string
  style?: CSSProperties
  /** Lớp ngoài cùng: cho màn biến cả ô chọn thành một chip hay một hàng rộng (NewVideo.dc.html). */
  xstyle?: stylex.StyleXStyles
  /** Nút đóng (chưa mở danh sách): chiều cao, viền, khoảng cách giữa các phần. */
  triggerStyle?: stylex.StyleXStyles
  /** Cỡ mũi tên xuống (mặc định 15); chip nhỏ trong câu dùng 12. */
  chevronSize?: number
  /** Chữ đứng yên trước giá trị trong nút mở, vd “Kênh” / “Template”. */
  prefix?: ReactNode
  /** Bề rộng popover (px) khi nút mở hẹp hơn danh sách — ChipSelect dùng 240. */
  popWidth?: number
  /** Kiểu của chính giá trị đang chọn (ChipSelect in đậm giá trị như `.vv-chip b`). */
  valueStyle?: stylex.StyleXStyles
  /** Nhấn thu nút mở còn 98% (`.vv-chip:active`). */
  pressScale?: boolean
}

export type RichSelectProps = KitSelectProps & {
  onPreview?: (option: Option) => void
  /** Actual playback state belongs to the audio owner; no simulated playback timer. */
  playing?: string | null
  footer?: ReactNode
  onManage?: () => void
}

export type ComboboxProps = KitSelectProps & {
  loading?: boolean
  /** Câu giải thích đầy đủ dưới tiêu đề “Không tải được danh sách …”. */
  loadError?: string
  onRetry?: () => void
  onAddProvider?: () => void
  footer?: ReactNode
  /** Kit "Đang chọn: …" helper under the input. Off by default: the input already shows the value. */
  showCurrent?: boolean
  /** Danh từ của lựa chọn trong các câu trạng thái (“Đang tải danh sách model…”). Mặc định “lựa chọn”. */
  noun?: string
}

export type MultiComboboxProps = Omit<KitSelectProps, 'value' | 'onChange'> & {
  value: readonly string[]
  onChange: (value: string[]) => void
  limit?: number
  creatable?: boolean
  onCreate?: (label: string) => Option
}

const sx = stylex.create({
  col: { display: 'flex', flexDirection: 'column', gap: 6, minWidth: 0 },
  popWrap: { position: 'relative' },
  popWrapOpen: { zIndex: 30 },
  input: { paddingTop: 1, paddingBottom: 1, paddingInline: 2 },
  inputCompact: { height: 24, fontSize: 12 },
  small: { color: color['--color-fg-4'], fontSize: 11.5 },
  meta: { fontSize: 11, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  desc: { fontSize: 11.5, color: kit['--kit-fg-icon'] },
  iconButton: {
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto',
    width: 24, height: 24, borderWidth: 0, borderStyle: 'none', borderRadius: 6, padding: 0,
    backgroundColor: 'transparent', color: color['--color-fg-4'],
  },
  iconButtonSmall: { width: 18, height: 18, borderRadius: 5, color: ACC },
  errorMsg: { fontSize: 11.5, color: kit['--kit-err-soft'] },
  tickWrap: { width: 16, display: 'flex', color: ACC },
  tickPop: { animationName: { default: 'kit-tick', [NO_MOTION]: 'none' }, animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.3,1.5)', animationFillMode: 'both' },
  avatar: (size: string) => ({
    width: size, height: size, flex: '0 0 auto', borderRadius: '50%',
    color: BG, fontSize: 12, fontWeight: 600,
    display: 'flex', alignItems: 'center', justifyContent: 'center', overflow: 'hidden',
  }),
  eq: { display: 'flex', alignItems: 'center', gap: 2, height: 12 },
  eqBar: {
    display: 'inline-block', width: 2, borderRadius: 1, backgroundColor: 'currentColor',
    animationName: { default: 'kit-select-wave', [NO_MOTION]: 'none' }, animationDuration: '1s', animationTimingFunction: 'ease-in-out', animationIterationCount: 'infinite',
  },
  eqDelay: (delay: string) => ({ animationDelay: delay }),
  pop: (maxHeight: string) => ({
    position: 'absolute', zIndex: 40, left: 0, right: 0, top: 'calc(100% + 6px)',
    backgroundColor: color['--color-surface-4'], borderWidth: 1, borderStyle: 'solid',
    borderColor: L4, borderRadius: 14, boxShadow: kit['--kit-shadow-pop'],
    padding: 6, transformOrigin: 'top', maxHeight, overflowY: 'auto',
    animationName: { default: 'kit-select-pop', [NO_MOTION]: 'none' }, animationDuration: '.17s', animationTimingFunction: 'cubic-bezier(.2,.9,.3,1)', animationFillMode: 'both',
  }),
  popUp: { top: 'auto', bottom: 'calc(100% + 6px)', transformOrigin: 'bottom' },
  popModel: { minWidth: 'min(380px, calc(100vw - 40px))', top: 'calc(100% + 8px)' },
  popModelUp: { top: 'auto', bottom: 'calc(100% + 8px)' },
  popRole: { left: 'auto', right: 0, top: 'calc(100% + 6px)', width: 300, borderRadius: 12 },
  groupLabel: { display: 'block', fontSize: 10.5, paddingTop: 6, paddingInline: 10, paddingBottom: 4 },
  groupLabelCount: { display: 'flex', justifyContent: 'space-between', fontSize: 10.5, paddingTop: 8, paddingInline: 10, paddingBottom: 4 },
  groupCount: { color: FG_OFF },
  opt: {
    display: 'flex', alignItems: 'center', gap: 10, width: '100%', minHeight: 34,
    paddingTop: 6, paddingBottom: 6, paddingInline: 10, borderWidth: 0, borderStyle: 'none', borderRadius: 9,
    backgroundColor: { default: 'transparent', ':hover': line['--line-1'] },
    color: color['--color-fg'], fontSize: 13, textAlign: 'left', cursor: 'pointer', opacity: 1,
  },
  optHi: { backgroundColor: kit['--kit-wash-75'] },
  optDisabled: {
    color: FG_OFF, cursor: 'not-allowed', backgroundColor: 'transparent',
  },
  optRich: { minHeight: 48 },
  optModel: { minHeight: 38, paddingRight: 8 },
  optRole: { alignItems: 'flex-start', paddingTop: 8, paddingBottom: 8, borderRadius: 8 },
  roleLabel: { lineHeight: 1.4 },
  roleDescription: { fontSize: 12, lineHeight: 1.4, color: color['--color-fg-4'], whiteSpace: 'normal' },
  sep: { height: 1, backgroundColor: line['--line-2'], marginTop: 6, marginBottom: 6, marginInline: 4 },
  ghost: { backgroundColor: { default: 'transparent', ':hover': line['--line-2'] } },
  footerLabel: { flex: 1, color: color['--color-fg-2'] },
  /* Ô chọn (kit-sel-trg) */
  trg: {
    display: 'flex', alignItems: 'center', gap: 8, width: '100%', height: 36,
    minWidth: 0,
    paddingTop: 0, paddingBottom: 0, paddingLeft: 12, paddingRight: 10,
    borderRadius: 10, backgroundColor: color['--color-surface-0'],
    borderWidth: 1, borderStyle: 'solid', borderColor: L4,
    color: color['--color-fg'], fontSize: 13, textAlign: 'left', cursor: 'pointer',
    transition: { default: 'border-color .15s, box-shadow .15s, background .15s', [NO_MOTION]: 'none' },
  },
  trgNormal: {
    backgroundColor: { default: color['--color-surface-0'], ':hover': kit['--kit-surface-18'] },
    borderColor: { default: L4, ':hover': kit['--kit-wash-18'], ':focus-visible': ACC55 },
    boxShadow: { default: 'none', ':focus-visible': kit['--kit-ring-acc'] },
  },
  trgOpen: { borderColor: ACC55, boxShadow: kit['--kit-ring-acc'] },
  trgInvalid: {
    borderColor: { default: ERRB55, ':hover': ERRB55, ':focus-visible': ERRB55 },
    boxShadow: { default: kit['--kit-ring-err'], ':focus-visible': kit['--kit-ring-err'] },
  },
  trgDisabled: { opacity: .45, cursor: 'not-allowed' },
  trgCompact: { paddingTop: 0, paddingBottom: 0, paddingLeft: 10, paddingRight: 8, borderRadius: 8, fontSize: 12 },
  trgCompactH: { height: 32 },
  trgRole: { height: 32, paddingLeft: 12, paddingRight: 10, borderRadius: 9, fontSize: 13 },
  /* ChipSelect — `.vv-chip` của kit (Kit.dc.html › Huy hiệu · chip): 28 px, chữ 12,
     bo 8, nền surface-1, viền field; điện thoại cao 44. Mũi tên lật 180 ms có sẵn
     ở `chev`, nhấn thu .98 ở `.vv-chip:active` (kit.css › motion:controls). */
  chipTrg: {
    width: 'auto', height: 28, gap: 6, paddingLeft: 10, paddingRight: 8, borderRadius: 8,
    fontSize: 12, color: color['--color-fg-2'],
    backgroundColor: { default: color['--color-surface-1'], ':hover': kit['--kit-surface-18'] },
    borderColor: { default: line['--line-4'], ':hover': kit['--kit-wash-22'] },
  },
  chipPress: { transform: { ':active': 'scale(.98)' } },
  chipTrgM: { height: 44, paddingLeft: 12, paddingRight: 10, borderRadius: 12, fontSize: 12.5 },
  chipCustom: { borderColor: kit['--kit-warn-30'] },
  chipValue: { flex: '1 1 auto', minWidth: 0, fontWeight: 500, color: color['--color-fg'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  trgRoleDisabled: { opacity: .6 },
  chev: { flexShrink: 0, transition: { default: 'transform .18s', [NO_MOTION]: 'none' }, color: color['--color-fg-4'] },
  chevOpen: { transform: 'rotate(180deg)' },
  valueCol: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' },
  valueRich: { lineHeight: 1.3 },
  valueInline: { flexDirection: 'row', alignItems: 'center', gap: 6, overflow: 'hidden' },
  modelName: { flexShrink: 0, fontSize: 12.5, fontFamily: 'var(--font-mono)' },
  modelNameNowrap: { whiteSpace: 'nowrap' },
  modelDesc: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: kit['--kit-fg-icon'], fontSize: 12.5 },
  modelDescFill: { flex: '1 1 auto' },
  modelDescInline: { fontSize: 12, color: kit['--kit-fg-sub'] },
  modelChevInline: { color: kit['--kit-fg-sub'] },
  modelOptionName: { fontSize: 12.5, fontFamily: 'var(--font-mono)', lineHeight: 1.35 },
  modelOptionDesc: { fontSize: 12, lineHeight: 1.35 },
  modelTrigger: { paddingLeft: 11, paddingRight: 8, borderRadius: 9 },
  valueLabel: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  prefix: { flex: '0 0 auto', whiteSpace: 'nowrap' },
  valuePlaceholder: { flex: '1 1 auto', minWidth: 0, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  iconLead: { display: 'flex', color: color['--color-fg-3'] },
  empty: { padding: 10 },
  preview: {
    width: 28, height: 28, borderRadius: '50%', borderWidth: 1, borderStyle: 'solid',
    borderColor: L4, backgroundColor: 'transparent', color: ACC,
    display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto', padding: 0,
  },
  /* Hộp combobox (kit-sel-box) */
  box: {
    display: 'flex', alignItems: 'center', gap: 8, minHeight: 36,
    paddingTop: 4, paddingBottom: 4, paddingInline: 8, borderRadius: 10,
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid',
    transition: TRANSITION,
  },
  boxNormal: {
    borderColor: { default: L4, ':focus-within': ACC55 },
    boxShadow: { default: 'none', ':focus-within': kit['--kit-ring-acc'] },
  },
  boxInvalid: {
    borderColor: { default: ERRB55, ':focus-within': ERRB55 },
    boxShadow: { default: kit['--kit-ring-err'], ':focus-within': kit['--kit-ring-err'] },
  },
  boxDisabled: {
    opacity: .45,
    borderColor: { default: L4, ':focus-within': ACC55 },
    boxShadow: { default: 'none', ':focus-within': kit['--kit-ring-acc'] },
  },
  boxCompact: { paddingTop: 0, paddingBottom: 0, paddingLeft: 10, paddingRight: 8, borderRadius: 8, fontSize: 12 },
  boxTall: { paddingTop: 0, paddingBottom: 0, paddingLeft: 10, paddingRight: 8 },
  boxWrap: { flexWrap: 'wrap', paddingTop: 5, paddingBottom: 5, paddingInline: 8 },
  boxInput: {
    flex: '1 1 60px', minWidth: 60, height: 26, backgroundColor: 'transparent',
    borderWidth: 0, borderStyle: 'none', outline: 'none', color: color['--color-fg'], fontSize: 13,
  },
  loadError: { display: 'flex', flexDirection: 'column', gap: 8, paddingTop: 12, paddingBottom: 12, paddingInline: 10 },
  loadErrorTitle: { fontSize: 13, color: kit['--kit-err-text'], display: 'flex', gap: 8, alignItems: 'center' },
  loadErrorBody: { fontSize: 12, color: kit['--kit-fg-icon'] },
  retry: {
    alignSelf: 'flex-start', height: 30, paddingInline: 10, fontSize: 12, borderRadius: 8,
    display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: 7,
    whiteSpace: 'nowrap', borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-12'],
    backgroundColor: { default: 'transparent', ':hover': line['--line-2'] },
    color: color['--color-fg'], fontWeight: 500, position: 'relative',
    transition: { default: 'background .15s, box-shadow .2s, transform .08s, border-color .15s', [NO_MOTION]: 'none' },
  },
  optLabel: { flex: '1 1 auto', minWidth: 0, fontSize: 12.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  optLabelPlain: { flex: 1 },
  noMatch: { paddingTop: 14, paddingBottom: 14, paddingInline: 10, display: 'flex', flexDirection: 'column', gap: 4 },
  noMatchTitle: { fontSize: 13, color: color['--color-fg-2'] },
  noMatchHint: { fontSize: 12, color: color['--color-fg-4'] },
  showCurrent: { color: color['--color-fg-2'] },
  tag: {
    height: 26, paddingTop: 0, paddingBottom: 0, paddingLeft: 9, paddingRight: 4,
    borderRadius: 7, backgroundColor: kit['--kit-acc-12'],
    borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-acc-25'],
    color: kit['--kit-chip-tag'], fontSize: 12, display: 'flex', alignItems: 'center',
    gap: 4, whiteSpace: 'nowrap',
    animationName: { default: 'kit-tick', [NO_MOTION]: 'none' }, animationDuration: '.3s', animationTimingFunction: 'cubic-bezier(.2,.9,.3,1.5)', animationFillMode: 'both',
  },
  cb: {
    width: 18, height: 18, borderRadius: 5, borderWidth: 1.5, borderStyle: 'solid',
    borderColor: kit['--kit-wash-28'], display: 'flex', alignItems: 'center',
    justifyContent: 'center', flex: '0 0 auto',
    transition: { default: 'background .15s, border-color .15s', [NO_MOTION]: 'none' },
  },
  cbOn: { backgroundColor: ACC, borderColor: ACC },
  metaOff: { fontSize: 11, color: FG_OFF },
  count: { fontSize: 11 },
  countFull: { color: color['--color-warn'] },
  countIdle: { color: color['--color-fg-4'] },
  mark: {
    backgroundColor: kit['--kit-acc-22'], color: kit['--kit-mark-fg'],
    borderRadius: 3, paddingTop: 0, paddingBottom: 0, paddingInline: 1,
  },
  loading: { display: 'flex', flexDirection: 'column', gap: 8, padding: 8 },
  loadingRow: { display: 'flex', alignItems: 'center', gap: 8, color: color['--color-fg-3'], fontSize: 12.5 },
  spinner: {
    width: 14, height: 14, borderRadius: '50%', borderWidth: 2, borderStyle: 'solid',
    borderColor: kit['--kit-spin-track'], borderTopColor: color['--color-fg'],
    animationName: { default: 'kit-select-spin', [NO_MOTION]: 'none' }, animationDuration: '.9s', animationTimingFunction: 'linear', animationIterationCount: 'infinite',
    flex: '0 0 auto',
  },
  skel: (width: string) => ({
    width, height: 26, backgroundImage: kit['--kit-grad-skel'],
    backgroundSize: '200% 100%', borderRadius: 6,
    animationName: { default: 'kit-select-skeleton', [NO_MOTION]: 'none' }, animationDuration: '1.4s', animationTimingFunction: 'linear', animationIterationCount: 'infinite',
  }),
  labelRow: { display: 'flex', justifyContent: 'space-between' },
})

/**
 * Trạng thái mở/đóng + mục đang sáng của một listbox. Xuất cho các hộp tự dựng (vd. bộ chọn dự án/truyện)
 * để dùng chung Arrow/Home/End, Esc theo lớp và đóng khi bấm ra ngoài thay vì viết listbox thứ hai.
 */
export function usePopup(disabled?: boolean, defaultOpen = false, initialActive = -1) {
  const root = useRef<HTMLDivElement>(null)
  const [open, setOpen] = useState(defaultOpen && !disabled)
  const [active, setActive] = useState(initialActive)
  useEffect(() => {
    if (!open) return
    const outside = (event: PointerEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    document.addEventListener('pointerdown', outside)
    return () => document.removeEventListener('pointerdown', outside)
  }, [open])
  // Danh sách là một lớp: Esc đóng danh sách trước, không đóng hộp thoại bên dưới (17-kit, chốt 25/10).
  useEscapeLayer(open, () => setOpen(false))
  return { root, open, setOpen, active, setActive }
}

/** Mục kế tiếp còn chọn được theo ArrowUp/ArrowDown/Home/End (bỏ qua mục disabled). */
export function nextActive(options: readonly Option[], active: number, key: string): number {
  const count = options.length
  if (!count) return -1
  if (key === 'Home') {
    for (let index = 0; index < count; index++) if (!options[index].disabled) return index
    return -1
  }
  if (key === 'End') {
    for (let index = count - 1; index >= 0; index--) if (!options[index].disabled) return index
    return -1
  }
  const step = key === 'ArrowDown' ? 1 : -1
  for (let offset = 1; offset <= count; offset++) {
    const index = (active + step * offset + count * offset) % count
    if (!options[index].disabled) return index
  }
  return -1
}

function useActiveScroll(open: boolean, id: string, active: number) {
  useEffect(() => {
    if (!open || active < 0) return
    document.getElementById(`${id}-option-${active}`)?.scrollIntoView({ block: 'nearest' })
  }, [open, id, active])
}

/** Gõ chữ trên danh sách đóng/mở: bộ đệm 700 ms như select gốc. */
function useTypeAhead() {
  const state = useRef({ text: '', timer: 0 })
  useEffect(() => () => window.clearTimeout(state.current.timer), [])
  return (key: string) => {
    window.clearTimeout(state.current.timer)
    state.current.text += key
    state.current.timer = window.setTimeout(() => { state.current.text = '' }, 700)
    return state.current.text
  }
}

function Label({ id, label, right }: { id: string; label?: string; right?: ReactNode }) {
  if (!label && !right) return null
  const text = label ? <label className="lbl" htmlFor={id} style={{ fontSize: 10.5 }}>{label}</label> : <span />
  return right ? <div {...stylex.props(sx.labelRow)}>{text}{right}</div> : text
}

function ErrorMessage({ id, error }: { id: string; error?: string }) {
  return error ? <span id={`${id}-error`} role="alert" {...stylex.props(sx.errorMsg)}>{error}</span> : null
}

function Tick({ selected, animate }: { selected: boolean; animate?: boolean }) {
  return <span aria-hidden="true" {...stylex.props(sx.tickWrap)}>{selected && <Icon name="tick-circle" size={16} {...(animate ? stylex.props(sx.tickPop) : {})} />}</span>
}

function Avatar({ option, size = 30 }: { option: Option; size?: number }) {
  const p = stylex.props(sx.avatar(`${size}px`))
  // background ở style nội tuyến: có thể là gradient avatarColor (StyleX chỉ có backgroundColor).
  return <span aria-hidden="true" className={p.className} style={{ ...p.style, background: option.avatarColor ?? 'var(--color-accent)' }}>{option.avatar ?? option.label.slice(0, 2).toLocaleUpperCase('vi')}</span>
}

function WaveBars() {
  return <span aria-hidden="true" {...stylex.props(sx.eq)}><span {...stylex.props(sx.eqBar)} /><span {...stylex.props(sx.eqBar, sx.eqDelay('-.3s'))} /><span {...stylex.props(sx.eqBar, sx.eqDelay('-.6s'))} /></span>
}

/** Lề an toàn ngang tối thiểu giữa popover và khung nhìn (px). */
const POP_VIEWPORT_EDGE = 12

type PopFit = { left?: number; right?: number; width?: number; originX?: number }

/**
 * Popover của ô chọn: khi mở sát mép phải (chip lọc trên điện thoại, ô chọn trong
 * cột hẹp) thì kéo hộp vào trong khung nhìn thay vì để tràn/cắt; giữ neo ở tâm
 * trigger bằng `transform-origin` nên hộp vẫn “mọc” từ chip. Không portal: hộp
 * vẫn nằm trong `popWrap` như bố cục cũ, chỉ đổi offset ngang.
 */
function Pop({ max, children, onKeyDown, upward = false, model = false, role = false, width }: { max: number; children: ReactNode; onKeyDown?: (event: KeyboardEvent<HTMLDivElement>) => void; upward?: boolean; model?: boolean; role?: boolean; width?: number }) {
  const ref = useRef<HTMLDivElement>(null)
  const [fit, setFit] = useState<PopFit | null>(null)
  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const place = () => {
      const wrap = el.parentElement
      if (!wrap) return
      const wr = wrap.getBoundingClientRect()
      const w = el.offsetWidth
      const vw = document.documentElement.clientWidth || window.innerWidth
      if (!w || !vw) return
      const next: PopFit = {}
      if (role) {
        // `right: 0` — bám mép phải trigger; kéo vào khi tràn phải hoặc tràn trái.
        const min = wr.right - vw + POP_VIEWPORT_EDGE
        const max = wr.right - w - POP_VIEWPORT_EDGE
        const value = min > max ? max : Math.min(Math.max(0, min), max)
        if (Math.abs(value) > 0.5) next.right = value
        const originX = value + w - wr.width / 2
        if (Math.abs(value) > 0.5) next.originX = Math.min(w - 8, Math.max(8, originX))
      } else {
        // `left: 0` — bám mép trái trigger; kéo trái khi tràn phải.
        const min = POP_VIEWPORT_EDGE - wr.left
        const max = vw - POP_VIEWPORT_EDGE - w - wr.left
        const value = min > max ? max : Math.min(Math.max(0, min), max)
        if (Math.abs(value) > 0.5) {
          next.left = value
          // Hộp co giãn theo trigger: cố định bề rộng đo được khi phải dịch.
          if (!width) next.width = w
          next.originX = Math.min(w - 8, Math.max(8, wr.width / 2 - value))
        }
      }
      setFit((prev) => JSON.stringify(prev ?? {}) === JSON.stringify(next) ? prev : next)
    }
    place()
    const frame = requestAnimationFrame(place)
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => {
      cancelAnimationFrame(frame)
      window.removeEventListener('resize', place)
      window.removeEventListener('scroll', place, true)
    }
  }, [role, width, model, upward])
  const p = stylex.props(sx.pop(`${max}px`), role && sx.popRole, upward && sx.popUp, model && sx.popModel, model && upward && sx.popModelUp)
  const fitStyle: CSSProperties = fit ? {
    ...(fit.left !== undefined ? { left: fit.left, right: 'auto' } : {}),
    ...(fit.right !== undefined ? { right: fit.right, left: 'auto' } : {}),
    ...(fit.width !== undefined ? { width: fit.width } : {}),
    ...(fit.originX !== undefined ? { transformOrigin: `${fit.originX}px ${upward ? 'bottom' : 'top'}` } : {}),
  } : {}
  return <div ref={ref} data-select-pop data-placement={upward ? 'top' : 'bottom'} className={p.className} style={{ ...p.style, ...(width ? { width, right: 'auto' } : {}), ...fitStyle }} onKeyDown={onKeyDown}>{children}</div>
}

function GroupLabel({ children, count }: { children: ReactNode; count?: number }) {
  if (count == null) return <div className={`lbl ${stylex.props(sx.groupLabel).className}`} style={{ fontSize: 10.5 }}>{children}</div>
  return <div className={`lbl ${stylex.props(sx.groupLabelCount).className}`} style={{ fontSize: 10.5 }}><span>{children}</span><span className={`num ${stylex.props(sx.groupCount).className}`}>{count}</span></div>
}

function FooterAction({ icon, tone, children, onClick, action }: { icon: string; tone: string; children: ReactNode; onClick: () => void; action?: boolean }) {
  return <button type="button" {...stylex.props(sx.opt)} data-select-action={action || undefined} onMouseDown={(event) => event.preventDefault()} onClick={onClick}><Icon name={icon} size={15} style={{ color: tone }} /><span {...stylex.props(sx.footerLabel)}>{children}</span></button>
}

function SelectList({ rich = false, inlineDescription, options, value, onChange, id: suppliedId, name, label, 'aria-label': ariaLabel, placeholder, disabled, error, style, className, onPreview, playing, footer, onManage, defaultOpen, compact, height, autoFocus, icon, variant, xstyle, triggerStyle, chevronSize, prefix, popWidth, valueStyle, pressScale }: RichSelectProps & { rich?: boolean }) {
  const generatedId = useId()
  const id = suppliedId ?? generatedId
  const L = getKitLabels().selects
  const ph = placeholder ?? L.choose
  const initial = Math.max(0, options.findIndex((option) => option.value === value && !option.disabled))
  const { root, open, setOpen, active, setActive } = usePopup(disabled, defaultOpen, initial)
  const trigger = useRef<HTMLButtonElement>(null)
  const typed = useTypeAhead()
  const selected = options.find((option) => option.value === value)
  const [upward, setUpward] = useState(false)
  useEffect(() => {
    if (!open) return
    const place = () => {
      const rect = trigger.current?.getBoundingClientRect()
      const height = root.current?.querySelector<HTMLElement>('[data-select-pop]')?.offsetHeight || Math.min(300, options.length * 34 + (footer ? 55 : 12))
      if (rect) setUpward(window.innerHeight - rect.bottom < height + 6 && rect.top > window.innerHeight - rect.bottom)
    }
    place()
    window.addEventListener('resize', place)
    window.addEventListener('scroll', place, true)
    return () => { window.removeEventListener('resize', place); window.removeEventListener('scroll', place, true) }
  }, [open, options.length, footer])
  useActiveScroll(open, id, active)
  const begin = () => {
    const selectedIndex = options.findIndex((option) => option.value === value && !option.disabled)
    setActive(selectedIndex >= 0 ? selectedIndex : nextActive(options, -1, 'ArrowDown'))
    setOpen(true)
  }
  const choose = (option: Option) => {
    if (disabled || option.disabled) return
    onChange(option.value)
    setOpen(false)
    trigger.current?.focus()
  }
  const keyboard = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.nativeEvent.isComposing || disabled) return
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); setOpen(false); trigger.current?.focus(); return }
    if ((event.target as HTMLElement).closest('[data-select-action]')) return
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault()
      if (!open) begin()
      else setActive(nextActive(options, active, event.key))
      trigger.current?.focus()
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      if (!open) begin()
      else if (active >= 0 && options[active]) choose(options[active])
    } else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
      const index = typeAheadIndex(options, open ? active : initial, typed(event.key))
      if (index >= 0) { event.preventDefault(); setActive(index); setOpen(true) }
    }
  }
  const rootP = stylex.props(sx.col, xstyle)
  const wrap = stylex.props(sx.popWrap, open && sx.popWrapOpen)
  const trg = stylex.props(sx.trg,
    disabled ? sx.trgDisabled : error ? sx.trgInvalid : open ? sx.trgOpen : sx.trgNormal,
    compact && sx.trgCompact,
    compact && sx.trgCompactH,
    inlineDescription && sx.modelTrigger,
    variant === 'project-role' && sx.trgRole,
    variant === 'project-role' && disabled && sx.trgRoleDisabled,
    pressScale && sx.chipPress,
    triggerStyle,
  )
  const chev = stylex.props(sx.chev, open && sx.chevOpen)
  return <div ref={root} className={[rootP.className, className].filter(Boolean).join(' ')} style={{ ...rootP.style, ...style }} onKeyDown={keyboard} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false) }}>
    <Label id={id} label={label} />
    {name && <input type="hidden" name={name} value={value} disabled={disabled} />}
    <div className={wrap.className} style={wrap.style}>
      <button ref={trigger} id={id} type="button" role="combobox" className={trg.className} aria-label={ariaLabel ?? label ?? L.chooseItem} aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? `${id}-list` : undefined} aria-activedescendant={open && active >= 0 ? `${id}-option-${active}` : undefined} aria-invalid={!!error || undefined} aria-describedby={error ? `${id}-error` : undefined} disabled={disabled} autoFocus={autoFocus} title={selected ? [selected.label, inlineDescription && selected.description].filter(Boolean).join(' · ') : ph} onClick={() => open ? setOpen(false) : begin()} style={{ ...trg.style, ...(rich ? { height: 48 } : {}), ...(height ? { height } : {}) }}>
        {rich && selected && <Avatar option={selected} />}
        {!rich && icon && <span aria-hidden="true" {...stylex.props(sx.iconLead)}>{icon}</span>}
        {prefix != null && <span {...stylex.props(sx.prefix)}>{prefix}</span>}
        {selected
          ? <span {...stylex.props(sx.valueCol, rich && sx.valueRich, inlineDescription && sx.valueInline)}>
            <span {...stylex.props(inlineDescription ? sx.modelName : sx.valueLabel, valueStyle)}>{selected.label}</span>
            {inlineDescription && selected.description && <span {...stylex.props(sx.modelDesc)}>· {selected.description}</span>}
            {rich && selected.description && <span {...stylex.props(sx.desc, sx.valueLabel)}>{selected.description}</span>}
          </span>
          : <span {...stylex.props(sx.valuePlaceholder)}>{ph}</span>}
        <Icon name="arrow-down-2" size={chevronSize ?? 15} className={chev.className} style={chev.style} />
      </button>
      {open && <Pop max={variant === 'project-role' ? 400 : 300} upward={upward} model={inlineDescription} role={variant === 'project-role'} width={popWidth}>
        <div id={`${id}-list`} role="listbox" aria-label={ariaLabel ?? label ?? L.options}>
          {options.length === 0 && <div role="status" {...stylex.props(sx.small, sx.empty)}>{L.empty}</div>}
          {options.map((option, index) => {
            const opt = stylex.props(sx.opt, option.disabled ? sx.optDisabled : index === active && sx.optHi, rich && sx.optRich, inlineDescription && sx.optModel, variant === 'project-role' && sx.optRole)
            const common = {
              id: `${id}-option-${index}`, role: 'option', className: opt.className, title: [option.label, option.description].filter(Boolean).join(' · '), 'data-hi': index === active, 'aria-selected': option.value === value, 'aria-disabled': option.disabled || undefined,
              onMouseEnter: () => { if (!option.disabled) setActive(index) }, onClick: () => choose(option),
              style: opt.style,
            } as const
            const content = <>
              {rich && <Avatar option={option} />}
              {!rich && option.icon && <span aria-hidden="true" style={{ display: 'flex', flexShrink: 0, marginTop: variant === 'project-role' ? 7 : 0 }}>{option.icon}</span>}
              <span {...stylex.props(sx.valueCol, rich && sx.valueRich, !rich && sx.optLabelPlain, variant === 'project-role' && sx.roleLabel)}>
                <span {...stylex.props(sx.valueLabel, inlineDescription && sx.modelOptionName)}>{option.label}</span>
                {(rich || inlineDescription || variant === 'project-role') && option.description && <span {...stylex.props(sx.desc, sx.valueLabel, inlineDescription && sx.modelOptionDesc, variant === 'project-role' && sx.roleDescription)}>{option.description}</span>}
              </span>
              {option.code && <span className={`mono ${stylex.props(sx.meta).className}`}>{option.code}</span>}
              {option.meta}
              {rich && onPreview && <button type="button" data-select-action {...stylex.props(sx.preview, sx.ghost)} disabled={option.disabled} aria-label={(playing === option.value ? L.previewStop : L.previewStart)(option.label)} aria-pressed={playing === option.value} onClick={(event) => { event.stopPropagation(); onPreview(option) }}>{playing === option.value ? <WaveBars /> : <Icon name="play" size={13} />}</button>}
              <Tick selected={option.value === value} animate />
            </>
            return <Fragment key={option.value}>
              {option.group && (index === 0 || options[index - 1].group !== option.group) && <GroupLabel>{option.group}</GroupLabel>}
              {rich
                ? <div {...common} onMouseDown={(event) => { if (!(event.target as HTMLElement).closest('button')) event.preventDefault() }}>{content}</div>
                : <button {...common} type="button" tabIndex={-1} disabled={option.disabled} onMouseDown={(event) => event.preventDefault()}>{content}</button>}
            </Fragment>
          })}
        </div>
        {(footer || onManage) && <><div {...stylex.props(sx.sep)} />{footer ?? <FooterAction icon="setting-2" tone="var(--color-fg-3)" action onClick={() => { setOpen(false); trigger.current?.focus(); onManage?.() }}>{L.manageVoices}</FooterAction>}</>}
      </Pop>}
    </div>
    <ErrorMessage id={id} error={error} />
  </div>
}

export function KitSelect(props: KitSelectProps) { return <SelectList {...props} /> }
export function RichSelect(props: RichSelectProps) { return <SelectList {...props} rich /> }

export type ChipSelectProps = Omit<KitSelectProps, 'label'> & {
  /** Tên đọc cho trình đọc màn hình (chip không có nhãn nhìn thấy). */
  label: string
  /** `sm` 28 px (máy tính) · `m` 44 px (điện thoại). */
  size?: 'sm' | 'm'
  /** `.is-custom` — viền cam nhạt cho giá trị người dùng tự đặt (“riêng”). */
  custom?: boolean
}

/**
 * Chip chọn “khoá : giá trị ▾” (04-components › ChipSelect · Kit.dc.html):
 * nút `.vv-chip` mở danh sách chọn một. Mũi tên lật 180 ms, nhấn thu .98,
 * popover mở bằng chuyển động của kit; tôn trọng `prefers-reduced-motion`.
 */
export function ChipSelect({ label, size = 'sm', custom, triggerStyle, chevronSize, valueStyle, ...rest }: ChipSelectProps) {
  return <KitSelect {...rest}
    aria-label={rest['aria-label'] ?? label}
    pressScale
    chevronSize={chevronSize ?? 13}
    popWidth={rest.popWidth ?? 240}
    triggerStyle={[sx.chipTrg, size === 'm' && sx.chipTrgM, custom && sx.chipCustom, triggerStyle]}
    valueStyle={[sx.chipValue, valueStyle]} />
}

function Highlight({ label, query }: { label: string; query: string }) {
  const needle = normalize(query.trim())
  if (!needle) return <>{label}</>
  // Map normalized characters back to UTF-16 offsets, preserving decomposed accents.
  let normalized = ''
  let offset = 0
  const spans: { start: number; end: number }[] = []
  for (const char of label) {
    const part = normalize(char)
    if (!part && spans.length) spans[spans.length - 1].end = offset + char.length
    for (let index = 0; index < part.length; index++) spans.push({ start: offset, end: offset + char.length })
    normalized += part
    offset += char.length
  }
  const at = normalized.indexOf(needle)
  if (at < 0) return <>{label}</>
  const start = spans[at].start
  const end = spans[at + needle.length - 1].end
  return <>{label.slice(0, start)}<mark {...stylex.props(sx.mark)}>{label.slice(start, end)}</mark>{label.slice(end)}</>
}

function LoadingOptions({ noun }: { noun: string }) {
  return <div role="status" {...stylex.props(sx.loading)}>
    <span {...stylex.props(sx.loadingRow)}><span aria-hidden="true" {...stylex.props(sx.spinner)} />{getKitLabels().selects.loadingList(noun)}</span>
    {[undefined, '80%', '90%'].map((width) => <span key={width ?? 'full'} aria-hidden="true" {...stylex.props(sx.skel(width ?? '100%'))} />)}
  </div>
}



export function Combobox({ options, value, onChange, id: suppliedId, label, 'aria-label': ariaLabel, name, placeholder, disabled, error, loading, loadError, onRetry, onAddProvider, footer, className, style, defaultOpen, compact, showCurrent, noun, inlineDescription = false, chevronSize, popWidth }: ComboboxProps) {
  const generatedId = useId()
  const L = getKitLabels().selects
  const ph = placeholder ?? L.search
  const thing = noun ?? L.noun
  const id = suppliedId ?? generatedId
  const { root, open, setOpen, active, setActive } = usePopup(disabled, defaultOpen, 0)
  const trigger = useRef<HTMLButtonElement>(null)
  const input = useRef<HTMLInputElement>(null)
  const returnFocus = useRef(false)
  const wasOpen = useRef(open)
  const [query, setQuery] = useState('')
  const selected = options.find((option) => option.value === value)
  const groups = filterOptionGroups(options, query)
  const flat = groups.flatMap((group) => group.options)
  const ready = !loading && !loadError
  const currentActive = ready && active >= 0 && active < flat.length && !flat[active].disabled ? active : -1
  useActiveScroll(open, id, currentActive)
  // Mở bằng nút đóng: ô tìm chỉ dựng sau khi mở nên phải tự focus; đóng bằng
  // bàn phím/chọn xong thì trả focus về nút để không rơi khỏi trình đọc màn hình.
  useLayoutEffect(() => {
    if (open && inlineDescription) input.current?.focus()
  }, [open, inlineDescription])
  useLayoutEffect(() => {
    const closing = wasOpen.current && !open
    wasOpen.current = open
    if (!closing) return
    // Layer-level Escape can close before the input handler runs. Its removal
    // leaves focus on body; restore the trigger, but never steal Tab's target.
    const restore = returnFocus.current || document.activeElement === document.body
    returnFocus.current = false
    if (inlineDescription && restore) trigger.current?.focus()
  }, [open, inlineDescription])
  const begin = (seed = '') => {
    setQuery(seed)
    const list = seed ? filterOptionGroups(options, seed).flatMap((group) => group.options) : options
    setActive(nextActive(list, -1, 'ArrowDown'))
    setOpen(true)
  }
  const close = (focusTrigger: boolean) => {
    if (focusTrigger) returnFocus.current = true
    setOpen(false); setQuery('')
  }
  const choose = (option: Option) => {
    if (disabled || option.disabled || !ready) return
    onChange(option.value)
    close(inlineDescription)
  }
  const keyboard = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing) return
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(inlineDescription) }
    else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key) && (open || event.key.startsWith('Arrow'))) {
      event.preventDefault()
      if (!open) begin()
      else if (ready) setActive(nextActive(flat, currentActive, event.key))
    } else if (event.key === 'Enter' && open) {
      event.preventDefault()
      if (currentActive >= 0) choose(flat[currentActive])
    }
  }
  /** Nút đóng của ô chọn model: mở như KitSelect, gõ chữ đầu để tìm ngay. */
  const triggerKeyboard = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.nativeEvent.isComposing || disabled) return
    if (['ArrowDown', 'ArrowUp', 'Home', 'End', 'Enter', ' '].includes(event.key)) { event.preventDefault(); begin(); return }
    if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) { event.preventDefault(); begin(event.key) }
  }
  let optionIndex = 0
  const rootP = stylex.props(sx.col)
  const wrap = stylex.props(sx.popWrap, open && sx.popWrapOpen)
  const box = stylex.props(sx.box, disabled ? sx.boxDisabled : error ? sx.boxInvalid : sx.boxNormal, compact ? sx.boxCompact : sx.boxTall)
  const ghost = stylex.props(sx.iconButton, sx.ghost)
  const chev = stylex.props(sx.chev, open && sx.chevOpen)
  const chevClosed = stylex.props(sx.chev, sx.modelChevInline)
  const inp = stylex.props(sx.boxInput, compact ? sx.inputCompact : sx.input)
  const trg = stylex.props(sx.trg, disabled ? sx.trgDisabled : error ? sx.trgInvalid : sx.trgNormal, compact && sx.trgCompact, compact && sx.trgCompactH)
  return <div ref={root} className={[rootP.className, className].filter(Boolean).join(' ')} style={{ ...rootP.style, ...style }} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) { setOpen(false); setQuery('') } }}>
    <Label id={id} label={label} />
    {name && <input type="hidden" name={name} value={value} disabled={disabled} />}
    <div className={wrap.className} style={wrap.style}>
      {/* Ô chọn model: nút đóng như KitSelect (id mono + ngữ cảnh provider + chevron);
          ô tìm chỉ dựng khi mở nên nút đóng không mang icon kính lúp. */}
      {inlineDescription && !open
        ? <button ref={trigger} id={id} type="button" role="combobox" className={trg.className} style={trg.style} aria-label={ariaLabel ?? label ?? L.choose} aria-haspopup="listbox" aria-expanded={false} aria-invalid={!!error || undefined} aria-describedby={error ? `${id}-error` : undefined} disabled={disabled} title={selected ? [selected.label, selected.description].filter(Boolean).join(' · ') : value || ph} onClick={() => begin()} onKeyDown={triggerKeyboard}>
          {selected
            ? <>
              <span {...stylex.props(sx.modelName, sx.modelNameNowrap)}>{selected.label}</span>
              {selected.description && <span {...stylex.props(sx.modelDesc, sx.modelDescFill, sx.modelDescInline)}>{selected.description}</span>}
            </>
            : value
              ? <span {...stylex.props(sx.modelName, sx.modelNameNowrap)}>{value}</span>
              : <span {...stylex.props(sx.valuePlaceholder)}>{ph}</span>}
          <Icon name="arrow-down-2" size={chevronSize ?? 15} className={chevClosed.className} style={chevClosed.style} />
        </button>
        : <div className={`kit-frame ${box.className}`} data-invalid={!!error || undefined} data-disabled={disabled || undefined} style={{ ...box.style, ...(compact ? { minHeight: 32 } : inlineDescription ? { height: 36 } : { height: 38 }) }}>
          {!compact && <Icon name="search-normal-1" size={15} style={{ color: 'var(--color-fg-4)' }} />}
          <input ref={input} id={id} className={`mono ${inp.className}`} role="combobox" aria-label={ariaLabel ?? label ?? L.choose} aria-haspopup="listbox" aria-expanded={open} aria-autocomplete="list" aria-controls={open ? `${id}-list` : undefined} aria-activedescendant={open && currentActive >= 0 ? `${id}-option-${currentActive}` : undefined} aria-invalid={!!error || undefined} aria-describedby={error ? `${id}-error` : undefined} autoComplete="off" spellCheck={false} disabled={disabled} placeholder={ph} value={open ? query : selected?.label ?? value} onFocus={() => { if (!open) begin() }} onClick={() => { if (!open) begin() }} onChange={(event) => { const text = event.target.value; setQuery(text); setActive(nextActive(filterOptionGroups(options, text).flatMap((group) => group.options), -1, 'ArrowDown')); setOpen(true) }} onKeyDown={keyboard} style={inp.style} />
          {open && query && <button type="button" className={ghost.className} aria-label={L.clearSearch} onMouseDown={(event) => event.preventDefault()} onClick={() => { setQuery(''); setActive(nextActive(options, -1, 'ArrowDown')); input.current?.focus() }} style={ghost.style}><Icon name="add" size={14} style={{ transform: 'rotate(45deg)' }} /></button>}
          <button type="button" className={ghost.className} tabIndex={-1} disabled={disabled} aria-label={open ? L.closeList : L.openList} onMouseDown={(event) => event.preventDefault()} onClick={() => { if (open) close(inlineDescription); else { input.current?.focus(); begin() } }} style={ghost.style}><Icon name="arrow-down-2" size={chevronSize ?? 15} className={chev.className} style={chev.style} /></button>
        </div>}
      {open && <Pop max={300} width={popWidth} onKeyDown={(event) => { if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); close(inlineDescription) } }}>
        <div id={`${id}-list`} role="listbox" aria-label={ariaLabel ?? label ?? L.options} aria-busy={loading || undefined}>
          {loading ? <LoadingOptions noun={thing} /> : loadError ? <div role="alert" {...stylex.props(sx.loadError)}><span {...stylex.props(sx.loadErrorTitle)}><Icon name="warning-2" size={15} />{L.loadFailed(thing)}</span><span {...stylex.props(sx.loadErrorBody)}>{loadError}</span>{onRetry && <button type="button" {...stylex.props(sx.retry)} onMouseDown={(event) => event.preventDefault()} onClick={onRetry}><Icon name="refresh-2" size={13} />{getKitLabels().retry}</button>}</div> : groups.length ? groups.map((group) => <div key={group.label} role="group" aria-label={group.label || L.options}>
            {group.label && <GroupLabel count={group.options.length}>{group.label}</GroupLabel>}
            {group.options.map((option) => { const index = optionIndex++; const opt = stylex.props(sx.opt, option.disabled ? sx.optDisabled : index === currentActive && sx.optHi); return <button id={`${id}-option-${index}`} key={option.value} type="button" role="option" className={opt.className} style={opt.style} data-hi={index === currentActive} aria-selected={option.value === value} disabled={option.disabled} tabIndex={-1} onMouseDown={(event) => event.preventDefault()} onMouseEnter={() => { if (!option.disabled) setActive(index) }} onClick={() => choose(option)}><span className={`mono ${stylex.props(sx.optLabel).className}`}><Highlight label={option.label} query={query} /></span>{option.meta != null && <span {...stylex.props(sx.meta)}>{option.meta}</span>}<Tick selected={option.value === value} /></button> })}
          </div>) : <div role="status" {...stylex.props(sx.noMatch)}><span {...stylex.props(sx.noMatchTitle)}>{L.noMatch(thing, query)}</span><span {...stylex.props(sx.noMatchHint)}>{L.noMatchHint(thing)}</span></div>}
        </div>
        {ready && (footer || onAddProvider) && <><div {...stylex.props(sx.sep)} />{footer ?? <FooterAction icon="add" tone="var(--color-accent)" onClick={() => { setOpen(false); onAddProvider?.() }}>{L.addProvider}</FooterAction>}</>}
      </Pop>}
    </div>
    {showCurrent && value && <span {...stylex.props(sx.small)}>{L.currentLabel}<span className={`mono ${stylex.props(sx.showCurrent).className}`}>{selected?.label ?? value}</span></span>}
    <ErrorMessage id={id} error={error} />
  </div>
}

export function MultiCombobox({ options, value, onChange, id: suppliedId, label, 'aria-label': ariaLabel, name, placeholder, disabled, error, limit = 5, creatable = true, onCreate, className, style, defaultOpen }: MultiComboboxProps) {
  const generatedId = useId()
  const L = getKitLabels().selects
  const id = suppliedId ?? generatedId
  const { root, open, setOpen, active, setActive } = usePopup(disabled, defaultOpen, 0)
  const input = useRef<HTMLInputElement>(null)
  const [query, setQuery] = useState('')
  const [created, setCreated] = useState<Option[]>([])
  const allOptions = [...options, ...created.filter((option) => !options.some((existing) => existing.value === option.value))]
  const full = value.length >= limit
  const visible = filterOptionGroups(allOptions, query).flatMap((group) => group.options).map((option) => ({ ...option, disabled: option.disabled || (full && !value.includes(option.value)) }))
  const text = query.trim()
  const canCreate = creatable && !!text && !full && !allOptions.some((option) => normalize(option.label) === normalize(text) || normalize(option.value) === normalize(text)) && !value.some((item) => normalize(item) === normalize(text))
  const candidates: Option[] = canCreate ? [...visible, { value: '', label: text }] : visible
  const currentActive = active >= 0 && active < candidates.length && !candidates[active].disabled ? active : -1
  useActiveScroll(open, id, currentActive)
  const toggle = (option: Option) => {
    if (disabled || option.disabled) return
    onChange(toggleMultiValue(value, option, limit)); setQuery(''); input.current?.focus()
  }
  const create = () => {
    if (disabled || !canCreate) return
    const option = onCreate ? onCreate(text) : { value: text, label: text }
    if (option.disabled || value.includes(option.value)) return
    setCreated((previous) => [...previous.filter((existing) => existing.value !== option.value), option])
    onChange([...value, option.value]); setQuery(''); input.current?.focus()
  }
  const keyboard = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing) return
    if (event.key === 'Backspace' && !query && value.length) { event.preventDefault(); onChange(value.slice(0, -1)) }
    else if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); setOpen(false) }
    else if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key) && (open || event.key.startsWith('Arrow'))) {
      event.preventDefault(); setOpen(true); setActive(nextActive(candidates, currentActive, event.key))
    } else if (event.key === 'Enter') {
      event.preventDefault()
      if (!open) { setOpen(true); return }
      if (currentActive >= 0 && currentActive < visible.length) toggle(visible[currentActive])
      else if (canCreate && (currentActive === visible.length || !visible.length)) create()
      else { const first = nextActive(visible, -1, 'ArrowDown'); if (first >= 0) toggle(visible[first]) }
    }
  }
  const labelOf = (item: string) => allOptions.find((option) => option.value === item)?.label ?? item
  const rootP = stylex.props(sx.col)
  const wrap = stylex.props(sx.popWrap, open && sx.popWrapOpen)
  const box = stylex.props(sx.box, disabled ? sx.boxDisabled : error ? sx.boxInvalid : sx.boxNormal, sx.boxWrap)
  const count = stylex.props(sx.count, full ? sx.countFull : sx.countIdle)
  const inp = stylex.props(sx.boxInput, sx.input)
  return <div ref={root} className={[rootP.className, className].filter(Boolean).join(' ')} style={{ ...rootP.style, ...style }} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false) }}>
    <Label id={id} label={label} right={<span id={`${id}-count`} className={`num ${count.className}`} aria-live="polite">{value.length}/{limit}</span>} />
    {name && value.map((item) => <input key={item} type="hidden" name={name} value={item} disabled={disabled} />)}
    <div className={wrap.className} style={wrap.style}>
      <div className={`kit-frame ${box.className}`} data-invalid={!!error || undefined} data-disabled={disabled || undefined} style={{ ...box.style, height: 'auto' }} onClick={() => { if (!disabled) { input.current?.focus(); setOpen(true) } }}>
        {value.map((item) => <span key={item} {...stylex.props(sx.tag)}>{labelOf(item)}<button type="button" disabled={disabled} aria-label={L.removeTag(labelOf(item))} onClick={(event) => { event.stopPropagation(); onChange(value.filter((existing) => existing !== item)); input.current?.focus() }} {...stylex.props(sx.iconButton, sx.iconButtonSmall)}><Icon name="add" size={12} style={{ transform: 'rotate(45deg)' }} /></button></span>)}
        <input ref={input} id={id} role="combobox" aria-label={ariaLabel ?? label ?? L.chooseMany} aria-expanded={open} aria-autocomplete="list" aria-controls={open ? `${id}-list` : undefined} aria-activedescendant={open && currentActive >= 0 ? `${id}-option-${currentActive}` : undefined} aria-invalid={!!error || undefined} aria-describedby={`${id}-count${error ? ` ${id}-error` : ''}`} disabled={disabled} autoComplete="off" placeholder={value.length ? '' : placeholder ?? L.searchMany} value={query} onFocus={() => setOpen(true)} onChange={(event) => { setQuery(event.target.value); setActive(0); setOpen(true) }} onKeyDown={keyboard} className={inp.className} style={inp.style} />
      </div>
      {open && <Pop max={270}>
        <div id={`${id}-list`} role="listbox" aria-label={ariaLabel ?? label ?? L.options} aria-multiselectable="true">
          {visible.map((option, index) => { const selected = value.includes(option.value); const opt = stylex.props(sx.opt, option.disabled ? sx.optDisabled : index === currentActive && sx.optHi); const cb = stylex.props(sx.cb, selected && sx.cbOn); return <Fragment key={option.value}>
            {option.group && (index === 0 || visible[index - 1].group !== option.group) && <GroupLabel>{option.group}</GroupLabel>}
            <button id={`${id}-option-${index}`} type="button" role="option" className={opt.className} style={opt.style} data-hi={index === currentActive} aria-selected={selected} disabled={option.disabled} tabIndex={-1} onMouseDown={(event) => event.preventDefault()} onMouseEnter={() => { if (!option.disabled) setActive(index) }} onClick={() => toggle(option)}>
              <span aria-hidden="true" className={cb.className} style={cb.style} data-on={selected}>{selected && <svg width="11" height="11" viewBox="0 0 24 24" fill="none"><path d="M5 12.5l4.5 4.5L19 7.5" stroke="var(--color-bg)" strokeWidth="3.2" strokeLinecap="round" strokeLinejoin="round" /></svg>}</span>
              <span {...stylex.props(sx.optLabelPlain)}><Highlight label={option.label} query={query} /></span>{option.meta != null && <span className={`num ${stylex.props(sx.metaOff).className}`}>{option.meta}</span>}
            </button>
          </Fragment> })}
          {canCreate && <><div aria-hidden="true" {...stylex.props(sx.sep)} />{(() => { const opt = stylex.props(sx.opt); return <button id={`${id}-option-${visible.length}`} type="button" role="option" className={opt.className} style={opt.style} data-hi={currentActive === visible.length || !visible.length} aria-selected={false} tabIndex={-1} onMouseDown={(event) => event.preventDefault()} onMouseEnter={() => setActive(visible.length)} onClick={create}><Icon name="add" size={15} style={{ color: 'var(--color-accent)' }} /><span {...stylex.props(sx.optLabelPlain)}>{L.create(text)}</span><Kbd>Enter</Kbd></button> })()}</>}
          {!visible.length && !canCreate && <div role="status" {...stylex.props(sx.small, sx.noMatch)}>{full ? L.limitReached(limit) : L.noMatchGeneric}</div>}
        </div>
      </Pop>}
    </div>
    <ErrorMessage id={id} error={error} />
  </div>
}
