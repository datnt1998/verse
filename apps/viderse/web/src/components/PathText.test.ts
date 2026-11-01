// @vitest-environment node
// 04-components › Đường dẫn thư mục (#78) — cắt giữa theo thư mục, đo bằng bề
// rộng thật. Bài kiểm tra dùng đúng số đo artboard nêu: mono 11,5px ≈ 7,17
// px/ký tự, dòng đường dẫn ≈ 430px ở 1440 và ≈ 290px ở 390.
import { describe, expect, it } from 'vitest'
import { fitPath, type Measure } from './PathText'

const CHAR = 430 / 60 // artboard: ≈ 60 ký tự mono 11,5px trong 430px
const measure: Measure = (text) => text.length * CHAR

const LONG = '/Volumes/Samsung T7/Studio 2026/Kênh YouTube/Viderse Projects'

describe('fitPath — cắt giữa theo thư mục', () => {
  it('đường ngắn giữ nguyên, không thêm “…”', () => {
    expect(fitPath('~/.viderse/projects', 430, measure)).toBe('~/.viderse/projects')
    expect(fitPath('/Volumes/Samsung T7/Viderse Projects', 430, measure)).toBe('/Volumes/Samsung T7/Viderse Projects')
  })

  it('1440 (≈430px): giữ đầu + 2 thư mục cuối', () => {
    expect(fitPath(LONG, 430, measure)).toBe('/Volumes/Samsung T7/…/Kênh YouTube/Viderse Projects')
  })

  it('390 (≈290px): giữ đầu + 1 thư mục cuối', () => {
    expect(fitPath(LONG, 290, measure)).toBe('/Volumes/Samsung T7/…/Viderse Projects')
  })

  it('không bao giờ cắt cuối: luôn giữ đuôi tên thư mục', () => {
    for (const width of [430, 290]) {
      const out = fitPath(LONG, width, measure)
      expect(out.startsWith('/Volumes/Samsung T7/…/')).toBe(true)
      expect(out.endsWith('Projects')).toBe(true)
      expect(measure(out)).toBeLessThanOrEqual(width)
    }
    // Hẹp hơn cả phần đầu + 1 thư mục cuối: giữ đầu, cắt ký tự ở ĐẦU tên cuối
    // (đuôi tên vẫn còn) — không bao giờ bỏ đuôi.
    for (const width of [200, 150, 120]) {
      const out = fitPath(LONG, width, measure)
      expect(out.startsWith('/Volumes/Samsung T7/…/…')).toBe(true)
      expect('Projects'.endsWith(out.slice(-1))).toBe(true)
    }
  })

  it('nhà (~) giữ “~” làm phần đầu', () => {
    expect(fitPath('~/.viderse/projects/2026/Studio/Kênh YouTube', 120, measure)).toBe('~/…/Kênh YouTube')
  })

  it('tên thư mục cuối quá dài mới cắt ký tự ở ĐẦU tên', () => {
    const longName = `/Users/dat/${'a'.repeat(60)}`
    const out = fitPath(longName, 120, measure)
    expect(out.startsWith('/Users/…/…')).toBe(true)
    expect(out.endsWith('a')).toBe(true)
    expect(measure(out)).toBeLessThanOrEqual(120)
  })

  it('chưa đo được bề rộng (0) thì in đủ đường dẫn', () => {
    expect(fitPath(LONG, 0, measure)).toBe(LONG)
  })
})
