import { renderToStaticMarkup } from 'react-dom/server'
import type * as ReactModule from 'react'
import type * as ClientModule from '../../api/client'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getAudioScript } from '../../api/client'

const state = vi.hoisted(() => ({ effects: [] as (() => unknown)[], missing: true, volumeError: null as unknown, requested: [] as unknown[][], live: new Map<string, (data: unknown) => void>() }))
const builtIn = { id: 'Adam', name: 'Adam', gender: 'nam', builtIn: true }
const own = { id: 'giong-tri-ha', name: 'Giọng Tri Hạ', gender: 'nu', builtIn: false, durationSec: 18 }
// Tập 1 có chương 1 đang tạo giọng: đủ để kiểm tra cập nhật tiến độ tại chỗ.
const volumeData = () => ({ chapters: [{ chapter: 1, title: 'Một', status: 'review', issues: 0, durationSec: null, lockedAt: null, segments: 7, speakers: 1, audio: { segmentsDone: 0, segments: 7, durationSec: null, rendering: true, percent: 0 }, changedSegments: 0 }] })
const queueData = () => ({ jobs: [{ jobId: 'j1', chapter: 1, state: 'rendering', segmentsDone: 0, segments: 7, etaSec: 9, speaker: 'dan', percent: 0 }] })
vi.mock('react', async (original) => ({ ...await original<typeof ReactModule>(), useEffect: (effect: () => unknown) => { state.effects.push(effect) } }))
vi.mock('../../api/client', async (original) => ({ ...await original<typeof ClientModule>(), getAudioScript: vi.fn() }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: { role: 'admin', projects: {} } }) }))
vi.mock('../../data', () => ({
  keys: { story: (...parts: unknown[]) => ['story', ...parts], project: (...parts: unknown[]) => ['project', ...parts] },
  invalidate: vi.fn(), setData: vi.fn(), useConnection: () => ({ writable: true }),
  // Một luồng SSE cho mỗi tab: useAudioEvents đăng ký khung `live`, test phát khung qua map này.
  subscribeEvents: (type: string, handler: (data: unknown) => void) => { state.live.set(type, handler); return () => state.live.delete(type) },
  useAction: () => ({ pending: false, success: false, run: vi.fn() }),
  useResource: (key: unknown[]) => {
    state.requested.push(key)
    const ready = { ready: !state.missing, engine: { ready: !state.missing, state: state.missing ? 'missing' : 'ready' } }
    // GET /projects/:p/voices đã gộp giọng dự án và giọng có sẵn; /tts/voices chỉ có giọng có sẵn.
    const voices = key[0] === 'tts' ? [builtIn] : [own, builtIn]
    const data = key.includes('ready') ? ready
      : key.includes('volume') ? volumeData()
        : key.includes('queue') ? queueData()
          : key.includes('voices') ? voices : undefined
    const error = key[0] === 'tts' && state.missing ? new ApiError(503, 'Bộ đọc chưa được cài') : key.includes('volume') ? state.volumeError : null
    return { data, error, status: 'ready', refreshing: false, refresh: vi.fn() }
  },
}))
import { invalidate, setData } from '../../data'
import { applyRenderToQueue, applyRenderToVolume, errText, loadAudioScript, useAudioData, useAudioEvents } from './useAudio'

let voices: { id: string }[] = []
function Data() { const data = useAudioData('p', 's', 1); voices = data.voices; return <span>{data.error}</span> }

/** Phát một khung `live` đúng như /api/events phát (data = khung audio cũ). */
function emitLive(kind: string, payload: unknown) {
  const handler = state.live.get('live')
  if (!handler) throw new Error('useAudioEvents chưa đăng ký khung live')
  handler({ project: 'p', story: 's', kind, data: { type: kind, category: 'AUDIO', data: payload } })
}

beforeEach(() => { state.effects = []; state.requested = []; state.missing = true; state.volumeError = null; state.live.clear(); voices = []; vi.mocked(getAudioScript).mockReset() })

describe('trạng thái audio thật', () => {
  it('404 bản đọc là chưa soạn, không phải lỗi tải', async () => {
    vi.mocked(getAudioScript).mockRejectedValue(new ApiError(404, 'không tìm thấy'))
    await expect(loadAudioScript('p', 's', 1, 1)).resolves.toBeNull()
  })
  it('vẫn giữ lỗi tải khác và nội dung bản đọc đã có', async () => {
    const error = new ApiError(503, 'Máy chủ chưa sẵn sàng')
    vi.mocked(getAudioScript).mockRejectedValue(error)
    await expect(loadAudioScript('p', 's', 1, 1)).rejects.toBe(error)
    expect(errText('Không soạn được bản đọc')).toBe('Không soạn được bản đọc')
  })
  it('không lặp cảnh báo tải giọng khi thẻ chưa cài đã giải thích', () => {
    expect(renderToStaticMarkup(<Data />)).not.toContain('Bộ đọc chưa được cài')
    state.volumeError = new ApiError(500, 'Không đọc được tập')
    expect(renderToStaticMarkup(<Data />)).toContain('Không đọc được tập')
  })
  it('mỗi giọng chỉ xuất hiện một lần, giữ thứ tự giọng dự án rồi giọng có sẵn', () => {
    state.missing = false
    renderToStaticMarkup(<Data />)
    expect(voices.map((voice) => voice.id)).toEqual(['giong-tri-ha', 'Adam'])
    expect(state.requested.some((key) => key[0] === 'tts')).toBe(false)
  })
  it('đọc khung live để hiện lỗi soạn và cập nhật hàng đợi', () => {
    const onCompose = vi.fn(), onQueue = vi.fn(), onRender = vi.fn()
    function Events() { useAudioEvents('p', 's', { onCompose, onQueue, onRender }); return null }
    renderToStaticMarkup(<Events />)
    const cleanup = state.effects[0]()
    const compose = { chapter: 1, state: 'error', error: 'Không kết nối được nhà cung cấp AI' }
    emitLive('audio.compose', compose)
    emitLive('audio.queue', { jobs: [] })
    const render = { jobId: 'j1', chapter: 1, segment: 3, segments: 7, segmentId: 's0003', speaker: 'dan', percent: 42.857142857142854, etaSec: 4, state: 'rendering' }
    emitLive('audio.render', render)
    expect(onRender).toHaveBeenCalledWith(render)
    expect(onCompose).toHaveBeenCalledWith(compose)
    expect(onQueue).toHaveBeenCalledWith([])
    if (typeof cleanup === 'function') cleanup()
  })
  it('bỏ khung live của truyện khác', () => {
    const onRender = vi.fn()
    function Events() { useAudioEvents('p', 's', { onRender }); return null }
    renderToStaticMarkup(<Events />)
    state.effects[0]()
    state.live.get('live')!({ project: 'p', story: 'khac', kind: 'audio.render', data: { type: 'audio.render', data: { jobId: 'j1' } } })
    expect(onRender).not.toHaveBeenCalled()
  })
})

