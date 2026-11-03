// @vitest-environment happy-dom
// V17 — Kết nối AI, wire khớp internal/web/ai.go: thêm OpenRouter → chọn 2
// model → Lưu PATCH selected; key sai → thẻ cam (Đổi khoá · Thử lại · Thêm
// theo tên); catalog theo trang {models,total,nextCursor} — cửa sổ ≤200 hàng
// với Trang trước/sau; openai_compat gửi cả khoá + địa chỉ; gỡ provider
// khoá/địa chỉ qua xác nhận cam trước DELETE; ChatGPT authorize giữ cửa sổ
// trong gesture + link fallback khi popup bị chặn.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { useEffect, useState } from 'react'
import { invalidate } from '@noverse/ui/data'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ConnectSection } from './ConnectSection'
import { AddProviderBody } from './AddProvider'
import { ModelCatalog, PAGE } from './ModelCatalog'
import { ChatGPTLoginAction } from './ChatGPTLogin'
import type { AiModel, AiProvider } from '../../../api/ai'
import { aiCopy } from '../../../i18n/ai'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const chatgpt: AiProvider = {
  id: 'chatgpt', kind: 'chatgpt', auth: 'subscription', label: 'ChatGPT',
  catalogCount: 12, selected: [], status: 'ready', dailyPaidImageLimit: 50,
}
const grok: AiProvider = {
  id: 'grok', kind: 'grok', auth: 'subscription', label: 'Grok',
  catalogCount: 4, selected: [], status: 'logged_out', dailyPaidImageLimit: 50,
}
const openrouter: AiProvider = {
  id: 'p-or', kind: 'openrouter', auth: 'api_key', label: 'OpenRouter',
  keyHint: '••••7c2e', catalogCount: 340, selected: [], status: 'ready', dailyPaidImageLimit: 50,
}
const failedProvider: AiProvider = {
  id: 'p-failed', kind: 'openrouter', auth: 'api_key', label: 'OpenRouter',
  catalogCount: 0, selected: [], status: 'auth_failed', dailyPaidImageLimit: 50,
}

const catalog: AiModel[] = [
  { id: 'minimax/minimax-m2.5', caps: ['text', 'vision'], contextK: 205, price: { inPerM: 0.3, outPerM: 1.2 }, recommended: true },
  { id: 'openai/gpt-5.2', caps: ['text'], contextK: 400, price: { inPerM: 2, outPerM: 8 } },
  { id: 'google/nano-banana', caps: ['image'], price: { perImage: 0.039 } },
]

const roots: Root[] = []
let calls: { url: string; method: string; body?: string }[] = []
let providerState: AiProvider[] = []
let manyModels = false
let loginPhase = 'idle'

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))

