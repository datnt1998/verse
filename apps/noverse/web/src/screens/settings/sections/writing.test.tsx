// @vitest-environment happy-dom
// Gói AL — Tuỳ chỉnh › Cách viết: cấp truyện/dự án chỉ còn thẻ chỉ dẫn (mở Dự án & truyện),
// cấp “Mọi truyện” giữ “Văn phong chung” với ô “Văn phong của bạn” mờ; hai công tắc giữ nguyên.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { SettingsContext } from '../types'
import type { Config, ConfigDoc, ConfigScope, Snapshot } from '../../../api/types'
import { cache } from '../../../data'
import { WritingSection } from './writing'

const api = vi.hoisted(() => ({ setReview: vi.fn(async () => ({})) }))
vi.mock('../../../api/client', async (original) => ({ ...(await original<typeof import('../../../api/client')>()), ...api }))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  cache.clear()
})
beforeEach(() => { vi.clearAllMocks() })

const PROJECT = 'dit-dang-audio'
const STORY = '01-tra-xanh-cuop-kich-ban'

const doc = (scope: ConfigScope, config: Config): ConfigDoc => ({ scope, path: '', exists: true, config })

function ctxOf(scope: ConfigScope, sources: Record<string, ConfigScope>): SettingsContext {
  return {
    scope, project: PROJECT, story: STORY,
    labels: scope === 'story' ? { noun: 'truyện' } : scope === 'project' ? { noun: 'dự án' } : {},
    presets: {
      providers: [], reasoning: [], styles: ['default', 'romance', 'suspense', 'fantasy', 'wuxia', 'giong-tre'],
      packs: [{ id: 'nu-tan-bao-thu', ten: 'Nữ tần trọng sinh báo thù, vả mặt', short: 'Nữ tần báo thù', so_chuong: 8, nguoi_nghe: 'Nữ 18–35', hasRules: true, layers: [], params: [] }],
    },
    response: {
      target: doc(scope, scope === 'global' ? { style: 'default' } : {}),
      effective: { style: scope === 'story' ? 'nu-tan-bao-thu' : 'romance' },
      chain: [doc('global', { style: 'default' }), doc('project', { style: 'romance' }), doc('story', scope === 'story' ? { style: 'nu-tan-bao-thu' } : {})],
      sources: { style: sources.style },
    },
    draft: {}, dirty: {}, errors: {},
    effective: { style: scope === 'story' ? 'nu-tan-bao-thu' : 'romance' },
    canEdit: true, narrow: false, saveBusy: false, lastError: null, readOnly: false,
    snapshot: scope === 'story' ? ({ advanceMode: 'auto' } as unknown as Snapshot) : null,
    canEditScope: () => true,
    ovr: () => ({ on: false, scope: 'global' as ConfigScope }),
    setField: () => {}, resetField: () => {},
    runSave: async () => undefined,
  } as unknown as SettingsContext
}

async function mount(scope: ConfigScope, source: ConfigScope) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<WritingSection ctx={ctxOf(scope, { style: source })} />) })
  await act(async () => { await Promise.resolve() })
  return container
}

describe('Gói AL › Tuỳ chỉnh › Cách viết theo cấp', () => {
  it('cấp truyện: thẻ chỉ dẫn mở tab Cách viết của truyện, không còn lưới văn phong', async () => {
    const container = await mount('story', 'story')
    expect(container.textContent).toContain('Văn phong của truyện này')
    expect(container.textContent).toContain('Gói Nữ tần báo thù · riêng truyện này')
    const link = [...container.querySelectorAll('a')].find((a) => a.textContent?.includes('Mở Cách viết của truyện'))!
    expect(link.getAttribute('href')).toBe(`#/p/${PROJECT}/s/${STORY}?tab=style`)
    expect(container.querySelectorAll('button[role="radio"]').length).toBe(0)
    expect(container.textContent).toContain('Duyệt từng chương')
    expect(container.textContent).toContain('Trọng tài hỏi bạn khi phân vân')
  })

  it('cấp truyện theo dự án: thẻ chỉ dẫn ghi nguồn “theo dự án”', async () => {
    const container = await mount('story', 'project')
    expect(container.textContent).toContain('theo dự án')
    expect([...container.querySelectorAll('a')].some((a) => a.getAttribute('href')?.endsWith('?tab=style'))).toBe(true)
  })

  it('cấp dự án: thẻ chỉ dẫn mở mục Cách viết của dự án', async () => {
    const container = await mount('project', 'project')
    expect(container.textContent).toContain('Văn phong mặc định của dự án')
    const link = [...container.querySelectorAll('a')].find((a) => a.textContent?.includes('Mở Cách viết của dự án'))!
    expect(link.getAttribute('href')).toBe(`#/p/${PROJECT}?section=pstyle`)
    expect(container.querySelectorAll('button[role="radio"]').length).toBe(0)
  })

  it('Mọi truyện: giữ “Văn phong chung”, ô “Văn phong của bạn” mờ, không thẻ chỉ dẫn', async () => {
    const container = await mount('global', 'global')
    expect(container.textContent).toContain('Văn phong chung')
    expect(container.textContent).not.toContain('Mở Cách viết')
    const custom = [...container.querySelectorAll<HTMLButtonElement>('button[role="radio"]')]
      .find((node) => node.textContent?.includes('Văn phong của bạn'))!
    expect(custom.disabled).toBe(true)
    expect(custom.textContent).toContain('Văn mẫu thêm theo từng truyện ở Dự án & truyện')
    const builtIn = [...container.querySelectorAll<HTMLButtonElement>('button[role="radio"]')]
      .find((node) => node.textContent?.includes('Hồi hộp'))!
    expect(builtIn.disabled).toBe(false)
  })
})
