// Bảng chọn model ngay trong thẻ nhà cung cấp (Settings.dc.html `.mpanel`: tìm · lọc · ô chọn ·
// ngữ cảnh · giá · nhãn trợ lý · khoá bỏ chọn khi đang dùng · Thêm theo tên · ảo hoá > 50 hàng).
// Chọn/bỏ chọn ghi vào bản nháp providers.<id>.models; Lưu ở thanh dưới của SettingsScreen.
import { useEffect, useMemo, useState, type CSSProperties, type ReactNode } from 'react'
import { listProviderModels, testConfig } from '../../../api/client'
import type { Config, ModelConfig, ModelListContextSource, ProviderConfig } from '../../../api/types'
import { useResource } from '../../../data'
import { formatContextK } from '../../../lib/modelContext'
import { Icon } from '@noverse/ui'
import { Shimmer, Spinner } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { configForSave, mergeConfig } from '../lib/config'
import type { SettingsContext } from '../types'
import {
  draftItem, itemOf, lockMessage, manualResult, noneText, pickRows, priceText, searchPlaceholder,
  selectedLine, shouldWriteContext, sourceLine, usageOf, type PickFilter, type PickItem,
} from './modelPick'

const ROW_H = 46
const LIST_MAX = 288
const OVERSCAN = 6

// Màu artboard `.mpanel`/`.mrow`/`.mres`/`.mtag` quy về token (hardcodedColors: màu mới chỉ qua var()).
const panel: CSSProperties = {
  marginLeft: 54, padding: 12, borderRadius: 14, background: 'var(--color-surface-0)',
  border: '1px solid var(--line-2)', display: 'flex', flexDirection: 'column', gap: 10,
}
const pbtn: CSSProperties = {
  height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid var(--line-4)',
  background: 'transparent', color: 'var(--color-fg)', fontSize: 12.5, display: 'flex', alignItems: 'center',
  gap: 6, whiteSpace: 'nowrap',
}
const pbtnTxt: CSSProperties = { ...pbtn, border: '1px solid transparent', color: 'var(--color-fg-3)' }
const mres = (bg: string, fg: string): CSSProperties => ({
  display: 'flex', alignItems: 'flex-start', gap: 8, padding: '8px 10px', borderRadius: 9,
  background: bg, color: fg, fontSize: 12.5, lineHeight: 1.5,
})
const RES_OK = mres('var(--kit-ok-8)', 'var(--color-ok)')
const RES_WARN = mres('var(--kit-warn-7)', 'var(--kit-warn-text)')
const RES_ERR = mres('var(--kit-errf-7)', 'var(--color-err)')
const RES_INFO = mres('var(--kit-wash-4)', 'var(--color-fg-2)')
const tag = (bg: string, fg: string): CSSProperties => ({
  height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'inline-flex',
  alignItems: 'center', whiteSpace: 'nowrap', background: bg, color: fg,
})
const TAG_USE = tag('var(--kit-acc-12)', 'var(--kit-badge-run)')
const TAG_SUG = tag('var(--kit-ok-6)', 'var(--color-ok)')
const fieldBox: CSSProperties = {
  height: 34, padding: '0 10px', display: 'flex', alignItems: 'center', gap: 8, borderRadius: 9,
  background: 'var(--color-card)', border: '1px solid var(--line-4)', color: 'var(--color-fg)',
}

function ResultRow({ tone, children }: { tone: 'ok' | 'warn' | 'err' | 'info'; children: ReactNode }) {
  const sx = tone === 'ok' ? RES_OK : tone === 'warn' ? RES_WARN : tone === 'err' ? RES_ERR : RES_INFO
  const icon = tone === 'ok' ? 'tick-circle' : tone === 'err' ? 'warning-2' : 'info-circle'
  return (
    <div role={tone === 'err' ? 'alert' : 'status'} style={sx}>
      <Icon name={icon} size={16} style={{ flex: '0 0 auto', marginTop: 2 }} />
      <span style={{ flex: '1 1 auto' }}>{children}</span>
    </div>
  )
}

