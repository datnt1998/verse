// Store việc chạy nền — GET /api/jobs + job.* trên luồng chung /api/events (14-jobs-dock.md §1.7).
// Một bản duy nhất cho cả app: dock gắn ở AppShell nên không mount lại khi đổi màn.
// Trạng thái pill/bảng/chấm + “Báo khi xong” lưu localStorage theo người dùng.
import { useMemo, useSyncExternalStore } from 'react'
import {
  ApiError, downloadExportFile, downloadJobFile, getJobs, getRevealPath, getSystem, jobAction, revealFolder,
} from '../../api/client'
import type { AudioExportJobResult, Job, JobAction } from '../../api/types'
import {
  autoDownloadBlocked, downloadStateOf, exportDownloadSource, exportLook, exportWatchOf, exportWhat, isAudioExportJob,
  jobError, setDownloadState,
} from './exportFlow'
import { events, subscribeEvents } from '../../data/events'
import { cache, sameData } from '@noverse/ui/data'
import { keys } from '../../data/keys'
import { notificationsEnabled, setNotificationUser } from '../../mobile/pwa'
import { navigate } from '../../router'
import { toast } from '@noverse/ui'
import {
  dockReducer, initialDock, MAX_TOASTS, TOAST_MS, toastKindFor,
  type DockEvent, type DockState, type ToastKind,
} from './logic'
import { copyText } from '../../lib/clipboard'

export type JobToast = { id: number; job: Job; kind: ToastKind; left: number; leaving: boolean }

export type JobsState = {
  jobs: Job[]
  loaded: boolean
  error: unknown
  refreshing: boolean
  updatedAt: number | null
  dock: DockState
  toasts: JobToast[]
  /** rê chuột vào chồng toast → dừng đếm, bung cả hai */
  hoverToasts: boolean
  notify: boolean
  /** id việc → số lần nháy nền mint (bảng đang mở khi việc xong) */
  flash: Record<string, number>
  /** dòng đang thu max-height trước khi bỏ */
  out: Record<string, true>
  /** tăng mỗi lần pill cần nảy + vòng sáng mint */
  bumpN: number
}

const TICK = 120
const TOAST_LEAVE_MS = 520
const ROW_OUT_MS = 300
const PANEL_IN_MS = 520
const PANEL_OUT_MS = 340

type Prefs = { mode: 'pill' | 'mini' | 'panel'; notify: boolean }
const prefKey = (user: string) => `ainovel.jobsDock.${user}`
const notifyGranted = () => typeof Notification !== 'undefined' && Notification.permission === 'granted'
function readPrefs(user: string): Prefs {
  try {
    const raw = localStorage.getItem(prefKey(user))
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<Prefs>
      return {
        mode: parsed.mode === 'mini' || parsed.mode === 'panel' ? parsed.mode : 'pill',
        notify: typeof parsed.notify === 'boolean' ? parsed.notify : notifyGranted(),
      }
    }
  } catch { /* localStorage bị chặn */ }
  return { mode: 'pill', notify: notifyGranted() }
}
function writePrefs(user: string, prefs: Prefs) {
  try { localStorage.setItem(prefKey(user), JSON.stringify(prefs)) } catch { /* đầy / bị chặn */ }
}

// Lưu ngay khi báo (kể cả bảng/notification), không chỉ khi bấm ×: tải lại hay SSE
// phát lại không được đọc lại cùng một kết quả. Mỗi lần chạy của một việc có finishedAt riêng.
const MAX_ANNOUNCEMENTS = 100
const announcementKey = (user: string) => `ainovel.jobsAnnouncements.${user}`
const completionKey = (job: Job) => JSON.stringify([job.id, job.finishedAt ?? ''])
let announced = new Set<string>()
function readAnnouncements(user: string): Set<string> {
  try {
    const entries: unknown = JSON.parse(localStorage.getItem(announcementKey(user)) ?? '[]')
    if (Array.isArray(entries)) return new Set(entries.filter((entry): entry is string => typeof entry === 'string').slice(-MAX_ANNOUNCEMENTS))
  } catch { /* localStorage bị chặn / dữ liệu cũ hỏng */ }
  return new Set()
}
function rememberCompletion(job: Job): boolean {
  const key = completionKey(job)
  if (announced.has(key)) return false
  announced.add(key)
  if (announced.size > MAX_ANNOUNCEMENTS) announced.delete(announced.values().next().value!)
  try { localStorage.setItem(announcementKey(username), JSON.stringify([...announced])) } catch { /* đầy / bị chặn: vẫn dedupe trong phiên */ }
  return true
}

