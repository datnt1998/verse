// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { Route } from '../router'
import { t } from '../i18n/vi'
import { AppHeader } from './AppHeader'

const target = vi.hoisted(() => ({ value: null as { project: string; story: string } | null }))
vi.mock('../commands/target', () => ({ helpTarget: () => target.value, rememberHelpTarget: () => {} }))

const library = { projects: [{ id: 'dit-dang-audio', name: 'Dít đăng audio', legacy: false }], candidates: [] }
vi.mock('../api/library', () => ({
  useLibrary: () => ({ library, storyOf: () => null, ensureProject: () => {}, projectDetail: () => null }),
}))
vi.mock('../api/session', () => ({ useSession: () => ({ user: { username: 'admin', role: 'admin' } }) }))
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); target.value = null })

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

// Bàn viết: nút ẩn/hiện Đại cương + bảng phải phải là 32×32 bo 8 (Main.dc.html) và KHÔNG co lại.
// Trước đây là Button variant="icon" (36×36 bo 10); khi header hẹp flex ép nó còn ~19px nên hiện
// thành một “viên thuốc” dọc cạnh biểu tượng Noverse.
it('Bàn viết: hai nút panel đúng 32×32 bo 8, không co giãn, và biểu tượng vẫn là bản phẳng 22px', async () => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => {
    root.render(<AppHeader route={{ name: 'workspace', project: 'dit-dang-audio', story: 'tuoi-sang' }} />)
  })
  const toggles = [...container.querySelectorAll<HTMLButtonElement>('button[aria-pressed]')]
  expect(toggles.map((button) => button.getAttribute('aria-label'))).toEqual([t.nav.toggleOutline, t.nav.toggleDetail])
  for (const [index, button] of toggles.entries()) {
    expect(button.style.width).toBe('32px')
    expect(button.style.height).toBe('32px')
    expect(button.style.borderRadius).toBe('8px')
    expect(button.style.flex).toBe('0 0 auto')
    // Artboard: mở = nền surface-5 + chữ fg; đóng = trong suốt + fg-4 (Main.dc.html leftBtnBg/leftBtnFg).
    expect(button.style.background).toBe('var(--color-surface-5)')
    expect(button.style.color).toBe('var(--color-fg)')
    // Icon = span mask (web/src/ui/Icon.tsx), không phải <svg>; cỡ khoá bằng lớp StyleX.
    const icon = button.firstElementChild as HTMLElement | null
    expect(icon?.style.maskImage || icon?.style.webkitMaskImage).toContain(index === 0 ? 'sidebar-left' : 'sidebar-right')
  }

  const mark = container.querySelector<SVGSVGElement>('svg[aria-label="Noverse"]')!
  expect(mark.style.height).toBe('22px')
  // Dưới 32px: bản phẳng — ba điểm dừng mỗi quả, không bóng đổ, không ánh sáng (23-brand.md §1).
  expect(mark.querySelectorAll('radialGradient stop')).toHaveLength(12)
  expect(mark.querySelectorAll('g[filter]')).toHaveLength(0)
  expect(mark.querySelector('feDropShadow')).toBeNull()
})
