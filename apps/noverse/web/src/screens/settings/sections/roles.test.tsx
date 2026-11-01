// @vitest-environment happy-dom
import { act, useState } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Config, ConfigResponse } from '../../../api/types'
import { getPath, setPath } from '../lib/config'
import type { SettingsContext } from '../types'
import { swapProvider, undoSwapProvider } from '../../../api/client'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), Icon: () => null, AgentOrb: () => null }))
vi.mock('../../../api/client', () => ({
  getConfigModels: vi.fn(async (_provider: string, models: string[]) => ({ models: models.map((name) => ({
    name, displayName: name, contextWindow: 200000, registryWindow: 200000, contextSource: 'registry', summaryThreshold: 170000,
    automaticWindow: 200000, automaticSource: 'registry',
    thinking: name === 'gpt-6.1-sol' ? ['', 'low', 'medium', 'high', 'xhigh'] : ['', 'low', 'medium', 'high'],
  })) })),
  swapProvider: vi.fn(async () => ({
    swapped: ['default', 'architect', 'writer', 'editor'].map((role) => ({
      role,
      from: { provider: 'openai', model: 'gpt-6.1-sol', reasoning_effort: 'xhigh' },
      to: { provider: 'deepseek', model: 'deepseek-chat', reasoning_effort: '' },
    })),
    skipped: [], undoToken: 'undo-1', expiresAt: '', version: 'w2',
  })),
  undoSwapProvider: vi.fn(async () => ({ restored: [], version: 'w3' })),
}))
import { RolesSection } from './roles'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []
afterEach(async () => { for (const root of roots.splice(0)) await act(() => root.unmount()); document.body.replaceChildren(); vi.clearAllMocks() })

const globalDoc: Config = {
  provider: 'openai', model: 'gpt-6.1-sol',
  providers: { openai: { models: [{ name: 'gpt-6.1-sol' }, { name: 'gpt-6-luna' }] }, deepseek: { models: [{ name: 'deepseek-chat' }] } } as Config['providers'],
  roles: { writer: { provider: '', model: '', fallbacks: [{ provider: 'deepseek', model: 'deepseek-chat' }] } },
}
const responseOf = (patch: Partial<ConfigResponse> = {}): ConfigResponse => ({
  target: { config: globalDoc }, effective: globalDoc, version: 'w', sources: {}, ...patch,
} as ConfigResponse)

type HarnessProps = {
  admin?: boolean
  initial?: Config
  response?: ConfigResponse
  error?: string
  onSave?: (next: Config) => void
  onRefresh?: () => ConfigResponse
}

/** Bắt chước index.tsx: lưu ngay, bản nháp đổi trước khi máy chủ trả lời. */
function Harness({ admin = true, initial = globalDoc, response = responseOf(), error = '', onSave, onRefresh }: HarnessProps) {
  const [fresh, setFresh] = useState<ConfigResponse | null>(null)
  response = fresh ?? response
  const [draft, setDraft] = useState<Config>(initial)
  const [appModelsError, setAppModelsError] = useState(error)
  const retryAppModels = vi.fn(() => setAppModelsError(''))
  const ctx = {
    scope: 'global', admin, canEdit: admin, draft, effective: response.effective, presets: null, snapshot: null,
    response, connection: { writable: true }, refreshConfig: () => {
      const next = onRefresh?.()
      if (next) { setFresh(next); setDraft(next.target.config) }
    }, goSection: vi.fn(),
    appModelsError, retryAppModels,
    saveAppModels: async (mutate: (config: Config) => Config) => {
      const next = mutate(draft)
      onSave?.(next)
      setDraft(next)
      return true
    },
  } as unknown as SettingsContext
  return <RolesSection ctx={ctx} />
}

async function mount(props: HarnessProps = {}) {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<Harness {...props} />) })
  return container
}
const row = (container: HTMLElement, key: string) => container.querySelector<HTMLElement>(`[data-role="${key}"]`)!
const picker = (container: HTMLElement, key: string) => row(container, key).querySelector<HTMLButtonElement>('[role="combobox"]')!
const levels = (container: HTMLElement, key: string) => [...row(container, key).querySelectorAll<HTMLElement>('[role="radio"]')]
const chooseModel = async (container: HTMLElement, key: string, label: string) => {
  await act(async () => { picker(container, key).click() })
  const option = [...row(container, key).querySelectorAll<HTMLElement>('[role="option"]')].find((node) => node.textContent?.includes(label))
  expect(option).toBeTruthy()
  await act(async () => { option!.click() })
}

