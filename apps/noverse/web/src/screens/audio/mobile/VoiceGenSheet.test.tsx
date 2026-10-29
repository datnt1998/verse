// @vitest-environment happy-dom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { AudioChapter } from '../../../api/types'
import type { AudioData } from '../useAudio'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.mock('../../../api/client', () => ({ controlAudioQueue: vi.fn(), enqueueAudioChapters: vi.fn(), prioritizeAudioChapter: vi.fn() }))
vi.mock('../../../data', () => ({ keys: { story: (...parts: unknown[]) => parts, jobs: () => ['jobs'] }, useAction: () => ({ run: vi.fn(), busy: false, error: null }) }))
vi.mock('../../../mobile/primitives', () => ({
  BottomSheet: ({ open, children, label }: { open: boolean; children: ReactNode; label?: string }) => open ? <section role="dialog" aria-label={label}>{children}</section> : null,
  useMobileToast: () => vi.fn(),
}))
vi.mock('../../../shell/jobs/useJobs', () => ({ useJobSelection: () => false }))
vi.mock('../../../shell/jobs/store', () => ({ toggleNotify: vi.fn() }))
vi.mock('../useAudio', () => ({ errText: String }))
import { VoiceGenSheet } from './VoiceGenSheet'

const chapter = (n: number, patch: Partial<AudioChapter> = {}): AudioChapter => ({
  chapter: n, title: `Tên chương ${n}`, status: 'locked', issues: 0, durationSec: 960, lockedAt: null,
  segments: 46, speakers: 5, changedSegments: 0,
  audio: { segments: 46, segmentsDone: 46, durationSec: 960, rendering: false, percent: 100 },
  ...patch,
})
const data = (chapters = [1, 2, 3, 4, 5, 6, 7, 8].map((n) =>
  n === 1 ? chapter(n, { chapterChanged: true, changedSegments: 3 })
  : n === 4 ? chapter(n, { scriptChanged: true, changedSegments: 2 })
  : n === 7 ? chapter(n, { status: 'review', issues: 2, audio: null, durationSec: null })
  : n === 8 ? chapter(n, { status: 'todo', audio: null, segments: 0, durationSec: null })
  : chapter(n))): AudioData => ({
  volume: { chapters }, queue: [], cast: null, voices: [], settings: { quality: 'standard', autoOnLock: true, normalize: { numbers: true, dates: true, money: true, symbols: true, abbreviations: true, chapterTitles: true, dropBrackets: true } },
  timeline: { jobs: [], qualityEtaSec: { fast: 240, standard: 780, high: 1500 } },
  ready: null, flags: [], live: null, loading: false, refreshing: false, updatedAt: 1, error: '', conflict: null, composeError: '',
  savePending: false, saveSuccess: false, viewCurrent: vi.fn(), overwrite: vi.fn(), refresh: vi.fn(), reloadVoices: vi.fn(),
  saveCast: vi.fn(), saveSettings: vi.fn(), reloadQueue: vi.fn(), addFlag: vi.fn(), removeFlag: vi.fn(),
})
const props = { project: 'p', story: 's', volume: 1, open: true, onClose: vi.fn(), onVoice: vi.fn(), onExport: vi.fn(), writable: true }
let root: Root
let container: HTMLDivElement
afterEach(async () => { await act(() => root?.unmount()); container?.remove() })
async function mount(dataset = data()) {
  container = document.createElement('div'); document.body.append(container); root = createRoot(container)
  await act(() => root.render(<VoiceGenSheet {...props} data={dataset} />))
  return container
}

it('merges both old sheets into Cần làm → Đang tạo → Đã có giọng → Cách đọc → one footer button', async () => {
  const view = await mount()
  const text = view.textContent ?? ''
  expect(text).toContain('Tạo giọng · Tập 1')
  expect(text).toContain('Cần làm · 4')
  expect(text).toContain('2 chương sửa sau khi tạo giọng')
  expect(text).toContain('Cách đọc')
  expect(text).toContain('Tự tạo giọng khi chốt chương')
  expect(text).not.toContain('Hàng đợi')
  expect(text).not.toContain('Thêm chương')
  expect(text).not.toContain('Chốt & tạo')
  expect(view.querySelector('.vg-cta')?.textContent).toContain('Tạo giọng 4 chương')
})

it('preselects every stale and missing-voice row so the footer already knows the work', async () => {
  const view = await mount()
  const boxes = view.querySelectorAll('[role="checkbox"]')
  expect(boxes).toHaveLength(4)
  expect([...boxes].every((box) => box.getAttribute('aria-checked') === 'true')).toBe(true)
  expect(view.textContent).toContain('Chương đã sửa')
  expect(view.textContent).toContain('2 chỗ cần xem')
  expect(view.textContent).toContain('2 cập nhật · 2 mới · Chuẩn')
})

it('turns the edited-chapter warning card into a preselected Cần làm row, not a separate banner', async () => {
  const view = await mount()
  expect(view.querySelector('[role="alert"]')).toBeNull()
  expect(view.textContent).toContain('1. Tên chương 1')
  expect(view.textContent).toContain('3 đoạn đổi · AI soạn lại phần đổi rồi đọc lại')
  expect(view.textContent).toContain('2 đoạn đổi · chỉ đọc lại đoạn đổi')
})

it('shows running, queued and error rows in Đang tạo with their own labels', async () => {
  const rows = data()
  rows.queue = [
    { jobId: 'j1', chapter: 1, state: 'rendering', kind: 'update', segmentsDone: 2, segments: 3, etaSec: 60, speaker: 'dan', percent: 60 },
    { jobId: 'j2', chapter: 7, state: 'queued', segmentsDone: 0, segments: 46, etaSec: 780, speaker: 'dan', percent: 0 },
    { jobId: 'j3', chapter: 4, state: 'error', segmentsDone: 0, segments: 2, etaSec: 0, speaker: 'dan', percent: 0, error: 'Bộ đọc không phản hồi' },
    { jobId: 'j4', chapter: 8, state: 'queued', segmentsDone: 0, segments: 46, etaSec: 600, speaker: 'dan', percent: 0 },
  ]
  const view = await mount(rows)
  const text = view.textContent ?? ''
  expect(text).toContain('Đang tạo · 4')
  expect(text).toContain('Đang cập nhật')
  expect(text).toContain('Tiếp theo')
  expect(text).toContain('Dừng do lỗi')
  expect(text).toContain('Thử lại')
  expect(text).toContain('Làm trước')
  expect(text).toContain('Tạm dừng')
})

it('offers Nghe & xuất cả tập only when every chapter is up to date', async () => {
  const view = await mount(data([1, 2].map((n) => chapter(n))))
  expect(view.textContent).toContain('Cả 2 chương đã có giọng mới nhất')
  expect(view.textContent).toContain('Nghe & xuất cả tập')
  expect(view.textContent).not.toContain('Cập nhật 2 chương')
})
