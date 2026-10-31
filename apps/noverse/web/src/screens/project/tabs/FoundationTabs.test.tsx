// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PremiseTab, WorldTab } from './FoundationTabs'
import { premiseRows } from './foundation'
import type { FoundationResource } from './useFoundation'
Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => document.body.replaceChildren())
const refresh = async () => {}
function resource(sections: { heading: string; text: string }[], missing: string[], world: { category: string; rule: string; boundary: string }[]): FoundationResource {
  const rows = premiseRows(sections, missing)
  return { premise: { data: { sections, missing, updatedAt: null }, refresh }, world: { data: world, refresh }, rows, required: rows, ready: true } as FoundationResource
}
describe('foundation assistant requests', () => {
  it('replaces a removed premise section with an actionable missing row, retaining every other section', async () => {
    const host = document.createElement('div'); document.body.append(host)
    const root = createRoot(host)
    const steer = vi.fn()
    const sections = [{ heading: 'Vùng cấm sáng tác', text: 'Không đổi người kể.' }, { heading: 'Cam kết cốt lõi', text: 'Giữ lời hứa.' }]
    await act(() => root.render(<PremiseTab foundation={resource(sections, [], [])} onSteer={steer} />))
    expect(host.textContent).toContain('Không đổi người kể.')
    await act(() => root.render(<PremiseTab foundation={resource(sections.slice(1), ['Vùng cấm sáng tác'], [])} onSteer={steer} />))
    expect(host.textContent).not.toContain('Không đổi người kể.')
    expect(host.textContent).toContain('Giữ lời hứa.')
    const buttons = [...host.querySelectorAll('button')]
    await act(() => buttons.find((button) => button.textContent === 'Nhờ bổ sung')!.click())
    expect(steer).toHaveBeenCalledWith('Bổ sung mục “Vùng cấm sáng tác” cho tiền đề: ')
    await act(() => root.unmount())
  })
  it('replaces world groups with the empty state and requests a foundation based on written chapters', async () => {
    const host = document.createElement('div'); document.body.append(host)
    const root = createRoot(host); const steer = vi.fn()
    await act(() => root.render(<WorldTab foundation={resource([], [], [{ category: 'technology', rule: 'Không quay ngược thời gian.', boundary: 'Không ngoại lệ.' }])} project="p" completed={3} onSteer={steer} />))
    expect(host.querySelector('[role=group]')?.getAttribute('aria-label')).toBe('Công nghệ')
    await act(() => root.render(<WorldTab foundation={resource([], [], [])} project="p" completed={3} onSteer={steer} />))
    expect(host.querySelector('[role=group]')).toBeNull()
    expect(host.querySelectorAll('button')).toHaveLength(1)
    await act(() => host.querySelector('button')!.click())
    expect(steer).toHaveBeenCalledWith('Lập quy tắc thế giới từ 3 chương đã viết')
    await act(() => root.unmount())
  })
})
