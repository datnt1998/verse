// Mock việc chạy nền — GET /api/jobs · job.* trên /api/events · POST /api/jobs/:id/{pause|resume|cancel|retry}
// (14-jobs-dock.md §1.7 · GAP-26). Thêm POST /api/jobs/demo (chỉ mock) để dựng đúng từng kịch bản
// của Jobs.dc.html: {scenario: running|expanded|finishing|attention|error|mini|empty} hoặc
// {action: add|finish|attention|error|finishAll|reset} như bảng thử của artboard; {hold: true|false} đóng băng/tiếp tục mô phỏng tiến độ.
import type { IncomingMessage, ServerResponse } from 'node:http'
import type { Job, JobAction, JobEvent, TtsInstallProgress, User } from '../src/api/types'
import { emitGlobalEvent } from './events'
import { armExportFailure } from './exportDemo'

type CanRead = (user: User, storyId: string) => boolean

const jobs = new Map<string, Job>()
const jobEvents = new Map<string, JobEvent>()
type ControlAction = 'pause' | 'resume' | 'cancel' | 'retry'
const audioControls = new Map<string, (action: ControlAction, body?: Record<string, unknown>) => void | Promise<void>>()
/** Tốc độ mô phỏng: tỉ lệ tăng mỗi nhịp + số phút ước tính cho cả việc. */
const sims = new Map<string, { rate: number; mins: number; steps?: string[]; template?: string }>()
let canRead: CanRead = () => true
let seq = 0

const TICK_MS = 600
// Truyện mẫu của artboard.
const P = 'dit-dang-audio'
const S = '01-tra-xanh-cuop-kich-ban'
const route = (tail: string) => `/p/${P}/s/${S}${tail}`
const exportFile = 'xuat-ban/tra-xanh-cuop-kich-ban.epub'

type Template = {
  kind: Job['kind']; title: string; agent?: Job['agent']; steps?: string[]; unit?: { label: string; total: number }
  mins: number; rate: number; indet?: boolean; href: string; step?: string
  done?: { title: string; sub: string; actions: JobAction[] }
}