/** Nhích đồng hồ ảo để cache/fetch xong — không chờ thời gian thật. */
const flush = async () => {
  for (let i = 0; i < 4; i++) await act(async () => { await vi.advanceTimersByTimeAsync(50) })
}

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    if (url === '/api/ai/providers' && method === 'GET') return respond(providerState)
    if (url === '/api/ai/providers' && method === 'POST') {
      const body = JSON.parse(init!.body as string) as { kind: AiProvider['kind']; apiKey?: string }
      if (body.apiKey === 'bad-key') {
        providerState = [{ ...failedProvider }, ...providerState]
        return respond({ error: 'nhà cung cấp từ chối khoá', reason: 'auth_failed' }, 401)
      }
      const id = providerState.some((provider) => provider.id === 'p-new') ? 'p-new-2' : 'p-new'
      providerState = [{ ...openrouter, id, kind: body.kind, selected: [] }, ...providerState]
      return respond(providerState[0], 201)
    }
    const models = url.match(/^\/api\/ai\/providers\/([^/]+)\/models(\?.*)?$/)
    if (models) {
      // Wire thật: {models,total,nextCursor} — keyset theo `cursor`.
      const params = new URLSearchParams(models[2] ?? '')
      const cursor = params.get('cursor')
      const limit = Number(params.get('limit') ?? 200)
      if (params.get('filter') === 'selected') {
        const sel = providerState.find((p) => p.id === models[1])?.selected ?? []
        const models2 = catalog.filter((m) => sel.includes(m.id))
        return respond({ models: models2, total: models2.length, nextCursor: null })
      }
      if (manyModels) {
        const all: AiModel[] = Array.from({ length: 500 }, (_, i) => ({ id: `vendor/model-${String(i).padStart(3, '0')}`, caps: ['text'] }))
        const start = cursor ? all.findIndex((m) => m.id === cursor) + 1 : 0
        const page = all.slice(start, start + limit)
        return respond({ models: page, total: all.length, nextCursor: start + limit < all.length ? page.at(-1)!.id : null })
      }
      const filtered = params.get('filter') === 'recommended' ? catalog.filter((model) => model.recommended) : catalog
      return respond({ models: filtered, total: filtered.length, nextCursor: null })
    }
    if (url.match(/^\/api\/ai\/providers\/[^/]+\/test$/)) return respond({ ok: true, ms: 120 })
    if (url.match(/^\/api\/ai\/providers\/[^/]+\/login$/) && method === 'GET') {
      return respond({ phase: loginPhase, authUrl: loginPhase === 'authorize' ? 'https://auth.openai.com/authorize?x=1' : undefined })
    }
    if (url.match(/^\/api\/ai\/providers\/[^/]+\/login$/) && method === 'POST') {
      loginPhase = 'authorize'
      return respond({ phase: 'authorize', authUrl: 'https://auth.openai.com/authorize?x=1' }, 202)
    }
    const del = url.match(/^\/api\/ai\/providers\/([^/]+)$/)
    if (del && method === 'DELETE') {
      providerState = providerState.filter((p) => p.id !== del[1])
      return respond({ ok: true })
    }
    if (del && method === 'PATCH') {
      const body = JSON.parse(init!.body as string) as { selected?: string[] }
      providerState = providerState.map((p) => p.id === del[1] ? { ...p, selected: body.selected ?? p.selected } : p)
      return respond(providerState.find((p) => p.id === del[1]))
    }
    if (url === '/api/ai/agents') {
      return respond({ director: { model: { provider: '', model: '' }, effort: 'auto', fallback: [] }, artist: { promptModel: { provider: '', model: '' }, draw: { provider: '', model: '' }, fallbackOn: true, drawFallback: [] } })
    }
    return respond({ error: 'not found', reason: 'not_found' }, 404)
  }))
}

async function render(node = <ConnectSection />) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(node) })
  await flush()
  return container
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
}

const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))

/** Dialog xác nhận cuối cùng (confirm() mount riêng trong body). */
const lastDialog = () => [...document.querySelectorAll('[role="dialog"],[role="alertdialog"]')].pop()

const setInput = async (input: HTMLInputElement, value: string) => {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

beforeEach(() => {
  vi.useFakeTimers()
  void invalidate()
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} })))
  providerState = [chatgpt, grok]
  calls = []
  manyModels = false
  loginPhase = 'idle'
  installFetch()
})

