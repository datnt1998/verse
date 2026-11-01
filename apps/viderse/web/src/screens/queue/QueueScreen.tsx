// Màn Hàng chờ — artboard Queue (state=running|paused|limited|empty) trên
// desktop và sheet Hàng chờ của MMain trên 390px. Đọc GET /api/queue qua
// cache (SSE queue.updated/clone.job.* làm mới); mọi thao tác PATCH
// /api/queue: pause/resume, Quy tắc chạy, kéo đổi thứ tự (position float),
// Lên đầu, Bỏ khỏi hàng chờ.
import { useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from 'react'
import { Icon, toast } from '@noverse/ui'
import { ActionButton, ChannelAvatar, Orb, ScreenEmpty } from '../../components'
import { assistantName, type Assistant } from '../../components/agents'
import { vi as t } from '../../i18n/vi'
import { paths, useRoute } from '../../router'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { channelTint } from '../../lib/channelTint'
import { fmtDuration } from '../explore/fmt'
import { MobileQueue } from './MobileQueue'
import { useQueue } from './useQueue'
import type { QueueQueuedItem, QueueView } from '../../api/clone'
import * as stylex from '@stylexjs/stylex'
import { text } from '../../styles/shared.stylex'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { fmtEta, movePosition, quotaLine, quotaName, quotaText, sourceLabel, viewState, waitLine } from './queueView'
import { AdminReadOnly } from '../../components/AdminReadOnly'

const tt = t.screens.queue

const STEP_NAMES = ['download', 'backdrop', 'transcript', 'metadata', 'scenes', 'images', 'build', 'score', 'export']
const STEP_LABELS = ['Tải video', 'Chọn nền', 'Làm phụ đề', 'Nội dung đăng', 'Chia cảnh', 'Vẽ ảnh', 'Dựng', 'Đo hoàn thiện', 'Xuất']

/** Orb + màu chữ của trợ lý theo job.agent (fallback scout khi chuỗi lạ). */
const agentOf = (agent: string): Assistant =>
  (['scout', 'director', 'artist', 'qa'] as const).includes(agent as Assistant) ? agent as Assistant : 'scout'


/** Stepper ± của Quy tắc chạy: label + aria nhóm, mỗi bước PATCH ngay. */
function Stepper({ aria, value, min, max, fmt, disabled, onChange }: {
  aria: string
  value: number
  min: number
  max: number
  fmt?: (v: number) => string
  disabled: boolean
  onChange: (v: number) => void
}) {
  return (
    <div role="group" aria-label={aria} className={`vq-step${fmt ? ' vq-step--wide' : ''} ${stylex.props(styles["vq-step"]).className}`}>
      <button type="button" className={`vq-step__btn ${stylex.props(styles["vq-step__btn"]).className}`} aria-label={`${aria} giảm`} disabled={disabled || value <= min}
        onClick={() => onChange(value - 1)}><Icon name="minus" size={14} /></button>
      <span className={`num mono ${stylex.props(styles.stepValue, !!fmt && styles.stepWide).className}`} aria-live="polite">{fmt ? fmt(value) : value}</span>
      <button type="button" className={`vq-step__btn ${stylex.props(styles["vq-step__btn"]).className}`} aria-label={`${aria} tăng`} disabled={disabled || value >= max}
        onClick={() => onChange(value + 1)}><Icon name="add" size={14} /></button>
    </div>
  )
}

export function QueueScreen() {
  const [filter, setFilter] = useState<string>('all')
  const [dragId, setDragId] = useState<string | null>(null)
  const [dropId, setDropId] = useState<string | null>(null)
  const [menuId, setMenuId] = useState<string | null>(null)
  const mobile = useMediaQuery('(max-width: 767px)')
  const queue = useQueue()
  const { view, res, channels, channelsRes, items, patch, busy, online } = queue
  const { route } = useRoute()
  const focusId = route.name === 'queue' ? route.item : undefined
  const focusRef = useRef<string | null>(null)
  useEffect(() => {
    if (!focusId || focusRef.current === focusId) return
    const row = document.getElementById(`queue-desktop-${focusId}`)
    if (row) { row.scrollIntoView?.({ block: 'center' }); row.focus(); focusRef.current = focusId }
  }, [focusId, items])
  const chName = (id: string) => channels.get(id)?.name || id
  const chColor = (id: string) => ({ bg: channelTint(id), ring: channelTint(id) })
  const shown = filter === 'all' ? items : items.filter((i) => i.channelId === filter)
  const filters = useMemo(() => {
    const list = new Map<string, number>()
    for (const i of items) list.set(i.channelId, (list.get(i.channelId) ?? 0) + 1)
    return [...list.entries()]
  }, [items])
  const removeItem = async (item: QueueQueuedItem) => {
    if (await patch({ id: item.id, action: 'remove' })) {
      toast(tt.mobile.removed(1), 'ok', { label: t.common.undo, onClick: async () => { await patch({ id: item.id, action: 'restore' }) } }, { undo: true })
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
      xstyle={[styles['vq-pause'], view.paused && !mobile && styles.pausePaused]}
      variant={view.paused && !mobile ? 'primary' : 'outline'}
      onClick={() => void patch({ paused: !view.paused }, (v) => ({ ...v, paused: !v.paused }))}
      disabled={busy || !online}
    >
      <span {...stylex.props(styles.pauseLabel, inlineLabels.content)}><Icon name={view.paused ? 'play' : 'pause'} size={16} />
      {view.paused ? tt.resume : mobile ? tt.pause : tt.pauseLong}</span>
    </ActionButton>
  )

  const menu = (item: QueueQueuedItem) => menuId === item.id && (
    <div role="menu" className={`vq-menu ${stylex.props(styles.liquidCard, styles["vq-menu"]).className}`}>
      <button type="button" role="menuitem" disabled={busy || !online || items[0]?.id === item.id} {...stylex.props(styles.menuItem)} onClick={() => { setMenuId(null); void patch({ id: item.id, action: 'front' }) }}>
        <Icon name="arrow-up" size={15} />{tt.menuTop}
      </button>
      <a role="menuitem" {...stylex.props(styles.menuItem)} href={`#${paths.channel(item.channelId)}`}><Icon name="channel" size={15} />{tt.menuChannel}</a>
      {item.sourceUrl && <a role="menuitem" {...stylex.props(styles.menuItem)} href={item.sourceUrl} target="_blank" rel="noreferrer"><Icon name="link" size={15} />{tt.menuSource}</a>}
      <div aria-hidden="true" className={`vq-menu__sep ${stylex.props(styles["vq-menu__sep"]).className}`} />
      <button type="button" role="menuitem" disabled={busy || !online} className={`vq-menu__danger ${stylex.props(styles.menuItem, styles["vq-menu__danger"]).className}`} onClick={() => { setMenuId(null); void removeItem(item) }}>
        <Icon name="trash" size={15} />{tt.removeItem}
      </button>
    </div>
  )

  const runningCard = (j: QueueView['running'][number], i: number) => {
    const agent = agentOf(j.agent)
    const stepIdx = Math.max(0, Math.min(8, j.stepIndex))
    const imageLimited = state === 'limited' && stepIdx === 5
    return (
      <article key={j.jobId} className={`vq-run ${stylex.props(styles.liquidCard, styles["vq-run"], imageLimited && styles.runLimited).className}`} data-limited={imageLimited || undefined}>
        <div className={`vq-run__top ${stylex.props(styles["vq-run__top"]).className}`}>
          <span className={`vq-run__thumb ${stylex.props(styles["vq-run__thumb"]).className}`} aria-hidden="true">
            {j.thumbnailUrl && <img {...stylex.props(styles.image)} src={j.thumbnailUrl} alt="" />}
            <span className={`num mono vq-run__dur ${stylex.props(styles["vq-run__dur"]).className}`}>{fmtDuration(j.durationSec)}</span>
          </span>
          <div className={`vq-run__id ${stylex.props(styles["vq-run__id"]).className}`}>
            <span className={`vq-run__title ${stylex.props(styles["vq-run__title"]).className}`} title={j.title}>{j.title}</span>
            <span className={`vq-run__ch ${stylex.props(styles["vq-run__ch"]).className}`}><ChannelAvatar name={chName(j.channelId)} size={18} color={chColor(j.channelId)} /><span className={`vq-run__chn ${stylex.props(styles["vq-run__chn"]).className}`}>{chName(j.channelId)}{j.templateName ? ` · ${j.templateName}` : ''}</span></span>
          </div>
        </div>
        <div className={`vq-run__agent ${stylex.props(styles["vq-run__agent"]).className}`}>
          <Orb of={agent} size={20} live={!view?.paused && !imageLimited} />
          <span className={`vq-run__doing ${stylex.props(styles["vq-run__doing"]).className}`}><b {...stylex.props(agent === 'scout' ? styles.inkScout : agent === 'director' ? styles.inkDirector : agent === 'artist' ? styles.inkArtist : styles.inkQa)}>{assistantName(agent)}</b><span {...stylex.props(styles.doingText, imageLimited && styles.doingLimited)}> · {j.now}</span></span>
          <span className={`num vq-run__eta ${stylex.props(styles["vq-run__eta"]).className}`}>{j.etaMinutes != null ? tt.etaLeft(Math.ceil(j.etaMinutes)) : ''}</span>
        </div>
        <div role="img" aria-label={STEP_LABELS[stepIdx]} className={`vq-run__segs ${stylex.props(styles["vq-run__segs"]).className}`}>
          {STEP_NAMES.map((s, k) => (
            <span key={s} className={`vq-seg ${stylex.props(styles["vq-seg"], k < stepIdx && styles.segmentDone, k === stepIdx && (agent === "scout" ? styles.segmentScout : agent === "director" ? styles.segmentDirector : agent === "artist" ? styles.segmentArtist : styles.segmentQa), k === stepIdx && imageLimited && styles.segmentLimited).className}`} data-done={k < stepIdx || undefined} data-cur={k === stepIdx || undefined} data-agent={k === stepIdx ? agent : undefined} />
          ))}
        </div>
        <div className={`vq-run__foot ${stylex.props(styles["vq-run__foot"]).className}`}>
          <span className={`num vq-run__meta ${stylex.props(styles["vq-run__meta"]).className}`}>{tt.labels.lane(i + 1)} · Bước {stepIdx + 1}/9 · {STEP_LABELS[stepIdx]}{j.startedAt ? ` · ${tt.started(fmtEta(j.startedAt))}` : ''}</span>
          {view?.paused && j.pausedAt && <span className={`vq-run__note ${stylex.props(styles["vq-run__note"]).className}`}>{tt.runningNote}</span>}
          <a className={`vq-open ${stylex.props(styles["vq-open"]).className}`} href={`#${paths.workbench({ video: j.jobId })}`}><Icon name="workbench" size={14} />{tt.openWorkbench}</a>
        </div>
      </article>
    )
  }

  const qrow = (item: QueueQueuedItem, n: number): ReactNode => (
    <div key={item.id} id={`queue-desktop-${item.id}`} tabIndex={-1} role="row" className={`vq-row ${stylex.props(styles["vq-row"], dragId === item.id && styles.rowDrag, dropId === item.id && styles.rowDrop, focusId === item.id && styles.rowDrop).className}`} data-drag={dragId === item.id || undefined} data-drop={dropId === item.id || undefined}
      onDragOver={(e) => { if (dragId && dragId !== item.id) { e.preventDefault(); setDropId(item.id) } }}
      onDragLeave={() => setDropId((d) => (d === item.id ? null : d))}
      onDrop={(e) => void onDrop(e, item.id)}>
      <span className={`num vq-num ${stylex.props(styles["vq-num"]).className}`} role="cell">{n}</span>
      <span
        className={`vq-handle ${stylex.props(styles["vq-handle"]).className}`} role="button" aria-label={tt.reorder.handle(item.title)} title={tt.reorder.handle(item.title)}
        draggable onDragStart={(e) => onDragStart(e, item.id)} onDragEnd={() => { setDragId(null); setDropId(null) }}
      />
      <span className={`vq-thumb ${stylex.props(styles["vq-thumb"]).className}`} aria-hidden="true">
        {item.thumbnailUrl && <img {...stylex.props(styles.image)} src={item.thumbnailUrl} alt="" />}
        {item.durationSec != null && <span className={`num mono vq-thumb__dur ${stylex.props(styles["vq-thumb__dur"]).className}`}>{fmtDuration(item.durationSec)}</span>}
      </span>
      <span role="cell" className={`vq-title ${stylex.props(styles["vq-title"]).className}`}>
        <span className={`vq-title__name ${stylex.props(styles["vq-title__name"]).className}`} title={item.title}>{item.title || item.source}</span>
        <span className={`vq-title__src ${stylex.props(styles["vq-title__src"]).className}`}>{sourceLabel(item)}</span>
      </span>
      <span className={`vq-ch ${stylex.props(styles["vq-ch"]).className}`} role="cell">
        <ChannelAvatar name={chName(item.channelId)} size={22} color={chColor(item.channelId)} />
        <span className={`vq-ch__name ${stylex.props(styles["vq-ch__name"]).className}`}>{chName(item.channelId)}</span>
      </span>
      <span className={`vq-tpl ${stylex.props(styles["vq-tpl"]).className}`} role="cell">
        <span className={`vq-tpl__name ${stylex.props(styles["vq-tpl__name"]).className}`}>{item.templateName || '—'}</span>
        {item.waitReason === 'no_template' && <span className={`vq-tpl__warn ${stylex.props(styles["vq-tpl__warn"]).className}`} title={tt.reasons.waitFix}>{tt.tplFix}</span>}
      </span>
      <span role="cell" className={`vq-start ${stylex.props(styles["vq-start"]).className}`}>
        <span className={`num vq-start__eta ${stylex.props(styles["vq-start__eta"]).className}`}>{view?.paused ? '—' : fmtEta(item.etaStart)}</span>
        <span className={`vq-start__sub ${stylex.props(styles["vq-start__sub"]).className}`}>{waitLine(item, view?.paused ?? false)}</span>
      </span>
      <span role="cell" className={`vq-acts ${stylex.props(styles["vq-acts"]).className}`}>
        <button type="button" className={`vq-icon ${stylex.props(styles["vq-icon"]).className}`} aria-label={tt.menuTopOf(item.title)} title={tt.menuTop}
          onClick={() => void patch({ id: item.id, action: 'front' })}><Icon name="arrow-up" size={15} /></button>
        <button type="button" className={`vq-icon ${stylex.props(styles["vq-icon"]).className}`} aria-haspopup="menu" aria-expanded={menuId === item.id} aria-label={tt.menuMore(item.title)}
          onClick={() => setMenuId((m) => (m === item.id ? null : item.id))}><Icon name="more" size={15} /></button>
      </span>
      {menu(item)}
    </div>
  )

  const rulesPanel = view && (
    <div className={`vq-rules ${stylex.props(styles["vq-rules"]).className}`}>
          <AdminReadOnly note="Quy tắc chạy áp cho cả máy nên chỉ quản trị viên được đổi. Bạn vẫn xem được cấu hình hiện tại.">
          <section className={`vq-card ${stylex.props(styles.liquidCard, styles["vq-card"]).className}`} aria-labelledby="vq-h-w">
            <div className={`vq-card__row ${stylex.props(styles["vq-card__row"]).className}`}>
              <h3 id="vq-h-w" className={`vq-h3 ${stylex.props(styles.h3).className}`}>{tt.rules.lanes}</h3>
              <Stepper aria={tt.rules.lanesTitle} value={view.workers.total} min={1} max={6} disabled={busy || !online}
                onChange={(v) => void patch({ workers: v }, (old) => ({ ...old, workers: { ...old.workers, total: v } }))} />
            </div>
            <p className={`vq-note ${stylex.props(styles["vq-note"]).className}`}>{tt.workerGuidance}</p>
            <div className={`vq-sep ${stylex.props(styles["vq-sep"]).className}`} />
            <div className={`vq-card__row ${stylex.props(styles["vq-card__row"]).className}`}>
              <span className={`vq-sub ${stylex.props(styles["vq-sub"]).className}`}>{tt.rules.imageSlots}<span className={`vq-sub__s ${stylex.props(styles["vq-sub__s"]).className}`}>{tt.imageSlotsSub}</span></span>
              <Stepper aria={tt.rules.imageSlotsTitle} value={view.imageWorkers} min={1} max={8} disabled={busy || !online}
                onChange={(v) => void patch({ imageWorkers: v }, (old) => ({ ...old, imageWorkers: v }))} />
            </div>
            <div className={`vq-card__row ${stylex.props(styles["vq-card__row"]).className}`}>
              <span className={`vq-sub ${stylex.props(styles["vq-sub"]).className}`}>{tt.rules.rescan}<span className={`vq-sub__s ${stylex.props(styles["vq-sub__s"]).className}`}>{tt.rescanSub}</span></span>
              <Stepper aria={tt.rules.rescanTitle} value={view.rescanHours} min={1} max={24} fmt={tt.rules.rescanUnit} disabled={busy || !online}
                onChange={(v) => void patch({ rescanHours: v }, (old) => ({ ...old, rescanHours: v }))} />
            </div>
          </section>
          </AdminReadOnly>

          <section className={`vq-card ${stylex.props(styles.liquidCard, styles["vq-card"]).className}`} aria-labelledby="vq-h-d">
            <div className={`vq-card__row ${stylex.props(styles["vq-card__row"]).className}`}><h3 id="vq-h-d" className={`vq-h3 ${stylex.props(styles.h3).className}`}>{tt.rules.pace}</h3>{view.pace[0] && <a className={`vq-link ${stylex.props(styles["vq-link"]).className}`} href={`#${paths.channel(view.pace[0].channelId, 'sources')}`}>{tt.paceEdit}</a>}</div>
            {view.pace.map((p) => (
              <div key={p.channelId} className={`vq-pace ${stylex.props(styles["vq-pace"]).className}`}>
                <div className={`vq-pace__row ${stylex.props(styles["vq-pace__row"]).className}`}>
                  <ChannelAvatar name={chName(p.channelId)} size={20} color={chColor(p.channelId)} />
                  <span className={`vq-pace__name ${stylex.props(styles["vq-pace__name"]).className}`}>{chName(p.channelId)}</span>
                  <span className={`num vq-pace__v ${stylex.props(styles["vq-pace__v"]).className}`}>{p.used}/{p.limit}</span>
                </div>
                <div className={`vq-bar ${stylex.props(styles["vq-bar"]).className}`} role="img" aria-label={`${chName(p.channelId)}: ${p.used}/${p.limit}`}>
                  <span {...stylex.props(styles.barFill)} style={{ width: `${p.limit > 0 ? Math.min(100, (p.used / p.limit) * 100) : 0}%` }} />
                </div>
              </div>
            ))}
            <span className={`vq-mini ${stylex.props(styles["vq-mini"]).className}`}>{tt.paceNote}</span>
          </section>

          {view.imageQuota.length > 0 && (
            <section className={`vq-card ${stylex.props(styles.liquidCard, styles["vq-card"]).className}`} aria-labelledby="vq-h-img">
              <div className={`vq-card__row ${stylex.props(styles["vq-card__row"]).className}`}><h3 id="vq-h-img" className={`vq-h3 ${stylex.props(styles.h3).className}`}>{tt.rules.quota}</h3><a className={`vq-link ${stylex.props(styles["vq-link"]).className}`} href={`#${paths.settings()}`}>{tt.quotaSettings}</a></div>
              {view.imageQuota.map((q) => (
                <div key={`${q.provider}-${q.role}`} className={`vq-pace ${stylex.props(styles["vq-pace"]).className}`}>
                  <div className={`vq-pace__row ${stylex.props(styles["vq-pace__row"]).className}`}>
                    <span className={`vq-pace__name vq-pace__name--b ${stylex.props(styles["vq-pace__name"], styles["vq-pace__name--b"]).className}`}>{quotaName(q)}</span>
                    <span className={`vq-mini ${stylex.props(styles["vq-mini"]).className}`}>{q.role === 'main' ? tt.quotaMain : tt.quotaFallback}</span>
                    <span className={`num vq-pace__v ${stylex.props(styles["vq-pace__v"]).className}`}>{quotaText(q)}</span>
                  </div>
                  <div className={`vq-bar ${stylex.props(styles["vq-bar"]).className}`} role="img" aria-label={`${quotaName(q)}: ${quotaText(q)}`}>
                    <span {...stylex.props(styles.barFill)} style={{ width: `${q.limit ? Math.min(100, (q.used / q.limit) * 100) : 0}%` }} />
                  </div>
                  {q.resetsAt && <span className={`num vq-mini ${stylex.props(styles["vq-mini"]).className}`}>{tt.rules.quotaReset(fmtEta(q.resetsAt))}</span>}
                </div>
              ))}
            </section>
          )}
    </div>
  )


  if (mobile) return <MobileQueue queue={queue} />
  return (
    <div className={`vq ${stylex.props(styles["vq"]).className}`} data-state={state}>
      <main className={`vq-center vv-wide ${stylex.props(styles["vq-center"]).className}`}>
        <div className={`vq-head ${stylex.props(styles["vq-head"]).className}`}>
          <div className={`vq-head__text ${stylex.props(styles["vq-head__text"]).className}`}>
            <h1 {...stylex.props(styles.heading)}>{tt.title}</h1>
            <p {...stylex.props(styles.lede)}>{view?.paused ? tt.ledePaused(pausedTime) : tt.lede(view?.workers.total ?? 0)}</p>
          </div>
          {state !== 'empty' && pauseBtn}
        </div>

        {state === 'limited' && quota && (
          <div role="status" className={`vq-banner vq-banner--limited ${stylex.props(styles["vq-banner"], styles["vq-banner--limited"]).className}`}>
            <Icon name="danger" size={16} />
            <span className={`vq-banner__text ${stylex.props(styles["vq-banner__text"], state === "limited" && styles.bannerLimitedText).className}`}><b>{quota.title}</b> {quota.detail}</span>
            <a className={`vq-banner__link ${stylex.props(styles["vq-banner__link"]).className}`} href={`#${paths.settings()}`}>{tt.banners.limitedLink}</a>
          </div>
        )}
        {state === 'paused' && (
          <div role="status" className={`vq-banner vq-banner--paused ${stylex.props(styles["vq-banner"], styles["vq-banner--paused"]).className}`}>
            <Icon name="pause" size={16} />
            <span className={`vq-banner__text ${stylex.props(styles["vq-banner__text"]).className}`}>{tt.mobile.paused(pausedTime)} {tt.banners.pausedBody(items.length)}</span>
          </div>
        )}

        {res.status === 'error' && !view && (
          <ScreenEmpty title={tt.fail.title} hint="" action={<button type="button" className={`vq-btn ${stylex.props(styles["vq-btn"]).className}`} onClick={() => void res.refresh().catch(() => toast.err(tt.fail.title))}>{tt.fail.retry}</button>} />
        )}
        {view === undefined && res.status !== 'error' && <ScreenEmpty title={t.common.loading} hint="" />}
        {view !== undefined && state === 'empty' && (
          <div role="status" className={`vq-empty ${stylex.props(styles["vq-empty"]).className}`}>
            <span className={`vq-empty__orbs ${stylex.props(styles["vq-empty__orbs"]).className}`} aria-hidden="true"><Orb of="scout" size={22} dim /><Orb of="director" size={22} dim /><Orb of="artist" size={22} dim /><Orb of="qa" size={22} dim /></span>
            <b {...stylex.props(styles.emptyTitle)}>{tt.empty.title}</b>
            <span className={`vq-empty__hint ${stylex.props(styles["vq-empty__hint"]).className}`}>{tt.empty.hint}</span>
            <div className={`vq-empty__acts ${stylex.props(styles["vq-empty__acts"]).className}`}>
              <a className={`vq-btn ${stylex.props(styles["vq-btn"]).className}`} href={`#${paths.explore()}`}><Icon name="radar" size={16} />{tt.empty.explore}</a>
              <a className={`vq-btn vq-btn--pri ${stylex.props(styles["vq-btn"], styles["vq-btn--pri"]).className}`} href={`#${paths.new()}`}><Icon name="add" size={16} />{tt.empty.newVideo}</a>
            </div>
          </div>
        )}

        {view !== undefined && state !== 'empty' && (
          <>
            {view.running.length > 0 && (
              <section aria-labelledby="vq-h-run" className={`vq-secrun ${stylex.props(styles["vq-secrun"]).className}`}>
                <div className={`vq-sechead ${stylex.props(styles["vq-sechead"]).className}`}>
                  <h2 {...stylex.props(styles.sectionHeading)} id="vq-h-run">{tt.runningNow}</h2>
                  <span className={`vq-count num ${stylex.props(styles["vq-count"]).className}`}><b {...stylex.props(styles.countValue)}>{view.running.length}</b> {tt.lanes}</span>
                  <span className={`vq-sechead__hint ${stylex.props(styles["vq-sechead__hint"]).className}`}>{view.paused ? tt.runHintPaused : tt.runHint}</span>
                </div>
        <div className={`vq-runs ${stylex.props(styles["vq-runs"]).className}`}>{view.running.map(runningCard)}</div>
              </section>
            )}

            {view.rotation.order.length > 0 && (
              <section className={`vq-rot ${stylex.props(styles.liquidCard, styles["vq-rot"]).className}`} aria-labelledby="vq-h-rot">
                <div className={`vq-rot__head ${stylex.props(styles["vq-rot__head"]).className}`}><Icon name="refresh" size={16} /><h2 {...stylex.props(styles.rotationHeading)} id="vq-h-rot">{tt.rotation}</h2></div>
                <ol className={`vq-rot__list ${stylex.props(styles["vq-rot__list"]).className}`}>
                  {view.rotation.order.map((cid, i) => {
                    const skip = view.rotation.skipped.find((s) => s.channelId === cid)
                    const pace = view.pace.find((p) => p.channelId === cid)
                    return (
                      <li {...stylex.props(styles.rotationItem)} key={cid}>
                        <span className={`vq-rot__ch ${stylex.props(styles["vq-rot__ch"], view.rotation.next === cid && styles.rotationNext, !!skip && styles.rotationSkip).className}`} data-skip={!!skip || undefined} data-next={view.rotation.next === cid || undefined}>
                          <ChannelAvatar name={chName(cid)} size={26} color={chColor(cid)} />
                          <span className={`vq-rot__meta ${stylex.props(styles["vq-rot__meta"]).className}`}><b {...stylex.props(styles.rotationName)}>{chName(cid)}</b><span className={`num ${stylex.props(styles.rotationMeta).className}`}>{skip ? tt.skipLabel(waitSkip(skip.reason)) : pace ? tt.rules.channelSuffix(pace.used, pace.limit) : ''}</span></span>
                        </span>
                        {i < view.rotation.order.length - 1 && <Icon name="arrow-r" size={14} />}
                      </li>
                    )
                  })}
                  <li aria-hidden="true" className={`vq-rot__loop ${stylex.props(styles.rotationItem, styles["vq-rot__loop"]).className}`}><Icon name="refresh" size={14} />{tt.rotationLoop}</li>
                </ol>
                <p className={`vq-rot__text ${stylex.props(styles["vq-rot__text"]).className}`}>{tt.rotationText}</p>
              </section>
            )}

            <section aria-labelledby="vq-h-q" className={`vq-qsec ${stylex.props(styles["vq-qsec"]).className}`}>
              <div className={`vq-sechead vq-sechead--q ${stylex.props(styles["vq-sechead"], styles["vq-sechead--q"]).className}`}>
                <h2 {...stylex.props(styles.sectionHeading)} id="vq-h-q">{tt.comingUp}</h2>
                {filters.length > 0 && (
                  <div role="tablist" aria-label={tt.filterAria} className={`vq-tabs ${stylex.props(styles["vq-tabs"]).className}`}>
                    <button type="button" role="tab" {...stylex.props(styles.tab, filter === "all" && styles.tabSelected)} aria-selected={filter === 'all'} onClick={() => setFilter('all')}>{tt.filterAll}<span className={`num ${stylex.props(styles.tabCount).className}`}>{items.length}</span></button>
                    {filters.map(([cid, n]) => (
                      <button key={cid} type="button" role="tab" {...stylex.props(styles.tab, filter === cid && styles.tabSelected)} aria-selected={filter === cid} onClick={() => setFilter(cid)}>{chName(cid)}<span className={`num ${stylex.props(styles.tabCount).className}`}>{n}</span></button>
                    ))}
                  </div>
                )}
                <span className={`vq-draghint ${stylex.props(styles["vq-draghint"]).className}`}>{tt.dragHint}</span>
              </div>
              <div className={`vq-table ${stylex.props(styles.liquidCard, styles["vq-table"]).className}`} role="table" aria-label={tt.comingUp}>
                <div role="row" className={`vq-row vq-row--head ${stylex.props(styles["vq-row"], styles["vq-row--head"]).className}`}>
                  <span role="columnheader" className={`vq-num ${stylex.props(styles["vq-num"]).className}`}>#</span>
                  <span className={`vq-handle ${stylex.props(styles["vq-handle"]).className}`} aria-hidden="true" />
                  <span role="columnheader" className={`vq-title ${stylex.props(styles["vq-title"]).className}`} style={{ gridColumn: 'span 2' }}>{tt.colVideo}</span>
                  <span className={`vq-ch ${stylex.props(styles["vq-ch"]).className}`} role="columnheader">{tt.colChannel}</span>
                  <span className={`vq-tpl ${stylex.props(styles["vq-tpl"]).className}`} role="columnheader">{tt.colTemplate}</span>
                  <span role="columnheader">{tt.colStart}</span>
                  <span {...stylex.props(text.srOnly)} role="columnheader">{tt.colActions}</span>
                </div>
                {shown.map((item, i) => qrow(item, i + 1))}
                {shown.length === 0 && <div className={`vq-norows ${stylex.props(styles["vq-norows"]).className}`}>{tt.noRows}</div>}
                <div className={`vq-foot ${stylex.props(styles["vq-foot"]).className}`}>
                  <span>{autoPickNames.length > 0 ? tt.autoPickNote(autoPickNames.join(' · ')) : tt.autoPickOff}</span>
                </div>
              </div>
            </section>
          </>
        )}
      </main>

      {/* Sidebar Quy tắc chạy (desktop) */}
      {view !== undefined && (
        <aside className={`vq-side ${stylex.props(styles["vq-side"]).className}`} aria-label={tt.rulesAria}>
          <span className={`vq-grp ${stylex.props(styles["vq-grp"]).className}`}>{tt.rulesTitle}</span>
          {rulesPanel}


          {view.doneToday.length > 0 && (
            <section className={`vq-done ${stylex.props(styles["vq-done"]).className}`} aria-labelledby="vq-h-done">
              <h3 id="vq-h-done" className={`vq-done__h ${stylex.props(styles["vq-done__h"]).className}`}>{tt.doneHeading(view.doneToday.length)}</h3>
              {view.doneToday.map((d) => (
                <a key={d.jobId} className={`vq-done__row ${stylex.props(styles["vq-done__row"]).className}`} href={`#${paths.workbench({ video: d.jobId })}`}>
                  <span className={`vq-done__dot ${stylex.props(styles["vq-done__dot"], d.state === "review" && styles.doneReview).className}`} data-state={d.state} aria-hidden="true" />
                  <span className={`vq-done__t ${stylex.props(styles["vq-done__t"]).className}`}>{d.title}</span>
                  <span className={`num vq-done__at ${stylex.props(styles["vq-done__at"]).className}`}>{d.state === 'review' ? tt.status.review : fmtEta(d.at)}</span>
                </a>
              ))}
            </section>
          )}
        </aside>
      )}
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

const styles = stylex.create({
  "vq": { "display": "flex", "flex": { "default": "1", "@media (max-width: 767px)": "0 0 auto" }, "minHeight": 0 },
  "vq-center": { "flex": "1 1 640px", "minWidth": 0, "overflowY": "auto", "padding": `${s["--vv-s-22"]} ${s["--vv-s-28"]} ${s["--vv-s-32"]}`, "display": { "default": "flex", "@media (max-width: 767px)": "none" }, "flexDirection": "column", "gap": s["--vv-s-18"] },
  "vq-head": { "display": "flex", "flexWrap": "wrap", "alignItems": "flex-end", "gap": `${s["--vv-s-12"]} ${s["--vv-s-16"]}` },
  "vq-head__text": { "flex": "1 1 380px", "minWidth": 0 },
  "heading": { "margin": 0, "fontSize": "22px", "fontWeight": 600, "letterSpacing": "-0.02em" },
  "lede": { "margin": `${s["--vv-s-4"]} 0 0`, "fontSize": "13.5px", "color": color["--color-fg-3"] },
  "vq-banner": { "display": "flex", "flexWrap": "wrap", "alignItems": "center", "gap": `${s["--vv-s-10"]} ${s["--vv-s-14"]}`, "padding": `${s["--vv-s-12"]} ${s["--vv-s-14"]}`, "borderRadius": r["--vv-r-12"], "fontSize": "13.5px", "lineHeight": 1.55 },
  "vq-banner--limited": { borderWidth: "1px", borderStyle: "solid", borderColor: `color-mix(in srgb, ${color["--color-err"]} 35%, transparent)`, backgroundColor: "var(--vv-warn-bg)", color: color["--color-err"] },
  "bannerLimitedText": { "color": "var(--vv-warn-ink)" },
  "vq-banner--paused": { flexWrap: "nowrap", gap: s["--vv-s-12"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-4"], backgroundColor: color["--color-card"], color: color["--color-fg-2"] },
  "vq-banner__text": { "flex": "1 1 420px", "minWidth": 0 },
  "vq-banner__link": { height: "30px", padding: `0 ${s["--vv-s-11"]}`, display: "flex", alignItems: "center", gap: s["--vv-s-7"], borderRadius: r["--vv-r-8"], borderWidth: "1px", borderStyle: "solid", borderColor: `color-mix(in srgb, ${color["--color-err"]} 40%, transparent)`, color: "var(--vv-warn-ink)", fontSize: "12.5px", whiteSpace: "nowrap", textDecoration: "none" },
  "vq-sechead": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-10"] },
  "vq-sechead--q": { "flexWrap": "wrap", "gap": `${s["--vv-s-8"]} ${s["--vv-s-10"]}` },
  "vq-count": { display: "inline-flex", alignItems: "center", gap: s["--vv-s-6"], height: "26px", padding: `0 ${s["--vv-s-9"]} 0 ${s["--vv-s-7"]}`, borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"], borderRadius: r["--vv-r-7"], backgroundColor: kit["--kit-surface-15"], fontSize: "12px", color: "var(--vv-fg-note)", whiteSpace: "nowrap" },
  "countValue": { "fontWeight": 500, "color": color["--color-fg"] },
  "vq-sechead__hint": { "marginLeft": "auto", "fontSize": "12px", "color": "var(--vv-fg-note)" },
  "vq-secrun": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-10"] },
  "vq-qsec": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-10"] },
  "vq-runs": { "display": "grid", "gridTemplateColumns": "repeat(auto-fit, minmax(min(400px, 100%), 1fr))", "gap": s["--vv-s-12"] },
  "vq-run": { "borderRadius": r["--vv-r-14"], "padding": s["--vv-s-14"], "display": "flex", "flexDirection": "column", "gap": s["--vv-s-12"] },
  "vq-run__top": { "display": "flex", "gap": s["--vv-s-12"], "minWidth": 0 },
  "vq-run__thumb": { position: "relative", width: "112px", height: "63px", borderRadius: r["--vv-r-8"], flex: "0 0 auto", overflow: "hidden", backgroundColor: color["--color-surface-3"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-1"] },
  "image": { "width": "100%", "height": "100%", "objectFit": "cover" },
  "vq-run__dur": { position: "absolute", right: s["--vv-s-4"], bottom: s["--vv-s-3"], fontSize: "10px", padding: `0 ${s["--vv-s-4"]}`, borderRadius: r["--vv-r-4"], backgroundColor: `color-mix(in srgb, ${color["--color-bg"]} 72%, transparent)` },
  "vq-run__id": { "flex": "1", "minWidth": 0, "display": "flex", "flexDirection": "column", "gap": s["--vv-s-5"] },
  "vq-run__title": { "fontSize": "14px", "fontWeight": 500, "lineHeight": 1.4, "display": "-webkit-box", "WebkitLineClamp": 2, "WebkitBoxOrient": "vertical", "overflow": "hidden" },
  "vq-run__ch": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-7"], "minWidth": 0, "fontSize": "12px", "color": "var(--vv-fg-note)" },
  "vq-run__chn": { "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-run__agent": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-9"], "minWidth": 0 },
  "vq-run__doing": { "flex": "1", "minWidth": 0, "fontSize": "13px", "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "doingText": { "color": color["--color-fg-2"] },
  "vq-run__eta": { "fontSize": "12px", "color": "var(--vv-fg-note)", "whiteSpace": "nowrap" },
  "vq-run__segs": { "display": "flex", "gap": s["--vv-s-3"] },
  "vq-seg": { flex: "1", height: "4px", borderRadius: r["--vv-r-2"], backgroundColor: line["--line-3"] },
  "segmentDone": { backgroundColor: kit["--kit-fg-off"] },
  "segmentScout": { backgroundColor: "var(--vv-scout-ink)" },
  "segmentDirector": { backgroundColor: "var(--vv-director-ink)" },
  "segmentArtist": { backgroundColor: "var(--vv-artist-ink)" },
  "segmentQa": { backgroundColor: "var(--vv-qa-ink)" },
  "runLimited": { "borderColor": `color-mix(in srgb, ${color["--color-err"]} 30%, transparent)` },
  "doingLimited": { "color": color["--color-err"] },
  "segmentLimited": { backgroundColor: color["--color-err"] },
  "vq-run__foot": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "minWidth": 0 },
  "vq-run__meta": { "fontSize": "12px", "color": "var(--vv-fg-note)", "whiteSpace": "nowrap", "flex": "1", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-run__note": { "fontSize": "11.5px", "color": color["--color-warn"], "whiteSpace": "nowrap" },
  "vq-open": { height: "30px", padding: `0 ${s["--vv-s-11"]}`, display: "inline-flex", alignItems: "center", gap: s["--vv-s-7"], borderRadius: r["--vv-r-8"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-4"], fontSize: "12.5px", color: color["--color-fg"], textDecoration: "none", whiteSpace: "nowrap" },
  "vq-rot": { "borderRadius": r["--vv-r-14"], "padding": `${s["--vv-s-14"]} ${s["--vv-s-16"]}`, "display": "flex", "flexDirection": "column", "gap": s["--vv-s-12"] },
  "vq-rot__head": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "color": color["--color-fg-3"] },
  "rotationHeading": { "margin": 0, "fontSize": "13.5px", "fontWeight": 600, "color": color["--color-fg"] },
  "vq-rot__list": { "margin": 0, "padding": 0, "listStyle": "none", "display": "flex", "flexWrap": "wrap", "alignItems": "center", "gap": s["--vv-s-8"] },
  "rotationItem": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "color": color["--color-fg-4"] },
  "vq-rot__ch": { height: "38px", padding: `0 ${s["--vv-s-12"]} 0 ${s["--vv-s-6"]}`, display: "flex", alignItems: "center", gap: s["--vv-s-9"], borderRadius: r["--vv-r-10"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"], backgroundColor: color["--color-surface-1"] },
  "rotationNext": { "borderColor": line["--line-4"] },
  "rotationSkip": { "opacity": 0.55 },
  "vq-rot__meta": { "display": "flex", "flexDirection": "column", "lineHeight": 1.25 },
  "rotationName": { "fontSize": "12.5px", "fontWeight": 500, "whiteSpace": "nowrap" },
  "rotationMeta": { "fontSize": "11px", "color": color["--color-fg-4"], "whiteSpace": "nowrap" },
  "vq-rot__loop": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-6"], "fontSize": "12px", "color": color["--color-fg-4"] },
  "vq-rot__text": { "margin": 0, "fontSize": "12.5px", "color": color["--color-fg-3"], "lineHeight": 1.6 },
  "vq-tabs": { display: "flex", gap: s["--vv-s-2"], padding: s["--vv-s-3"], borderRadius: r["--vv-r-9"], backgroundColor: color["--color-surface-1"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"] },
  "tab": { height: "26px", padding: `0 ${s["--vv-s-9"]}`, display: "flex", alignItems: "center", gap: s["--vv-s-6"], borderRadius: r["--vv-r-6"], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", color: color["--color-fg-3"], fontSize: "12.5px", cursor: "pointer" },
  "tabSelected": { backgroundColor: color["--color-surface-4"], color: color["--color-fg"] },
  "tabCount": { "color": color["--color-fg-4"] },
  "vq-draghint": { "marginLeft": "auto", "fontSize": "12px", "color": color["--color-fg-4"] },
  "vq-table": { "borderRadius": r["--vv-r-14"], "overflow": "visible" },
  "vq-row": { position: "relative", display: "grid", alignItems: "center", gridTemplateColumns: { "default": "24px 14px 64px minmax(0, 1fr) 196px 156px 92px 64px", "@media (min-width: 768px) and (max-width: 1380px)": "24px 14px 64px minmax(0, 1fr) 176px 88px 64px", "@media (min-width: 768px) and (max-width: 1100px)": "24px 64px minmax(0, 1fr) 88px 64px" }, gap: s["--vv-s-12"], padding: `${s["--vv-s-8"]} ${s["--vv-s-14"]}`, borderBottomWidth: { default: "1px", ":last-of-type": 0 }, borderBottomStyle: { default: "solid", ":last-of-type": "none" }, borderBottomColor: { default: "var(--vv-hover-row)", ":last-of-type": "currentColor" } },
  "vq-row--head": { "padding": `${s["--vv-s-9"]} ${s["--vv-s-14"]}`, "borderBottomColor": line["--line-1"], "fontSize": "12px", "color": "var(--vv-fg-note)" },
  "rowDrag": { "opacity": 0.45 },
  "rowDrop": { "boxShadow": `inset 0 2px 0 ${color["--color-accent"]}` },
  "vq-num": { "textAlign": "right", "fontSize": "12.5px", "color": "var(--vv-fg-note)" },
  "vq-handle": { "width": "10px", "height": "16px", "cursor": "grab", "opacity": 0.55, "backgroundImage": `radial-gradient(circle, ${color["--color-fg-3"]} 1.2px, transparent 1.4px)`, "backgroundSize": "5px 5px", "display": { "default": null, "@media (min-width: 768px) and (max-width: 1100px)": "none" }, "backgroundColor": { default: null, ":hover": color["--color-surface-3"] } },
  "vq-thumb": { position: "relative", width: "64px", height: "36px", borderRadius: r["--vv-r-6"], overflow: "hidden", backgroundColor: color["--color-surface-3"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"] },
  "vq-thumb__dur": { position: "absolute", right: s["--vv-s-3"], bottom: s["--vv-s-2"], fontSize: "9.5px", padding: `0 ${s["--vv-s-3"]}`, borderRadius: r["--vv-r-3"], backgroundColor: `color-mix(in srgb, ${color["--color-bg"]} 72%, transparent)` },
  "vq-title": { "minWidth": 0, "display": "flex", "flexDirection": "column", "gap": s["--vv-s-1"] },
  "vq-title__name": { "fontSize": "13.5px", "fontWeight": 500, "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-title__src": { "fontSize": "12px", "color": "var(--vv-fg-note)", "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-ch": { "minWidth": 0, "display": { "default": "flex", "@media (min-width: 768px) and (max-width: 1100px)": "none" }, "alignItems": "center", "gap": s["--vv-s-8"] },
  "vq-ch__name": { "fontSize": "12.5px", "color": color["--color-fg-2"], "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-tpl": { "minWidth": 0, "display": { "default": "flex", "@media (min-width: 768px) and (max-width: 1380px)": "none" }, "flexDirection": "column", "gap": s["--vv-s-1"] },
  "vq-tpl__name": { "fontSize": "12.5px", "color": color["--color-fg-2"], "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-tpl__warn": { "fontSize": "11.5px", "color": color["--color-warn"], "whiteSpace": "nowrap" },
  "vq-start": { "display": "flex", "flexDirection": "column", "lineHeight": 1.35 },
  "vq-start__eta": { "fontSize": "12.5px" },
  "vq-start__sub": { "fontSize": "11px", "color": "var(--vv-fg-note)", "whiteSpace": "nowrap" },
  "vq-acts": { "display": "flex", "justifyContent": "flex-end", "gap": s["--vv-s-2"] },
  "vq-icon": { width: "28px", height: "28px", borderRadius: r["--vv-r-7"], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: { default: "transparent", ":hover": color["--color-surface-3"] }, backgroundImage: { default: "none" }, color: { default: color["--color-fg-4"], ":hover": color["--color-fg"] }, display: "flex", alignItems: "center", justifyContent: "center", cursor: "pointer" },
  "vq-norows": { "padding": `${s["--vv-s-24"]} ${s["--vv-s-14"]}`, "textAlign": "center", "fontSize": "13px", "color": color["--color-fg-4"] },
  "vq-foot": { "padding": `${s["--vv-s-10"]} ${s["--vv-s-14"]}`, "display": "flex", "flexWrap": "wrap", "gap": `${s["--vv-s-6"]} ${s["--vv-s-16"]}`, "fontSize": "12px", "color": color["--color-fg-4"] },
  "vq-menu": { position: "absolute", right: s["--vv-s-12"], top: s["--vv-s-44"], zIndex: 5, width: "232px", padding: s["--vv-s-5"], borderRadius: r["--vv-r-11"], backgroundColor: color["--color-surface-2"], display: "flex", flexDirection: "column" },
  "menuItem": { display: "flex", alignItems: "center", gap: s["--vv-s-8"], padding: `${s["--vv-s-7"]} ${s["--vv-s-8"]}`, borderRadius: r["--vv-r-7"], fontSize: "13px", color: { default: color["--color-fg-2"], ":hover": color["--color-fg"] }, backgroundColor: { default: "transparent", ":hover": color["--color-surface-3"] }, backgroundImage: { default: "none" }, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", cursor: "pointer", textAlign: "left", textDecoration: "none" },
  "vq-menu__danger": { "color": { default: color["--color-warn"], ":hover": color["--color-warn"] } },
  "vq-menu__sep": { height: "1px", margin: `${s["--vv-s-4"]} ${s["--vv-s-6"]}`, backgroundColor: line["--line-2"] },
  "vq-empty": { "flex": "1", "minHeight": "420px", "display": "flex", "flexDirection": "column", "alignItems": "center", "justifyContent": "center", "gap": s["--vv-s-14"], "textAlign": "center" },
  "vq-empty__orbs": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "opacity": 0.85 },
  "emptyTitle": { "fontSize": "18px", "fontWeight": 600 },
  "vq-empty__hint": { "maxWidth": "440px", "fontSize": "13.5px", "color": color["--color-fg-3"], "lineHeight": 1.6 },
  "vq-empty__acts": { "display": "flex", "flexWrap": "wrap", "justifyContent": "center", "gap": s["--vv-s-8"], "marginTop": s["--vv-s-4"] },
  "vq-btn": { height: "36px", padding: `0 ${s["--vv-s-13"]}`, display: "inline-flex", alignItems: "center", gap: s["--vv-s-8"], borderRadius: r["--vv-r-10"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-1"], fontSize: "13px", color: color["--color-fg"], textDecoration: "none", backgroundColor: "transparent", backgroundImage: "none", cursor: "pointer" },
  "vq-btn--pri": { backgroundColor: color["--color-fg"], color: color["--color-bg"], fontWeight: 600, borderWidth: 0, borderStyle: "none", borderColor: "currentColor" },
  "vq-side": { "flex": "0 0 340px", "maxWidth": "340px", "minHeight": 0, "overflowY": "auto", "padding": `${s["--vv-s-22"]} ${s["--vv-s-16"]} ${s["--vv-s-24"]}`, "display": { "default": "flex", "@media (max-width: 767px)": "none" }, "flexDirection": "column", "gap": s["--vv-s-12"], "boxShadow": `inset 1px 0 0 ${line["--line-1"]}`, "paddingLeft": `calc(${s["--vv-s-16"]} + ${s["--vv-s-1"]})` },
  "vq-grp": { "padding": `0 ${s["--vv-s-4"]}`, "fontSize": "11.5px", "fontWeight": 600, "textTransform": "uppercase", "letterSpacing": "0.06em", "color": "var(--vv-fg-note)" },
  "vq-card": { "borderRadius": r["--vv-r-14"], "padding": s["--vv-s-14"], "display": "flex", "flexDirection": "column", "gap": s["--vv-s-10"] },
  "vq-card__row": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-10"] },
  "vq-sub": { "flex": "1", "minWidth": 0, "display": "flex", "flexDirection": "column", "fontSize": "13px", "fontWeight": 500 },
  "vq-sub__s": { "fontSize": "12px", "color": "var(--vv-fg-note)", "fontWeight": 400 },
  "vq-note": { "margin": 0, "fontSize": "12.5px", "color": color["--color-fg-3"], "lineHeight": 1.55 },
  "vq-sep": { height: "1px", backgroundColor: line["--line-2"] },
  "vq-link": { "fontSize": "12px", "color": { default: color["--color-fg-3"], ":hover": color["--color-fg"] }, "textDecoration": "none" },
  "vq-mini": { "fontSize": "12px", "color": "var(--vv-fg-note)" },
  "vq-step": { display: "flex", alignItems: "stretch", height: "32px", borderRadius: r["--vv-r-10"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-4"], backgroundColor: color["--color-surface-0"], overflow: "hidden" },
  "vq-step__btn": { width: "32px", borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: { default: "transparent", ":hover:not(:disabled)": color["--color-surface-3"] }, backgroundImage: { default: "none" }, color: { default: color["--color-fg"], ":hover:not(:disabled)": color["--color-fg"] }, display: "flex", alignItems: "center", justifyContent: "center", cursor: { default: "pointer", ":disabled": "default" }, opacity: { default: null, ":disabled": 0.35 } },
  "stepValue": { "minWidth": "42px", "display": "flex", "alignItems": "center", "justifyContent": "center", "boxShadow": `inset 1px 0 0 ${line["--line-3"]}, inset -1px 0 0 ${line["--line-3"]}`, "fontSize": "13px", "whiteSpace": "nowrap" },
  "stepWide": { "minWidth": "64px" },
  "vq-pace": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-6"] },
  "vq-pace__row": { "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"] },
  "vq-pace__name": { "flex": "1", "minWidth": 0, "fontSize": "12.5px", "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-pace__name--b": { "fontWeight": 500 },
  "vq-pace__v": { "fontSize": "12.5px", "color": color["--color-fg-2"] },
  "vq-bar": { height: "4px", borderRadius: r["--vv-r-2"], backgroundColor: color["--color-surface-3"], overflow: "hidden" },
  "barFill": { display: "block", height: "100%", borderRadius: "inherit", backgroundColor: color["--color-fg-3"] },
  "vq-done": { "padding": `${s["--vv-s-4"]} ${s["--vv-s-4"]} 0`, "display": "flex", "flexDirection": "column", "gap": s["--vv-s-8"] },
  "vq-done__h": { "margin": 0, "fontSize": "12.5px", "fontWeight": 500, "color": color["--color-fg-3"] },
  "vq-done__row": { display: "flex", alignItems: "center", gap: s["--vv-s-9"], padding: s["--vv-s-6"], margin: `0 calc(-1 * ${s["--vv-s-6"]})`, borderRadius: r["--vv-r-8"], fontSize: "12.5px", color: color["--color-fg"], textDecoration: "none", backgroundColor: { default: null, ":hover": color["--color-surface-2"] }, backgroundImage: { default: null } },
  "vq-done__dot": { width: "7px", height: "7px", borderRadius: "50%", backgroundColor: color["--color-ok"], flex: "0 0 auto" },
  "doneReview": { backgroundColor: color["--color-warn"] },
  "vq-done__t": { "flex": "1", "minWidth": 0, "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "vq-done__at": { "color": color["--color-fg-4"], "whiteSpace": "nowrap" },
  "vq-pause": { "height": { "default": "36px", "@media (max-width: 767px)": "40px" }, paddingBlock: 0, paddingInline: { default: s["--vv-s-13"], "@media (max-width: 767px)": s["--vv-s-14"] }, "gap": s["--vv-s-8"], "borderRadius": { "default": r["--vv-r-10"], "@media (max-width: 767px)": r["--vv-r-12"] }, "borderColor": kit["--kit-line-12"], "fontSize": { "default": "13px", "@media (max-width: 767px)": "13.5px" }, "fontWeight": 400, "lineHeight": { "default": "1.55", "@media (max-width: 767px)": "1.5" } },
  "pausePaused": { paddingBlock: 0, paddingInline: s["--vv-s-14"], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", fontWeight: 600 },
  "pauseLabel": { "gap": s["--vv-s-8"] },
  "vq-rules": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-12"] },
  "sectionHeading": { "margin": 0, "fontSize": "14px", "fontWeight": 600 },
  "h3": { "margin": 0, "flex": "1", "fontSize": "13.5px", "fontWeight": 600 },
  "liquidCard": { backgroundColor: "var(--card-bg)", borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"], boxShadow: "var(--card-shadow)" },
  inkScout: { color: 'var(--vv-scout-ink)' },
  inkDirector: { color: 'var(--vv-director-ink)' },
  inkArtist: { color: 'var(--vv-artist-ink)' },
  inkQa: { color: 'var(--vv-qa-ink)' },
})


const inlineLabels = stylex.create({
  content: { display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-8'] },
})