// Giá trị lấy nguyên từ Jobs.dc.html (T); `rate` là mức tăng mỗi nhịp 120 ms của artboard.
const T: Record<string, Template> = {
  plan: {
    kind: 'write', agent: 'architect', title: 'Lên dàn ý Tập 2', steps: ['Đọc tóm tắt Tập 1', 'Chọn tuyến chính', 'Chia chương', 'Viết đề cương'],
    mins: 4, rate: 0.0021, href: route('/work'),
    done: { title: 'Đã lên dàn ý Tập 2', sub: '12 chương · 3 tuyến truyện mới', actions: [{ kind: 'open', label: 'Xem dàn ý', icon: 'note-text', primary: true, href: `/p/${P}/s/${S}` }] },
  },
  grade: {
    kind: 'write', agent: 'editor', title: 'Chấm chương 24', steps: ['Đọc chương', 'Soát nhất quán', 'Chấm điểm'], mins: 1, rate: 0.0062, href: route('/read/24'),
    done: { title: 'Đã chấm chương 24 · 8,1 điểm', sub: '2 góp ý nhỏ về nhịp cảnh cuối', actions: [{ kind: 'open', label: 'Xem góp ý', icon: 'book-1', primary: true, href: route('/read/24') }] },
  },
  steer: {
    kind: 'write', agent: 'arbiter', title: 'Xét góp ý của bạn', steps: ['Đọc góp ý', 'Đối chiếu đại cương', 'Quyết định'], mins: 1, rate: 0.008, href: route('/work'),
    done: { title: 'Trọng tài đã nhận góp ý', sub: 'Áp dụng từ chương 26', actions: [{ kind: 'open', label: 'Xem quyết định', icon: 'arrow-right-2', primary: true, href: route('/work') }] },
  },
  write: {
    kind: 'write', agent: 'writer', title: 'Viết chương 25', steps: ['Đọc bối cảnh', 'Viết bản nháp', 'Soát nhất quán', 'Biên tập chấm điểm', 'Lưu chương'],
    mins: 6, rate: 0.0016, href: route('/work'),
    done: { title: 'Đã viết xong chương 25', sub: '3.214 chữ · Biên tập chấm 8,6', actions: [{ kind: 'open', label: 'Đọc chương', icon: 'book-1', primary: true, href: route('/read/25') }] },
  },
  audio: {
    kind: 'audio', title: 'Tạo giọng Tập 1', unit: { label: 'Chương', total: 12 }, mins: 18, rate: 0.0011, href: route('/audio/1/make'),
    done: {
      title: 'Đã tạo giọng Tập 1', sub: '12 chương · 2 giờ 41 phút',
      actions: [
        { kind: 'open', label: 'Nghe', icon: 'play', primary: true, href: route('/audio/1/listen') },
        { kind: 'open', label: 'Ghép & xuất', icon: 'export-curve', href: route('/audio/1/listen') },
      ],
    },
  },
  export: {
    kind: 'export', title: 'Xuất EPUB', unit: { label: 'Chương', total: 24 }, mins: 1, rate: 0.0085, href: route('/read'),
    done: {
      title: 'Đã xuất EPUB · 24 chương', sub: 'Trà Xanh Cướp Kịch Bản.epub · 1,2 MB',
      actions: [
        { kind: 'download', label: 'Tải xuống', icon: 'document-download', primary: true, path: exportFile, href: `/api/projects/${P}/stories/${S}/exports/file?path=${encodeURIComponent(exportFile)}` },
        { kind: 'reveal', label: 'Mở thư mục', icon: 'folder-open', path: 'xuat-ban' },
      ],
    },
  },
  sync: {
    kind: 'sync', title: 'Tiếp nhận chương sửa tay', unit: { label: 'Chương', total: 3 }, mins: 1, rate: 0.012, href: route('/work'),
    done: { title: 'Đã tiếp nhận 2 chương sửa tay', sub: 'Chương 3, 7 · phát hiện 1 chỗ mâu thuẫn', actions: [{ kind: 'open', label: 'Xem chỗ mâu thuẫn', icon: 'arrow-right-2', primary: true, href: route('/read/7') }] },
  },
  simulate: {
    kind: 'simulate', title: 'Học văn phong', mins: 3, rate: 0.006, indet: true, step: 'Đang phân tích văn mẫu…', href: route('/work'),
    done: { title: 'Đã cập nhật hồ sơ văn phong', sub: 'Học từ 6 văn mẫu', actions: [{ kind: 'open', label: 'Xem hồ sơ', icon: 'arrow-right-2', primary: true, href: '/settings/writing' }] },
  },
  import: {
    kind: 'import', title: 'Nhập truyện tra-xanh.txt', mins: 1, rate: 0, step: 'Đã chia 42 chương — chờ bạn xác nhận', href: route('/work?dlg=import'),
    done: { title: 'Đã nhập truyện tra-xanh.txt', sub: '42 chương', actions: [{ kind: 'open', label: 'Đọc truyện', icon: 'book-1', primary: true, href: route('/read') }] },
  },
  review: {
    kind: 'write', title: 'Chương 24 chờ bạn duyệt', mins: 1, rate: 0, step: 'Đang bật duyệt từng chương — đọc rồi cho viết tiếp', href: route('/read/24'),
  },
  audioErr: {
    kind: 'audio', title: 'Tạo giọng chương 7', unit: { label: 'Chương', total: 12 }, mins: 18, rate: 0.0011, href: route('/audio/1/make'),
  },
}
const WAIT_ACTIONS: Record<string, JobAction[]> = {
  import: [{ kind: 'open', label: 'Xem & xác nhận', icon: 'arrow-right-2', primary: true, href: route('/work?dlg=import') }],
  review: [{ kind: 'open', label: 'Đọc & duyệt', icon: 'book-1', primary: true, href: route('/read/24') }],
}
const ERRORS: Record<string, string> = { audioErr: 'Bộ đọc trên máy không phản hồi sau 3 lần thử' }

type Seed = [template: string, progress: number | 'wait' | 'err']
const SCEN: Record<string, Seed[]> = {
  running: [['write', 0.46], ['audio', 0.38], ['export', 0.62]],
  expanded: [['plan', 0.34], ['audio', 0.38], ['export', 0.62], ['review', 'wait']],
  finishing: [['grade', 0.9], ['export', 0.93]],
  attention: [['import', 'wait'], ['audio', 0.52]],
  error: [['audioErr', 'err'], ['export', 0.3]],
  mini: [['write', 0.46], ['audio', 0.38], ['export', 0.62]],
  empty: [],
}
const POOL = ['plan', 'grade', 'steer', 'sync', 'simulate', 'export', 'audio']
let poolI = 0