describe('tiến độ tạo giọng không tải lại màn', () => {
  // Đếm "fetch": trước khi sửa, mỗi sự kiện audio.render (mỗi đoạn!) gọi refresh()
  // → invalidate cả nhóm audio → tải lại 7 tài nguyên + skeleton/nháy. Sau khi sửa
  // chỉ setData tại chỗ; xong/hỏng chương mới làm mới đúng bản đọc của chương đó.
  it('chip và vòng phần trăm đổi tại chỗ, không gọi invalidate', () => {
    renderToStaticMarkup(<Data />)
    state.effects.forEach((effect) => effect())
    vi.mocked(invalidate).mockClear()
    vi.mocked(setData).mockClear()
    const render = { jobId: 'j1', chapter: 1, segment: 3, segments: 7, speaker: 'dan', percent: 42.9, etaSec: 4, state: 'rendering' }
    emitLive('audio.render', render)
    expect(vi.mocked(invalidate)).not.toHaveBeenCalled()
    const written = vi.mocked(setData).mock.calls.map(([key]) => key as unknown[])
    expect(written.some((key) => key.includes('queue'))).toBe(true)
    expect(written.some((key) => key.includes('volume'))).toBe(true)
  })
  it('tiến độ một đoạn vào đúng job và đúng chương, không dựng job/chương mới', () => {
    const render = { jobId: 'j1', chapter: 1, segment: 3, segments: 7, speaker: 'dan', percent: 42.9, etaSec: 4, state: 'rendering' as const }
    const jobs = applyRenderToQueue([{ jobId: 'j1', chapter: 1, state: 'rendering', segmentsDone: 0, segments: 7, etaSec: 9, speaker: 'dan', percent: 0 }], render)
    expect(jobs[0]).toMatchObject({ percent: 42.9, segmentsDone: 3, state: 'rendering' })
    const volume = applyRenderToVolume({ chapters: [{ chapter: 1, title: 'Một', status: 'review', issues: 0, durationSec: null, lockedAt: null, segments: 7, speakers: 1, audio: { segmentsDone: 0, segments: 7, durationSec: null, rendering: true, percent: 0 }, changedSegments: 0 }] }, render)
    expect(volume.chapters[0]?.audio).toMatchObject({ segmentsDone: 3, rendering: true })
    expect(applyRenderToQueue([], render)).toEqual([])
    expect(applyRenderToVolume({ chapters: [] }, render)).toEqual({ chapters: [] })
  })
  it('xong chương thì chỉ làm mới bản đọc của chương đó (một fetch, không phải cả nhóm)', () => {
    renderToStaticMarkup(<Data />)
    state.effects.forEach((effect) => effect())
    vi.mocked(invalidate).mockClear()
    const render = { jobId: 'j1', chapter: 1, segment: 7, segments: 7, speaker: 'dan', percent: 100, etaSec: 0, state: 'done' }
    emitLive('audio.render', render)
    expect(vi.mocked(invalidate).mock.calls).toEqual([[['story', 'p', 's', 'audio', 'script', 1, 1]]])
  })
  it('bỏ qua sự kiện khi hàng đợi/tập chưa tải xong, không dựng dữ liệu giả', () => {
    const render = { jobId: 'j1', chapter: 1, segment: 3, segments: 7, speaker: 'dan', percent: 42.9, etaSec: 4, state: 'rendering' as const }
    expect(applyRenderToQueue([], render)).toEqual([])
    expect(applyRenderToVolume({ chapters: [] }, render)).toEqual({ chapters: [] })
  })
})

it('keeps jobs from other volumes out of the active volume cache after SSE', () => {
  renderToStaticMarkup(<Data />)
  state.effects.forEach(effect => effect())
  vi.mocked(setData).mockClear()
  emitLive('audio.queue', { jobs: [queueData().jobs[0], { ...queueData().jobs[0], jobId: 'other-volume', chapter: 9 }] })
  const write = vi.mocked(setData).mock.calls.find(([key]) => (key as unknown[]).includes('queue'))
  expect(write?.[1]).toMatchObject({ jobs: [{ jobId: 'j1' }] })
})
