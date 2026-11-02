// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { expect, it } from 'vitest'
import { ActionMenu } from './Menus'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

it('keeps trigger menus open for an inside press and dismisses them on an outside press', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  try {
    await act(async () => { root.render(<ActionMenu label="Actions" items={[{ id: 'view', label: 'View' }]} />) })
    const trigger = container.querySelector<HTMLButtonElement>('button[aria-haspopup="menu"]')!
    await act(async () => { trigger.click() })
    const menu = document.querySelector<HTMLElement>('[role="menu"]')!
    expect(menu).not.toBeNull()
    await act(async () => { menu.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })) })
    expect(document.querySelector('[role="menu"]')).not.toBeNull()
    await act(async () => { document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })) })
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
  } finally {
    await act(async () => { root.unmount() })
    container.remove()
  }
})