/** Dấu vết api_key trong khoá cache (đổi khoá → lấy lại danh sách) mà không để lộ khoá. */
const keySeal = (provider: ProviderConfig): string => {
  const key = provider.api_key ?? provider.api_key_hint ?? ''
  let h = 0
  for (let i = 0; i < key.length; i++) h = (h * 31 + key.charCodeAt(i)) >>> 0
  return `${key.length}.${h.toString(36)}`
}

export function ModelPicker({ ctx, id, name, provider, preset, local, oauth, draftModels, intro, onEditKey }: {
  ctx: SettingsContext
  id: string
  name: string
  /** provider đã gộp (cấp toàn cục = chính draft) */
  provider: ProviderConfig
  preset: { oauth: boolean; defaultModel: string } | null
  local: boolean
  /** tài khoản OAuth đã kết nối (gói thuê bao) */
  oauth: boolean
  draftModels: ModelConfig[]
  /** vừa thêm nhà cung cấp: lời nhắc “chọn ít nhất một model” */
  intro: boolean
  onEditKey: () => void
}) {
  const selected = draftModels.map((model) => model.name).filter(Boolean)
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<PickFilter>(selected.length ? 'all' : 'sug')
  const [reload, setReload] = useState(0)
  const [manualOpen, setManualOpen] = useState(false)
  const [manualName, setManualName] = useState('')
  const [manualState, setManualState] = useState<{ pending?: boolean; kind?: 'ok' | 'warn' | 'err'; text?: string }>({})
  const [scroll, setScroll] = useState(0)

  // Cấu hình "như sẽ lưu" để khoá ô đang dùng cho trợ lý (Mặc định · vai · dự phòng).
  const merged = useMemo(() => mergeConfig(ctx.effective, ctx.draft), [ctx.effective, ctx.draft])
  const useOf = (model: string) => usageOf(merged, id, model)

  // Đang gõ khoá: chờ ngừng gõ 600 ms mới lấy lại danh sách (mỗi ký tự không thành một lần gọi nhà cung cấp).
  const seal = keySeal(provider)
  const [fetchSeal, setFetchSeal] = useState(seal)
  useEffect(() => {
    if (seal === fetchSeal) return
    const timer = setTimeout(() => setFetchSeal(seal), 600)
    return () => clearTimeout(timer)
  }, [seal, fetchSeal])
  const resource = useResource(
    ['config-models-list', id, provider.type ?? '', provider.base_url ?? '', fetchSeal, oauth, reload],
    () => listProviderModels(id, configForSave({ provider: id, providers: { [id]: provider } }), reload > 0),
  )
  const list = resource.data
  const failed = !!list?.error || !!resource.error
  const ready = !!list && !list.error && !list.unsupported

  // Model đã chọn mà danh sách không trả (thêm theo tên / catalog đã đổi) vẫn hiện để bỏ chọn được.
  const items = useMemo<PickItem[]>(() => {
    const ctxOf = new Map(draftModels.filter((m) => m.context_window).map((m) => [m.name, m.context_window!]))
    const fetched = (list?.models ?? []).map((model) => itemOf(model, ctxOf.get(model.name)))
    const extra = selected.filter((s) => !fetched.some((item) => item.name === s)).map((s) => draftItem(s, ctxOf.get(s)))
    return [...fetched, ...extra]
  }, [list, draftModels, selected.join('\n')])
  const rows = useMemo(() => pickRows(items, query, filter, selected), [items, query, filter, selected.join('\n')])
  const virtual = rows.length > 50
  const shown = virtual ? Math.min(rows.length, Math.ceil(LIST_MAX / ROW_H) + OVERSCAN * 2) : rows.length


  const writeModels = (next: ModelConfig[]) => ctx.setField(`providers.${id}.models`, next)
  const toggle = (item: PickItem) => {
    if (!ctx.canEdit) return
    const on = selected.includes(item.name)
    if (on) {
      const use = useOf(item.name)
      if (use.length) { toast.warn(lockMessage(use)); return }
      writeModels(draftModels.filter((model) => model.name !== item.name))
      return
    }
    const entry: ModelConfig = { name: item.name }
    if (shouldWriteContext(item) && item.contextWindow > 0) entry.context_window = item.contextWindow
    writeModels([...draftModels, entry])
  }

  const checkManual = async () => {
    const model = manualName.trim()
    if (!model || manualState.pending) return
    setManualState({ pending: true })
    try {
      const result = await testConfig(id, model, configForSave({ provider: id, providers: { [id]: provider } }))
      if (!result.ok) { setManualState({ kind: 'err', text: result.message || 'Không thử được model này.' }); return }
      if (!selected.includes(model)) {
        const entry: ModelConfig = { name: model }
        const source: ModelListContextSource = result.contextSource ?? 'default'
        if (source === 'provider' && result.contextWindow) entry.context_window = result.contextWindow
        writeModels([...draftModels, entry])
      }
      const message = manualResult(model, result.latencyMs, result.contextWindow ?? 0, result.contextSource, formatContextK)
      setManualState({ kind: message.kind, text: message.text })
    } catch (error) {
      setManualState({ kind: 'err', text: error instanceof Error ? error.message : 'Không thử được model này.' })
    }
  }

  const kind = oauth ? 'subscription' : local ? 'local' : 'key'
  const manualRow = ctx.canEdit && (
    <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
      <input type="text" value={manualName} aria-label={`Tên model của ${name}`} placeholder={`Tên model, ví dụ ${items[0]?.name ?? preset?.defaultModel ?? 'ten-model'}`}
        onChange={(event) => { setManualName(event.target.value); setManualState({}) }}
        onKeyDown={(event) => { if (event.key === 'Enter') void checkManual() }}
        style={{ ...fieldBox, flex: '1 1 240px', minWidth: 0, fontSize: 12.5, fontFamily: 'var(--font-mono)', outline: 'none' }} />
      <button type="button" className="hov-ghost" onClick={() => void checkManual()} disabled={!manualName.trim() || manualState.pending} style={pbtn}>
        {manualState.pending && <Spinner size={13} color="currentColor" />}Kiểm tra &amp; thêm
      </button>
      <button type="button" className="hov-ghost" onClick={() => { setManualOpen(false); setManualState({}) }} style={pbtnTxt}>Đóng</button>
    </div>
  )
  const manualResultRow = manualState.text && manualState.kind !== 'err' && (
    <ResultRow tone={manualState.kind === 'warn' ? 'warn' : 'ok'}>{manualState.text}</ResultRow>
  )


  return (
    <div role="region" aria-label={`Model của ${name}`} style={panel}>
      {resource.initialLoading && (
        <div role="status" style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '18px 6px', fontSize: 13, color: 'var(--color-fg-2)' }}>
          <Spinner size={14} /><Shimmer>Đang lấy danh sách model từ {name}…</Shimmer>
        </div>
      )}

      {!resource.initialLoading && failed && (
        <>
          <ResultRow tone="err">
            <b style={{ fontWeight: 600 }}>Không lấy được danh sách model.</b> {list?.message || (resource.error instanceof Error ? resource.error.message : 'Kiểm tra lại kết nối, hoặc thêm model theo tên nếu bạn biết chắc tên model.')}
          </ResultRow>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {!preset?.oauth && <button type="button" className="hov-ghost" onClick={onEditKey} style={pbtn}>Đổi khoá</button>}
            <button type="button" className="hov-ghost" onClick={() => setReload((n) => n + 1)} style={pbtn}><Icon name="refresh-2" size={14} />Thử lại</button>
            {ctx.canEdit && <button type="button" className="hov-ghost" onClick={() => setManualOpen(true)} style={pbtnTxt}>Thêm theo tên</button>}
          </div>
          {manualOpen && manualRow}
          {manualState.kind === 'err' && manualState.text && <ResultRow tone="err">{manualState.text}</ResultRow>}
          {manualResultRow}
        </>
      )}

      {!resource.initialLoading && !failed && list?.unsupported && (
        <>
          <ResultRow tone="info">{name} không có API lấy danh sách model. Thêm model theo tên — Noverse sẽ thử gọi model trước khi thêm.</ResultRow>
          {manualRow}
          {manualResultRow}
          {manualState.kind === 'err' && manualState.text && <ResultRow tone="err">{manualState.text}</ResultRow>}
        </>
      )}

      {!resource.initialLoading && ready && (
        <>
          {intro && selected.length === 0 && (
            <ResultRow tone="warn">Đã kết nối {name}. Chọn ít nhất một model để các trợ lý dùng được. Noverse đã lọc sẵn vài model hợp viết truyện tiếng Việt.</ResultRow>
          )}
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <label style={{ ...fieldBox, flex: '1 1 220px', minWidth: 0 }}>
              <Icon name="search-normal-1" size={14} style={{ color: 'var(--color-fg-4)' }} />
              <input type="search" value={query} aria-label={searchPlaceholder(list.total)} placeholder={searchPlaceholder(list.total)}
                onChange={(event) => setQuery(event.target.value)}
                style={{ flex: '1 1 auto', minWidth: 0, border: 'none', outline: 'none', background: 'transparent', color: 'var(--color-fg)', font: 'inherit', fontSize: 13 }} />
            </label>
            <div role="group" aria-label="Lọc model" style={{ display: 'inline-flex', gap: 2, padding: 3, borderRadius: 10, background: 'var(--color-surface-0)', border: '1px solid var(--line-2)' }}>
              {([['sel', `Đã chọn · ${selected.length}`], ['sug', 'Gợi ý'], ['all', 'Tất cả']] as [PickFilter, string][]).map(([id, label]) => (
                <button key={id} type="button" aria-pressed={filter === id} onClick={() => setFilter(id)}
                  style={{ height: 28, padding: '0 11px', border: 'none', borderRadius: 7, fontSize: 12.5, whiteSpace: 'nowrap', background: filter === id ? 'var(--color-surface-7)' : 'transparent', color: filter === id ? 'var(--color-fg)' : 'var(--color-fg-3)' }}>{label}</button>
              ))}
            </div>
          </div>
          <div role="group" aria-label={`Model của ${name}`} onScroll={(event) => setScroll(event.currentTarget.scrollTop)}
            style={{ display: 'flex', flexDirection: 'column', maxHeight: LIST_MAX, overflow: 'auto', margin: '0 -4px', padding: '0 4px' }}>
            {virtual
              ? <VirtualRows rows={rows} selected={selected} canEdit={ctx.canEdit} kind={kind} useOf={useOf} scroll={scroll} toggle={toggle} />
              : rows.map((item) => <Row key={item.name} item={item} selected={selected} canEdit={ctx.canEdit} kind={kind} useOf={useOf} toggle={toggle} />)}
            {!rows.length && <div style={{ padding: '18px 8px', fontSize: 13, color: 'var(--color-fg-3)' }}>{noneText(query, filter)}</div>}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', paddingTop: 8, borderTop: '1px solid var(--line-1)', fontSize: 12, color: 'var(--kit-fg-icon)' }}>
            <button type="button" className="hov-ghost" onClick={() => setReload((n) => n + 1)} aria-label={`Tải lại danh sách model của ${name}`}
              style={{ width: 24, height: 24, padding: 0, border: 'none', borderRadius: 6, background: 'transparent', color: 'var(--kit-fg-icon)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="refresh-2" size={13} />
            </button>
            <span style={{ flex: '1 1 auto' }}>{sourceLine({ from: name, fetchedAt: list.fetchedAt, shown, filtered: rows.length, total: list.total, hasPrice: items.some((item) => item.price) })}</span>
            {ctx.canEdit && <button type="button" className="hov-ghost" onClick={() => setManualOpen((v) => !v)} style={{ ...pbtnTxt, height: 28, color: 'var(--color-accent)' }}>Không thấy model? Thêm theo tên</button>}
          </div>
          {manualOpen && manualRow}
          {manualState.kind === 'err' && manualState.text && <ResultRow tone="err">{manualState.text}</ResultRow>}
          {manualResultRow}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12.5, color: 'var(--color-fg-3)' }}>
            <span style={{ flex: '1 1 auto' }}>{selectedLine(selected.length)}</span>
            {selected.length > 0 && (
              <button type="button" className="hov-ghost" onClick={() => ctx.goSection('models')} style={pbtn}>Gán cho trợ lý<Icon name="arrow-right-2" size={13} /></button>
            )}
          </div>
        </>
      )}
    </div>
  )
}