// Cùng một luồng toàn cục với story.updated/chapter.updated; không mở SSE riêng cho dock.
// Việc của máy (cài bộ đọc) không thuộc truyện nào: chỉ quản trị viên thấy, giống máy chủ thật.
const visible = (user: User, job: Job) => (job.project === '' && job.story === ''
  ? user.role === 'admin'
  : canRead(user, `${job.project}/${job.story}`))
function emit(event: string, job: Job) {
  emitGlobalEvent(event, job)
}
const EVENT_OF: Record<Job['status'], string> = { running: 'job.progress', paused: 'job.progress', waiting: 'job.waiting', failed: 'job.failed', done: 'job.done' }

function put(job: Job, created = false) {
  jobs.set(job.id, job)
  emit(created ? 'job.created' : EVENT_OF[job.status], job)
}
function drop(id: string) {
  const job = jobs.get(id)
  if (!job) return
  jobs.delete(id); sims.delete(id)
  jobEvents.delete(id)
  hiddenProgress.delete(id)
  audioControls.delete(id)
  emitGlobalEvent('job.removed', { id, project: job.project, story: job.story })
}

// ─── mô phỏng tiến độ ───
const derived = (job: Job, sim: { mins: number; steps?: string[] }): Partial<Job> => {
  const p = job.progress ?? 0
  const patch: Partial<Job> = { etaSec: Math.round((1 - p) * sim.mins * 60) }
  if (sim.steps) {
    const i = Math.min(sim.steps.length - 1, Math.floor(p * sim.steps.length))
    patch.unit = { done: i, total: sim.steps.length, label: 'Bước' }
    patch.step = sim.steps[i]
  } else if (job.unit) {
    patch.unit = { ...job.unit, done: Math.min(job.unit.total, Math.floor(p * job.unit.total)) }
  }
  return patch
}

function finish(job: Job) {
  const sim = sims.get(job.id)
  const tpl = sim?.template ? T[sim.template] : undefined
  const { etaSec: _eta, ...rest } = job
  put({
    ...rest, status: 'done', progress: 1, finishedAt: new Date().toISOString(),
    unit: job.unit ? { ...job.unit, done: job.unit.total } : undefined,
    result: job.result ?? tpl?.done ?? { title: `${job.title} · xong`, sub: '', actions: [] },
  })
  sims.delete(job.id)
}

/** Việc không rõ % (Học văn phong) đếm tiến độ ngầm ở đây, không gửi ra ngoài. */
const hiddenProgress = new Map<string, number>()

/** Đóng băng mô phỏng tiến độ (POST /api/jobs/demo {hold:true}) để đo giao diện với số liệu đứng yên. */
let held = false

setInterval(() => {
  if (held) return
  for (const job of [...jobs.values()]) {
    const sim = sims.get(job.id)
    if (job.status !== 'running' || !sim || sim.rate <= 0) continue
    const step = sim.rate * (TICK_MS / 120)
    if (job.progress === null) {
      const hidden = Math.min(1, (hiddenProgress.get(job.id) ?? 0) + step)
      hiddenProgress.set(job.id, hidden)
      if (hidden >= 1) { hiddenProgress.delete(job.id); finish(job) }
      continue
    }
    const progress = Math.min(1, job.progress + step)
    if (progress >= 1) { finish({ ...job, progress: 1 }); continue }
    const next: Job = { ...job, progress }
    put({ ...next, ...derived(next, sim) })
  }
}, TICK_MS).unref?.()

function create(name: string, seed: number | 'wait' | 'err' = 0): Job {
  const tpl = T[name]
  const id = `${P}/${S}/${tpl.kind}/${++seq}`
  const status: Job['status'] = seed === 'wait' ? 'waiting' : seed === 'err' ? 'failed' : 'running'
  const progress = tpl.indet ? null : typeof seed === 'number' ? seed : null
  let job: Job = {
    id, kind: tpl.kind, title: tpl.title, status, progress, href: tpl.href, project: P, story: S,
    startedAt: new Date(Date.now() - 60_000 + seq).toISOString(),
    ...(tpl.agent ? { agent: tpl.agent } : {}),
    ...(tpl.step ? { step: tpl.step } : {}),
    ...(tpl.unit ? { unit: { done: 0, total: tpl.unit.total, label: tpl.unit.label } } : {}),
  }
  if (status === 'waiting') job.result = { title: tpl.title, sub: tpl.step ?? '', actions: WAIT_ACTIONS[name] ?? [] }
  if (status === 'failed') job = { ...job, error: ERRORS[name] ?? 'Lỗi không rõ', unit: tpl.unit ? { done: 6, total: tpl.unit.total, label: tpl.unit.label } : undefined, progress: 0.52 }
  const sim = { rate: tpl.rate, mins: tpl.mins, steps: tpl.steps, template: name }
  sims.set(id, sim)
  if (status === 'running') job = { ...job, ...derived({ ...job, progress: progress ?? 0 }, sim) }
  put(job, true)
  return job
}