let state: JobsState = {
  jobs: [], loaded: false, dock: initialDock, toasts: [], hoverToasts: false,
  notify: false, flash: {}, out: {}, bumpN: 0, error: null, refreshing: false, updatedAt: null,
}
const subs = new Set<() => void>()
const set = (patch: Partial<JobsState>) => { state = { ...state, ...patch }; subs.forEach((fn) => fn()) }
const subscribe = (fn: () => void) => { subs.add(fn); return () => { subs.delete(fn) } }

const doNotSubscribe = () => () => {}
export function useJobsState(enabled = true): JobsState {
  const snapshot = useMemo(() => {
    if (enabled) return () => state
    const frozen = state
    return () => frozen
  }, [enabled])
  return useSyncExternalStore(enabled ? subscribe : doNotSubscribe, snapshot, snapshot)
}

export function useJobsSelector<S>(select: (value: JobsState) => S): S {
  const snapshot = useMemo(() => {
    let selected = select(state)
    return () => {
      const next = select(state)
      if (!sameData(selected, next)) selected = next
      return selected
    }
  }, [select])
  return useSyncExternalStore(subscribe, snapshot, snapshot)
}

// ─── dock ───
let username = ''
let prefs: Prefs = { mode: 'pill', notify: false }
let settleTimer: ReturnType<typeof setTimeout> | undefined

const reducedMotion = () => typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches

export function dispatchDock(event: DockEvent) {
  const previous = state.dock
  const next = dockReducer(previous, event)
  if (next === state.dock) return
  const opening = next.panelAnim === 'in' && state.dock.panelAnim !== 'in'
  // Bảng mở → toast tắt hết (dòng việc nháy thay toast).
  set(opening ? { dock: next, toasts: [], hoverToasts: false } : { dock: next })
  if (next.mode !== 'hidden' && next.mode !== prefs.mode && !next.closingTo) {
    prefs = { ...prefs, mode: next.mode }
    if (username) writePrefs(username, prefs)
  }
  if (next.panelAnim !== previous.panelAnim || opening) {
    clearTimeout(settleTimer)
    if (next.panelAnim) {
      settleTimer = setTimeout(() => dispatchDock({ type: 'settle' }), reducedMotion() ? 0 : next.panelAnim === 'in' ? PANEL_IN_MS : PANEL_OUT_MS)
    }
  }
}

// ─── toasts ───
let toastSeq = 0
let tickTimer: ReturnType<typeof setInterval> | undefined

function ensureTicking() {
  if (tickTimer || !state.toasts.some((item) => !item.leaving)) return
  tickTimer = setInterval(() => {
    if (!state.toasts.some((item) => !item.leaving)) { clearInterval(tickTimer); tickTimer = undefined; return }
    if (state.hoverToasts) return
    let expired: number[] = []
    const toasts = state.toasts.map((item) => {
      // Toast “đang tải về” ghim lại cho tới khi tệp rơi vào máy (§AE).
      const pinned = isAudioExportJob(item.job) && exportLook(item.job).phase === 'dl'
      if (item.leaving || pinned) return item
      const left = item.left - TICK
      if (left <= 0) expired = [...expired, item.id]
      return { ...item, left }
    })
    set({ toasts })
    expired.forEach(dismissToast)
  }, TICK)
}

