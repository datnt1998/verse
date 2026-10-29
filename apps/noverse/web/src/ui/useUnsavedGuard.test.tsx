// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import { useRoute } from '../router'
import { useUnsavedGuard } from './useUnsavedGuard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); vi.restoreAllMocks() })

it('asks with the kit dialog for in-app navigation and keeps the page when declined', async () => {
  const nativeConfirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
  location.hash = '#/settings/writing'
  function Guarded() {
    useUnsavedGuard(true)
    return <a href="#/projects">Dự án</a>
  }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<Guarded />) })
  await act(async () => { container.querySelector('a')!.click() })
  expect(nativeConfirm).not.toHaveBeenCalled()
  expect(document.querySelector('[role="alertdialog"]')?.textContent).toContain('Bỏ thay đổi chưa lưu?')
  expect(location.hash).toBe('#/settings/writing')
})

it('keeps the route on back/forward hash changes until the kit dialog is answered', async () => {
  location.hash = '#/settings/writing'
  // Router đăng ký listener trước guard, như App thật; ghi mọi route đã render.
  const rendered: string[] = []
  function Route() { const name = useRoute().route.name; rendered.push(name); return <output>{name}</output> }
  function Guarded() { useUnsavedGuard(true); return null }
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<><Route /><Guarded /></>) })
  expect(container.textContent).toBe('settings')
  await act(async () => {
    location.hash = '#/projects'
    await new Promise((resolve) => setTimeout(resolve, 20)) // lib ES2022: chưa có Promise.withResolvers
  })
  expect(new Set(rendered)).toEqual(new Set(['settings']))
  expect(container.textContent).toBe('settings')
  expect(location.hash).toBe('#/settings/writing')
  expect(document.querySelector('[role="alertdialog"]')).not.toBeNull()
})
