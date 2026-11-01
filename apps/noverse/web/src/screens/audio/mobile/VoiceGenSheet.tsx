// §AK — ngăn Tạo giọng · Tập N (MVoiceGen.dc.html): thay hai sheet cũ (hàng đợi + chọn chương).
// Thứ tự: dải trạng thái → Cần làm (chọn sẵn) → Đang tạo → Đã có giọng (thu gọn) → Cách đọc → tự tạo → chân một nút.
import { useEffect, useMemo, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { controlAudioQueue, enqueueAudioChapters, prioritizeAudioChapter } from '../../../api/client'
import type { AudioChapter, AudioQueueJob, AudioSettings } from '../../../api/types'
import { keys, useAction } from '../../../data'
import { t } from '../../../i18n/vi'
import { charInitials } from '../../../lib/cast'
import { BottomSheet, useMobileToast } from '../../../mobile/primitives'
import { color, kit, line, tint } from '@noverse/ui/tokens.stylex'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { CharAvatar } from '../../../ui/CharAvatar'
import { Switch } from '@noverse/ui'
import { toggleNotify } from '../../../shell/jobs/store'
import { useJobSelection } from '../../../shell/jobs/useJobs'
import { formatClock } from '../logic'
import { speakerTint } from '../ui'
import type { AudioData } from '../useAudio'
import { errText } from '../useAudio'
import { genAction, genCounts, genMeta, genQueueInput, genRows, genStrip, genSummary, genTag, pickEtaSec, preselect, rowEtaSec, type GenRow } from '../genWork'
import { Ring } from './Ring'

const c = t.audio.generation
const qualities = ['fast', 'standard', 'high'] as const
const s = stylex.create({
  root: { display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0, color: color['--color-fg'] },
  head: { display: 'flex', alignItems: 'center', gap: 10, padding: '2px 16px 8px', flexShrink: 0 },
  title: { flexGrow: 1, fontSize: 18, fontWeight: 600, lineHeight: 1.3 },
  close: { width: 36, height: 36, borderRadius: '50%', borderWidth: 0, backgroundColor: line['--line-2'], color: color['--color-fg'], display: 'flex', alignItems: 'center', justifyContent: 'center' },
  scroll: { overflowY: 'auto', scrollbarWidth: 'none', flexGrow: 1, minHeight: 0, display: 'flex', flexDirection: 'column', padding: '0 16px 18px' },
  ov: { display: 'flex', gap: 3, height: 8, flexShrink: 0 },
  cell: { flexGrow: 1, flexBasis: 0, borderRadius: 3 },
  cellDone: { backgroundColor: kit['--kit-ok-55'] },
  cellStale: { backgroundColor: color['--color-warn'] },
  cellNew: { backgroundColor: kit['--kit-wash-12'] },
  cellErr: { backgroundColor: color['--color-err'] },
  cellQueue: { backgroundColor: kit['--kit-acc-25'] },
  cellRun: { backgroundColor: kit['--kit-acc-20'] },
  // §AT — `.ov span.hold` (MVoiceGen.dc.html): ô xám trung tính cho chương chờ máy đọc.
  cellHold: { backgroundColor: kit['--kit-wash-25'] },
  cellRunFill: { display: 'block', height: '100%', borderRadius: 3, backgroundColor: color['--color-accent'] },
  leg: { display: 'flex', flexWrap: 'wrap', gap: '4px 12px', fontSize: 12.5, color: color['--color-fg-3'], padding: '8px 0 4px' },
  swatch: { display: 'inline-block', width: 8, height: 8, borderRadius: 2, marginRight: 5 },
  swOk: { backgroundColor: color['--color-ok'] }, swWarn: { backgroundColor: color['--color-warn'] }, swMuted: { backgroundColor: kit['--kit-wash-30'] }, swRun: { backgroundColor: color['--color-accent'] },
  swHold: { backgroundColor: kit['--kit-wash-30'] },
  ghead: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '2px 8px', padding: '16px 4px 8px', fontSize: 12.5, color: kit['--kit-fg-meta'], flexShrink: 0 },
  gtitle: { fontSize: 12, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: color['--color-fg-2'] },
  gtitleWarn: { color: color['--color-warn'] }, gtitleRun: { color: kit['--kit-badge-run'] },
  gtitleHold: { color: color['--color-fg-2'] },
  gnote: { order: 3, flexBasis: '100%' },
  glink: { marginLeft: 'auto', height: 30, padding: '0 10px', borderWidth: 0, borderRadius: 8, backgroundColor: 'transparent', color: color['--color-accent'], fontSize: 13 },
  gbar: { display: 'flex', gap: 8, padding: '0 0 8px' },
  grp: { borderRadius: 16, backgroundColor: color['--color-card'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], overflow: 'hidden', flexShrink: 0 },
  grpWarn: { borderColor: kit['--kit-warn-28'] },
  row: { display: 'flex', alignItems: 'center', gap: 12, minHeight: 62, padding: '10px 12px 10px 14px', width: '100%', borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left' },
  rowBorder: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'] },
  dim: { opacity: .55 },
  text: { flexGrow: 1, flexBasis: 0, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4, lineHeight: 1.3 },
  tt: { fontSize: 15, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  mt: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6, fontSize: 12.5, color: kit['--kit-fg-meta'] },
  tg: { height: 22, padding: '0 8px', borderRadius: 7, fontSize: 12, display: 'inline-flex', alignItems: 'center', whiteSpace: 'nowrap', backgroundColor: line['--line-1'], color: color['--color-fg-2'] },
  tgWarn: { backgroundColor: kit['--kit-warn-14'], color: color['--color-warn'] },
  tgRun: { backgroundColor: kit['--kit-acc-16'], color: kit['--kit-badge-run'] },
  tgErr: { backgroundColor: kit['--kit-errf-14'], color: kit['--kit-err-soft'] },
  ico: { width: 32, height: 32, borderRadius: '50%', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  icoDone: { backgroundColor: kit['--kit-ok-12'], color: color['--color-ok'] },
  icoStale: { backgroundColor: kit['--kit-warn-14'], color: color['--color-warn'] },
  icoErr: { backgroundColor: kit['--kit-errf-14'], color: kit['--kit-err-soft'] },
  icoNew: { borderWidth: 1.5, borderStyle: 'dashed', borderColor: kit['--kit-wash-22'] },
  icoQueue: { borderWidth: 1.5, borderStyle: 'dashed', borderColor: kit['--kit-acc-45'] },
  // §AT — `.ico.hold` (MVoiceGen.dc.html): viền đứt xám, đồng hồ; không đỏ, không phải lỗi.
  icoHold: { borderWidth: 1.5, borderStyle: 'dashed', borderColor: kit['--kit-wash-22'], color: kit['--kit-fg-meta'] },
  // §AT — `.vmnote`: ghi chú máy đọc trong nhóm chờ, trung tính.
  vm: { display: 'flex', alignItems: 'flex-start', gap: 10, padding: '10px 12px', margin: '0 0 8px', borderRadius: 12, backgroundColor: kit['--kit-wash-4'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'], fontSize: 13, color: kit['--kit-fg-meta'], lineHeight: 1.5, flexShrink: 0 },
  vmDot: { flexShrink: 0, width: 8, height: 8, margin: '6px 3px 0', borderRadius: '50%', backgroundColor: kit['--kit-wash-30'] },
  vmText: { flexGrow: 1, flexBasis: 0, minWidth: 0 },
  box: { width: 24, height: 24, borderRadius: 7, borderWidth: 1.5, borderStyle: 'solid', borderColor: kit['--kit-wash-28'], flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  boxOn: { backgroundColor: color['--color-accent'], borderColor: color['--color-accent'], color: color['--color-bg'] },
  boxOnWarn: { backgroundColor: color['--color-warn'], borderColor: color['--color-warn'] },
  chp: { height: 32, padding: '0 12px', borderRadius: 999, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-12'], backgroundColor: 'transparent', color: color['--color-fg'], fontSize: 13, whiteSpace: 'nowrap', flexShrink: 0, display: 'inline-flex', alignItems: 'center', gap: 6 },
  chpOn: { borderColor: kit['--kit-acc-50'], backgroundColor: kit['--kit-acc-12'], color: kit['--kit-mark-fg'] },
  pct: { fontSize: 14, fontWeight: 600, color: kit['--kit-badge-run'], flexShrink: 0 },
  qg: { display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 4, padding: 4, borderRadius: 14, backgroundColor: color['--color-card'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], marginTop: 8, flexShrink: 0 },
  qcell: { height: 56, borderWidth: 0, borderRadius: 10, backgroundColor: 'transparent', color: color['--color-fg-2'], fontSize: 13, whiteSpace: 'nowrap', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2 },
  qcellOn: { backgroundColor: color['--color-surface-7'], color: color['--color-fg'], fontWeight: 500 },
  qmeta: { fontSize: 11.5, color: kit['--kit-fg-meta'], fontWeight: 400 },
  note: { margin: '8px 14px 0', fontSize: 12.5, color: kit['--kit-fg-meta'], lineHeight: 1.5, flexShrink: 0 },
  footer: { flexShrink: 0, padding: '10px 16px 28px', backgroundColor: color['--color-surface-3'], borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'], display: 'flex', flexDirection: 'column', gap: 8 },
  sum: { fontSize: 13, color: color['--color-fg-3'], textAlign: 'center', lineHeight: 1.45 },
  cta: { height: 50, borderRadius: 14, borderWidth: 0, backgroundColor: color['--color-fg'], color: color['--color-bg'], fontSize: 16, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8, width: '100%' },
  ctaWarn: { backgroundColor: color['--color-warn'], color: kit['--kit-warn-on'] },
  ctaGhost: { backgroundColor: line['--line-3'], color: color['--color-fg'] },
})

function VoiceRow({ data, onVoice }: { data: AudioData; onVoice: () => void }) {
  const count = data.cast?.mode === 'single' ? 1 : data.cast?.speakers.length ?? 0
  return <div {...stylex.props(s.grp)}>
    <button {...stylex.props(s.row)} style={{ minHeight: 58 }} onClick={onVoice}>
      <span style={{ display: 'flex', flexShrink: 0 }}>
        {data.cast?.speakers.slice(0, data.cast.mode === 'single' ? 1 : 4).map((v, i) =>
          <CharAvatar key={v.id} name={v.name} ini={v.ini} size={32} style={{ fontSize: 11, fontWeight: 600, marginLeft: i ? -8 : 0, border: '2px solid var(--color-card)', background: 'var(--color-surface-6)', color: 'var(--color-fg)', boxShadow: `0 0 0 1px ${speakerTint(data.cast, v.id, .85) ?? 'var(--line-5)'}` }} />)}
        {!data.cast?.speakers.length && <CharAvatar name={c.narrator} ini={charInitials(c.narrator)} size={32} style={{ fontSize: 11 }} />}
      </span>
      <span {...stylex.props(s.text)}>
        <span {...stylex.props(s.tt)}>{data.cast?.mode === 'single' ? c.single : data.cast?.voiceMode === 'pack' ? c.pack : `${c.cast} · ${count} ${c.voiceCount}`}</span>
        <span {...stylex.props(s.mt)}>{c.voiceNote}</span>
      </span>
      <Icon name="arrow-right-2" size={16} style={{ color: 'var(--color-fg-4)' }} />
    </button>
  </div>
}

const isTodo = (row: GenRow) => row.state === 'stale' || row.state === 'new'
const isBusy = (row: GenRow) => ['run', 'queue', 'paused', 'err', 'hold'].includes(row.state)

export function VoiceGenSheet({ project, story, volume, data, open, onClose, onVoice, onExport, writable, initialRedo, machineHold }: {
  project: string; story: string; volume: number; data: AudioData
  open: boolean; onClose: () => void; onVoice: () => void; onExport: () => void
  writable: boolean; initialRedo?: number[]
  /** §AT — ghi chú máy đọc trong nhóm “Đang chờ máy đọc” (spinner khi đang chuẩn bị bộ đọc). */
  machineHold?: { note: string; spinning: boolean } | null
}) {
  const chapters = data.volume?.chapters ?? []
  const jobs = data.queue
  const rows = useMemo(() => genRows(chapters, jobs), [chapters, jobs])
  const strip = useMemo(() => genStrip(rows), [rows])
  const [selected, setSelected] = useState<ReadonlySet<number>>(new Set())
  const [redo, setRedo] = useState<ReadonlySet<number>>(new Set())
  const [doneOpen, setDoneOpen] = useState(false)
  const notify = useJobSelection((state) => state.notify)
  const toast = useMobileToast()
  const quality = data.settings?.quality ?? 'standard'
  const estimates = data.timeline?.qualityEtaSec
  const eta = estimates?.[quality]

  const preselected = useRef(false)

  useEffect(() => {
    if (!open) { preselected.current = false; return }
    // Dữ liệu về sau khi mở ngăn (deep-link/mở ngay khi mount) → chọn sẵn khi hàng có mặt.
    if (preselected.current || rows.length === 0) return
    preselected.current = true
    const marked = new Set(initialRedo ?? [])
    // `genRedo` (MVoiceGen.dc.html · redo): chương đã có giọng được chọn tạo lại cũng nằm trong ô chọn.
    setSelected(new Set([...preselect(rows), ...marked]))
    setRedo(marked)
    setDoneOpen(marked.size > 0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, rows])

  const todoRows = rows.filter((row) => isTodo(row) || row.state === 'composing')
  const selectableTodo = todoRows.filter(isTodo)
  const busyRows = rows.filter(isBusy)
  const doneRows = rows.filter((row) => row.state === 'done')
  const pickedRows = rows.filter((row) =>
    (isTodo(row) && selected.has(row.chapter.chapter)) || (row.state === 'done' && redo.has(row.chapter.chapter) && selected.has(row.chapter.chapter)))
  const counts = genCounts(rows, new Set(pickedRows.map((row) => row.chapter.chapter)), redo)
  const pickEta = pickEtaSec(pickedRows, new Set(pickedRows.map((row) => row.chapter.chapter)), eta)
  const action = genAction(counts.update, counts.fresh, counts.redo)
  const summary = genSummary(counts, { quality, etaSec: pickEta })
  const allTodoOn = selectableTodo.length > 1 && selectableTodo.every((row) => selected.has(row.chapter.chapter))
  const staleCount = rows.filter((row) => row.state === 'stale').length

  const queueAction = useAction(async (fn: () => Promise<unknown>) => fn(), { invalidate: [keys.story(project, story, 'audio'), keys.jobs()] })
  const enqueue = useAction(() => enqueueAudioChapters(project, story, volume, genQueueInput(pickedRows, quality, data.cast?.voiceMode ?? data.cast?.mode ?? 'cast')), { invalidate: [keys.story(project, story, 'audio'), keys.jobs()] })

  const toggle = (row: GenRow) => {
    const n = row.chapter.chapter
    const next = new Set(selected)
    if (next.has(n)) next.delete(n)
    else next.add(n)
    setSelected(next)
  }
  const markRedo = (n: number) => {
    const nextRedo = new Set(redo); nextRedo.add(n); setRedo(nextRedo)
    const nextSel = new Set(selected); nextSel.add(n); setSelected(nextSel)
  }

  const tagsOf = (row: GenRow): ReactNode => {
    const tag = genTag(row)
    const parts: ReactNode[] = []
    if (tag.text) parts.push(<span key="t" {...stylex.props(s.tg, tag.tone === 'warn' && s.tgWarn, tag.tone === 'run' && s.tgRun, tag.tone === 'err' && s.tgErr)}>{tag.text}</span>)
    if (row.state === 'new' && row.chapter.issues > 0) parts.push(<span key="i" {...stylex.props(s.tg, s.tgWarn)}>{row.chapter.issues} {c.review}</span>)
    if (row.state === 'done' && redo.has(row.chapter.chapter) && selected.has(row.chapter.chapter))
      parts.push(<span key="r" {...stylex.props(s.tg, s.tgRun)}>{c.redoFromStart}</span>)
    return parts
  }

  const rowIco = (row: GenRow): ReactNode => {
    switch (row.state) {
      case 'done': return <span {...stylex.props(s.ico, s.icoDone)}><Icon name="tick-circle" size={16} /></span>
      case 'stale': return <span {...stylex.props(s.ico, s.icoStale)}><Icon name="refresh-2" size={15} /></span>
      case 'err': return <span {...stylex.props(s.ico, s.icoErr)}><Icon name="warning-2" size={15} /></span>
      case 'run': return <Ring size={32} percent={(row.job?.percent ?? 0) / 100}>{null}</Ring>
      case 'paused': return <span {...stylex.props(s.ico, s.icoQueue)}><Icon name="pause" size={13} /></span>
      case 'queue': return <span {...stylex.props(s.ico, s.icoQueue)} />
      case 'hold': return <span {...stylex.props(s.ico, s.icoHold)}><Icon name="clock" size={15} /></span>
      case 'composing': return <Ring size={32} percent={0}>{null}</Ring>
      default: return <span {...stylex.props(s.ico, s.icoNew)} />
    }
  }

  const rowView = (row: GenRow, index: number, groupWarn: boolean): ReactNode => {
    const n = row.chapter.chapter
    const checkbox = isTodo(row) || (row.state === 'done' && redo.has(n))
    const on = selected.has(n)
    const rowEta = rowEtaSec(row, eta)
    const meta = genMeta(row, { etaSec: rowEta, redoSelected: row.state === 'done' && on, unlockedEta: estimates !== undefined })
    const clickable = checkbox || row.state === 'done'
    const act: { label: string; go: () => void } | null =
      row.state === 'err' ? { label: c.retry, go: () => void queueAction.run(() => controlAudioQueue(project, story, volume, 'retry')) }
      : row.state === 'queue' && row.position > 1 ? { label: c.prioritize, go: () => void queueAction.run(() => prioritizeAudioChapter(project, story, volume, n)) }
      : row.state === 'done' && !redo.has(n) ? { label: c.redo, go: () => markRedo(n) }
      : null
    return <div key={`${row.state}-${n}`} role={checkbox ? 'checkbox' : 'group'} aria-checked={checkbox ? on : undefined} aria-disabled={!clickable || undefined} tabIndex={checkbox ? 0 : undefined}
      {...stylex.props(s.row, index > 0 && s.rowBorder, row.state === 'composing' && s.dim)}
      onClick={checkbox ? () => toggle(row) : undefined}
      onKeyDown={checkbox ? (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); toggle(row) } } : undefined}>
      {rowIco(row)}
      <span {...stylex.props(s.text)}>
        <span {...stylex.props(s.tt)} title={`${n}. ${row.chapter.title}`}>{n}. {row.chapter.title}</span>
        <span {...stylex.props(s.mt)}>{tagsOf(row)}<span title={meta}>{meta}</span></span>
      </span>
      {row.state === 'run' && <span {...stylex.props(s.pct)}>{Math.round(row.job?.percent ?? 0)}%</span>}
      {checkbox && <span {...stylex.props(s.box, on && s.boxOn, on && groupWarn && s.boxOnWarn)} aria-hidden="true">{on && <Icon name="tick-circle" size={15} />}</span>}
      {act && <button type="button" {...stylex.props(s.chp)} disabled={!writable || queueAction.busy} onClick={(event) => { event.stopPropagation(); act.go() }}>{act.label}</button>}
    </div>
  }

  const queuePaused = jobs.some((job) => job.state === 'paused') && !jobs.some((job) => job.state === 'rendering')
  const groupHead = (title: string, note: string, tone: '' | 'warn' | 'run' | 'hold', link?: { text: string; go: () => void }) =>
    <div {...stylex.props(s.ghead)}>
      <b {...stylex.props(s.gtitle, tone === 'warn' && s.gtitleWarn, tone === 'run' && s.gtitleRun, tone === 'hold' && s.gtitleHold)}>{title}</b>
      {link && <button type="button" {...stylex.props(s.glink)} onClick={link.go}>{link.text}</button>}
      <span {...stylex.props(s.gnote)}>{note}</span>
    </div>

  return <BottomSheet open={open} onClose={onClose} height={790} label={`${c.title} · Tập ${volume}`} style={{ paddingBottom: 0 }}>
    <div {...stylex.props(s.root)}>
      <div {...stylex.props(s.head)}>
        <span {...stylex.props(s.title)}>{c.title} · Tập {volume}</span>
        <button aria-label={c.close} {...stylex.props(s.close)} onClick={onClose}><Icon name="add" size={17} style={{ transform: 'rotate(45deg)' }} /></button>
      </div>
      <div {...stylex.props(s.scroll)}>
        <div className={`vg-ov ${stylex.props(s.ov).className}`} aria-hidden="true">
          {strip.cells.map((cell) => <span key={cell.key} {...stylex.props(s.cell,
            cell.cls === 'done' && s.cellDone, cell.cls === 'stale' && s.cellStale, cell.cls === 'new' && s.cellNew,
            cell.cls === 'err' && s.cellErr, cell.cls === 'queue' && s.cellQueue, cell.cls === 'hold' && s.cellHold, cell.cls === 'run' && s.cellRun)}>
            {cell.cls === 'run' && <span {...stylex.props(s.cellRunFill)} style={{ width: `${cell.pct}%` }} />}
          </span>)}
        </div>
        <div className={`vg-ovleg ${stylex.props(s.leg).className}`}>
          {strip.legend.map((item) => <span key={item.color}><i {...stylex.props(s.swatch,
            item.color === 'ok' && s.swOk, item.color === 'warn' && s.swWarn, item.color === 'muted' && s.swMuted, item.color === 'run' && s.swRun, item.color === 'hold' && s.swHold)} />{item.text}</span>)}
        </div>

        {todoRows.length > 0 && <>
          {groupHead(`${c.needWork} · ${todoRows.length}`, staleCount ? c.editedAfterVoice(staleCount) : '', staleCount ? 'warn' : '',
            selectableTodo.length > 1 ? {
              text: allTodoOn ? c.clearAll : c.selectAllShort,
              go: () => setSelected(allTodoOn ? new Set([...selected].filter((n) => !selectableTodo.some((row) => row.chapter.chapter === n))) : new Set([...selected, ...selectableTodo.map((row) => row.chapter.chapter)])),
            } : undefined)}
          <div className={`vg-grp ${stylex.props(s.grp, staleCount > 0 && s.grpWarn).className}`}>
            {todoRows.map((row, index) => rowView(row, index, staleCount > 0))}
          </div>
        </>}

        {busyRows.length > 0 && (() => {
          // §AT — có chương chờ máy đọc thì cả nhóm đổi sang nhãn trung tính, ghi chú máy đọc, chỉ còn “Báo khi bắt đầu”.
          const holding = busyRows.some((row) => row.state === 'hold')
          return <>
            {groupHead(holding ? `${c.waitingMachine} · ${busyRows.length}` : `${c.generating} · ${busyRows.length}`,
              holding ? c.holdNote : queuePaused ? c.pausedNote : c.serverNote, holding ? 'hold' : 'run')}
            {holding && machineHold && <div {...stylex.props(s.vm)} role="status">
              {machineHold.spinning ? <Spinner size={15} color={color['--color-info']} style={{ marginTop: 2 }} /> : <span {...stylex.props(s.vmDot)} aria-hidden="true" />}
              <span {...stylex.props(s.vmText)}>{machineHold.note}</span>
            </div>}
            <div className={`vg-gbar ${stylex.props(s.gbar).className}`}>
              {holding
                ? <button type="button" role="switch" aria-checked={notify} {...stylex.props(s.chp, notify && s.chpOn)} onClick={() => void toggleNotify()}>
                  <Icon name="notification" size={14} />{c.notifyStart}
                </button>
                : <>
                  <button type="button" {...stylex.props(s.chp)} disabled={!writable || queueAction.busy}
                    onClick={() => void queueAction.run(() => controlAudioQueue(project, story, volume, queuePaused ? 'resume' : 'pause'))}>
                    <Icon name={queuePaused ? 'play' : 'pause'} size={14} />{queuePaused ? c.resume : c.pause}
                  </button>
                  <button type="button" role="switch" aria-checked={notify} {...stylex.props(s.chp, notify && s.chpOn)} onClick={() => void toggleNotify()}>
                    <Icon name="notification" size={14} />{c.notify}
                  </button>
                </>}
            </div>
            <div className={`vg-grp ${stylex.props(s.grp).className}`}>
              {busyRows.map((row, index) => rowView(row, index, false))}
            </div>
          </>
        })()}

        {doneRows.length > 0 && <>
          {groupHead(`${c.finished} · ${doneRows.length}`, '', '', {
            text: doneOpen ? c.collapse : c.viewRedo,
            go: () => setDoneOpen(!doneOpen),
          })}
          {doneOpen && <div className={`vg-grp ${stylex.props(s.grp).className}`}>{doneRows.map((row, index) => rowView(row, index, false))}</div>}
        </>}

        {groupHead(c.howToRead, c.forUpcoming, '')}
        <VoiceRow data={data} onVoice={onVoice} />
        <div className={`vg-qg ${stylex.props(s.qg).className}`} role="radiogroup" aria-label={c.quality}>
          {qualities.map((q) => <button key={q} type="button" role="radio" aria-checked={q === quality} disabled={!writable || data.savePending}
            {...stylex.props(s.qcell, q === quality && s.qcellOn)}
            onClick={() => { if (data.settings) void data.saveSettings({ ...data.settings, quality: q }) }}>
            {c[q]}<small {...stylex.props(s.qmeta)}>{estimates?.[q] ? `≈ ${Math.round(estimates[q]! / 60)} ${c.shortMinutesPerChapter}` : c.noEstimate}</small>
          </button>)}
        </div>
        {qualities.some((q) => !estimates?.[q]) && <p {...stylex.props(s.note)}>{c.estimateReason}.</p>}
        <p {...stylex.props(s.note)}>{c.keepDone}</p>
        <div {...stylex.props(s.grp)} style={{ marginTop: 14 }}>
          <div {...stylex.props(s.row)} style={{ minHeight: 58 }}>
            <span {...stylex.props(s.text)}>
              <span {...stylex.props(s.tt)}>{c.auto}</span>
              <span {...stylex.props(s.mt)}>{c.autoNote}</span>
            </span>
            <Switch size={[51, 31]} checked={data.settings?.autoOnLock ?? false} label={c.auto} disabled={!writable || data.savePending}
              onChange={() => { if (data.settings) void data.saveSettings({ ...data.settings, autoOnLock: !data.settings.autoOnLock }) }} />
          </div>
        </div>
        {Boolean(enqueue.error) && <p className="m-error" role="alert">{errText(enqueue.error)}</p>}
      </div>
      <div className={`vg-sfoot ${stylex.props(s.footer).className}`}>
        <span className={`vg-sum ${stylex.props(s.sum).className}`}>{summary}</span>
        {action && <button type="button" className={`vg-cta ${stylex.props(s.cta, action.tone === 'update' && s.ctaWarn).className}`} disabled={!writable || enqueue.busy} style={{ opacity: !writable || enqueue.busy ? .4 : 1 }}
          onClick={async () => {
            const result = await enqueue.run()
            if (result) {
              setSelected(new Set()); setRedo(new Set())
              toast(c.queuedToast(result.chapters.length))
            }
          }}>
          <Icon name={action.icon} size={18} />{action.label}
        </button>}
        {!action && !counts.busy && counts.todo === 0 && <button type="button" className={`vg-cta-alt ${stylex.props(s.cta, s.ctaGhost).className}`} onClick={() => { onClose(); onExport() }}>
          <Icon name="headphone" size={18} />{c.listenExportAll}
        </button>}
      </div>
    </div>
  </BottomSheet>
}