export function dismissToast(id: number) {
  if (!state.toasts.some((item) => item.id === id && !item.leaving)) return
  set({ toasts: state.toasts.map((item) => (item.id === id ? { ...item, leaving: true } : item)) })
  // Rơi vào pill (420 ms) + khung bọc thu lại sau 200 ms (320 ms) → bỏ hẳn, pill nảy.
  setTimeout(() => set({ toasts: state.toasts.filter((item) => item.id !== id), bumpN: state.bumpN + 1 }), reducedMotion() ? 0 : TOAST_LEAVE_MS)
}

export const setHoverToasts = (hoverToasts: boolean) => { if (state.hoverToasts !== hoverToasts) set({ hoverToasts }) }

function pushToast(job: Job, kind: ToastKind) {
  const fresh: JobToast = { id: ++toastSeq, job, kind, left: TOAST_MS, leaving: false }
  const live = [...state.toasts.filter((item) => item.job.id !== job.id), fresh]
  // Tối đa 2 toast: toast cũ nhất (không đang rời) rơi vào pill.
  const staying = live.filter((item) => !item.leaving)
  const overflow = staying.slice(0, Math.max(0, staying.length - MAX_TOASTS)).map((item) => item.id)
  set({ toasts: live })
  overflow.forEach(dismissToast)
  ensureTicking()
}

function notifyBrowser(job: Job, kind: ToastKind) {
  if (!state.notify || typeof document === 'undefined' || !document.hidden || !notifyGranted()) return
  if (kind === 'done' && notificationsEnabled()) return // PWA gửi qua service worker; không báo hai lần.
  const title = kind === 'done' ? (job.result?.title ?? job.title) : kind === 'err' ? `${job.title} dừng lại` : job.title
  const body = kind === 'done' ? (job.result?.sub ?? '') : kind === 'err' ? jobError(job) : (job.step ?? job.result?.sub ?? '')
  try { new Notification(title, { body, tag: job.id }) } catch { /* trình duyệt không cho tạo từ trang */ }
}

/** Báo khi xong: xin quyền ở lần bật đầu tiên; bị chặn thì giữ tắt. */
export async function toggleNotify() {
  const next = !state.notify
  if (next && typeof Notification === 'undefined') {
    toast.err('Trình duyệt này không hỗ trợ thông báo.')
    return
  }
  if (next && typeof Notification !== 'undefined' && Notification.permission === 'default') {
    const permission = await Notification.requestPermission().catch(() => 'denied' as NotificationPermission)
    if (permission !== 'granted') { toast.err('Trình duyệt đang chặn thông báo — bật lại trong cài đặt trang.'); return }
  } else if (next && typeof Notification !== 'undefined' && Notification.permission === 'denied') {
    toast.err('Trình duyệt đang chặn thông báo — bật lại trong cài đặt trang.')
    return
  }
  prefs = { ...prefs, notify: next }
  if (username) writePrefs(username, prefs)
  set({ notify: next })
}

// ─── dữ liệu ───
/** Việc xong đã được dọn khi dock tự ẩn — không hiện lại cho tới khi đổi trạng thái. */
const cleared = new Set<string>()

function upsert(job: Job) {
  const prev = state.jobs.find((item) => item.id === job.id)
  if (job.status === 'done' && cleared.has(completionKey(job))) return
  const jobs = prev ? state.jobs.map((item) => (item.id === job.id ? job : item)) : [...state.jobs, job]
  set({ jobs })
  if (!prev) dispatchDock({ type: 'jobAdded' })
  const kind = job.status === 'done' ? (rememberCompletion(job) ? 'done' : null) : toastKindFor(prev?.status, job.status)
  // Việc ghép audio: “chờ đủ giọng” không phải Cần bạn — dock/thẻ nổi đã hiện trạng thái.
  if (!kind || (isAudioExportJob(job) && kind === 'wait')) return
  notifyBrowser(job, kind)
  if (state.dock.mode === 'panel') {
    set({ flash: { ...state.flash, [job.id]: (state.flash[job.id] ?? 0) + 1 } })
    if (kind === 'done') set({ bumpN: state.bumpN + 1 })
  } else {
    if (kind === 'done') set({ bumpN: state.bumpN + 1 })
    pushToast(job, kind)
    // §AE — việc ghép do tab này tạo với Tự tải bật: tải thành phẩm ngay trong tab.
    if (kind === 'done' && isAudioExportJob(job) && exportWatchOf(job)?.auto) void autoDownloadExport(job)
  }
}

