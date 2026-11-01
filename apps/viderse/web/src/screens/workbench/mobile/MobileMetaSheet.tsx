// Sheet "Nội dung đăng" của Bàn dựng điện thoại (artboard MMain › meta). Dùng
// đúng API của cột phải máy tính: đọc/sửa metadata, viết lại theo phần, lịch sử
// và dùng lại bản cũ. Không có dữ liệu tĩnh.
import { Suspense, lazy, useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Sheet, useEscapeLayer } from '@noverse/ui'
import { getHistory, getMetadata, mutateMetadata, type History, type MetaResponse, type Metadata, type MetadataPartName } from '../../../api/workbenchPanels'
import type { SuggestionReason } from '../../../api/suggestions'
import { ReasonPopover } from '../../../components/reason/ReasonPopover'
import { screenWorkbench } from '../../../i18n/screens'
import { metadataDescriptionBody, metadataDescriptionWithFooter } from '../panels/panelLogic'
import { mw } from './mworkbench.stylex'
const LearnSlot = lazy(() => import('../../../components/learn/LearnSlot').then((module) => ({ default: module.LearnSlot })))

const c = screenWorkbench.mobile.metaSheet
const PARTS: MetadataPartName[] = ['title', 'description', 'tags']

type Draft = Pick<Metadata, 'title' | 'description' | 'tags'>

const draftOf = (data: Metadata | null | undefined, footer?: string | null): Draft => {
  return {
    title: data?.title ?? '',
    description: metadataDescriptionBody(data?.description ?? '', footer),
    tags: data?.tags ?? [],
  }
}

/** Sheet chỉ gắn thân khi mở để mỗi lần mở đọc lại đúng video/revision. */
export function MobileMetaSheet({ open, jobId, initialView = 'edit', onClose, onRefresh }: {
  open: boolean
  jobId: string
  initialView?: 'edit' | 'rewrite' | 'history'
  onClose: () => void
  onRefresh: () => void
}) {
  return open ? <OpenMetaSheet jobId={jobId} initialView={initialView} onClose={onClose} onRefresh={onRefresh} /> : null
}

