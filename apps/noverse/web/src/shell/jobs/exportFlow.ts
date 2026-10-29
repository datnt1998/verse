// Gói AE (28/10) — ghép & xuất audio chạy nền: công tắc “Tự tải về máy này khi xong”
// (nhớ theo thiết bị), sổ việc do tab này tạo, và mô hình hiển thị chung của
// toast máy tính (AudioExport.dc.html) lẫn thẻ nổi điện thoại (MAudio.dc.html).
// Thuần logic — không đụng React ở đây.
import { useSyncExternalStore } from 'react'
import type { AudioExportJobResult, Job, JobErrorInfo } from '../../api/types'
import { formatBytes, formatEta } from '../../screens/audio/logic'

// ─── Công tắc Tự tải — nhớ theo thiết bị (localStorage), mặc định bật ───
const AUTO_KEY = 'ainovel.audioExport.autoDownload'

export function getAutoDownload(): boolean {
  try { return localStorage.getItem(AUTO_KEY) !== '0' } catch { return true }
}
export function setAutoDownload(next: boolean) {
  try { localStorage.setItem(AUTO_KEY, next ? '1' : '0') } catch { /* đầy / bị chặn */ }
  autoListeners.forEach((fn) => fn())
}
const autoListeners = new Set<() => void>()
export function useAutoDownload(): [boolean, (next: boolean) => void] {
  const value = useSyncExternalStore(
    (fn) => { autoListeners.add(fn); return () => { autoListeners.delete(fn) } },
    getAutoDownload,
    getAutoDownload,
  )
  return [value, setAutoDownload]
}

// ─── Sổ việc ghép do chính tab này tạo (§AE: chỉ tab tạo mới tự tải) ───
export type ExportWatch = {
  /** "Tập 1" · "3 chương" · "chương 4" · "bản thử chương 4" */
  what: string
  /** động từ thường — 'ghép' (cả tập/bản thử) · 'xuất' (từng chương) */
  verb: string
  /** công tắc Tự tải lúc bấm */
  auto: boolean
  /** tải mọi tệp thay vì chỉ tệp chính (xuất lẻ nhiều tệp không zip) */
  all: boolean
}
const watches = new Map<string, ExportWatch>()
const pendingWatches = new Map<string, ExportWatch>()

export const exportJobKey = (project: string, story: string, volume: number) =>
  `${project}/${story}/audio/${volume}`

/** Gọi ngay khi bấm Ghép & xuất / Xuất N chương / Xuất thử — trước cả khi POST trả jobId. */
export function registerExportWatch(project: string, story: string, volume: number, watch: ExportWatch, jobId?: string) {
  if (jobId) watches.set(jobId, watch)
  else pendingWatches.set(exportJobKey(project, story, volume), watch)
}
export const clearExportWatch = (jobId: string) => { watches.delete(jobId) }

/** Việc ghép/xuất audio của một tập: id "<p>/<s>/audio/<v>/export" (và biến thể). */
export const isAudioExportJob = (job: Job) => job.kind === 'export' && /\/audio\/\d+\/export/.test(job.id)

export function exportWatchOf(job: Job): ExportWatch | undefined {
  let watch = watches.get(job.id)
  if (watch) return watch
  if (!isAudioExportJob(job)) return undefined
  const key = job.id.replace(/\/export.*$/, '')
  watch = pendingWatches.get(key)
  if (watch) { pendingWatches.delete(key); watches.set(job.id, watch) }
  return watch
}

// ─── Trạng thái tải trong tab (toast thanh vạch / thẻ dl) ───
export type DownloadState = { phase: 'dl' | 'done' | 'blocked'; pct: number | null }
const downloads = new Map<string, DownloadState>()
export const downloadStateOf = (job: Job) => downloads.get(job.id)
export const setDownloadState = (jobId: string, next: DownloadState | undefined) => {
  if (next) downloads.set(jobId, next); else downloads.delete(jobId)
}

/** Mô phỏng “trình duyệt chặn tự tải” (mock/thử tay): localStorage ainovel.dlBlock = '1'. */
export function autoDownloadBlocked(): boolean {
  try { return localStorage.getItem('ainovel.dlBlock') === '1' } catch { return false }
}

// ─── Lỗi việc (string hoặc §AE {message, chapter, cause}) ───
export function jobErrorInfo(job: Job): JobErrorInfo | null {
  const error = job.error
  if (error && typeof error === 'object') return error
  if (typeof error === 'string' && error) return { message: error }
  return null
}
export const jobError = (job: Job): string => jobErrorText(job.error)
/** Lời lỗi từ `Job.error` thô (string hoặc §AE) — dùng cho sự kiện `JobEvent.error` lẫn việc. */
export function jobErrorText(error: string | JobErrorInfo | null | undefined): string {
  if (!error) return ''
  return typeof error === 'string' ? error : error.message
}

