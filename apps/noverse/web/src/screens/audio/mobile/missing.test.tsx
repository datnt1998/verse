import { renderToStaticMarkup } from 'react-dom/server'
import type { ReactNode } from 'react'
import { beforeEach, expect, it, vi } from 'vitest'
import type { JobsState } from '../../../shell/jobs/store'

const state = vi.hoisted(() => ({ missing: true, playable: false }))
vi.mock('../../../router', () => ({ useRoute: () => ({ route: { name: 'audio', project: 'p', story: 's' }, search: new URLSearchParams() }), navigate: vi.fn(), paths: { settings: () => '/settings' } }))
vi.mock('../../../data', () => ({ keys: { story: (...parts: unknown[]) => parts, config: (...parts: unknown[]) => ['config', ...parts], jobs: () => ['jobs'] }, useConnection: () => ({ writable: true }), useResource: () => ({ data: null, error: null }), useAction: () => ({ run: vi.fn() }) }))
vi.mock('../../../mobile/primitives', () => ({ MobilePage: ({ children }: { children: ReactNode }) => <main>{children}</main>, BottomSheet: () => null, useMobileToast: () => vi.fn() }))
vi.mock('../../../shell/jobs/useJobs', () => ({
  // Chạy đúng selector trên kho việc rỗng — index cần mảng, thẻ nổi cần undefined.
  useJobSelection: <T,>(select: (state: JobsState) => T): T => select({ jobs: [] } as unknown as JobsState),
}))
vi.mock('./ExportSheet', () => ({ ExportSheet: () => null }))
vi.mock('../useAudio', () => ({
  errText: String, loadAudioScript: vi.fn(), useAudioAccess: () => ({ writable: true }),
  useAudioData: () => ({ ready: { ready: !state.missing, engine: { ready: !state.missing, state: state.missing ? 'missing' : 'ready' } }, volume: { chapters: [{ chapter: 1, title: 'Chương một', status: state.playable ? 'locked' : 'todo', audio: state.playable ? { segments: 1, segmentsDone: 1, durationSec: 3 } : undefined }] }, queue: [], voices: [], cast: null, refresh: vi.fn() }),
  usePlayer: () => ({ duration: 0, time: 0, rate: 1, playing: false }), usePreviewAudio: () => ({ stop: vi.fn() }),
}))
import MobileAudio from './index'

beforeEach(() => { state.missing = true; state.playable = false })
it('M3 keeps waiting chapters visible even when the voice engine is missing', () => {
  const html = renderToStaticMarkup(<MobileAudio project="p" story="s" volume={1} />)
  expect(html).toContain('Chưa cài bộ đọc giọng nói')
  expect(html).toContain('ma-chapters')
  expect(html).toContain('Chưa soạn bản đọc')
  expect(html).not.toContain('Không tải lại được')
})
it('vẫn cho nghe các chương đã có giọng dù bộ đọc không còn trên máy', () => {
  state.playable = true
  const html = renderToStaticMarkup(<MobileAudio project="p" story="s" volume={1} />)
  expect(html).toContain('Nghe chương 1')
  expect(html).toContain('Chương một')
})
it('bộ đọc sẵn sàng có danh sách chờ bình thường', () => {
  state.missing = false
  const html = renderToStaticMarkup(<MobileAudio project="p" story="s" volume={1} />)
  expect(html).toContain('ma-chapters')
  expect(html).toContain('Chưa soạn bản đọc')
  expect(html).not.toContain('Chưa cài bộ đọc giọng nói')
})
