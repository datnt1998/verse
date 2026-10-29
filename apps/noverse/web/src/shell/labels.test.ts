// Tên hiển thị: dự án/truyện không bao giờ hiện slug hay tiền tố `01 ·`; header điện thoại không lặp 2 dòng.
import { describe, expect, it } from 'vitest'
import { parseHash } from '../router'
import { documentTitle, humanizeSlug, mobileHeaderCopy, projectLabel, storyLabel } from './labels'

const project = (patch: Partial<{ name: string; legacy: boolean }> = {}) => ({ name: 'Kênh truyện audio', legacy: false, ...patch })

describe('projectLabel', () => {
  it('dự án `_` → "Truyện cũ", không hiện id thô', () => {
    expect(projectLabel(undefined, '_')).toBe('Truyện cũ')
    expect(projectLabel(project({ name: '_', legacy: true }), '_')).toBe('Truyện cũ')
  })

  it('lấy project.json.name, không phải slug', () => {
    expect(projectLabel(project(), 'dit-dang-audio')).toBe('Kênh truyện audio')
    expect(projectLabel(project({ name: '  Kênh truyện audio  ' }), 'dit-dang-audio')).toBe('Kênh truyện audio')
  })

  it('thiếu tên → tên dựng từ slug, không bao giờ trả slug thô; không có dự án → —', () => {
    expect(projectLabel(project({ name: '' }), 'dit-dang-audio')).toBe('Dit Dang Audio')
    expect(projectLabel(undefined, 'chua-tai')).toBe('Chua Tai')
    expect(projectLabel(undefined, 'chua-tai')).not.toMatch(/-/)
    expect(projectLabel(undefined, null)).toBe('—')
  })
})

describe('storyLabel', () => {
  const story = (patch: Partial<{ story: string; title: string; tempName: boolean }> = {}) => ({
    story: '01-tra-xanh-cuop-kich-ban', title: 'Trà Xanh Cướp Kịch Bản', tempName: false, ...patch,
  })

  it('lấy story.json.title, không có tiền tố số hay slug', () => {
    const label = storyLabel(story())
    expect(label).toBe('Trà Xanh Cướp Kịch Bản')
    expect(label).not.toMatch(/^\d+\s*·|^\d+-/)
  })

  it('engine điền chính slug vào title → dựng tên đọc được, bỏ "01-"', () => {
    expect(storyLabel(story({ title: '01-tra-xanh-cuop-kich-ban' }))).toBe('Tra Xanh Cuop Kich Ban')
    expect(storyLabel(story({ title: 'tra-xanh-cuop-kich-ban' }))).toBe('Tra Xanh Cuop Kich Ban')
    expect(storyLabel(story({ title: '' }))).toBe('Tra Xanh Cuop Kich Ban')
  })

  it('thư mục tạm NN-truyen-moi → "Truyện chưa đặt tên"', () => {
    expect(storyLabel(story({ story: '02-truyen-moi', title: '02-truyen-moi', tempName: true }))).toBe('Truyện chưa đặt tên')
    expect(storyLabel(story({ story: '02-truyen-moi', title: '', tempName: false }))).toBe('Truyện chưa đặt tên')
  })

  it('chưa tải truyện: chỉ có slug từ route → vẫn không lộ slug; không có gì → rỗng', () => {
    expect(storyLabel({ story: '03-dem-truoc-bao' })).toBe('Dem Truoc Bao')
    expect(storyLabel(null)).toBe('')
  })

  it('title người dùng đặt trông giống slug nhưng khác thư mục thì giữ nguyên', () => {
    expect(storyLabel(story({ title: '2024-trở-lại' }))).toBe('2024-trở-lại')
  })
})

describe('humanizeSlug', () => {
  it('tách dấu gạch, viết hoa từng từ, tuỳ chọn bỏ tiền tố số', () => {
    expect(humanizeSlug('dit-dang-audio')).toBe('Dit Dang Audio')
    expect(humanizeSlug('12-hai-muoi', true)).toBe('Hai Muoi')
    expect(humanizeSlug('12-hai-muoi')).toBe('12 Hai Muoi')
  })
})

