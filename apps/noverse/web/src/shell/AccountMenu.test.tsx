// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import type * as Router from '../router'
const fixture = vi.hoisted(() => ({ shared: true, navigate: vi.fn() }))
vi.mock('../api/session', () => ({ useSession: () => ({ user: { username: 'dat', name: 'Dat', role: 'admin', grants: fixture.shared ? { noverse: 'admin', viderse: 'admin' } : null }, apps: [], session: null, signOut: vi.fn() }) }))
vi.mock('../api/storage', () => ({ getRemembered: () => [], setNextRoute: vi.fn() }))
vi.mock('../data', () => ({ keys: { sessions: () => ['sessions'], users: () => ['users'] }, useResource: () => ({ data: { sessions: [], users: [] } }), useAction: (fn: () => Promise<unknown>) => ({ run: fn, busy: false, pending: false }) }))
vi.mock('../commands/target', () => ({ rememberHelpTarget: vi.fn() }))
vi.mock('../router', async (original) => ({ ...(await original<typeof Router>()), navigate: fixture.navigate }))
vi.mock('./version', () => ({ useVersionInfo: () => null, hasNewVersion: () => false, versionText: () => '' }))
vi.mock('./WhatsNew', () => ({ WhatsNewDialog: () => null }))
import { AccountMenu } from './AccountMenu'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root, host: HTMLDivElement
beforeEach(() => { fixture.shared = true; vi.clearAllMocks(); host = document.createElement('div'); document.body.append(host); root = createRoot(host) })
afterEach(async () => { await act(async () => root.unmount()); host.remove() })
it.each([false, true])('chỉ thu gọn lối điều hướng trong menu của kho chung, shared=%s', async (shared) => {
  fixture.shared = shared
  await act(async () => root.render(<AccountMenu />))
  await act(async () => host.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!.click())
  const actions = [...host.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')]
  const settings = actions.find((button) => button.textContent?.includes('Tuỳ chỉnh'))
  const help = actions.find((button) => button.textContent?.includes('Hướng dẫn lệnh'))
  if (shared) { expect(settings).toBeUndefined(); expect(help).toBeUndefined() }
  else {
    expect(settings).toBeDefined(); expect(help).toBeDefined()
    await act(async () => settings!.click())
    expect(fixture.navigate).toHaveBeenCalledWith('/settings')
    await act(async () => host.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!.click())
    await act(async () => [...host.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')].find((button) => button.textContent?.includes('Hướng dẫn lệnh'))!.click())
    expect(fixture.navigate).toHaveBeenCalledWith('/help/commands')
  }
})
