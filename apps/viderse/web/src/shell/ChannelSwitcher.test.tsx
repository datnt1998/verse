// @vitest-environment happy-dom
// Bộ đổi kênh đợt 36 (Gói V32): “Mọi kênh” là chồng avatar MỘT chữ cái
// (điện thoại 22px chồng −7, máy tính 20px chồng −6, viền tách 2px màu thanh);
// tên dài cắt “…” trong khi nút Việc nền và avatar giữ nguyên chỗ; chưa có
// kênh thì thay bằng nút “+ Tạo kênh” mở luồng tạo kênh.
// Vitest biên dịch atom nhưng không chèn CSS: so hợp đồng atom ngữ nghĩa
// (cùng cách Segmented.test.tsx của kit).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import * as stylex from '@stylexjs/stylex'
import { ChannelSwitcher, type SwitcherChannel } from './ChannelSwitcher'
import { t } from '../i18n/all'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
async function render(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  return container
}
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
})

const contract = stylex.create({
  overlapPhone: { marginLeft: 'calc(-1 * var(--vv-s-7))' },
  overlapDesktop: { marginLeft: 'calc(-1 * var(--vv-s-6))' },
  overlapMenu: { marginLeft: 'calc(-1 * var(--vv-s-5))' },
  label: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
})
const atomsOf = (style: stylex.StyleXStyles) => stylex.props(style).className!.split(' ').filter(Boolean)
const hasAtoms = (node: Element, style: stylex.StyleXStyles) => {
  const atoms = atomsOf(style)
  expect(atoms.length).toBeGreaterThan(0)
  return atoms.every((atom) => node.classList.contains(atom))
}

const CHANNELS: SwitcherChannel[] = [
  { id: 'mt', name: 'Midnight Tales', initials: 'MT', color: { bg: 'rgb(46,34,54)', ring: 'rgb(215,155,255)' } },
  { id: 'cl', name: 'Chuyện Lạ Đêm Khuya', initials: 'CL', color: { bg: 'rgb(48,36,24)', ring: 'rgb(246,203,140)' } },
  { id: 'qh', name: 'Quiet Hours', initials: 'QH', color: { bg: 'rgb(31,44,39)', ring: 'rgb(166,237,193)' } },
]
const entry = (c: HTMLElement) => c.querySelector('.vv-switcher, button[aria-haspopup="menu"]') as HTMLElement
const stacked = (c: HTMLElement) => {
  const holder = entry(c).querySelector('[aria-hidden="true"]') as HTMLElement
  return [...holder.children] as HTMLElement[]
}

describe('Bộ đổi kênh · Mọi kênh', () => {
  it('điện thoại: chồng avatar 22px MỘT chữ cái, chồng −7px, viền tách 2px', async () => {
    const c = await render(<ChannelSwitcher channels={CHANNELS} mobileBar onPick={() => {}} />)
    const avatars = stacked(c)
    expect(avatars).toHaveLength(3)
    expect(avatars.map((a) => a.textContent)).toEqual(['M', 'C', 'Q'])
    for (const avatar of avatars) {
      expect(avatar.style.width).toBe('22px')
      expect(avatar.style.height).toBe('22px')
      expect(avatar.style.fontSize).toBe('10px')
      expect(avatar.style.boxShadow).toContain('2px var(--color-surface-1)')
    }
    expect(hasAtoms(avatars[1], contract.overlapPhone)).toBe(true)
    expect(hasAtoms(avatars[2], contract.overlapPhone)).toBe(true)
    expect(hasAtoms(avatars[0], contract.overlapPhone)).toBe(false)
  })

  it('máy tính: cũng một chữ cái, 20px, chồng −6px', async () => {
    const c = await render(<ChannelSwitcher channels={CHANNELS} onPick={() => {}} />)
    const avatars = stacked(c)
    expect(avatars.map((a) => a.textContent)).toEqual(['M', 'C', 'Q'])
    for (const avatar of avatars) {
      expect(avatar.style.width).toBe('20px')
      expect(avatar.style.fontSize).toBe('10px')
    }
    expect(hasAtoms(avatars[1], contract.overlapDesktop)).toBe(true)
    expect(hasAtoms(avatars[0], contract.overlapDesktop)).toBe(false)
  })

  it('tên dài cắt “…” — nhãn co được, hai nút phải không phải co', async () => {
    const c = await render(<ChannelSwitcher channels={CHANNELS} mobileBar current="cl" onPick={() => {}} />)
    const label = entry(c).querySelector('.vv-switcher__name') as HTMLElement
    expect(label.textContent).toBe('Chuyện Lạ Đêm Khuya')
    expect(hasAtoms(label, contract.label)).toBe(true)
    // Nhãn co theo thanh; phần còn lại (avatar + mũi tên) là hằng số.
    expect(label.style.flexShrink === '' || label.style.flexShrink === '1').toBe(true)
  })

  it('câu aria-label theo 06-copy: “đang xem” / “đang chọn”', async () => {
    const all = await render(<ChannelSwitcher channels={CHANNELS} mobileBar onPick={() => {}} />)
    expect(entry(all).getAttribute('aria-label')).toBe(`${t.shell.channelSwitcher}, ${t.shell.viewingAllChannels}`)
    const one = await render(<ChannelSwitcher channels={CHANNELS} mobileBar current="mt" onPick={() => {}} />)
    expect(entry(one).getAttribute('aria-label')).toBe(`${t.shell.channelSwitcher}, ${t.shell.selectingChannel('Midnight Tales')}`)
  })

  it('một kênh vẫn là bộ đổi kênh (menu có Tạo kênh)', async () => {
    const c = await render(<ChannelSwitcher channels={CHANNELS.slice(0, 1)} mobileBar onPick={() => {}} />)
    expect(entry(c).tagName).toBe('BUTTON')
    expect(entry(c).getAttribute('aria-haspopup')).toBe('dialog')
  })
})