describe('Trợ lý AI — model dùng chung, lưu ngay', () => {
  it('quản trị đổi model → thấy ngay, một bản cấu hình hoàn chỉnh, không có nút Lưu', async () => {
    const saves: Config[] = []
    const container = await mount({ onSave: (next) => saves.push(next) })
    expect(picker(container, 'writer').textContent).toContain('gpt-6.1-sol')
    await chooseModel(container, 'writer', 'gpt-6-luna')
    expect(picker(container, 'writer').textContent).toContain('gpt-6-luna')
    expect(saves).toHaveLength(1)
    // Một lần ghi có đủ provider + model của vai, không phải hai lần ghi rời.
    expect(getPath(saves[0], 'roles.writer.provider')).toBe('openai')
    expect(getPath(saves[0], 'roles.writer.model')).toBe('gpt-6-luna')
    expect(getPath(saves[0], 'provider')).toBe('openai')
    expect([...container.querySelectorAll('button')].some((button) => button.textContent === 'Lưu')).toBe(false)
  })

  it('mức suy luận đúng danh sách máy chủ trả theo model, có “Tự động” và “Tối thiểu”', async () => {
    const container = await mount()
    expect(levels(container, 'writer').map((node) => node.textContent)).toEqual(['Tự động', 'Thấp', 'Vừa', 'Cao', 'Rất cao'])
    await chooseModel(container, 'writer', 'gpt-6-luna')
    expect(levels(container, 'writer').map((node) => node.textContent)).toEqual(['Tự động', 'Thấp', 'Vừa', 'Cao'])
    expect(container.textContent).toContain('3 mức của gpt-6-luna')
  })

  it('đổi model làm mức đang chọn không còn hợp lệ → về Tự động và nói rõ mức cũ', async () => {
    const initial: Config = { ...globalDoc, roles: { writer: { provider: 'openai', model: 'gpt-6.1-sol', reasoning_effort: 'xhigh' } } }
    const saves: Config[] = []
    const container = await mount({ initial, onSave: (next) => saves.push(next) })
    expect(levels(container, 'writer').find((node) => node.textContent === 'Rất cao')?.getAttribute('aria-checked')).toBe('true')
    await chooseModel(container, 'writer', 'gpt-6-luna')
    expect(getPath(saves.at(-1), 'roles.writer.reasoning_effort')).toBe('')
    expect(levels(container, 'writer')[0].getAttribute('aria-checked')).toBe('true')
    expect(container.textContent).toContain('Mức Rất cao không có ở gpt-6-luna — đã về Tự động.')
  })

  it('chọn mức suy luận ghi ngay, chép provider+model khi vai đang theo Mặc định', async () => {
    const saves: Config[] = []
    const container = await mount({ onSave: (next) => saves.push(next) })
    const high = levels(container, 'architect').find((node) => node.textContent === 'Cao')!
    await act(async () => { high.click() })
    expect(saves).toHaveLength(1)
    expect(getPath(saves[0], 'roles.architect.provider')).toBe('openai')
    expect(getPath(saves[0], 'roles.architect.model')).toBe('gpt-6.1-sol')
    expect(getPath(saves[0], 'roles.architect.reasoning_effort')).toBe('high')
    expect(high.getAttribute('aria-checked')).toBe('true')
  })

  it('người viết chỉ xem: thấy ghi chú, không đổi được model hay mức', async () => {
    const container = await mount({ admin: false })
    expect(container.textContent).toContain('Model dùng chung cho mọi người nên chỉ quản trị đổi được.')
    expect(picker(container, 'writer').disabled).toBe(true)
    expect(levels(container, 'writer').every((node) => (node as HTMLButtonElement).disabled)).toBe(true)
  })

  it('lưu lỗi: giữ lựa chọn đã lưu trước đó và có nút Thử lại', async () => {
    const container = await mount({ error: 'Mất kết nối máy chủ. Đang dùng lựa chọn đã lưu trước đó — bấm Thử lại để lưu lại.' })
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Đang dùng lựa chọn đã lưu trước đó')
    const retry = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Thử lại')!
    expect(retry).toBeTruthy()
    await act(async () => { retry.click() })
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('hết hạn gói: thẻ Bắt sửa, chuyển cả loạt và Hoàn tác được', async () => {
    const response = responseOf({ accounts: { openai: { expired: true } } as unknown as ConfigResponse['accounts'] })
    const container = await mount({ response })
    expect(container.textContent).toContain('ChatGPT đã hết hạn gói')
    expect(container.textContent).toContain('Bắt sửa')
    const swap = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent?.startsWith('Chuyển 4 trợ lý sang'))!
    expect(swap).toBeTruthy()
    await act(async () => { swap.click() })
    expect(swapProvider).toHaveBeenCalledWith('openai', 'deepseek', 'deepseek-chat', 'w')
    expect(container.textContent).toContain('Đã chuyển 4 trợ lý sang nhà cung cấp khác')
    const undo = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Hoàn tác')!
    await act(async () => { undo.click() })
    expect(undoSwapProvider).toHaveBeenCalledWith('undo-1')
    expect(container.textContent).toContain('ChatGPT đã hết hạn gói')
  })

  it('Hoàn tác vẫn còn sau khi refresh đổi model chính sang nhà cung cấp không lỗi', async () => {
    const healthy: Config = { ...globalDoc, provider: 'deepseek', model: 'deepseek-chat', roles: {} }
    const container = await mount({
      response: responseOf({ providerErrors: { openai: { kind: 'expired', message: '' } } }),
      onRefresh: () => responseOf({ target: { ...responseOf().target, config: healthy }, effective: healthy, version: 'w2', providerErrors: { openai: { kind: 'expired', message: '' } } }),
    })
    const swap = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent?.startsWith('Chuyển 4 trợ lý sang'))!
    await act(async () => { swap.click() })
    expect(picker(container, 'default').textContent).toContain('deepseek-chat')
    const undo = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Hoàn tác')
    expect(undo).toBeTruthy()
    await act(async () => { undo!.click() })
    expect(undoSwapProvider).toHaveBeenCalledWith('undo-1')
  })

  it('quota là tạm thời: Cảnh báo, giờ thử lại, không mời đổi nhà cung cấp', async () => {
    const retryAt = new Date(Date.now() + 4 * 60_000).toISOString()
    const response = responseOf({ providerErrors: { openai: { kind: 'quota', message: '', retryAt } } })
    const container = await mount({ response })
    expect(container.textContent).toContain('ChatGPT tạm hết lượt')
    expect(container.textContent).toContain('Cảnh báo')
    expect(container.textContent).toContain('tự thử lại lúc')
    expect([...container.querySelectorAll('button')].some((button) => button.textContent?.startsWith('Chuyển'))).toBe(false)
    expect([...container.querySelectorAll('button')].some((button) => button.textContent?.startsWith('Đăng nhập lại'))).toBe(false)
  })

  it('khoá hỏng: nói rõ nhập khoá mới cho đúng nhà cung cấp', async () => {
    const response = responseOf({ providerErrors: { openai: { kind: 'key', message: 'invalid_api_key' } } })
    const container = await mount({ response })
    expect(container.textContent).toContain('Khoá ChatGPT không còn dùng được')
    expect([...container.querySelectorAll('button')].some((button) => button.textContent === 'Nhập khoá mới ChatGPT')).toBe(true)
    const tech = [...container.querySelectorAll<HTMLButtonElement>('button')].find((button) => button.textContent === 'Chi tiết kỹ thuật')!
    await act(async () => { tech.click() })
    expect(container.textContent).toContain('provider=openai · kind=key')
  })

  it('dự phòng đang lỗi hiện đúng loại lỗi trên chip', async () => {
    const response = responseOf({ providerErrors: { deepseek: { kind: 'key', message: '' } } })
    const container = await mount({ response })
    expect(container.textContent).toContain('1 · deepseek-chat · khoá không dùng được')
  })
})
