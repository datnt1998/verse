// §AK (10-audio.md › A8, MVoiceGen.dc.html): một bộ trạng thái chương + một hàm nhãn nút chính
// dùng chung cho thẻ trang Nghe, chân ngăn Tạo giọng và thanh Cập nhật ở Phòng thu.
import type { AudioChapter, AudioQueueJob, AudioSettings } from '../../api/types'
import { t } from '../../i18n/vi'

const c = () => t.audio.generation

/** Trạng thái một chương trên dải/ngăn — bảng A8. `composing` = AI đang soạn bản đọc (nhóm Cần làm, không chọn được). */
export type GenState = 'done' | 'stale' | 'new' | 'composing' | 'run' | 'queue' | 'paused' | 'err'

export type GenRow = {
  chapter: AudioChapter
  job: AudioQueueJob | null
  state: GenState
  /** `position` của job, suy ra từ thứ tự hàng đợi khi server chưa gửi. */
  position: number
}

const ACTIVE: AudioQueueJob['state'][] = ['queued', 'rendering', 'paused', 'error']

export const liveJob = (jobs: readonly AudioQueueJob[], chapter: number) =>
  jobs.find((job) => job.chapter === chapter && ACTIVE.includes(job.state)) ?? null

/** Job này đọc lại những đoạn đổi (update) hay cả chương (create/redo). */
export const jobKind = (job: AudioQueueJob | null, chapter?: AudioChapter): 'create' | 'update' | 'redo' =>
  job?.kind ?? (chapter && (chapter.chapterChanged || chapter.scriptChanged) ? 'update' : 'create')

const done = (chapter: AudioChapter) =>
  !!chapter.audio && chapter.audio.segments > 0 && chapter.audio.segmentsDone >= chapter.audio.segments

const stale = (chapter: AudioChapter) => Boolean(chapter.chapterChanged || chapter.scriptChanged)

/** Thứ tự chờ 1…n trong các job `queued` (khớp `position` phía server). */
const queueRank = (jobs: readonly AudioQueueJob[]) => {
  const rank = new Map<string, number>()
  let next = 0
  for (const job of jobs) if (job.state === 'queued') rank.set(job.jobId, ++next)
  return rank
}

/** Xếp chương vào một trạng thái hiển thị — thứ tự: job đang sống > cần cập nhật > đã có giọng > đang soạn > chưa có. */
export const genRows = (chapters: readonly AudioChapter[], jobs: readonly AudioQueueJob[]): GenRow[] => {
  const rank = queueRank(jobs)
  return chapters.map((chapter) => {
    const job = liveJob(jobs, chapter.chapter)
    const state: GenState = job
      ? job.state === 'queued' ? 'queue' : job.state === 'rendering' ? 'run' : job.state === 'paused' ? 'paused' : 'err'
      : stale(chapter) ? 'stale'
      : done(chapter) ? 'done'
      : chapter.status === 'draft' ? 'composing'
      : 'new'
    return { chapter, job, state, position: job?.position ?? (job ? rank.get(job.jobId) ?? 0 : 0) }
  })
}

/** Ô màu trên dải trạng thái (MVoiceGen `.ov`): done · stale · run · queue · new · err. */
export const ovCell = (row: GenRow): 'done' | 'stale' | 'run' | 'queue' | 'new' | 'err' =>
  row.state === 'paused' ? 'queue' : row.state === 'composing' ? 'new' : row.state

/** Hàm nhãn nút chính duy nhất — “Cập nhật n chương” (vàng, refresh) khi chỉ còn cập nhật, ngược lại “Tạo giọng n chương” (trắng, micro). */
export function genAction(update: number, fresh: number, redo: number): { label: string; tone: 'update' | 'create'; icon: 'refresh-2' | 'microphone-2' } | null {
  const total = update + fresh + redo
  if (!total) return null
  const onlyUpdate = fresh === 0 && redo === 0
  return onlyUpdate
    ? { label: c().updateChapters(total), tone: 'update', icon: 'refresh-2' }
    : { label: c().createChapters(total), tone: 'create', icon: 'microphone-2' }
}

