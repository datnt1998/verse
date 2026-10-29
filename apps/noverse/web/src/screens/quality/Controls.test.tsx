// @vitest-environment happy-dom
// Câu 14 — Quality: chuyển màn Việc cần làm/Ma trận là `tablist` (tab + aria-selected + aria-controls),
// Mức và Phạm vi là bộ lọc dữ liệu nên `radiogroup` (radio + aria-checked). Không dùng aria-pressed.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { QualityControls } from './Controls'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const mounted: { root: Root; container: HTMLElement }[] = []
afterEach(async () => {
  for (const { root, container } of mounted.splice(0)) {
    await act(() => root.unmount())
    container.remove()
  }
})

async function mount(props: Partial<Parameters<typeof QualityControls>[0]> = {}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  mounted.push({ root, container })
  const all = { view: 'todo' as const, level: 'all' as const, scope: 'all' as const, todo: 6, onView: () => {}, onLevel: () => {}, onScope: () => {}, ...props }
  await act(async () => { root.render(<QualityControls {...all} />) })
  return container
}

describe('Quality › vai trò ARIA của ba nhóm chọn', () => {
  it('Việc cần làm / Ma trận là tablist–tab, trỏ tới tabpanel quality-view-panel', async () => {
    const container = await mount()
    const list = container.querySelector<HTMLElement>('[role=tablist]')!
    expect(list.getAttribute('aria-label')).toBe('Cách xem')
    const tabs = [...list.querySelectorAll<HTMLElement>('[role=tab]')]
    expect(tabs.map((tab) => tab.id)).toEqual(['quality-view-todo', 'quality-view-matrix'])
    expect(tabs.map((tab) => tab.getAttribute('aria-selected'))).toEqual(['true', 'false'])
    expect(tabs.every((tab) => tab.getAttribute('aria-controls') === 'quality-view-panel')).toBe(true)
    expect(tabs.some((tab) => tab.hasAttribute('aria-pressed'))).toBe(false)
  })

  it('Mức và Phạm vi là radiogroup, mục đang chọn mang aria-checked', async () => {
    const container = await mount({ view: 'matrix' })
    const groups = [...container.querySelectorAll<HTMLElement>('[role=radiogroup]')]
    expect(groups.map((group) => group.getAttribute('aria-label'))).toEqual(['Mức', 'Phạm vi'])
    for (const group of groups) {
      const radios = [...group.querySelectorAll<HTMLElement>('[role=radio]')]
      expect(radios.length).toBeGreaterThan(1)
      expect(radios.filter((radio) => radio.getAttribute('aria-checked') === 'true')).toHaveLength(1)
    }
  })

  it('Phạm vi chỉ có ở Ma trận; bấm Ma trận gọi onView', async () => {
    expect((await mount()).querySelector('[aria-label="Phạm vi"]')).toBeNull()
    const onView = vi.fn()
    const container = await mount({ onView })
    const matrix = container.querySelector<HTMLElement>('#quality-view-matrix')!
    await act(async () => { matrix.click() })
    expect(onView).toHaveBeenCalledWith('matrix')
  })
})
