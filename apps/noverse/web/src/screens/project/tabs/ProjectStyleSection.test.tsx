// @vitest-environment happy-dom
// Gói AL — mục “Cách viết mặc định của dự án”: Đổi mở lưới, chọn ô ghi cấp dự án,
// danh sách truyện phân biệt “Theo dự án” / truyện chọn riêng. Không mount cả ProjectOverview.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type * as Client from '../../../api/client'
import type * as UI from '@noverse/ui'
import type { Config, ConfigResponse, ConfigScope, ProjectDetail, StorySummary } from '../../../api/types'
import { cache } from '../../../data'
import { ProjectStyleSection } from './ProjectStyleSection'

const api = vi.hoisted(() => ({
  getConfig: vi.fn(), putConfig: vi.fn(), getConfigPresets: vi.fn(), canManageProject: () => true,
}))
vi.mock('../../../api/client', async (original) => ({ ...(await original<typeof Client>()), ...api }))
vi.mock('../../../api/session', () => ({
  useSession: () => ({ user: { username: 'dat', name: 'Dat', role: 'admin', projects: {} }, setUser: () => {}, checked: true }),
}))
const toasts = vi.hoisted(() => [] as string[])
vi.mock('@noverse/ui', async (original) => {
  const mod = await original<typeof UI>()
  const push = (text: string) => { toasts.push(text); return 1 }
  return { ...mod, toast: Object.assign(push, { ok: push, warn: push, err: push, info: push }) }
})

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  cache.clear()
})
beforeEach(() => { toasts.length = 0; vi.clearAllMocks() })

const PROJECT = 'dit-dang-audio'
const story = (id: string, title: string): StorySummary => ({
  id: `${PROJECT}/${id}`, project: PROJECT, story: id, folder: `kich_ban/${id}`, title,
  status: 'writing', tags: [], idea: '', phase: 'writing', completedChapters: 4, totalChapters: 8,
  totalWords: 9000, volumes: 1, open: true, running: false, locked: false, updatedAt: null,
  exportsCount: 0, tempName: false,
})
const OWN = story('01-tra-xanh', 'Trà Xanh Cướp Kịch Bản')
const INHERIT = story('02-ban-than', 'Bạn Thân Giật Chồng')
const detail = (): ProjectDetail => ({
  version: 'v1',
  project: {
    id: PROJECT, name: 'Kênh truyện audio', description: '', notes: '', universeName: '',
    legacy: false, storyCount: 2, runningCount: 0, updatedAt: null, createdAt: null, path: '',
  },
  stories: [OWN, INHERIT], ideas: [], assets: [],
})

const doc = (scope: ConfigScope, config: Config) => ({ scope, path: '', exists: true, config })
const projectConfig = (): ConfigResponse => ({
  version: 'v1', target: doc('project', { style: 'romance' }),
  effective: { style: 'romance' },
  chain: [doc('global', { style: 'default' }), doc('project', { style: 'romance' })],
  sources: { style: 'project' },
})
const storyConfig = (own: boolean): ConfigResponse => ({
  target: doc('story', own ? { style: 'nu-tan-bao-thu' } : {}),
  effective: { style: own ? 'nu-tan-bao-thu' : 'romance' },
  chain: [doc('global', { style: 'default' }), doc('project', { style: 'romance' }), doc('story', own ? { style: 'nu-tan-bao-thu' } : {})],
  sources: { style: own ? 'story' : 'project' },
})

api.getConfigPresets.mockResolvedValue({
  providers: [], reasoning: [], styles: ['default', 'romance', 'suspense', 'fantasy', 'wuxia', 'giong-tre'],
  packs: [{ id: 'nu-tan-bao-thu', ten: 'Nữ tần trọng sinh báo thù, vả mặt', short: 'Nữ tần báo thù', so_chuong: 8, nguoi_nghe: 'Nữ 18–35', hasRules: true, layers: [], params: [] }],
})

async function mount() {
  api.getConfig.mockImplementation(async (scope: ConfigScope) =>
    scope === 'project' ? projectConfig() : storyConfig(true))
  api.getConfig.mockImplementation(async (scope: ConfigScope, _p?: string, s?: string) =>
    scope === 'project' ? projectConfig() : storyConfig(s === OWN.story))
  api.putConfig.mockImplementation(async (scope: ConfigScope, config: Config) => ({
    version: 'v2', target: doc(scope, config), effective: { style: config.style ?? 'default' },
    chain: [doc('global', { style: 'default' }), doc(scope, config)], sources: { style: scope },
  }))
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<ProjectStyleSection project={PROJECT} detail={detail()} />) })
  await act(async () => { await Promise.resolve() })
  return container
}

describe('Gói AL › Cách viết mặc định của dự án', () => {
  it('chỉ liệt kê truyện chọn văn phong khác', async () => {
    const container = await mount()
    const rows = [...container.querySelectorAll<HTMLElement>('.st-smp')]
    const own = rows.find((row) => row.textContent?.includes(OWN.title))!
    expect(rows.some((row) => row.textContent?.includes(INHERIT.title))).toBe(false)
    expect(own.getAttribute('href')).toBe(`#/p/${PROJECT}/s/${OWN.story}?tab=style`)
  })

  it('Đổi mở lưới và chọn ô ghi PUT config cấp dự án + toast', async () => {
    const container = await mount()
    const toggle = [...container.querySelectorAll('button')].find((node) => node.textContent === 'Đổi')!
    await act(async () => { toggle.click() })
    const card = [...container.querySelectorAll<HTMLButtonElement>('button[role="radio"]')]
      .find((node) => node.textContent?.includes('Hồi hộp'))!
    await act(async () => { card.click() })
    expect(api.putConfig).toHaveBeenCalledWith('project', { style: 'suspense' }, PROJECT, undefined, 'v1')
    expect(toasts.some((text) => text === 'Đã đổi văn phong mặc định của dự án · truyện mới sẽ dùng')).toBe(true)
  })
})