/** ETA giây cho một chương sắp làm; `undefined` = chưa đo. */
export const rowEtaSec = (row: GenRow, qualityEtaSec: number | undefined): number | undefined => {
  const { chapter } = row
  if (row.state === 'stale') {
    if (chapter.updateEtaSec !== undefined) return chapter.updateEtaSec
    if (qualityEtaSec !== undefined && chapter.changedSegments > 0 && chapter.segments > 0)
      return Math.round((qualityEtaSec * chapter.changedSegments) / chapter.segments)
    return undefined
  }
  return qualityEtaSec
}

/** ETA của cả lượt chọn — chỉ có số khi mọi chương đã chọn đều đo được. */
export const pickEtaSec = (rows: readonly GenRow[], picked: ReadonlySet<number>, qualityEtaSec: number | undefined): number | undefined => {
  let total = 0
  for (const row of rows) {
    if (!picked.has(row.chapter.chapter)) continue
    const eta = rowEtaSec(row, qualityEtaSec)
    if (eta === undefined) return undefined
    total += eta
  }
  return total
}

/** Nhãn tag của một hàng (bảng A8); `issues` thêm tag “n chỗ cần xem” phía gọi. */
export const genTag = (row: GenRow): { text: string; tone: '' | 'warn' | 'run' | 'err' } => {
  const k = c()
  switch (row.state) {
    case 'stale': return { text: row.chapter.chapterChanged ? k.chapterEdited : k.scriptEdited, tone: 'warn' }
    case 'new': return { text: row.chapter.status === 'todo' ? k.noScript : k.unlocked, tone: '' }
    case 'composing': return { text: k.composing, tone: 'run' }
    case 'run': return { text: jobKind(row.job, row.chapter) === 'update' ? k.updating : k.active, tone: 'run' }
    case 'paused': return { text: k.paused, tone: 'warn' }
    case 'queue': return { text: row.position <= 1 ? k.nextUp : k.waitingAt(row.position), tone: '' }
    case 'err': return { text: k.failed, tone: 'err' }
    case 'done': return { text: '', tone: '' }
  }
}

/** Dòng mô tả dưới tên chương trong ngăn. */
export const genMeta = (row: GenRow, opts: { etaSec?: number; redoSelected?: boolean; unlockedEta?: boolean }): string => {
  const k = c()
  const { chapter, job, state } = row
  const eta = opts.etaSec
  const approx = eta !== undefined ? ` · ${k.estimate(Math.max(1, Math.round(eta / 60)))}` : opts.unlockedEta ? '' : ` · ${k.noEstimate}`
  if (state === 'stale') {
    const how = chapter.chapterChanged ? k.chapterChangedDesc : k.scriptChangedDesc
    const segs = chapter.changedSegments > 0 ? `${k.changedSegs(chapter.changedSegments)} · ` : ''
    return `${segs}${how}${approx}`
  }
  if (state === 'new') {
    if (chapter.status === 'todo') return eta !== undefined ? `${k.composeFirst}${approx}` : `${k.composeFirst} · ${k.composeEstimateReason}`
    return `${chapter.segments} ${k.segments}${approx}`
  }
  if (state === 'composing') return k.composing
  if (state === 'run' || state === 'paused') {
    const seg = job ? `${k.segment} ${job.segmentsDone}/${job.segments}` : ''
    const left = job && job.etaSec > 0 ? ` · ${k.remaining} ${t.audio.logic.phut(Math.max(1, Math.round(job.etaSec / 60)))}` : ''
    return `${seg}${left}`
  }
  if (state === 'queue') {
    const kind = jobKind(job, chapter)
    return kind === 'update' ? k.updateTask : job?.needsScript ? k.composeFirst : k.fullChapter
  }
  if (state === 'err') return job?.error ?? k.failed
  // done
  const dur = chapter.audio?.durationSec ?? chapter.durationSec ?? 0
  return `${Math.floor(dur / 60)}:${String(Math.floor(dur % 60)).padStart(2, '0')}${opts.redoSelected && eta !== undefined ? approx : ''}`
}

