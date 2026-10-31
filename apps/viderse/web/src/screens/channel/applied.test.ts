// Test thuần cho cột phải “Đang áp dụng cho kênh”: nhãn “riêng” chỉ khi khác
// mặc định kênh mới, giá trị đọc từ dữ liệu kênh thật, mỗi dòng mở đúng tab.
import { describe, expect, it } from 'vitest'
import type { Channel } from '../../api/channels'
import { appliedRows, isDefaultTemplateSet } from './applied'

const base = (patch: Partial<Channel> = {}): Channel => ({
  id: 'c1', name: 'Chuyện Lạ', youtubeUrl: null, language: 'vi',
  styleNote: null, descriptionFooter: null, alwaysTags: [],
  autoPick: true, pacePerDay: 5, templateCursor: 0,
  autonomy: { preset: 'balanced', pick: 'all', approve: { mode: 'ask', min: 85 }, onError: 'retry_once', belowQuality: 'ask' }, priority: 'steady',
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' },
  avatarUrl: null, bannerUrl: null,
  sources: [{ rivalId: 'r1', title: 'Một Phim', handle: '@motphim', linkedAt: '2026-10-20T00:00:00Z', reference: true }],
  templates: [{ id: 't1', name: 'Mẫu chuẩn · Chuyện Lạ', position: 0 }],
  identity: { state: 'ready' },
  createdAt: '2026-10-20T00:00:00Z', updatedAt: '2026-10-20T00:00:00Z',
  ...patch,
})

const row = (channel: Channel, key: string) => {
  const found = appliedRows(channel).find((item) => item.key === key)
  if (!found) throw new Error(`thiếu dòng ${key}`)
  return found
}

describe('Đang áp dụng cho kênh', () => {
  it('kênh mới mặc định: không dòng nào “riêng”, đúng nhãn mặc định', () => {
    const channel = base()
    const rows = appliedRows(channel)
    expect(rows.map((item) => item.key)).toEqual(['auto', 'templates', 'titles', 'ai'])
    expect(rows.every((item) => !item.own)).toBe(true)
    expect(row(channel, 'auto').value).toBe('Bật · Nhịp 5/ngày')
    expect(row(channel, 'templates').value).toContain('tiếp theo: Mẫu chuẩn · Chuyện Lạ')
    expect(row(channel, 'titles').value).toBe('Tiếng Việt · mặc định của kênh mới')
  })

  it('nhịp khác 5 hoặc tắt Tự lấy → “riêng”; Tuỳ chỉnh mở tab Nguồn', () => {
    const slower = base({ pacePerDay: 7 })
    expect(row(slower, 'auto').own).toBe(true)
    expect(row(slower, 'auto').value).toBe('Bật · Nhịp 7/ngày')
    expect(row(slower, 'auto').href).toBe('#/channels/c1?tab=sources')

    const off = base({ autoPick: false })
    expect(row(off, 'auto').own).toBe(true)
    expect(row(off, 'auto').value).toBe('Tắt · Nhịp 5/ngày')
  })

  it('Template “riêng” khi không còn đúng bộ Mẫu chuẩn; Tuỳ chỉnh mở tab Template', () => {
    expect(isDefaultTemplateSet(base())).toBe(true)
    const extra = base({
      templates: [
        { id: 't1', name: 'Mẫu chuẩn · Chuyện Lạ', position: 0 },
        { id: 't2', name: 'Phim tài liệu', position: 1 },
      ],
    })
    expect(row(extra, 'templates').own).toBe(true)
    expect(row(extra, 'templates').value).toBe('2 template · tiếp theo: Mẫu chuẩn · Chuyện Lạ')
    expect(row(extra, 'templates').href).toBe('#/channels/c1?tab=templates')

    const renamed = base({ templates: [{ id: 't1', name: 'Khác', position: 0 }] })
    expect(row(renamed, 'templates').own).toBe(true)
  })

  it('Tiêu đề/mô tả/tag “riêng” khi có giọng văn, đoạn cuối hoặc tag', () => {
    const styled = base({ styleNote: 'Kể chậm', descriptionFooter: 'Mỗi đêm', alwaysTags: ['a', 'b', 'c'] })
    const titles = row(styled, 'titles')
    expect(titles.own).toBe(true)
    expect(titles.value).toBe('Tiếng Việt · giọng văn, đoạn cuối, 3 tag luôn thêm')
    expect(titles.href).toBe('#/channels/c1?tab=settings')

    const tagsOnly = base({ alwaysTags: ['a'] })
    expect(row(tagsOnly, 'titles').value).toBe('Tiếng Việt · 1 tag luôn thêm')
  })

  it('Trợ lý AI không bao giờ “riêng” và mở Cài đặt › Trợ lý AI', () => {
    const ai = row(base(), 'ai')
    expect(ai.own).toBe(false)
    expect(ai.label).toContain('chung mọi kênh')
    expect(ai.href).toBe('#/settings?section=agents')
  })
})
