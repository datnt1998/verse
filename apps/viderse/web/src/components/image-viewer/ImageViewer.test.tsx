// @vitest-environment happy-dom
// Hành vi người dùng của Trình xem ảnh: chọn theo ID, ← → bỏ ô chưa vẽ,
// không nuốt phím khi đang gõ và Esc kết thúc lớp tương tác.
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { ImageViewer, type ImageViewerItem } from './index'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const items: ImageViewerItem[] = [
  { id: 'one', label: 'Cảnh 1', url: '/one.webp' },
  { id: 'two', label: 'Cảnh 2', disabled: true },
  { id: 'three', label: 'Cảnh 3', url: '/three.webp' },
]

function Harness() {
  const [selected, setSelected] = useState('one')
  const [open, setOpen] = useState(true)
  return <>
    {open && <ImageViewer
      mode="view"
      title="Video"
      items={items}
      selectedId={selected}
      onSelect={setSelected}
      side={<textarea aria-label="Góp ý cho Hoạ sĩ" />}
      footer={<span>Chân khung</span>}
      onClose={() => setOpen(false)}
    />}
    <output data-selected={selected} />
  </>
}

let root: Root
let host: HTMLDivElement
beforeEach(() => {
  host = document.createElement('div'); document.body.append(host)
  root = createRoot(host)
})
afterEach(async () => {
  await act(async () => root.unmount())
  host.remove(); document.body.replaceChildren()
})

const selected = () => host.querySelector('output')?.getAttribute('data-selected')
const press = async (key: string, target: EventTarget = document.body) => {
  await act(async () => { target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })) })
}

it('← → bỏ qua ô chưa vẽ và quay vòng', async () => {
  await act(async () => root.render(<Harness />))
  expect(selected()).toBe('one')
  await press('ArrowRight')
  expect(selected()).toBe('three')
  await press('ArrowLeft')
  expect(selected()).toBe('one')
  await press('ArrowLeft')
  expect(selected()).toBe('three')
})

it('ô chưa vẽ không chọn được và không nhận focus bàn phím', async () => {
  await act(async () => root.render(<Harness />))
  const thumb = document.querySelector<HTMLButtonElement>('[role="option"][aria-label^="Cảnh 2"]')!
  expect(thumb.getAttribute('aria-disabled')).toBe('true')
  expect(thumb.tabIndex).toBe(-1)
  await act(async () => thumb.click())
  expect(selected()).toBe('one')
})

it('không đổi ảnh khi con trỏ đang ở ô góp ý', async () => {
  await act(async () => root.render(<Harness />))
  const input = document.querySelector<HTMLTextAreaElement>('textarea')!
  input.focus()
  await press('ArrowRight', input)
  expect(selected()).toBe('one')
})

it('Esc kết thúc tương tác của viewer', async () => {
  await act(async () => root.render(<Harness />))
  await press('Escape')
  const active = [...document.querySelectorAll('[role="dialog"]')].filter(node => !node.closest('[inert], [aria-hidden="true"]'))
  expect(active).toHaveLength(0)
})
