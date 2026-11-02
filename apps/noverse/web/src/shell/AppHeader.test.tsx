// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { Route } from '../router'
import { AppHeader } from './AppHeader'

const target = vi.hoisted(() => ({ value: null as { project: string; story: string } | null }))
const session = vi.hoisted(() => ({ user: null as null | { username: string; role: 'admin'; grants?: Record<string, unknown> | null } }))
vi.mock('../commands/target', () => ({ helpTarget: () => target.value, rememberHelpTarget: () => {} }))

const library = { projects: [{ id: 'dit-dang-audio', name: 'Dít đăng audio', legacy: false }], candidates: [] }
vi.mock('../api/library', () => ({
  useLibrary: () => ({ library, storyOf: () => null, ensureProject: () => {}, projectDetail: () => null }),
}))
vi.mock('../api/session', () => ({ useSession: () => ({ user: session.user }) }))
// Menu/kho có kiểm thử riêng; bài định tuyến header không khởi động truy vấn nền của chúng.
vi.mock('./AccountMenu', () => ({ AccountMenu: () => null }))
vi.mock('../screens/settings/storage/indicators', () => ({ StoragePill: () => null }))
vi.mock('../screens/project/useProject', async (importOriginal) => ({
  ...await importOriginal<typeof import('../screens/project/useProject')>(),
  useCanReveal: () => false,
}))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); target.value = null; session.user = null })

async function navOn(route: Route) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<AppHeader route={route} />) })
  const nav = container.querySelector('nav[aria-label="Điều hướng chính"]')!
  return (label: string) => [...nav.children].find((item) => item.textContent === label)?.getAttribute('href') ?? null
}

// Leaving Bàn viết for Dự án / Cài đặt used to lock Bàn viết, Trình đọc and Audio (no story in the path).
it('keeps Bàn viết, Trình đọc and Audio reachable from screens without a story, using the last opened story', async () => {
  target.value = { project: 'dit-dang-audio', story: 'tuoi-sang' }
  for (const route of [{ name: 'project', project: 'dit-dang-audio' }, { name: 'settings' }] as Route[]) {
    const href = await navOn(route)
    expect(href('Bàn viết')).toBe('#/p/dit-dang-audio/s/tuoi-sang/work')
    expect(href('Trình đọc')).toBe('#/p/dit-dang-audio/s/tuoi-sang/read')
    expect(href('Audio')).toBe('#/p/dit-dang-audio/s/tuoi-sang/audio')
    expect(href('Dự án')).toBe('#/p/dit-dang-audio')
  }
})

it('does not borrow a story from another project, and never renders a dead "#" link', async () => {
  target.value = { project: 'khac', story: 'x' }
  const href = await navOn({ name: 'project', project: 'dit-dang-audio' } as Route)
  expect(href('Bàn viết')).toBeNull()
  target.value = null
  const bare = await navOn({ name: 'settings' } as Route)
  expect(bare('Dự án')).toBe('#/')
  expect(bare('Truyện mới')).toBeNull()
})

it('names the project the root route opens instead of a dash', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(<AppHeader route={{ name: 'project', project: null }} />) })
  const switcher = container.querySelector('button[title="Đổi dự án"]')
  expect(switcher?.textContent).toContain('Dít đăng audio')
})


it('keeps a project option mounted through pointer-down so its click can switch projects', async () => {
  const other = { id: 'other-project', name: 'Dự án khác', legacy: false }
  library.projects.push(other)
  const originalHash = window.location.hash
  try {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container); roots.push(root)
    await act(async () => { root.render(<AppHeader route={{ name: 'project', project: 'dit-dang-audio' }} />) })
    const trigger = container.querySelector<HTMLButtonElement>('button[title="Đổi dự án"]')!
    await act(async () => { trigger.click() })
    await act(async () => {
      trigger.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
      trigger.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
      trigger.click()
    })
    expect(container.querySelector('[role="listbox"]')).toBeNull()
    await act(async () => { trigger.click() })
    const option = [...container.querySelectorAll<HTMLButtonElement>('[role="option"]')]
      .find((row) => row.textContent?.includes(other.name))!
    await act(async () => {
      option.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
      option.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }))
    })
    expect(option.isConnected).toBe(true)
    await act(async () => { option.click() })
    expect(window.location.hash).toBe('#/p/other-project')
    expect(container.querySelector('[role="listbox"]')).toBeNull()
  } finally {
    library.projects.pop()
    window.location.hash = originalHash
  }
})
