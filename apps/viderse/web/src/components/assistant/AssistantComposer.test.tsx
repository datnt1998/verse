// @vitest-environment happy-dom
// Thẻ Kế hoạch của ô Nhắn trợ lý: bỏ/giữ bước, đếm "Làm N bước", nhấn mạnh khi có
// bước đưa video vào làm, Esc/Huỷ không chạy gì, Sửa câu trả lại câu đã gõ, lỗi thật.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AssistantComposer } from './AssistantComposer'
import { assistantCopy as copy } from '../../i18n/assistant'

vi.mock('../../data/status', () => ({ useOnline: () => true }))

const step = (id: string, over: Record<string, unknown> = {}) => ({
  id, intent: 'redraw_image', params: {}, title: `Việc ${id}`, sub: `Phụ ${id}`, agent: 'artist',
  enqueuesVideo: false, detail: '', reruns: [], ...over,
})
const plan = (steps: unknown[], over: Record<string, unknown> = {}) => ({ ok: true, planId: 'p1', steps, total: { images: 0 }, ...over })

let calls: { url: string; body: unknown }[] = []
let planReply: () => Response
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })

const roots: Root[] = []
const flush = (ms = 20) => act(async () => { await new Promise<void>((resolve) => { setTimeout(resolve, ms) }) })

const mount = async (scope: Parameters<typeof AssistantComposer>[0]['scope']) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<AssistantComposer scope={scope} label="Nhắn cho trợ lý" placeholder="Nhắn" />) })
  return container
}

const type = async (el: HTMLElement, text: string) => {
  const input = el.querySelector('input')!
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, text)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await act(async () => { el.querySelector('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })) })
  await flush()
}
const button = (el: ParentNode, text: string) => [...el.querySelectorAll('button')].find((b) => b.textContent?.includes(text)) as HTMLButtonElement | undefined
const click = async (target: Element | undefined | null) => { expect(target).toBeTruthy(); await act(async () => { (target as HTMLElement).click() }); await flush() }
const rows = (el: ParentNode) => [...el.querySelectorAll<HTMLElement>('[role="checkbox"]')]
const key = async (target: Element, k: string) => act(async () => { target.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true })) })

