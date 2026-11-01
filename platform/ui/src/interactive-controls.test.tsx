// @vitest-environment happy-dom
// Interactive kit controls: Dialog body pad, underline Tabs, ChipSelect, Dropzone/FileRow,
// ProgressBar chưa biết %, Button icon vuông 44/48.
import { act, StrictMode, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Dropzone, FileRow } from './Dropzone'
import { ProgressBar } from './feedback'
import { ChipSelect } from './Selects'
import { Segmented, Tabs } from './Tabs'
import { ActionMenu } from './Menus'
import { Dialog } from './Dialog'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
})

async function mount(node: React.ReactNode) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  return container
}


describe('Dialog focus ownership', () => {
  it('restores the opener after closing, without stealing focus on rerender in StrictMode', async () => {
    function Fixture() {
      const [open, setOpen] = useState(false)
      const [revision, setRevision] = useState(0)
      return <><button onClick={() => setOpen(true)}>Open dialog</button>{open && <Dialog title="Edit" onClose={() => setOpen(false)}><button onClick={() => setRevision(value => value + 1)}>Revision {revision}</button></Dialog>}</>
    }
    const container = await mount(<StrictMode><Fixture /></StrictMode>)
    const opener = container.querySelector<HTMLButtonElement>('button')!
    opener.focus()
    await act(async () => { opener.click() })
    const edit = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent === 'Revision 0')!
    edit.focus()
    await act(async () => { edit.click() })
    expect(document.activeElement).toBe(edit)
    await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(document.activeElement).toBe(opener)
  })

  it('returns to the exact parent-dialog trigger before restoring the page trigger', async () => {
    function Fixture() {
      const [outer, setOuter] = useState(false)
      const [inner, setInner] = useState(false)
      return <><button onClick={() => setOuter(true)}>Page trigger</button>{outer && <Dialog title="Outer" onClose={() => setOuter(false)}><button onClick={() => setInner(true)}>Child trigger</button>{inner && <Dialog title="Inner" onClose={() => setInner(false)}><input aria-label="Value" /></Dialog>}</Dialog>}</>
    }
    const container = await mount(<Fixture />)
    const pageTrigger = container.querySelector<HTMLButtonElement>('button')!
    pageTrigger.focus()
    await act(async () => { pageTrigger.click() })
    const childTrigger = [...document.querySelectorAll<HTMLButtonElement>('[role="dialog"] button')].find(button => button.textContent === 'Child trigger')!
    childTrigger.focus()
    await act(async () => { childTrigger.click() })
    await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(document.activeElement).toBe(childTrigger)
    expect(document.querySelectorAll('[role="dialog"]')).toHaveLength(1)
    await act(async () => { document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })) })
    expect(document.activeElement).toBe(pageTrigger)
  })
})

describe('Tabs gạch chân', () => {
  it('tablist có aria-selected, id/aria-controls theo cặp tabs và mũi tên đổi tab', async () => {
    const onChange = vi.fn()
    const container = await mount(
      <Tabs variant="underline" label="Thư viện" value="templates" onChange={onChange} tabs={{ id: 'lib', panel: 'lib-panel' }}
        items={[{ id: 'templates', label: 'Template' }, { id: 'prompts', label: 'Prompt ảnh' }]} />,
    )
    const tabs = container.querySelectorAll('[role="tab"]')
    expect(tabs).toHaveLength(2)
    expect(tabs[0].getAttribute('aria-selected')).toBe('true')
    expect(tabs[0].id).toBe('lib-templates')
    expect(tabs[0].getAttribute('aria-controls')).toBe('lib-panel')
    await act(async () => { tabs[0].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })) })
    expect(onChange).toHaveBeenCalledWith('prompts')
  })
})

it('keeps custom values unselected but keyboard reachable, skipping disabled choices', async () => {
  const onChange = vi.fn()
  const container = await mount(<Segmented label="Motion" value="custom" onChange={onChange} options={[
    { id: 'locked', label: 'Locked', disabled: true },
    { id: 'still', label: 'Still' },
    { id: 'zoom', label: 'Zoom' },
  ]} />)
  const choices = [...container.querySelectorAll<HTMLButtonElement>('[role="radio"]')]
  expect(choices.every(choice => choice.getAttribute('aria-checked') === 'false')).toBe(true)
  expect(choices.filter(choice => choice.tabIndex === 0)).toEqual([choices[1]])
  choices[1].focus()
  await act(async () => { choices[1].dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })) })
  expect(onChange).toHaveBeenLastCalledWith('zoom')
  expect(document.activeElement).toBe(choices[2])
})

