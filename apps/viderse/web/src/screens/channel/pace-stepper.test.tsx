// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it, vi } from 'vitest'
import { PaceStepper } from './PaceStepper'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
it('commits Enter without allowing the containing create form to submit', async () => {
  const host = document.createElement('div'); document.body.append(host)
  const root = createRoot(host)
  const save = vi.fn()
  try {
    await act(async () => { root.render(<form><PaceStepper value={5} onSave={save} /></form>) })
    const input = host.querySelector('input')!
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, '7')
      input.dispatchEvent(new Event('input', { bubbles: true }))
    })
    const enter = new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })
    await act(async () => { input.dispatchEvent(enter) })
    expect(save).toHaveBeenCalledWith(7)
    expect(enter.defaultPrevented).toBe(true)
  } finally {
    await act(async () => root.unmount()); host.remove()
  }
})
