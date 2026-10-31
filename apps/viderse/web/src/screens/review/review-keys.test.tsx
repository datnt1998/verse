import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useReviewKeys } from './useReviewKeys'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(() => { for (const root of roots.splice(0)) act(() => root.unmount()); document.body.innerHTML = '' })
function mount() {
  const calls = { nextJob: vi.fn(), previousJob: vi.fn(), nextImage: vi.fn(), previousImage: vi.fn(), redraw: vi.fn(), approve: vi.fn() }
  function Harness() { useReviewKeys(calls); return <><textarea aria-label="Góp ý" /><button>Nút</button></> }
  const node = document.createElement('div'); document.body.append(node); const root = createRoot(node); roots.push(root)
  act(() => root.render(<Harness />))
  return calls
}
describe('phím tắt duyệt', () => {
  it('ánh xạ J K mũi tên R A', () => {
    const calls = mount()
    for (const key of ['j', 'k', 'ArrowRight', 'ArrowLeft', 'r', 'a']) act(() => document.body.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })))
    for (const call of Object.values(calls)) expect(call).toHaveBeenCalledTimes(1)
  })
  it('không chạy khi đang gõ hoặc có phím bổ trợ', () => {
    const calls = mount(); const input = document.querySelector('textarea')!
    act(() => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true })); input.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true })); document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true })) })
    expect(calls.approve).not.toHaveBeenCalled(); expect(calls.nextJob).not.toHaveBeenCalled()
  })
})
