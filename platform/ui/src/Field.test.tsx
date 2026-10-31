// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it } from 'vitest'
import { Field, PasswordField } from './Field'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
it.each([Field, PasswordField])('preserves field chrome when composing a consumer class', async (Component) => {
  const el = document.createElement('div')
  const root = createRoot(el)
  try {
    await act(() => root.render(<Component className="consumer-height" aria-label="Mật khẩu" />))
    const input = el.querySelector('input')!
    expect(input.classList.contains('consumer-height')).toBe(true)
    expect(input.classList.length).toBeGreaterThan(1)
  } finally { await act(() => root.unmount()) }
})
