import { describe, expect, it } from 'vitest'
import { AVATAR_GRADIENTS, avatarOf, keyMask, providerName, subLine, testedLabel } from './connectLogic'

describe('Kết nối AI', () => {
  it('avatarOf: chữ cái đầu viết hoa, gradient quay vòng theo vị trí thẻ', () => {
    expect(avatarOf('deepseek', 2)).toEqual({ ini: 'D', bg: AVATAR_GRADIENTS[2] })
    expect(avatarOf('Đại Việt', 0).ini).toBe('Đ')
    expect(avatarOf('x', 3).bg).toBe(AVATAR_GRADIENTS[0])
    expect(avatarOf('', 1).ini).toBe('?')
  })

  it('providerName: bỏ phần trong ngoặc, ChatGPT cho openai-codex, không preset thì dùng id', () => {
    expect(providerName('openrouter', { id: 'openrouter', label: 'OpenRouter (Claude, Gemini, DeepSeek, Qwen...)' })).toBe('OpenRouter')
    expect(providerName('openai-codex', { id: 'openai-codex', label: 'ChatGPT Plus/Pro (Codex Subscription)' })).toBe('ChatGPT')
    expect(providerName('provider-1', null)).toBe('provider-1')
  })

  it('keyMask: lấy 4 ký tự cuối sau dấu … của gợi ý máy chủ', () => {
    expect(keyMask('sk-o…7890')).toBe('••••7890')
    expect(keyMask('…')).toBe('••••')
    expect(keyMask('oauth')).toBe('••••')
    expect(keyMask(undefined)).toBe('••••')
  })

  it('subLine: đã đăng nhập (xanh), khoá API, chưa có khoá, chạy trên máy này; 0 model → cảnh báo vàng', () => {
    expect(subLine({ oauth: true, account: { name: 'dat@gmail.com', plan: 'Plus' }, keySet: true, local: false, models: 3 }))
      .toEqual({ text: 'Đã đăng nhập · dat@gmail.com · gói Plus · 3 model đã chọn', tone: 'ok' })
    expect(subLine({ oauth: true, account: { name: 'a@b.c', plan: '' }, keySet: true, local: false, models: 1 }).text)
      .toBe('Đã đăng nhập · a@b.c · 1 model đã chọn')
    expect(subLine({ oauth: true, account: null, keySet: false, local: false, models: 0 }))
      .toEqual({ text: 'Chưa đăng nhập · chưa chọn model, trợ lý chưa dùng được', tone: 'warn' })
    expect(subLine({ oauth: false, account: null, keySet: true, keyHint: 'sk-…abcd', local: false, models: 2 }))
      .toEqual({ text: 'Khoá API ••••abcd · 2 model đã chọn', tone: 'mute' })
    expect(subLine({ oauth: false, account: null, keySet: true, keyHint: 'sk-…abcd', local: false, models: 0 }))
      .toEqual({ text: 'Khoá API ••••abcd · chưa chọn model, trợ lý chưa dùng được', tone: 'warn' })
    expect(subLine({ oauth: false, account: null, keySet: false, local: true, models: 1 }).text).toBe('Chạy trên máy này · 1 model đã chọn')
    expect(subLine({ oauth: false, account: null, keySet: false, local: false, models: 0 }).text).toBe('Chưa có khoá API · chưa chọn model, trợ lý chưa dùng được')
  })

  it('testedLabel: độ trễ làm tròn', () => {
    expect(testedLabel(419.6)).toBe('Ổn · 420 ms')
  })
})
