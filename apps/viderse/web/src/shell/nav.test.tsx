// Đích mục nav "Kênh": kênh xem gần nhất còn tồn tại, không thì kênh đầu, chỉ
// khi chưa có kênh nào mới mở màn Tạo kênh. Test riêng hàm thuần + nav render.
// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { MainNav } from './MainNav'
import { channelsHref, lastViewedChannelId, rememberChannel } from '../lib/lastChannel'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []

afterEach(async () => {
  localStorage.clear()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
})

describe('đích mục Kênh', () => {
  it('chưa có kênh → màn Tạo kênh', () => {
    expect(channelsHref([])).toBe('/channels/new')
  })

  it('có kênh nhưng chưa xem kênh nào → kênh đầu', () => {
    expect(channelsHref([{ id: 'c1' }, { id: 'c2' }])).toBe('/channels/c1')
  })

  it('mở kênh xem gần nhất khi kênh đó còn tồn tại', () => {
    rememberChannel('c2')
    expect(lastViewedChannelId()).toBe('c2')
    expect(channelsHref([{ id: 'c1' }, { id: 'c2' }])).toBe('/channels/c2')
  })

  it('kênh đã xoá thì rơi về kênh đầu', () => {
    rememberChannel('gone')
    expect(channelsHref([{ id: 'c1' }])).toBe('/channels/c1')
  })

  it('nav render đúng href và giữ link tạo kênh khi không truyền đích', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    roots.push(root)
    await act(async () => {
      root.render(<MainNav route={{ name: 'workbench' }} reviewCount={0} channelHref="/channels/c2" />)
    })
    expect(container.querySelector('a[href="#/channels/c2"]')?.textContent).toContain('Kênh')

    const second = document.createElement('div')
    document.body.append(second)
    const root2 = createRoot(second)
    roots.push(root2)
    await act(async () => {
      root2.render(<MainNav route={{ name: 'workbench' }} reviewCount={0} />)
    })
    expect(second.querySelector('a[href="#/channels/new"]')).toBeTruthy()
  })
})
