// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it, vi } from 'vitest'
import type { AddressTerms, Snapshot } from '../../api/types'
import { mockBrainDetailOf, SAMPLE_STORY } from '../../../mock/brain'
import { StoryTab } from './StoryTab'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const brain = mockBrainDetailOf(SAMPLE_STORY)!
const address: AddressTerms = {
  terms: [
    { id: 'incoming', from: 'th', fromName: 'Lâm Tri Hạ', to: 'nv', toName: 'Tống Nhược Vy', call: 'cô', self: 'tôi', fromChapter: 2, untilChapter: null, reason: '' },
    { id: 'outgoing', from: 'nv', fromName: 'Tống Nhược Vy', to: 'th', toName: 'Lâm Tri Hạ', call: 'em', self: 'chị', fromChapter: 1, untilChapter: null, reason: '' },
  ], relations: [], latestWritten: 3, writing: null,
}
vi.mock('../project/cast/data', () => ({
  useBrain: () => ({ data: brain }),
  useAddressTerms: () => ({ data: address }),
  useCharacters: () => ({ data: { characters: [{ name: 'Tống Nhược Vy', kind: 'antagonist', role: 'Phản diện' }] } }),
}))
let root: Root | null = null
let container: HTMLElement | null = null
afterEach(() => { act(() => root?.unmount()); container?.remove() })
it('danh sách lấy nhân vật Não truyện; thẻ rê Nhược Vy dùng đúng chiều gọi Tri Hạ', async () => {
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
  await act(async () => { root!.render(<StoryTab project="p" story="s" snapshot={{ inProgressChapter: 4, supportingCount: 0 } as Snapshot} />) })
  const row = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('Tống Nhược Vy'))!
  await act(async () => { row.dispatchEvent(new FocusEvent('focusin', { bubbles: true })) })
  expect(container.querySelector('[role="tooltip"]')?.textContent).toContain('gọi Lâm Tri Hạ “em”, tự xưng “chị”')
  expect(container.textContent).toContain('Phùng Chấn')
})
