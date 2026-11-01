import { expect, it } from 'vitest'
import type { AudioChapter, AudioQueueJob } from '../../api/types'
import { genAction, genCounts, genMeta, genQueueInput, genRows, genStrip, genSummary, genTag, preselect, rowEtaSec } from './genWork'

/** Bộ chương của MVoiceGen.dc.html: 1 · 4 đã sửa, 7 chưa chốt, 8 chưa soạn bản đọc, còn lại đã có giọng. */
const chapter = (n: number, patch: Partial<AudioChapter> = {}): AudioChapter => ({
  chapter: n, title: `Chương ${n}`, status: 'locked', issues: 0, durationSec: 960, lockedAt: null,
  segments: 46, speakers: 5, changedSegments: 0,
  audio: { segments: 46, segmentsDone: 46, durationSec: 960, rendering: false, percent: 100 },
  ...patch,
})
const rowsFixture = () => [1, 2, 3, 4, 5, 6, 7, 8].map((n) =>
  n === 1 ? chapter(n, { chapterChanged: true, changedSegments: 3 })
  : n === 4 ? chapter(n, { scriptChanged: true, changedSegments: 2 })
  : n === 7 ? chapter(n, { status: 'review', issues: 2, audio: null, segments: 46, durationSec: null })
  : n === 8 ? chapter(n, { status: 'todo', audio: null, segments: 0, durationSec: null })
  : chapter(n))

it('one label function: Cập nhật n chương when only updates, Tạo giọng n chương when new/redo are picked', () => {
  expect(genAction(2, 0, 0)).toEqual({ label: 'Cập nhật 2 chương', tone: 'update', icon: 'refresh-2' })
  expect(genAction(2, 2, 0)).toEqual({ label: 'Tạo giọng 4 chương', tone: 'create', icon: 'microphone-2' })
  expect(genAction(2, 1, 1)).toEqual({ label: 'Tạo giọng 4 chương', tone: 'create', icon: 'microphone-2' })
  expect(genAction(0, 0, 0)).toBeNull()
})

it('classifies chapters and preselects every Cần làm row (stale + missing voice)', () => {
  const rows = genRows(rowsFixture(), [])
  expect(rows.map((row) => row.state)).toEqual(['stale', 'done', 'done', 'stale', 'done', 'done', 'new', 'new'])
  expect([...preselect(rows)].sort((a, b) => a - b)).toEqual([1, 4, 7, 8])
  expect(genTag(rows[0])).toEqual({ text: 'Chương đã sửa', tone: 'warn' })
  expect(genTag(rows[3])).toEqual({ text: 'Bản đọc đã sửa', tone: 'warn' })
  expect(genTag(rows[6])).toEqual({ text: 'Chưa chốt', tone: '' })
  expect(genTag(rows[7])).toEqual({ text: 'Chưa soạn bản đọc', tone: '' })
})

it('sends needed/update chapters in chapters and re-read-from-scratch chapters in redo', () => {
  const rows = genRows(rowsFixture(), [])
  const picked = rows.filter((row) => [1, 4, 7, 2].includes(row.chapter.chapter))
  expect(genQueueInput(picked, 'high', 'cast')).toEqual({ chapters: [1, 4, 7], redo: [2], quality: 'high', voiceMode: 'cast', lock: true })
  const locked = genRows(rowsFixture().map((row) => ({ ...row, status: 'locked' as const })), [])
  expect(genQueueInput([locked[7]], 'fast', 'single').lock).toBe(false)
})

it('derives job kind, position and labels for queue rows without changing the chapter state', () => {
  const jobs: AudioQueueJob[] = [
    { jobId: 'j1', chapter: 1, state: 'rendering', kind: 'update', segmentsDone: 2, segments: 3, etaSec: 60, speaker: 'dan', percent: 60 },
    { jobId: 'j2', chapter: 4, state: 'queued', segmentsDone: 0, segments: 2, etaSec: 120, speaker: 'dan', percent: 0 },
    { jobId: 'j3', chapter: 7, state: 'queued', segmentsDone: 0, segments: 46, etaSec: 780, speaker: 'dan', percent: 0 },
    { jobId: 'j4', chapter: 8, state: 'error', segmentsDone: 0, segments: 0, etaSec: 0, speaker: 'dan', percent: 0, error: 'Bộ đọc không phản hồi' },
  ]
  const rows = genRows(rowsFixture(), jobs)
  expect(rows.map((row) => row.state)).toEqual(['run', 'done', 'done', 'queue', 'done', 'done', 'queue', 'err'])
  expect(rows[0].position).toBe(0)
  expect(rows[3].position).toBe(1)
  expect(rows[6].position).toBe(2)
  expect(genTag(rows[0])).toEqual({ text: 'Đang cập nhật', tone: 'run' })
  expect(genTag(rows[3])).toEqual({ text: 'Tiếp theo', tone: '' })
  expect(genTag(rows[6])).toEqual({ text: 'Chờ · thứ 2', tone: '' })
  expect(genTag(rows[7])).toEqual({ text: 'Dừng do lỗi', tone: 'err' })
})

