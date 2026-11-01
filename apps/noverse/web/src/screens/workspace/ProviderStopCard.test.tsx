// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Config, ConfigResponse, Snapshot } from '../../api/types'

const state = vi.hoisted(() => ({ config: null as ConfigResponse | null, refresh: vi.fn(), toast: vi.fn() }))
vi.mock('../../data/appModels', () => ({
  useAppConfig: () => ({ data: state.config, refresh: state.refresh }),
  appConfigDoc: (response: ConfigResponse | null) => response?.target.config ?? {},
}))
vi.mock('../../data/useResource', () => ({ useResource: () => ({ data: { name: 'model-b', contextWindow: 200000 } }) }))
vi.mock('../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../api/client', () => ({
  getConfigModels: vi.fn(),
  swapProvider: vi.fn(async () => ({ swapped: [{ role: 'default' }], undoToken: 'undo' })),
}))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), Icon: () => null, toast: state.toast }))
import { swapProvider } from '../../api/client'
import { ProviderStopCard } from './ProviderStopCard'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const config: Config = {
  provider: 'openai', model: 'model-a',
  providers: { openai: { models: [{ name: 'model-a' }] }, anthropic: { models: [{ name: 'model-b' }] }, deepseek: { models: [{ name: 'model-c' }] } },
}
const stopped = (provider: string, kind: 'expired' | 'key' = 'expired') => ({ currentChapter: 4, inProgressChapter: 4, isRunning: false, providerError: { provider, kind, message: '' } }) as Snapshot
let root: Root
let container: HTMLDivElement
const resume = vi.fn()
beforeEach(() => {
  vi.clearAllMocks()
  state.config = { target: { config }, version: 'v1' } as ConfigResponse
  container = document.createElement('div')
  document.body.append(container)
  root = createRoot(container)
})
afterEach(async () => { await act(() => root.unmount()); container.remove() })
async function render(snapshot: Snapshot) {
  await act(async () => { root.render(<ProviderStopCard snapshot={snapshot} isRunning={snapshot.isRunning} onResume={resume} />) })
}
async function swap() {
  const button = [...container.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent === 'Đổi và viết tiếp')!
  await act(async () => { button.click() })
}

describe('provider stop recovery episodes', () => {
  it('shows a second provider failure on the same mounted card after a successful swap and runtime resume', async () => {
    await render(stopped('openai'))
    await swap()
    expect(swapProvider).toHaveBeenCalledWith('openai', 'anthropic', 'model-b', 'v1')
    expect(resume).toHaveBeenCalledOnce()
    state.config = { target: { config: { ...config, provider: 'anthropic', model: 'model-b' } }, version: 'v2' } as ConfigResponse
    await render({ ...stopped('openai'), providerError: null, isRunning: true })
    expect(container.textContent).toContain('Đang viết tiếp chương 4')
    await render(stopped('anthropic', 'key'))
    const alert = container.querySelector('[role="alert"]')
    expect(alert).not.toBeNull()
    expect(alert?.textContent).toContain('Khoá Claude không còn dùng được')
    const recover = [...alert!.querySelectorAll<HTMLButtonElement>('button')].find((item) => item.textContent === 'Đổi và viết tiếp')
    expect(recover?.disabled).toBe(false)
    expect(container.textContent).not.toContain('Đang viết tiếp chương')
    expect(container.textContent).not.toContain('Đã chuyển 1 trợ lý')
  })

  it('never claims writing resumed merely because configuration swap or the resume callback completed', async () => {
    await render(stopped('openai'))
    await swap()
    expect(container.textContent).not.toContain('Đang viết tiếp chương')
    expect(container.querySelector('[role="alert"]')).not.toBeNull()
    await render({ ...stopped('openai'), providerError: null })
    expect(container.textContent).not.toContain('Đang viết tiếp chương')
  })
})
