import { describe, expect, it } from 'vitest'
import type { Config, ProviderModel } from '../../../api/types'
import {
  draftItem, fold, itemOf, lockMessage, manualResult, modelRefsOf, noneText, pickRows, priceText,
  providerGroups, searchPlaceholder, selectedLine, shouldWriteContext, sourceLine, usageOf, type PickItem,
} from './modelPick'

const item = (name: string, extra: Partial<PickItem> = {}): PickItem => ({
  name, subtitle: '', contextWindow: 131072, recommended: false, contextSource: 'provider', registryWindow: 0, ...extra,
})

const CATALOG: PickItem[] = [
  item('qwen/qwen3-235b', { recommended: true, subtitle: 'Viết tiếng Việt tốt' }),
  item('deepseek/deepseek-r1', { recommended: true }),
  item('google/gemini-2.5-pro', { recommended: true }),
  item('moonshotai/kimi-k2'),
  item('meta-llama/llama-4'),
]

describe('bảng chọn model', () => {
  it('fold: bỏ dấu tiếng Việt và hạ chữ', () => {
    expect(fold('Ngữ Cảnh Đắt')).toBe('ngu canh dat')
    expect(fold('Đường')).toBe('duong')
  })

  it('pickRows: tìm không dấu theo tên hoặc mô tả; đã chọn lên đầu', () => {
    expect(pickRows(CATALOG, 'qwen', 'all', []).map((r) => r.name)).toEqual(['qwen/qwen3-235b'])
    expect(pickRows(CATALOG, 'tieng viet', 'all', []).map((r) => r.name)).toEqual(['qwen/qwen3-235b'])
    expect(pickRows(CATALOG, 'qwen3 235', 'all', []).map((r) => r.name)).toEqual(['qwen/qwen3-235b'])
    const rows = pickRows(CATALOG, '', 'all', ['moonshotai/kimi-k2', 'qwen/qwen3-235b'])
    expect(rows.slice(0, 2).map((r) => r.name)).toEqual(['qwen/qwen3-235b', 'moonshotai/kimi-k2'])
    expect(rows).toHaveLength(5)
  })

  it('pickRows: lọc Đã chọn / Gợi ý (gợi ý gồm cả model đã chọn)', () => {
    expect(pickRows(CATALOG, '', 'sel', ['moonshotai/kimi-k2']).map((r) => r.name)).toEqual(['moonshotai/kimi-k2'])
    expect(pickRows(CATALOG, '', 'sug', ['moonshotai/kimi-k2']).map((r) => r.name))
      .toEqual(['moonshotai/kimi-k2', 'qwen/qwen3-235b', 'deepseek/deepseek-r1', 'google/gemini-2.5-pro'])
    expect(pickRows(CATALOG, 'không-có', 'all', [])).toEqual([])
  })

  it('modelRefsOf/usageOf: Mặc định · vai · dự phòng; vai rỗng thừa kế cấp trên', () => {
    const config: Config = {
      provider: 'openrouter', model: 'qwen/qwen3-235b',
      roles: {
        writer: { provider: 'openrouter', model: 'deepseek/deepseek-r1' },
        editor: { provider: '', model: '', fallbacks: [{ provider: 'openrouter', model: 'qwen/qwen3-235b' }] },
      },
    }

    expect(usageOf(config, 'openrouter', 'qwen/qwen3-235b')).toEqual(['Mặc định', 'Biên tập viên', 'Biên tập viên (dự phòng)'])
    expect(usageOf(config, 'openrouter', 'deepseek/deepseek-r1')).toEqual(['Người viết'])
    expect(usageOf(config, 'openrouter', 'moonshotai/kimi-k2')).toEqual([])
    // vai thiếu provider/model → kế thừa Mặc định
    const inherit: Config = { provider: 'p1', model: 'm1', roles: { writer: { provider: '', model: '' } } }
    expect(usageOf(inherit, 'p1', 'm1')).toEqual(['Mặc định', 'Người viết'])
    // vai kỹ thuật giữ nguyên key
    expect(modelRefsOf({ provider: 'p', model: 'm', roles: { import_segment: { provider: 'p', model: 'm' } } }).map((r) => r.label))
      .toEqual(['Mặc định', 'import_segment'])
  })

  it('providerGroups: nhóm theo nhà cung cấp; nhóm rỗng khi chưa chọn model', () => {
    const config: Config = {
      provider: 'openrouter', model: 'qwen/a',
      providers: {
        openrouter: { models: [{ name: 'qwen/a' }, { name: 'deepseek/b' }, { name: '' }] },
        anthropic: { models: [] },
      },
      roles: { writer: { provider: 'gemini', model: 'gemini-2.5-pro' } },
    }
    expect(providerGroups(config)).toEqual([
      { provider: 'anthropic', models: [] },
      { provider: 'gemini', models: [] },
      { provider: 'openrouter', models: [{ provider: 'openrouter', model: 'qwen/a' }, { provider: 'openrouter', model: 'deepseek/b' }] },
    ])
  })

  it('lockMessage: câu khoá bỏ chọn', () => {
    expect(lockMessage(['Người viết'])).toBe('Đang dùng cho Người viết. Đổi model của trợ lý đó trước khi bỏ chọn.')
    expect(lockMessage(['Mặc định', 'Người viết'])).toBe('Đang dùng cho Mặc định, Người viết. Đổi model của trợ lý đó trước khi bỏ chọn.')
  })

  it('shouldWriteContext: chỉ ghi context_window khi provider báo và danh mục không biết', () => {
    expect(shouldWriteContext({ contextSource: 'provider', registryWindow: 0 })).toBe(true)
    expect(shouldWriteContext({ contextSource: 'provider', registryWindow: 131072 })).toBe(false)
    expect(shouldWriteContext({ contextSource: 'registry', registryWindow: 0 })).toBe(false)
    expect(shouldWriteContext({ contextSource: 'default', registryWindow: 0 })).toBe(false)
    expect(shouldWriteContext({ contextSource: 'model_config', registryWindow: 0 })).toBe(false)
  })

  it('itemOf/draftItem: context_window đã khai ghi đè hiển thị; model thêm tay không có ngữ cảnh', () => {
    const listed: ProviderModel = { name: 'a/b', displayName: 'A B', contextWindow: 131072, contextSource: 'provider', registryWindow: 0, recommended: false }
    expect(itemOf(listed).contextWindow).toBe(131072)
    expect(itemOf(listed, 64000).contextWindow).toBe(64000)
    expect(draftItem('moi-model', 64000)).toMatchObject({ name: 'moi-model', contextWindow: 64000, recommended: false })
    expect(draftItem('moi-model').contextWindow).toBe(0)
  })

  it('priceText: khoá API hiện vào/ra; gói thuê bao → theo gói; Ollama → miễn phí; không giá → —', () => {
    expect(priceText({ input: 0.13, output: 0.6 }, 'key')).toBe('$0,13 vào · $0,6 ra')
    expect(priceText({ input: 2, output: 15 }, 'key')).toBe('$2 vào · $15 ra')
    expect(priceText(undefined, 'subscription')).toBe('theo gói')
    expect(priceText(undefined, 'local')).toBe('miễn phí')
    expect(priceText(undefined, 'key')).toBe('—')
  })

  it('noneText/searchPlaceholder/selectedLine: câu trống và đếm', () => {
    expect(noneText('abc', 'all')).toBe('Không có model nào khớp “abc”. Thử tên khác, hoặc thêm theo tên.')
    expect(noneText('', 'sel')).toBe('Chưa chọn model nào.')
    expect(noneText('', 'sug')).toBe('Không có model gợi ý.')
    expect(searchPlaceholder(312)).toBe('Tìm trong 312 model')
    expect(searchPlaceholder(4)).toBe('Tìm model')
    expect(selectedLine(0)).toBe('Chưa chọn model nào.')
    expect(selectedLine(2)).toContain('Đã chọn 2 model.')
  })

  it('sourceLine: mốc lấy + đếm; > 50 hàng → “đang hiện n trong m”', () => {
    const at = '2026-10-28T10:24:00Z'
    const hour = new Date(at).getHours()
    const base = { from: 'OpenRouter', fetchedAt: at, hasPrice: false }
    expect(sourceLine({ ...base, shown: 9, filtered: 312, total: 312 }))
      .toBe(`Lấy từ OpenRouter lúc ${String(hour).padStart(2, '0')}:24 · đang hiện 9 trong 312, gõ tên để tìm`)
    expect(sourceLine({ ...base, shown: 4, filtered: 4, total: 4 })).toContain('· 4 model')
    expect(sourceLine({ ...base, shown: 4, filtered: 4, total: 4, hasPrice: true })).toContain('giá tính trên 1 triệu token')
  })

  it('manualResult: ok kèm ngữ cảnh tự nhận; default → cảnh báo mức dự phòng 200K', () => {
    const fmt = (n: number) => `${Math.round(n / 1000)}K`
    expect(manualResult('deepseek/deepseek-v3.2', 640, 163840, 'provider', fmt))
      .toEqual({ kind: 'ok', text: 'deepseek/deepseek-v3.2 trả lời được · 640 ms · ngữ cảnh 164K (tự nhận). Đã thêm và chọn.' })
    expect(manualResult('abc', 500, 0, 'model_config', fmt).text).toContain('(đã khai)')
    const warn = manualResult('xa-la/model', 640, 200000, 'default', fmt)
    expect(warn.kind).toBe('warn')
    expect(warn.text).toContain('mức dự phòng 200K')
    expect(warn.text).toContain('Trợ lý AI')
  })
})