/** Lỗi do nhạc mở đầu/kết thúc → nút “Ghép không nhạc” (retry {intro:false}). */
export const musicFailure = (job: Job) => /music|intro|outro/i.test(jobErrorInfo(job)?.cause ?? '')

// ─── Chữ trên toast/thẻ ───
const fallbackWhat = (job: Job) =>
  job.title.replace(/^Ghép\s*(?:&\s*xuất)?\s*(?:·\s*)?/i, '').replace(/^Xuất\s+(?:từng\s+)?(?:chương\s+)?(?:·\s*)?/i, '').trim() || job.title

export const exportWhat = (job: Job) => exportWatchOf(job)?.what ?? fallbackWhat(job)
export const exportVerb = (job: Job) => exportWatchOf(job)?.verb ?? (/^xuất/i.test(job.title) ? 'xuất' : 'ghép')
const cap = (text: string) => text.charAt(0).toLocaleUpperCase('vi-VN') + text.slice(1)

/** §AE — máy chủ đặt `files`/`zipUrl` ngay trên `result`; vẫn nhận nhánh `data` cho tương thích. */
export function exportData(job: Job) {
  const result = job.result as (NonNullable<Job['result']> & Partial<AudioExportJobResult>) | undefined
  const data = result && ('files' in result || 'zipUrl' in result) ? result : result?.data
  return data && typeof data === 'object' && ('files' in data || 'zipUrl' in data)
    ? data as AudioExportJobResult
    : undefined
}

/** Tên tệp hiển thị + dung lượng; zip ưu tiên, rồi tệp đầu. */
export function exportFilesLabel(job: Job): { name: string; size: string } {
  const data = exportData(job)
  const zip = data?.zipUrl
  const file = data?.files?.[0]
  if (zip) return { name: zip.split('/').pop() ?? 'gói.zip', size: '' }
  if (file) return { name: file.name, size: formatBytes(file.sizeBytes ?? 0) }
  return { name: job.result?.sub ?? '', size: '' }
}

/** Nguồn tải của thành phẩm: url đầy đủ (qua downloadJobFile) hoặc path (qua downloadExportFile). */
export function exportDownloadSource(job: Job): { href?: string; path?: string; name: string } | null {
  const data = exportData(job)
  const zip = data?.zipUrl
  const file = data?.files?.[0]
  if (zip) return zip.includes('?path=') || zip.startsWith('http')
    ? { href: zip, name: 'gói.zip' }
    : { path: zip, name: zip.split('/').pop() ?? 'gói.zip' }
  if (file?.url) return { href: file.url, name: file.name }
  if (file?.path) return { path: file.path, name: file.name }
  const action = job.result?.actions.find((item) => item.kind === 'download')
  if (action?.href) return { href: action.href, name: action.path?.split('/').pop() ?? 'export' }
  if (action?.path) return { path: action.path, name: action.path.split('/').pop() ?? 'export' }
  return null
}

/** Mọi tệp của xuất lẻ không zip (tải lần lượt). */
export function exportAllSources(job: Job): { href?: string; path?: string; name: string }[] {
  const files = exportData(job)?.files ?? []
  const out = files.map((file) => file.url
    ? { href: file.url, name: file.name }
    : { path: file.path, name: file.name })
  return out.length ? out : (exportDownloadSource(job) ? [exportDownloadSource(job)!] : [])
}

/** Route SPA của màn “Đã xuất” cho việc ghép tập này (Xem bản xuất / Xem tệp). */
export const exportedHref = (job: Job) => `${job.href.replace(/[?#].*$/, '')}?xem=da-xuat`

// ─── Mô hình hiển thị chung (toast máy tính · thẻ nổi điện thoại) ───
export type ExportPhase = 'run' | 'dl' | 'downloaded' | 'manual' | 'blocked' | 'fail'
export type ExportActionId = 'openFile' | 'openExport' | 'download' | 'share' | 'retry' | 'retryNoMusic'
export type ExportLook = {
  phase: ExportPhase
  title: string
  sub: string
  note?: string
  tone: 'ok' | 'warn' | 'err'
  /** thanh vạch: số % khi đang tải */
  bar: number | null
  actions: ExportActionId[]
}

