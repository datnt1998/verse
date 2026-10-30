// ModelPicker — chọn model cho một trợ lý (artboard Settings ai=director|artist,
// MSettings sheet=director|artist). Chỉ liệt kê model ĐÃ CHỌN ở Kết nối AI,
// gom theo nhà cung cấp ("ChatGPT · đăng nhập gói"), tag năng lực bên phải;
// model thiếu năng lực bắt buộc bị làm mờ + không chọn được. Caps KHÔNG RÕ
// (catalog chưa tải) không bịa — để chọn được, server 422 sẽ báo inline.
// Wire: lựa chọn là ModelRef {provider, model} (domain.AgentModels).
// Bàn phím: popover là listbox thật — Arrow/Home/End di chuyển active (bỏ qua
// mục bị mờ), Enter/Space chọn, Esc đóng và trả focus về nút mở.
import { useEffect, useId, useRef, useState } from 'react'
import { Icon, usePopup } from '@noverse/ui'
import { navigate } from '../../../router'
import type { AiModel, AiProvider, ModelRef } from '../../../api/ai'
import { aiCopy, capsLine } from '../../../i18n/ai'
import { capClass } from '../connect/ModelCatalog'
import { planLabel } from '../connect/ProviderCard'
import { priceText } from '../connect/ModelCatalog'

const a = aiCopy.agents

export type Picked = {
  ref: ModelRef
  label: string
  providerLabel: string
  /** Loại kết nối — dòng nhóm “ChatGPT · đăng nhập gói”. */
  auth: 'subscription' | 'api_key' | 'local'
  /** Gói đã lưu của provider (chatgpt) → nhóm “ChatGPT · gói Plus”. */
  plan?: string
  /** Caps thật từ catalog đã chọn; undefined = chưa tải → không đoán. */
  caps: string[] | undefined
  contextK?: number
  /** Giá mỗi ảnh (USD) — thẻ nguồn vẽ. */
  perImage?: number
  /** Giá thật của model (api_key) — dòng chú thích “$1,25 vào · $10 ra”. */
  price?: AiModel['price']
}

export const refId = (ref: ModelRef) => (ref.model ? `${ref.provider}/${ref.model}` : '')

/** Ghép model đã chọn của các provider; caps lấy từ catalog đã tải (nếu có). */
export function pickedFrom(providers: AiProvider[], catalogOf: (id: string) => AiModel[] | undefined): Picked[] {
  const out: Picked[] = []
  for (const p of providers) {
    const catalog = catalogOf(p.id)
    const byId = new Map((catalog ?? []).map((m) => [m.id, m]))
    for (const model of p.selected) {
      const hit = byId.get(model)
      out.push({
        ref: { provider: p.id, model },
        label: model,
        providerLabel: p.label,
        auth: p.auth,
        plan: p.plan,
        caps: catalog === undefined ? undefined : (hit?.caps ?? []),
        contextK: hit?.contextK,
        perImage: hit?.price?.perImage,
        price: hit?.price,
      })
    }
  }
  return out
}

/** needs: 'vision' → đọc ảnh; 'image' → vẽ ảnh; 'text' → viết chữ.
 *  caps undefined (chưa tải) = không chặn — thiếu thật server sẽ từ chối.
 *  Câu chặn theo artboard: model chỉ vẽ ảnh luôn là “Chỉ vẽ ảnh, không viết chữ”
 *  (kể cả khi đang cần đọc ảnh); model thiếu đúng năng lực mới dùng câu riêng. */
export function blockedReason(picked: Picked, needs?: 'vision' | 'image' | 'text'): string | null {
  if (picked.caps === undefined) return null
  const caps = picked.caps
  const onlyDraws = caps.includes('image') && !caps.includes('text') && !caps.includes('vision')
  if (needs === 'vision' && !caps.includes('vision')) return onlyDraws ? a.blockedImage : a.blockedVision
  if (needs === 'image' && !caps.includes('image')) return a.blockedDraw
  if (needs === 'text' && !caps.includes('text')) return onlyDraws ? a.blockedImage : a.blockedText
  return null
}

type NavItem = { item: Picked; reason: string | null }

/** Danh sách phẳng có index điều hướng (dùng chung cho keyboard + render). */
export function navItems(picked: Picked[], needs?: 'vision' | 'image' | 'text'): NavItem[] {
  return picked.map((item) => ({ item, reason: blockedReason(item, needs) }))
}

