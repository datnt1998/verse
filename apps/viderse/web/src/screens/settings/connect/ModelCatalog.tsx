import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, connSearchMarker, connRowMarker, connManualRowMarker, connSheetRowMarker } from '../settings.stylex'
// ModelCatalog — danh mục model của một nhà cung cấp (artboard Settings
// conn=added|models|manual, MSettings sheet=models).
//   • desktop (inline): ô tìm + lọc Đã chọn/Gợi ý/Tất cả + hàng tick + dòng
//     "Không thấy model? Thêm theo tên" + tổng kết và "Gán cho trợ lý".
//   • điện thoại (sheet): danh sách hàng tick + dòng tổng kết, không tìm/lọc
//     (MSettings sheet=models).
// Hợp đồng trang: GET models?q&filter&limit=200&cursor= → {models,total,
// nextCursor}. Hiển thị đúng MỘT trang (≤ PAGE hàng) — DOM không bao giờ quá
// PAGE dù danh mục vài trăm model; lựa chọn giữ trong bản nháp qua mọi trang.
import { useEffect, useMemo, useState } from 'react'
import { Button, Icon, Spinner } from '@noverse/ui'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { useDebounced } from '../../../lib/useDebounced'
import { checkModel, listModels, type AiModel, type AiProvider, type ModelFilter } from '../../../api/ai'
import { aiCopy, capsLine } from '../../../i18n/ai'

/** Số hàng mỗi trang — trùng limit server mặc định; DOM tối đa đúng PAGE. */
export const PAGE = 200

const c = aiCopy.connect

/** Nhãn năng lực ngắn của một model (“Chữ”, “Chữ · đọc ảnh”, “Vẽ ảnh”). */
export function capTags(caps: readonly string[]): string[] {
  return [capsLine(caps)]
}

/** Capability pill colors shared by provider catalogs and agent pickers. */
export function capStyles(caps: readonly string[]): stylex.StyleXStyles {
  return [settingsStyles.connCap, caps.includes('image') ? settingsStyles.connCapI
    : caps.includes('vision') ? settingsStyles.connCapTv : settingsStyles.connCapT]
}

const nf = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 3 })
const usd = (v: number) => `$${nf.format(v)}`

/** Dòng giá: ảnh → $/ảnh; chữ → $in / $out; gói/trên máy → chữ. */
export function priceText(p?: AiModel['price']): string {
  if (!p) return ''
  if (p.perImage) return c.perImage(usd(p.perImage))
  if (p.inPerM != null || p.outPerM != null) return c.tokens(usd(p.inPerM ?? 0), usd(p.outPerM ?? 0))
  return ''
}

export function priceLabel(model: AiModel, provider: AiProvider): string {
  if (provider.auth === 'subscription') return c.inPkg
  if (provider.auth === 'local') return c.free
  return priceText(model.price)
}