function scenario(name: string) {
  for (const id of [...jobs.keys()]) drop(id)
  for (const [tpl, seed] of SCEN[name] ?? []) create(tpl, seed)
}

function demoAction(action: string) {
  if (action === 'reset') return scenario('running')
  if (action === 'add') { create(POOL[poolI++ % POOL.length]); return }
  if (action === 'attention') { create('import', 'wait'); return }
  if (action === 'error') { create('audioErr', 'err'); return }
  if (action === 'finish') {
    const top = [...jobs.values()].filter((job) => job.status === 'running').sort((a, b) => (b.progress ?? 0) - (a.progress ?? 0))[0]
    if (top) finish(top)
    return
  }
  if (action === 'finishAll') {
    for (const job of [...jobs.values()]) {
      if (job.status === 'running' || job.status === 'paused') finish(job)
      else if (job.status === 'waiting' || job.status === 'failed') finish({ ...job, result: undefined })
    }
  }
}

// ─── nguồn việc thật trong mock (engine viết · import/simulate/sync · xuất) ───
const JOB_TITLE: Record<string, string> = { import: 'Nhập truyện', simulate: 'Học văn phong', importsim: 'Nhập hồ sơ văn phong', sync: 'Tiếp nhận chương sửa tay', export: 'Xuất bản', diag: 'Phân tích sức khoẻ truyện' }

/** Audio mock giữ tiến độ thật của từng lượt dựng, cùng danh sách và luồng với dock. */
export function mirrorAudioJob(job: Job, control?: (action: ControlAction, body?: Record<string, unknown>) => void | Promise<void>) {
  if (control) audioControls.set(job.id, control)
  put(job, !jobs.has(job.id))
}

export const removeAudioJob = (id: string) => drop(id)

/** Việc của máy: cài bộ đọc trên máy (mock/tts.ts gọi mỗi mốc tiến độ). */
const VOICE_INSTALL_ID = 'voice/install'

export function mirrorVoiceInstallJob(progress: TtsInstallProgress, control?: (action: ControlAction) => void | Promise<void>) {
  if (progress.state === 'idle') { drop(VOICE_INSTALL_ID); return }
  if (control) audioControls.set(VOICE_INSTALL_ID, control)
  const prev = jobs.get(VOICE_INSTALL_ID)
  const job: Job = {
    id: VOICE_INSTALL_ID, kind: 'voice', title: 'Cài bộ đọc trên máy',
    status: progress.state === 'running' ? 'running' : progress.state === 'error' ? 'failed' : 'done',
    progress: progress.state === 'running' ? Math.min(1, (progress.percent ?? 0) / 100) : progress.state === 'done' ? 1 : null,
    href: '/settings/voice', project: '', story: '', startedAt: prev?.startedAt ?? progress.updated,
    ...(progress.state === 'running' ? { step: progress.message } : {}),
    ...(progress.state === 'error' ? { step: progress.message, error: progress.error ?? progress.message } : {}),
    ...(progress.state === 'done'
      ? { finishedAt: progress.updated, result: { title: 'Đã cài xong bộ đọc trên máy', sub: progress.message, actions: [] } }
      : {}),
  }
  put(job, !prev)
}

