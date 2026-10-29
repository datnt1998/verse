// @vitest-environment happy-dom
// Thông tin dự án (Settings.dc.html #info, scope=project): thứ tự ô và nhãn/placeholder của “Tên vũ trụ”.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { ProjectDetail } from '../../../api/types'
import type * as Client from '../../../api/client'
import { cache } from '../../../data'
import type { SettingsContext } from '../types'
import { InfoSection } from './info'

const PROJECT: ProjectDetail = {
  version: 'v1',
  project: {
    id: 'kenh', name: 'Kênh truyện audio', description: 'Truyện ngắn đô thị', notes: 'Giọng miền Bắc',
    universeName: '', legacy: false, storyCount: 1, runningCount: 0, updatedAt: null, createdAt: null, path: '',
  },
  stories: [], ideas: [], assets: [],
}

vi.mock('../../../api/client', async (original) => ({
  ...await original<typeof Client>(),
  getProject: async () => PROJECT,
  updateProject: async () => PROJECT.project,
}))

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  cache.clear()
})

async function mount() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  const ctx = { project: 'kenh', story: '', canEdit: true, registerExtra: () => {} } as unknown as SettingsContext
  await act(async () => { root.render(<InfoSection ctx={ctx} scope="project" />) })
  await act(async () => { await Promise.resolve() })
  return container
}

describe('Thông tin dự án › thứ tự ô', () => {
  it('Tên dự án · Mô tả · Tên vũ trụ · Ghi chú cho trợ lý — tên vũ trụ nằm dưới Mô tả, trên Ghi chú', async () => {
    const container = await mount()
    const labels = [...container.querySelectorAll('.st-sec label > span:first-child')].map((node) => node.textContent)
    expect(labels).toEqual(['Tên dự án', 'Mô tả', 'Tên vũ trụ · hiện ở Não dự án', 'Ghi chú cho trợ lý'])
  })

  it('ô tên vũ trụ: placeholder cố định “Để trống thì dùng tên dự án”, không lấy tên dự án làm gợi ý', async () => {
    const container = await mount()
    const input = [...container.querySelectorAll<HTMLInputElement>('input.st-field')]
      .find((node) => node.placeholder !== '')!
    expect(input.value).toBe('')
    expect(input.placeholder).toBe('Để trống thì dùng tên dự án')
  })
})
