// @vitest-environment happy-dom
// Danh sách lệnh `/` của ô Nhắn trợ lý: chỉ lệnh hợp lệ với video đang mở, lọc khi gõ,
// ↑↓ Enter Esc và chuột. Chọn lệnh chỉ điền chữ, không gửi gì.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AssistantComposer } from './AssistantComposer'
import { slashCommandsFor, type SlashCommand, type SlashJob } from './slashCommands'

vi.mock('../../data/status', () => ({ useOnline: () => true }))

const job = (over: Partial<SlashJob> = {}): SlashJob => ({ stageGroup: 'working', activeStep: 'images', noImages: false, ...over })
const ids = (j: SlashJob) => slashCommandsFor(j).map((command) => command.id)

describe('lệnh hợp lệ với video đang mở', () => {
  it('theo thứ tự của đặc tả, chỉ những lệnh có nghĩa ở trạng thái hiện tại', () => {
    expect(ids(job())).toEqual(['redraw', 'provider', 'pause'])
    expect(ids(job({ stageGroup: 'paused' }))).toEqual(['redraw', 'provider', 'resume'])
    expect(ids(job({ stageGroup: 'attention', activeStep: 'score' }))).toEqual(['redraw', 'provider', 'export-anyway'])
    expect(ids(job({ stageGroup: 'ready' }))).toEqual(['redraw', 'provider', 'posted'])
  })

  it('video không vẽ ảnh không có /redraw; video đã đăng không còn lệnh nào', () => {
    expect(ids(job({ noImages: true }))).toEqual(['provider', 'pause'])
    expect(ids(job({ stageGroup: 'posted' }))).toEqual(['redraw'])
  })
})

const roots: Root[] = []
const mount = async (commands: SlashCommand[]) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<AssistantComposer scope={{ jobId: 'j1' }} label="Nhắn cho trợ lý" placeholder="Nhắn" commands={commands} hints={{ esc: true }} />) })
  return container
}
const input = (el: HTMLElement) => el.querySelector('input')!
const typeInto = async (el: HTMLElement, text: string) => act(async () => {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input(el), text)
  input(el).dispatchEvent(new Event('input', { bubbles: true }))
})
const press = async (el: HTMLElement, key: string) => {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
  await act(async () => { input(el).dispatchEvent(event) })
  return event
}
const options = (el: HTMLElement) => [...el.querySelectorAll<HTMLElement>('[role="option"]')]
const usages = (el: HTMLElement) => options(el).map((option) => option.firstElementChild!.textContent)

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 404, headers: { 'content-type': 'application/json' } })))
})
afterEach(async () => {
  await act(async () => { for (const root of roots.splice(0)) root.unmount() })
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('danh sách lệnh trên ô nhập', () => {
  it('gõ / mở danh sách, gõ tiếp thì lọc, hết khớp hoặc có dấu cách thì đóng', async () => {
    const el = await mount(slashCommandsFor(job()))
    expect(options(el)).toHaveLength(0)
    await typeInto(el, '/')
    expect(usages(el)).toEqual(['/redraw <cảnh>', '/provider <chatgpt|grok|gemini>', '/pause'])
    await typeInto(el, '/p')
    expect(usages(el)).toEqual(['/provider <chatgpt|grok|gemini>', '/pause'])
    await typeInto(el, '/pa')
    expect(usages(el)).toEqual(['/pause'])
    await typeInto(el, '/xyz')
    expect(options(el)).toHaveLength(0)
    await typeInto(el, '/redraw 3')
    expect(options(el)).toHaveLength(0)
  })

  it('↑↓ chọn vòng quanh, Enter điền tên lệnh vào ô và không gửi gì', async () => {
    const el = await mount(slashCommandsFor(job()))
    await typeInto(el, '/')
    expect(options(el)[0].getAttribute('aria-selected')).toBe('true')
    await press(el, 'ArrowUp')
    expect(options(el)[2].getAttribute('aria-selected')).toBe('true')
    await press(el, 'ArrowDown')
    await press(el, 'ArrowDown')
    expect(options(el)[1].getAttribute('aria-selected')).toBe('true')
    const enter = await press(el, 'Enter')
    expect(enter.defaultPrevented).toBe(true)
    expect(input(el).value).toBe('/provider ')
    expect(options(el)).toHaveLength(0)
    expect(fetch).not.toHaveBeenCalled()
  })

  it('Esc chỉ đóng danh sách: không đụng tới Esc của màn (tạm dừng video)', async () => {
    const el = await mount(slashCommandsFor(job()))
    const outside = vi.fn()
    document.addEventListener('keydown', outside)
    await typeInto(el, '/')
    await press(el, 'Escape')
    document.removeEventListener('keydown', outside)
    expect(options(el)).toHaveLength(0)
    expect(outside).not.toHaveBeenCalled()
    expect(input(el).value).toBe('/')
    await typeInto(el, '/r')
    expect(usages(el)).toEqual(['/redraw <cảnh>'])
  })

  it('bấm một dòng điền lệnh đó vào ô', async () => {
    const el = await mount(slashCommandsFor(job()))
    await typeInto(el, '/pa')
    await act(async () => { options(el)[0].click() })
    expect(input(el).value).toBe('/pause ')
    expect(options(el)).toHaveLength(0)
  })

  it('lệnh không tham số đã gõ đủ thì Enter gửi luôn thay vì điền thêm dấu cách', async () => {
    const el = await mount(slashCommandsFor(job()))
    await typeInto(el, '/pause')
    const enter = await press(el, 'Enter')
    expect(enter.defaultPrevented).toBe(false)
    expect(input(el).value).toBe('/pause')
  })

  it('không có lệnh nào (kênh, Mọi kênh) thì / chỉ là chữ thường', async () => {
    const el = await mount([])
    await typeInto(el, '/')
    expect(options(el)).toHaveLength(0)
    expect(input(el).getAttribute('role')).toBeNull()
  })
})
