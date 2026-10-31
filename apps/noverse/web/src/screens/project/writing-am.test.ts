import { describe, expect, it, vi } from 'vitest'
import type { ConfigResponse } from '../../api/types'
import { resolveStoryTab } from './StoryView'
import { styleSourceText, writerConfig } from '../settings/sections/style'
import { saveQuickConfig } from './applied'
import { putConfig } from '../../api/client'
vi.mock('../../api/client', () => ({ putConfig: vi.fn() }))

describe('Gói AM · phạm vi cách viết', () => {
  it('liên kết quy tắc cũ mở tab gộp, tham số lạ về dàn ý', () => {
    expect(resolveStoryTab('rules')).toBe('style')
    expect(resolveStoryTab('style')).toBe('style')
    expect(resolveStoryTab('missing')).toBe('outline')
  })
  it('chỉ đánh dấu giá trị kế thừa ở đúng cấp', () => {
    expect(styleSourceText('story', 'story')).toBe('')
    expect(styleSourceText('project', 'story')).toBe('theo dự án')
    expect(styleSourceText('global', 'story')).toBe('theo dự án')
    expect(styleSourceText('project', 'project')).toBe('')
    expect(styleSourceText('global', 'project')).toBe('chung mọi truyện')
  })
  it('chọn nhanh giữ cấu hình của cấp đích, không chép giá trị kế thừa', async () => {
    const own = { budget: { book_usd: 12 }, roles: { editor: { provider: 'a', model: 'edit' } } }
    const previous: ConfigResponse = { version: 'v1', target: { scope: 'story', config: own, path: '', exists: true }, effective: { ...own, style: 'romance', providers: { parent: { models: [{ name: 'large' }] } } }, chain: [], sources: { style: 'project' } }
    vi.mocked(putConfig).mockResolvedValue(previous)
    await saveQuickConfig('story', 'p', 's', previous, { style: 'suspense' })
    expect(putConfig).toHaveBeenLastCalledWith('story', { ...own, style: 'suspense' }, 'p', 's', 'v1')
    await saveQuickConfig('project', 'p', undefined, previous, { advance_mode: 'review' })
    expect(putConfig).toHaveBeenLastCalledWith('project', { ...own, advance_mode: 'review' }, 'p', undefined, 'v1')
    vi.mocked(putConfig).mockRejectedValueOnce(new Error('Không lưu được'))
    await expect(saveQuickConfig('story', 'p', 's', previous, { style: 'default' })).rejects.toThrow('Không lưu được')
    expect(previous.target.config).toEqual(own)
  })
  it('Người viết đổi đúng role writer và quay về kế thừa không chạm trợ lý khác', () => {
    const editor = { provider: 'a', model: 'edit' }
    const original = { style: 'romance', roles: { default: editor, editor, writer: { provider: 'b', model: 'old', reasoning_effort: 'high' } } }
    const next = writerConfig(original, { provider: 'c', model: 'new' })
    expect(next.roles).toEqual({ default: editor, editor, writer: { provider: 'c', model: 'new', reasoning_effort: 'high' } })
    expect(writerConfig(next, null)).toEqual({ style: 'romance', roles: { default: editor, editor } })
    expect(original.roles.writer.model).toBe('old')
  })
})
