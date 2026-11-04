import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles } from '../settings.stylex'
import { Button, Combobox, Icon, type Option } from '@noverse/ui'
import { navigate } from '../../../router'
import type { AiModel, AiProvider, ModelRef } from '../../../api/ai'
import { aiCopy, capsLine } from '../../../i18n/ai'
import { capStyles } from '../connect/ModelCatalog'
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
  p.auth === 'subscription'
    ? (p.plan ? a.groupPlan(p.providerLabel, planLabel(p.plan)) : p.providerLabel)
    : `${p.providerLabel} · ${a.groupAuth[p.auth]}`

/** Shared option data keeps the provider groups, closed-trigger provider context, capability guards and real prices. */
function modelOptions(picked: Picked[], needs?: 'vision' | 'image' | 'text'): Option[] {
  return picked.map((item) => {
    const reason = blockedReason(item, needs)
    const providerContext = groupTitle(item)
    return {
      value: refId(item.ref),
      label: item.label,
      description: providerContext,
      group: providerContext,
      disabled: reason != null,
      meta: <span {...stylex.props(settingsStyles.modelOptionMeta)}>
        <span {...stylex.props(settingsStyles.modelOptionNote)}>{reason ?? noteOf(item)}</span>
        {item.caps !== undefined && <span {...stylex.props(capStyles(item.caps))}>{capsLine(item.caps)}</span>}
      </span>,
    }
  })
}

export function ModelList({ picked, value, needs, onPick, variant = 'pop' }: {
  picked: Picked[]
  value: string
  needs?: 'vision' | 'image' | 'text'
  onPick: (picked: Picked) => void
  variant?: 'pop' | 'sheet'
}) {
  return <Combobox
    aria-label={a.pickModel}
    options={modelOptions(picked, needs)}
    value={value}
    placeholder={picked.length ? a.pickModel : a.pickerEmpty}
    defaultOpen={variant === 'sheet'}
    onChange={(id) => { const item = picked.find((p) => refId(p.ref) === id); if (item) onPick(item) }}
  />
}

/** The kit owns focus, search, disabled-option navigation and reduced motion. */
export function ModelPicker({ picked, value, needs, onPick, placeholder, disabled, label, title, sub }: {
  picked: Picked[]
  value: string
  needs?: 'vision' | 'image' | 'text'
  onPick: (picked: Picked) => void
  placeholder?: string
  disabled?: boolean
  label?: string
  title: string
  sub: string
}) {
  return <Combobox
    aria-label={label ?? title}
    options={modelOptions(picked, needs)}
    value={value}
    disabled={disabled}
    inlineDescription
    chevronSize={12}
    popWidth={400}
    placeholder={placeholder ?? a.pickModel}
    onChange={(id) => { const item = picked.find((p) => refId(p.ref) === id); if (item) onPick(item) }}
    footer={<>
      <p {...stylex.props(settingsStyles.modelOptionNote)}>{sub}</p>
      <Button variant="quiet" size="sm" icon={<Icon name="key" size={13} />} onClick={() => navigate('/settings?section=connect')}>{a.pickerFooter}</Button>
    </>}
  />
}
