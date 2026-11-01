// @vitest-environment happy-dom
// 04-components › SegmentedTabs (đợt 34, #80) — nhóm chọn của Viderse co theo
// nhãn, không bao giờ cắt “…” (không có `text-overflow: ellipsis` ở mục), nhóm
// hẹp hơn nội dung thì cuộn ngang (`.kit-seg`), `--fill` chia đều nhưng nhãn
// dài vẫn giữ đủ chữ (`min-width: max-content`). Viên trượt đo theo hộp thật.
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import * as stylex from '@stylexjs/stylex'
import { Segmented as KitSegmented } from '@noverse/ui'
import { afterEach, describe, expect, it } from 'vitest'
import { Segmented } from './Segmented'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
async function mount(node: React.ReactNode) {
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

// Vitest biên dịch atom nhưng không chèn CSS vào happy-dom: so hợp đồng atom
// ngữ nghĩa, như controls.test.tsx của kit.
const contract = stylex.create({
  content: { flex: '0 0 auto' },
  equal: { flex: 1, minWidth: 0 },
  fill: { flex: '1 1 0', minWidth: 'max-content' },
  nowrap: { whiteSpace: 'nowrap' },
})
const hasStyles = (node: HTMLElement, style: stylex.StyleXStyles) => {
  const atoms = stylex.props(style).className!.split(' ').filter(Boolean)
  expect(atoms.length).toBeGreaterThan(0)
  for (const atom of atoms) expect(node.classList.contains(atom)).toBe(true)
}
const radios = (container: HTMLElement) => [...container.querySelectorAll<HTMLElement>('[role=radio]')]

const SORTS = [
  { id: 'outlier', label: 'Nổi trội' },
  { id: 'newest', label: 'Mới nhất' },
  { id: 'views', label: 'Lượt xem' },
]

describe('Segmented của Viderse', () => {
  it('mặc định co theo nhãn: mục flex 0 0 auto, nhãn không cắt “…”', async () => {
    const container = await mount(<Segmented label="Sắp xếp" value="newest" onChange={() => {}} options={SORTS} />)
    for (const radio of radios(container)) {
      hasStyles(radio, contract.content)
      const label = radio.querySelector('span')!
      expect(label.getAttribute('style') ?? '').not.toContain('ellipsis')
      expect((label as HTMLElement).style.textOverflow).toBe('')
      expect((label as HTMLElement).style.overflow).not.toBe('hidden')
    }
  })

  it('nhóm có .kit-seg: cuộn ngang ẩn thanh cuộn khi hẹp hơn nội dung', async () => {
    const container = await mount(<Segmented label="Sắp xếp" value="newest" onChange={() => {}} options={SORTS} />)
    expect(container.querySelector('[role=radiogroup]')!.classList.contains('kit-seg')).toBe(true)
  })

  it('fit="fill" chia đều nhưng nhãn dài vẫn giữ đủ chữ (min-width: max-content)', async () => {
    const container = await mount(<Segmented fit="fill" label="Loại video" value="long" onChange={() => {}} options={[{ id: 'short', label: 'Ngắn' }, { id: 'long', label: 'Video dài nhiều chữ' }]} />)
    for (const radio of radios(container)) hasStyles(radio, contract.fill)
  })

  it('kit dùng chung vẫn mặc định chia đều (Noverse không đổi)', async () => {
    const container = await mount(<KitSegmented label="Nhóm" value="a" onChange={() => {}} options={[{ id: 'a', label: 'A' }, { id: 'b', label: 'B' }]} />)
    for (const radio of radios(container)) hasStyles(radio, contract.equal)
  })

  it('không còn “…” ở nhãn khi đổi mục', async () => {
    function Harness() {
      const [v, setV] = useState('outlier')
      return <Segmented label="Sắp xếp" value={v} onChange={setV} options={SORTS} />
    }
    const container = await mount(<Harness />)
    await act(async () => { radios(container)[2].click() })
    expect(radios(container)[2].getAttribute('aria-checked')).toBe('true')
    expect(radios(container)[2].textContent).toBe('Lượt xem')
    for (const radio of radios(container)) expect(radio.textContent).not.toContain('…')
  })
})
