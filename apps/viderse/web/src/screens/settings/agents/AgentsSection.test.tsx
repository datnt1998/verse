// @vitest-environment happy-dom
// V17 — Trợ lý AI, wire khớp domain.AgentModels: picker chỉ liệt kê model đã
// chọn ở Kết nối AI (metadata từ filter=selected, không bịa caps); PUT
// /api/ai/agents nối tiếp từ một bản nháp — thay đổi mới không bị PUT cũ đè;
// 422 capability_missing theo field dạng chấm (director.model, director.
// fallback, artist.promptModel, artist.draw, artist.drawFallback) → câu
// inline đúng chỗ, không báo "đã áp dụng".
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { invalidate } from '@noverse/ui/data'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { blockedReason, pickedFrom } from './ModelPicker'
import { AgentsSection } from './AgentsSection'
import type { AiAgents, AiModel, AiProvider } from '../../../api/ai'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const providers: AiProvider[] = [
  { id: 'p-or', kind: 'openrouter', auth: 'api_key', label: 'OpenRouter', catalogCount: 2, selected: ['openai/gpt-5.2', 'google/nano-banana'], status: 'ready', dailyPaidImageLimit: 50 },
]

const catalog: AiModel[] = [
  { id: 'openai/gpt-5.2', caps: ['text', 'vision'], contextK: 400 },
  { id: 'google/nano-banana', caps: ['image'], price: { perImage: 0.039 } },
]

const agents: AiAgents = {
  director: { model: { provider: '', model: '' }, effort: 'auto', fallback: [] },
  artist: { promptModel: { provider: 'p-or', model: 'openai/gpt-5.2' }, draw: { provider: 'p-or', model: 'google/nano-banana' }, fallbackOn: true, drawFallback: [] },
}

const roots: Root[] = []
let calls: { url: string; method: string; body?: string }[] = []
let err422: { field: string; need: string } | null = null
let holdPut: Promise<Response> | null = null
let savedAgents: AiAgents = agents

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))
/** Nhích đồng hồ ảo để cache/fetch xong — không chờ thời gian thật. */
const flush = async () => {
  for (let i = 0; i < 4; i++) await act(async () => { await vi.advanceTimersByTimeAsync(50) })
}

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<AgentsSection />) })
  await flush()
  return container
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))

beforeEach(() => {
  vi.useFakeTimers()
  void invalidate()
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} })))
  calls = []
  err422 = null
  holdPut = null
  savedAgents = structuredClone(agents)
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    if (url === '/api/ai/providers') return respond(providers)
    if (url.includes('/models')) {
      // Wire trang: filter=selected trả đúng model đã chọn kèm caps thật.
      return respond({ models: catalog, total: catalog.length, nextCursor: null })
    }
    if (url === '/api/ai/agents' && method === 'PUT') {
      const body = JSON.parse(init!.body as string) as AiAgents
      if (holdPut) {
        const response = await holdPut
        if (response.ok) savedAgents = body
        return response
      }
      if (err422) {
        return respond({ error: 'thiếu năng lực', reason: 'capability_missing', field: err422.field, need: err422.need }, 422)
      }
      savedAgents = body
      return respond(body)
    }
    if (url === '/api/ai/agents') return respond(savedAgents)
    return respond({ error: 'not found', reason: 'not_found' }, 404)
  }))
})