/** Nguồn của danh mục — chữ trên dòng chân (artboard: “Lấy từ OpenRouter…”). */
export function catalogSource(provider: AiProvider): string {
  switch (provider.kind) {
    case 'chatgpt': return 'Codex'
    case 'grok': return 'xAI'
    case 'gemini': return 'Google AI Studio'
    case 'openrouter': return 'OpenRouter'
    case 'openai': return 'OpenAI'
    case 'deepseek': return 'DeepSeek'
    case 'anthropic': return 'Anthropic'
    case 'xai': return 'xAI'
    case 'ollama': return 'Ollama trên máy này'
    default: return provider.baseUrl ? provider.baseUrl.replace(/^https?:\/\//, '').replace(/\/.*/, '') : provider.label
  }
}

/** Chú thích của hàng model — dữ liệu thật (năng lực + ngữ cảnh), không bịa.
 *  Model thêm tay mang đúng nhãn của artboard (“Thêm theo tên”). */
function noteOf(model: AiModel): string {
  if (model.manual) return c.manualNote
  const caps = capsLine(model.caps).toLowerCase()
  return model.contextK ? `${caps} · ${c.context(model.contextK)}` : caps
}

export function ModelCatalog({ provider, draft, onToggle, variant = 'inline', autoFilter, onAssign }: {
  provider: AiProvider
  draft: AiModel[]
  /** meta = model thật trong trang (giữ caps/context khi lưu bản nháp). */
  onToggle: (id: string, on: boolean, meta?: AiModel) => void
  variant?: 'inline' | 'sheet'
  /** conn=added mở sẵn bộ lọc Gợi ý; conn=models giữ Tất cả. */
  autoFilter?: ModelFilter
  onAssign?: () => void
}) {
  const inline = variant === 'inline'
  const [q, setQ] = useState('')
  const [filter, setFilter] = useState<ModelFilter>(autoFilter ?? 'all')
  const [page, setPage] = useState(0)
  /** cursor của trang đang mở = id cuối trang trước; cursors[i] = cursor trang i. */
  const [cursors, setCursors] = useState<(string | null)[]>([null])
  const [manualOpen, setManualOpen] = useState(false)
  const [manualName, setManualName] = useState('')
  const [manualMsg, setManualMsg] = useState<{ ok: boolean; text: string } | null>(null)
  const [manualBusy, setManualBusy] = useState(false)

  const debouncedQ = useDebounced(q.trim(), 250)
  const cursor = cursors[page] ?? null
  const models = useResource(
    keys.aiProviderModels(provider.id, debouncedQ, filter, cursor),
    ({ signal }) => listModels(provider.id, { q: debouncedQ || undefined, filter, cursor, limit: PAGE, signal }),
  )

  // Nhà cung cấp không có model nào được gợi ý (provider tự thêm) → lọc Gợi ý
  // rỗng; rơi về Tất cả một lần để danh mục không trống sau khi thêm.
  useEffect(() => {
    if (filter !== 'recommended' || !models.data) return
    if (models.data.models.length === 0 && (models.data.total > 0 || provider.catalogCount > 0)) setFilter('all')
  }, [filter, models.data])

  // Đổi tìm/lọc → về trang đầu (cursor mới không còn đúng tập).
  useEffect(() => { setPage(0); setCursors([null]) }, [debouncedQ, filter])

  // Ghép model "thêm theo tên" chưa có trong trang — vẫn tick được.
  const rows = useMemo(() => {
    const loaded = models.data?.models ?? []
    if (page > 0) return loaded
    const seen = new Set(loaded.map((m) => m.id))
    const extra = draft.filter((m) => !seen.has(m.id))
    return [...extra, ...loaded]
  }, [models.data, draft, page])

  const selectedIds = useMemo(() => new Set(draft.map((m) => m.id)), [draft])
  const total = models.data?.total ?? provider.catalogCount ?? 0
  const hasNext = models.data?.nextCursor != null
  const shown = rows.length
  const at = provider.catalogAt ? new Date(provider.catalogAt).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) : ''

  function go(next: number) {
    if (next < 0 || (next > page && !hasNext)) return
    setCursors((prev) => {
      const copy = prev.slice(0, next)
      if (next > 0 && copy.length === next) copy.push(models.data?.nextCursor ?? null)
      return copy.length ? copy : [null]
    })
    setPage(next)
  }

  async function addManual() {
    const name = manualName.trim()
    if (!name || manualBusy) return
    setManualBusy(true)
    setManualMsg(null)
    try {
      const res = await checkModel(provider.id, name)
      if (res.ok) {
        const caps = res.caps ?? []
        onToggle(name, true, { id: name, caps, contextK: res.contextK, manual: true, recommended: false })
        const ctx = res.contextK ? c.manualCtx(res.contextK) : ''
        setManualMsg({ ok: true, text: c.manualOk(name, res.ms ?? 0, ctx, capsLine(caps).toLowerCase()) })
        setManualName('')
      } else {
        setManualMsg({ ok: false, text: c.manualError(name, provider.label) })
      }
    } catch {
      setManualMsg({ ok: false, text: c.manualError(name, provider.label) })
    } finally {
      setManualBusy(false)
    }
  }

  const filterOpts: { id: ModelFilter; label: string }[] = [
    { id: 'selected', label: c.filterSelected(draft.length) },
    { id: 'recommended', label: c.filterRecommended },
    { id: 'all', label: c.filterAll },
  ]

  const pending = models.data === undefined && !models.error
  const failedList = models.error != null
  const list = inline ? (
    <div className={`conn-rows ${stylex.props(settingsStyles.connRows).className}`} role="list" aria-label={provider.label}>
      {pending && <Spinner size={16} />}
      {failedList && <span className={`conn-sub conn-sub--err ${stylex.props(settingsStyles.connSub, settingsStyles.connSubErr).className}`}>{c.catalogFail}</span>}
      {!pending && !failedList && rows.length === 0 && <span className={`conn-sub ${stylex.props(settingsStyles.connSub).className}`}>{c.catalogEmpty}</span>}
      {rows.map((m) => {
        const on = selectedIds.has(m.id)
        return (
          <label key={m.id} className={`conn-row ${stylex.props(connRowMarker, settingsStyles.connRow).className}`} role="listitem">
            {/* Ô tick NATIVE đúng artboard (accent-color đỏ, mặt trắng khi trống,
                dấu tick trắng khi chọn — CSS ép light-scheme cho riêng ô). */}
            <input
              type="checkbox"
              checked={on}
              onChange={(e) => onToggle(m.id, e.target.checked, m)}
              aria-label={m.id} {...stylex.props(settingsStyles.ConnRowInputTypeCheckbox_)}
            />
            <span className={`conn-row__body ${stylex.props(settingsStyles.connRowBody).className}`}>
              <span className={`conn-row__top ${stylex.props(settingsStyles.connRowTop).className}`}>
                <span className={`conn-row__id ${stylex.props(settingsStyles.connRowId).className}`}>{m.id}</span>
                {m.recommended && <span className={`conn-cap conn-cap--ok ${stylex.props(settingsStyles.connCap, settingsStyles.connCapOk).className}`}>{c.recommended}</span>}
              </span>
              <span className={`conn-row__note ${stylex.props(settingsStyles.connRowNote).className}`}>{noteOf(m)}</span>
            </span>
            <span className={`conn-cap ${stylex.props(capStyles(m.caps)).className}`}>{capsLine(m.caps)}</span>
            <span className={`conn-row__price ${stylex.props(settingsStyles.connRowPrice).className}`}>{priceLabel(m, provider)}</span>
          </label>
        )
      })}
    </div>
  ) : (
    // Điện thoại (artboard): ô tick lớn bên trái, mã + chú thích, pill năng lực
    // bên phải; KHÔNG cột giá (dòng chân nói đơn vị giá) và không tag Gợi ý.
    <div className={`conn-sheet-list ${stylex.props(settingsStyles.connSheetList).className}`} role="list" aria-label={provider.label}>
      {pending && <Spinner size={16} />}
      {failedList && <span className={`conn-sub conn-sub--err ${stylex.props(settingsStyles.connSub, settingsStyles.connSubErr).className}`}>{c.catalogFail}</span>}
      {!pending && !failedList && rows.length === 0 && <span className={`conn-sub ${stylex.props(settingsStyles.connSub).className}`}>{c.catalogEmpty}</span>}
      {rows.map((m) => {
        const on = selectedIds.has(m.id)
        return (
          <button
            key={m.id}
            type="button"
            role="checkbox"
            aria-checked={on}
            className={`conn-sheet-row ${stylex.props(connSheetRowMarker, settingsStyles.connSheetRow).className}`}
            onClick={() => onToggle(m.id, !on, m)}
          >
            <span className={`conn-sheet-row__check ${stylex.props(settingsStyles.connSheetRowCheck).className}`} data-on={on ? 'true' : 'false'} aria-hidden="true" />
            <span className={`conn-sheet-row__body ${stylex.props(settingsStyles.connSheetRowBody).className}`}>
              <span className={`conn-sheet-row__id ${stylex.props(settingsStyles.connSheetRowId).className}`}>{m.id}</span>
              <span className={`conn-sheet-row__note ${stylex.props(settingsStyles.connSheetRowNote).className}`}>{priceText(m.price) || noteOf(m)}</span>
            </span>
            <span className={`conn-cap ${stylex.props(capStyles(m.caps)).className}`}>{capsLine(m.caps)}</span>
          </button>
        )
      })}
    </div>
  )

  return (
    <div {...stylex.props((!(variant === 'sheet')) && settingsStyles.connCat)}>
      {inline && (
        <div className={`conn-cat__tools ${stylex.props(settingsStyles.connCatTools).className}`}>
          <label className={`conn-search ${stylex.props(connSearchMarker, settingsStyles.connSearch).className}`}>
            <Icon name="search" size={14} className="conn-ic" />
            <span className="sr-only">{c.searchLabel}</span>
            <input
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder={c.searchIn(total)} {...stylex.props(settingsStyles.ConnSearchInput)}
            />
          </label>
          <div className={`conn-filters ${stylex.props(settingsStyles.connFilters).className}`} role="radiogroup" aria-label={c.filterLabel}>
            {filterOpts.map((f) => (
              <button
                key={f.id}
                type="button"
                role="radio"
                aria-checked={filter === f.id}
                className={`conn-filter ${stylex.props(settingsStyles.connFilter).className}`}
                onClick={() => setFilter(f.id)}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>
      )}

      {list}

      {inline && (
        <>
          <span className={`conn-cat__foot ${stylex.props(settingsStyles.connCatFoot).className}`} role="status">
            {c.catalogFoot(catalogSource(provider), at, shown, total)}
            {c.catalogPriceNote}
          </span>
          {(page > 0 || hasNext) && (
            <div className={`conn-cat__pager ${stylex.props(settingsStyles.connCatPager).className}`}>
              <Button size={inline ? 'compact' : 'lg'} variant="quiet" disabled={page === 0} onClick={() => go(page - 1)}>{c.catalogPrev}</Button>
              <Button size={inline ? 'compact' : 'lg'} variant="quiet" disabled={!hasNext} onClick={() => go(page + 1)}>{c.catalogNext}</Button>
            </div>
          )}

          {manualOpen ? (
            <div className={`conn-manual ${stylex.props(settingsStyles.connManual).className}`}>
              <div className={`conn-manual__row ${stylex.props(connManualRowMarker, settingsStyles.connManualRow).className}`}>
                <label style={{ flex: 1, minWidth: 0 }}>
                  <span className="sr-only">{c.manualLabel}</span>
                  <input
                    className={`conn-input mono ${stylex.props(settingsStyles.connInput, settingsStyles.ConnManualRowInput).className}`}
                    type="text"
                    value={manualName}
                    placeholder={c.manualPlaceholder}
                    aria-label={c.manualLabel}
                    onChange={(e) => setManualName(e.target.value)}
                    onKeyDown={(e) => { if (e.key === 'Enter') void addManual() }}
                  />
                </label>
                <Button size={inline ? 'compact' : 'lg'} variant="ghost" pending={manualBusy} pendingLabel={c.manualBusy} onClick={() => void addManual()} disabled={!manualName.trim()}>
                  {c.manualCheck}
                </Button>
                <Button size={inline ? 'compact' : 'lg'} variant="quiet" onClick={() => setManualOpen(false)}>{c.manualClose}</Button>
              </div>
              {manualMsg && (
                <span {...stylex.props((!(manualMsg.ok)) && settingsStyles.connSub, (!(manualMsg.ok)) && settingsStyles.connSubErr, (manualMsg.ok) && settingsStyles.connManualOk)} role="status">
                  {manualMsg.text}
                </span>
              )}
            </div>
          ) : (
            <button type="button" className={`conn-link ${stylex.props(settingsStyles.connLink).className}`} onClick={() => setManualOpen(true)}>{c.manualOpen}</button>
          )}

          <div className={`conn-sum ${stylex.props(settingsStyles.connSum).className}`}>
            <span className={`conn-sum__text ${stylex.props(settingsStyles.connSumText, (!(draft.length)) && settingsStyles.connSumTextWarn).className}`}>
              {draft.length ? c.selectSum(draft.length) : c.selectNone}
            </span>
            {draft.length > 0 && onAssign && (
              <Button size={inline ? 'compact' : 'lg'} variant="ghost" onClick={onAssign}>
                {c.assign}<Icon name="arrow-r" size={13} />
              </Button>
            )}
          </div>
        </>
      )}
    </div>
  )
}