describe('Bộ đổi kênh · chưa có kênh', () => {
  it('điện thoại: nút “Tạo kênh” cùng cỡ, mở luồng tạo kênh', async () => {
    const c = await render(<ChannelSwitcher channels={[]} mobileBar onPick={() => {}} />)
    const link = entry(c)
    expect(link.tagName).toBe('A')
    expect(link.getAttribute('href')).toBe('#/channels/new')
    expect(link.textContent).toContain(t.nav.createChannel)
    expect(link.classList.contains('vv-switcher--new')).toBe(true)
    expect(c.querySelector('[aria-haspopup="menu"]')).toBeNull()
  })

  it('máy tính: vẫn là nút “Tạo kênh”, không menu rỗng', async () => {
    const c = await render(<ChannelSwitcher channels={[]} onPick={() => {}} />)
    const link = entry(c)
    expect(link.getAttribute('href')).toBe('#/channels/new')
    expect(link.textContent).toContain(t.nav.createChannel)
    expect(c.querySelector('[role="menu"]')).toBeNull()
  })
})

describe('Bộ đổi kênh · menu', () => {
  it('dòng Mọi kênh trong menu dùng avatar 16px hai chữ cái', async () => {
    const c = await render(<ChannelSwitcher channels={CHANNELS} sub="Hôm nay 4/7" onPick={() => {}} />)
    await act(async () => { entry(c).click() })
    const first = document.querySelector('[role="menuitemradio"]') as HTMLElement
    expect(first.textContent).toContain(t.nav.allChannels)
    const avatars = [...first.querySelectorAll('[role="img"]')] as HTMLElement[]
    expect(avatars.map((a) => a.textContent)).toEqual(['MT', 'CL', 'QH'])
    for (const avatar of avatars) expect(avatar.style.width).toBe('16px')
    expect(hasAtoms(avatars[1], contract.overlapMenu)).toBe(true)
  })

  it('điện thoại: pointerdown trong sheet portal không đóng trước khi chọn kênh', async () => {
    const picked: (string | undefined)[] = []
    const c = await render(<ChannelSwitcher channels={CHANNELS} mobileBar onPick={id => picked.push(id)} />)
    await act(async () => entry(c).click())
    const row = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent?.includes('Chuyện Lạ Đêm Khuya'))!
    await act(async () => row.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })))
    expect(row.isConnected).toBe(true)
    await act(async () => row.click())
    expect(picked).toEqual(['cl'])
    expect(document.querySelector('[role="dialog"]')).toBeNull()
  })
  it('bấm một kênh trong menu gọi onPick rồi đóng menu', async () => {
    const picked: (string | undefined)[] = []
    const c = await render(<ChannelSwitcher channels={CHANNELS} onPick={(id) => picked.push(id)} />)
    await act(async () => { entry(c).click() })
    const rows = [...document.querySelectorAll('[role="menuitemradio"]')] as HTMLElement[]
    await act(async () => { rows[2].click() })
    expect(picked).toEqual(['cl'])
    expect(document.querySelector('[role="menu"]')).toBeNull()
  })
})