function remove(id: string) {
  if (!state.jobs.some((item) => item.id === id)) return
  set({ jobs: state.jobs.filter((item) => item.id !== id) })
}

/** Dock tự ẩn (xong hết + 4,2 s): dọn các mục xong. */
export function clearDone() {
  const done = state.jobs.filter((item) => item.status === 'done')
  done.forEach((item) => cleared.add(completionKey(item)))
  if (done.length) set({ jobs: state.jobs.filter((item) => item.status !== 'done') })
}

function replaceAll(jobs: Job[], initial: boolean) {
  const known = new Map(state.jobs.map((item) => [item.id, item]))
  const fresh = jobs.filter((item) => !(item.status === 'done' && cleared.has(completionKey(item))))
  if (initial) {
    fresh.filter((item) => item.status === 'done').forEach(rememberCompletion)
    set({ jobs: fresh, loaded: true })
    if (fresh.length) dispatchDock({ type: 'restore', mode: prefs.mode })
    return
  }
  // Kết nối lại: đồng bộ, báo những đổi trạng thái bị lỡ.
  const retained = state.jobs.filter((item) => fresh.some((next) => next.id === item.id))
  if (retained.length !== state.jobs.length) set({ jobs: retained })
  fresh.forEach((item) => { if (known.get(item.id) !== item) upsert(item) })
}

let unsubscribeEvents: (() => void) | undefined
let unsubscribeReconnect: (() => void) | undefined
let retry: ReturnType<typeof setTimeout> | undefined
let attempt = 0
let running = false
let generation = 0
let lastFetchAt = 0

async function connect(initial: boolean, current = generation) {
  lastFetchAt = Date.now()
  clearTimeout(retry)
  try {
    await cache.fetch(keys.jobs(), () => getJobs())
    const resource = cache.get<{ jobs: Job[] }>(keys.jobs())
    if (resource.error) throw resource.error
    const data = resource.data
    if (!data) return
    if (!running || current !== generation) return
    attempt = 0
    replaceAll(data.jobs, initial && !state.loaded)
    if (state.error !== null || state.updatedAt === null) set({ error: null, refreshing: false, updatedAt: Date.now() })
  } catch (error) {
    if (!running || current !== generation) return
    if (error instanceof ApiError && error.status === 401) return
    set({ error, refreshing: false })
    retry = setTimeout(() => void connect(initial, current), Math.min(1000 * 2 ** attempt++, 30000))
  }
}

export const refreshJobsNow = () => connect(!state.loaded)

function listen() {
  unsubscribeEvents = subscribeEvents(
    ['job.created', 'job.progress', 'job.waiting', 'job.failed', 'job.done', 'job.removed'],
    (data, meta) => {
      if (!data || typeof data !== 'object' || !('id' in data) || typeof data.id !== 'string') return
      if (meta.type === 'job.removed') remove(data.id)
      else upsert(data as Job)
    },
  )
  unsubscribeReconnect = events.onReconnect(() => { void connect(false) })
  addEventListener('focus', refreshJobs)
  addEventListener('online', refreshJobs)
  document.addEventListener('visibilitychange', refreshJobs)
}

function refreshJobs() {
  if (running && !document.hidden && Date.now() - lastFetchAt >= 15000) void connect(false)
}

let holders = 0

