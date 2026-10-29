import { afterEach, expect, it, vi } from 'vitest'
import {
  activateAudioScenario, audioStateOf, chapterRows, defaultExportOptions, MockAudioExportConflict,
  queueTimelineOf, requestAudioExport, resetMockAudio, scriptFor,
} from './audio'

afterEach(resetMockAudio)

it('starts a trial export as a background job ({jobId}) and keeps the volume export it found', async () => {
  const state = audioStateOf('test-z', 'mobile-trial', () => {})
  activateAudioScenario(state, 'mobileExported')
  const full = structuredClone(state.exported)
  expect(full).not.toBeNull()

  // §AE: POST trả jobId ngay; thành phẩm tới qua job.* chứ không nằm trong response.
  const reply = requestAudioExport(state, { ...defaultExportOptions, format: 'mp3', chapters: [9], trial: true })
  expect(reply).toEqual({ jobId: `${state.project}/${state.story}/audio/${state.volume}/export` })
  expect(state.pendingExport?.options.trial).toBe(true)

  await vi.waitFor(() => expect(state.pendingExport).toBeNull(), { timeout: 4000, interval: 20 })
  // A7 — tên tệp theo quy tắc mới: {Tên ngắn} · Tập N · Chương NN · {tên} (bản thử).mp3
  expect([...state.exportArtifacts.keys()].some((key) => key.startsWith('tap-01/') && /· Chương 09 · .+ \(bản thử\)\.mp3$/.test(key))).toBe(true)
  expect(state.exported).toEqual(full)
})

it('refuses a second merge on the same volume while one is pending — 409 carries that job’s id', () => {
  const state = audioStateOf('test-z', 'mobile-conflict', () => {})
  activateAudioScenario(state, 'mobileExported')
  expect(chapterRows(state).some((row) => row.chapter === 12 && row.audio)).toBe(true)
  state.chapters[12].audioDone = 0
  state.scripts.set(12, (state.scripts.get(12) ?? []).map((segment) => ({ ...segment, audio: null })))

  // “Xuất khi xong”: việc nằm chờ đủ giọng → vẫn đang giữ tập này.
  const waiting = requestAudioExport(state, { ...defaultExportOptions, whenReady: true, chapters: [12] })
  const pending = state.pendingExport
  expect(pending).not.toBeNull()

  let conflict: unknown
  try { requestAudioExport(state, { ...defaultExportOptions, chapters: [9], trial: true }) } catch (error) { conflict = error }
  expect(conflict).toBeInstanceOf(MockAudioExportConflict)
  expect((conflict as MockAudioExportConflict).jobId).toBe(waiting.jobId)
  expect(state.pendingExport).toBe(pending)
})

it('keeps the real 32-chapter shape with only three prepared and voiced chapters', () => {
  const state = audioStateOf('test-z', 'mobile-unprepared', () => {})
  expect(activateAudioScenario(state, 'mobileUnprepared')).toBe(true)
  const rows = chapterRows(state)
  expect(rows).toHaveLength(32)
  expect(rows.filter((row) => row.status === 'todo')).toHaveLength(29)
  expect(rows[3]).toMatchObject({ chapter: 4, status: 'todo', durationSec: null, segments: 0, speakers: 0, audio: null })
  expect(scriptFor(state, 4)).toBeNull()
  expect(queueTimelineOf(state).qualityEtaSec).toBeUndefined()
})

it('keeps volumes isolated and lists unprepared chapters in the selected mock volume', () => {
  const metadata = { title: 'Truyện dài', chapterTitles: { 41: 'Tập hai bắt đầu' }, totalChapters: 64, volume: 1 }
  const first = audioStateOf('test-z', 'volume-switch', () => {}, metadata)
  const second = audioStateOf('test-z', 'volume-switch', () => {}, { ...metadata, volume: 2 } as typeof metadata)
  expect(second).not.toBe(first)
  expect(second.volume).toBe(2)
  expect(chapterRows(second).map(row => row.chapter)).toEqual(Array.from({ length: 24 }, (_, i) => i + 41))
  expect(chapterRows(second)[0]).toMatchObject({ title: 'Tập hai bắt đầu', status: 'todo', audio: null })
  expect(scriptFor(second, 41)).toBeNull()
  expect(second.jobs.size).toBe(0)
})
