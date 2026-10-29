// @vitest-environment happy-dom
// A7 · Gói AF — Media Session: một bộ thông tin dùng cho màn khoá/thông báo/xe.
// Bản sai cũ: artist "Chưa chọn giọng", bìa chữ "CG", thiếu album — test khoá chống lặp.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useMediaSession, type MediaTrack } from './mediaSession'
import type { Player } from './useAudio'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
const containers: HTMLElement[] = []

let metadata: { title?: string; artist?: string; album?: string; artwork?: { src: string; sizes: string; type: string }[] } | null
let playbackState: string
let position: { duration: number; playbackRate: number; position: number } | undefined
let handlers: Map<string, ((details?: unknown) => void) | null>
let metadataCalls: number

class FakeMediaMetadata {
  title: string
  artist: string
  album: string
  artwork: { src: string; sizes: string; type: string }[]
  constructor(init: { title: string; artist?: string; album?: string; artwork?: { src: string; sizes: string; type: string }[] }) {
    this.title = init.title
    this.artist = init.artist ?? ''
    this.album = init.album ?? ''
    this.artwork = init.artwork ?? []
  }
}

const session = {
  setActionHandler(action: string, handler: ((details?: unknown) => void) | null) { handlers.set(action, handler) },
  setPositionState(state: typeof position) { position = state },
}
Object.defineProperty(session, 'metadata', { get: () => metadata, set: (value: typeof metadata) => { metadata = value }, configurable: true })
Object.defineProperty(session, 'playbackState', { get: () => playbackState, set: (value: string) => { playbackState = value }, configurable: true })

async function mount(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root); containers.push(container)
  await act(async () => { root.render(node) })
  return { root, container }
}

function player(): Player {
  return {
    playing: false, time: 0, duration: 0, rate: 1,
    setRate: vi.fn(), play: vi.fn(async () => {}), error: '', toggle: vi.fn(),
    pause: vi.fn(), seek: vi.fn(), skip: vi.fn(),
  }
}

function Probe({ track, player, prev, next }: { track: MediaTrack | null; player: Player; prev?: () => void; next?: () => void }) {
  useMediaSession(track, player, { onPrevious: prev, onNext: next })
  return null
}

beforeEach(() => {
  metadata = null
  playbackState = ''
  position = undefined
  metadataCalls = 0
  handlers = new Map()
  vi.stubGlobal('MediaMetadata', class extends FakeMediaMetadata {
    constructor(init: { title: string }) { super(init); metadataCalls++ }
  })
  Object.defineProperty(navigator, 'mediaSession', { value: session, configurable: true })
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  for (const container of containers.splice(0)) container.remove()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

const track4: MediaTrack = {
  project: 'dit-dang-audio', story: '01-tra-xanh', volume: 1,
  title: 'Chương 4 · Bảy mươi ba giây', album: 'Trà Xanh Cướp Kịch Bản · Tập 1', duration: 1171,
}
const track5: MediaTrack = { ...track4, title: 'Chương 5 · Gương cũ', duration: 900 }

describe('useMediaSession (A7)', () => {
  it('đặt đủ tên · Noverse Audio · album · bìa nhiều cỡ', async () => {
    await mount(<Probe track={track4} player={player()} />)
    expect(metadata?.title).toBe('Chương 4 · Bảy mươi ba giây')
    expect(metadata?.artist).toBe('Noverse Audio')
    expect(metadata?.album).toBe('Trà Xanh Cướp Kịch Bản · Tập 1')
    const sizes = (metadata?.artwork ?? []).map((item) => item.sizes)
    expect(sizes).toEqual(['96x96', '128x128', '192x192', '256x256', '384x384', '512x512'])
    expect(metadata?.artwork?.[0]).toMatchObject({ src: '/api/projects/dit-dang-audio/stories/01-tra-xanh/cover?size=96&volume=1', type: 'image/png' })
  })

  it('đổi chương cập nhật lại metadata', async () => {
    const { root } = await mount(<Probe track={track4} player={player()} />)
    await act(async () => { root.render(<Probe track={track5} player={player()} />) })
    expect(metadata?.title).toBe('Chương 5 · Gương cũ')
    expect(metadataCalls).toBe(2)
  })

  it('đăng ký nút tua 10 giây và chương trước/sau', async () => {
    const pl = player()
    const prev = vi.fn()
    const next = vi.fn()
    await mount(<Probe track={track4} player={pl} prev={prev} next={next} />)
    handlers.get('seekbackward')!({})
    expect(pl.skip).toHaveBeenCalledWith(-10)
    handlers.get('seekforward')!({ seekOffset: 10 })
    expect(pl.skip).toHaveBeenCalledWith(10)
    handlers.get('previoustrack')!({})
    expect(prev).toHaveBeenCalledTimes(1)
    handlers.get('nexttrack')!({})
    expect(next).toHaveBeenCalledTimes(1)
    handlers.get('play')!({})
    expect(pl.play).toHaveBeenCalled()
    handlers.get('pause')!({})
    expect(pl.pause).toHaveBeenCalled()
  })

  it('setPositionState kèm tốc độ phát và đổi theo thời gian', async () => {
    const pl = player()
    const { root } = await mount(<Probe track={track4} player={pl} />)
    expect(position).toEqual({ duration: 1171, playbackRate: 1, position: 0 })
    const moved = { ...pl, time: 312.5, rate: 1.25, playing: true }
    await act(async () => { root.render(<Probe track={track4} player={moved} />) })
    expect(position).toEqual({ duration: 1171, playbackRate: 1.25, position: 312.5 })
    expect(playbackState).toBe('playing')
  })

  it('vị trí phát kẹp trong khoảng thời lượng', async () => {
    const pl = { ...player(), time: 5000 }
    await mount(<Probe track={track4} player={pl} />)
    expect(position?.position).toBe(1171)
  })

  it('gỡ sạch khi unmount: metadata null, handler null', async () => {
    const { root } = await mount(<Probe track={track4} player={player()} prev={() => {}} next={() => {}} />)
    expect(metadata).not.toBeNull()
    await act(() => root.unmount())
    expect(metadata).toBeNull()
    for (const action of ['play', 'pause', 'seekbackward', 'seekforward', 'seekto', 'stop', 'previoustrack', 'nexttrack'])
      expect(handlers.get(action)).toBeNull()
    expect(playbackState).toBe('none')
  })

  it('mất track (chưa chọn chương) thì gỡ metadata ngay', async () => {
    const { root } = await mount(<Probe track={track4} player={player()} />)
    await act(async () => { root.render(<Probe track={null} player={player()} />) })
    expect(metadata).toBeNull()
  })
})
