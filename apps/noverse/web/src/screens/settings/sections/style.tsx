// Gói AL — mảnh “Cách viết” dùng chung: lưới Gói thể loại + Kiểu viết, thẻ “Gói … gồm” (đầy đủ
// cho Tuỳ chỉnh, rút gọn cho tab truyện), tên hiển thị của khoá `style` và hàm lưu ngay.
// Ba cấp vẫn ghi `config.style` qua PUT /api/config — không đổi API.
import { useState, type ReactNode } from 'react'
import { putConfig } from '../../../api/client'
import type { Config, ConfigPresets, ConfigScope, ConfigResponse, PackPreset } from '../../../api/types'
import { cache, invalidate, keys } from '../../../data'
import { configForSave } from '../lib/config'
import {
  findPack, layerCounts, packBadge, packCards, packDesc, packInfoMeta,
  packLabel, packParamChips, packRuleSummary, PARAM_ICON, sectionCount, sectionRef,
} from '../lib/packs'
import { Icon } from '@noverse/ui'
import '../style.css'

/** Thứ tự + lời mô tả của 5 văn phong có sẵn (artboard `STY`). */
export const BUILT_IN: { id: string; label: string; desc: string }[] = [
  { id: 'default', label: 'Chung', desc: 'Nhịp hài hoà, tả bằng giác quan, thoại tự nhiên, súc tích.' },
  { id: 'romance', label: 'Ngôn tình', desc: 'Cảm xúc tinh tế, chú trọng tương tác và nội tâm.' },
  { id: 'suspense', label: 'Hồi hộp', desc: 'Cài manh mối, nhịp dồn, kết chương bỏ ngỏ.' },
  { id: 'fantasy', label: 'Huyền huyễn', desc: 'Thế giới rộng, hệ thống sức mạnh rõ ràng.' },
  { id: 'wuxia', label: 'Kiếm hiệp', desc: 'Giang hồ, chiêu thức, khí khái nhân vật.' },
]
export const CUSTOM_LABEL = 'Văn phong của bạn'

export type StyleCard = { id: string; label: string; desc: string; disabled?: boolean }

/**
 * Khả năng chọn “Văn phong của bạn” theo cấp (Gói AL, 20-settings-rules.md › Cách viết — vị trí):
 * - `story`: chỉ chọn được khi truyện đã có văn mẫu (`samples`); câu mô tả ăn theo số mẫu.
 * - `blocked`: cấp dự án / “Mọi truyện” — ô mờ, hướng dẫn thêm văn mẫu ở Dự án & truyện.
 * - `undefined` (gọi cũ): giữ hành vi cũ — chọn được khi tệp `presets.styles` có tên tự tạo.
 */
export type CustomAvailability = 'story' | 'blocked' | undefined

const CUSTOM_DESC = 'Học từ văn mẫu của bạn (lệnh /simulate)'

function customDesc(state: CustomAvailability, samples: { count: number; chars: number } | null | undefined, story: boolean): string {
  if (state === 'blocked') return 'Văn mẫu thêm theo từng truyện ở Dự án & truyện'
  if (state === 'story') {
    if (samples && samples.count > 0) return `Học từ ${samples.count} văn mẫu của truyện này · ${samples.chars.toLocaleString('vi-VN')} chữ`
    return 'Thêm văn mẫu ở bên dưới để dùng'
  }
  void story
  return samples && samples.count > 0
    ? `Học từ ${samples.count} văn mẫu bạn đưa vào · ${samples.chars.toLocaleString('vi-VN')} chữ`
    : CUSTOM_DESC
}

/**
 * 5 thẻ có sẵn rồi tới “Văn phong của bạn” (tệp tự tạo trong `presets.styles`, cả tên đang chọn nếu
 * tệp đã mất). Artboard luôn có 6 thẻ; thẻ tự tạo mờ theo `custom` (xem `CustomAvailability`).
 * Id gói thể loại không bị coi là văn phong tự tạo.
 */
