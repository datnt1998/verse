import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { Config } from '../../../api/types'

const state = vi.hoisted(() => ({
  config: { provider: 'openai', model: 'gpt-5' } as Config,
  catalog: { openai: [{ name: 'gpt-5', thinking: ['', 'low', 'medium', 'high', 'xhigh'] }] } as Record<string, { name: string; thinking: string[] }[]>,
}))
vi.mock('../../../data/useResource', () => ({
  useResource: (key: string[]) => {
    if (key[0] === 'app-model-catalog') return { data: state.catalog, error: null, status: 'ready', refreshing: false, refresh: vi.fn() }
    if (key[0] === 'config-presets') return { data: { providers: [] }, error: null, status: 'ready', refreshing: false, refresh: vi.fn() }
    return { data: { target: { config: state.config }, effective: state.config, version: 'v' }, error: null, status: 'ready', refreshing: false, refresh: vi.fn() }
  },
}))
vi.mock('../../../data/useAction', () => ({ useAction: () => ({ busy: false, pending: false, error: null, run: vi.fn() }) }))
vi.mock('../../../api/session', () => ({ useSession: () => ({ user: { role: 'admin' } }) }))
vi.mock('../../../api/client', () => ({ getConfigModels: vi.fn(), getConfigPresets: vi.fn(), testConfig: vi.fn(), swapProvider: vi.fn(), undoSwapProvider: vi.fn() }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()), Icon: () => null, AgentOrb: () => null }))
vi.mock('../../../data/modelContext', () => ({ useModelContext: () => ({ data: undefined, error: null, status: 'ready' }), saveModelContexts: vi.fn() }))
import { Models } from './Models'

const render = (role: { provider?: string; model?: string; reasoning_effort?: string } = {}) => {
  state.config = { provider: 'openai', model: 'gpt-5', providers: { openai: { models: [{ name: 'gpt-5' }] } }, roles: { writer: role as never } }
  return renderToStaticMarkup(<Models active="writer" onPick={vi.fn()} disabled={false} toast={vi.fn()} />)
}

describe('M4 Model › model dùng chung cả app (Gói AU)', () => {
  it('chưa chọn model: không in “Đang dùng: · .”', () => {
    state.config = { providers: { openai: { models: [{ name: 'gpt-5' }] } }, roles: { writer: { provider: '', model: '' } } }
    const html = renderToStaticMarkup(<Models active="writer" onPick={vi.fn()} disabled={false} toast={vi.fn()} />)
    expect(html).not.toContain('Đang dùng')
    expect(html).toContain('Chưa chọn model cho trợ lý này')
  })

  it('model cũ không còn trong danh sách: nêu tên model và nhà cung cấp, bỏ dấu chấm giữa khi thiếu nhà cung cấp', () => {
    expect(render({ provider: 'old', model: 'gpt-4' })).toContain('Đang dùng: <span class="mm-mono">gpt-4</span> · old.')
    expect(render({ provider: '', model: 'gpt-4' })).toContain('Đang dùng: <span class="mm-mono">gpt-4</span> · openai. Model này chưa có')
  })

  it('model có trong danh sách: không hiện ghi chú', () => {
    expect(render({ provider: 'openai', model: 'gpt-5' })).not.toContain('chưa có trong danh sách')
  })

  it('mức suy luận đủ 8 nhãn theo model, có “Tự động” và “Tối thiểu”', () => {
    state.config = { provider: 'openai', model: 'gpt-5', providers: { openai: { models: [{ name: 'gpt-5' }] } }, roles: { writer: { provider: 'openai', model: 'gpt-5', reasoning_effort: 'high' } } }
    state.catalog = { openai: [{ name: 'gpt-5', thinking: ['off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'] }] }
    const html = renderToStaticMarkup(<Models active="writer" onPick={vi.fn()} disabled={false} toast={vi.fn()} />)
    for (const label of ['Tự động', 'Tắt', 'Tối thiểu', 'Thấp', 'Vừa', 'Cao', 'Rất cao', 'Tối đa']) expect(html).toContain(label)
    expect(html).toContain('7 mức của gpt-5')
  })
})
