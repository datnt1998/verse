// @vitest-environment happy-dom
// Lỗi POST /api/rivals → câu hiện ngay dưới ô nhập theo `code` (mỗi code một
// câu), exists kèm link Mở đối thủ. Kiểm cả render trong form.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AddRivalForm, addRivalError } from './AddRivalForm'
import { ApiError } from '../../api/request'
import { vi as t } from '../../i18n/vi'

const tt = t.screens.explore.addErrors
const apiErr = (status: number, body: object) => new ApiError(status, 'api error', undefined, body)

const roots: Root[] = []
const respond = (body: unknown, status = 200) =>
  Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } }))

async function mount(opened: { id?: string } = {}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => {
    root.render(<AddRivalForm onAdded={() => {}} onOpen={(id) => { opened.id = id }} />)
  })
  return container
}

const typeIn = async (input: HTMLInputElement, value: string) => {
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set
    setter?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

const submit = async (form: HTMLFormElement) => {
  await act(async () => {
    form.dispatchEvent(new SubmitEvent('submit', { bubbles: true, cancelable: true }))
  })
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('addRivalError — mã lỗi → câu dưới ô', () => {
  it('invalid_input', () => {
    expect(addRivalError(apiErr(400, { code: 'invalid_input' })).message).toBe(tt.invalid_input)
  })
  it('not_found', () => {
    expect(addRivalError(apiErr(404, { code: 'not_found' })).message).toBe(tt.not_found)
  })
  it('exists mang theo id đối thủ đã có', () => {
    const e = addRivalError(apiErr(409, { code: 'exists', rival: { id: 'r9' } }))
    expect(e.message).toBe(tt.exists)
    expect(e.rivalId).toBe('r9')
  })
  it('rate_limited điền giờ thử lại từ retryAt', () => {
    const at = new Date('2026-10-30T15:30:00').toISOString()
    const time = new Date(at).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    expect(addRivalError(apiErr(429, { code: 'rate_limited', retryAt: at })).message).toContain(time)
  })
  it('blocked ưu tiên recovery của server', () => {
    expect(addRivalError(apiErr(403, { code: 'blocked', recovery: 'Thử lại sau ít phút.' })).message).toBe('Thử lại sau ít phút.')
  })
  it('unexpected_page', () => {
    expect(addRivalError(apiErr(502, { code: 'unexpected_page' })).message).toBe(tt.unexpected_page)
  })
  it('lỗi không phải ApiError → message của lỗi', () => {
    expect(addRivalError(new Error('mạng lỗi')).message).toBe('Mạng lỗi')
  })
})

describe('form thêm đối thủ — lỗi hiện dưới ô', () => {
  it('mỗi code render câu lỗi của nó trong role=alert', async () => {
    const cases: [{ status: number; body: object }, RegExp | string][] = [
      [{ status: 400, body: { error: 'e', code: 'invalid_input' } }, tt.invalid_input],
      [{ status: 404, body: { error: 'e', code: 'not_found' } }, tt.not_found],
      [{ status: 429, body: { error: 'e', code: 'rate_limited', retryAt: new Date('2026-10-30T15:30:00').toISOString() } }, /thử lại lúc/],
      [{ status: 403, body: { error: 'e', code: 'blocked' } }, tt.blocked],
      [{ status: 502, body: { error: 'e', code: 'unexpected_page' } }, tt.unexpected_page],
    ]
    for (const [{ status, body }, text] of cases) {
      vi.stubGlobal('fetch', vi.fn(() => respond(body, status)))
      const el = await mount()
      await typeIn(el.querySelector('input')!, '@kenh')
      await submit(el.querySelector('form')!)
      const alert = el.querySelector('[role="alert"]')
      expect(alert?.textContent).toMatch(typeof text === 'string' ? text : text)
    }
  })

  it('exists → link "Mở" chọn đối thủ đã theo dõi', async () => {
    vi.stubGlobal('fetch', vi.fn(() => respond({ error: 'e', code: 'exists', rival: { id: 'r9' } }, 409)))
    const opened: { id?: string } = {}
    const el = await mount(opened)
    await typeIn(el.querySelector('input')!, '@kenh')
    await submit(el.querySelector('form')!)
    await click([...el.querySelectorAll('a')].find((a) => a.textContent === tt.existsOpen))
    expect(opened.id).toBe('r9')
  })
})
