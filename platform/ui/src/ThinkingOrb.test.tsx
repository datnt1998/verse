// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ThinkingOrb } from './ThinkingOrb'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { document.body.replaceChildren(); vi.restoreAllMocks() })

type Call = { method: string; args: unknown[] }

/** Canvas 2D stub: ghi lại lời gọi vẽ. */
function stubCanvas(): { calls: Call[] } {
  const calls: Call[] = []
  let bound: HTMLCanvasElement | null = null
  const ctx = new Proxy({}, {
    get: (_target, prop) => {
      if (prop === 'canvas') return bound
      return (...args: unknown[]) => calls.push({ method: String(prop), args })
    },
    set: (_target, prop, value) => { calls.push({ method: `set ${String(prop)}`, args: [value] }); return true },
  })
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(function (this: HTMLCanvasElement) {
    bound = this
    return ctx as unknown as CanvasRenderingContext2D
  })
  return { calls }
}

function mount(size = 22, state: 'thinking' | 'idle' = 'thinking', label?: string) {
  const host = document.createElement('div')
  document.body.appendChild(host)
  act(() => { createRoot(host).render(<ThinkingOrb size={size} state={state} label={label} />) })
  return host.querySelector('canvas')!
}

const rafStub = () => {
  const queue: FrameRequestCallback[] = []
  vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => { queue.push(cb); return queue.length })
  vi.stubGlobal('cancelAnimationFrame', () => {})
  return queue
}

/** matchMedia stub với `matches` đổi được — component đọc mq.matches mỗi khung. */
const reduceMotion = (on: boolean) => {
  const mq = { get matches() { return flag.on }, media: '(prefers-reduced-motion: reduce)' }
  const flag = { on }
  vi.stubGlobal('matchMedia', (q: string) => (q.includes('reduce') ? mq : { matches: false, media: q }) as MediaQueryList)
  return flag
}

describe('ThinkingOrb — Main.dc.html startOrbit', () => {
  it('render canvas.orbit-cv 22px, role=img + aria-label', () => {
    stubCanvas(); rafStub(); reduceMotion(false)
    const cv = mount()
    expect(cv.classList.contains('orbit-cv')).toBe(true)
    expect(cv.getAttribute('role')).toBe('img')
    expect(cv.getAttribute('aria-label')).toBe('Writer is writing')
    expect(cv.style.width).toBe('22px')
    expect(cv.style.verticalAlign).toBe('-6px')
  })

  it('vẽ một frame ngay khi mount; mỗi khung 150 chấm, khung sau ≤30ms bị bỏ qua', () => {
    const { calls } = stubCanvas()
    const queue = rafStub(); reduceMotion(false)
    mount()
    // frame đầu vẽ đồng bộ — canvas không trống khi chờ rAF (fix SxJDesk)
    expect(calls.filter((c) => c.method === 'fill')).toHaveLength(150)
    expect(queue.length).toBe(1)
    act(() => { queue.shift()!(100) })
    expect(calls.filter((c) => c.method === 'fill')).toHaveLength(300)
    expect(calls.filter((c) => c.method === 'arc')).toHaveLength(300)
    // khung kế quá sớm → bỏ qua (giới hạn ~30fps)
    const before = calls.length
    act(() => { queue.shift()!(110) })
    expect(calls.length).toBe(before)
    act(() => { queue.shift()!(140) })
    expect(calls.filter((c) => c.method === 'fill')).toHaveLength(450)
  })

  it('prefers-reduced-motion → khung đứng yên t=1.2; tắt reduce thì quay lại', () => {
    const { calls } = stubCanvas()
    const queue = rafStub()
    const mq = reduceMotion(true)
    mount()
    // vẽ frame tĩnh ngay, không để canvas trống
    expect(calls.filter((c) => c.method === 'fill')).toHaveLength(150)
    const snapshots = () => calls.filter((c) => c.method === 'arc').map((c) => c.args.join(','))
    const frame1 = snapshots()
    // vòng lặp vẫn chạy nhưng mq.matches giữ t=1.2 → mọi khung giống hệt nhau (đứng yên)
    act(() => { queue.shift()!(500) })
    const frame2 = snapshots().slice(150)
    expect(frame2).toHaveLength(150)
    expect(frame2).toEqual(frame1)
    // đổi prefers-reduced-motion lúc đang chạy → frame kế chuyển động
    mq.on = false
    act(() => { queue.shift()!(1000) })
    const frame3 = snapshots().slice(300)
    expect(frame3).toHaveLength(150)
    expect(frame3).not.toEqual(frame1)
  })

  it('idle → vẫn vẽ 150 chấm, không sóng', () => {
    const { calls } = stubCanvas()
    const queue = rafStub(); reduceMotion(false)
    mount(22, 'idle')
    expect(calls.filter((c) => c.method === 'fill')).toHaveLength(150)
    act(() => { queue.shift()!(100) })
    expect(calls.filter((c) => c.method === 'fill')).toHaveLength(300)
  })

  it('label tùy biến', () => {
    stubCanvas(); rafStub(); reduceMotion(false)
    expect(mount(44, 'thinking', 'Đang chờ').getAttribute('aria-label')).toBe('Đang chờ')
  })
})
