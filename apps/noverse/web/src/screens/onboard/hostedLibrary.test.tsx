// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import * as api from '../../api/client'
import type { ConfigPresets, ConfigResponse, LibraryResponse, SystemInfo, User } from '../../api/types'
import { LibraryProvider } from '../../api/library'
import { cache } from '@noverse/ui/data'
import { navigate } from '../../router'
import { OnboardScreen } from './index'
import { LibraryRootRows } from '../settings/sections/LibraryRootRow'
import { useValidation } from '../project/useProject'

vi.mock('../../api/client', async (original) => ({
  ...await original<typeof import('../../api/client')>(),
  getLibrary: vi.fn(), getConfig: vi.fn(), getConfigPresets: vi.fn(), getSystem: vi.fn(),
  getFsPlaces: vi.fn(), inspectLibraryPath: vi.fn(), putLibrary: vi.fn(), putLibraryRoot: vi.fn(),
  validateProject: vi.fn(),
}))
const user: User = {
  username: 'hosted-admin', name: 'Quản trị', role: 'admin', projects: {}, disabled: false,
  createdAt: '', updatedAt: '', onboardedAt: null, mustChangePassword: false, lastSeenAt: null,
}
vi.mock('../../api/session', () => ({ useSession: () => ({ user, session: null, refresh: vi.fn() }) }))
vi.mock('../../shell/AppShell', () => ({ useMediaQuery: () => false }))
vi.mock('../../router', async (original) => ({ ...await original<typeof import('../../router')>(), navigate: vi.fn() }))
vi.mock('../../data/Provider', async (original) => ({ ...await original<typeof import('../../data/Provider')>(), useConnection: () => ({ writable: true }) }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
vi.stubGlobal('matchMedia', () => ({ matches: false, addEventListener() {}, removeEventListener() {} }))
const roots: Root[] = []
const library = (filesystemLibrary: boolean): LibraryResponse => ({
  configured: true, storageMode: filesystemLibrary ? 'file' : 'postgres', rootDisplay: '', rootPath: '', projects: [], candidates: [],
  config: { version: 1, dirs: { ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' }, slugWords: 3,
    autoExport: { onVolume: false, onBook: false, onChapter: false, formats: ['txt'], pattern: '', onExists: 'replace' } },
})
beforeEach(() => {
  vi.mocked(api.getLibrary).mockResolvedValue(library(false))
  vi.mocked(api.getConfig).mockResolvedValue({ target: { config: {} }, effective: {}, chain: [], sources: {} } as unknown as ConfigResponse)
  vi.mocked(api.getConfigPresets).mockResolvedValue({ providers: [], styles: [], reasoning: [] } as ConfigPresets)
  vi.mocked(api.getSystem).mockResolvedValue({ canReveal: false } as SystemInfo)
  vi.mocked(api.getFsPlaces).mockResolvedValue([{ id: 'home', label: 'Thư mục nhà', path: '/tmp/home', icon: 'user' }])
  vi.mocked(api.inspectLibraryPath).mockResolvedValue({ kind: 'empty', projects: 0, stories: 0, items: 0, place: 'Thư mục nhà', display: 'Thư mục nhà › Truyện Noverse', name: 'Truyện Noverse' })
})
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren(); cache.clear(); vi.clearAllMocks()
})
async function mount(element: React.ReactNode) {
  const container = document.createElement('div'); document.body.append(container)
  const root = createRoot(container); roots.push(root)
  await act(async () => { root.render(element) })
  return container
}

it.each(['postgres', 'container'] as const)('%s onboarding continues without browsing, inspecting or writing a library folder/config', async (storageMode) => {
  const storageWarning = storageMode === 'container' ? 'Thư mục này sẽ mất khi cập nhật container — hãy gắn volume.' : ''
  vi.mocked(api.getLibrary).mockResolvedValue({ ...library(false), storageMode, storageWarning })
  const container = await mount(<OnboardScreen step="library" />)
  expect(container.textContent).toContain(storageMode === 'postgres' ? 'Truyện được lưu trong cơ sở dữ liệu của máy chủ' : 'Truyện được lưu trong vùng dữ liệu của Noverse')
  if (storageWarning) expect(container.textContent).toContain(storageWarning)
  expect(container.querySelector('[aria-label="Thư mục lưu truyện"]')).toBeNull()
  expect(api.getFsPlaces).not.toHaveBeenCalled()
  expect(api.inspectLibraryPath).not.toHaveBeenCalled()
  const next = [...container.querySelectorAll('button')].find((button) => button.textContent === 'Tiếp tục')!
  expect(next.disabled).toBe(false)
  await act(async () => next.click())
  expect(navigate).toHaveBeenCalledWith('/welcome/project', false)
  expect(api.putLibrary).not.toHaveBeenCalled()
  expect(api.putLibraryRoot).not.toHaveBeenCalled()
})

it('hosted settings explains server storage without change/reveal-folder actions', async () => {
  const container = await mount(<LibraryRootRows library={library(false)} canReveal onChanged={vi.fn()} />)
  expect(container.textContent).toContain('Truyện được lưu trong cơ sở dữ liệu của máy chủ')
  expect(container.querySelector('button')).toBeNull()
})

it.each(['file', undefined] as const)('file onboarding (%s) still browses folders and keeps its selection controls', async (storageMode) => {
  vi.mocked(api.getLibrary).mockResolvedValue({ ...library(true), storageMode, configured: false })
  const container = await mount(<OnboardScreen step="library" />)
  expect(container.querySelector('[aria-label="Thư mục lưu truyện"]')).not.toBeNull()
  expect(container.textContent).toContain('Chọn thư mục khác…')
  expect(api.getFsPlaces).toHaveBeenCalled()
  expect(api.inspectLibraryPath).toHaveBeenCalled()
})

it('hosted project health never asks the server to validate a filesystem tree', async () => {
  vi.mocked(api.validateProject).mockResolvedValue({ ok: true, issues: [], tree: { name: 'project', path: 'project', kind: 'dir', role: 'project', status: 'ok' } })
  function ProjectHealth() { useValidation('hosted-project'); return null }
  await mount(<LibraryProvider><ProjectHealth /></LibraryProvider>)
  expect(api.getLibrary).toHaveBeenCalled()
  expect(api.validateProject).not.toHaveBeenCalled()
})

it('file project health still validates its filesystem tree', async () => {
  vi.mocked(api.getLibrary).mockResolvedValue(library(true))
  vi.mocked(api.validateProject).mockResolvedValue({ ok: true, issues: [], tree: { name: 'project', path: 'project', kind: 'dir', role: 'project', status: 'ok' } })
  function ProjectHealth() { useValidation('file-project'); return null }
  await mount(<LibraryProvider><ProjectHealth /></LibraryProvider>)
  expect(api.validateProject).toHaveBeenCalledWith('file-project')
})