/** Dock (máy tính) / sheet việc nền (điện thoại) gọi khi mount, đã đăng nhập; nhiều nơi gọi vẫn một kết nối. */
export function startJobs(user: string) {
  holders++
  if (!running || username !== user) {
    stopJobs()
    if (username !== user) {
      cleared.clear()
      set({ jobs: [], loaded: false, toasts: [], flash: {}, out: {}, dock: initialDock, hoverToasts: false, error: null, refreshing: false, updatedAt: null })
    }
    username = user
    announced = readAnnouncements(user)
    prefs = readPrefs(user)
    running = true
    setNotificationUser(user)
    attempt = 0
    set({ notify: prefs.notify })
    if (state.dock.panelAnim) dispatchDock({ type: 'settle' })
    listen()
    ensureTicking()
    void connect(true)
  }
  let released = false
  return () => {
    if (released) return
    released = true
    if (--holders === 0) stopJobs()
  }
}

function stopJobs() {
  running = false
  setNotificationUser(null)
  generation++
  unsubscribeEvents?.(); unsubscribeEvents = undefined
  unsubscribeReconnect?.(); unsubscribeReconnect = undefined
  removeEventListener('focus', refreshJobs)
  removeEventListener('online', refreshJobs)
  document.removeEventListener('visibilitychange', refreshJobs)
  clearTimeout(retry)
  clearTimeout(settleTimer)
  clearInterval(tickTimer); tickTimer = undefined
}

// ─── hành động ───
export const pauseJob = (job: Job) => jobAction(job.id, 'pause').then(() => {})
export const resumeJob = (job: Job) => jobAction(job.id, 'resume').then(() => {})
export const retryJob = (job: Job) => jobAction(job.id, 'retry').then(() => {})

/** Huỷ / bỏ khỏi danh sách / bỏ qua: dòng thu max-height → 0 (300 ms) rồi bỏ. */
export async function cancelJob(job: Job) {
  set({ out: { ...state.out, [job.id]: true } })
  try {
    await jobAction(job.id, 'cancel')
  } catch (error) {
    const { [job.id]: _, ...out } = state.out
    set({ out })
    throw error
  }
  await new Promise<void>((resolve) => setTimeout(resolve, reducedMotion() ? 0 : ROW_OUT_MS))
  const { [job.id]: _, ...out } = state.out
  if (job.status === 'done') cleared.add(job.id)
  set({ out, jobs: state.jobs.filter((item) => item.id !== job.id) })
}

