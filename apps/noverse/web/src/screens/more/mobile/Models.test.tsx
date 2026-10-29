import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { ModelsResponse } from '../../../api/types'

const state = vi.hoisted(() => ({ data: undefined as ModelsResponse | undefined }))
vi.mock('../../../data/useResource', () => ({ useResource: () => ({ data: state.data, error: null, status: 'ready', refreshing: false, refresh: vi.fn() }) }))
vi.mock('../../../data/useAction', () => ({ useAction: () => ({ busy: false, pending: false, error: null, run: vi.fn() }) }))
vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: () => null,  AgentOrb: () => null }))
vi.mock('../../../data/modelContext', () => ({ useModelContext: () => ({ data: undefined, error: null, status: 'ready' }), saveModelContexts: vi.fn() }))
import { Models } from './Models'

const catalog = (role: { provider: string; model: string }): ModelsResponse => ({
  roles: [{ role: 'writer', thinking: '', thinkingOptions: [], ...role }],
  providers: [{ provider: 'openai', label: 'OpenAI', ready: true, models: [{ name: 'gpt-5', contextWindow: 200000, contextSource: 'registry', registryWindow: 200000, thinking: [] }] }],
})
const render = (role: { provider: string; model: string }) => {
  state.data = catalog(role)
  return renderToStaticMarkup(<Models project="p" story="s" active="writer" onPick={vi.fn()} disabled={false} toast={vi.fn()} />)
}

describe('M4 Model › vai chưa có model trong danh sách (E2EM-13)', () => {
  it('chưa chọn model: không in “Đang dùng: · .”', () => {
    const html = render({ provider: '', model: '' })
    expect(html).not.toContain('Đang dùng')
    expect(html).toContain('Chưa chọn model cho trợ lý này')
  })

  it('model cũ không còn trong danh sách: nêu tên model và nhà cung cấp, bỏ dấu chấm giữa khi thiếu nhà cung cấp', () => {
    expect(render({ provider: 'old', model: 'gpt-4' })).toContain('Đang dùng: <span class="mm-mono">gpt-4</span> · old.')
    expect(render({ provider: '', model: 'gpt-4' })).toContain('Đang dùng: <span class="mm-mono">gpt-4</span>. Model này chưa có')
  })

  it('model có trong danh sách: không hiện ghi chú', () => {
    expect(render({ provider: 'openai', model: 'gpt-5' })).not.toContain('chưa có trong danh sách')
  })
})
