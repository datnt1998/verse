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
    expect(rows.every((item) => !item.own)).toBe(true)
  })

  it('nhịp khác 5, lấy video khác “Mọi video mới” hoặc ưu tiên lượt xem → “riêng”; Tuỳ chỉnh mở tab Nguồn', () => {
    const slower = base({ pacePerDay: 7 })
    expect(row(slower, 'auto').own).toBe(true)
    expect(row(slower, 'auto').href).toBe('#/channels/c1?tab=sources')

    // Dòng "Lấy video" đọc Mức tự chủ + Ưu tiên kênh (SourcesTab đã thay công tắc Tự lấy cũ).
    const selective = base({ autonomy: { ...base().autonomy, pick: 'suggest' } })
    expect(row(selective, 'auto').own).toBe(true)

    const views = base({ priority: 'views' })
    expect(row(views, 'auto').own).toBe(true)
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
    expect(row(extra, 'templates').href).toBe('#/channels/c1?tab=templates')

    const renamed = base({ templates: [{ id: 't1', name: 'Khác', position: 0 }] })
    expect(row(renamed, 'templates').own).toBe(true)
  })

  it('Tiêu đề/mô tả/tag “riêng” khi có giọng văn, đoạn cuối hoặc tag', () => {
    const styled = base({ styleNote: 'Kể chậm', descriptionFooter: 'Mỗi đêm', alwaysTags: ['a', 'b', 'c'] })
    const titles = row(styled, 'titles')
    expect(titles.own).toBe(true)
    expect(titles.href).toBe('#/channels/c1?tab=settings')

  })

  it('Trợ lý AI không bao giờ “riêng” và mở Cài đặt › Trợ lý AI', () => {
    const ai = row(base(), 'ai')
    expect(ai.own).toBe(false)
    expect(ai.href).toBe('#/settings?section=agents')
  })
})