export const openHref = (href: string) => { if (href) navigate(href.replace(/^#/, '')) }

let canReveal: boolean | null = null

/** Nút kết quả của việc (§1.4): Tải xuống · Mở thư mục (/api/reveal, chép đường dẫn khi không mở được) · mở màn. */
export async function runAction(job: Job, action: JobAction) {
  if (action.kind === 'download') {
    const fallbackName = action.path?.split('/').pop() || 'export'
    if (action.href) await downloadJobFile(action.href, fallbackName)
    else if (action.path) await downloadExportFile(job.project, job.story, action.path)
    return
  }
  if (action.kind === 'reveal') {
    const input = { target: 'exports' as const, project: job.project, story: job.story }
    if (canReveal === null) canReveal = (await getSystem()).canReveal
    if (canReveal) {
      const result = await revealFolder(input)
      toast.ok(`Đã mở ${result.path}`)
    } else {
      const result = await getRevealPath(input)
      try { await copyText(result.path) }
      catch { throw new Error('Không chép được đường dẫn — hãy cho phép truy cập bộ nhớ tạm.') }
      toast.ok(`Đã chép đường dẫn ${result.path}`)
    }
    return
  }
  openHref(action.href ?? job.href)
}

// ─── §AE — ghép & xuất audio chạy nền (10-audio.md · 14-jobs-dock.md §AE) ───

/** Thử lại việc ghép bị lỗi — chạy tiếp từ chương lỗi; `{intro:false}` = Ghép không nhạc. */
export const retryExport = (job: Job, noMusic = false) =>
  jobAction(job.id, 'retry', noMusic ? { intro: false } : undefined).then(() => {})

/** Cập nhật % tải của toast/vạch đang hiện cho việc này (re-render qua store). */
export const patchDl = (job: Job, pct: number | null) => {
  setDownloadState(job.id, { phase: 'dl', pct })
  set({})
}

/** Tải thành phẩm của việc ghép (cử chỉ người dùng hoặc tự tải). Mọi nguồn: zip → tệp đầu. */
export async function downloadExport(job: Job, onProgress?: (pct: number | null) => void) {
  const source = exportDownloadSource(job)
  if (!source) throw new Error('Chưa có tệp để tải')
  if (source.href) return downloadJobFile(source.href, source.name, onProgress)
  return downloadExportFile(job.project, job.story, source.path ?? '', onProgress)
}

/** Mở tệp vừa tải trong tab mới (thay “Mở tệp” của desktop, web không mở file manager). */
export function openExportFile(job: Job) {
  const source = exportDownloadSource(job)
  if (!source) return
  const url = source.href ?? `/api/projects/${encodeURIComponent(job.project)}/stories/${encodeURIComponent(job.story)}/exports/file?path=${encodeURIComponent(source.path ?? '')}`
  window.open(url, '_blank', 'noopener')
}

/** Chia sẻ thành phẩm qua navigator.share (điện thoại); tải blob rồi gói File. */
export async function shareExportJob(job: Job) {
  const data = job.result?.data as AudioExportJobResult | undefined
  const files = (data?.files ?? []).filter((item) => /\.(m4b|mp3|wav|srt|zip)$/i.test(item.name))
  if (!navigator.share) throw new Error('Trình duyệt chưa hỗ trợ chia sẻ. Hãy dùng Tải xuống.')
  if (!files.length) throw new Error('Chưa có tệp sách nói để chia sẻ')
  const sharedFiles = await Promise.all(files.map(async (item) => {
    const url = item.url ?? `/api/projects/${encodeURIComponent(job.project)}/stories/${encodeURIComponent(job.story)}/exports/file?path=${encodeURIComponent(item.path ?? item.name)}`
    const response = await fetch(url, { credentials: 'same-origin' })
    if (!response.ok) throw new Error('Không tải được tệp để chia sẻ')
    return new File([await response.blob()], item.name, { type: response.headers.get('content-type') ?? 'application/octet-stream' })
  }))
  if (!navigator.canShare?.({ files: sharedFiles })) throw new Error('Thiết bị chưa hỗ trợ chia sẻ tệp. Hãy dùng Tải xuống.')
  await navigator.share({ title: exportWhat(job), files: sharedFiles })
}

/**
 * Tự tải khi ghép xong (§AE): chỉ với việc do tab này tạo và tab đang mở;
 * tab ẩn thì thôi — tải sau từ Xem bản xuất. Trình duyệt chặn → thẻ/toast cam.
 */
async function autoDownloadExport(job: Job) {
  if (typeof document !== 'undefined' && document.hidden) return
  setDownloadState(job.id, { phase: 'dl', pct: null })
  set({})
  try {
    if (autoDownloadBlocked()) throw new Error('blocked')
    await downloadExport(job, (pct) => patchDl(job, pct))
    setDownloadState(job.id, { phase: 'done', pct: 100 })
  } catch {
    setDownloadState(job.id, { phase: 'blocked', pct: null })
  }
  set({})
}

/** “Tải xuống” do người dùng bấm — vẫn qua pha 'dl' để toast/thẻ hiện vạch tải. */
export async function downloadExportManual(job: Job) {
  setDownloadState(job.id, { phase: 'dl', pct: null })
  set({})
  try {
    await downloadExport(job, (pct) => patchDl(job, pct))
    setDownloadState(job.id, { phase: 'done', pct: 100 })
  } catch (error) {
    setDownloadState(job.id, undefined)
    set({})
    throw error
  }
  set({})
}
