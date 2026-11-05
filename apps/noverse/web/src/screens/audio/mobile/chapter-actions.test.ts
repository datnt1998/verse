import { describe, expect, it } from 'vitest'
import type { AudioChapter, AudioQueueJob } from '../../../api/types'
import type { GenRow } from '../genWork'
import { chapterActions, type ChapterActionId, type ChapterActionItem } from './chapter-actions'

const chapter = (n: number, patch: Partial<AudioChapter> = {}): AudioChapter => ({
  chapter: n, title: `Tên chương ${n}`, status: 'locked', issues: 0, durationSec: 960, lockedAt: null,
  segments: 46, speakers: 5, changedSegments: 0,
  audio: { segments: 46, segmentsDone: 46, durationSec: 960, rendering: false, percent: 100 },
  ...patch,
})

const job = (patch: Partial<AudioQueueJob> = {}): AudioQueueJob => ({
  jobId: `j1`, chapter: 1, state: 'queued', segmentsDone: 0, segments: 46, etaSec: 0, speaker: 'dan', percent: 0,
  ...patch,
})

const row = (state: GenRow['state'], patch: Partial<AudioChapter> = {}, jobPatch: Partial<AudioQueueJob> = {}): GenRow => ({
  chapter: chapter(1, patch),
  job: ['run', 'queue', 'paused', 'err', 'hold'].includes(state) ? job(jobPatch) : null,
  state,
  position: 2,
})

const ctx = { ready: true, playing: false, writable: true, quality: 'standard' as const }
const ids = (items: ChapterActionItem[]) => items.map((item) => item.id)
const item = (items: ChapterActionItem[], id: ChapterActionId) => items.find((entry) => entry.id === id)

describe('menu thao tác một chương', () => {
  it('chương đã sửa: nghe/xuất/chia sẻ bản cũ, cập nhật chỉ đoạn đổi, tạo lại phải xác nhận', () => {
    const items = chapterActions(row('stale', { chapterChanged: true, changedSegments: 3 }), ctx)
    expect(ids(items)).toEqual(['play', 'export', 'share', 'update', 'redo'])
    expect(item(items, 'update')?.disabled).toBeFalsy()
    expect(item(items, 'redo')?.confirm).toBeTruthy()
    expect(item(items, 'redo')?.confirm?.destructive).toBeFalsy()
  })

  it('chương chưa có giọng chỉ có một việc: tạo giọng đúng chương này', () => {
    const items = chapterActions(row('new', { status: 'todo', segments: 0, durationSec: null, audio: null }), { ...ctx, ready: false })
    expect(ids(items)).toEqual(['create'])
    expect(item(items, 'create')?.disabled).toBeFalsy()
  })

  it('job lỗi thử lại được; mất job thì khoá thử lại', () => {
    const items = chapterActions(row('err', {}, { state: 'error', error: 'lỗi' }), { ...ctx, ready: false })
    expect(ids(items)).toEqual(['retry'])
    expect(item(items, 'retry')?.disabled).toBeFalsy()
    expect(item(chapterActions({ ...row('err'), job: null }, { ...ctx, ready: false }), 'retry')?.disabled).toBe(true)
  })

  it('chờ ở vị trí 2 có làm trước/tạm dừng/huỷ; vị trí 1 không có làm trước', () => {
    const items = chapterActions(row('queue'), { ...ctx, ready: false })
    expect(ids(items)).toEqual(['prioritize', 'pauseJob', 'cancelJob'])
    expect(item(items, 'cancelJob')?.confirm?.destructive).toBe(true)
    expect(ids(chapterActions({ ...row('queue'), position: 1 }, { ...ctx, ready: false }))).toEqual(['pauseJob', 'cancelJob'])
  })

  it('đang tạo thì tạm dừng + huỷ; đang tạm dừng thì tiếp tục + huỷ', () => {
    expect(ids(chapterActions(row('run', {}, { state: 'rendering', segmentsDone: 12 }), { ...ctx, ready: false }))).toEqual(['pauseJob', 'cancelJob'])
    expect(ids(chapterActions(row('paused', {}, { state: 'paused' }), { ...ctx, ready: false }))).toEqual(['resumeJob', 'cancelJob'])
  })

  it('chờ máy đọc / đang soạn bản đọc không có thao tác', () => {
    expect(chapterActions(row('hold'), { ...ctx, ready: false })).toEqual([])
    expect(chapterActions(row('composing'), { ...ctx, ready: false })).toEqual([])
  })

  it('chương đã có giọng: nghe, xuất riêng, chia sẻ, tạo lại — không có cập nhật', () => {
    expect(ids(chapterActions(row('done'), ctx))).toEqual(['play', 'export', 'share', 'redo'])
  })

  it('đang phát đúng chương thì mục đầu là tạm dừng phát', () => {
    expect(ids(chapterActions(row('done'), { ...ctx, playing: true }))[0]).toBe('pausePlayback')
  })

  it('tài khoản chỉ đọc vẫn nghe/chia sẻ được nhưng mọi thao tác ghi đều khoá', () => {
    const items = chapterActions(row('stale', { scriptChanged: true }), { ...ctx, writable: false })
    expect(item(items, 'play')?.disabled).toBeFalsy()
    expect(item(items, 'share')?.disabled).toBeFalsy()
    expect(item(items, 'export')?.disabled).toBe(true)
    expect(item(items, 'update')?.disabled).toBe(true)
    expect(item(items, 'redo')?.disabled).toBe(true)

    const queued = chapterActions({ ...row('queue'), position: 2 }, { ...ctx, writable: false, ready: false })
    expect(item(queued, 'prioritize')?.disabled).toBe(true)
    expect(item(queued, 'pauseJob')?.disabled).toBe(true)
    expect(item(queued, 'cancelJob')?.disabled).toBe(true)
  })
})