describe('ChipSelect', () => {
  it('nút chip mở danh sách, chọn một phát onChange và đóng', async () => {
    const onChange = vi.fn()
    const container = await mount(
      <ChipSelect label="Kênh" prefix="Kênh" value="a" onChange={onChange}
        options={[{ value: 'a', label: 'Midnight Tales' }, { value: 'b', label: 'Quiet Hours' }]} />,
    )
    const trigger = container.querySelector<HTMLButtonElement>('button[role="combobox"]')!
    expect(trigger.getAttribute('aria-expanded')).toBe('false')
    await act(async () => { trigger.click() })
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    const options = container.querySelectorAll('[role="option"]')
    expect(options).toHaveLength(2)
    await act(async () => { (options[1] as HTMLElement).click() })
    expect(onChange).toHaveBeenCalledWith('b')
  })
})

describe('Dropzone + FileRow', () => {
  it('bấm và Enter gọi onPick; thả tệp gọi onDrop với đúng danh sách', async () => {
    const onPick = vi.fn()
    const onDrop = vi.fn()
    const container = await mount(<Dropzone label="Thả tệp" title="Kéo tệp .zip vào đây" onPick={onPick} onDrop={onDrop} />)
    const zone = container.querySelector<HTMLElement>('[role="button"]')!
    await act(async () => { zone.click() })
    await act(async () => { zone.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    expect(onPick).toHaveBeenCalledTimes(2)
    const file = new File(['x'], 'a.zip', { type: 'application/zip' })
    await act(async () => {
      const event = new Event('drop', { bubbles: true }) as Event & { dataTransfer: { files: File[] } }
      Object.defineProperty(event, 'dataTransfer', { value: { files: [file], items: [] } })
      zone.dispatchEvent(event)
    })
    expect(onDrop).toHaveBeenCalledWith([file])
  })

  it('dòng tệp nói đúng trạng thái và mở danh sách cảnh báo tại chỗ', async () => {
    const container = await mount(<FileRow name="a.zip" status="ok" meta="Đã nhập · 14 media vào kho media" warnings={['Font thiếu']} warningsLabel="1 điều cần biết" />)
    expect(container.textContent).toContain('Đã nhập · 14 media vào kho media')
    const toggle = container.querySelector<HTMLButtonElement>('button[aria-expanded]')!
    expect(toggle.getAttribute('aria-expanded')).toBe('false')
    await act(async () => { toggle.click() })
    expect(toggle.getAttribute('aria-expanded')).toBe('true')
    expect(container.textContent).toContain('Font thiếu')
  })

  it('ProgressBar chưa biết % không bịa aria-valuenow', () => {
    const html = renderToStaticMarkup(<ProgressBar indeterminate />)
    expect(html).toContain('role="progressbar"')
    expect(html).not.toContain('aria-valuenow')
  })
})


describe('Menu links and commands', () => {
  it('keeps native navigation and includes links in keyboard traversal without activating locked items', async () => {
    const command = vi.fn()
    const container = await mount(<ActionMenu label="Actions" items={[
      { id: 'channel', label: 'Channel', href: '#/channels/a', target: '_blank', rel: 'noopener' },
      { id: 'front', label: 'Move first', onSelect: command },
      { id: 'locked', label: 'Unavailable', href: '#/locked', lockedReason: 'Still running' },
      { id: 'disabled', label: 'Disabled', href: '#/disabled', disabled: true },
    ]} />)
    const trigger = container.querySelector<HTMLButtonElement>('[aria-haspopup="menu"]')!
    await act(async () => { trigger.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })) })
    const link = document.querySelector<HTMLAnchorElement>('[role="menuitem"][href]')!
    expect(document.activeElement).toBe(link)
    expect(link.href).toMatch(/#\/channels\/a$/)
    expect(link.target).toBe('_blank')
    await act(async () => { link.dispatchEvent(new KeyboardEvent('keydown', { key: 'End', bubbles: true })) })
    const locked = document.querySelector<HTMLButtonElement>('[role="menuitem"][aria-disabled="true"]')!
    expect(document.activeElement).toBe(locked)
    await act(async () => { locked.click() })
    expect(command).not.toHaveBeenCalled()
    expect(document.querySelector('[role="menu"]')).not.toBeNull()
    await act(async () => { locked.dispatchEvent(new KeyboardEvent('keydown', { key: 'Home', bubbles: true })) })
    expect(document.activeElement).toBe(link)
    await act(async () => { link.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })) })
    expect(document.activeElement?.textContent).toBe('Move first')
    await act(async () => { (document.activeElement as HTMLButtonElement).click() })
    expect(command).toHaveBeenCalledOnce()
    expect(document.querySelector('[role="menu"]')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })
})
