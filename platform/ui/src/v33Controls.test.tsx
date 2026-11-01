// @vitest-environment happy-dom
// V33 kit controls: Dialog body pad, Tabs gạch chân, ChipSelect, Dropzone/FileRow,
// ProgressBar chưa biết %, Button icon vuông 44/48.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import * as stylex from '@stylexjs/stylex'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Button } from './Button'
import { Dialog } from './Dialog'
import { Dropzone, FileRow } from './Dropzone'
import { ProgressBar } from './feedback'
import { ChipSelect } from './Selects'
import { Tabs } from './Tabs'

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

const atomsOf = (html: string) => new Set((html.match(/class="([^"]*)"/g) ?? []).join(' ').split(/[" ]+/).filter(Boolean))

describe('Dialog thân hộp', () => {
  it('mặc định bọc thân có đệm theo padX; padBody={false} trả con trực tiếp', async () => {
    await mount(<Dialog title="Nhập" onClose={() => {}}><p>Thân</p></Dialog>)
    const body = document.querySelector('.kit-dlg-body')
    expect(body).not.toBeNull()
    expect(body!.textContent).toBe('Thân')
    await act(async () => { roots.pop()!.unmount() })
    document.body.replaceChildren()

    await mount(<Dialog title="Danh sách" onClose={() => {}} padBody={false}><p>Dòng</p></Dialog>)
    expect(document.querySelector('.kit-dlg-body')).toBeNull()
    expect(document.querySelector('[role="dialog"]')!.textContent).toContain('Dòng')
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

describe('Button icon theo cỡ', () => {
  it('icon + lg vuông 44, icon + xl vuông 48', () => {
    const contract = stylex.create({ lg: { width: 44, borderRadius: 12 }, xl: { width: 48, borderRadius: 13 } })
    const lg = atomsOf(renderToStaticMarkup(<Button variant="icon" size="lg" aria-label="Thêm" icon={<span />} />))
    for (const atom of stylex.props(contract.lg).className!.split(' ').filter(Boolean)) expect(lg.has(atom)).toBe(true)
    const xl = atomsOf(renderToStaticMarkup(<Button variant="icon" size="xl" aria-label="Thêm" icon={<span />} />))
    for (const atom of stylex.props(contract.xl).className!.split(' ').filter(Boolean)) expect(xl.has(atom)).toBe(true)
  })
})