export function styleCards(
  presetStyles: string[], current: string,
  options: {
    packs?: string[]
    samples?: { count: number; chars: number } | null
    custom?: CustomAvailability
    story?: boolean
  } = {},
): StyleCard[] {
  const known = new Set([...BUILT_IN.map((item) => item.id), ...(options.packs ?? [])])
  const custom = presetStyles.filter((name) => !known.has(name))
  const customMissing = current && !known.has(current) && !custom.includes(current)
  if (customMissing) custom.unshift(current)
  const story = options.story ?? false
  const desc = customDesc(options.custom, options.samples, story)
  const cards: StyleCard[] = BUILT_IN.map(({ id, label, desc: text }) => ({ id, label, desc: text }))
  // Thẻ đang chọn giữ khả dụng (không khoá chính lựa chọn hiện tại); còn lại mờ theo cấp:
  // 'blocked' = văn mẫu không thể thêm ở cấp này; 'story' chưa mẫu = chưa chọn được.
  const selectable = (id: string): boolean => {
    if (id === current && customMissing) return true
    if (options.custom === 'blocked') return false
    if (options.custom === 'story' && !(options.samples && options.samples.count > 0)) return id === current
    return true
  }
  const blocked = (id: string) => !selectable(id)
  if (custom.length === 0) return [...cards, { id: '', label: CUSTOM_LABEL, desc, disabled: true }]
  return [...cards, ...custom.map((name, index) => ({
    id: name, label: index === 0 ? CUSTOM_LABEL : name, desc, ...(blocked(name) ? { disabled: true } : null),
  }))]
}

/** Tên hiển thị của một id `style`: gói → “Gói <tên ngắn>”, kiểu có sẵn → nhãn artboard, lạ → id. */
export function styleDisplayName(presets: ConfigPresets | null | undefined, id: string | undefined): string {
  if (!id || id === 'default') return 'Chung'
  const pack = findPack(presets?.packs, id)
  if (pack) return `Gói ${packLabel(pack)}`
  const builtin = BUILT_IN.find((item) => item.id === id)
  if (builtin) return builtin.label
  return id
}

/** Nguồn “riêng/theo” của khoá style ở một cấp, cho thẻ chỉ dẫn và chip truyện. */
export function styleSourceText(source: ConfigScope | undefined, scope: ConfigScope): string {
  if (!source || source === scope) return ''
  return scope === 'story' ? 'theo dự án' : 'theo “Mọi truyện”'
}

/** Icon + màu từng tầng luật (artboard `pk.layers`): Master · thể loại · cách đo. */
const LAYER_ICON: Record<string, { icon: string; color: string }> = {
  master: { icon: 'book-1', color: 'var(--color-accent)' },
  pack: { icon: 'flag', color: 'var(--color-warn)' },
  measure: { icon: 'chart', color: 'var(--kit-info-fg)' },
}

const cardTitle: React.CSSProperties = { fontSize: 14, fontWeight: 500, flex: '1 1 auto' }
const cardDesc: React.CSSProperties = { fontSize: 12.5, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }

/**
 * Lưới “Gói thể loại” + “Kiểu viết” (Settings.dc.html + Outline.dc.html `wpacks`/`wstyles`).
 * Chọn một ô gọi `onPick` — ở tab truyện/mục dự án hàm này lưu ngay, ở Tuỳ chỉnh ghi vào bản nháp.
 */
