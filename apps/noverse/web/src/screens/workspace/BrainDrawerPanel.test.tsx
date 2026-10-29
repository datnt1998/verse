// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import type { AddressTerms } from '../../api/types'
import { mockBrainDetailOf, SAMPLE_STORY } from '../../../mock/brain'
import { BrainDrawer } from './BrainDrawerPanel'
import { BrainScreen } from '../brain'
import { StoryGraph } from '../brain/StoryGraph'
import { closeBrainDrawer, openBrainDrawer } from './brainDrawer'
import type * as GraphModule from '../brain/StoryGraph'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const brain = mockBrainDetailOf(SAMPLE_STORY)!
const address: AddressTerms = {
  terms: [], relations: [], latestWritten: 3,
  writing: { chapter: 4, role: 'writer', state: 'rewriting' },
}
vi.mock('../project/cast/data', () => ({
  useBrain: () => ({ data: brain, error: null, refresh: async () => {} }),
  useAddressTerms: () => ({ data: address, error: null, refresh: async () => {} }),
}))
vi.mock('../../shell/useNames', () => ({ useStoryName: () => 'Truyện mẫu' }))
vi.mock('../brain/useChapterQuery', () => ({ useChapterQuery: () => [4, vi.fn()] }))
vi.mock('../brain/StoryGraph', async (importOriginal) => {
  const original = await importOriginal<typeof GraphModule>()
  return { StoryGraph: vi.fn(original.StoryGraph) }
})

beforeAll(() => {
  vi.stubGlobal('Worker', class { postMessage = vi.fn(); terminate = vi.fn() })
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    measureText: (text: string) => ({ width: text.length * 6 }),
  } as CanvasRenderingContext2D)
  window.matchMedia = vi.fn().mockImplementation((media: string) => ({
    matches: media.includes('reduced-motion'), media,
    addEventListener() {}, removeEventListener() {},
  }))
})
let root: Root | null = null
let container: HTMLElement | null = null
afterEach(() => {
  act(() => root?.unmount())
  closeBrainDrawer()
  container?.remove()
  root = null
  container = null
  vi.mocked(StoryGraph).mockClear()
})
const mount = async (drawer: boolean) => {
  openBrainDrawer('pc', 4)
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(drawer ? <BrainDrawer project="p" story="s" /> : <BrainScreen project="p" story="s" />) })
  return document.querySelector<HTMLElement>('.brain-wrap')!
}

describe('Gói U — một đồ thị và một bộ lọc chương', () => {
  it('màn Não truyện và ngăn kéo gọi cùng StoryGraph; compact chỉ bỏ điều khiển', async () => {
    await mount(false)
    expect(StoryGraph).toHaveBeenCalledWith(expect.objectContaining({ brain, chapter: 4 }), undefined)
    await act(async () => { root!.unmount() })
    root = null
    container!.remove()
    vi.mocked(StoryGraph).mockClear()
    const graph = await mount(true)
    expect(StoryGraph).toHaveBeenCalledWith(expect.objectContaining({ brain, chapter: 4, compact: true }), undefined)
    expect(graph.querySelector('[role="toolbar"]')).toBeNull()
    expect(document.querySelector('.brain-tl')).not.toBeNull()
    expect(document.querySelector('[role="radiogroup"]')).toBeNull()
  })

  it.each([false, true])('lọc cùng quan hệ chương 4 và 5 (ngăn kéo=%s)', async (drawer) => {
    const graph = await mount(drawer)
    expect(graph).not.toBeNull()
    const selected = graph.querySelector('[data-id="pc"]')!
    await act(async () => { selected.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    const visibleLabels = () => [...graph.querySelectorAll('.bl')]
      .filter((node) => node.getAttribute('opacity') === '1').map((node) => node.querySelector('title')?.textContent)
    expect(visibleLabels()).toContain('Ép xoá bản live')
    expect(visibleLabels()).not.toContain('Chống lưng')
    expect(graph.querySelector('[data-id="pc"] circle[stroke-dasharray="3 2.4"]')).not.toBeNull()
    if (drawer) {
      const next = document.querySelector<HTMLButtonElement>('button[aria-label="Chương sau"]')!
      expect(next).not.toBeNull()
      await act(async () => { next.click() })
      expect(visibleLabels()).toContain('Chống lưng')
      expect(graph.querySelector('[data-id="pc"] circle[stroke-dasharray]')).toBeNull()
    }
  })

  it('Enter/Space chọn được cả vật; chi tiết có mô tả và diễn biến chỉ xem', async () => {
    const graph = await mount(true)
    await act(async () => {
      graph.querySelector('ul button[data-node-id="kb"]')!.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))
    })
    expect(graph.querySelector('ul button[data-node-id="kb"]')?.getAttribute('aria-pressed')).toBe('true')
    const detail = document.querySelector('aside[aria-label="Chi tiết"]')!
    expect(detail.textContent).toContain('Bản Tri Hạ viết')
    expect(detail.textContent).toContain('Diễn biến')
    expect(detail.querySelector('button')).toBeNull()
  })
})
