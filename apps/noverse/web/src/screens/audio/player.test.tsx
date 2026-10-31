// @vitest-environment happy-dom
// iOS chặn play() gọi sau microtask (NotAllowedError). Thẻ audio giả lập ở đây
// từ chối đúng như Safari: chỉ thẻ đã được play() đồng bộ trong cử chỉ mới phát tiếp.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mediaErrorText, playError, usePlayer, usePreviewAudio, type Player, type PreviewAudio } from './useAudio'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const iosBlocked = () => new DOMException(
  'The request is not allowed by the user agent or the platform in the current context, possibly because the user denied permission.',
  'NotAllowedError',
)

const roots: Root[] = []
const containers: HTMLElement[] = []
const unlocked = new WeakSet<HTMLMediaElement>()
let afterMicrotask = false
let playMock: ReturnType<typeof vi.fn>

/** Như iOS: play() sau microtask trên thẻ chưa mở khoá → NotAllowedError. */
async function mount(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root); containers.push(container)
  await act(async () => { root.render(node) })
  return { root, container }
}

beforeEach(() => {
  afterMicrotask = false
  playMock = vi.fn(function (this: HTMLMediaElement) {
    if (afterMicrotask && !unlocked.has(this)) return Promise.reject(iosBlocked())
    unlocked.add(this)
    return Promise.resolve()
  })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(playMock as unknown as HTMLMediaElement['play'])
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
  // Browser media events arrive after assigning src, not synchronously as in
  // happy-dom. Tests dispatch metadata/canplay at their real async cutpoints.
  vi.spyOn(HTMLMediaElement.prototype, 'src', 'set').mockImplementation(function (this: HTMLMediaElement, value: string) {
    this.setAttribute('src', value)
  })
})
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  for (const container of containers.splice(0)) container.remove()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

/** Queue một microtask như handler thật rồi chạy `run` — play() đồng bộ phải chạy trước. */
async function tap(button: HTMLElement, run: () => void) {
  await act(async () => {
    Promise.resolve().then(() => { afterMicrotask = true })
    run()
    button.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  })
}

describe('lỗi phát audio luôn là tiếng Việt', () => {
  it('NotAllowedError của iOS không lộ nguyên văn tiếng Anh', () => {
    expect(playError(iosBlocked())).toBe('Chạm nút phát để nghe.')
    expect(playError(new DOMException('x', 'NotSupportedError'))).toBe('Không phát được tệp âm thanh này.')
    expect(playError(new DOMException('x', 'AbortError'))).toBe('')
    expect(playError(new DOMException('x', 'EncodingError'))).toBe('Không phát được tệp âm thanh.')
  })

  it('lỗi mạng/giải mã của thẻ audio cũng là tiếng Việt', () => {
    const media = (code: number) => ({ error: { code } }) as unknown as HTMLMediaElement
    expect(mediaErrorText(media(2))).toContain('Không tải được tệp âm thanh')
    expect(mediaErrorText(media(4))).toContain('Không phát được tệp âm thanh')
    expect(mediaErrorText(media(1))).toBe('')
    expect(mediaErrorText(null)).toContain('Không phát được tệp âm thanh')
  })
})

