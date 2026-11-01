// @vitest-environment happy-dom
import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const mocks = vi.hoisted(() => ({ navigate: vi.fn(), pause: vi.fn(), player: vi.fn(), export: vi.fn(), queue: vi.fn(), pending: false }))
vi.mock('../../../router', () => ({
  useRoute: () => ({ search: new URLSearchParams() }), navigate: mocks.navigate,
  paths: { settings: () => '/settings/mobile', audio: (p: string, s: string, v: number) => `/p/${p}/s/${s}/audio/${v}/listen` },
}))
vi.mock('../../../data', () => ({
  keys: { story: (...parts: unknown[]) => parts, config: (...parts: unknown[]) => ['config', ...parts], jobs: () => ['jobs'], tts: () => ['tts'] },
  useConnection: () => ({ writable: true }), useAction: () => ({ run: vi.fn() }),
  useResource: (key: unknown[]) => ({ data: key.includes('outline') ? { volumes: [
    { index: 1, title: 'Có giọng', arcs: [{ chapters: [1] }], chapters: [] },
    { index: 2, title: 'Chưa tạo giọng', arcs: [{ chapters: [9, 10] }], chapters: [] },
    { index: 3, title: 'Tập khung', arcs: [], chapters: [] },
  ] } : null, error: null, refresh: vi.fn() }),
}))
vi.mock('../../../mobile/primitives', () => ({
  MobilePage: ({ title, actions, children }: { title: ReactNode; actions: ReactNode; children: ReactNode }) => <main>{title}{actions}{children}</main>,
  BottomSheet: ({ open, title, label, children }: { open: boolean; title?: string; label?: string; children: ReactNode }) => open ? <section role="dialog" aria-label={title ?? label}>{children}</section> : null,
  useMobileToast: () => vi.fn(),
}))
vi.mock('../../../shell/jobs/useJobs', () => ({ useJobSelection: () => [] }))
vi.mock('../../../shell/useNames', () => ({ useStoryName: () => 'Khi sếp học vibecode' }))
vi.mock('../packVoice', () => ({ usePackVoice: () => null }))
vi.mock('../mediaSession', () => ({ useMediaSession: vi.fn() }))
vi.mock('../../../ui/PanelRefreshBar', () => ({ PanelRefreshBar: () => null }))
vi.mock('./ExportJobCard', () => ({ ExportJobCard: () => null }))
vi.mock('./ExportSheet', () => ({ ExportSheet: (props: unknown) => { mocks.export(props); return null } }))
vi.mock('./VoiceGenSheet', () => ({ VoiceGenSheet: (props: unknown) => { mocks.queue(props); return null } }))
vi.mock('./voice', () => ({ VoiceSheet: () => null }))
vi.mock('../useAudio', () => ({
  loadAudioScript: vi.fn(), useAudioAccess: () => ({ writable: true }),
  useAudioData: (_p: string, _s: string, requested: number) => {
    const volume = mocks.pending ? 1 : requested
    return {
      volume: { chapters: (volume === 1 ? [1] : [9, 10]).map(chapter => ({ chapter, title: `Tập ${volume} chương ${chapter}`, status: 'todo', audio: volume === 1 ? { segments: 1, segmentsDone: 1, durationSec: 5 } : null })) },
      updatedAt: mocks.pending ? null : 123,
      queue: [], voices: [], cast: null, ready: { ready: false, engine: { ready: false, state: 'missing' } },
    }
  },
  usePlayer: (src: string | null) => { mocks.player(src); return { duration: 0, time: 0, rate: 1, playing: false, pause: mocks.pause } },
}))
import MobileAudio from './index'
let root: Root
let container: HTMLDivElement
afterEach(async () => { await act(() => root?.unmount()); container?.remove(); vi.clearAllMocks() })
async function mount(volume = 1) {
  container = document.createElement('div'); document.body.append(container); root = createRoot(container)
  await act(() => root.render(<MobileAudio project="p" story="s" volume={volume} />))
}
it('opens the volume picker, not More, and includes volumes without scripts/audio', async () => {
  await mount()
  await act(() => container.querySelector<HTMLButtonElement>('.ma-heading')!.click())
  expect(mocks.navigate).not.toHaveBeenCalled()
  const picker = container.querySelector('[role="dialog"][aria-label="Chọn tập"]')
  expect(picker).not.toBeNull()
  expect(picker?.textContent).toContain('Tập 2')
  const second = Array.from(picker!.querySelectorAll('button')).find(button => button.textContent?.includes('Tập 2'))!
  expect(second.disabled).toBe(false)
  await act(() => second.click())
  expect(mocks.pause).toHaveBeenCalled()
  expect(mocks.navigate).toHaveBeenCalledWith('/p/p/s/s/audio/2/listen')
  mocks.pending = true
  await act(() => root.render(<MobileAudio project="p" story="s" volume={2} />))
  expect(container.querySelector('.ma-heading')?.textContent).toContain('Tập 1')
  expect(mocks.player.mock.calls.at(-1)?.[0]).toContain('/volumes/1/')
  expect(mocks.export).toHaveBeenLastCalledWith(expect.objectContaining({ volume: 1 }))
  mocks.pending = false
  await act(() => root.render(<MobileAudio project="p" story="s" volume={2} />))
  expect(container.textContent).toContain('Tập 2 chương 9')
  expect(container.textContent).not.toContain('Tập 1 chương 1')
  expect(container.textContent).toContain('Chưa soạn bản đọc')
  expect(mocks.player).toHaveBeenLastCalledWith(null)
  expect(container.querySelector('.ma-mini')).toBeNull()
  expect(mocks.export).toHaveBeenLastCalledWith(expect.objectContaining({ volume: 2, readyChapters: [], trialChapter: 9 }))
  expect(mocks.queue).toHaveBeenLastCalledWith(expect.objectContaining({ volume: 2, data: expect.objectContaining({ queue: [] }) }))
})
