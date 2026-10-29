// @vitest-environment happy-dom
// 17-kit › Phím Esc (chốt 25/10): một lần Esc chỉ đóng lớp trên cùng; lớp dưới và các listener
// Esc cũ của màn phía sau không được chạy.
import { act, useState, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { useEscapeLayer } from './Overlays'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | null = null
afterEach(() => { act(() => root?.unmount()); root = null; document.body.innerHTML = '' })

function Layer({ active, onClose }: { active: boolean; onClose: () => void }) {
  useEscapeLayer(active, onClose)
  return null
}
function mount(node: ReactNode) {
  const host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  act(() => root!.render(node))
}
const press = (key = 'Escape') => act(() => { document.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })) })

describe('lớp Esc', () => {
  it('hai lớp mở: một lần Esc chỉ đóng lớp trên cùng', () => {
    const under = vi.fn()
    const over = vi.fn()
    mount(<><Layer active onClose={under} /><Layer active onClose={over} /></>)
    press()
    expect(over).toHaveBeenCalledTimes(1)
    expect(under).not.toHaveBeenCalled()
  })

  it('listener Esc cũ (document/window) không chạy khi còn lớp mở', () => {
    const legacyDocument = vi.fn()
    const legacyWindow = vi.fn()
    document.addEventListener('keydown', legacyDocument)
    window.addEventListener('keydown', legacyWindow)
    const close = vi.fn()
    mount(<Layer active onClose={close} />)
    press()
    expect(close).toHaveBeenCalledTimes(1)
    expect(legacyDocument).not.toHaveBeenCalled()
    expect(legacyWindow).not.toHaveBeenCalled()
    document.removeEventListener('keydown', legacyDocument)
    window.removeEventListener('keydown', legacyWindow)
  })

  it('không còn lớp nào mở thì Esc tới màn phía sau (Bàn viết: Tạm dừng)', () => {
    const legacy = vi.fn()
    document.addEventListener('keydown', legacy)
    mount(<Layer active={false} onClose={vi.fn()} />)
    press()
    expect(legacy).toHaveBeenCalledTimes(1)
    document.removeEventListener('keydown', legacy)
  })

  it('đóng lớp trên rồi Esc đóng tiếp lớp dưới, rồi mới tới màn phía sau', () => {
    const legacy = vi.fn()
    document.addEventListener('keydown', legacy)
    const closed: string[] = []
    function Pair() {
      const [underOpen, setUnderOpen] = useState(true)
      const [topOpen, setTopOpen] = useState(true)
      useEscapeLayer(underOpen, () => { closed.push('under'); setUnderOpen(false) })
      useEscapeLayer(topOpen, () => { closed.push('top'); setTopOpen(false) })
      return null
    }
    mount(<Pair />)
    press()
    expect(closed).toEqual(['top'])
    expect(legacy).not.toHaveBeenCalled()
    press()
    expect(closed).toEqual(['top', 'under'])
    expect(legacy).not.toHaveBeenCalled()
    press()
    expect(closed).toEqual(['top', 'under'])
    expect(legacy).toHaveBeenCalledTimes(1)
    document.removeEventListener('keydown', legacy)
  })

  it('phím khác không bị chặn', () => {
    const close = vi.fn()
    const legacy = vi.fn()
    document.addEventListener('keydown', legacy)
    mount(<Layer active onClose={close} />)
    press('a')
    expect(close).not.toHaveBeenCalled()
    expect(legacy).toHaveBeenCalledTimes(1)
    document.removeEventListener('keydown', legacy)
  })
})