export type GenCounts = { update: number; fresh: number; redo: number; busy: number; todo: number; unlockedNew: number; total: number; done: number }

/** Đếm theo lựa chọn hiện tại + tình trạng cả tập (redo = chương `done` được đánh dấu tạo lại). */
export function genCounts(rows: readonly GenRow[], picked: ReadonlySet<number>, redo: ReadonlySet<number>): GenCounts {
  let update = 0, fresh = 0, redoN = 0, busy = 0, todo = 0, unlockedNew = 0, doneN = 0
  for (const row of rows) {
    if (row.state === 'done') doneN++
    const isTodo = row.state === 'stale' || row.state === 'new' || row.state === 'composing'
    if (isTodo) todo++
    const on = picked.has(row.chapter.chapter)
    if (on && row.state === 'new' && row.chapter.status !== 'locked') unlockedNew++
    if (on && row.state === 'stale') update++
    else if (on && row.state === 'new') fresh++
    else if (on && row.state === 'done' && redo.has(row.chapter.chapter)) redoN++
    if (['run', 'queue', 'paused', 'err', 'composing'].includes(row.state)) busy++
  }
  return { update, fresh, redo: redoN, busy, todo, unlockedNew, total: update + fresh + redoN, done: doneN }
}

/** Câu tóm tắt ở chân ngăn (`.sum` trên MVoiceGen). */
export function genSummary(counts: GenCounts, opts: { quality: AudioSettings['quality']; etaSec?: number }): string {
  const k = c()
  if (counts.total === 0) {
    if (counts.busy) return `${k.noWork} · ${k.nGenerating(counts.busy)}`
    if (counts.todo) return k.pickChapters
    return k.allVoiced(counts.done)
  }
  const parts = [
    counts.update ? k.sumUpdate(counts.update) : '',
    counts.fresh ? k.sumNew(counts.fresh) : '',
    counts.redo ? k.sumRedo(counts.redo) : '',
  ].filter(Boolean).join(' · ')
  let text = `${parts} · ${k[opts.quality]}`
  if (opts.etaSec !== undefined) text += ` · ${k.estimate(Math.max(1, Math.round(opts.etaSec / 60)))}`
  if (counts.busy) text += `, ${k.afterGenerating}`
  if (counts.unlockedNew) text += `. ${k.lockNote}`
  return text
}

/** Chọn sẵn mọi chương “Cần làm” (cần cập nhật + chưa có giọng) khi mở ngăn — bảng A8. */
export const preselect = (rows: readonly GenRow[]): Set<number> =>
  new Set(rows.filter((row) => row.state === 'stale' || row.state === 'new').map((row) => row.chapter.chapter))

export type GenQueueInput = {
  chapters: number[]
  redo: number[]
  quality: AudioSettings['quality']
  voiceMode: 'cast' | 'single' | 'pack'
  /** chỉ chốt khi có chương chưa chốt được chọn; chương sửa sau khi chốt giữ nguyên trạng thái chốt. */
  lock: boolean
}

/** Payload `POST …/audio/queue`: chương cần cập nhật/tạo mới vào `chapters`, chương “Tạo lại từ đầu” vào `redo`. */
export function genQueueInput(picked: readonly GenRow[], quality: AudioSettings['quality'], voiceMode: 'cast' | 'single' | 'pack'): GenQueueInput {
  return {
    chapters: picked.filter((row) => row.state !== 'done').map((row) => row.chapter.chapter),
    redo: picked.filter((row) => row.state === 'done').map((row) => row.chapter.chapter),
    quality,
    voiceMode,
    lock: picked.some((row) => row.state === 'new' && row.chapter.status !== 'locked'),
  }
}

/** “Chương 1 và 4” — tên ngắn trên thanh Cập nhật (A8). */
const chapterList = (list: readonly number[]) => list.length <= 1 ? String(list[0] ?? '')
  : `${list.slice(0, -1).join(', ')}${c().andJoin}${list[list.length - 1]}`