/** Gương của SSE `job` cũ (runJob): import chờ xác nhận = `waiting`. */
export function mirrorJobEvent(p: string, s: string, event: JobEvent) {
  const kind: Job['kind'] = event.kind === 'importsim' ? 'simulate' : (event.kind as Job['kind'])
  // Id thật = `<project>/<story>/` + id worker `<kind>/<n>` (importsim → `simulate/<n>`), xem jobTracker.begin + qualifyJob.
  const id = `${p}/${s}/${event.job}`
  jobEvents.set(id, event)
  const prev = jobs.get(id)
  const base = `/p/${encodeURIComponent(p)}/s/${encodeURIComponent(s)}`
  const status: Job['status'] = event.error ? 'failed' : event.done ? 'done' : event.awaiting ? 'waiting' : 'running'
  const doneActions: JobAction[] = event.kind === 'diag'
    ? [{ kind: 'open', label: 'Xem báo cáo', primary: true, href: `${base}/work?dlg=diag:last` }]
    : event.export ? [
      { kind: 'download', label: 'Tải xuống', primary: true, path: event.export.path, href: `/api/projects/${encodeURIComponent(p)}/stories/${encodeURIComponent(s)}/exports/file?path=${encodeURIComponent(event.export.path)}` },
      { kind: 'reveal', label: 'Mở thư mục', path: event.export.path },
    ] : [{ kind: 'open', label: event.kind === 'simulate' ? 'Xem hồ sơ' : 'Xem nhật ký', primary: true, href: event.kind === 'simulate' ? '/settings/writing' : `${base}/activity` }]
  const job: Job = {
    id, kind, title: JOB_TITLE[event.kind] ?? event.kind, status,
    progress: kind === 'simulate' ? (event.done ? 1 : null) : event.total ? Math.min(1, event.current / event.total) : null,
    step: event.message, href: event.awaiting ? `${base}/work?dlg=import` : `${base}/work`, project: p, story: s,
    startedAt: prev?.startedAt ?? new Date().toISOString(),
    ...(event.error ? { error: event.error } : {}),
    ...(event.done ? { finishedAt: prev?.finishedAt ?? new Date().toISOString(), result: { title: `${JOB_TITLE[event.kind] ?? event.kind} · xong`, sub: event.message, actions: doneActions, data: event.export ?? event.sync ?? event.simulate ?? event.import } } : {}),
    ...(event.awaiting ? { result: { title: JOB_TITLE[event.kind] ?? event.kind, sub: event.message, data: event.import, actions: [{ kind: 'open' as const, label: 'Xem & xác nhận', icon: 'arrow-right-2', primary: true, href: `${base}/work?dlg=import` }] } } : {}),
  }
  put(job, !prev)
}

/** POST /exports của mock: việc xuất ngắn, xong có Tải xuống · Mở thư mục. */
export function startExportJob(p: string, s: string, file: string, chapters: number) {
  const id = `${p}/${s}/export/${++seq}`
  const total = Math.max(1, chapters)
  const format = file.split('.').pop()?.toUpperCase() ?? 'TXT'
  const href = `/api/projects/${encodeURIComponent(p)}/stories/${encodeURIComponent(s)}/exports/file?path=${encodeURIComponent(file)}`
  sims.set(id, { rate: 0.03, mins: 0.5 })
  put({
    id, kind: 'export', title: `Xuất ${format}`, status: 'running', progress: 0, unit: { done: 0, total, label: 'Chương' }, etaSec: 30,
    href: `/p/${encodeURIComponent(p)}/s/${encodeURIComponent(s)}/read`, project: p, story: s, startedAt: new Date().toISOString(),
    result: {
      title: `Đã xuất ${format} · ${total} chương`, sub: file.split('/').pop() ?? file,
      actions: [
        { kind: 'download', label: 'Tải xuống', icon: 'document-download', primary: true, path: file, href },
        { kind: 'reveal', label: 'Mở thư mục', icon: 'folder-open', path: file.split('/').slice(0, -1).join('/') },
      ],
    },
  }, true)
  return id
}

/** Gương của snapshot engine: đang viết → việc `write/1` không rõ %, agent = Người viết; dừng → bỏ việc. */
export function syncWriteJob(p: string, s: string, snap: { isRunning: boolean; currentChapter: number; totalChapters: number; completedCount: number }) {
  const id = `${p}/${s}/write/1`
  const prev = jobs.get(id)
  if (!snap.isRunning) { drop(id); return }
  const chapter = Math.max(1, snap.currentChapter)
  const job: Job = {
    id, kind: 'write', agent: 'writer', title: `Viết chương ${chapter}`, status: 'running', progress: null, step: 'Viết bản nháp',
    ...(snap.totalChapters > 0 ? { unit: { done: snap.completedCount, total: snap.totalChapters, label: 'Chương' } } : {}),
    href: `/p/${encodeURIComponent(p)}/s/${encodeURIComponent(s)}/work`, project: p, story: s,
    startedAt: prev?.startedAt ?? new Date().toISOString(),
  }
  if (prev && prev.title === job.title && prev.unit?.done === job.unit?.done) return
  put(job, !prev)
}