describe('usePlayer phát trong cử chỉ chạm', () => {
  function PlayerPlayer({ src, autoplay = false, expose }: { src: string | null; autoplay?: boolean; expose: (player: Player) => void }) {
    const player = usePlayer(src, { autoplay })
    expose(player)
    return <button id="play" onClick={() => {
      Promise.resolve().then(() => { afterMicrotask = true })
      player.toggle()
    }}>Phát</button>
  }

  it('play() chạy đồng bộ trong handler, không sau microtask', async () => {
    let api!: Player
    const { container } = await mount(<PlayerPlayer src="/api/a.mp3" expose={(value) => { api = value }} />)
    await tap(container.querySelector('#play')!, () => {})
    expect(playMock).toHaveBeenCalledTimes(1)
    expect(api.error).toBe('')
  })

  it('giữ một thẻ audio cho mọi chương (không tạo thẻ mới sau khi đã mở khoá)', async () => {
    const RealAudio = window.Audio
    let created = 0
    class CountingAudio extends RealAudio { constructor() { super(); created += 1 } }
    vi.stubGlobal('Audio', CountingAudio)
    let api!: Player
    const { container, root } = await mount(<PlayerPlayer src="/api/a.mp3" expose={(value) => { api = value }} />)
    expect(created).toBe(1)
    await tap(container.querySelector('#play')!, () => {})
    // Chương kế tiếp: đổi src trên cùng thẻ, autoplay không cần chạm lại.
    await act(async () => { root.render(<PlayerPlayer src="/api/b.mp3" autoplay expose={(value) => { api = value }} />) })
    await act(async () => { await Promise.resolve() })
    expect(created).toBe(1)
    expect(api.error).toBe('')
    expect(playMock.mock.calls.length).toBeGreaterThanOrEqual(2)
    vi.unstubAllGlobals()
  })

  it('đổi chương bằng tap gọi play(url mới) ngay trong cử chỉ', async () => {
    let api!: Player
    await mount(<PlayerPlayer src="/api/a.mp3" expose={(value) => { api = value }} />)
    await act(async () => {
      Promise.resolve().then(() => { afterMicrotask = true })
      void api.play('/api/b.mp3').catch(() => {})
    })
    expect(playMock).toHaveBeenCalledTimes(1)
    const element = playMock.mock.instances[0] as HTMLMediaElement
    expect(element.src).toContain('/api/b.mp3')
    expect(new URL(element.src).searchParams.get('inline')).toBe('1')
    expect(api.error).toBe('')
  })

  it('bám currentTime từng khung hình trong 30 giây kể cả khi đứng tiếng và đổi tốc độ', async () => {
    const frames = new Map<number, FrameRequestCallback>()
    let frameID = 0
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
      frames.set(++frameID, callback)
      return frameID
    })
    vi.stubGlobal('cancelAnimationFrame', (id: number) => { frames.delete(id) })
    let api!: Player
    const { root } = await mount(<PlayerPlayer src="/api/a.mp3" expose={(value) => { api = value }} />)
    await act(async () => { await api.play() })
    const audio = playMock.mock.instances[0] as HTMLAudioElement
    await act(() => {
      audio.currentTime = 0
      audio.dispatchEvent(new Event('play'))
    })
    let mediaTime = 0
    let maxDrift = 0
    for (let frame = 1; frame <= 1800; frame++) {
      // 5 giây đứng tiếng: đồng hồ thật không nhích, dù khung hình vẫn chạy.
      if (frame < 600 || frame >= 900) mediaTime += (frame >= 1200 ? 1.5 : 1) / 60
      if (frame === 1200) await act(() => { api.setRate(1.5) })
      await act(() => {
        audio.currentTime = mediaTime
        const pending = [...frames.values()]
        frames.clear()
        for (const callback of pending) callback(frame * 1000 / 60)
      })
      expect(Math.abs(api.time - audio.currentTime)).toBeLessThanOrEqual(0.1)
      maxDrift = Math.max(maxDrift, Math.abs(api.time - audio.currentTime))
    }
    expect(maxDrift).toBe(0)
    expect(mediaTime).toBeGreaterThanOrEqual(30)
    expect(audio.playbackRate).toBe(1.5)
    await act(() => {
      audio.dispatchEvent(new Event('pause'))
    })
    expect(frames.size).toBe(0)
    await act(() => { root.unmount() })
    roots.splice(roots.indexOf(root), 1)
    expect(frames.size).toBe(0)
  })

  it('renews an expired redirect through same-origin authorization and restores the seek position', async () => {
    let api!: Player
    await mount(<PlayerPlayer src="/api/chapter.mp3" expose={(value) => { api = value }} />)
    await act(async () => { await api.play() })
    const audio = playMock.mock.instances[0] as HTMLAudioElement
    audio.currentTime = 43
    Object.defineProperty(audio, 'error', { configurable: true, value: { code: 2 } })
    const calls = vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length
    await act(() => { audio.dispatchEvent(new Event('error')) })
    expect(vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length).toBe(calls + 1)
    expect(new URL(audio.src).pathname).toBe('/api/chapter.mp3')
    expect(new URL(audio.src).searchParams.has('_media')).toBe(true)
    audio.currentTime = 0
    await act(() => { audio.dispatchEvent(new Event('loadedmetadata')) })
    expect(audio.currentTime).toBe(43)
    expect(playMock).toHaveBeenCalledTimes(2)
    await act(() => { audio.dispatchEvent(new Event('error')) })
    expect(vi.mocked(HTMLMediaElement.prototype.load).mock.calls.length).toBe(calls + 1)
    expect(api.error).not.toBe('')
  })

  it('refreshes stale same-origin media on seek and resume, preserving the requested time', async () => {
    let now = 1_000
    vi.spyOn(Date, 'now').mockImplementation(() => now)
    let api!: Player
    await mount(<PlayerPlayer src="/api/chapter.mp3" expose={(value) => { api = value }} />)
    await act(async () => { await api.play() })
    const audio = playMock.mock.instances[0] as HTMLAudioElement
    now += 241_000
    await act(() => { api.seek(67) })
    expect(new URL(audio.src).searchParams.get('_media')).toBe(String(now))
    await act(() => { audio.dispatchEvent(new Event('loadedmetadata')) })
    expect(audio.currentTime).toBe(67)
    await act(() => { api.pause() })
    now += 241_000
    await act(async () => { await api.play() })
    expect(new URL(audio.src).searchParams.get('_media')).toBe(String(now))
    audio.currentTime = 0
    await act(() => { audio.dispatchEvent(new Event('loadedmetadata')) })
    expect(audio.currentTime).toBe(67)
  })

  it('does not restart playback when a stale seek refreshes the source after natural end', async () => {
    let now = 1_000
    vi.spyOn(Date, 'now').mockImplementation(() => now)
    let api!: Player
    await mount(<PlayerPlayer src="/api/chapter.mp3" expose={(value) => { api = value }} />)
    await act(async () => { await api.play() })
    const audio = playMock.mock.instances[0] as HTMLAudioElement
    const calls = playMock.mock.calls.length
    await act(() => { audio.dispatchEvent(new Event('ended')) })
    now += 241_000
    await act(() => { api.seek(30) })
    await act(() => { audio.dispatchEvent(new Event('loadedmetadata')) })
    expect(audio.currentTime).toBe(30)
    expect(playMock.mock.calls.length).toBe(calls)
    await act(() => { audio.dispatchEvent(new Event('error')) })
    await act(() => { audio.dispatchEvent(new Event('loadedmetadata')) })
    expect(playMock.mock.calls.length).toBe(calls)
  })
})

describe('usePreviewAudio mở khoá trong cử chỉ', () => {
  function PreviewPlayer({ expose }: { expose: (api: PreviewAudio) => void }) {
    const preview = usePreviewAudio()
    expose(preview)
    return <button id="preview" onClick={() => {
      Promise.resolve().then(() => { afterMicrotask = true })
      void preview.play('seg-1', async () => '/api/seg.mp3').catch(() => {})
    }}>Nghe thử</button>
  }

  it('play() đầu tiên chạy đồng bộ trước khi chờ URL', async () => {
    let api!: PreviewAudio
    const { container } = await mount(<PreviewPlayer expose={(value) => { api = value }} />)
    await act(async () => {
      Promise.resolve().then(() => { afterMicrotask = true })
      container.querySelector<HTMLElement>('#preview')!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    })
    await act(async () => { for (let i = 0; i < 5; i += 1) await Promise.resolve() })
    // Lượt mở khoá đồng bộ + lượt phát thật sau khi URL về, cùng một thẻ.
    expect(playMock.mock.calls.length).toBe(2)
    expect(playMock.mock.instances[0]).toBe(playMock.mock.instances[1])
    expect(api.playingId).toBe('seg-1')
  })
})
