// @vitest-environment happy-dom
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
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

import { useRulesEditor, useRulesCount, type RulesEditor } from './useRulesEditor'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
let editor: RulesEditor
let shownCount: number | undefined
const roots: Root[] = []
afterEach(async () => { await act(async () => { roots.splice(0).forEach((root) => root.unmount()) }) })
function Probe({ scope }: { scope: 'story' | 'project' | 'global' }) {
  editor = useRulesEditor({ scope, project: 'p', story: 's' })
  shownCount = useRulesCount(scope, scope === 'global' ? undefined : 'p', scope === 'story' ? 's' : undefined)
  return null
}
async function mount(scope: 'story' | 'project' | 'global' = 'story') {
  const root = createRoot(document.createElement('div'))
  roots.push(root)
  await act(async () => { root.render(<Probe scope={scope} />) })
  return root
}

beforeEach(() => {
  vi.clearAllMocks()
  api.applyRules.mockResolvedValue({ snapshot: {}, effectiveFrom: 26 })
  api.keepRuntimeRule.mockResolvedValue({ ok: true })
  api.dropRuntimeRule.mockResolvedValue({ ok: true })
  effective.current = {
    snapshot: null, timing: { effectiveFrom: 26, inProgress: 25, completed: 24 },
    items: [{ text: 'Kể ngôi thứ nhất.', scope: 'story', chips: [{ kind: 'pref', text: 'Ngôi thứ nhất' }], uncertain: false, uncertainNote: '', status: 'ready' }],
    runtime: [
      { id: 'rt-1', text: 'Tri Hạ không khóc.', chapter: 12, createdAt: '2026-10-21T09:00:00Z', chips: [], uncertain: [], status: 'ready' },
      { id: 'rt-2', text: 'Bỏ cái này.', chapter: 13, createdAt: '2026-10-21T09:00:00Z', chips: [], uncertain: [], status: 'ready' },
    ],
  }
})

describe('thêm quy tắc', () => {
  it('chuẩn hoá thành chip; thẻ mới ở đầu danh sách, đánh dấu fresh, tính là 1 thay đổi', async () => {
    api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [{ kind: 'max', text: 'thở dài', limit: 2 }], status: 'ready' })
    await mount()
    await act(async () => { await editor.add('Không dùng từ ‘thở dài’ quá 2 lần mỗi chương') })
    expect(editor.lists.story[0]).toMatchObject({ text: 'Không dùng từ ‘thở dài’ quá 2 lần mỗi chương.', fresh: true, chips: [{ kind: 'max', limit: 2 }] })
    expect(editor.count).toBe(1)
    expect(editor.thinking).toBe('')
  })

  it('normaliser lỗi: thẻ vẫn lưu như “Lời dặn” (degraded), không chặn, báo lý do', async () => {
    api.understandRule.mockRejectedValue(new ApiError(409, 'Chưa có model để đọc quy tắc'))
    await mount()
    let warning: string | undefined
    await act(async () => { warning = await editor.add('Đừng để chương quá dài') })
    expect(warning).toBe('Chưa có model để đọc quy tắc')
    expect(editor.lists.story[0]).toMatchObject({ text: 'Đừng để chương quá dài.', status: 'degraded', chips: [{ kind: 'pref', text: 'Đừng để chương quá dài.' }] })
    expect(editor.count).toBe(1)
  })

  it('đang đọc một quy tắc thì không nhận thêm (không chồng thẻ)', async () => {
    let release: (value: unknown) => void = () => {}
    api.understandRule.mockReturnValue(new Promise((resolve) => { release = resolve }))
    await mount()
    let first: Promise<unknown> = Promise.resolve()
    await act(async () => { first = editor.add('Một.') })
    expect(editor.thinking).toBe('Một.')
    await act(async () => { await editor.add('Hai.') })
    expect(api.understandRule).toHaveBeenCalledTimes(1)
    await act(async () => { release({ structured: {}, preferences: '', uncertain: [], chips: [], status: 'ready' }); await first })
  })
})

describe('khung cam', () => {
  it('chọn cách hiểu: viết lại rồi chuẩn hoá lại; “Giữ như lời dặn”: bỏ khung và không đếm là thay đổi', async () => {
    effective.current.items = [{ text: 'Đừng để chương quá dài.', scope: 'story', chips: [{ kind: 'pref', text: 'Độ dài' }], uncertain: true, uncertainNote: 'quá dài là bao nhiêu?', options: ['Khoảng 3.000 chữ mỗi chương'], status: 'ready' }]
    api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [{ kind: 'pref', text: '~3.000 chữ' }], status: 'ready' })
    await mount()
    const key = editor.lists.story[0].key
    await act(async () => { await editor.resolve('story', key, 'Khoảng 3.000 chữ mỗi chương') })
    expect(api.understandRule).toHaveBeenCalledWith('Khoảng 3.000 chữ mỗi chương', 'story', 'p', 's', expect.anything())
    expect(editor.lists.story[0]).toMatchObject({ text: 'Khoảng 3.000 chữ mỗi chương', uncertain: false, note: '' })
    expect(editor.count).toBe(1)
  })
})