afterEach(async () => {
  await invalidate()
  vi.useRealTimers()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('Trợ lý AI', () => {

  it('picker chỉ có model đã chọn; model thiếu vision bị mờ cho Hoạ sĩ', async () => {
    const el = await render()
    const artistCard = [...el.querySelectorAll('.agt-row')].find((c) => c.textContent?.includes('Hoạ sĩ'))
    expect(artistCard).toBeTruthy()
    await click(artistCard!.querySelector('[role="combobox"]'))
    const list = artistCard!.querySelector('[role="listbox"]')
    expect(list).toBeTruthy()
    const disabled = [...list!.querySelectorAll<HTMLButtonElement>('[role="option"]')].filter((node) => node.disabled)
    expect(disabled).toHaveLength(1)
    expect(disabled[0].textContent).toContain('google/nano-banana')
    calls = []
    await click(disabled[0])
    expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(0)
  })

  it('catalog filter=selected: caps thật của model ngoài trang đầu vẫn đúng', async () => {
    const el = await render()
    // Nano Banana caps chỉ ['image'] — bị mờ cho model viết prompt (vision).
    const artistCard = [...el.querySelectorAll('.agt-row')].find((c) => c.textContent?.includes('Hoạ sĩ'))
    await click(artistCard!.querySelector('[role="combobox"]'))
    const nano = [...artistCard!.querySelectorAll<HTMLButtonElement>('[role="option"]')].find((o) => o.textContent?.includes('google/nano-banana'))
    expect(nano?.disabled).toBe(true)
    const draw = [...artistCard!.querySelectorAll('[role="radio"]')].find((d) => d.textContent?.includes('nano-banana'))

    expect(draw).toBeTruthy()
  })
  it('searches grouped models and immediately displays the selected model while saving the real reference', async () => {
    const el = await render()
    const card = el.querySelectorAll('.agt-card')[0]
    const trigger = card.querySelector('[role="combobox"]')
    await click(trigger)
    const search = card.querySelector('input[role="combobox"]') as HTMLInputElement
    await act(async () => {
      search.focus()
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(search, 'gpt-5.2')
      search.dispatchEvent(new Event('input', { bubbles: true }))
    })
    const options = card.querySelectorAll('[role="option"]')
    expect(options).toHaveLength(1)
    await act(async () => { search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })) })
    expect(el.querySelector<HTMLInputElement>('.agt-card input[role="combobox"]')?.value).toContain('openai/gpt-5.2')
    const save = calls.find((call) => call.method === 'PUT')
    expect(save).toBeTruthy()
    expect(JSON.parse(save!.body!).director.model).toEqual({ provider: 'p-or', model: 'openai/gpt-5.2' })
  })

  it('model catalog chưa tải → không bịa caps: không mờ, vẫn chọn được', async () => {
    // Trường hợp catalog filter=selected chưa về → caps undefined → không dim,
    // không phán thiếu năng lực (server 422 sẽ báo inline nếu sai thật).
    const unknown = pickedFrom(providers, () => undefined)
    expect(unknown.every((p) => p.caps === undefined)).toBe(true)
    expect(unknown.every((p) => blockedReason(p, 'vision') === null)).toBe(true)
    // Và model có trong catalog vẫn giữ caps thật kể cả khi provider mất dấu.
    const known = pickedFrom(providers, () => catalog)
    expect(known.find((p) => p.ref.model === 'google/nano-banana')?.caps).toEqual(['image'])
  })

  it('hai thay đổi liên tiếp trong lúc PUT đầu chưa xong → cả hai được gửi, không đè nhau', async () => {
    let resolve!: (r: Response) => void
    holdPut = new Promise<Response>((r) => { resolve = r })
    const el = await render()
    const directorCard = [...el.querySelectorAll('.agt-row')].find((c) => c.textContent?.includes('Đạo diễn'))!
    // Thay đổi 1: effort Thấp → PUT đầu bị giữ.
    await click(byText(directorCard, 'button', 'Thấp'))
    await act(async () => { await Promise.resolve() })
    expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(1)
    expect(JSON.parse(calls.find((c) => c.method === 'PUT')!.body!).director.effort).toBe('low')
    // Thay đổi 2 trong lúc PUT 1 đang treo: thêm fallback.
    await click(byText(directorCard, 'button', 'Thêm'))
    const addList = directorCard.querySelector('.agt-addlist')
    await click(addList!.querySelector('[role="combobox"]'))
    await click([...addList!.querySelectorAll('[role="option"]')].find((o) => o.textContent?.includes('openai/gpt-5.2')))
    await act(async () => { await Promise.resolve() })
    // Chưa có PUT thứ hai chồng lên.
    expect(calls.filter((c) => c.method === 'PUT')).toHaveLength(1)
    // Giải phóng PUT 1 → PUT 2 gửi doc MỚI NHẤT (effort low + fallback).
    holdPut = null
    await act(async () => { resolve(Response.json(agents)) })
    await flush()
    const puts = calls.filter((c) => c.method === 'PUT')
    expect(puts).toHaveLength(2)
    const body2 = JSON.parse(puts[1].body!) as AiAgents
    expect(body2.director.effort).toBe('low')
    expect(body2.director.fallback).toEqual([{ provider: 'p-or', model: 'openai/gpt-5.2' }])
  })

  it('chọn model dự phòng → PUT ModelRef đúng', async () => {
    const el = await render()
    const directorCard = [...el.querySelectorAll('.agt-row')].find((c) => c.textContent?.includes('Đạo diễn'))
    await click(byText(directorCard!, 'button', 'Thêm'))
    const addList = directorCard!.querySelector('.agt-addlist')
    expect(addList).toBeTruthy()
    await click(addList!.querySelector('[role="combobox"]'))
    const nano = [...addList!.querySelectorAll('[role="option"]')].find((o) => o.textContent?.includes('google/nano-banana'))
    expect(nano?.hasAttribute('disabled')).toBe(true)
    calls = []
    await click([...addList!.querySelectorAll('[role="option"]')].find((o) => o.textContent?.includes('openai/gpt-5.2')))
    const put = calls.find((c) => c.method === 'PUT' && c.url === '/api/ai/agents')
    expect(put).toBeTruthy()
    expect(JSON.parse(put!.body!).director.fallback).toEqual([{ provider: 'p-or', model: 'openai/gpt-5.2' }])
  })

  it.each([
    ['director.model', 'Đạo diễn', 'vision', 'không đọc được ảnh'],
    ['director.fallback', 'Đạo diễn', 'text', 'không viết được chữ'],
    ['artist.promptModel', 'Hoạ sĩ', 'vision', 'không đọc được ảnh'],
    ['artist.draw', 'Hoạ sĩ', 'image', 'không vẽ được ảnh'],
    ['artist.drawFallback', 'Hoạ sĩ', 'image', 'không vẽ được ảnh'],
  ])('422 capability_missing field=%s → câu inline đúng chỗ, không báo đã lưu', async (field, _card, need, message) => {
    err422 = { field, need }
    const el = await render()
    const directorCard = [...el.querySelectorAll('.agt-row')].find((c) => c.textContent?.includes('Đạo diễn'))!
    await click(byText(directorCard, 'button', 'Thấp'))
    await act(async () => { await Promise.resolve() })
    await flush()
    const err = [...el.querySelectorAll('.agt-caperr')].find((e) => e.textContent?.includes(message))
    expect(err).toBeTruthy()
    // Sau lỗi: câu lỗi nằm inline đúng trường; không có dòng “đã lưu” nào
    // (artboard không có câu thường trực, và lưu thất bại thì không được báo xong).
    const applied = [...el.querySelectorAll('.agt-applied')].map((h) => h.textContent)
    expect(applied).toHaveLength(0)
  })

  it('switch "Vẽ lại bằng nguồn khác khi lỗi" có tên truy cập', async () => {
    const el = await render()
    const sw = [...el.querySelectorAll('[role="switch"]')].find((s) => s.getAttribute('aria-label') === 'Vẽ lại bằng nguồn khác khi lỗi')
    expect(sw).toBeTruthy()
  })
})
