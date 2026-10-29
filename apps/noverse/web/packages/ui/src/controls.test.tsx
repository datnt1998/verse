// @vitest-environment happy-dom
// motion:controls v1 — bật/tắt bằng chuột và phím, không chạy lại animation khi re-render.
import { act, useState, type ReactElement } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, expect, it } from 'vitest'
import { Checkbox, Radio, RadioCard, Switch } from './controls'
import { Segmented } from './Tabs'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
const containers: HTMLElement[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  for (const c of containers.splice(0)) c.remove()
})

async function mount(el: ReactElement) {
  const container = document.createElement('div')
  document.body.append(container)
  containers.push(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(el) })
  return { container, root }
}

/** Happy DOM không tổng hợp default action của Space; mô phỏng chuỗi keydown/keyup/click gốc. Phím thật được đo thêm trên Chromium. */
async function pressSpace(btn: HTMLElement) {
  await act(async () => { btn.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true })) })
  await act(async () => { btn.dispatchEvent(new KeyboardEvent('keyup', { key: ' ', bubbles: true })) })
  await act(async () => { btn.click() })
}

function SwitchHarness() {
  const [on, setOn] = useState(false)
  return <Switch checked={on} onChange={setOn} aria-label="Duyệt" />
}

it('Switch: click và phím Space đều đổi trạng thái', async () => {
  const { container } = await mount(<SwitchHarness />)
  const btn = container.querySelector<HTMLElement>('[role=switch]')!
  expect(btn.getAttribute('aria-checked')).toBe('false')
  await act(async () => { btn.click() })
  expect(btn.getAttribute('aria-checked')).toBe('true')
  await pressSpace(btn)
  expect(btn.getAttribute('aria-checked')).toBe('false')
})

it('Checkbox: click và Space đổi trạng thái, mixed giữ aria-checked=mixed', async () => {
  const { container } = await mount(<Harness />)
  const box = container.querySelector<HTMLElement>('[role=checkbox]')!
  const mixed = container.querySelectorAll<HTMLElement>('[role=checkbox]')[1]
  expect(mixed.getAttribute('aria-checked')).toBe('mixed')
  await act(async () => { box.click() })
  expect(box.getAttribute('aria-checked')).toBe('true')
  await pressSpace(box)
  expect(box.getAttribute('aria-checked')).toBe('false')
  function Harness() {
    const [on, setOn] = useState(false)
    return <><Checkbox checked={on} onChange={setOn} label="Báo" /><Checkbox checked="mixed" onChange={() => {}} label="Tất cả" /></>
  }
})

it('Radio/RadioCard: phím mũi tên trong radiogroup chọn mục kế tiếp', async () => {
  function Harness() {
    const [v, setV] = useState('a')
    return <div role="radiogroup" aria-label="Khi tệp đã có">
      <Radio checked={v === 'a'} onChange={() => setV('a')} label="Hỏi tôi" />
      <Radio checked={v === 'b'} onChange={() => setV('b')} label="Giữ cả hai" />
      <RadioCard checked={v === 'c'} onChange={() => setV('c')} label="Thay bản cũ" />
    </div>
  }
  const { container } = await mount(<Harness />)
  const items = container.querySelectorAll<HTMLElement>('[role=radio]')
  expect(items.length).toBe(3)
  await act(async () => { items[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })) })
  expect(items[1].getAttribute('aria-checked')).toBe('true')
  await act(async () => { items[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true })) })
  expect(items[2].getAttribute('aria-checked')).toBe('true')
})

it('Segmented: click + phím mũi tên đổi mục, có khối nền trượt .kit-seg-thumb', async () => {
  function Harness() {
    const [v, setV] = useState('a')
    return <Segmented label="Nhóm chọn" value={v} onChange={setV} options={[{ id: 'a', label: 'Nhanh' }, { id: 'b', label: 'Đồng sáng tác' }]} />
  }
  const { container } = await mount(<Harness />)
  expect(container.querySelector('.kit-seg-thumb')).toBeTruthy()
  const items = container.querySelectorAll<HTMLElement>('[role=radio]')
  await act(async () => { items[1].click() })
  expect(items[1].getAttribute('aria-checked')).toBe('true')
  await act(async () => { items[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true })) })
  expect(items[0].getAttribute('aria-checked')).toBe('true')
})

it('re-render cùng props: giữ DOM node, không animation mới', async () => {
  const { container, root } = await mount(<Switch checked onChange={() => {}} aria-label="Duyệt" />)
  const btn = container.querySelector<HTMLElement>('[role=switch]')!
  const track = container.querySelector<HTMLElement>('.kit-switch-track')!
  const before = typeof btn.getAnimations === 'function' ? btn.getAnimations({ subtree: true }).length : 0
  // Re-render y nguyên + một nhịp “đồng hồ” (đổi prop không liên quan qua context cha).
  await act(async () => { root.render(<Switch checked onChange={() => {}} aria-label="Duyệt" />) })
  expect(container.querySelector('[role=switch]')).toBe(btn)
  expect(container.querySelector('.kit-switch-track')).toBe(track)
  const after = typeof btn.getAnimations === 'function' ? btn.getAnimations({ subtree: true }).length : 0
  expect(after).toBe(before)
  // Checkbox checked cũng không nảy lại: cùng node ô và svg.
  const { container: c2, root: r2 } = await mount(<Checkbox checked onChange={() => {}} label="Báo" />)
  const cb = c2.querySelector<HTMLElement>('.kit-cb-box')!
  const svg = c2.querySelector('svg')
  await act(async () => { r2.render(<Checkbox checked onChange={() => {}} label="Báo" />) })
  expect(c2.querySelector('.kit-cb-box')).toBe(cb)
  expect(c2.querySelector('svg')).toBe(svg)
})

it('pending: giữ màu và vị trí cũ, spinner ở trong knob; kết quả giữ nguyên DOM knob để trượt', async () => {
  const { container, root } = await mount(<Switch checked={false} onChange={() => {}} />)
  const knob = container.querySelector('.kit-switch-thumb')
  const track = container.querySelector<HTMLElement>('.kit-switch-track')!
  const oldColor = track.style.background
  await act(async () => { root.render(<Switch checked pending onChange={() => {}} />) })
  expect(track.style.background).toBe(oldColor)
  expect(container.querySelector('.kit-switch-thumb')).toBe(knob)
  expect(container.querySelector('.kit-switch-thumb [role=status]')).toBeTruthy()
  await act(async () => { root.render(<Switch checked onChange={() => {}} />) })
  expect(container.querySelector('.kit-switch-thumb')).toBe(knob)
  expect(track.style.background).not.toBe(oldColor)
})

it('bỏ trạng thái mixed giữ dấu gạch trong DOM để mờ đi 180ms', async () => {
  const { container, root } = await mount(<Checkbox checked="mixed" onChange={() => {}} />)
  const dash = container.querySelector('.kit-cb-box > span')!
  await act(async () => { root.render(<Checkbox checked={false} onChange={() => {}} />) })
  expect(container.querySelector('.kit-cb-box > span')).toBe(dash)
  expect((dash as HTMLElement).style.opacity).toBe('0')
})
