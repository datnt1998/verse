import { describe, expect, it } from 'vitest'
import type { Config } from '../../../api/types'
import {
  configForCompare, configForSave, configsEqual, deletePath, getPath, mergeConfig, modelOptions, oauthAccountName, oauthPlanLabel,
  providerTypeOptions, pruneEmpty, reasoningSegments, rebaseConfig, rolesUsing, setPath, sourceOf, stableStringify, testModelFor,
} from './config'

const base: Config = {
  provider: 'codex', model: 'gpt-5.4',
  providers: {
    codex: { type: 'openai', api: 'responses', base_url: 'https://api.openai.com/v1', models: [{ name: 'gpt-5.4', context_window: 320000 }], api_key_set: true, api_key_hint: 'sk-…x4f2' },
  },
  roles: { default: { provider: 'codex', model: 'gpt-5.4' }, writer: { provider: 'codex', model: 'gpt-5.4', fallbacks: [{ provider: 'anthropic', model: 'claude' }] } },
  budget: { book_usd: 50, warn_ratio: 0.8, hard_stop: true },
}

describe('getPath / setPath / deletePath', () => {
  it('đọc đường dẫn lồng nhau', () => {
    expect(getPath(base, 'roles.writer.model')).toBe('gpt-5.4')
    expect(getPath(base, 'budget.book_usd')).toBe(50)
    expect(getPath(base, 'providers.codex.api_key_set')).toBe(true)
    expect(getPath(base, 'roles.editor.model')).toBeUndefined()
  })

  it('ghi không sửa bản gốc và tạo nhánh thiếu', () => {
    const next = setPath(base, 'roles.editor.model', 'o4-mini')
    expect(getPath(next, 'roles.editor.model')).toBe('o4-mini')
    expect(getPath(base, 'roles.editor.model')).toBeUndefined()
    expect(getPath(next, 'roles.writer.model')).toBe('gpt-5.4')
  })

  it('xoá khoá = undefined, dọn nhánh rỗng khi prune', () => {
    const next = setPath(base, 'roles.editor.model', 'o4-mini')
    const removed = deletePath(next, 'roles.editor.model')
    expect(getPath(removed, 'roles.editor.model')).toBeUndefined()
    expect(stableStringify(removed)).toBe(stableStringify(base))
  })
})

describe('pruneEmpty / stableStringify', () => {
  it('bỏ object rỗng, giữ mảng rỗng và chuỗi rỗng', () => {
    expect(pruneEmpty({ a: {}, b: { c: {} }, d: [], e: '', f: [{ g: {} }] })).toEqual({ d: [], e: '', f: [{}] })
  })

  it('ổn định theo thứ tự khoá', () => {
    expect(stableStringify({ b: 1, a: { d: 2, c: 3 } })).toBe(stableStringify({ a: { c: 3, d: 2 }, b: 1 }))
  })
})

describe('configForSave / configForCompare', () => {
  it('bỏ api_key_set, api_key_hint, api_key rỗng', () => {
    const saved = configForSave({ ...base, providers: { codex: { ...base.providers!.codex, api_key: '' } } })
    expect(saved.providers?.codex.api_key_set).toBeUndefined()
    expect(saved.providers?.codex.api_key_hint).toBeUndefined()
    expect(saved.providers?.codex.api_key).toBeUndefined()
  })

  it('giữ api_key mới và clear_api_key để server xử lý', () => {
    const saved = configForSave({ providers: { codex: { api_key: 'sk-new', clear_api_key: true } } })
    expect(saved.providers?.codex.api_key).toBe('sk-new')
    expect(saved.providers?.codex.clear_api_key).toBe(true)
  })

  it('giữ nhà cung cấp chỉ còn khoá đã lưu (không models) để PUT không xoá khoá', () => {
    const saved = configForSave({ provider: 'deepseek', providers: { deepseek: { api_key_set: true, api_key_hint: 'sk-k…0000', models: [] }, old: { api_key_set: true } } })
    expect(saved.providers).toEqual({ deepseek: { models: [] }, old: {} })
    expect(configForSave({ providers: { a: undefined as never } }).providers).toBeUndefined()
  })

  it('so sánh bỏ qua hint nhưng tính cờ xoá khoá là thay đổi', () => {
    expect(configsEqual(base, { ...base })).toBe(true)
    expect(configsEqual(base, { ...base, providers: { codex: { ...base.providers!.codex, api_key_hint: 'sk-…zzzz' } } })).toBe(true)
    expect(configsEqual(base, { ...base, providers: { codex: { ...base.providers!.codex, clear_api_key: true } } })).toBe(false)
    expect(configsEqual(base, { ...base, budget: { ...base.budget, book_usd: 60 } })).toBe(false)
  })

  it('chỉnh rồi khôi phục → không dirty (không còn object rỗng)', () => {
    const edited = setPath(base, 'roles.editor.model', 'o4-mini')
    const restored = deletePath(edited, 'roles.editor.model')
    expect(configForCompare(restored)).toEqual(configForCompare(base))
  })

  it('không viết lại type của provider khi lưu', () => {
    const saved = configForSave({
      provider: 'openrouter',
      providers: { openrouter: { type: 'openrouter', base_url: 'https://openrouter.ai/api/v1', api_key_set: true } },
    })
    expect(saved.providers?.openrouter.type).toBe('openrouter')
    expect(configsEqual(
      { providers: { openrouter: { type: 'openrouter' } } },
      { providers: { openrouter: { type: 'openrouter' } } },
    )).toBe(true)
  })
})

