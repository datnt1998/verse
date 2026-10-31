// Màn Hàng chờ — artboard Queue (state=running|paused|limited|empty) trên
// desktop và sheet Hàng chờ của MMain trên 390px. Đọc GET /api/queue qua
// cache (SSE queue.updated/clone.job.* làm mới); mọi thao tác PATCH
// /api/queue: pause/resume, Quy tắc chạy, kéo đổi thứ tự (position float),
// Lên đầu, Bỏ khỏi hàng chờ.
import { useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from 'react'
import { Dialog, Icon, toast } from '@noverse/ui'
import { ActionButton, ChannelAvatar, Orb, ScreenEmpty } from '../../components'
import { ASSISTANT_INK, assistantName, type Assistant } from '../../components/agents'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { getQueue, patchQueue, type QueuePatch, type QueueQueuedItem, type QueueView } from '../../api/clone'
import { listChannels } from '../../api/channels'
import { ApiError } from '../../api/request'
import { vi as t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { channelTint } from '../../lib/channelTint'
import { fmtDuration } from '../explore/fmt'
import './queue.css'
import { fmtEta, movePosition, quotaLine, quotaName, quotaText, sortItems, sourceLabel, viewState, waitLine } from './queueView'

const tt = t.screens.queue

const STEP_NAMES = ['download', 'backdrop', 'transcript', 'metadata', 'scenes', 'images', 'build', 'score', 'export']
const STEP_LABELS = ['Tải video', 'Chọn nền', 'Làm phụ đề', 'Nội dung đăng', 'Chia cảnh', 'Vẽ ảnh', 'Dựng', 'Đo hoàn thiện', 'Xuất']
const STEP_AGENT: Assistant[] = ['scout', 'scout', 'scout', 'director', 'artist', 'artist', 'qa', 'qa', 'qa']

/** Orb + màu chữ của trợ lý theo job.agent (fallback scout khi chuỗi lạ). */
const agentOf = (agent: string): Assistant =>
  (['scout', 'director', 'artist', 'qa'] as const).includes(agent as Assistant) ? agent as Assistant : 'scout'


/** Stepper ± của Quy tắc chạy: label + aria nhóm, mỗi bước PATCH ngay. */
function Stepper({ aria, value, min, max, fmt, onChange }: {
  aria: string
  value: number
  min: number
  max: number
  fmt?: (v: number) => string
  onChange: (v: number) => void
}) {
  return (
    <div role="group" aria-label={aria} className="vq-step">
      <button type="button" className="vq-step__btn" aria-label={`${aria} giảm`} disabled={value <= min}
        onClick={() => onChange(value - 1)}><Icon name="minus" size={14} /></button>
      <span className="num mono" aria-live="polite">{fmt ? fmt(value) : value}</span>
      <button type="button" className="vq-step__btn" aria-label={`${aria} tăng`} disabled={value >= max}
        onClick={() => onChange(value + 1)}><Icon name="add" size={14} /></button>
    </div>
  )
}

export function QueueScreen() {
  const [view, setView] = useState<QueueView | null>(null)
  const [filter, setFilter] = useState<string>('all')
  const [dragId, setDragId] = useState<string | null>(null)
  const [dropId, setDropId] = useState<string | null>(null)
  const [menuId, setMenuId] = useState<string | null>(null)
  const [editRules, setEditRules] = useState(false)
  const mobile = useMediaQuery('(max-width: 767px)')
  const patchBusy = useRef(0)

  const res = useResource<QueueView>(keys.queue(), ({ signal }) => getQueue(signal))
  const channelsRes = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  // Nạp dữ liệu từ cache vào state cục bộ (để optimistic PATCH); sự kiện
  // queue.updated/clone.job.* đẩy res.data mới vào đây.
  useEffect(() => { if (res.data) setView(res.data) }, [res.data])
  const channels = useMemo(() => new Map((channelsRes.data?.channels ?? []).map((c) => [c.id, c])), [channelsRes.data])
  const chName = (id: string) => channels.get(id)?.name || id
  const chColor = (id: string) => ({ bg: channelTint(id), ring: channelTint(id) })

  const items = useMemo(() => sortItems(view?.items ?? []), [view?.items])
  const shown = filter === 'all' ? items : items.filter((i) => i.channelId === filter)
  const filters = useMemo(() => {
    const list = new Map<string, number>()
    for (const i of items) list.set(i.channelId, (list.get(i.channelId) ?? 0) + 1)
    return [...list.entries()]
  }, [items])

  const patch = async (body: QueuePatch, optimistic?: (v: QueueView) => QueueView) => {
    const before = view
    if (before && optimistic) setView(optimistic(before))
    patchBusy.current += 1
    try {
      const fresh = await patchQueue(body)
      setView(fresh)
    } catch (err) {
      if (before) setView(before)
      toast.err(tt.toasts.failed)
      if (!(err instanceof ApiError)) console.error('patch queue failed', err)
    } finally {
      patchBusy.current -= 1
    }
  }

  const reorder = async (id: string, beforeId: string | null) => {
    const pos = movePosition(items, id, beforeId)
    if (pos === null) return
    setDropId(null); setDragId(null)
    await patch({ id, position: pos }, (v) => ({
      ...v,
      items: v.items.map((i) => (i.id === id ? { ...i, position: pos } : i)),
    }))
  }

  const onDragStart = (e: DragEvent, id: string) => {
    setDragId(id)
    e.dataTransfer.setData('text/plain', id)
    e.dataTransfer.effectAllowed = 'move'
  }
  const onDrop = async (e: DragEvent, beforeId: string | null) => {
    e.preventDefault()
    const id = e.dataTransfer.getData('text/plain') || dragId
    if (id && id !== beforeId) await reorder(id, beforeId)
    else { setDragId(null); setDropId(null) }
  }

  const state = view ? viewState(view) : 'empty'
  const quota = view ? quotaLine(view.imageQuota) : null
  const pausedTime = view?.pausedAt ? fmtEta(view.pausedAt) : null
  const autoPickNames = (channelsRes.data?.channels ?? []).filter((c) => c.autoPick).map((c) => c.name)

  const pauseBtn = view && (
    <ActionButton
      className="vq-pause"
      variant={view.paused ? 'primary' : 'outline'}
      onClick={() => void patch({ paused: !view.paused }, (v) => ({ ...v, paused: !v.paused }))}
      disabled={patchBusy.current > 0}
    >
      <Icon name={view.paused ? 'play' : 'pause'} size={mobile ? 16 : 14} />
      {view.paused ? tt.resume : mobile ? tt.pause : tt.pauseLong}
    </ActionButton>
  )

  const menu = (item: QueueQueuedItem) => menuId === item.id && (
    <div role="menu" className="vq-menu card">
      <button type="button" role="menuitem" onClick={() => { setMenuId(null); void patch({ id: item.id, action: 'front' }) }}>
        <Icon name="arrow-up" size={15} />{tt.menuTop}
      </button>
      <a role="menuitem" href={`#${paths.channel(item.channelId)}`}><Icon name="channel" size={15} />{tt.menuChannel}</a>
      {item.sourceUrl && <a role="menuitem" href={item.sourceUrl} target="_blank" rel="noreferrer"><Icon name="link" size={15} />{tt.menuSource}</a>}
      <div aria-hidden="true" className="vq-menu__sep" />
      <button type="button" role="menuitem" className="vq-menu__danger" onClick={() => { setMenuId(null); void patch({ id: item.id, action: 'remove' }, (v) => ({ ...v, items: v.items.filter((i) => i.id !== item.id) })) }}>
        <Icon name="trash" size={15} />{tt.removeItem}
      </button>
    </div>
  )

  const runningCard = (j: QueueView['running'][number], i: number) => {
    const agent = agentOf(j.agent)
    const stepIdx = Math.max(0, Math.min(8, j.stepIndex))
    return (
      <article key={j.jobId} className="vq-run card">
        <div className="vq-run__top">
          <span className="vq-run__thumb" aria-hidden="true">
            {j.thumbnailUrl && <img src={j.thumbnailUrl} alt="" />}
            <span className="num mono vq-run__dur">{fmtDuration(j.durationSec)}</span>
          </span>
          <div className="vq-run__id">
            <span className="vq-run__title" title={j.title}>{j.title}</span>
            <span className="vq-run__ch"><ChannelAvatar name={chName(j.channelId)} size={18} color={chColor(j.channelId)} /><span className="vq-run__chn">{chName(j.channelId)}{j.templateName ? ` · ${j.templateName}` : ''}</span></span>
          </div>
        </div>
        <div className="vq-run__agent">
          <Orb of={agent} size={20} live={!view?.paused} />
          <span className="vq-run__doing"><b style={{ color: ASSISTANT_INK[agent] }}>{assistantName(agent)}</b><span> · {j.now}</span></span>
          <span className="num vq-run__eta">{j.etaMinutes != null ? tt.etaLeft(Math.ceil(j.etaMinutes)) : ''}</span>
        </div>
        <div role="img" aria-label={STEP_LABELS[stepIdx]} className="vq-run__segs">
          {STEP_NAMES.map((s, k) => (
            <span key={s} className="vq-seg" data-done={k < stepIdx || undefined} data-cur={k === stepIdx || undefined} data-agent={k === stepIdx ? agent : undefined} />
          ))}
        </div>
        <div className="vq-run__foot">
          <span className="num vq-run__meta">{tt.labels.lane(i + 1)} · Bước {stepIdx + 1}/9 · {STEP_LABELS[stepIdx]}{j.startedAt ? ` · ${tt.started(fmtEta(j.startedAt))}` : ''}</span>
          {view?.paused && j.pausedAt && <span className="vq-run__note">{tt.runningNote}</span>}
          <a className="vq-open" href={`#${paths.workbench({ video: j.jobId })}`}><Icon name="workbench" size={14} />{tt.openWorkbench}</a>
        </div>
      </article>
    )
  }

  const qrow = (item: QueueQueuedItem, n: number): ReactNode => (
    <div key={item.id} role="row" className="vq-row" data-drag={dragId === item.id || undefined} data-drop={dropId === item.id || undefined}
      onDragOver={(e) => { if (dragId && dragId !== item.id) { e.preventDefault(); setDropId(item.id) } }}
      onDragLeave={() => setDropId((d) => (d === item.id ? null : d))}
      onDrop={(e) => void onDrop(e, item.id)}>
      <span className="num vq-num" role="cell">{n}</span>
      <span
        className="vq-handle" role="button" aria-label={tt.reorder.handle(item.title)} title={tt.reorder.handle(item.title)}
        draggable onDragStart={(e) => onDragStart(e, item.id)} onDragEnd={() => { setDragId(null); setDropId(null) }}
      />
      <span className="vq-thumb" aria-hidden="true">
        {item.thumbnailUrl && <img src={item.thumbnailUrl} alt="" />}
        {item.durationSec != null && <span className="num mono vq-thumb__dur">{fmtDuration(item.durationSec)}</span>}
      </span>
      <span role="cell" className="vq-title">
        <span className="vq-title__name" title={item.title}>{item.title || item.source}</span>
        <span className="vq-title__src">{sourceLabel(item)}</span>
      </span>
      <span className="vq-ch" role="cell">
        <ChannelAvatar name={chName(item.channelId)} size={22} color={chColor(item.channelId)} />
        <span className="vq-ch__name">{chName(item.channelId)}</span>
      </span>
      <span className="vq-tpl" role="cell">
        <span className="vq-tpl__name">{item.templateName || '—'}</span>
        {item.waitReason === 'no_template' && <span className="vq-tpl__warn" title={tt.reasons.waitFix}>{tt.tplFix}</span>}
      </span>
      <span role="cell" className="vq-start">
        <span className="num vq-start__eta">{view?.paused ? '—' : fmtEta(item.etaStart)}</span>
        <span className="vq-start__sub">{waitLine(item, view?.paused ?? false)}</span>
      </span>
      <span role="cell" className="vq-acts">
        <button type="button" className="vq-icon" aria-label={tt.menuTopOf(item.title)} title={tt.menuTop}
          onClick={() => void patch({ id: item.id, action: 'front' })}><Icon name="arrow-up" size={15} /></button>
        <button type="button" className="vq-icon" aria-haspopup="menu" aria-expanded={menuId === item.id} aria-label={tt.menuMore(item.title)}
          onClick={() => setMenuId((m) => (m === item.id ? null : item.id))}><Icon name="more" size={15} /></button>
      </span>
      {menu(item)}
    </div>
  )

  const rulesPanel = view && (
    <div className="vq-rules">
          <section className="card vq-card" aria-labelledby="vq-h-w">
            <div className="vq-card__row">
              <h3 id="vq-h-w" className="vq-h3">{tt.rules.lanes}</h3>
              <Stepper aria={tt.rules.lanesTitle} value={view.workers.total} min={1} max={6}
                onChange={(v) => void patch({ workers: v }, (old) => ({ ...old, workers: { ...old.workers, total: v } }))} />
            </div>
            <p className="vq-note">{tt.workersText(view.workers.used, view.workers.total)}</p>
            <div className="vq-sep" />
            <div className="vq-card__row">
              <span className="vq-sub">{tt.rules.imageSlots}<span className="vq-sub__s">{tt.imageSlotsSub}</span></span>
              <Stepper aria={tt.rules.imageSlotsTitle} value={view.imageWorkers} min={1} max={8}
                onChange={(v) => void patch({ imageWorkers: v }, (old) => ({ ...old, imageWorkers: v }))} />
            </div>
            <div className="vq-card__row">
              <span className="vq-sub">{tt.rules.rescan}<span className="vq-sub__s">{tt.rescanSub}</span></span>
              <Stepper aria={tt.rules.rescanTitle} value={view.rescanHours} min={1} max={24} fmt={tt.rules.rescanUnit}
                onChange={(v) => void patch({ rescanHours: v }, (old) => ({ ...old, rescanHours: v }))} />
            </div>
          </section>

          <section className="card vq-card" aria-labelledby="vq-h-d">
            <div className="vq-card__row"><h3 id="vq-h-d" className="vq-h3">{tt.rules.pace}</h3><a className="vq-link" href={`#${paths.channelNew()}`}>{tt.paceEdit}</a></div>
            {view.pace.map((p) => (
              <div key={p.channelId} className="vq-pace">
                <div className="vq-pace__row">
                  <ChannelAvatar name={chName(p.channelId)} size={20} color={chColor(p.channelId)} />
                  <span className="vq-pace__name">{chName(p.channelId)}</span>
                  <span className="num vq-pace__v">{p.used}/{p.limit}</span>
                </div>
                <div className="vq-bar" role="img" aria-label={`${chName(p.channelId)}: ${p.used}/${p.limit}`}>
                  <span style={{ width: `${p.limit > 0 ? Math.min(100, (p.used / p.limit) * 100) : 0}%` }} />
                </div>
              </div>
            ))}
            <span className="vq-mini">{tt.paceNote}</span>
          </section>

          {view.imageQuota.length > 0 && (
            <section className="card vq-card" aria-labelledby="vq-h-img">
              <div className="vq-card__row"><h3 id="vq-h-img" className="vq-h3">{tt.rules.quota}</h3><a className="vq-link" href={`#${paths.settings()}`}>{tt.quotaSettings}</a></div>
              {view.imageQuota.map((q) => (
                <div key={`${q.provider}-${q.role}`} className="vq-pace">
                  <div className="vq-pace__row">
                    <span className="vq-pace__name vq-pace__name--b">{quotaName(q)}</span>
                    <span className="vq-mini">{q.role === 'main' ? tt.quotaMain : tt.quotaFallback}</span>
                    <span className="num vq-pace__v">{quotaText(q)}</span>
                  </div>
                  <div className="vq-bar" role="img" aria-label={`${quotaName(q)}: ${quotaText(q)}`}>
                    <span style={{ width: `${q.limit ? Math.min(100, (q.used / q.limit) * 100) : 0}%` }} />
                  </div>
                  {q.resetsAt && <span className="num vq-mini">{tt.rules.quotaReset(fmtEta(q.resetsAt))}</span>}
                </div>
              ))}
            </section>
          )}
    </div>
  )

  // ── Mobile: sheet của MMain (backdrop + dialog, danh sách rút gọn). ──
  const mobileSheet = view && (
    <>
      <div aria-hidden="true" className="vq-backdrop" onClick={() => navigate(paths.workbench())} />
      <section role="dialog" aria-modal="true" aria-label={tt.sheetTitle} className="vq-sheet">
        <div className="vq-sheet__grip" aria-hidden="true"><span /></div>
        <div className="vq-sheet__head">
          <span className="vq-sheet__title">{tt.title}<span className="num vq-sheet__sub">{tt.sheetSub(view.running.length, items.length)}</span></span>
          {pauseBtn}
        </div>
        <div className="vq-sheet__scroll">
          {view.paused && <div role="status" className="vq-sheet__paused">{tt.banners.pausedBody(items.length)}</div>}
          {view.running.length > 0 && (
            <div className="vq-sheet__group">
              <span className="vq-sec">{tt.runningNow}</span>
              {view.running.map((j) => (
                <a key={j.jobId} href={`#${paths.workbench({ video: j.jobId })}`} className="vq-sheet__run">
                  <Orb of={agentOf(j.agent)} size={22} live={!view.paused} />
                  <span className="vq-sheet__runmeta"><span className="vq-ell vq-sheet__rt">{j.title}</span><span className="vq-ell vq-sheet__rs">{j.now}</span></span>
                  <ChannelAvatar name={chName(j.channelId)} size={26} color={chColor(j.channelId)} />
                </a>
              ))}
            </div>
          )}
          {items.length > 0 && (
            <div className="vq-sheet__group vq-sheet__group--next">
              <span className="vq-sec">{tt.nextRotation}</span>
              {items.map((item, i) => (
                <div key={item.id} className="vq-sheet__row">
                  <span className="num vq-sheet__num">{i + 1}</span>
                  <ChannelAvatar name={chName(item.channelId)} size={26} color={chColor(item.channelId)} />
                  <span className="vq-sheet__meta"><span className="vq-ell vq-sheet__rt">{item.title || item.source}</span><span className="vq-ell vq-sheet__rs">{waitLine(item, view.paused) || fmtEta(item.etaStart)}</span></span>
                </div>
              ))}
            </div>
          )}
          {state === 'empty' && <div role="status" className="vq-sheet__empty">{tt.empty.title} — {tt.empty.hint}</div>}
          {view.imageQuota.length > 0 && (
            <div className="vq-sheet__quota">
              <Icon name="gallery" size={16} />
              <span className="vq-sheet__qname">{quotaName(view.imageQuota[0])} {tt.sheetQuotaToday}</span>
              <span className="num">{view.imageQuota[0].limit == null ? `${view.imageQuota[0].used}` : `${view.imageQuota[0].used}/${view.imageQuota[0].limit}`}</span>
              {view.imageQuota[0].resetsAt && <span className="vq-sheet__qreset">· {tt.rules.quotaReset(fmtEta(view.imageQuota[0].resetsAt))}</span>}
            </div>
          )}
          <div className="vq-sheet__rules"><span>{tt.sheetRules(view.workers.total)}</span><button type="button" onClick={() => setEditRules(true)}>{tt.paceEdit}</button></div>
        </div>
      </section>
    </>
  )

  return (
    <div className="vq" data-state={state}>
      {/* Desktop */}
      <main className="vq-center">
        <div className="vq-head">
          <div className="vq-head__text">
            <h1>{tt.title}</h1>
            <p>{view?.paused ? tt.ledePaused(pausedTime) : tt.lede(view?.workers.total ?? 0)}</p>
          </div>
          {state !== 'empty' && pauseBtn}
        </div>

        {state === 'limited' && quota && (
          <div role="status" className="vq-banner vq-banner--limited">
            <Icon name="danger" size={16} />
            <span className="vq-banner__text"><b>{quota.title}</b> {quota.detail}</span>
            <a className="vq-banner__link" href={`#${paths.settings()}`}>{tt.banners.limitedLink}</a>
          </div>
        )}
        {state === 'paused' && (
          <div role="status" className="vq-banner vq-banner--paused">
            <Icon name="pause" size={16} />
            <span className="vq-banner__text">{tt.banners.pausedBody(items.length)}</span>
          </div>
        )}

        {res.status === 'error' && !view && (
          <ScreenEmpty title={tt.fail.title} hint="" action={<button type="button" className="vq-btn" onClick={() => void res.refresh()}>{tt.fail.retry}</button>} />
        )}
        {view === null && res.status !== 'error' && <ScreenEmpty title={t.common.loading} hint="" />}
        {view !== null && state === 'empty' && (
          <div role="status" className="vq-empty">
            <span className="vq-empty__orbs" aria-hidden="true"><Orb of="scout" size={22} dim /><Orb of="director" size={22} dim /><Orb of="artist" size={22} dim /><Orb of="qa" size={22} dim /></span>
            <b>{tt.empty.title}</b>
            <span className="vq-empty__hint">{tt.empty.hint}</span>
            <div className="vq-empty__acts">
              <a className="vq-btn" href={`#${paths.explore()}`}><Icon name="radar" size={16} />{tt.empty.explore}</a>
              <a className="vq-btn vq-btn--pri" href={`#${paths.new()}`}><Icon name="add" size={16} />{tt.empty.newVideo}</a>
            </div>
          </div>
        )}

        {view !== null && state !== 'empty' && (
          <>
            {view.running.length > 0 && (
              <section aria-labelledby="vq-h-run" className="vq-secrun">
                <div className="vq-sechead">
                  <h2 id="vq-h-run">{tt.runningNow}</h2>
                  <span className="vq-count num"><b>{view.running.length}</b> {tt.lanes}</span>
                  <span className="vq-sechead__hint">{view.paused ? tt.runHintPaused : tt.runHint}</span>
                </div>
        <div className="vq-runs">{view.running.map(runningCard)}</div>
              </section>
            )}

            {view.rotation.order.length > 1 && (
              <section className="card vq-rot" aria-labelledby="vq-h-rot">
                <div className="vq-rot__head"><Icon name="refresh" size={16} /><h2 id="vq-h-rot">{tt.rotation}</h2></div>
                <ol className="vq-rot__list">
                  {view.rotation.order.map((cid, i) => {
                    const skip = view.rotation.skipped.find((s) => s.channelId === cid)
                    const pace = view.pace.find((p) => p.channelId === cid)
                    return (
                      <li key={cid}>
                        <span className="vq-rot__ch" data-skip={!!skip || undefined} data-next={view.rotation.next === cid || undefined}>
                          <ChannelAvatar name={chName(cid)} size={26} color={chColor(cid)} />
                          <span className="vq-rot__meta"><b>{chName(cid)}</b><span className="num">{skip ? tt.skipLabel(waitSkip(skip.reason)) : pace ? tt.rules.channelSuffix(pace.used, pace.limit) : ''}</span></span>
                        </span>
                        {i < view.rotation.order.length - 1 && <Icon name="arrow-r" size={14} />}
                      </li>
                    )
                  })}
                  <li aria-hidden="true" className="vq-rot__loop"><Icon name="refresh" size={14} />{tt.rotationLoop}</li>
                </ol>
                <p className="vq-rot__text">{tt.rotationText}</p>
              </section>
            )}

            <section aria-labelledby="vq-h-q" className="vq-qsec">
              <div className="vq-sechead vq-sechead--q">
                <h2 id="vq-h-q">{tt.comingUp}</h2>
                {filters.length > 0 && (
                  <div role="tablist" aria-label={tt.filterAria} className="vq-tabs">
                    <button type="button" role="tab" aria-selected={filter === 'all'} onClick={() => setFilter('all')}>{tt.filterAll}<span className="num">{items.length}</span></button>
                    {filters.map(([cid, n]) => (
                      <button key={cid} type="button" role="tab" aria-selected={filter === cid} onClick={() => setFilter(cid)}>{chName(cid)}<span className="num">{n}</span></button>
                    ))}
                  </div>
                )}
                <span className="vq-draghint">{tt.dragHint}</span>
              </div>
              <div className="card vq-table" role="table" aria-label={tt.comingUp}>
                <div role="row" className="vq-row vq-row--head">
                  <span role="columnheader" className="vq-num">#</span>
                  <span className="vq-handle" aria-hidden="true" />
                  <span role="columnheader" className="vq-title" style={{ gridColumn: 'span 2' }}>{tt.colVideo}</span>
                  <span className="vq-ch" role="columnheader">{tt.colChannel}</span>
                  <span className="vq-tpl" role="columnheader">{tt.colTemplate}</span>
                  <span role="columnheader">{tt.colStart}</span>
                  <span className="sr-only" role="columnheader">{tt.colActions}</span>
                </div>
                {shown.map((item, i) => qrow(item, i + 1))}
                {shown.length === 0 && <div className="vq-norows">{tt.noRows}</div>}
                <div className="vq-foot">
                  <span>{autoPickNames.length > 0 ? tt.autoPickNote(autoPickNames.join(' · ')) : tt.autoPickOff}</span>
                </div>
              </div>
            </section>
          </>
        )}
      </main>

      {/* Sidebar Quy tắc chạy (desktop) */}
      {view !== null && (
        <aside className="vq-side" aria-label={tt.rulesAria}>
          <span className="vq-grp">{tt.rulesTitle}</span>
          {!editRules && rulesPanel}


          {view.doneToday.length > 0 && (
            <section className="vq-done" aria-labelledby="vq-h-done">
              <h3 id="vq-h-done" className="vq-done__h">{tt.doneHeading(view.doneToday.length)}</h3>
              {view.doneToday.map((d) => (
                <a key={d.jobId} className="vq-done__row" href={`#${paths.workbench({ video: d.jobId })}`}>
                  <span className="vq-done__dot" data-state={d.state} aria-hidden="true" />
                  <span className="vq-done__t">{d.title}</span>
                  <span className="num vq-done__at">{d.state === 'review' ? tt.status.review : fmtEta(d.at)}</span>
                </a>
              ))}
            </section>
          )}
        </aside>
      )}
      {mobileSheet}
      {editRules && <Dialog title={tt.rulesTitle} onClose={() => setEditRules(false)} width={420}>{rulesPanel}</Dialog>}
    </div>
  )
}

/** Nhãn "bỏ lượt" của một kênh trong vòng luân phiên. */
function waitSkip(reason: string): string {
  switch (reason) {
    case 'pace_full': return t.screens.queue.reasons.paceFull
    case 'no_template': return t.screens.queue.reasons.waitFix
    case 'paused_channel': return t.screens.queue.reasons.channelPaused
    case 'workers_busy': return t.screens.queue.reasons.workersBusy
    default: return t.screens.queue.reasons.rotation
  }
}