beforeEach(() => {
  ;(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true
  calls = []
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    calls.push({ url, body: init?.body ? JSON.parse(String(init.body)) : undefined })
    if (url === '/api/assistant/plan') return planReply()
    if (url === '/api/assistant/run-plan') return json({ runId: 'r1', state: 'running', steps: [{ id: 'a', state: 'queued' }] }, 202)
    return json({ error: 'not found' }, 404)
  }))
})
afterEach(async () => {
  await act(async () => { for (const root of roots.splice(0)) root.unmount() })
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

const runCalls = () => calls.filter((call) => call.url === '/api/assistant/run-plan')

describe('thẻ Kế hoạch', () => {
  it('gửi câu chỉ lập kế hoạch; bỏ tích một bước đổi "Làm N bước" và chỉ gửi bước còn tích', async () => {
    planReply = () => json(plan([step('a'), step('b'), step('c')]))
    const el = await mount('all')
    await type(el, 'đưa video lên đầu, làm thêm 2 video')
    expect(calls.map((call) => call.url)).toEqual(['/api/assistant/plan'])
    expect(calls[0].body).toEqual({ scope: 'all', text: 'đưa video lên đầu, làm thêm 2 video' })
    expect(rows(el).map((row) => row.getAttribute('aria-checked'))).toEqual(['true', 'true', 'true'])
    expect(button(el, copy.plan.run(3))).toBeTruthy()

    await click(rows(el)[1])
    expect(rows(el)[1].getAttribute('aria-checked')).toBe('false')
    await click(button(el, copy.plan.run(2)))
    expect(runCalls()).toHaveLength(1)
    expect(runCalls()[0].body).toEqual({ planId: 'p1', steps: ['a', 'c'] })
  })

  it('bỏ hết bước thì nút Làm khoá và không gọi run-plan', async () => {
    planReply = () => json(plan([step('a')]))
    const el = await mount('all')
    await type(el, 'làm một việc')
    await click(rows(el)[0])
    const run = button(el, copy.plan.run(0))!
    expect(run.disabled).toBe(true)
    await click(run)
    expect(runCalls()).toHaveLength(0)
  })

  it('nút Làm đánh dấu khi có bước đưa video vào làm, và bỏ bước đó thì hết đánh dấu', async () => {
    planReply = () => json(plan([step('a'), step('b', { enqueuesVideo: true, intent: 'make_videos' })]))
    const el = await mount('all')
    await type(el, 'làm thêm video')
    expect(button(el, copy.plan.run(2))!.hasAttribute('data-enqueues-video')).toBe(true)
    await click(rows(el)[1])
    expect(button(el, copy.plan.run(1))!.hasAttribute('data-enqueues-video')).toBe(false)
  })

  it('Esc đóng thẻ mà không chạy gì', async () => {
    planReply = () => json(plan([step('a'), step('b')]))
    const el = await mount('all')
    await type(el, 'việc')
    await key(el.querySelector('input')!, 'Escape')
    expect(el.querySelector('[role="group"]')).toBeNull()
    expect(runCalls()).toHaveLength(0)
  })

  it('Sửa câu đóng thẻ và trả câu đã gõ vào ô nhập', async () => {
    planReply = () => json(plan([step('a'), step('b')]))
    const el = await mount('all')
    await type(el, 'cảnh 5 cho y tá cầm đèn pin')
    expect(el.querySelector('input')!.value).toBe('')
    await click(button(el, copy.plan.editText))
    expect(el.querySelector('input')!.value).toBe('cảnh 5 cho y tá cầm đèn pin')
    expect(rows(el)).toHaveLength(0)
    expect(runCalls()).toHaveLength(0)
  })

  it('một việc trên một video hiện thẻ xác nhận; Làm chạy đúng bước đó', async () => {
    planReply = () => json(plan([step('a', { title: 'Vẽ lại cảnh 5', detail: 'Hoạ sĩ vẽ lại theo góp ý', reruns: [{ agent: 'artist', label: 'cảnh 5' }, { agent: 'qa', label: 'dựng lại' }], cost: { images: 1 } })], { imagesLeftToday: 7 }))
    const el = await mount({ jobId: 'j1' })
    await type(el, 'cảnh 5 cho y tá cầm đèn pin')
    expect(el.querySelector('[aria-label="Xác nhận việc cho Hoạ sĩ"]')).toBeTruthy()
    expect(el.textContent).toContain('Vẽ lại cảnh 5')
    expect(el.textContent).toContain('1 ảnh')
    expect(runCalls()).toHaveLength(0)
    await click(button(el, copy.confirm.run))
    expect(runCalls()[0].body).toEqual({ planId: 'p1', steps: ['a'] })
  })

  it('không hiểu: nói rõ "Chưa làm được việc này", gợi ý chỉ lập kế hoạch mới', async () => {
    planReply = () => json({ ok: false, unknown: { lead: 'Trợ lý chỉ làm việc trên video đang mở.', suggestions: [{ label: 'Tạm dừng video này', text: 'tạm dừng video này' }] } })
    const el = await mount({ jobId: 'j1' })
    await type(el, 'xoá kênh')
    expect(el.textContent).toContain(copy.unknown.title)
    planReply = () => json(plan([step('p', { intent: 'pause' })]))
    await click(button(el, 'Tạm dừng video này'))
    expect(calls.at(-1)!.body).toEqual({ scope: { jobId: 'j1' }, text: 'tạm dừng video này' })
    expect(runCalls()).toHaveLength(0)
  })

  it('không có model: hiện lỗi thật, giữ được Thử lại và không chạy gì', async () => {
    planReply = () => json({ error: 'chưa có model', code: 'model_unavailable' }, 503)
    const el = await mount('all')
    await type(el, 'việc')
    expect(el.querySelector('[role="alert"]')!.textContent).toContain(copy.states.model)
    expect(button(el, copy.states.retry)).toBeTruthy()
    expect(runCalls()).toHaveLength(0)
  })

  it('câu dài quá 500 ký tự không gửi đi', async () => {
    planReply = () => json(plan([step('a')]))
    const el = await mount('all')
    await type(el, 'a'.repeat(501))
    expect(calls).toHaveLength(0)
    expect(el.textContent).toContain(copy.states.tooLong)
  })
})