describe('rebaseConfig khi ghi đè sau 409', () => {
  it('chỉ ghi văn phong của tab này, giữ cách đẩy chương của tab kia', () => {
    expect(rebaseConfig(
      { style: 'default', advance_mode: 'auto' },
      { style: 'wuxia', advance_mode: 'auto' },
      { style: 'default', advance_mode: 'review' },
    )).toEqual({ style: 'wuxia', advance_mode: 'review' })
  })

  it('giữ thay đổi từ xa trong cùng nhánh và áp dụng xoá khoá của tab này', () => {
    expect(rebaseConfig(
      { roles: { writer: { provider: 'p', model: 'old' } }, style: 'default' },
      { roles: { writer: { provider: 'p', model: 'new' } } },
      { roles: { writer: { provider: 'remote', model: 'old' } }, style: 'default' },
    )).toEqual({ roles: { writer: { provider: 'remote', model: 'new' } } })
  })
})

describe('providerTypeOptions', () => {
  // type của từng preset trong GET /api/config/presets (preset "custom" không kèm type).
  const presetTypes = ['ollama', 'openrouter', 'openai-codex', 'xai-oauth', 'gemini', 'anthropic', 'deepseek',
    'openai', 'qwen', 'glm', 'grok', 'bedrock', '']

  it('phủ mọi giao thức presets và backend chấp nhận', () => {
    const options = providerTypeOptions('openrouter', presetTypes)
    for (const type of presetTypes.filter(Boolean)) expect(options).toContain(type)
    for (const type of ['openai', 'anthropic', 'gemini', 'openrouter', 'deepseek', 'qwen', 'glm', 'grok', 'bedrock', 'ollama', 'openai-codex', 'xai-oauth']) {
      expect(options).toContain(type)
    }
    expect(options).not.toContain('')
  })

  it('không phụ thuộc presets (máy chủ lỗi tải presets vẫn đủ lựa chọn)', () => {
    const options = providerTypeOptions(undefined)
    for (const type of ['openai', 'anthropic', 'gemini', 'openrouter', 'deepseek', 'qwen', 'glm', 'grok', 'bedrock', 'ollama', 'openai-codex', 'xai-oauth']) {
      expect(options).toContain(type)
    }
  })

  it('giữ type đang lưu kể cả khi không có trong danh sách chuẩn', () => {
    expect(providerTypeOptions('my-proxy')).toContain('my-proxy')
    expect(providerTypeOptions('my-proxy', presetTypes)).toContain('my-proxy')
  })

  it('không lặp giá trị, không đổi type đang lưu', () => {
    const options = providerTypeOptions('openrouter', ['openrouter', 'openai'])
    expect(options.filter((type) => type === 'openrouter')).toHaveLength(1)
    expect(options[options.indexOf('openrouter')]).toBe('openrouter')
    expect(providerTypeOptions(undefined)).not.toContain('')
  })
})

describe('modelOptions / rolesUsing / sourceOf', () => {
  it('liệt kê model theo provider', () => {
    expect(modelOptions(base)).toEqual([{ provider: 'codex', model: 'gpt-5.4' }])
  })

  it('nhãn vai trò đang gán', () => {
    expect(rolesUsing(base, 'codex', 'gpt-5.4')).toEqual(['Mặc định', 'WRITER'])
    expect(rolesUsing(base, 'codex', 'nope')).toEqual([])
  })

  it('ưu tiên sources của server, rồi suy ra từ draft', () => {
    expect(sourceOf('budget.book_usd', { 'budget.book_usd': 'project' }, {}, 'story')).toBe('project')
    expect(sourceOf('budget.book_usd', {}, { budget: { book_usd: 50 } }, 'story')).toBe('story')
    expect(sourceOf('budget.book_usd', {}, {}, 'story')).toBe('global')
  })
})

