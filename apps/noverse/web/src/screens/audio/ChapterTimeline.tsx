import { useEffect, useId, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { AudioTimelineChapter } from '../../api/types'
import { t } from '../../i18n/vi'
import { color, kit, kind, audioTimeline as token } from '@noverse/ui/tokens.stylex'
import { Icon } from '@noverse/ui'
import { formatClock } from './logic'
import { timelineCentres, timelineIndex, timelineMode, timelineSeconds, timelineTicks, timelineTotal } from './timeline'
import { updateBar, type GenRow } from './genWork'

const styles = stylex.create({
 card: { borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 10, minWidth: 0 },
 header: { display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 12 },
 title: { fontSize: 13.5, fontWeight: 600, whiteSpace: 'nowrap' },
 total: { fontSize: 12, color: color['--color-fg-4'], textAlign: 'right' },
 body: { position: 'relative', display: 'flex', flexDirection: 'column', gap: 8, minWidth: 0 },
 strip: { display: 'flex', gap: 4, height: 40 },
 stripBar: { gap: 2, height: 28 },
 stripDense: { gap: 1 },
 cell: { minWidth: 0, flexBasis: 0, position: 'relative', overflow: 'hidden', borderRadius: 9, padding: '0 8px', display: 'flex', alignItems: 'center', gap: 6, fontSize: 11.5, cursor: 'pointer', borderWidth: 1, borderStyle: 'dashed', borderColor: kit['--kit-line-12'], color: color['--color-fg-4'], backgroundColor: 'transparent', transition: 'filter .12s, transform .12s', ':hover': { filter: 'brightness(1.35)' }, ':focus-visible': { filter: 'brightness(1.35)', outline: `2px solid ${color['--color-accent']}`, outlineOffset: 2 } },
 done: { borderStyle: 'solid', borderColor: token['--audio-tl-ok-32'], backgroundColor: kit['--kit-ok-8'], color: color['--color-fg'] },
 now: { borderStyle: 'solid', borderColor: kit['--kit-acc-55'], backgroundColor: color['--color-surface-3'], color: color['--color-fg'] },
 number: { justifyContent: 'center', padding: 0, borderRadius: 7 },
 numberDone: { backgroundColor: token['--audio-tl-ok-14'], color: kind['--kind-ally-fg'] },
 bar: { padding: 0, borderRadius: 3, borderWidth: 0, backgroundColor: kit['--kit-line-12'] },
 barDone: { backgroundColor: kit['--kit-ok-50'] },
 barNow: { backgroundColor: kit['--kit-acc-22'] },
 barUnlocked: { backgroundColor: 'transparent', backgroundImage: token['--audio-tl-hatch'] },
 fill: { position: 'absolute', left: 0, top: 0, bottom: 0, opacity: .32 },
 fillBar: { opacity: .9 },
 pen: { position: 'absolute', top: 4, bottom: 4, width: 2, borderRadius: 1, backgroundColor: kit['--kit-mark-fg'] },
 numberText: { position: 'relative', fontWeight: 600 },
 tick: { position: 'relative', color: color['--color-ok'] },
 meta: { position: 'relative', marginLeft: 'auto', color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
 tooltip: { position: 'absolute', bottom: 'calc(100% + 8px)', transform: 'translateX(-50%)', zIndex: 5, padding: '7px 10px', borderRadius: 10, backgroundColor: color['--color-surface-6'], boxShadow: token['--audio-tl-tooltip-shadow'], fontSize: 12, lineHeight: 1.45, whiteSpace: 'nowrap', pointerEvents: 'none', maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis' },
 bold: { fontWeight: 600 },
 secondary: { color: color['--color-fg-3'] },
 tooltipDone: { color: color['--color-ok'] },
 tooltipNow: { color: color['--color-accent'] },
 ruler: { position: 'relative', height: 16 },
 rulerTick: { position: 'absolute', top: 0, transform: 'translateX(-50%)', fontSize: 11, color: token['--audio-tl-ruler'], whiteSpace: 'nowrap', '::before': { content: '""', position: 'absolute', left: '50%', top: -5, width: 1, height: 4, backgroundColor: kit['--kit-wash-18'] } },
 liveTick: { color: kit['--kit-badge-run'], fontWeight: 600 },
 legend: { display: 'flex', flexWrap: 'wrap', gap: '4px 14px', fontSize: 11.5, color: token['--audio-tl-legend'] },
 swatch: { display: 'inline-block', width: 10, height: 10, borderRadius: 3, marginRight: 6, verticalAlign: -1 },
 swatchDone: { backgroundColor: kit['--kit-ok-50'] },
 swatchNow: { backgroundImage: token['--audio-tl-run'] },
 swatchQueued: { backgroundColor: kit['--kit-line-12'] },
 swatchUnlocked: { backgroundImage: token['--audio-tl-hatch-key'] },
 hint: { color: token['--audio-tl-ruler'] },
 stale: { borderStyle: 'solid', borderColor: kit['--kit-warn-50'], backgroundColor: kit['--kit-warn-9'], color: kit['--kit-warn-soft'] },
 staleNow: { backgroundColor: kit['--kit-warn-20'] },
 staleBar: { borderWidth: 0, backgroundColor: kit['--kit-warn-9'] },
 requeue: { borderColor: kit['--kit-acc-50'], backgroundColor: 'transparent', color: kit['--kit-badge-run'] },
 requeueBar: { borderWidth: 0, backgroundColor: kit['--kit-acc-50'] },
 staleMeta: { position: 'relative', color: color['--color-fg-4'], whiteSpace: 'nowrap' },
 requeueMeta: { position: 'relative', color: color['--color-fg-4'], whiteSpace: 'nowrap' },
 // Gói AK — thanh “đã sửa sau khi tạo giọng” ngay trên dải chương (AudioExport.dc.html · `upd`).
 upd: { display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 12, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-warn-30'], backgroundColor: kit['--kit-warn-7'], color: kit['--kit-warn-bar'], marginBottom: 2 },
 updQueued: { borderColor: kit['--kit-acc-30'], backgroundColor: kit['--kit-acc-6'], color: kit['--kit-acc-bar'] },
 updText: { flexGrow: 1, minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 },
 updTitle: { fontSize: 13, fontWeight: 600, lineHeight: 1.4 },
 updSub: { fontSize: 12, opacity: .8, lineHeight: 1.4 },
 updButton: { flexShrink: 0, height: 32, padding: '0 14px', borderRadius: 9, borderWidth: 0, backgroundColor: color['--color-warn'], color: kit['--kit-warn-on'], fontSize: 12.5, fontWeight: 600, whiteSpace: 'nowrap' },
})

export function ChapterTimeline({ volume, chapters, madeSec, estTotalSec, paused = false, onListen, updateRows, onUpdate, canUpdate = false, qualityEtaSec }: {
 volume: number; chapters: AudioTimelineChapter[]; madeSec: number; estTotalSec?: number; paused?: boolean; onListen: (chapter: number) => void
 /** Gói AK — chương đã sửa sau khi tạo giọng: ô “đã sửa” + thanh Cập nhật ngay trên dải. */
 updateRows?: GenRow[]; onUpdate?: (chapters: number[]) => void; canUpdate?: boolean; qualityEtaSec?: number
}) {
 const strip = useRef<HTMLDivElement>(null)
 const buttons = useRef<(HTMLButtonElement | null)[]>([])
 const tooltip = useRef<HTMLDivElement>(null)
 const tooltipId = useId()
 const [width, setWidth] = useState(0)
 const [selected, setSelected] = useState<number | null>(null)
 const [hovered, setHovered] = useState<number | null>(null)
 const [focused, setFocused] = useState<number | null>(null)
 useEffect(() => {
  const element = strip.current
  if (!element) return
  // Đo ngay khi mount: ResizeObserver không chạy ở tab ẩn/khung hình đầu → dải không còn hiện nhầm chế độ “bar”.
  setWidth(element.getBoundingClientRect().width)
  const observer = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width))
  observer.observe(element)
  return () => observer.disconnect()
 }, [])
 // Select from the measured available width, excluding the normal four-pixel gaps.
 const mode = timelineMode(chapters.length ? (width - 4 * (chapters.length - 1)) / chapters.length : 0)
 const bar = mode === 'bar'
 const gap = bar ? chapters.length > 80 ? 1 : 2 : 4
 const centres = timelineCentres(chapters, width, gap)
 const active = chapters.findIndex(chapter => chapter.state === 'rendering')
 const current = Math.min(chapters.length - 1, selected ?? Math.max(0, active))
 const shown = hovered ?? focused
 const shownChapter = shown === null ? undefined : chapters[shown]
 useEffect(() => {
  if (!tooltip.current || shown === null || !strip.current || !buttons.current[shown]) return
  const half = tooltip.current.getBoundingClientRect().width / 2
  const cell = buttons.current[shown]!.getBoundingClientRect()
  const centre = cell.left - strip.current.getBoundingClientRect().left + cell.width / 2
  tooltip.current.style.left = `${Math.max(half, Math.min(width - half, centre))}px`
 }, [shown, width, mode])
 const status = (chapter: AudioTimelineChapter) => t.audioTimeline.status(chapter.state, formatClock(timelineSeconds(chapter)), Math.round(chapter.pct))
 // §AK — chương đã sửa sau khi tạo giọng: “đã sửa” (chưa xếp) hoặc “chờ cập nhật” (đã vào hàng đợi).
 const changeOf = new Map<number, 'stale' | 'requeue'>()
 for (const row of updateRows ?? []) {
  if (!row.chapter.chapterChanged && !row.chapter.scriptChanged) continue
  changeOf.set(row.chapter.chapter, row.job && ['queued', 'rendering', 'paused'].includes(row.job.state) ? 'requeue' : 'stale')
 }
 const update = updateBar(updateRows ?? [], qualityEtaSec, chapters.some(chapter => chapter.state === 'rendering'))
 return <section aria-label={t.audioTimeline.title} className={`card ${stylex.props(styles.card).className}`}>
  <div {...stylex.props(styles.header)}>
   <span {...stylex.props(styles.title)}>{t.audioTimeline.volume(volume, chapters.length)}</span>
   <span {...stylex.props(styles.total)}>{timelineTotal(chapters, madeSec, estTotalSec)}</span>
  </div>
  {update && <div className={`upd ${stylex.props(styles.upd, update.queued && styles.updQueued).className}`} role="status">
   <Icon name="refresh-2" size={16} />
   <span {...stylex.props(styles.updText)}>
    <b className={`upd-tx ${stylex.props(styles.updTitle).className}`}>{update.text}</b>
    <span className={`upd-sub ${stylex.props(styles.updSub).className}`}>{update.sub}</span>
   </span>
   {!update.queued && <button type="button" className={`upd-btn ${stylex.props(styles.updButton).className}`} disabled={!canUpdate || !onUpdate} onClick={() => onUpdate?.(update.chapters)}>{update.button}</button>}
  </div>}
  <div {...stylex.props(styles.body)} onMouseLeave={() => setHovered(null)}>
   {shownChapter && <div ref={tooltip} role="tooltip" id={tooltipId} {...stylex.props(styles.tooltip)}>
    <span {...stylex.props(styles.bold)}>{t.audioTimeline.chapter(shownChapter.chapter)}</span><span {...stylex.props(styles.secondary)}> · {shownChapter.title}</span><br />
    <span {...stylex.props(styles.secondary, shownChapter.state === 'done' && styles.tooltipDone, shownChapter.state === 'rendering' && styles.tooltipNow)}>{status(shownChapter)}</span>
   </div>}
   <div ref={strip} role="group" aria-label={t.audioTimeline.group(volume)} data-mode={mode}
    {...stylex.props(styles.strip, bar && styles.stripBar, bar && chapters.length > 80 && styles.stripDense)}
    onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(null) }}
    onKeyDown={event => {
     const index = timelineIndex(event.key, focused ?? current, chapters.length)
     if (index === null) return
     event.preventDefault()
     setSelected(index)
     buttons.current[index]?.focus()
    }}>
    {chapters.map((chapter, index) => {
     const done = chapter.state === 'done', now = chapter.state === 'rendering'
     const change = changeOf.get(chapter.chapter)
     const stale = change === 'stale', requeue = change === 'requeue'
     const fill = `${Math.round(chapter.pct)}%`
     return <button key={chapter.chapter} ref={element => { buttons.current[index] = element }} type="button" tabIndex={index === current ? 0 : -1}
      aria-label={`${t.audioTimeline.chapter(chapter.chapter)} · ${chapter.title} · ${status(chapter)}${stale ? ` · ${t.audio.generation.editedCell}` : requeue ? ` · ${t.audio.generation.waitUpdateCell}` : ''}`} aria-describedby={shown === index ? tooltipId : undefined}
      {...stylex.props(styles.cell, done && styles.done, now && styles.now, mode === 'number' && styles.number, mode === 'number' && done && styles.numberDone, bar && styles.bar, bar && done && styles.barDone, bar && now && styles.barNow, bar && chapter.state === 'unlocked' && styles.barUnlocked, stale && styles.stale, requeue && styles.requeue, bar && stale && styles.staleBar, bar && requeue && styles.requeueBar)}
      style={{ flexGrow: timelineSeconds(chapter) }}
      onMouseEnter={() => setHovered(index)} onFocus={() => { setHovered(null); setFocused(index); setSelected(index) }}
      onClick={() => { if (done) onListen(chapter.chapter) }}>
      {now && <><span aria-hidden="true" className={`audio-run ${stylex.props(styles.fill, bar && styles.fillBar).className}`} style={{ width: fill, animationPlayState: paused ? 'paused' : undefined }} />{mode === 'full' && <span aria-hidden="true" {...stylex.props(styles.pen)} style={{ left: fill }} />}</>}
      {!bar && <span {...stylex.props(styles.numberText)}>{chapter.chapter}</span>}
      {mode === 'full' && change
       ? <span title={chapter.staleReason || (stale ? t.audio.generation.editedCellTip : t.audio.generation.updQueuedSub)} {...stylex.props(stale ? styles.staleMeta : styles.requeueMeta)}>{stale ? t.audio.generation.editedCell : t.audio.generation.waitUpdateCell}</span>
       : mode !== 'full' && change
       ? <span title={chapter.staleReason || (stale ? t.audio.generation.editedCellTip : t.audio.generation.waitUpdateCell)} {...stylex.props(stale ? styles.staleMeta : styles.requeueMeta)}><Icon name={stale ? 'refresh-2' : 'timer-1'} size={13} /></span>
       : mode === 'full' && <><span {...stylex.props(styles.meta)}>{done ? formatClock(timelineSeconds(chapter)) : now ? fill : chapter.state === 'unlocked' ? t.audioTimeline.unlocked.toLowerCase() : t.audioTimeline.wait}</span>{done && <Icon name="tick-circle-bulk" size={13} {...stylex.props(styles.tick)} />}</>}
     </button>
    })}
   </div>
   {bar && <>
    <div aria-hidden="true" {...stylex.props(styles.ruler)}>{timelineTicks(chapters).map(tick => <span key={tick.index} {...stylex.props(styles.rulerTick, tick.active && styles.liveTick)} style={{ left: centres[tick.index] }}>{tick.active ? t.audioTimeline.live(chapters[tick.index].chapter, Math.round(chapters[tick.index].pct)) : chapters[tick.index].chapter}</span>)}</div>
    <div aria-hidden="true" {...stylex.props(styles.legend)}>{(['done', 'rendering', 'queued', 'unlocked'] as const).map(state => <span key={state}><i {...stylex.props(styles.swatch, state === 'done' ? styles.swatchDone : state === 'rendering' ? styles.swatchNow : state === 'queued' ? styles.swatchQueued : styles.swatchUnlocked)} />{t.audioTimeline[state]}</span>)}<span {...stylex.props(styles.hint)}>{t.audioTimeline.hint}</span></div>
   </>}
  </div>
 </section>
}
