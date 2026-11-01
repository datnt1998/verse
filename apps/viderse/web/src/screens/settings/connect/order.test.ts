import { describe, expect, it } from 'vitest'
import type { AiProvider } from '../../../api/ai'
import { existingCardFor, orderProviders } from './order'

const card = (id: string, kind: AiProvider['kind'], status: AiProvider['status'] = 'ready'): AiProvider => ({
  id, kind, auth: 'api_key', label: id, catalogCount: 0, selected: [], status, dailyPaidImageLimit: 50,
})

describe('thứ tự thẻ Kết nối AI', () => {
  const list = [card('chatgpt', 'chatgpt'), card('grok', 'grok'), card('gemini', 'gemini', 'auth_failed'), card('ollama', 'ollama'), card('deepseek', 'deepseek', 'unreachable')]

  it('thẻ lỗi lên đầu, còn lại giữ thứ tự thêm (cũ trước)', () => {
    expect(orderProviders(list, null).map((p) => p.id)).toEqual(['gemini', 'deepseek', 'chatgpt', 'grok', 'ollama'])
  })

  it('thẻ vừa thêm đứng trên cả thẻ lỗi; không còn trong danh sách thì bị bỏ qua', () => {
    expect(orderProviders(list, 'ollama').map((p) => p.id)).toEqual(['ollama', 'gemini', 'deepseek', 'chatgpt', 'grok'])
    expect(orderProviders(list, 'gone').map((p) => p.id)).toEqual(['gemini', 'deepseek', 'chatgpt', 'grok', 'ollama'])
  })
})

describe('chọn lại nguồn đã có', () => {
  const list = [card('p-gem', 'gemini', 'auth_failed'), card('p-ol', 'ollama'), card('p-oc', 'openai_compat')]

  it('loại đã có thẻ (kể cả thẻ lỗi) dẫn tới thẻ đó', () => {
    expect(existingCardFor(list, 'gemini')).toBe('p-gem')
    expect(existingCardFor(list, 'openai')).toBeUndefined()
  })

  it('Ollama và Tương thích OpenAI vẫn được thêm thẻ mới', () => {
    expect(existingCardFor(list, 'ollama')).toBeUndefined()
    expect(existingCardFor(list, 'openai_compat')).toBeUndefined()
  })
})