export type UpdateBar = {
  chapters: number[]
  segments: number
  etaSec?: number
  /** Cả nhóm đã vào hàng đợi → thanh đổi sang “Đã xếp chương …, chờ cập nhật”. */
  queued: boolean
  text: string
  sub: string
  /** Nhãn nút chính, cùng hàm với thẻ trang Nghe và chân ngăn. */
  button: string
  tone: 'warn' | 'queue'
}

const ACTIVE_STATES: AudioQueueJob['state'][] = ['queued', 'rendering', 'paused']

/** Thanh trên dải chương Phòng thu (AudioExport.dc.html · `upd`): chương đã sửa sau khi tạo giọng. */
export function updateBar(rows: readonly GenRow[], qualityEtaSec: number | undefined, anyRunning: boolean): UpdateBar | null {
  const changed = rows.filter((row) => row.chapter.chapterChanged || row.chapter.scriptChanged)
  if (changed.length === 0) return null
  const chapters = changed.map((row) => row.chapter.chapter)
  const segments = changed.reduce((sum, row) => sum + row.chapter.changedSegments, 0)
  const queued = changed.every((row) => !!row.job && ACTIVE_STATES.includes(row.job.state))
  const estimated = changed.reduce<number | undefined>((sum, row) => {
    const eta = rowEtaSec(row, qualityEtaSec)
    return sum === undefined || eta === undefined ? undefined : sum + eta
  }, 0)
  const eta = estimated !== undefined && estimated > 0 ? ` · ${c().estimate(Math.max(1, Math.round(estimated / 60)))}` : ''
  const k = c()
  if (queued) {
    return {
      chapters, segments, etaSec: estimated, queued: true, tone: 'queue',
      text: k.updQueuedText(chapterList(chapters), segments, anyRunning ? `, ${k.updQueuedAfter}` : ''),
      sub: k.updQueuedSub,
      button: '',
    }
  }
  return {
    chapters, segments, etaSec: estimated, queued: false, tone: 'warn',
    text: k.updStaleText(chapterList(chapters)),
    sub: segments > 0 ? k.updStaleSub(segments, eta) : `${k.keepRest}`,
    button: genAction(changed.length, 0, 0)!.label,
  }
}

/** Ô trên dải trạng thái: lớp màu (`.ov` của MVoiceGen) + % đổ của ô đang chạy. */
export type GenStripCell = { cls: 'done' | 'stale' | 'run' | 'queue' | 'new' | 'err'; pct: number; key: number }
export type GenStrip = { cells: GenStripCell[]; text: string; legend: { color: 'ok' | 'warn' | 'muted' | 'run'; text: string }[] }

/** Dải trạng thái + chú thích (`.ov` / `.ovleg` / `ovText`) — dùng chung thẻ trang Nghe và đầu ngăn. */
export function genStrip(rows: readonly GenRow[]): GenStrip {
  const k = c()
  const cells = rows.map((row) => ({
    cls: ovCell(row), key: row.chapter.chapter,
    pct: row.job && (row.state === 'run' || row.state === 'paused') ? Math.round(row.job.percent) : 0,
  }))
  const n = (pred: (row: GenRow) => boolean) => rows.filter(pred).length
  const doneN = n((row) => row.state === 'done')
  const staleN = n((row) => row.state === 'stale')
  const newN = n((row) => row.state === 'new' || row.state === 'composing')
  const busyN = n((row) => ['run', 'queue', 'paused', 'err', 'composing'].includes(row.state))
  const textParts = [
    `${doneN}/${rows.length} ${k.hasVoice}`,
    staleN ? k.legNeedUpdate(staleN) : '',
    newN ? k.legNoVoice(newN) : '',
    busyN ? k.legGenerating(busyN) : '',
  ].filter(Boolean)
  const legend = ([
    ['ok', `${doneN} ${k.hasVoice}`],
    ['warn', k.legNeedUpdate(staleN)],
    ['muted', k.legNoVoice(newN)],
    ['run', k.legGenerating(busyN)],
  ] as const).filter(([, label]) => Number.parseInt(label, 10) > 0).map(([color, text]) => ({ color, text }))
  return { cells, text: textParts.join(' · '), legend }
}