describe('testModelFor', () => {
  it('provider chưa khai báo model ⇒ không gửi model để máy chủ dùng defaultModel của preset', () => {
    expect(testModelFor(undefined)).toBeUndefined()
    expect(testModelFor([])).toBeUndefined()
    expect(testModelFor([{ name: '' }])).toBeUndefined()
  })

  it('chỉ dùng model của chính provider, không mượn model chính của provider khác', () => {
    expect(testModelFor([{ name: 'claude-sonnet-4' }])).toBe('claude-sonnet-4')
  })
})

describe('oauthPlanLabel', () => {
  it('ChatGPT giữ “ChatGPT <gói>”; xAI không có gói thì chỉ hiện tên dịch vụ', () => {
    expect(oauthPlanLabel({ id: 'openai-codex', label: 'ChatGPT Plus/Pro (Codex)' }, 'Plus')).toBe('ChatGPT Plus')
    expect(oauthPlanLabel({ id: 'xai-oauth', label: 'SuperGrok / X Premium+' }, '')).toBe('SuperGrok / X Premium+')
    expect(oauthPlanLabel({ id: 'xai-oauth', label: 'SuperGrok / X Premium+' }, 'Heavy')).toBe('SuperGrok / X Premium+ Heavy')
  })
})

describe('Claude Pro/Max trong Cài đặt', () => {
  it('nhãn dịch vụ cắt phần mô tả trong ngoặc của preset', () => {
    expect(oauthPlanLabel({ id: 'anthropic-oauth', label: 'Claude Pro/Max (Gói thuê bao Anthropic - đăng nhập OAuth)' }, '')).toBe('Claude Pro/Max')
  })
  it('thẻ tài khoản Claude hiện “email · tổ chức”, provider khác hiện email', () => {
    const account = { email: 'dat@example.com', label: 'dat@example.com · Truyện Việt' }
    expect(oauthAccountName({ id: 'anthropic-oauth' }, account)).toBe('dat@example.com · Truyện Việt')
    expect(oauthAccountName({ id: 'openai-codex' }, account)).toBe('dat@example.com')
    expect(oauthAccountName({ id: 'anthropic-oauth' }, { email: 'a@b.c', label: '' })).toBe('a@b.c')
  })
  it('anthropic-oauth là giao thức hợp lệ trong select', () => {
    expect(providerTypeOptions('')).toContain('anthropic-oauth')
  })
})

describe('reasoningSegments (Settings.dc.html: Tự động / Thấp / Vừa / Cao)', () => {
  const labels = (current: string) => reasoningSegments(current).map((segment) => segment.label)
  it.each(['', 'low', 'medium', 'high'])('giá trị %j nằm trong 4 đoạn của artboard → đúng 4 đoạn', (current) => {
    expect(labels(current)).toEqual(['Tự động', 'Thấp', 'Vừa', 'Cao'])
  })
  it.each([['off', 'Tắt'], ['xhigh', 'Rất cao'], ['max', 'Tối đa']])('giá trị đã lưu %s ngoài 4 mức → thêm đoạn thứ 5 "%s" đang chọn', (current, label) => {
    const segments = reasoningSegments(current)
    expect(segments).toHaveLength(5)
    expect(segments[4]).toEqual({ id: current, label })
  })
  it('đoạn thứ 5 tự biến mất khi người dùng chọn một trong 4 mức', () => {
    expect(reasoningSegments('xhigh')).toHaveLength(5)
    expect(reasoningSegments('high')).toHaveLength(4)
  })
  it('mức lạ vẫn hiện nguyên giá trị thay vì mất', () => {
    expect(labels('ultra')).toEqual(['Tự động', 'Thấp', 'Vừa', 'Cao', 'ultra'])
  })
})

describe('mergeConfig (cấu hình “như sẽ lưu”: effective + draft)', () => {
  it('object gộp sâu, mảng thay thế, khoá undefined bỏ qua', () => {
    const effective: Config = {
      provider: 'p1', model: 'm1',
      providers: { p1: { models: [{ name: 'm1' }] } },
      roles: { writer: { provider: 'p1', model: 'm1', fallbacks: [{ provider: 'p2', model: 'x' }] } },
    }
    const draft: Config = {
      providers: { p1: { models: [{ name: 'm1' }, { name: 'm2' }] }, p2: { models: [] } },
      roles: { writer: { provider: 'p1', model: 'm2' } },
    }
    const merged = mergeConfig(effective, draft)
    expect(merged.provider).toBe('p1')
    expect(merged.providers?.p1?.models).toEqual([{ name: 'm1' }, { name: 'm2' }])
    expect(merged.providers?.p2?.models).toEqual([])
    expect(merged.roles?.writer?.provider).toBe('p1')
    expect(merged.roles?.writer?.model).toBe('m2')
    expect(merged.roles?.writer?.fallbacks).toEqual([{ provider: 'p2', model: 'x' }])
    // không đụng đầu vào
    expect(effective.roles?.writer?.model).toBe('m1')
  })
})
