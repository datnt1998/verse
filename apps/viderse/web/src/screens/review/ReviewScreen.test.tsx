import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ReviewScreen } from './ReviewScreen'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const state = vi.hoisted(() => ({ mutate: vi.fn().mockResolvedValue(true), loadImages: vi.fn(), status: 'running' }))
vi.mock('./useReview', () => ({ imageURL: () => '', useReview: () => ({ jobs: [{ id: 'j', channelId: 'c', source: { title: 'Video' }, subline: '' }], images: { j: { manifest: { revision: 1 }, items: [{ id: 'a', kind: 'scene', status: state.status, prompt: 'p', versions: [], sceneId: '1' }, { id: 'b', kind: 'scene', status: 'review', prompt: 'q', versions: [], sceneId: '2' }] } }, imageErrors: {}, pending: {}, loadImages: state.loadImages, loadJobs: vi.fn(), mutate: state.mutate }) }))
vi.mock('../../data/status', () => ({ useOnline: () => true }))
vi.mock('../../data/useResource', () => ({ useResource: () => ({ data: { channels: [{ id: 'c', name: 'Kênh' }] } }) }))
vi.mock('../../components', () => ({ Orb: () => null }))
let root: Root | undefined
afterEach(() => { if (root) act(() => root!.unmount()); root = undefined; document.body.innerHTML = ''; vi.clearAllMocks() })
describe('duyệt ảnh đang vẽ', () => {
  it('khoá duyệt với lý do nhưng vẫn chuyển và vẽ lại ảnh khác', async () => {
    const node = document.createElement('div'); document.body.append(node); root = createRoot(node)
    await act(async () => { root!.render(<ReviewScreen />) })
    const approve = [...node.querySelectorAll('button')].find(b => b.textContent?.includes('Duyệt cả video'))!
    expect(approve.getAttribute('aria-disabled')).toBe('true')
    expect(approve.title).toContain('Chờ Hoạ sĩ')
    act(() => approve.click()); expect(state.mutate).not.toHaveBeenCalled()
    act(() => node.querySelector<HTMLButtonElement>('[aria-label="Cảnh 2"]')!.click())
    const redraw = [...node.querySelectorAll('button')].find(b => b.textContent?.startsWith('Vẽ lại　'))!
    await act(async () => { redraw.click() })
    expect(state.mutate).toHaveBeenCalledWith('j', expect.objectContaining({ id: 'b' }), 'regenerate', '')
  })
})