describe('mobileHeaderCopy — header điện thoại (390×844)', () => {
  it('Tuỳ chỉnh: Noverse / Tuỳ chỉnh', () => {
    const copy = mobileHeaderCopy(parseHash('#/settings').route, null, null)
    expect(copy).toEqual({ meta: 'Noverse', title: 'Tuỳ chỉnh' })
    expect(copy.meta).not.toBe(copy.title)
  })

  it('màn Dự án: Thư viện / tên dự án', () => {
    const copy = mobileHeaderCopy(parseHash('#/p/dit-dang-audio').route, 'Kênh truyện audio', null)
    expect(copy).toEqual({ meta: 'Thư viện', title: 'Kênh truyện audio' })
    expect(copy.meta).not.toBe(copy.title)
  })

  it('màn truyện: "tên dự án · Tập 1" / tên truyện', () => {
    const copy = mobileHeaderCopy(parseHash('#/p/dit-dang-audio/s/01-tra-xanh/work').route, 'Kênh truyện audio', 'Trà Xanh Cướp Kịch Bản')
    expect(copy).toEqual({ meta: 'Kênh truyện audio · Tập 1', title: 'Trà Xanh Cướp Kịch Bản' })
  })

  it('truyện mới: dự án / Truyện mới', () => {
    const copy = mobileHeaderCopy(parseHash('#/p/dit-dang-audio/new').route, 'Kênh truyện audio', null)
    expect(copy).toEqual({ meta: 'Kênh truyện audio', title: 'Truyện mới' })
  })

  it('chưa biết tên dự án thì dùng nhãn Thư viện, không lộ tên thư mục gốc', () => {
    const copy = mobileHeaderCopy(parseHash('#/p/chua-tai').route, null, null)
    expect(copy).toEqual({ meta: 'Noverse', title: 'Thư viện' })
  })

  it('mọi màn đều có 2 dòng khác nhau', () => {
    const cases: [string, string | null, string | null][] = [
      ['#/settings', null, null],
      ['#/p/dit-dang-audio', 'Kênh truyện audio', null],
      ['#/p/_/s/ban-cu/read/3', 'Truyện cũ', 'Bản cũ'],
      ['#/p/dit-dang-audio/new', 'Kênh truyện audio', null],
      // Dự án tên trùng nhãn "Thư viện" vẫn phải khác dòng
      ['#/p/thu-vien', 'Thư viện', null],
    ]
    for (const [hash, projectName, storyTitle] of cases) {
      const copy = mobileHeaderCopy(parseHash(hash).route, projectName, storyTitle)
      expect(copy.meta, hash).not.toBe(copy.title)
      expect(copy.meta, hash).not.toBe('')
      expect(copy.title, hash).not.toBe('')
    }
  })
})

describe('documentTitle — tiêu đề tab trình duyệt', () => {
  it('truyện: tên truyện · tên dự án · Noverse, không slug', () => {
    const { route } = parseHash('#/p/dit-dang-audio/s/01-tra-xanh/work')
    const title = documentTitle(route, 'Kênh truyện audio', 'Trà Xanh Cướp Kịch Bản')
    expect(title).toBe('Trà Xanh Cướp Kịch Bản · Kênh truyện audio · Noverse')
    expect(title).not.toMatch(/dit-dang|01-/)
  })

  it('Tuỳ chỉnh, dự án, truyện mới', () => {
    expect(documentTitle(parseHash('#/settings').route, null, null)).toBe('Tuỳ chỉnh · Noverse')
    expect(documentTitle(parseHash('#/p/dit-dang-audio').route, 'Kênh truyện audio', null)).toBe('Kênh truyện audio · Noverse')
    expect(documentTitle(parseHash('#/p/dit-dang-audio/new').route, 'Kênh truyện audio', null)).toBe('Truyện mới · Kênh truyện audio · Noverse')
  })
})
