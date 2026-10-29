// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest'
import type { Job } from '../../api/types'
import {
  exportLook, exportStartError, exportWatchOf, getAutoDownload, isAudioExportJob, musicFailure,
  registerExportWatch, setAutoDownload, setDownloadState,
} from './exportFlow'

const job = (over: Partial<Job> = {}): Job => ({
  id: 'p/s/audio/1/export', kind: 'export', title: 'Ghép Tập 1', status: 'running',
  progress: 0.34, step: 'Đang ghép chương 3/8', unit: { done: 2, total: 8, label: 'Chương' },
  etaSec: 46, href: '/p/p/s/s/audio/1', project: 'p', story: 's', startedAt: '2026-10-28T09:00:00Z',
  ...over,
})
const doneJob = (over: Partial<Job> = {}): Job => job({
  status: 'done', progress: 1, finishedAt: '2026-10-28T09:01:00Z',
  result: {
    title: 'Đã ghép Tập 1', sub: '1 tệp',
    actions: [
      { kind: 'download', label: 'Tải xuống', primary: true, href: '/api/projects/p/stories/s/exports/file?path=tap-01/a.mp3' },
      { kind: 'open', label: 'Xem bản xuất', href: '/p/p/s/s/audio/1?xem=da-xuat' },
    ],
    data: { files: [{ name: 'Trà xanh — Tập 1.mp3', sizeBytes: 151_000_000, url: '/api/projects/p/stories/s/exports/file?path=tap-01/a.mp3' }] },
  },
  ...over,
})

beforeEach(() => { localStorage.clear() })

describe('công tắc Tự tải — nhớ theo thiết bị', () => {
  it('mặc định bật, ghi localStorage và đọc lại', () => {
    expect(getAutoDownload()).toBe(true)
    setAutoDownload(false)
    expect(localStorage.getItem('ainovel.audioExport.autoDownload')).toBe('0')
    expect(getAutoDownload()).toBe(false)
    setAutoDownload(true)
    expect(getAutoDownload()).toBe(true)
  })
})

describe('nhận diện và sổ theo dõi việc ghép', () => {
  it('isAudioExportJob chỉ nhận việc export của audio', () => {
    expect(isAudioExportJob(job())).toBe(true)
    expect(isAudioExportJob(job({ id: 'p/s/export/3', title: 'Xuất EPUB' }))).toBe(false)
    expect(isAudioExportJob(job({ kind: 'audio' }))).toBe(false)
  })
  it('watch chưa có jobId được nhận khi job SSE đến', () => {
    registerExportWatch('p', 's', 1, { what: 'Tập 1', verb: 'ghép', auto: true, all: false })
    expect(exportWatchOf(job())).toMatchObject({ what: 'Tập 1', auto: true })
  })
})

describe('nhánh toast kết quả (AudioExport.dc.html)', () => {
  it('đang chạy: “Đang ghép Tập 1” + Chương 3/8', () => {
    registerExportWatch('p', 's', 1, { what: 'Tập 1', verb: 'ghép', auto: true, all: false }, 'p/s/audio/1/export')
    const look = exportLook(job())
    expect(look.phase).toBe('run')
    expect(look.title).toBe('Đang ghép Tập 1')
    expect(look.sub).toContain('Chương 3/8')
  })
  it('tự tải bật + đang tải → vạch tải', () => {
    const j = doneJob({ id: 'p/s/audio/1/export-dl' })
    setDownloadState(j.id, { phase: 'dl', pct: 40 })
    const look = exportLook(j)
    expect(look.phase).toBe('dl')
    expect(look.title).toContain('đang tải về máy')
    expect(look.bar).toBe(40)
  })
  it('tự tải xong → “Đã tải về máy này” + Mở tệp · Xem bản xuất', () => {
    const j = doneJob({ id: 'p/s/audio/1/export-dl-ok' })
    setDownloadState(j.id, { phase: 'done', pct: 100 })
    const look = exportLook(j)
    expect(look.phase).toBe('downloaded')
    expect(look.title).toBe('Đã tải về máy này · Tập 1')
    expect(look.sub).toContain('trong thư mục Tải về')
    expect(look.actions).toEqual(['openFile', 'openExport'])
  })
  it('tự tải tắt → “Đã ghép xong” + Tải xuống', () => {
    const look = exportLook(doneJob({ id: 'p/s/audio/1/export-manual' }))
    expect(look.phase).toBe('manual')
    expect(look.title).toBe('Đã ghép xong Tập 1')
    expect(look.actions).toEqual(['download', 'openExport'])
  })
  it('trình duyệt chặn → toast cam + Tải xuống', () => {
    const j = doneJob({ id: 'p/s/audio/1/export-blocked' })
    setDownloadState(j.id, { phase: 'blocked', pct: null })
    const look = exportLook(j)
    expect(look.phase).toBe('blocked')
    expect(look.tone).toBe('warn')
    expect(look.title).toContain('chưa tải được')
    expect(look.actions[0]).toBe('download')
  })
  it('lỗi ở chương: tiêu đề mang số chương + Thử lại + Ghép không nhạc khi lỗi nhạc', () => {
    const j = job({ status: 'failed', progress: 0.62, error: { message: 'Nhạc mở đầu đã bị xoá khỏi Tài sản', chapter: 5, retryable: true, cause: 'intro_missing' } })
    const look = exportLook(j)
    expect(look.phase).toBe('fail')
    expect(look.title).toBe('Ghép Tập 1 dừng ở chương 5')
    expect(look.sub).toBe('Nhạc mở đầu đã bị xoá khỏi Tài sản')
    expect(look.actions).toEqual(['retry', 'retryNoMusic'])
    expect(musicFailure(j)).toBe(true)
    expect(musicFailure(job({ status: 'failed', error: { message: 'Hết bộ nhớ', cause: 'engine' } }))).toBe(false)
  })
})

describe('lỗi gửi lệnh ghép', () => {
  it('409 “đang ghép cùng tập” có lời riêng + chỉ dẫn dock', () => {
    const out = exportStartError({ status: 409 })
    expect(out?.conflict).toBe(true)
    expect(out?.text).toContain('một việc ghép đang chạy')
    expect(out?.text).toContain('Việc chạy nền')
  })
  it('409 kèm lời máy chủ thì giữ lời máy chủ, thêm chỉ dẫn nếu thiếu', () => {
    expect(exportStartError({ status: 409, message: 'Tập 2 đang xếp hàng' })?.text).toBe('Tập 2 đang xếp hàng. Theo dõi ở Việc chạy nền.')
    expect(exportStartError({ status: 409, message: 'Đang ghép — xem Việc chạy nền.' })?.text).toBe('Đang ghép — xem Việc chạy nền.')
  })
  it('lỗi khác hiện lời máy chủ, không phải conflict', () => {
    const out = exportStartError({ status: 503, message: 'Máy chưa có ffmpeg để ghép audio' })
    expect(out?.conflict).toBe(false)
    expect(out?.text).toBe('Máy chưa có ffmpeg để ghép audio')
  })
})
