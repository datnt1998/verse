import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReviewScreen } from './ReviewScreen'
import type * as ReviewModule from './useReview'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({ mutate: vi.fn().mockResolvedValue(true), loadImages: vi.fn(), status: 'running' }))
vi.mock('./useReview', async importOriginal => {
  const actual = await importOriginal<typeof ReviewModule>()
  return {
    ...actual,
    imageURL: (_job: string, image: { id: string }) => `/fixture/${image.id}`,
    useReview: () => ({ jobs: [{ id: 'j', channelId: 'c', source: { title: 'Video' }, subline: '' }, { id: 'k', channelId: 'c', source: { title: 'Video 2' }, subline: '' }], images: { j: { manifest: { revision: 1 }, items: [{ id: 'a', kind: 'scene', status: state.status, prompt: 'p', versions: [], sceneId: '1' }, { id: 'b', kind: 'scene', status: 'review', prompt: 'q', versions: [], sceneId: '2' }] }, k: { manifest: { revision: 1 }, items: [{ id: 'c', kind: 'scene', status: 'review', prompt: 'p', versions: [], sceneId: '1' }] } }, imageErrors: {}, pending: {}, loadImages: state.loadImages, loadJobs: vi.fn(), mutate: state.mutate }),
  }
})
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../data/useResource', () => ({ useResource: () => ({ data: { channels: [{ id: 'c', name: 'Kênh' }] } }) }))
vi.mock('../../components', () => ({ Orb: () => null }))
vi.mock('../../lib/useMediaQuery', () => ({ useMediaQuery: () => false }))
let root: Root | undefined
afterEach(() => { if (root) act(() => root!.unmount()); root = undefined; document.body.innerHTML = ''; vi.clearAllMocks() })
describe('duyệt ảnh đang vẽ', () => {
  it('khoá duyệt với lý do nhưng vẫn chuyển và vẽ lại ảnh khác', async () => {
    const node = document.createElement('div'); document.body.append(node); root = createRoot(node)
    await act(async () => { root!.render(<ReviewScreen />) })
    const approve = [...node.querySelectorAll('button')].find(b => b.textContent?.includes('Duyệt cả video'))!
    expect(approve.getAttribute('aria-disabled')).toBe('true')
    const describedBy = approve.getAttribute('aria-describedby')
    expect(describedBy && document.getElementById(describedBy)?.textContent).toContain('Chờ Hoạ sĩ')
    act(() => approve.click()); expect(state.mutate).not.toHaveBeenCalled()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Cảnh 2"]')!.click())
    const redraw = [...node.querySelectorAll('button')].find(b => b.textContent?.startsWith('Vẽ lại') && !b.textContent.includes('cả'))!
    await act(async () => { redraw.click() })
    expect(state.mutate).toHaveBeenCalledWith('j', expect.objectContaining({ id: 'b' }), 'regenerate', '')
  })
  it('duyệt xong chuyển đến video tiếp theo', async () => {
    state.status = 'review'
    const node = document.createElement('div'); document.body.append(node); root = createRoot(node)
    await act(async () => { root!.render(<ReviewScreen />) })
    const approve = [...node.querySelectorAll('button')].find(b => b.textContent?.includes('Duyệt cả video'))!
    await act(async () => { approve.click() })
    expect(node.querySelector('h1')?.textContent).toBe('Video 2')
    state.status = 'running'
  })
  it('ảnh bị chặn hiển thị ảnh cảnh bên cạnh', async () => {
    state.status = 'failed'
    const node = document.createElement('div'); document.body.append(node); root = createRoot(node)
    await act(async () => { root!.render(<ReviewScreen />) })
    expect(node.querySelector('main img')?.getAttribute('src')).toBe('/fixture/b')
    state.status = 'running'
  })
})
