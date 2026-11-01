// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Config, ConfigResponse, ModelRoleKey } from '../../../api/types'

const state = vi.hoisted(() => ({ response: null as ConfigResponse | null, refresh: vi.fn() }))
vi.mock('../../../data/useResource', () => ({ useResource: (key: string[]) => {
  const data = key[0] === 'app-model-catalog'
    ? { openai: [{ name: 'gpt-5', thinking: ['', 'low', 'high'] }], deepseek: [{ name: 'deepseek-chat', thinking: [''] }] }
    : key[0] === 'config-presets' ? { providers: [] } : state.response
  return { data, error: null, status: 'ready', refreshing: false, refresh: state.refresh }
} }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../../api/client', () => ({
  ConflictError: class extends Error {},
  getConfig: vi.fn(), getConfigModels: vi.fn(), getConfigPresets: vi.fn(), testConfig: vi.fn(),
  putConfig: vi.fn(async (_scope: string, config: Config) => ({ ...state.response, target: { config }, effective: config, version: 'v2' })),
  swapProvider: vi.fn(async () => ({ swapped: [{ role: 'default', from: { provider: 'openai', model: 'gpt-5' }, to: { provider: 'deepseek', model: 'deepseek-chat' } }], undoToken: 'undo-mobile', warning: 'Lựa chọn đã lưu; lượt đang chạy chưa đổi.' })),
  undoSwapProvider: vi.fn(async () => ({ restored: [], version: 'v3' })),
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), Icon: () => null, AgentOrb: () => null }))
vi.mock('../../../data/modelContext', () => ({ useModelContext: () => ({ data: undefined, error: null, status: 'ready' }), saveModelContexts: vi.fn() }))
import { putConfig, swapProvider, undoSwapProvider } from '../../../api/client'
import { Models } from './Models'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let root: Root
let container: HTMLDivElement
const toast = vi.fn()
const config: Config = { provider: 'openai', model: 'gpt-5', providers: { openai: { models: [{ name: 'gpt-5' }] }, deepseek: { models: [{ name: 'deepseek-chat' }] } } }
beforeEach(() => {
  vi.clearAllMocks()
  state.response = { target: { config }, effective: config, version: 'v1' } as ConfigResponse
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(async () => { await act(() => root.unmount()); container.remove() })
async function show(active: ModelRoleKey | null) {
  await act(async () => { root.render(<Models active={active} onPick={vi.fn()} disabled={false} toast={toast} />) })
}

describe('M4 Model interactions', () => {
  it('choosing reasoning for an inherited role persists provider, model and effort in the actual global PUT', async () => {
    await show('writer')
    const high = [...container.querySelectorAll<HTMLButtonElement>('[role="radio"]')].find((button) => button.textContent === 'Cao')!
    await act(async () => { high.click() })
    expect(putConfig).toHaveBeenCalledWith('global', expect.objectContaining({ roles: { writer: { provider: 'openai', model: 'gpt-5', reasoning_effort: 'high' } } }), undefined, undefined, 'v1')
    expect(high.getAttribute('aria-checked')).toBe('true')
  })

  it('does not announce a save before acknowledgement or after failure, and retries the failed choice', async () => {
    let reject!: (reason: Error) => void
    vi.mocked(putConfig).mockImplementationOnce(() => new Promise((_resolve, fail) => { reject = fail }))
    await show('writer')
    const high = [...container.querySelectorAll<HTMLButtonElement>('[role="radio"]')].find((button) => button.textContent === 'Cao')!
    await act(async () => { high.click() })
    expect(high.getAttribute('aria-checked')).toBe('true')
    expect(toast).not.toHaveBeenCalled()
    await act(async () => { reject(new Error('Mất kết nối khi lưu')) })
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Đang dùng lựa chọn đã lưu trước đó')
    expect(high.getAttribute('aria-checked')).toBe('false')
    expect(toast).not.toHaveBeenCalled()
    const retry = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Thử lại')!
    await act(async () => { retry.click() })
    expect(putConfig).toHaveBeenCalledTimes(2)
    expect(high.getAttribute('aria-checked')).toBe('true')
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('keeps undo after the refreshed primary provider becomes healthy and displays swap warnings', async () => {
    state.response = { ...state.response, providerErrors: { openai: { kind: 'expired', message: '' } } } as ConfigResponse
    await show(null)
    const swap = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent?.includes('Chuyển 4 trợ lý'))!
    await act(async () => { swap.click() })
    expect(swapProvider).toHaveBeenCalledWith('openai', 'deepseek', 'deepseek-chat', 'v1')
    expect(toast).toHaveBeenCalledWith('Lựa chọn đã lưu; lượt đang chạy chưa đổi.')
    const healthy: Config = { ...config, provider: 'deepseek', model: 'deepseek-chat' }
    state.response = { ...state.response, target: { config: healthy }, effective: healthy, version: 'v2' } as ConfigResponse
    await show(null)
    const undo = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Hoàn tác')
    expect(undo).toBeTruthy()
    await act(async () => { undo!.click() })
    expect(undoSwapProvider).toHaveBeenCalledWith('undo-mobile')
  })
})