function nextEnabled(items: NavItem[], active: number, key: string): number {
  const count = items.length
  if (!count) return -1
  if (key === 'Home') {
    for (let i = 0; i < count; i++) if (!items[i].reason) return i
    return -1
  }
  if (key === 'End') {
    for (let i = count - 1; i >= 0; i--) if (!items[i].reason) return i
    return -1
  }
  const step = key === 'ArrowDown' ? 1 : -1
  for (let offset = 1; offset <= count; offset++) {
    const i = (active + step * offset + count * offset) % count
    if (!items[i].reason) return i
  }
  return -1
}

/** Chú thích của hàng: năng lực + ngữ cảnh thật (design dùng câu biên tập —
 *  backend không có, nên chỉ hiện dữ liệu). */
function noteOf(p: Picked): string {
  // Artboard: hàng khoá API ghi giá thay cho dòng năng lực (pill đã có năng lực).
  const price = p.auth === 'subscription' ? '' : priceText(p.price)
  if (price) return price
  if (p.caps === undefined) return ''
  const caps = capsLine(p.caps).toLowerCase()
  return p.contextK ? `${caps} · ${aiCopy.connect.context(p.contextK)}` : caps
}

const groupTitle = (p: Picked) =>
  p.auth === 'subscription' && p.plan ? a.groupPlan(p.providerLabel, planLabel(p.plan)) : `${p.providerLabel} · ${a.groupAuth[p.auth]}`

export function ModelList({ picked, value, needs, onPick, listId, activeIndex, onActive, onKeyDown, variant = 'pop' }: {
  picked: Picked[]
  value: string
  needs?: 'vision' | 'image' | 'text'
  onPick: (picked: Picked) => void
  /** Điều hướng bàn phím (chỉ popover): id option + index đang active. */
  listId?: string
  activeIndex?: number
  onActive?: (index: number) => void
  onKeyDown?: (event: React.KeyboardEvent) => void
  /** pop = popover máy tính (dấu tick bên phải); sheet = sheet điện thoại (radio trái). */
  variant?: 'pop' | 'sheet'
}) {
  if (picked.length === 0) return <div className="agt-pick__empty">{a.pickerEmpty}</div>
  const items = navItems(picked, needs)
  let index = -1
  return (
    <div
      id={listId}
      role={listId ? 'listbox' : undefined}
      aria-activedescendant={listId && activeIndex != null && activeIndex >= 0 ? `${listId}-opt-${activeIndex}` : undefined}
      tabIndex={listId ? -1 : undefined}
      onKeyDown={onKeyDown}
      style={{ outline: 'none' }}
    >
      {picked.map((item, i) => {
        index = i
        const reason = items[i].reason
        const on = refId(item.ref) === value
        const firstOfGroup = i === 0 || groupTitle(picked[i - 1]) !== groupTitle(item)
        return (
          <div key={refId(item.ref)}>
            {firstOfGroup && <div className="agt-group">{groupTitle(item)}</div>}
            <button
              id={listId ? `${listId}-opt-${index}` : undefined}
              type="button"
              role="option"
              aria-selected={on}
              aria-disabled={reason != null || undefined}
              className={`agt-opt${on ? ' agt-opt--on' : ''}${reason ? ' agt-opt--dim' : ''}${index === activeIndex ? ' agt-opt--active' : ''}`}
              disabled={reason != null}
              tabIndex={-1}
              onClick={() => onPick(item)}
              onMouseEnter={() => onActive?.(index)}
              title={reason ?? undefined}
            >
              {variant === 'sheet' && <span className={`agt-radio${on ? ' agt-radio--on' : ''}`} aria-hidden="true" />}
              <span className="agt-opt__body">
                <span className="agt-opt__id">{item.label}</span>
                <span className="agt-opt__note">{reason ?? noteOf(item)}</span>
              </span>
              {item.caps !== undefined && (
                variant === 'sheet'
                  ? <span className={capClass(item.caps)}>{capsLine(item.caps)}</span>
                  : <span className="agt-opt__cap">{capsLine(item.caps)}</span>
              )}
              {variant === 'pop' && on && <Icon name="tick" size={15} className="agt-opt__check" />}
            </button>
          </div>
        )
      })}
    </div>
  )
}

