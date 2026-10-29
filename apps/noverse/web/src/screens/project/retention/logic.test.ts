// Bản đồ giữ chân (D5): câu tổng kết cạnh nút chuyển, câu “Muốn …” của thẻ Khung truyện,
// và chip cho biết ai điền khung truyện. Số liệu lấy đúng bộ của artboard.
import { describe, expect, it } from 'vitest'
import type { Frame, Retention } from '../../../api/types'
import { HOOK_LABEL, audienceLabels, audienceWants, frameSourceChip, mapSummary, wantsSentence } from './logic'

const retention: Retention = {
  empty: false,
  promiseAt: 0.44,
  twistAt: 0.5625,
  chapters: [
    { n: 1, title: 'Một', status: 'written', questions: ['Ai?'], promise: '', payoff: null, hook: null },
    { n: 2, title: 'Hai', status: 'unwritten', questions: ['Ai?', 'Gì?'], promise: '', payoff: null, hook: null },
  ],
  checks: [
    { ref: 'Master §3', ok: true, text: '', chapters: [] },
    { ref: 'Master §8', ok: true, text: '', chapters: [] },
    { ref: 'Master §7', ok: false, text: '', chapters: [5, 6, 7] },
    { ref: 'Master §2', ok: false, text: '', chapters: [1] },
  ],
}

describe('mapSummary', () => {
  it('đếm chương và số dòng kiểm lệch, thêm vế vị trí khi §3 và §8 cùng đạt', () => {
    expect(mapSummary(retention)).toBe('2 chương · 2 cảnh báo · đúng vị trí lời hứa và cú lật')
  })

  it('bỏ vế vị trí khi cảnh lời hứa ra ngoài vùng 35–60%', () => {
    const outside = {
      ...retention,
      checks: retention.checks.map((check) => check.ref === 'Master §3' ? { ...check, ok: false } : check),
    }
    expect(mapSummary(outside)).toBe('2 chương · 3 cảnh báo')
  })
})

describe('wantsSentence', () => {
  it('hạ chữ đầu để nối sau “Muốn” như artboard', () => {
    expect(wantsSentence('Được làm người thông minh, sắc sảo, thắng đẹp')).toBe('được làm người thông minh, sắc sảo, thắng đẹp')
  })

  it('bỏ khoảng trắng hai đầu, rỗng thì trả rỗng', () => {
    expect(wantsSentence('  Thương nhân vật  ')).toBe('thương nhân vật')
    expect(wantsSentence('   ')).toBe('')
  })
})

describe('frameSourceChip', () => {
  it('gói thể loại điền sẵn — trạng thái artboard vẽ', () => {
    expect(frameSourceChip('pack', 'Nữ tần trọng sinh báo thù, vả mặt')).toBe('Gói Nữ tần trọng sinh báo thù, vả mặt điền sẵn')
  })

  it('gói chưa rõ tên và hai nguồn còn lại', () => {
    expect(frameSourceChip('pack', null)).toBe('Gói thể loại điền sẵn')
    expect(frameSourceChip('architect', null)).toBe('Kiến trúc sư điền sẵn')
    expect(frameSourceChip('user', 'Nữ tần trọng sinh báo thù, vả mặt')).toBe('Bạn đã sửa')
  })
})

/** Bảng Master §1.2 của gói (5 nhóm) — cùng thứ tự với `audiences[]`. */
const frame: Pick<Frame, 'audience' | 'audiences'> = {
  audience: [2, 1],
  audiences: [
    { n: 0, label: 'Phụ nữ trung niên', wants: 'Thương nhân vật, rồi hả dạ khi kẻ ác bị quả báo' },
    { n: 1, label: 'Nữ 18–35', wants: 'Được làm người thông minh, sắc sảo, thắng đẹp' },
    { n: 2, label: 'Nam trung niên', wants: 'Làm lại đời, gánh vác gia đình, hoài niệm' },
  ],
}

describe('audienceLabels', () => {
  it('nối nhãn các nhóm theo đúng thứ tự mảng — gói C mặc định [2, 1]', () => {
    expect(audienceLabels(frame)).toBe('Nam trung niên · Nữ 18–35')
  })

  it('bỏ chỉ số ngoài bảng của gói, mảng rỗng thì không có nhãn', () => {
    expect(audienceLabels({ ...frame, audience: [9] })).toBe('')
    expect(audienceLabels({ ...frame, audience: [] })).toBe('')
  })

  it('BE cũ gửi null (gói không khai nhóm) → rỗng thay vì lỗi', () => {
    expect(audienceLabels({ audience: null, audiences: null })).toBe('')
    expect(audienceLabels({ audience: [1], audiences: null })).toBe('')
    expect(audienceLabels({ audience: null, audiences: frame.audiences })).toBe('')
  })
})

describe('audienceWants', () => {
  it('nối “họ muốn” của các nhóm đã chọn, hạ chữ đầu từng vế', () => {
    expect(audienceWants(frame)).toBe('làm lại đời, gánh vác gia đình, hoài niệm · được làm người thông minh, sắc sảo, thắng đẹp')
  })

  it('BE cũ gửi null → rỗng thay vì lỗi', () => {
    expect(audienceWants({ audience: null, audiences: null })).toBe('')
    expect(audienceWants({ audience: [1], audiences: null })).toBe('')
  })
})

describe('HOOK_LABEL', () => {
  it('có đúng bốn kiểu Master §7 và seed của chương cuối', () => {
    expect(Object.keys(HOOK_LABEL)).toEqual(['info', 'threat', 'decision', 'line', 'seed'])
    expect(HOOK_LABEL.seed).toBe('Hạt giống phần hai')
  })
})
