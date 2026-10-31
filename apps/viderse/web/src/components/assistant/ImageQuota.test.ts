import { describe, expect, it } from 'vitest'
import type { AiProvider } from '../../api/ai'
import { imageQuotaOf } from './ImageQuota'

const provider = (kind: AiProvider['kind'], quota?: AiProvider['quota']): AiProvider => ({
  id: kind, kind, auth: 'subscription', label: kind === 'chatgpt' ? 'ChatGPT' : 'Grok', status: 'ready', quota, catalogCount: 0, selected: [], dailyPaidImageLimit: 0,
})

describe('imageQuotaOf', () => {
  it('ưu tiên ChatGPT theo ngày, đầy hạn mức thì đánh dấu', () => {
    expect(imageQuotaOf([provider('grok', { used: 210, limit: 400, period: 'week', resetAt: '' }), provider('chatgpt', { used: 50, limit: 50, period: 'day', resetAt: '' })]))
      .toEqual({ text: 'Ảnh hôm nay 50/50 · ChatGPT', full: true })
  })
  it('chỉ có Grok thì ghi theo tuần; không có quota thì ẩn', () => {
    expect(imageQuotaOf([provider('grok', { used: 210, limit: 400, period: 'week', resetAt: '' })])?.text).toBe('Ảnh: Grok 210/400 tuần này')
    expect(imageQuotaOf([provider('chatgpt')])).toBeNull()
    expect(imageQuotaOf(undefined)).toBeNull()
  })
})
