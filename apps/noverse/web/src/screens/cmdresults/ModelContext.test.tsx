import { setTimeout as delay } from 'node:timers/promises'
// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Config, ConfigModelInfo, ModelsResponse } from '../../api/types'
import { cache } from '@noverse/ui/data'
import { formatContext } from './logic'
import { formatContextK } from '../../lib/modelContext'

const state = vi.hoisted(() => ({ admin: true, config: {} as Config }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: { role: state.admin ? 'admin' : 'writer' } }) }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: () => null,  AgentOrb: () => null,  toast: Object.assign(vi.fn(), { ok: vi.fn() }) }))
vi.mock('../../api/client', () => ({
  getModels: vi.fn(async (): Promise<ModelsResponse> => ({ roles: [{ role: 'writer', provider: 'openai', model: 'test-model', thinking: '', thinkingOptions: [''] }], providers: [{ provider: 'openai', label: 'OpenAI', ready: true, models: [{ name: 'test-model', contextWindow: 400000, registryWindow: 400000, contextSource: 'registry', thinking: [''] }] }] })),
  getConfigModels: vi.fn(async () => { const own = state.config.providers?.openai.models?.[0].context_window; return { models: [{ name: 'test-model', displayName: 'test-model', contextWindow: own ?? 400000, registryWindow: 400000, contextSource: own ? 'model_config' : 'registry', automaticWindow: 400000, automaticSource: 'registry', summaryThreshold: (own ?? 400000) * .85 } as ConfigModelInfo] } }),
  getConfig: vi.fn(async () => ({ target: { config: state.config }, version: 'v' })),
  putConfig: vi.fn(async (_scope, config) => { state.config = config; return {} }), setModel: vi.fn(), setThinking: vi.fn(),
}))
import { getConfigModels, getModels, putConfig } from '../../api/client'
import { ModelPanel } from './ModelPanel'
import { Models } from '../more/mobile/Models'
import { ContextSelect } from '../settings/sections/ContextSelect'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
beforeEach(() => { cache.clear(); vi.clearAllMocks(); state.admin = true; state.config = { providers: { openai: { models: [{ name: 'test-model' }] } } } })
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); cache.clear(); document.body.replaceChildren() })
async function mount(node: React.ReactNode) { const el = document.createElement('div'); document.body.append(el); const root = createRoot(el); roots.push(root); await act(async () => root.render(node)); return el }
const button = (text: string) => [...document.querySelectorAll<HTMLButtonElement>('button')].find(el => el.textContent === text || el.getAttribute('aria-label') === text)!
const click = async (text: string) => { expect(button(text), text).toBeTruthy(); await act(async () => button(text).click()) }
const input = async (value: string) => { const el = document.querySelector<HTMLInputElement>('input[type="number"]')!; expect(el).toBeTruthy(); await act(async () => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(el, value); el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })) }) }
const mobile = () => <Models project="p" story="s" active="writer" onPick={vi.fn()} disabled={false} toast={vi.fn()} />

describe('Gói W2 ngữ cảnh theo model', () => {
  it('formatter dùng K/M hoa và giữ phần lẻ theo đơn vị nghìn', () => {
    expect(formatContext(400000)).toBe('ngữ cảnh 400K')
    expect(formatContext(1000000)).toBe('ngữ cảnh 1M')
    expect(formatContextK(256000)).toBe('256K')
    expect(formatContextK(217600)).toBe('217,6K')
    expect(formatContextK(1000000)).toBe('1M')
    expect(formatContextK(0)).toBe('')
  })
  it('mở bảng model rồi render lại không tạo vòng tải catalog khi dữ liệu đã cũ', async () => {
    await mount(<ModelPanel project="p" story="s" onClose={vi.fn()} />)
    // The production cache captures its clock at creation; let its 2 s freshness expire.
    await delay(2100)
    await act(async () => roots[0].render(<ModelPanel project="p" story="s" onClose={vi.fn()} />))
    expect(getModels).toHaveBeenCalledTimes(1)
    expect(getConfigModels).toHaveBeenCalledTimes(1)
  })
  it('/model giữ thay đổi pending, chip và chân bảng; chỉ lưu khi bấm Lưu thay đổi', async () => {
    await mount(<ModelPanel project="p" story="s" onClose={vi.fn()} />)
    await click('Tự đặt'); await input('256')
    expect(putConfig).not.toHaveBeenCalled()
    expect(document.body.textContent).toContain('Ngữ cảnh 1 mô hình có thay đổi chưa lưu')
    expect(document.body.textContent).toContain('ngữ cảnh 256K')
    await click('Lưu thay đổi')
    expect(state.config.providers?.openai.models?.[0].context_window).toBe(256000)
    expect(document.body.textContent).not.toContain('có thay đổi chưa lưu')
  })
  it('mobile lưu ngay preset và Tự nhận xoá override', async () => {
    await mount(mobile()); await click('Tự đặt'); await click('128K')
    expect(state.config.providers?.openai.models?.[0].context_window).toBe(128000)
    expect(document.body.textContent).toContain('Đang dùng 128K')
    await click('Tự nhận · 400K')
    expect(state.config.providers?.openai.models?.[0]).not.toHaveProperty('context_window')
  })
  it('lưu /model cập nhật mobile và Settings qua cache chung', async () => {
    await mount(<ModelPanel project="p" story="s" onClose={vi.fn()} />)
    const el = await mount(mobile())
    const settings = await mount(<ContextSelect provider="openai" model={{ name: 'test-model', displayName: 'test-model', contextWindow: 400000, contextSource: 'registry', registryWindow: 400000, automaticWindow: 400000, automaticSource: 'registry', summaryThreshold: 340000 }} label="Ngữ cảnh" admin onSave={async () => {}} />)
    await click('Tự đặt'); await input('256'); await click('Lưu thay đổi')
    expect(el.querySelector<HTMLInputElement>('input[inputmode="numeric"]')?.value).toBe('256')
    expect(el.querySelector('[role="radio"][aria-label="Tự đặt"]')?.getAttribute('aria-checked')).toBe('true')
    expect(settings.textContent).toContain('256K · bạn đặt')
  })
  it('người viết chỉ xem, không đổi ngữ cảnh ở /model và mobile', async () => {
    state.admin = false
    await mount(<ModelPanel project="p" story="s" onClose={vi.fn()} />); await mount(mobile())
    const choices = [...document.querySelectorAll<HTMLButtonElement>('button')].filter(el => el.textContent === 'Tự đặt' || el.getAttribute('aria-label') === 'Tự đặt')
    expect(choices).toHaveLength(2); expect(choices.every(el => el.disabled)).toBe(true)
    choices.forEach(el => el.click()); expect(putConfig).not.toHaveBeenCalled()
  })
})
