// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { dismissToasts, setToastDuration, toast, ToastHost } from './Toast'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let host: HTMLDivElement
let phone = false

beforeEach(async () => {
  vi.useFakeTimers()
  phone = false
  vi.stubGlobal('matchMedia', (query: string) => ({ matches: query.includes('max-width') && phone, addEventListener() {}, removeEventListener() {} }))
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root.render(<ToastHost activity />))
})
afterEach(async () => {
  await act(() => { dismissToasts(); root.unmount() })
  host.remove()
  setToastDuration(undefined)
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

async function advance(ms: number) { await act(() => { vi.advanceTimersByTime(ms) }) }

describe('application toast lifetime boundaries', () => {
  it.each([
    { name: 'Viderse plain', policy: 4000, mobile: false, undo: false, explicit: undefined, lifetime: 4000 },
    { name: 'Viderse undo', policy: 4000, mobile: false, undo: true, explicit: undefined, lifetime: 4000 },
    { name: 'Viderse phone', policy: 4000, mobile: true, undo: false, explicit: undefined, lifetime: 4000 },
    { name: 'explicit undo override', policy: 4000, mobile: false, undo: true, explicit: 6000, lifetime: 6000 },
    { name: 'Noverse desktop fallback', policy: undefined, mobile: false, undo: false, explicit: undefined, lifetime: 6000 },
    { name: 'Noverse phone fallback', policy: undefined, mobile: true, undo: false, explicit: undefined, lifetime: 2500 },
    { name: 'Noverse undo fallback', policy: undefined, mobile: true, undo: true, explicit: undefined, lifetime: 5000 },
  ])('$name remains readable until expiry, then leaves the mounted host', async ({ policy, mobile, undo, explicit, lifetime }) => {
    phone = mobile
    setToastDuration(4000)
    setToastDuration(policy)
    await act(() => { toast('Kết quả QA', 'ok', undo ? { label: 'Hoàn tác', onClick: () => {} } : undefined, { undo, duration: explicit }) })
    await advance(lifetime - 1)
    expect(host.textContent).toContain('Kết quả QA')
    // Let the existing exit animation finish; its CSS/class implementation is not the contract.
    await advance(601)
    expect(host.textContent).not.toContain('Kết quả QA')
  })

  it('keeps a background card while a timed result expires under the same application policy', async () => {
    setToastDuration(4000)
    await act(() => {
      toast.card({ title: 'Việc cần xử lý', tone: 'err', action: { label: 'Mở', onClick: () => {} } })
      toast.card({ title: 'Việc đã xong', action: { label: 'Xem', onClick: () => {} }, duration: 5000 })
    })
    await advance(4999)
    expect(host.textContent).toContain('Việc cần xử lý')
    expect(host.textContent).toContain('Việc đã xong')
    await advance(601)
    expect(host.textContent).not.toContain('Việc đã xong')
    await advance(20_000)
    expect(host.textContent).toContain('Việc cần xử lý')
  })

  it('replaces a keyed result and starts the replacement lifetime rather than the original expiry', async () => {
    await act(() => { toast('Lỗi cũ', 'err', undefined, { key: 'qa-error', duration: 1000 }) })
    await advance(800)
    await act(() => { toast('Lỗi mới', 'err', undefined, { key: 'qa-error', duration: 1000 }) })
    await advance(800)
    expect(host.textContent).toContain('Lỗi mới')
    expect(host.textContent).not.toContain('Lỗi cũ')
    await advance(800)
    expect(host.textContent).not.toContain('Lỗi mới')
  })
})
