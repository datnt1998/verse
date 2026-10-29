// @vitest-environment happy-dom
// Gói S · Avatar nhân vật: màu/viền/huy hiệu theo loại vai, nhãn a11y “{tên}, {loại}”.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { CharAvatar, badgeGeometry } from './CharAvatar'
import { sortByKind } from '../lib/charKind'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root | undefined
let host: HTMLDivElement | undefined

afterEach(async () => {
  await act(() => root?.unmount())
  host?.remove()
  root = undefined
  host = undefined
})

const render = async (node: React.ReactNode) => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root!.render(node))
  return host.querySelector('[role="img"]') as HTMLElement
}

it('nhãn đọc “{tên}, {loại}” và thêm “, chưa lộ” khi chưa tới chương lộ', async () => {
  const lead = await render(<CharAvatar name="Lâm Tri Hạ" ini="TH" kind="lead" />)
  expect(lead.getAttribute('aria-label')).toBe('Lâm Tri Hạ, Chính')
  expect(lead.textContent).toBe('TH')

  const hidden = await render(<CharAvatar name="Phùng Chấn" kind="antagonist" hidden />)
  expect(hidden.getAttribute('aria-label')).toBe('Phùng Chấn, Phản diện, chưa lộ')
})

it('viền nét đứt khi chưa lộ khác nét liền khi đã lộ', async () => {
  const shown = await render(<CharAvatar name="Đỗ Mẫn" kind="support" />)
  const hidden = await render(<CharAvatar name="Kỳ Dương" kind="ally" hidden />)
  // StyleX gom thuộc tính vào class, nên so class: chỉ cái chưa lộ nhận thêm luật nét đứt.
  expect(hidden.className).not.toBe(shown.className)
})

it('huy hiệu góc chỉ hiện cho Chính/Phản diện/Đồng minh và chỉ khi avatar > 20 px', async () => {
  const chip = await render(<CharAvatar name="Lâm Tri Hạ" kind="lead" size="sm" />)
  expect(chip.querySelector('[aria-hidden="true"]')).toBeNull()
  const list = await render(<CharAvatar name="Lâm Tri Hạ" kind="lead" size="md" />)
  expect(list.querySelector('[aria-hidden="true"]')).not.toBeNull()
  const support = await render(<CharAvatar name="Đỗ Mẫn" kind="support" size="lg" />)
  expect(support.querySelector('[aria-hidden="true"]')).toBeNull()
})

it('huy hiệu theo cỡ avatar: 13 px dưới 40, 16 px từ 40, lệch 3/2 px, ẩn ở ≤ 20 px', () => {
  // Hai mốc lấy đúng từ artboard (Main.dc.html): 26 → 13/−3, tâm lệch 8; 40 → 16/−2, tâm lệch 12,5;
  // 46 (nút đang chọn ở ngăn kéo) → 16/−2, tâm lệch 15,5.
  expect(badgeGeometry(20)).toBeNull()
  expect(badgeGeometry(18)).toBeNull()
  const md = badgeGeometry(26)!
  expect([md.size, md.icon, md.offset, md.border, md.dx]).toEqual([13, 8, 3, 1.5, 8])
  expect(badgeGeometry(28)!.dx).toBe(9)
  expect(badgeGeometry(32)!.dx).toBe(11)
  expect(badgeGeometry(38)!.dx).toBe(14)
  const lg = badgeGeometry(40)!
  expect([lg.size, lg.icon, lg.offset, lg.border, lg.dx]).toEqual([16, 10, 2, 1.5, 12.5])
  expect(badgeGeometry(44)!.dx).toBe(14.5)
  expect(badgeGeometry(46)!.dx).toBe(15.5)
  expect(badgeGeometry(52)!.dx).toBe(18.5)
})

it('huy hiệu luôn nằm trong hộp avatar + phần nhô ra, nên mép ngoài không bị cắt', async () => {
  const avatar = await render(<CharAvatar name="Lâm Tri Hạ" kind="lead" size={26} />)
  const badge = avatar.querySelector('[aria-hidden="true"]') as HTMLElement
  const g = badgeGeometry(26)!
  expect(badge.style.width).toBe(`${g.size}px`)
  expect(badge.style.height).toBe(`${g.size}px`)
  expect(badge.style.right).toBe(`-${g.offset}px`)
  expect(badge.style.bottom).toBe(`-${g.offset}px`)
  // Mép ngoài huy hiệu nhô ra 3 − 1,5 (viền avatar) = 1,5 px ở góc dưới phải.
  expect(g.offset - g.border).toBe(1.5)
})

it('sắp Chính → Phản diện → Đồng minh → Phụ, giữ thứ tự trong cùng loại', () => {
  const chars = [
    { name: 'a', kind: 'support' as const },
    { name: 'b', kind: 'antagonist' as const },
    { name: 'c', kind: 'lead' as const },
    { name: 'd' },
    { name: 'e', kind: 'ally' as const },
    { name: 'f', kind: 'antagonist' as const },
  ]
  expect(sortByKind(chars).map((item) => item.name)).toEqual(['c', 'b', 'f', 'e', 'a', 'd'])
})
