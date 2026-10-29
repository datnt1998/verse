// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { ConfigScope } from '../../../api/types'
import type { SettingsContext } from '../types'

const api = vi.hoisted(() => ({
  getConfig: vi.fn(),
  getRevealPath: vi.fn(),
  getRules: vi.fn(),
  revealFolder: vi.fn(),
}))
vi.mock('../../../api/client', async (importOriginal) => ({ ...(await importOriginal<object>()), ...api }))
vi.mock('../sections/LibraryRootRow', () => ({ LibraryRootRows: () => null }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), toast: { ok: vi.fn(), err: vi.fn() }, Icon: () => null }))
import { LocationRows } from './LocationRows'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); vi.clearAllMocks() })

const PATHS: Record<ConfigScope, { config: string; rules: string }> = {
  global: { config: '~/.ainovel/config.json', rules: '/home/dat/.ainovel/rules' },
  project: { config: 'proj/.ainovel/config.json', rules: '/lib/proj/.ainovel/rules' },
  story: { config: 'proj/kich_ban/s1/.ainovel/config.json', rules: '/lib/proj/kich_ban/s1/.ainovel/rules' },
}

beforeEach(() => {
  api.getConfig.mockImplementation(async (scope: ConfigScope) => ({ target: { scope, path: PATHS[scope].config, exists: true, config: {} } }))
  api.getRevealPath.mockImplementation(async (input: { target: string; scope: ConfigScope }) => ({ path: input.target === 'config' ? `/abs/${PATHS[input.scope].config}` : PATHS[input.scope].rules, canReveal: false }))
  api.getRules.mockResolvedValue({ chain: [
    { scope: 'global', dir: '', exists: true, files: [{ name: 'a.md', size: 1, updatedAt: null }] },
    { scope: 'project', dir: '', exists: true, files: [{ name: 'a.md', size: 1, updatedAt: null }, { name: 'b.md', size: 1, updatedAt: null }] },
    { scope: 'story', dir: '', exists: true, files: [{ name: 'c.md', size: 1, updatedAt: null }, { name: 'd.md', size: 1, updatedAt: null }, { name: 'e.md', size: 1, updatedAt: null }] },
  ] })
  api.revealFolder.mockResolvedValue({ ok: true, path: '' })
})

async function mount(viewScope: ConfigScope, canReveal: boolean) {
  const ctx = { scope: 'global', viewScope, project: viewScope === 'global' ? '' : 'proj', story: viewScope === 'story' ? 's1' : '', canReveal, library: null, reloadLibrary: () => undefined } as unknown as SettingsContext
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<LocationRows ctx={ctx} />) })
  await act(async () => { await Promise.resolve() })
  return container
}
const row = (container: HTMLElement, label: string) => [...container.querySelectorAll('div')].find((node) => node.textContent?.startsWith(label) && node.querySelector('button')) as HTMLElement
const button = (container: HTMLElement, label: string, text: string) => [...row(container, label).querySelectorAll('button')].find((node) => node.textContent === text) as HTMLButtonElement

describe('Nâng cao › Vị trí lưu trên máy chủ', () => {
  it('trên máy chủ: “Mở” chọn đúng tệp cấu hình của cấp đang chọn (target=config)', async () => {
    const container = await mount('project', true)
    expect(row(container, 'Tệp cấu hình').textContent).toContain('proj/.ainovel/config.json')
    await act(async () => { button(container, 'Tệp cấu hình', 'Mở').click() })
    expect(api.revealFolder).toHaveBeenCalledWith({ target: 'config', scope: 'project', project: 'proj', story: undefined })
  })

  it('ngoài máy chủ: “Chép đường dẫn” chép đường dẫn tuyệt đối, không mở gì', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const container = await mount('story', false)
    expect(button(container, 'Tệp cấu hình', 'Mở')).toBeUndefined()
    await act(async () => { button(container, 'Tệp cấu hình', 'Chép đường dẫn').click() })
    expect(writeText).toHaveBeenCalledWith('/abs/proj/kich_ban/s1/.ainovel/config.json')
    expect(api.revealFolder).not.toHaveBeenCalled()
  })

  it('“Thư mục quy tắc” ghi số tệp của đúng cấp đang chọn; cấp toàn cục chỉ ghi đường dẫn', async () => {
    expect(row(await mount('project', true), 'Thư mục quy tắc').textContent).toContain('…/.ainovel/rules/ · 2 tệp')
    document.body.replaceChildren()
    expect(row(await mount('story', true), 'Thư mục quy tắc').textContent).toContain('…/.ainovel/rules/ · 3 tệp')
    document.body.replaceChildren()
    const global = row(await mount('global', true), 'Thư mục quy tắc').textContent ?? ''
    expect(global).toContain('~/.ainovel/rules/')
    expect(global).not.toContain('tệp')
  })
})
