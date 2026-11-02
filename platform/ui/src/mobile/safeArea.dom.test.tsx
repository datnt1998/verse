// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { useVisualViewportKeyboard } from './safeArea'

function ViewportConsumer() {
  useVisualViewportKeyboard()
  return null
}

it('closes keyboard state and recovers shell bounds without a follow-up scroll event', async () => {
  vi.useFakeTimers()
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true)
  const originalViewport = Object.getOwnPropertyDescriptor(window, 'visualViewport')
  const originalHeight = Object.getOwnPropertyDescriptor(window, 'innerHeight')
  const viewport = Object.assign(new EventTarget(), { height: 820, offsetTop: 24, scale: 1 })
  let layoutHeight = 844
  Object.defineProperty(window, 'visualViewport', { configurable: true, get: () => viewport })
  Object.defineProperty(window, 'innerHeight', { configurable: true, get: () => layoutHeight })
  const host = document.createElement('div')
  document.body.append(host)
  const root = createRoot(host)
  const css = document.documentElement.style
  try {
    await act(() => root.render(<ViewportConsumer />))
    expect(css.getPropertyValue('--kb')).toBe('0px')
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('844px')

    const input = document.createElement('textarea')
    host.append(input)
    await act(async () => {
      input.focus()
      viewport.height = 520
      viewport.dispatchEvent(new Event('resize'))
      await vi.advanceTimersByTimeAsync(20)
    })
    expect(css.getPropertyValue('--kb')).toBe('300px')
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('')

    await act(async () => {
      input.blur()
      await vi.advanceTimersByTimeAsync(20)
    })
    expect(css.getPropertyValue('--kb')).toBe('0px')
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('844px')

    // Safari finishes its close animation without another viewport event.
    layoutHeight = 900
    viewport.height = 876
    await act(async () => { await vi.advanceTimersByTimeAsync(350) })
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('900px')

    layoutHeight = 760
    viewport.height = 736
    await act(() => window.dispatchEvent(new Event('pageshow')))
    expect(css.getPropertyValue('--kb')).toBe('0px')
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('760px')

    await act(async () => {
      input.readOnly = true
      input.focus()
      viewport.height = 420
      viewport.dispatchEvent(new Event('resize'))
      await vi.advanceTimersByTimeAsync(20)
    })
    expect(css.getPropertyValue('--kb')).toBe('0px')

    await act(async () => {
      input.readOnly = false
      viewport.scale = 1.8
      viewport.height = 400
      viewport.offsetTop = 120
      viewport.dispatchEvent(new Event('resize'))
      await vi.advanceTimersByTimeAsync(20)
    })
    expect(css.getPropertyValue('--kb')).toBe('0px')
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('')

    await act(() => root.render(null))
    expect(css.getPropertyValue('--kb')).toBe('')
    expect(css.getPropertyValue('--m-viewport-bottom')).toBe('')
  } finally {
    await act(() => root.unmount())
    host.remove()
    if (originalViewport) Object.defineProperty(window, 'visualViewport', originalViewport)
    else Reflect.deleteProperty(window, 'visualViewport')
    if (originalHeight) Object.defineProperty(window, 'innerHeight', originalHeight)
    else Reflect.deleteProperty(window, 'innerHeight')
    vi.useRealTimers()
    vi.unstubAllGlobals()
  }
})
