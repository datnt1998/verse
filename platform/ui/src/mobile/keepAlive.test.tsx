// @vitest-environment happy-dom
// Hợp đồng giữ-alive: đổi tab chỉ ẩn pane cũ (không unmount), pane ẩn dừng effect,
// hook trong pane ẩn không thấy pane đang hoạt động. Ba điều này là lý do tồn tại
// của `KeepAlive`/`MobilePanes`; gỡ chúng thì tab lại dựng lại màn từ đầu.
import { act, useEffect, useState, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { KeepAlive, MobilePanes } from './keepAlive'
import { useMobilePaneActive } from './runtime'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
})

const render = async (ui: ReactNode) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(ui) })
  return { container, root }
}

const paneOf = (container: HTMLElement, id: string) => container.querySelector(`[data-pane="${id}"]`) as HTMLElement

describe('KeepAlive', () => {
  it('ẩn rồi hiện lại: state còn nguyên, effect dừng khi ẩn và chạy lại khi hiện', async () => {
    let runs = 0
    let stops = 0
    const Counting = () => {
      const [state] = useState(() => 'đã mở')
      useEffect(() => { runs++; return () => { stops++ } }, [])
      return <div data-marker="counting">{state}</div>
    }
    const node = <Counting />
    const { container, root } = await render(<KeepAlive active>{node}</KeepAlive>)
    expect(runs).toBe(1)
    await act(async () => { root.render(<KeepAlive active={false}>{node}</KeepAlive>) })
    expect(stops).toBe(1)
    expect(container.querySelector('[data-marker="counting"]')?.textContent).toBe('đã mở')
    await act(async () => { root.render(<KeepAlive active>{node}</KeepAlive>) })
    expect(runs).toBe(2)
    expect(container.querySelector('[data-marker="counting"]')?.textContent).toBe('đã mở')
  })
})

describe('MobilePanes', () => {
  it('đổi tab chỉ ẩn pane cũ: DOM, state và vị trí cuộn còn nguyên, không mount lại', async () => {
    let mounts = 0
    const Counting = () => {
      const [state] = useState(() => { mounts++; return true })
      return <div data-marker="counting" data-alive={state ? 'yes' : 'no'}>đã mở</div>
    }
    const counting = <Counting />
    const other = <div data-marker="other" />
    const { container, root } = await render(<MobilePanes panes={[['read', counting], ['work', other]]} active="read" />)
    expect(mounts).toBe(1)
    const pane = paneOf(container, 'read')
    const inner = container.querySelector('[data-marker="counting"]') as HTMLElement
    inner.dataset.kept = 'yes'
    pane.scrollTop = 240

    await act(async () => { root.render(<MobilePanes panes={[['read', counting], ['work', other]]} active="work" />) })
    expect(getComputedStyle(pane).display).toBe('none')
    expect(pane.getAttribute('aria-hidden')).toBe('true')
    expect(pane.hasAttribute('data-pane-active')).toBe(false)
    expect(paneOf(container, 'work').getAttribute('aria-hidden')).toBeNull()
    expect(paneOf(container, 'work').hasAttribute('data-pane-active')).toBe(true)
    expect(getComputedStyle(paneOf(container, 'work')).display).toBe('flex')

    await act(async () => { root.render(<MobilePanes panes={[['read', counting], ['work', other]]} active="read" />) })
    expect(mounts).toBe(1)
    expect(container.querySelector('[data-marker="counting"]')).toBe(inner)
    expect(inner.dataset.kept).toBe('yes')
    expect(pane.scrollTop).toBe(240)
    expect(getComputedStyle(pane).display).toBe('flex')
  })

  it('hook trong pane ẩn thấy pane không hoạt động, pane hiện thấy hoạt động', async () => {
    const Probe = () => <div data-marker="probe">{useMobilePaneActive() ? 'on' : 'off'}</div>
    const probe = <Probe />
    const other = <div data-marker="other" />
    const { container, root } = await render(<MobilePanes panes={[['read', probe], ['work', other]]} active="read" />)
    expect(container.querySelector('[data-marker="probe"]')?.textContent).toBe('on')
    await act(async () => { root.render(<MobilePanes panes={[['read', probe], ['work', other]]} active="work" />) })
    expect(container.querySelector('[data-marker="probe"]')?.textContent).toBe('off')
  })

  it('pane đang ẩn không nhận trạng thái runtime dù nội dung vẫn mounted', async () => {
    let pushes = 0
    const Pushing = () => {
      const active = useMobilePaneActive()
      const [value] = useState('giữ')
      useEffect(() => { if (active) pushes++ }, [active])
      return <div data-marker="pushing">{value}</div>
    }
    const pushing = <Pushing />
    const other = <div data-marker="other" />
    const { container, root } = await render(<MobilePanes panes={[['read', pushing], ['work', other]]} active="read" />)
    expect(pushes).toBe(1)
    const node = container.querySelector('[data-marker="pushing"]')
    // Ẩn pane: nội dung vẫn nằm đó (không mount lại) nhưng effect dọn và không đẩy
    // thêm trạng thái; hiện lại mới đẩy.
    await act(async () => { root.render(<MobilePanes panes={[['read', pushing], ['work', other]]} active="work" />) })
    expect(container.querySelector('[data-marker="pushing"]')).toBe(node)
    expect(pushes).toBe(1)
    await act(async () => { root.render(<MobilePanes panes={[['read', pushing], ['work', other]]} active="read" />) })
    expect(pushes).toBe(2)
  })
})
