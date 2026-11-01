// Mô hình hiển thị của ảnh: ghép §26 với trường cũ, nhãn phiên bản, thay đổi so
// với bản trước, nguyên văn gửi máy chủ, giờ đặt lại thật, phần đã sửa.
import { describe, expect, it } from 'vitest'
import { jobImagesSchema, type ImageVersion } from '../../api/images'
import {
  changedParts, diffGroups, diffSummary, providerName, rawParams, rawSentText, resetAtLabel, stageWidthVar, toReviewItems, versionMeta, versionNote,
} from './imageView'

const base = (overrides: Partial<ImageVersion> = {}): ImageVersion => ({
  n: 2, state: 'ready', url: '/v2.jpg', previewUrl: null, width: 1280, height: 720,
  parts: [
    { kind: 'scene', text: 'A guard on a stopped escalator.', source: { type: 'artist_feedback', label: 'Hoạ sĩ sửa theo góp ý' } },
    { kind: 'style', text: 'Flat cinematic.', source: { type: 'image_prompt', label: 'prompt “Đêm xanh rêu”' } },
  ],
  sentText: 'A guard on a stopped escalator.\n\nFlat cinematic.',
  refs: [],
  provider: 'chatgpt', model: 'gpt-image-2', params: { size: '1280x720', quality: 'high' },
  feedback: 'tông lạnh hơn', diffFromPrev: [['A guard', ''], [' on the roof', 'ins'], [' on the stairs', 'del']],
  startedAt: '2026-11-01T16:58:00Z', finishedAt: '2026-11-01T16:58:44Z', durationSec: 44, costImages: 1, error: null,
  ...overrides,
})

describe('nhãn phiên bản', () => {
  it('bản đầu và bản theo góp ý', () => {
    expect(versionNote(base({ n: 1, feedback: null }))).toMatch(/^Bản 1 · Lần đầu · Hoạ sĩ viết từ lời kể · /)
    expect(versionNote(base())).toMatch(/^Bản 2 · Theo góp ý “tông lạnh hơn” · /)
  })

  it('dòng meta đúng thứ tự nhà cung cấp · model · cỡ · giờ vẽ · giờ · số ảnh', () => {
    expect(versionMeta(base())).toMatch(/^ChatGPT · gpt-image-2 · 1280×720 · vẽ 44 giây · \d\d:\d\d · tốn 1 ảnh$/)
    expect(versionMeta(base({ state: 'drawing', durationSec: null, finishedAt: null, costImages: 0 }))).toBe('ChatGPT · gpt-image-2 · 1280×720')
  })

  it('tên nhà cung cấp hiển thị, id lạ giữ nguyên', () => {
    expect(providerName('grok')).toBe('Grok')
    expect(providerName('my-provider')).toBe('my-provider')
  })

  it('id thẻ Kết nối AI hiện nhãn thẻ thay vì id nội bộ', () => {
    const cards = [{ id: 'openai_compat-61ed93e9', label: 'Tương thích OpenAI' }]
    expect(providerName('openai_compat-61ed93e9', cards)).toBe('Tương thích OpenAI')
    expect(versionMeta(base({ provider: 'openai_compat-61ed93e9' }), cards)).toContain('Tương thích OpenAI')
    expect(versionMeta(base({ provider: 'openai_compat-61ed93e9' }), cards)).not.toContain('openai_compat-61ed93e9')
  })
})

describe('thay đổi so với bản trước', () => {
  it('mỗi cụm thêm/bỏ liền nhau là một chỗ', () => {
    expect(diffGroups([['a', ''], ['b', 'ins'], ['c', 'del'], ['d', '']])).toBe(1)
    expect(diffGroups([['a', ''], ['b', 'ins'], ['c', ''], ['d', 'del']])).toBe(2)
    expect(diffGroups(null)).toBe(0)
  })

  it('câu tóm tắt theo góp ý và không góp ý', () => {
    expect(diffSummary(base())).toBe('So với bản 1: Hoạ sĩ đổi 1 chỗ theo góp ý của bạn.')
    expect(diffSummary(base({ feedback: null }))).toBe('So với bản 1: Hoạ sĩ đổi 1 chỗ.')
    expect(diffSummary(base({ diffFromPrev: null }))).toBeNull()
  })
})

describe('nguyên văn và khung', () => {
  it('nguyên văn là đúng sentText nhà cung cấp nhận, không thêm gì', () => {
    expect(rawSentText(base())).toBe('A guard on a stopped escalator.\n\nFlat cinematic.')
    expect(rawParams(base())).toBe('size: 1280x720 · quality: high · references: 0')
    expect(rawSentText(undefined)).toBe('')
  })

  it('bề rộng gốc làm trần khung, thiếu số thì 1280', () => {
    expect(stageWidthVar(base())).toBe('1280px')
    expect(stageWidthVar(base({ width: 0 }))).toBe('1280px')
    expect(stageWidthVar(base({ width: 720 }))).toBe('720px')
  })

  it('giờ đặt lại thật, không có thì 00:00', () => {
    const expected = new Date('2026-11-02T00:30:00Z').toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    expect(resetAtLabel('2026-11-02T00:30:00Z')).toBe(`đặt lại lúc ${expected}`)
    expect(resetAtLabel(undefined)).toBe('đặt lại lúc 00:00')
  })
})

describe('phần đã sửa', () => {
  it('chỉ trả phần khác bản đang dùng', () => {
    const version = base()
    expect(changedParts(version, { scene: 'A guard on a stopped escalator.' })).toEqual({})
    expect(changedParts(version, { scene: 'A guard on the roof.', style: 'Flat cinematic.' })).toEqual({ scene: 'A guard on the roof.' })
    expect(changedParts(undefined, { avoid: 'blurry' })).toEqual({ avoid: 'blurry' })
  })
})

describe('ghép §26 với trường cũ', () => {
  it('giữ lời kể, khung giờ, trạng thái duyệt của danh sách cũ', () => {
    const parsed = jobImagesSchema.parse({
      images: [{ id: 'a', kind: 'scene', index: 3, label: 'Cảnh 3', usedVersion: 2, refLimit: 4, versions: [base()] }],
    })
    const items = toReviewItems(
      [{ id: 'a', kind: 'scene', status: 'approved', prompt: 'p', currentVersion: 2, reviewVersion: 2, versions: [], sceneId: '3', narration: 'Lời kể.', startSec: 745, endSec: 1120 }],
      parsed.images,
      {},
    )
    expect(items).toHaveLength(1)
    expect(items[0]).toMatchObject({ label: 'Cảnh 3', index: 3, approved: true, narration: 'Lời kể.', startSec: 745, usedVersion: 2, refLimit: 4 })
  })

  it('chưa có §26 thì đọc bản đang xem từ dữ liệu cũ', () => {
    const items = toReviewItems(
      [{ id: 'b', kind: 'scene', status: 'review', prompt: 'prompt cũ', currentVersion: 2, reviewVersion: 2, versions: [{ version: 2, cutout: null, originalUrl: '/old.jpg' }], sceneId: '5', width: 1024, height: 576 }],
      undefined,
      {},
    )
    expect(items[0].versions).toHaveLength(1)
    expect(items[0].versions[0]).toMatchObject({ n: 2, url: '/old.jpg', sentText: 'prompt cũ', width: 1024 })
    expect(items[0].label).toBe('Cảnh 5')
  })
})