function OpenMetaSheet({ jobId, initialView, onClose, onRefresh }: { jobId: string; initialView: 'edit' | 'rewrite' | 'history'; onClose: () => void; onRefresh: () => void }) {
  const [meta, setMeta] = useState<MetaResponse | null>(null)
  const [draft, setDraft] = useState<Draft>({ title: '', description: '', tags: [] })
  const [history, setHistory] = useState<History[] | null>(null)
  const [rewrite, setRewrite] = useState(initialView === 'rewrite')
  const [parts, setParts] = useState<MetadataPartName[]>(['title'])
  const [direction, setDirection] = useState('')
  const [tag, setTag] = useState('')
  const [busy, setBusy] = useState(false)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')
  const dirty = useRef(false)
  const [reload, setReload] = useState(0)

  useEffect(() => {
    const controller = new AbortController()
    setError('')
    getMetadata(jobId, controller.signal)
      .then((value) => {
        if (controller.signal.aborted) return
        setMeta(value)
        if (!dirty.current) setDraft(draftOf(value.metadata, value.descriptionFooter))
      })
      .catch((err: unknown) => { if (!controller.signal.aborted) setError(err instanceof Error ? err.message : c.loadError) })
    if (initialView === 'history') {
      getHistory(jobId).then((value) => { if (!controller.signal.aborted) setHistory(value) }).catch((err: unknown) => {
        if (!controller.signal.aborted) setError(err instanceof Error ? err.message : c.loadError)
      })
    }
    return () => controller.abort()
  }, [jobId, reload, initialView])

  const run = async (work: () => Promise<void>) => {
    setBusy(true); setError(''); setNotice('')
    try { await work() } catch (err) { setError(err instanceof Error ? err.message : 'Không thực hiện được. Vui lòng thử lại.') } finally { setBusy(false) }
  }
  const save = () => void run(async () => {
    if (!meta) return
    const result = await mutateMetadata(jobId, '', {
      ...draft,
      description: metadataDescriptionWithFooter(draft.description, meta.descriptionFooter),
      tags: [...new Set([...draft.tags, ...(meta.alwaysTags ?? [])])],
      expectedRevision: meta.revision,
    })
    dirty.current = false
    setMeta(result)
    setDraft(draftOf(result.metadata, result.descriptionFooter))
    setHistory(null)
    setNotice(c.saved)
    onRefresh()
  })
  const loadHistory = () => void run(async () => {
    if (history) { setHistory(null); return }
    setRewrite(false)
    setHistory(await getHistory(jobId))
  })
  const restore = (at: string) => void run(async () => {
    if (!meta) return
    const result = await mutateMetadata(jobId, 'restore', { at, expectedRevision: meta.revision })
    setMeta(result); setDraft(draftOf(result.metadata, result.descriptionFooter)); setHistory(null); dirty.current = false; onRefresh()
  })
  const rewriteNow = (which: MetadataPartName[] = parts, note = direction) => void run(async () => {
    if (!meta || which.length === 0) return
    const result = await mutateMetadata(jobId, 'rewrite', { parts: which, direction: note, expectedRevision: meta.revision })
    setMeta(result); setDraft(draftOf(result.metadata, result.descriptionFooter)); setRewrite(false); dirty.current = false; onRefresh()
  })
  const copy = (text: string) => void navigator.clipboard.writeText(text).then(() => setNotice(c.copied)).catch(() => setNotice(screenWorkbench.mobile.metaCopyFailed))

  const data = meta?.metadata
  const changed = Boolean(data) && (draft.title !== data?.title || draft.description !== draftOf(data, meta?.descriptionFooter).description || draft.tags.join('\u0000') !== data?.tags.join('\u0000'))
  const always = new Set(meta?.alwaysTags ?? [])

  return <Sheet open onClose={onClose} label={c.title} size="md" height="auto">
    <div {...stylex.props(mw.sheetBody)}>
      <h2 {...stylex.props(mw.cardTitle)}>{c.title}</h2>
      {!meta && !error && <p role="status" {...stylex.props(mw.notice)}>Đang tải…</p>}
      {error && <div role="alert" {...stylex.props(mw.error)}>{error} <Button size="sm" variant="quiet" onClick={() => setReload((n) => n + 1)}>{c.retry}</Button></div>}

      {meta?.status === 'pending' && <div {...stylex.props(mw.card)}>
        <strong {...stylex.props(mw.cardTitle)}>{screenWorkbench.mobile.metaPending}</strong>
        <span {...stylex.props(mw.notice)}>{c.pending}</span>
      </div>}
      {meta?.status === 'writing' && <div role="status" {...stylex.props(mw.card)}>
        <strong {...stylex.props(mw.cardTitle)}>{screenWorkbench.mobile.metaWriting}</strong>
        <span {...stylex.props(mw.notice)}>{c.working}</span>
      </div>}
      {meta?.status === 'failed' && <div role="alert" {...stylex.props(mw.card)}>
        <strong {...stylex.props(mw.cardTitle)}>{screenWorkbench.mobile.metaFailed}</strong>
        <div {...stylex.props(mw.metaActions)}>
          <Button size="sm" variant="primary" disabled={busy} onClick={() => rewriteNow(['title', 'description', 'tags'], '')}>{screenWorkbench.mobile.retryJob}</Button>
          <a {...stylex.props(mw.moreLink)} href="#/settings">Đăng nhập lại ở Cài đặt ›</a>
        </div>
      </div>}

      {data && <>
        <div {...stylex.props(mw.metaActions)}>
          <Button size="sm" variant="ghost" disabled={busy} onClick={() => { setHistory(null); setRewrite((open) => !open) }} aria-expanded={rewrite}>{c.rewrite}</Button>
          <Button size="sm" variant="ghost" disabled={busy} onClick={loadHistory} aria-expanded={history !== null}>{c.history}</Button>
        </div>

        {rewrite && <div role="group" aria-label={c.rewrite} {...stylex.props(mw.rewriteBox)}>
          <div role="group" aria-label={c.parts.title} {...stylex.props(mw.chipRow)}>
            {PARTS.map((part) => <Button key={part} size="sm" variant={parts.includes(part) ? 'primary' : 'ghost'} aria-pressed={parts.includes(part)} onClick={() => setParts((current) => current.includes(part) ? current.filter((item) => item !== part) : [...current, part])}>{c.parts[part]}</Button>)}
          </div>
          <input {...stylex.props(mw.input)} aria-label={c.direction} value={direction} onChange={(event) => setDirection(event.target.value)} placeholder={c.directionPlaceholder} />
          <div {...stylex.props(mw.chipRow)}>{c.presets.map((text) => <Button key={text} size="sm" variant="ghost" onClick={() => setDirection(text)}>{text}</Button>)}</div>
          <span {...stylex.props(mw.notice)}>{c.rewriteNote}</span>
          <Button size="lg" variant="primary" block disabled={busy || parts.length === 0} onClick={() => rewriteNow()}>{c.rewriteSubmit(parts.length)}</Button>
        </div>}

        {history !== null && <div role="group" aria-label={c.history} {...stylex.props(mw.card)}>
          <div {...stylex.props(mw.historyRow)}>
            <span {...stylex.props(mw.historyText)}>
              <span {...stylex.props(mw.historyWho)}>{data.editedBy === 'owner' ? 'Bạn sửa' : 'Đạo diễn viết'}</span>
              <span {...stylex.props(mw.historyTitle)}>{data.title}</span>
            </span>
            <span {...stylex.props(mw.cardHint)}>{c.historyCurrent}</span>
          </div>
          {history.length === 0 && <span {...stylex.props(mw.notice)}>{c.historyEmpty}</span>}
          {history.map((item) => <div key={item.at} {...stylex.props(mw.historyRow)}>
            <span {...stylex.props(mw.historyText)}>
              <span {...stylex.props(mw.historyWho)}>{item.by === 'owner' ? 'Bạn sửa' : 'Đạo diễn viết'} · {new Date(item.at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })}</span>
              <span {...stylex.props(mw.historyTitle)}>{item.title}</span>
              <span {...stylex.props(mw.historyWhat)}>{item.parts.map((part) => c.parts[part]).join(', ')}</span>
            </span>
            <Button size="sm" variant="ghost" disabled={busy} onClick={() => restore(item.at)}>{c.reuse}</Button>
          </div>)}
        </div>}

        <div {...stylex.props(mw.field)}>
          <span {...stylex.props(mw.fieldHead)}>
            <span {...stylex.props(mw.fieldLabel)}>{c.titleField} {data.edited?.title && <span {...stylex.props(mw.edited)}>{c.edited}</span>}</span>
            <Button size="sm" variant="quiet" onClick={() => copy(draft.title)}>{c.copy}</Button>
          </span>
          <textarea aria-label={c.titleField} {...stylex.props(mw.input)} maxLength={100} value={draft.title} onChange={(event) => { dirty.current = true; setDraft((current) => ({ ...current, title: event.target.value })) }} />
          <MetaReason reason={data.reason?.title} field={c.titleField} />
          <Suspense fallback={null}><LearnSlot ownerKey={jobId} learn={meta.learn} /></Suspense>
        </div>

        <div {...stylex.props(mw.field)}>
          <span {...stylex.props(mw.fieldHead)}>
            <span {...stylex.props(mw.fieldLabel)}>{c.descriptionField} {data.edited?.description && <span {...stylex.props(mw.edited)}>{c.edited}</span>}</span>
            <Button size="sm" variant="quiet" onClick={() => copy(draft.description)}>{c.copy}</Button>
          </span>
          <textarea aria-label={c.descriptionField} {...stylex.props(mw.input, mw.textarea)} maxLength={Math.max(0, 5000 - (meta.descriptionFooter?.length ?? 0) - (meta.descriptionFooter ? 2 : 0))} value={draft.description} onChange={(event) => { dirty.current = true; setDraft((current) => ({ ...current, description: event.target.value })) }} />
          <MetaReason reason={data.reason?.description} field={c.descriptionField} />
          {meta.descriptionFooter && <div {...stylex.props(mw.card)}>
            <span {...stylex.props(mw.cardHint)}>{c.footerNote}</span>
            <p {...stylex.props(mw.promptLine)}>{meta.descriptionFooter}</p>
          </div>}
        </div>

        <div {...stylex.props(mw.field)}>
          <span {...stylex.props(mw.fieldHead)}>
            <span {...stylex.props(mw.fieldLabel)}>{c.tagsField} {data.edited?.tags && <span {...stylex.props(mw.edited)}>{c.edited}</span>}</span>
            <Button size="sm" variant="quiet" onClick={() => copy(draft.tags.join(', '))}>{c.copy}</Button>
          </span>
          <div {...stylex.props(mw.tagRow)}>
            {draft.tags.map((item, index) => <span key={`${item}-${index}`} {...stylex.props(mw.tag)}>
              {item}
              {!always.has(item) && <Button type="button" variant="icon" size="sm" aria-label={`Bỏ tag ${item}`} disabled={busy} xstyle={mw.tagRemove} onClick={() => { dirty.current = true; setDraft((current) => ({ ...current, tags: current.tags.filter((_, at) => at !== index) })) }}>×</Button>}
            </span>)}
          </div>
          <form onSubmit={(event) => { event.preventDefault(); const next = tag.trim(); if (!next) return; dirty.current = true; setDraft((current) => ({ ...current, tags: current.tags.includes(next) ? current.tags : [...current.tags, next] })); setTag('') }}>
            <input {...stylex.props(mw.input)} aria-label={c.addTag} value={tag} onChange={(event) => setTag(event.target.value)} placeholder={c.addTag} />
          </form>
          <MetaReason reason={data.reason?.tags} field={c.tagsField} />
        </div>

        <div {...stylex.props(mw.planFooter)}>
          <Button size="lg" variant="primary" block disabled={busy || !changed} onClick={save}>{c.save}</Button>
          <Button size="lg" variant="ghost" onClick={onClose}>{c.close}</Button>
        </div>
        {notice && <span role="status" {...stylex.props(mw.notice)}>{notice}</span>}
      </>}
    </div>
  </Sheet>
}

function MetaReason({ reason, field }: { reason: SuggestionReason | undefined; field: string }) {
  const [open, setOpen] = useState(false)
  useEscapeLayer(open, () => setOpen(false))
  if (!reason) return null
  return <div {...stylex.props(mw.field)}>
    <Button size="sm" variant="quiet" aria-expanded={open} onClick={() => setOpen(!open)}>{open ? screenWorkbench.mobile.metaWhyClose : screenWorkbench.mobile.metaWhy}</Button>
    {open && <ReasonPopover reason={reason} label={`${screenWorkbench.mobile.metaWhy} ${field}`} />}
  </div>
}