export function StylePicker({ value, presets, custom, story, samples, disabled, onPick, header }: {
  /** id `style` đang chọn (kể cả kế thừa — “Theo dự án” vẫn sáng ô của cấp kế thừa). */
  value: string
  presets: ConfigPresets | null
  /** 'story' = cấp truyện (mờ thẻ tự tạo cho tới khi có văn mẫu); 'blocked' = dự án / Mọi truyện. */
  custom?: CustomAvailability
  story?: boolean
  samples?: { count: number; chars: number } | null
  disabled?: boolean
  onPick: (id: string) => void
  /** Nội dung phụ (ví dụ badge kế thừa của Tuỳ chỉnh) không dùng ở bản artboard Outline. */
  header?: ReactNode
}) {
  const packs = packCards(presets?.packs)
  const cards = styleCards(presets?.styles ?? [], value, {
    packs: presets?.packs?.map((item) => item.id), samples, custom, story,
  })
  return (
    <div role="radiogroup" aria-label="Văn phong" style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
      {header}
      {packs.length > 0 && (
        <>
          <div className="st-grph">Gói thể loại<span>có bộ luật, chấm từng chương</span></div>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 8 }}>
            {packs.map((card) => {
              const on = card.id === value
              const badge = packBadge(card.pack)
              const desc = packDesc(card.pack)
              return (
                <button key={card.id} type="button" role="radio" aria-checked={on}
                  disabled={disabled || card.pack.available === false}
                  className={`st-style${on ? ' on' : ''}`} onClick={() => onPick(card.id)}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ fontSize: 14, fontWeight: 500, flex: '1 1 auto' }}>{card.label}</span>
                    <span className="st-pbadge" style={{
                      background: badge.ok ? 'var(--tint-ok)' : 'var(--line-1)',
                      color: badge.ok ? 'var(--color-ok)' : 'var(--kit-fg-icon)',
                    }}><Icon name={badge.ok ? 'shield-tick' : 'clock'} size={12} />{badge.label}</span>
                  </span>
                  {desc && <span style={cardDesc}>{desc}</span>}
                  {card.meta && <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{card.meta}</span>}
                </button>
              )
            })}
          </div>
        </>
      )}
      <div className="st-grph" style={{ marginTop: packs.length > 0 ? 4 : 0 }}>
        Kiểu viết<span>gợi ý giọng văn, không chấm theo bộ luật</span>
      </div>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 8 }}>
        {cards.map((card) => {
          const on = card.id !== '' && card.id === value
          return (
            <button key={card.id || 'custom-empty'} type="button" role="radio" aria-checked={on}
              disabled={disabled || card.disabled} className={`st-style${on ? ' on' : ''}`}
              onClick={() => onPick(card.id)}>
              <span style={{ fontSize: 14, fontWeight: 500 }}>{card.label}</span>
              <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)', lineHeight: 1.5 }}>{card.desc}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}

/** Thẻ “Gói … gồm” đầy đủ của Tuỳ chỉnh (tầng luật + danh sách § + số đo + giọng đọc + ghi chú). */
export function PackDetailsCard({ pack, audioHref }: { pack: PackPreset; audioHref?: string }) {
  const [packOpen, setPackOpen] = useState(false)
  const paramChips = packParamChips(pack)
  return (
    <section className="card st-sec" aria-label="Gói đang dùng" style={{ gap: 0 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 10, paddingBottom: 12 }}>
        <Icon name="shield-tick" size={18} style={{ color: 'var(--color-ok)' }} />
        <b style={{ flex: '1 1 auto', fontSize: 14.5, fontWeight: 600 }}>Gói {packLabel(pack)} gồm</b>
        {(pack.layers?.length ?? 0) > 0 && (
          <button type="button" className="hov-ghost" aria-expanded={packOpen} onClick={() => setPackOpen((open) => !open)}
            style={{
              height: 30, padding: '0 12px', borderRadius: 8, border: '1px solid var(--line-4)',
              background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12.5,
              display: 'flex', alignItems: 'center', gap: 6,
            }}>
            {packOpen ? 'Thu gọn' : 'Xem các mục'}
            <Icon name="arrow-down-2" size={13} style={{ transform: `rotate(${packOpen ? 180 : 0}deg)`, transition: 'transform .25s' }} />
          </button>
        )}
      </div>
      {(pack.layers ?? []).map((layer) => (
        <div className="st-lyr" key={layer.key}>
          <Icon name={LAYER_ICON[layer.key]?.icon ?? 'book-1'} size={17}
            style={{ color: LAYER_ICON[layer.key]?.color ?? 'var(--color-fg-3)', marginTop: 2 }} />
          <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2, lineHeight: 1.45 }}>
            <span style={{ fontSize: 13.5, fontWeight: 500 }}>{layer.title}</span>
            {layer.desc && <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>{layer.desc}</span>}
          </div>
          <span style={{ display: 'flex', gap: 6, flex: '0 0 auto' }}>
            {layerCounts(layer).map((count) => (
              <span key={count.text} style={{
                height: 22, padding: '0 8px', borderRadius: 6, fontSize: 11.5, display: 'inline-flex',
                alignItems: 'center', whiteSpace: 'nowrap',
                background: count.tone === 'err' ? 'var(--tint-err)' : count.tone === 'wrn' ? 'var(--tint-warn)' : 'var(--line-1)',
                color: count.tone === 'err' ? 'var(--color-err)' : count.tone === 'wrn' ? 'var(--color-warn)' : 'var(--color-fg-3)',
              }}>{count.text}</span>
            ))}
          </span>
        </div>
      ))}
      {packOpen && (pack.sections?.length ?? 0) > 0 && (
        <div className="st-drawer" style={{ padding: '4px 0 12px 29px', display: 'flex', flexDirection: 'column', gap: 8 }}>
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Luật chung · số mục bắt sửa / cảnh báo</span>
          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)', gap: '2px 18px' }}>
            {(pack.sections ?? []).map((section) => {
              const count = sectionCount(section)
              return (
                <div key={section.ref} style={{
                  display: 'flex', alignItems: 'center', gap: 8, minHeight: 30, fontSize: 12.5,
                  color: 'var(--color-fg-2)', borderTop: '1px solid var(--line-1)',
                }}>
                  <span className="num" style={{ flex: '0 0 30px', color: 'var(--color-fg-4)' }}>{sectionRef(section.ref)}</span>
                  <span style={{ flex: '1 1 auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{section.title}</span>
                  <span className="num" style={{
                    whiteSpace: 'nowrap',
                    color: count.tone === 'err' ? 'var(--color-err)' : count.tone === 'wrn' ? 'var(--color-warn)' : 'var(--color-fg-4)',
                  }}>{count.text}</span>
                </div>
              )
            })}
          </div>
        </div>
      )}
      {paramChips.length > 0 && (
        <div className="st-lyr" style={{ flexDirection: 'column', gap: 8 }}>
          <span style={{ fontSize: 13, fontWeight: 500 }}>Số đo dùng để chấm</span>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
            {paramChips.map((param) => (
              <span className="st-pchip" key={param.key}>
                <Icon name={PARAM_ICON[param.key] ?? 'note-text'} size={13} style={{ color: 'var(--kit-fg-icon)' }} />
                {param.text}
              </span>
            ))}
          </div>
        </div>
      )}
      {(pack.overrides?.count ?? 0) > 0 && (
        <div className="st-lyr" style={{ flexDirection: 'column', gap: 6 }}>
          <span style={{ fontSize: 13, fontWeight: 500 }}>Thay mặc định văn phong · {pack.overrides!.count} mục</span>
          {(pack.overrides?.rows ?? []).map((row) => (
            <div key={row.key} style={{ display: 'flex', gap: 10, fontSize: 12.5, lineHeight: 1.5 }}>
              <Icon name="arrow-right-2" size={13} style={{ color: 'var(--color-accent)', marginTop: 3 }} />
              <span style={{ color: 'var(--color-fg)' }}>{row.value}</span>
            </div>
          ))}
        </div>
      )}
      {pack.voice && (
        <div className="st-lyr" style={{ alignItems: 'center' }}>
          <Icon name="headphone" size={17} style={{ color: 'var(--color-fg-3)' }} />
          <span style={{ flex: '1 1 auto', fontSize: 13 }}>
            <span style={{ color: 'var(--color-fg-3)' }}>Giọng đọc theo gói:</span> {pack.voice}
          </span>
          {audioHref && (
            <a href={audioHref} style={{ fontSize: 12.5, color: 'var(--color-accent)', whiteSpace: 'nowrap' }}>Xem ở Audio ›</a>
          )}
        </div>
      )}
      <div style={{
        display: 'flex', gap: 10, alignItems: 'flex-start', marginTop: 4, padding: '10px 12px', borderRadius: 12,
        background: 'var(--kit-up-bg)', border: '1px solid var(--kit-up-bd)',
      }}>
        <Icon name="info-circle" size={15} style={{ color: 'var(--kit-info-fg)', marginTop: 2 }} />
        <span style={{ fontSize: 12.5, color: 'var(--kit-up-badge-fg)', lineHeight: 1.55 }}>
          Quy tắc bạn tự dặn ở <b style={{ fontWeight: 500 }}>Quy tắc viết</b> được thay mục cảnh báo và mục định hướng của gói, nhưng không thay mục bắt sửa.
          Nội dung gói chỉ xem ở đây.
        </span>
      </div>
    </section>
  )
}

/**
 * Bản rút gọn của “Gói … gồm” cho tab Cách viết của truyện (Outline.dc.html › tab=style):
 * tên gói + 3 dòng tóm tắt + ghi chú quy tắc + liên kết Chất lượng.
 */
export function PackSummaryCard({ pack, qualityHref }: { pack: PackPreset; qualityHref?: string }) {
  const summary = packRuleSummary(pack)
  const info = packInfoMeta(pack)
  const meta = info
  return (
    <section className="card" aria-label="Gói đang dùng"
      style={{ borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 8, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5 }}>
      <b style={{ fontSize: 14, fontWeight: 600, color: 'var(--color-fg)' }}>Gói {packLabel(pack)} gồm</b>
      {(summary.layers || summary.counts) && (
        <span style={{ display: 'flex', gap: 8 }}>
          <Icon name="shield-tick" size={15} style={{ color: 'var(--color-ok)', marginTop: 2, flex: '0 0 auto' }} />
          <span>Biên tập viên chấm theo {summary.layers}{summary.counts ? ` · ${summary.counts}` : ''}</span>
        </span>
      )}
      {meta && (
        <span style={{ display: 'flex', gap: 8 }}>
          <Icon name="chart" size={15} style={{ color: 'var(--kit-info-fg)', marginTop: 2, flex: '0 0 auto' }} />
          <span>{meta}</span>
        </span>
      )}
      {pack.voice && (
        <span style={{ display: 'flex', gap: 8 }}>
          <Icon name="headphone" size={15} style={{ color: 'var(--color-fg-3)', marginTop: 2, flex: '0 0 auto' }} />
          <span>Giọng đọc theo gói: {pack.voice}</span>
        </span>
      )}
      <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
        Quy tắc bạn tự dặn ở tab Quy tắc viết được thay mục cảnh báo của gói, không thay mục bắt sửa.
      </span>
      {qualityHref && <a href={qualityHref} style={{ fontSize: 12.5, color: 'var(--color-accent)' }}>Xem các mục của gói ở Chất lượng truyện ›</a>}
    </section>
  )
}

/**
 * Lưu ngay `style` ở một cấp (chọn ô = lưu ngay, không qua thanh Lưu — Gói AL).
 * `style: undefined` = xoá ghi đè (Dùng theo dự án). PUT cả tệp của cấp đó (giữ các khoá khác),
 * kèm If-Match = version đã đọc; ghi kết quả vào cache `keys.config(scope, …)` và trả bản mới.
 */
export async function saveStyle(scope: 'project' | 'story', project: string, story: string | undefined, style: string | undefined, previous: ConfigResponse): Promise<ConfigResponse> {
  const doc: Config = { ...(previous.target.config ?? {}) }
  if (style === undefined) delete doc.style
  else doc.style = style
  const next = await putConfig(scope, configForSave(doc), project, story, previous.version)
  cache.setData(keys.config(scope, project, story), next)
  void invalidate(['config'])
  return next
}