// ─── HTTP ───
const json = (res: ServerResponse, status: number, data: unknown) => {
  res.statusCode = status
  res.setHeader('content-type', 'application/json; charset=utf-8')
  res.end(JSON.stringify(data))
}

export async function handleJobsApi(args: {
  req: IncomingMessage; res: ServerResponse; seg: string[]; user: User; canRead: CanRead
  readBody: () => Promise<Record<string, unknown>>
  /** Tạm dừng/huỷ việc viết = dừng engine của truyện (snapshot mới sẽ bỏ việc). */
  stopWrite: (project: string, story: string) => void
}) {
  const { req, res, seg, user } = args
  canRead = args.canRead
  // GET /api/jobs
  if (seg.length === 1) {
    if (req.method !== 'GET') return json(res, 405, { error: 'Phương thức không được hỗ trợ' })
    return json(res, 200, { jobs: [...jobs.values()].filter((job) => visible(user, job)) })
  }
  if (req.method === 'GET') {
    const id = seg.slice(1).join('/')
    const job = jobs.get(id)
    if (!job || !visible(user, job)) return json(res, 404, { error: 'Không tìm thấy việc này' })
    return json(res, 200, { schemaVersion: 1, project: job.project, story: job.story, job, event: jobEvents.get(id) ?? null })
  }
  // POST /api/jobs/demo — chỉ có ở mock
  if (seg[1] === 'demo' && req.method === 'POST') {
    const body = await args.readBody()
    if (typeof body.hold === 'boolean') held = body.hold
    if (typeof body.scenario === 'string') {
      if (!(body.scenario in SCEN)) return json(res, 400, { error: `Không có kịch bản ${body.scenario}` })
      scenario(body.scenario)
    }
    if (typeof body.action === 'string') demoAction(body.action)
    // Gói AE: lần ghép kế tiếp dừng ở chương 1 để đo bgFail/thẻ đỏ (mock-only).
    if (body.action === 'exportFail') armExportFailure()
    return json(res, 200, { ok: true, jobs: [...jobs.values()] })
  }
  // POST /api/jobs/:id/{pause|resume|cancel|retry} — id chứa "/" nên ghép lại phần giữa
  const op = seg[seg.length - 1]
  const id = seg.slice(1, -1).join('/')
  const job = jobs.get(id)
  if (req.method !== 'POST' || !['pause', 'resume', 'cancel', 'retry'].includes(op)) return json(res, 404, { error: 'Không tìm thấy' })
  if (!job || !visible(user, job)) return json(res, 404, { error: 'Không tìm thấy việc này' })
  if (user.role !== 'admin' && !['owner', 'collab'].includes(user.projects[job.project])) return json(res, 403, { error: 'Bạn chỉ có quyền Chỉ xem ở dự án này' })
  const audioControl = audioControls.get(id)
  if (audioControl) {
    try {
      const body = await args.readBody()
      await audioControl(op as ControlAction, body)
      if (op === 'cancel') drop(id)
      return json(res, 200, { ok: true })
    } catch (error) {
      return json(res, 409, { error: error instanceof Error ? error.message : 'Không thực hiện được thao tác với bản đọc' })
    }
  }
  if (job.kind === 'write' && job.id.endsWith('/write/1') && (op === 'pause' || op === 'cancel')) {
    args.stopWrite(job.project, job.story)
  } else if (op === 'pause') {
    if (job.status !== 'running') return json(res, 409, { error: 'Việc này không đang chạy' })
    put({ ...job, status: 'paused' })
  } else if (op === 'resume') {
    if (job.status !== 'paused') return json(res, 409, { error: 'Việc này không đang tạm dừng' })
    put({ ...job, status: 'running' })
  } else if (op === 'retry') {
    if (job.status !== 'failed') return json(res, 409, { error: 'Chỉ thử lại được việc bị lỗi' })
    const { error: _error, ...rest } = job
    put({ ...rest, status: 'running', progress: 0.2 })
  } else {
    drop(id)
  }
  return json(res, 200, { ok: true })
}

/** Xoá toàn bộ việc và điều khiển khi truyện bị xoá vĩnh viễn. */
export function removeStoryJobs(project: string, story: string) {
  for (const job of [...jobs.values()]) {
    if (job.project === project && job.story === story) drop(job.id)
  }
}