afterEach(async () => {
  await invalidate()
  vi.useRealTimers()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('Kết nối AI', () => {
  it('thêm OpenRouter → chọn 2 model → thanh chưa lưu → Lưu PATCH selected', async () => {
    const el = await render()
    await click(byText(el, 'button', 'Thêm nhà cung cấp'))
    const tiles = [...el.querySelectorAll('[role="radio"]')] as HTMLButtonElement[]
    await click(tiles.find((tile) => tile.textContent?.startsWith('OpenRouter')))
    const keyInput = el.querySelector('input[type="password"]') as HTMLInputElement
    expect(keyInput).toBeTruthy()
    await setInput(keyInput, 'sk-or-good')
    await click(byText(el, 'button', 'Kết nối và lấy model'))
    await flush()

    // conn=added: ghi chú vàng “Đã kết nối … chọn ít nhất một model”.
    expect(el.querySelector('.conn-note')?.textContent).toContain('Đã kết nối OpenRouter')
    await click(byText(el, '[role="radio"]', aiCopy.connect.filterAll))
    await flush()

    const rows = [...el.querySelectorAll<HTMLButtonElement>('.conn-row [role="checkbox"]')]
    await click(rows[0])
    await click(rows[1])
    expect(el.querySelector('.conn-dirty')?.textContent).toContain('thay đổi chưa lưu')
    await click(byText(el.querySelector('.conn-dirty')!, 'button', 'Lưu'))
    const patch = calls.find((c) => c.method === 'PATCH' && c.url === '/api/ai/providers/p-new')
    expect(patch).toBeTruthy()
    expect(JSON.parse(patch!.body!)).toEqual({ selected: ['minimax/minimax-m2.5', 'openai/gpt-5.2'] })
  })

  it('conn=added: thanh Huỷ/Lưu hiện NGAY khi chưa tick model nào; Lưu ghi nháp; Huỷ gỡ thanh', async () => {
    const el = await render()
    await click(byText(el, 'button', 'Thêm nhà cung cấp'))
    const tiles = [...el.querySelectorAll('[role="radio"]')] as HTMLButtonElement[]
    await click(tiles.find((tile) => tile.textContent?.startsWith('OpenRouter')))
    await setInput(el.querySelector('input[type="password"]') as HTMLInputElement, 'sk-or-good')
    await click(byText(el, 'button', 'Kết nối và lấy model'))
    await flush()

    // Artboard conn=added: strip “1 thay đổi chưa lưu” hiện kể cả khi nháp rỗng.
    const dirtyBar = () => el.querySelector('.conn-dirty')
    expect(dirtyBar()?.textContent).toContain('thay đổi chưa lưu')
    expect(dirtyBar()?.textContent).toContain('1')

    // Huỷ: thanh biến mất, không PATCH nào đi ra.
    await click(byText(dirtyBar()!, 'button', 'Huỷ'))
    expect(dirtyBar()).toBeNull()
    expect(calls.some((c) => c.method === 'PATCH')).toBe(false)

    // Mở lại add một lần nữa → thẻ mới lại là một thay đổi; Lưu lần này phải
    // PATCH thật kể cả khi chọn rỗng (lưu lần kết nối).
    calls = []
    await click(byText(el, 'button', 'Thêm nhà cung cấp'))
    const tiles2 = [...el.querySelectorAll('[role="radio"]')] as HTMLButtonElement[]
    await click(tiles2.find((tile) => tile.textContent?.startsWith('DeepSeek')))
    await setInput(el.querySelector('input[type="password"]') as HTMLInputElement, 'sk-ds-good')
    await click(byText(el, 'button', 'Kết nối và lấy model'))
    await flush()
    expect(dirtyBar()?.textContent).toContain('thay đổi chưa lưu')
    await click(byText(dirtyBar()!, 'button', 'Lưu'))
    await flush()
    const patch = calls.find((c) => c.method === 'PATCH' && c.url === '/api/ai/providers/p-new-2')
    expect(patch).toBeTruthy()
    expect(JSON.parse(patch!.body!)).toEqual({ selected: [] })
    expect(dirtyBar()).toBeNull()
  })
  it('openai_compat hiện CẢ khoá + địa chỉ và gửi đủ hai trường', async () => {
    const el = await render(<AddProviderBody onAdded={() => {}} onReveal={() => {}} providers={[]} />)
    const tiles = [...el.querySelectorAll('[role="radio"]')] as HTMLButtonElement[]
    await click(tiles.find((t) => t.textContent?.startsWith('Tương thích OpenAI')))
    const inputs = [...el.querySelectorAll('input')] as HTMLInputElement[]
    expect(inputs).toHaveLength(2)
    await setInput(inputs.find((i) => i.type === 'password')!, 'sk-compat')
    await setInput(inputs.find((i) => i.type !== 'password')!, 'https://llm.example.com/v1')
    await click(byText(el, 'button', 'Kết nối và lấy model'))
    const post = calls.find((c) => c.method === 'POST' && c.url === '/api/ai/providers')
    expect(JSON.parse(post!.body!)).toEqual({ kind: 'openai_compat', apiKey: 'sk-compat', baseUrl: 'https://llm.example.com/v1' })
  })

  it('chọn lại nguồn đã có: không tạo thẻ thứ hai, cuộn tới thẻ đó và mở Đổi khoá', async () => {
    providerState = [chatgpt, grok, { ...openrouter }]
    const scroll = vi.fn()
    Element.prototype.scrollIntoView = scroll
    const el = await render()
    await click(byText(el, 'button', 'Thêm nhà cung cấp'))
    const tile = [...el.querySelectorAll('[role="radio"]')].find((t) => t.textContent?.startsWith('OpenRouter')) as HTMLButtonElement
    expect(tile.disabled).toBe(false)
    expect(tile.textContent).toContain('Đã kết nối')
    await click(tile)
    expect(calls.some((c) => c.method === 'POST')).toBe(false)
    expect(el.querySelector('.conn-add')).toBeNull()
    await flush()
    expect(scroll).toHaveBeenCalled()
    expect(el.querySelector('[data-kind="openrouter"] .conn-edit input')).toBeTruthy()
  })

  it('Ollama và Tương thích OpenAI đã có thẻ vẫn chọn được để thêm địa chỉ khác', async () => {
    const ollama: AiProvider = { ...openrouter, id: 'p-ol', kind: 'ollama', auth: 'local', label: 'Ollama' }
    const reveal = vi.fn()
    const el = await render(<AddProviderBody onAdded={() => {}} onReveal={reveal} providers={[ollama]} />)
    const tile = [...el.querySelectorAll('[role="radio"]')].find((t) => t.textContent?.startsWith('Ollama')) as HTMLButtonElement
    await click(tile)
    expect(reveal).not.toHaveBeenCalled()
    expect(el.querySelector('input[type="password"]')).toBeNull()
    expect(el.querySelectorAll('input').length).toBeGreaterThan(0)
  })

  it('thẻ lỗi nằm trên cùng dù được thêm sau; server trả cũ trước', async () => {
    providerState = [chatgpt, grok, { ...openrouter, id: 'p-ok' }, { ...failedProvider, id: 'p-bad', kind: 'deepseek', label: 'DeepSeek' }]
    const el = await render()
    const kinds = [...el.querySelectorAll('.conn-card[data-kind]')].map((n) => n.getAttribute('data-kind'))
    expect(kinds).toEqual(['deepseek', 'chatgpt', 'grok', 'openrouter'])
  })

  it('key sai (auth_failed) → thẻ cam với Đổi khoá · Thử lại · Thêm theo tên', async () => {
    providerState = [chatgpt, { ...failedProvider }]
    const el = await render()
    const card = [...el.querySelectorAll('.conn-card')].find((node) => node.textContent?.includes('Không lấy được danh sách model.'))
    expect(card).toBeTruthy()
    expect(card!.textContent).toContain('Không lấy được danh sách model.')
    for (const action of ['Đổi khoá', 'Thử lại', 'Thêm theo tên']) {
      expect([...card!.querySelectorAll('button')].some((b) => b.textContent?.includes(action))).toBe(true)
    }
  })

  it('gỡ provider api_key trong hàng Đổi khoá → cam xác nhận trước DELETE; Huỷ giữ nguyên', async () => {
    providerState = [openrouter]
    const el = await render()
    const card = el.querySelector('.conn-card')!
    expect(byText(card, 'button', 'Đăng xuất')).toBeFalsy()
    await click(byText(card, 'button', 'Đổi khoá'))
    await click(byText(card, 'button', 'Gỡ'))
    // Xác nhận hiện trước — chưa có DELETE nào.
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
    const dialog = lastDialog()
    expect(dialog).toBeTruthy()
    await click(byText(dialog!, 'button', 'Huỷ'))
    await act(async () => { await vi.advanceTimersByTimeAsync(300) })
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false)
    await click(byText(card, 'button', 'Gỡ'))
    await click(byText(lastDialog()!, 'button', 'Gỡ'))
    await act(async () => { await vi.advanceTimersByTimeAsync(30) })
    const del = calls.filter((c) => c.method === 'DELETE' && c.url === '/api/ai/providers/p-or')
    expect(del).toHaveLength(1)
  })

  it('grok logged_out → chỉ nút Đăng nhập Grok', async () => {
    const el = await render()
    const card = [...el.querySelectorAll('.conn-card')].find((c) => c.textContent?.includes('Grok'))
    expect(byText(card!, 'button', 'Đăng nhập Grok')).toBeTruthy()
    // Artboard: thẻ chưa đăng nhập không có Model/Thử kết nối.
    expect(byText(card!, 'button', 'Thử kết nối')).toBeFalsy()
  })

  it('ChatGPT: giữ cửa sổ trong gesture; popup bị chặn → link "Mở trang đăng nhập" đúng authUrl', async () => {
    const opened: string[] = []
    vi.spyOn(window, 'open').mockImplementation(() => {
      opened.push('called')
      return null // trình duyệt chặn popup
    })
    const el = await render(<ChatGPTLoginAction onDone={() => {}} />)
    await click(byText(el, 'button', 'Đăng nhập ChatGPT'))
    await flush()
    expect(opened).toHaveLength(1) // window.open được gọi NGAY trong click
    const link = document.querySelector('a.conn-link') as HTMLAnchorElement | null
    expect(link).toBeTruthy()
    expect(link!.href).toBe('https://auth.openai.com/authorize?x=1')
  })

  it('filters the real catalog through the segmented selection while keeping model choices', async () => {
    const toggled = vi.fn()
    const el = await render(<ModelCatalog provider={openrouter} draft={[catalog[0]]} onToggle={toggled} />)
    const image = byText(el, '[role="radio"]', aiCopy.connect.filterRecommended)!
    await click(image)
    expect(image.getAttribute('aria-checked')).toBe('true')
    await flush()
    const choices = [...el.querySelectorAll<HTMLButtonElement>('[role="checkbox"]')]
    expect(choices).toHaveLength(1)
    expect(choices[0].getAttribute('aria-label')).toBe('minimax/minimax-m2.5')
    expect(choices[0].getAttribute('aria-checked')).toBe('true')
    await click(choices[0])
    expect(toggled).toHaveBeenCalledWith('minimax/minimax-m2.5', false, catalog[0])
  })

  it('keeps the provider form guarded until its required key and compatible endpoint are present', async () => {
    const added = vi.fn()
    const el = await render(<AddProviderBody providers={[]} onAdded={added} onReveal={() => {}} />)
    await click(byText(el, '[role="radio"]', 'Tương thích OpenAI'))
    const submit = byText(el, 'button', aiCopy.connect.connectFetch) as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    await setInput(el.querySelector('input[type="password"]') as HTMLInputElement, 'unit-key')
    expect(submit.disabled).toBe(true)
    expect(calls.some((call) => call.method === 'POST' && call.url === '/api/ai/providers')).toBe(false)
  })

  it(`catalog 500 model: ${PAGE} hàng/trang + tổng thật 500, Trang sau mở trang 2`, async () => {
    manyModels = true
    const provider = { ...openrouter, catalogCount: 500 }
    const el = await render(<ModelCatalog provider={provider} draft={[]} onToggle={() => {}} />)
    expect(el.querySelectorAll('.conn-row')).toHaveLength(PAGE)
    expect(el.querySelector('.conn-cat__foot')?.textContent).toContain('200 trong 500')
    expect(el.querySelectorAll('.conn-row__id')[0].textContent).toBe('vendor/model-000')
    await click(byText(el, 'button', 'Trang sau'))
    await flush()
    // Cửa sổ di chuyển: vẫn ≤ PAGE hàng, nhưng đã ở trang 2.
    expect(el.querySelectorAll('.conn-row').length).toBeLessThanOrEqual(PAGE)
    expect(el.querySelectorAll('.conn-row__id')[0].textContent).toBe('vendor/model-200')
    await click(byText(el, 'button', 'Trang trước'))
    await flush()
    expect(el.querySelectorAll('.conn-row__id')[0].textContent).toBe('vendor/model-000')
  })

  it('chọn giữ qua trang: tick trang 1 → sang trang 2 → về vẫn còn tick', async () => {
    manyModels = true
    const provider = { ...openrouter, catalogCount: 500 }
    let draft: AiModel[] = []
    const Harness = () => {
      const [d, setD] = useState(draft)
      useEffect(() => { draft = d }, [d])
      return <ModelCatalog provider={provider} draft={d} onToggle={(id, on, meta) => setD((cur) => on ? [...cur, meta ?? { id, caps: [] }] : cur.filter((m) => m.id !== id))} />
    }
    const el = await render(<Harness />)
    await click(el.querySelectorAll('.conn-row [role="checkbox"]')[0])
    expect(draft[0].id).toBe('vendor/model-000')
    await click(byText(el, 'button', 'Trang sau'))
    await flush()
    await click(byText(el, 'button', 'Trang trước'))
    await flush()
    expect(el.querySelectorAll('.conn-row [role="checkbox"]')[0].getAttribute('aria-checked')).toBe('true')
  })
})
