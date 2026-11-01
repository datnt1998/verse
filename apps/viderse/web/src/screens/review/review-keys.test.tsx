import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useReviewKeys } from './useReviewKeys'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(() => { for (const root of roots.splice(0)) act(() => root.unmount()); document.body.innerHTML = '' })
function mount() {
  const calls = {
    nextJob: vi.fn(), previousJob: vi.fn(), nextImage: vi.fn(), previousImage: vi.fn(), redraw: vi.fn(), approve: vi.fn(),
    editPrompt: vi.fn(), compare: vi.fn(), escape: vi.fn(), submit: vi.fn(),
  }
  function Harness() { useReviewKeys(calls); return <><textarea aria-label="Góp ý" /><button>Nút</button></> }
  const node = document.createElement('div'); document.body.append(node); const root = createRoot(node); roots.push(root)
  act(() => root.render(<Harness />))
  return calls
}
describe('phím tắt duyệt', () => {
  it('ánh xạ J K mũi tên R A E V', () => {
    const calls = mount()
    for (const key of ['j', 'k', 'ArrowRight', 'ArrowLeft', 'r', 'a', 'e', 'v']) act(() => document.body.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })))
    for (const name of ['nextJob', 'previousJob', 'nextImage', 'previousImage', 'redraw', 'approve', 'editPrompt', 'compare'] as const) expect(calls[name]).toHaveBeenCalledTimes(1)
    expect(calls.escape).not.toHaveBeenCalled()
    expect(calls.submit).not.toHaveBeenCalled()
  })
  it('không chạy khi đang gõ hoặc có phím bổ trợ', () => {
    const calls = mount(); const input = document.querySelector('textarea')!
    act(() => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', bubbles: true })); input.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true })); document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', ctrlKey: true, bubbles: true })) })
    expect(calls.approve).not.toHaveBeenCalled(); expect(calls.nextJob).not.toHaveBeenCalled()
  })
  it('Esc huỷ và ⌘↵ gửi được cả khi con trỏ trong ô sửa prompt', () => {
    const calls = mount(); const input = document.querySelector('textarea')!
    act(() => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', metaKey: true, bubbles: true })) })
    act(() => { input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(calls.submit).toHaveBeenCalledTimes(1)
    expect(calls.escape).toHaveBeenCalledTimes(1)
    // Esc ngoài ô nhập cũng huỷ (bỏ chọn / đóng hộp thoại).
    act(() => { document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(calls.escape).toHaveBeenCalledTimes(2)
  })
})
