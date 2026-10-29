// @vitest-environment happy-dom
// Gói AL — tab Cách viết của truyện: chọn ô = lưu ngay PUT config.style cấp truyện + toast,
// “Dùng theo dự án” xoá ghi đè. Không mount cả StoryView.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type * as Client from '../../../api/client'
import type * as UI from '@noverse/ui'
import type { Config, ConfigResponse, ConfigScope, Snapshot, StorySummary } from '../../../api/types'
import { cache } from '../../../data'
import { StyleTab } from './StyleTab'

const api = vi.hoisted(() => ({
  getConfig: vi.fn(), putConfig: vi.fn(), getConfigPresets: vi.fn(), getProject: vi.fn(),
  getSnapshot: vi.fn(), getStyleSamples: vi.fn(), canWriteStory: () => true,
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
const STORY = '01-tra-xanh-cuop-kich-ban'

const summary = (): StorySummary => ({
  id: `${PROJECT}/${STORY}`, project: PROJECT, story: STORY, folder: `kich_ban/${STORY}`,
  title: 'Trà Xanh Cướp Kịch Bản Của Tôi', status: 'writing', tags: [], idea: '',
  phase: 'writing', completedChapters: 4, totalChapters: 8, totalWords: 9000, volumes: 1,
  open: true, running: false, locked: false, updatedAt: null, exportsCount: 0, tempName: false,
})

const doc = (scope: ConfigScope, config: Config) => ({ scope, path: '', exists: true, config })

/** Truyện kế thừa dự án (sources.style=project, effective=romance) hoặc chọn riêng (sources.style=story). */
const configOf = (own: boolean): ConfigResponse => ({
  version: 'v1',
  target: doc('story', own ? { style: 'suspense' } : {}),
  effective: { style: own ? 'suspense' : 'romance' },
  chain: [doc('global', { style: 'default' }), doc('project', { style: 'romance' }), doc('story', own ? { style: 'suspense' } : {})],
  sources: { style: own ? 'story' : 'project' },
})

api.getConfigPresets.mockResolvedValue({
  providers: [], reasoning: [],
  styles: ['default', 'romance', 'suspense', 'fantasy', 'wuxia', 'giong-tre'],
  packs: [{ id: 'nu-tan-bao-thu', ten: 'Nữ tần trọng sinh báo thù, vả mặt', short: 'Nữ tần báo thù', so_chuong: 8, nguoi_nghe: 'Nữ 18–35', hasRules: true, layers: [], params: [] }],
})
api.getSnapshot.mockResolvedValue({
  advanceMode: 'auto', completedCount: 4, inProgressChapter: 4,
} as unknown as Snapshot)
api.getProject.mockResolvedValue({ project: { name: 'Kênh truyện audio' } })
api.getStyleSamples.mockResolvedValue({
  samples: [{ id: 'm1.txt', title: 'Mở màn chương 1 — bản tay', chars: 3120, addedAt: '2026-10-24T00:00:00Z' }],
  total: { count: 1, chars: 3120 },
})

async function mount(own: boolean) {
  api.getConfig.mockResolvedValue(configOf(own))
  api.putConfig.mockImplementation(async (scope: ConfigScope, config: Config) => ({
    version: 'v2',
    target: doc(scope, config),
    effective: { ...config, style: config.style ?? 'romance' },
    chain: [doc('global', { style: 'default' }), doc('project', { style: 'romance' }), doc('story', config)],
    sources: { style: config.style ? 'story' : 'project' },
  }))
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<StyleTab project={PROJECT} summary={summary()} />) })
  await act(async () => { await Promise.resolve() })
  return container
}

const card = (container: HTMLElement, label: string) =>
  [...container.querySelectorAll<HTMLButtonElement>('button[role="radio"]')]
    .find((node) => node.textContent?.includes(label))!

describe('Gói AL › tab Cách viết của truyện', () => {
  it('chọn một kiểu viết lưu ngay config.style cấp truyện + toast “áp dụng từ chương”', async () => {
    const container = await mount(false)
    expect(api.getConfig).toHaveBeenCalledWith('story', PROJECT, STORY, expect.anything())
    await act(async () => { card(container, 'Hồi hộp').click() })
    expect(api.putConfig).toHaveBeenCalledWith('story', { style: 'suspense' }, PROJECT, STORY, 'v1')
    expect(toasts.some((text) => text === 'Đã đổi văn phong của truyện · áp dụng từ chương 5')).toBe(true)
  })

  it('“Dùng theo dự án” xoá ghi đè cấp truyện (PUT không còn style) + toast', async () => {
    const container = await mount(true)
    const reset = [...container.querySelectorAll('button')].find((node) => node.textContent === 'Dùng theo dự án')!
    await act(async () => { reset.click() })
    expect(api.putConfig).toHaveBeenCalledWith('story', {}, PROJECT, STORY, 'v1')
    expect(toasts.some((text) => text === 'Truyện dùng lại văn phong của dự án · từ chương 5')).toBe(true)
  })

  it('đang kế thừa: thanh kế thừa “Theo dự án”, không có nút Dùng theo dự án; có văn mẫu → “Văn phong của bạn” chọn được', async () => {
    const container = await mount(false)
    expect([...container.querySelectorAll('button')].some((node) => node.textContent === 'Dùng theo dự án')).toBe(false)
    expect(container.textContent).toContain('Theo dự án “Kênh truyện audio” · chọn một kiểu bên dưới để dùng riêng')
    const custom = card(container, 'Văn phong của bạn')
    expect(custom.disabled).toBe(false)
    expect(container.textContent).toContain('Văn phong của bạn · văn mẫu của truyện này')
    expect(container.textContent).toContain('Mở màn chương 1 — bản tay')
  })
})
