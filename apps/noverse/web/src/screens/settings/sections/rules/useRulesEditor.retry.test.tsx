// @vitest-environment happy-dom
// Settings.dc.html › rules=retry: lỗi 500 khi đọc quy tắc → thử lại lần 2/3 ở 0/10/20 giây,
// hết lượt thì lưu nguyên văn; lỗi 4xx vẫn hạ cấp ngay; “Huỷ” không để lại hẹn giờ.
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as clientModule from '../../../../api/client'
import { ApiError } from '../../../../api/client'
import * as dataModule from '../../../../data'
import type { RulesEffective } from '../../../../api/types'

const api = vi.hoisted(() => ({
  understandRule: vi.fn(), putRuleFile: vi.fn(), applyRules: vi.fn(), keepRuntimeRule: vi.fn(), dropRuntimeRule: vi.fn(),
  getRulesEffective: vi.fn(), getRuleViolations: vi.fn(),
}))
vi.mock('../../../../api/client', async (original) => ({ ...(await original<typeof clientModule>()), ...api, canCreateStory: () => true, canWriteStory: () => true }))
vi.mock('../../../../api/session', () => ({ useSession: () => ({ user: { username: 'dat', name: 'Dat', role: 'admin', projects: {} } }) }))
const effective: { current: RulesEffective } = { current: { snapshot: null, items: [], runtime: [] } }
vi.mock('../../../../data', async (original) => ({
  ...(await original<typeof dataModule>()),
  invalidate: vi.fn(async () => {}),
  useResource: (key: readonly unknown[]) => ({ data: key.includes('violations') ? { chapters: [] } : effective.current, error: null, status: 'ready', refreshing: false, refresh: vi.fn() }),
}))

import { useRulesEditor, type RulesEditor } from './useRulesEditor'
import { RULES_RETRY_INTERVAL_MS, RAW_SAVED_CHIP } from './logic'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let editor: RulesEditor
function Probe() {
  editor = useRulesEditor({ scope: 'story', project: 'p', story: 's' })
  return null
}
async function mount() {
  const root = createRoot(document.createElement('div'))
  await act(async () => { root.render(<Probe />) })
  return root
}

const busy = () => new ApiError(500, 'Không phân tích được quy tắc')

beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  api.applyRules.mockResolvedValue({ snapshot: {}, effectiveFrom: 26 })
  api.keepRuntimeRule.mockResolvedValue({ ok: true })
  api.dropRuntimeRule.mockResolvedValue({ ok: true })
  effective.current = { snapshot: null, items: [], runtime: [] }
})
afterEach(() => { vi.useRealTimers() })

describe('máy chủ bận khi đọc quy tắc (rules=retry)', () => {
  it('500 → thẻ chờ “lần 2/3”, thử lại ở 10s và 20s, hết lượt thì lưu nguyên văn', async () => {
    api.understandRule.mockRejectedValue(busy())
    await mount()
    await act(async () => { await editor.add('Không dùng từ thở dài quá 2 lần') })
    expect(editor.retry).toEqual({ text: 'Không dùng từ thở dài quá 2 lần.', attempt: 2, total: 3 })
    expect(editor.lists.story).toHaveLength(0)
    expect(api.understandRule).toHaveBeenCalledTimes(1)

    await act(async () => { await vi.advanceTimersByTimeAsync(RULES_RETRY_INTERVAL_MS) })
    expect(api.understandRule).toHaveBeenCalledTimes(2)
    expect(editor.retry).toMatchObject({ attempt: 3 })
    expect(editor.lists.story).toHaveLength(0)

    await act(async () => { await vi.advanceTimersByTimeAsync(RULES_RETRY_INTERVAL_MS) })
    expect(api.understandRule).toHaveBeenCalledTimes(3)
    expect(editor.retry).toBeNull()
    expect(editor.lists.story[0]).toMatchObject({ text: 'Không dùng từ thở dài quá 2 lần.', chips: [RAW_SAVED_CHIP], status: 'ready' })

    // không còn hẹn giờ nào chạy tiếp
    await act(async () => { await vi.advanceTimersByTimeAsync(RULES_RETRY_INTERVAL_MS * 3) })
    expect(api.understandRule).toHaveBeenCalledTimes(3)
  })

  it('thử lại lần 2 thành công → thẻ thường, không lưu nguyên văn', async () => {
    api.understandRule.mockRejectedValueOnce(busy()).mockResolvedValueOnce({ chips: [{ kind: 'max', text: 'thở dài', limit: 2 }], uncertain: [], status: 'ready' })
    await mount()
    await act(async () => { await editor.add('Không dùng từ thở dài quá 2 lần') })
    expect(editor.retry).not.toBeNull()
    await act(async () => { await vi.advanceTimersByTimeAsync(RULES_RETRY_INTERVAL_MS) })
    expect(editor.retry).toBeNull()
    expect(editor.lists.story[0]).toMatchObject({ status: 'ready', chips: [{ kind: 'max', limit: 2 }] })
  })

  it('lỗi 4xx (chưa có model) hạ cấp ngay, không chờ thử lại', async () => {
    api.understandRule.mockRejectedValue(new ApiError(409, 'Chưa có model để đọc quy tắc'))
    await mount()
    let error: string | undefined
    await act(async () => { error = await editor.add('Kể ngôi thứ nhất') })
    expect(error).toBe('Chưa có model để đọc quy tắc')
    expect(editor.retry).toBeNull()
    expect(editor.lists.story[0]).toMatchObject({ status: 'degraded' })
  })

  it('“Huỷ” xoá hẹn giờ và không thêm thẻ nào', async () => {
    api.understandRule.mockRejectedValue(busy())
    await mount()
    await act(async () => { await editor.add('Không dùng từ thở dài quá 2 lần') })
    await act(async () => { editor.cancelRetry() })
    expect(editor.retry).toBeNull()
    await act(async () => { await vi.advanceTimersByTimeAsync(RULES_RETRY_INTERVAL_MS * 3) })
    expect(api.understandRule).toHaveBeenCalledTimes(1)
    expect(editor.lists.story).toHaveLength(0)
  })

  it('“Lưu nguyên văn ngay” lưu ngay, không chờ lượt thử còn lại', async () => {
    api.understandRule.mockRejectedValue(busy())
    await mount()
    await act(async () => { await editor.add('Không dùng từ thở dài quá 2 lần') })
    await act(async () => { editor.saveRaw() })
    expect(editor.retry).toBeNull()
    expect(editor.lists.story[0]).toMatchObject({ chips: [RAW_SAVED_CHIP] })
    await act(async () => { await vi.advanceTimersByTimeAsync(RULES_RETRY_INTERVAL_MS * 3) })
    expect(api.understandRule).toHaveBeenCalledTimes(1)
  })
})