/** Nút mở popover chọn model (máy tính). */
export function ModelPicker({ picked, value, needs, onPick, placeholder, disabled, label, title, sub }: {
  picked: Picked[]
  value: string
  needs?: 'vision' | 'image' | 'text'
  onPick: (picked: Picked) => void
  placeholder?: string
  disabled?: boolean
  /** Tên truy cập của nút mở (vd "Model viết mô tả ảnh"). */
  label?: string
  /** Tiêu đề popover: “Model của Đạo diễn”. */
  title: string
  /** Câu dẫn dưới tiêu đề. */
  sub: string
}) {
  const pop = usePopup(disabled)
  const trigger = useRef<HTMLButtonElement>(null)
  const listId = useId()
  const [active, setActive] = useState(-1)
  const items = navItems(picked, needs)
  const current = picked.find((p) => refId(p.ref) === value)

  // Mở ra → active = lựa chọn hiện tại (hoặc mục đầu chọn được); đóng → trả
  // focus về nút mở (aria-activedescendant nên giữ focus trong listbox).
  useEffect(() => {
    if (!pop.open) return
    const at = items.findIndex((n) => refId(n.item.ref) === value && !n.reason)
    setActive(at >= 0 ? at : nextEnabled(items, -1, 'ArrowDown'))
    // Mở ra là thấy hàng đang chọn (artboard: hàng chọn nằm trong khung nhìn,
    // có tick bên phải) — cuộn CHỈ trong list; scrollIntoView của trình duyệt
    // kéo cả trang, làm mất tiêu đề mục ở đầu trang.
    if (at < 0) return
    const list = pop.root.current?.querySelector<HTMLElement>('.agt-pop__body')
    const row = list?.querySelector<HTMLElement>(`#${CSS.escape(`${listId}-opt-${at}`)}`)
    if (list && row) {
      const rowTop = row.getBoundingClientRect().top - list.getBoundingClientRect().top + list.scrollTop
      const rowBottom = rowTop + row.offsetHeight
      if (rowTop < list.scrollTop) list.scrollTop = rowTop
      else if (rowBottom > list.scrollTop + list.clientHeight) list.scrollTop = rowBottom - list.clientHeight
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pop.open])

  const close = () => {
    pop.setOpen(false)
    trigger.current?.focus()
  }

  function onKeyDown(event: React.KeyboardEvent) {
    switch (event.key) {
      case 'ArrowDown':
      case 'ArrowUp':
      case 'Home':
      case 'End': {
        event.preventDefault()
        setActive((prev) => nextEnabled(items, prev, event.key))
        return
      }
      case 'Enter':
      case ' ': {
        if (active >= 0 && items[active]) {
          event.preventDefault()
          onPick(items[active].item)
          close()
        }
        return
      }
      case 'Escape':
        event.preventDefault()
        close()
        return
      case 'Tab':
        pop.setOpen(false)
        return
    }
  }

  return (
    <div ref={pop.root} className="agt-pickwrap">
      <button
        ref={trigger}
        type="button"
        className={`agt-pick${pop.open ? ' agt-pick--open' : ''}`}
        onClick={() => pop.setOpen(!pop.open)}
        disabled={disabled}
        aria-haspopup="listbox"
        aria-expanded={pop.open}
        aria-label={label}
      >
        <span className="agt-pick__id">{current?.label || placeholder || a.pickModel}</span>
        {current && <span className="agt-pick__prov">{groupTitle(current)}</span>}
        <Icon name="arrow-d" size={13} className="agt-pick__chev" />
      </button>
      {pop.open && (
        <div className="agt-pop" ref={(el) => el?.focus({ preventScroll: true })} tabIndex={-1}>
          <div className="agt-pop__head">
            <span className="agt-pop__title">{title}</span>
            <span className="agt-pop__sub">{sub}</span>
          </div>
          <div className="agt-pop__body">
            <ModelList
            picked={picked}
            value={value}
            needs={needs}
            listId={listId}
            activeIndex={active}
            onActive={setActive}
            onKeyDown={onKeyDown}
            onPick={(p) => { onPick(p); close() }}
            />
          </div>
          <div className="agt-pop__foot">
            <button type="button" className="agt-pop__link" onClick={() => navigate('/settings?section=connect')}>
              <Icon name="key" size={13} />
              {a.pickerFooter}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