function Row({ item, selected, canEdit, kind, useOf, toggle, style }: {
  item: PickItem; selected: readonly string[]; canEdit: boolean; kind: 'subscription' | 'local' | 'key'
  useOf: (model: string) => string[]; toggle: (item: PickItem) => void; style?: CSSProperties
}) {
  const on = selected.includes(item.name)
  const use = useOf(item.name)
  const locked = on && use.length > 0
  return (
    <button type="button" role="checkbox" aria-checked={on} aria-disabled={locked || !canEdit || undefined}
      title={locked ? lockMessage(use) : undefined} onClick={() => toggle(item)}
      style={{
        display: 'grid', gridTemplateColumns: '18px minmax(0,1fr) auto auto', gap: 10, alignItems: 'center',
        minHeight: ROW_H, padding: '6px 8px', borderRadius: 9, border: 'none', background: 'transparent',
        color: 'var(--color-fg)', font: 'inherit', textAlign: 'left', cursor: locked || !canEdit ? 'default' : 'pointer',
        width: '100%', ...style,
      }}
      className="st-mrow">
      <span aria-hidden="true" style={{
        width: 16, height: 16, borderRadius: 5, display: 'flex', alignItems: 'center', justifyContent: 'center',
        border: on ? 'none' : '1.5px solid var(--line-5)',
        background: on ? (locked ? 'color-mix(in srgb, var(--kit-switch-on) 45%, transparent)' : 'var(--kit-switch-on)') : 'transparent',
      }}>
        {on && <svg viewBox="0 0 12 12" fill="none" style={{ width: 11, height: 11 }}><path d="M2.5 6.2 5 8.6l4.6-5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>}
      </span>
      <span style={{ minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.35 }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 8, minWidth: 0 }}>
          <span className="mono" style={{ fontSize: 12.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.name}</span>
          {use.map((label) => <span key={label} style={TAG_USE}>{label}</span>)}
          {!use.length && item.recommended && <span style={TAG_SUG}>Gợi ý</span>}
        </span>
        {item.subtitle && <span style={{ fontSize: 12, color: 'var(--kit-fg-icon)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{item.subtitle}</span>}
      </span>
      <span className="num" title="Ngữ cảnh" style={{
        flex: '0 0 auto', height: 20, padding: '0 7px', borderRadius: 6, background: 'var(--kit-wash-5)',
        color: 'var(--color-fg-3)', fontSize: 11.5, display: 'flex', alignItems: 'center',
      }}>{item.contextWindow ? formatContextK(item.contextWindow) : 'chưa rõ'}</span>
      <span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-4)', whiteSpace: 'nowrap', minWidth: 92, textAlign: 'right' }}>{priceText(item.price, kind)}</span>
    </button>
  )
}

/** > 50 hàng: chỉ render phần nhìn thấy (artboard “đang hiện 9 trong 312”). */
function VirtualRows({ rows, selected, canEdit, kind, useOf, scroll, toggle }: {
  rows: PickItem[]; selected: readonly string[]; canEdit: boolean; kind: 'subscription' | 'local' | 'key'
  useOf: (model: string) => string[]; scroll: number; toggle: (item: PickItem) => void
}) {
  const start = Math.max(0, Math.floor(scroll / ROW_H) - OVERSCAN)
  const end = Math.min(rows.length, Math.ceil((scroll + LIST_MAX) / ROW_H) + OVERSCAN)
  return (
    <div style={{ height: rows.length * ROW_H, position: 'relative' }}>
      {rows.slice(start, end).map((item, index) => (
        <Row key={item.name} item={item} selected={selected} canEdit={canEdit} kind={kind} useOf={useOf} toggle={toggle}
          style={{ position: 'absolute', top: (start + index) * ROW_H, left: 0, right: 0 }} />
      ))}
    </div>
  )
}