/** Một nguồn chữ cho cả hai đầu: tham số khác nhau giữa máy tính và điện thoại nằm ở `mobile`. */
export function exportLook(job: Job, mobile = false): ExportLook {
  const what = exportWhat(job)
  const verb = exportVerb(job)
  const done = cap(verb === 'xuất' ? 'xuất' : 'ghép')
  const dl = downloadStateOf(job)
  const file = exportFilesLabel(job)
  const fileSub = [file.name, file.size].filter(Boolean).join(' · ')
  if (job.status === 'failed') {
    const info = jobErrorInfo(job)
    return {
      phase: 'fail',
      title: info?.chapter ? `${done} ${what} dừng ở chương ${info.chapter}` : `${done} ${what} dừng lại`,
      sub: info?.message ?? '',
      note: mobile
        ? 'Chương đã ghép được giữ lại; thử lại sẽ chạy tiếp.'
        : `Các chương đã ghép được giữ lại. Thử lại sẽ chạy tiếp từ chương ${info?.chapter ?? 'lỗi'}.`,
      tone: 'err', bar: null,
      actions: musicFailure(job) ? ['retry', 'retryNoMusic'] : ['retry'],
    }
  }
  if (job.status !== 'done') {
    const step = [job.step, unitLine(job)].filter(Boolean).join(' · ')
    return {
      phase: 'run',
      title: `Đang ${verb} ${what}`,
      sub: step ? `${step}${mobile ? ' · chạy nền, cứ nghe tiếp' : ''}` : 'Chạy nền',
      tone: 'ok', bar: job.progress !== null ? Math.round(job.progress * 100) : null,
      actions: [],
    }
  }
  if (dl?.phase === 'dl') {
    return {
      phase: 'dl', tone: 'ok', bar: dl.pct,
      title: mobile ? `Đã ${verb} ${what} · đang tải về` : `Đã ${verb} ${what} · đang tải về máy`,
      sub: fileSub, actions: [],
    }
  }
  if (dl?.phase === 'blocked') {
    return {
      phase: 'blocked', tone: 'warn', bar: null,
      title: mobile ? `Đã ${verb} ${what} · chưa tải được` : `Đã ${verb} xong ${what} · chưa tải được`,
      sub: fileSub,
      note: mobile
        ? 'Máy chặn tự tải. Bấm Tải xuống, hoặc Chia sẻ để lưu vào Tệp.'
        : 'Trình duyệt chặn tự tải. Bấm Tải xuống; muốn lần sau tự tải, chọn “Cho phép” khi trình duyệt hỏi.',
      actions: ['download', 'share'],
    }
  }
  if (dl?.phase === 'done') {
    return {
      phase: 'downloaded', tone: 'ok', bar: null,
      title: `Đã tải về máy${mobile ? '' : ' này'} · ${what}`,
      sub: mobile ? fileSub : `${fileSub} · trong thư mục Tải về`,
      actions: mobile ? ['openFile', 'share'] : ['openFile', 'openExport'],
    }
  }
  return {
    phase: 'manual', tone: 'ok', bar: null,
    title: `Đã ${verb} xong ${what}`,
    sub: fileSub,
    actions: ['download', mobile ? 'share' : 'openExport'],
  }
}

function unitLine(job: Job): string {
  const unit = job.unit
  if (!unit || unit.total <= 0) return ''
  const current = job.status === 'done' ? unit.total : Math.min(unit.total, Math.max(1, unit.done + 1))
  return `${unit.label} ${current}/${unit.total}`
}

/** Thông báo khi POST xuất lỗi — 409 “đang ghép cùng tập” có lời riêng. */
export function exportStartError(error: { status?: number; message?: string } | null | undefined): { text: string; conflict: boolean } | null {
  if (!error) return null
  if (error.status === 409) {
    // Lời máy chủ là lời chính; thêm chỉ dẫn dock nếu máy chủ chưa nói (10-audio.md §AE).
    const text = error.message || 'Tập này đang có một việc ghép đang chạy.'
    return { conflict: true, text: /Việc chạy nền/i.test(text) ? text : `${text.replace(/[.。]?$/, '.')} Theo dõi ở Việc chạy nền.` }
  }
  return { conflict: false, text: error.message || 'Không gửi được lệnh ghép. Thử lại.' }
}

/** Dòng phụ pill/dock khi ghép đang chạy: “Chương 3/8 · 34% · còn 46 giây”. */
export function exportRunLine(job: Job): string {
  return [unitLine(job), job.progress !== null ? `${Math.round(job.progress * 100)}%` : '', job.etaSec ? formatEta(job.etaSec) : '']
    .filter(Boolean).join(' · ')
}