describe('Áp dụng', () => {
  it('ghi quy-tac.md (một dòng mỗi quy tắc), rồi /rules/apply với recheck mặc định bật, trả câu có “từ chương 26”', async () => {
    api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [], status: 'ready' })
    await mount()
    await act(async () => { await editor.add('Giọng văn châm biếm') })
    let toast = ''
    await act(async () => { toast = await editor.apply('truyện “Trà Xanh”') })
    expect(api.putRuleFile).toHaveBeenCalledWith('story', 'quy-tac.md', '- Giọng văn châm biếm.\n- Kể ngôi thứ nhất.\n', 'p', 's')
    expect(api.applyRules).toHaveBeenCalledWith('p', 's', true)
    expect(toast).toBe('Đã áp dụng · từ chương 26 · đang kiểm tra 24 chương cũ')
    expect(editor.count).toBe(0)
  })

  it('bỏ chọn kiểm tra lại → recheck=false và câu không nhắc kiểm tra', async () => {
    api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [], status: 'ready' })
    await mount()
    await act(async () => { editor.setRecheck(false) })
    await act(async () => { await editor.add('Một.') })
    let toast = ''
    await act(async () => { toast = await editor.apply('x') })
    expect(api.applyRules).toHaveBeenCalledWith('p', 's', false)
    expect(toast).toBe('Đã áp dụng · từ chương 26')
  })

  it('Giữ chuyển luật Trọng tài vào tệp truyện rồi gọi keep; Bỏ gọi DELETE (404 coi như đã mất)', async () => {
    await mount()
    const [keep, drop] = editor.runtime
    await act(async () => { editor.keepLearned(keep); editor.dropLearned(drop) })
    expect(editor.runtime).toEqual([])
    expect(editor.lists.story.map((entry) => entry.text)).toContain('Tri Hạ không khóc.')
    api.dropRuntimeRule.mockRejectedValue(new ApiError(404, 'Không tìm thấy'))
    await act(async () => { await editor.apply('x') })
    expect(api.putRuleFile).toHaveBeenCalledWith('story', 'quy-tac.md', expect.stringContaining('- Tri Hạ không khóc.'), 'p', 's')
    expect(api.keepRuntimeRule).toHaveBeenCalledWith('p', 's', 'rt-1')
    expect(api.dropRuntimeRule).toHaveBeenCalledWith('p', 's', 'rt-2')
  })

  it('cấp dự án / toàn cục không gọi /rules/apply và không có luật Trọng tài', async () => {
    api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [], status: 'ready' })
    effective.current = { snapshot: null, items: [] }
    await mount('project')
    await act(async () => { await editor.add('Thể loại showbiz') })
    let toast = ''
    await act(async () => { toast = await editor.apply('các truyện trong dự án “Kênh”') })
    expect(api.putRuleFile).toHaveBeenCalledWith('project', 'quy-tac.md', '- Thể loại showbiz.\n', 'p', undefined)
    expect(api.applyRules).not.toHaveBeenCalled()
    expect(toast).toBe('Đã áp dụng cho các truyện trong dự án “Kênh”')
  })
})

describe('nhóm kế thừa chỉ xem', () => {
  it('không sửa, xoá, chuyển hay chọn cách hiểu của quy tắc cấp trên', async () => {
    effective.current.items.push({ text: 'Quy tắc dự án.', scope: 'project', chips: [], uncertain: true, uncertainNote: 'Chưa rõ', options: ['Rõ hơn.'], status: 'ready' })
    await mount()
    const key = editor.lists.project[0].key
    await act(async () => {
      editor.edit('project', key)
      editor.remove('project', key)
      editor.move('project', key)
      await editor.resolve('project', key, 'Rõ hơn.')
    })
    expect(editor.lists.project.map((entry) => entry.text)).toEqual(['Quy tắc dự án.'])
    expect(editor.draftSeed).toBeNull()
    expect(editor.count).toBe(0)
    expect(api.understandRule).not.toHaveBeenCalled()
  })
})

describe('số quy tắc theo cấp', () => {
  it('chỉ đếm quy tắc riêng và học từ góp ý ở tab truyện', async () => {
    effective.current.items.push({ text: 'Kế thừa.', scope: 'project', chips: [], uncertain: false, uncertainNote: '', status: 'ready' })
    await mount()
    expect(shownCount).toBe(3)
  })
  it('số ở Tuỳ chỉnh cập nhật ngay dù trình sửa giữ ngữ cảnh của truyện đang mở', async () => {
    effective.current = { snapshot: null, items: [{ text: 'Chung.', scope: 'global', chips: [], uncertain: false, uncertainNote: '', status: 'ready' }] }
    api.understandRule.mockResolvedValue({ structured: {}, preferences: '', uncertain: [], chips: [], status: 'ready' })
    await mount('global')
    expect(shownCount).toBe(1)
    await act(async () => { await editor.add('Quy tắc mới.') })
    expect(shownCount).toBe(2)
  })
})