it('holds queued chapters for the reader machine with a neutral label, never the error one', () => {
  const jobs: AudioQueueJob[] = [1, 4, 7, 8].map((n, index) => ({
    jobId: `h${n}`, chapter: n, state: 'queued' as const, segmentsDone: 0, segments: 46, etaSec: 0, speaker: 'dan', percent: 0,
    waitingMachine: true, ...(n === 8 ? { needsScript: true } : {}), ...(index === 0 ? { kind: 'update' as const } : {}),
  }) as AudioQueueJob & { waitingMachine?: boolean })
  const rows = genRows(rowsFixture(), jobs)
  expect(rows.map((row) => row.state)).toEqual(['hold', 'done', 'done', 'hold', 'done', 'done', 'hold', 'hold'])
  expect(genTag(rows[0])).toEqual({ text: 'Đang chờ máy đọc', tone: '' })
  expect(genTag(rows[0]).text).not.toBe('Dừng do lỗi')
  expect(genTag(rows[0]).tone).not.toBe('err')
  expect(genMeta(rows[0], {})).toBe('thứ 1 · cập nhật đoạn đổi')
  expect(genMeta(rows[3], {})).toBe('thứ 2 · cập nhật đoạn đổi')
  expect(genMeta(rows[6], {})).toBe('thứ 3 · tạo mới')
  expect(genMeta(rows[7], {})).toBe('thứ 4 · AI soạn bản đọc trước')
  const counts = genCounts(rows, new Set(), new Set())
  expect(counts).toMatchObject({ busy: 4, hold: 4, todo: 0, total: 0 })
  expect(genSummary(counts, { quality: 'standard' })).toBe('Không còn chương nào cần làm · 4 chương đang chờ máy đọc')
  const strip = genStrip(rows)
  expect(strip.cells.map((cell) => cell.cls)).toEqual(['hold', 'done', 'done', 'hold', 'done', 'done', 'hold', 'hold'])
  expect(strip.text).toBe('4/8 có giọng · 4 đang chờ máy đọc')
  expect(strip.legend.map((item) => item.color)).toEqual(['ok', 'hold'])
})

it('keeps queued jobs without the machine flag on their normal queue label', () => {
  const rows = genRows(rowsFixture(), [{ jobId: 'q1', chapter: 7, state: 'queued', segmentsDone: 0, segments: 46, etaSec: 780, speaker: 'dan', percent: 0 }])
  expect(rows[6].state).toBe('queue')
  expect(genTag(rows[6])).toEqual({ text: 'Tiếp theo', tone: '' })
})

it('summarises the footer like the artboard: parts, quality, estimate, lock note', () => {
  const rows = genRows(rowsFixture(), [])
  const counts = genCounts(rows, preselect(rows), new Set())
  expect(counts).toMatchObject({ update: 2, fresh: 2, redo: 0, total: 4, busy: 0, todo: 4 })
  expect(genSummary(counts, { quality: 'standard', etaSec: 1200 })).toBe('2 cập nhật · 2 mới · Chuẩn · khoảng 20 phút. Chương chưa chốt sẽ được chốt theo bản đọc hiện tại.')
  const doneAll = genRows(rowsFixture().map((row) => ({ ...row, chapterChanged: false, scriptChanged: false, audio: row.audio ?? { segments: 46, segmentsDone: 46, durationSec: 960, rendering: false, percent: 100 }, status: 'locked' as const })), [])
  expect(genSummary(genCounts(doneAll, new Set(), new Set()), { quality: 'fast' })).toBe('Cả 8 chương đã có giọng mới nhất')
  expect(genSummary({ update: 2, fresh: 0, redo: 1, busy: 3, hold: 0, todo: 0, unlockedNew: 0, total: 3, done: 5 }, { quality: 'high' }))
    .toBe('2 cập nhật · 1 tạo lại từ đầu · Cao nhất, sau các chương đang tạo')
})

it('keeps the status strip and legend consistent with the row states', () => {
  const strip = genStrip(genRows(rowsFixture(), []))
  expect(strip.cells.map((cell) => cell.cls)).toEqual(['stale', 'done', 'done', 'stale', 'done', 'done', 'new', 'new'])
  expect(strip.text).toBe('4/8 có giọng · 2 cần cập nhật · 2 chưa có')
  expect(strip.legend.map((item) => item.color)).toEqual(['ok', 'warn', 'muted'])
})

it('only estimates updates from updateEtaSec (or the measured quality scaled to changed segments)', () => {
  const rows = genRows(rowsFixture().map((row) => row.chapter === 1 ? { ...row, updateEtaSec: 90 } : row), [])
  expect(rowEtaSec(rows[0], 780)).toBe(90)
  expect(rowEtaSec(rows[3], 780)).toBe(34)
  expect(rowEtaSec(rows[6], undefined)).toBeUndefined()
})
