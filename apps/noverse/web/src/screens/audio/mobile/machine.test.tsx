import { renderToStaticMarkup } from 'react-dom/server'
import type { ReactNode } from 'react'
import { beforeEach, expect, it, vi } from 'vitest'
import type { JobsState } from '../../../shell/jobs/store'

/** §AT — thẻ Máy đọc đầu màn Nghe: 4 biến thể của MAudio.dc.html (tweak `machine`). */
const state = vi.hoisted(() => ({
  tts: null as Record<string, unknown> | null,
  playable: false,
  waiting: false,
}))
vi.mock('../../../router', () => ({ useRoute: () => ({ route: { name: 'audio', project: 'p', story: 's' }, search: new URLSearchParams() }), navigate: vi.fn(), paths: { settings: () => '/settings' } }))
vi.mock('../../../data', () => ({
  keys: { story: (...parts: unknown[]) => parts, config: (...parts: unknown[]) => ['config', ...parts], jobs: () => ['jobs'], tts: () => ['tts'] },
  invalidate: vi.fn(),
  useConnection: () => ({ writable: true }),
  useResource: (key: readonly string[]) => ({ data: key.includes('status') ? state.tts : null, error: null }),
  useAction: () => ({ run: vi.fn() }),
}))
vi.mock('../../../mobile/primitives', () => ({ MobilePage: ({ children }: { children: ReactNode }) => <main>{children}</main>, BottomSheet: () => null, ConfirmSheet: () => null, useMobileToast: () => vi.fn() }))
vi.mock('../../../shell/jobs/useJobs', () => ({
  // Chạy đúng selector trên kho việc rỗng — index cần mảng, thẻ nổi cần undefined.
  useJobSelection: <T,>(select: (state: JobsState) => T): T => select({ jobs: [] } as unknown as JobsState),
}))
vi.mock('./ExportSheet', () => ({ ExportSheet: () => null }))
vi.mock('../useAudio', () => ({
  errText: String, loadAudioScript: vi.fn(), useAudioAccess: () => ({ writable: true }),
  useAudioData: () => ({
    volume: { chapters: [{ chapter: 1, title: 'Chương một', status: state.playable ? 'locked' : 'todo', audio: state.playable ? { segments: 1, segmentsDone: 1, durationSec: 3 } : undefined }] },
    queue: state.waiting ? [{ jobId: 'h1', chapter: 1, state: 'queued', segmentsDone: 0, segments: 46, etaSec: 0, speaker: 'dan', percent: 0, waitingMachine: true }] : [],
    voices: [], cast: null, refresh: vi.fn(),
  }),
  usePlayer: () => ({ duration: 0, time: 0, rate: 1, playing: false }), usePreviewAudio: () => ({ stop: vi.fn() }),
}))
import MobileAudio from './index'

const render = () => renderToStaticMarkup(<MobileAudio project="p" story="s" volume={1} />)

beforeEach(() => { state.tts = null; state.playable = false; state.waiting = false })

it('không còn lối cài trên máy tính và nút Kiểm tra lại ở màn Nghe', () => {
  const html = render()
  expect(html).not.toContain('Cài bộ đọc trên máy tính')
  expect(html).not.toContain('Kiểm tra lại')
  expect(html).not.toContain('Chưa cài bộ đọc giọng nói')
})

it('chưa có máy đọc: thẻ giải thích máy đọc là gì, chương vẫn hiện trong danh sách', () => {
  state.tts = { state: 'missing', channel: 'hosted' }
  const html = render()
  expect(html).toContain('Chưa có máy đọc để tạo giọng')
  expect(html).toContain('Bạn vẫn nghe được chương đã có giọng')
  expect(html).toContain('Xem Giọng đọc')
  expect(html).toContain('ma-chapters')
})

it('máy đọc ngoại tuyến: câu ngắn của thẻ điện thoại và chip số chương đang chờ', () => {
  state.tts = { state: 'offline', channel: 'hosted', machine: { name: 'Mac mini' }, lastSeenAt: new Date(Date.now() - 60_000).toISOString(), queueWaiting: 1 }
  state.waiting = true
  const html = render()
  expect(html).toContain('Máy đọc chưa kết nối')
  // MAudio: câu của thẻ đầu màn ngắn hơn bản Tuỳ chỉnh — không nhắc “không phải lỗi”.
  expect(html).toContain('Lần cuối ')
  expect(html).not.toContain('không phải lỗi')
  expect(html).toContain('1 chương đang chờ')
})

it('đang chuẩn bị bộ đọc: tên bước và phần trăm, không mã lỗi', () => {
  state.tts = { state: 'provisioning', channel: 'hosted', step: 'deps', percent: 42 }
  const html = render()
  expect(html).toContain('Đang chuẩn bị bộ đọc · Cài thư viện')
  expect(html).toContain('42%')
  expect(html).toContain('Chương bấm Tạo giọng sẽ chờ tới khi xong')
})

it('chuẩn bị lỗi: câu lý do theo §8.2 và nút Thử lại, không lộ mã', () => {
  state.tts = { state: 'error', channel: 'hosted', error: { code: 'no_disk', message: 'no_disk: ENOSPC' } }
  const html = render()
  expect(html).toContain('Máy đọc không đủ chỗ trống')
  expect(html).toContain('Thử lại')
  expect(html).not.toContain('no_disk')
  expect(html).not.toContain('ENOSPC')
})

it('chương đã xếp hàng chờ máy đọc mang nhãn trung tính, không phải Dừng do lỗi', () => {
  state.tts = { state: 'offline', channel: 'hosted', machine: { name: 'Mac mini' }, queueWaiting: 1 }
  state.waiting = true
  const html = render()
  expect(html).toContain('Đang chờ máy đọc')
  expect(html).not.toContain('Dừng do lỗi')
})

it('không dùng từ nội bộ ngoài phần Nâng cao', () => {
  for (const tts of [
    { state: 'missing', channel: 'hosted' },
    { state: 'offline', channel: 'hosted', machine: { name: 'Mac mini' }, queueWaiting: 2 },
    { state: 'provisioning', channel: 'hosted', step: 'model', percent: 10, bytesDone: 1_048_576, bytesTotal: 608_174_080 },
    { state: 'error', channel: 'hosted', error: { code: 'start_failed' } },
  ]) {
    state.tts = tts
    const text = render().replace(/<[^>]+>/g, ' ')
    for (const word of ['worker', 'container', 'uv', 'python', 'ffmpeg', 'start_failed', 'model_failed', 'http://']) {
      expect(text.toLowerCase(), `${word} trong ${String(tts.state)}`).not.toContain(word)
    }
  }
})

it('vẫn cho nghe các chương đã có giọng khi máy đọc chưa sẵn sàng', () => {
  state.playable = true
  state.tts = { state: 'offline', channel: 'hosted', machine: { name: 'Mac mini' } }
  const html = render()
  expect(html).toContain('Nghe chương 1')
  expect(html).toContain('Chương một')
})

it('hàng chương có nút ⋯ mở thao tác đúng chương, tách khỏi nút phát', () => {
  state.playable = true
  const html = render()
  expect(html).toContain('ma-chapter-menu')
  expect(html).toContain('aria-label="Thao tác chương 1"')
  expect(html).toContain('aria-haspopup="dialog"')
})
