import { describe, expect, it } from 'vitest'
import {
  clampSlugWords, dirNameState, renderPattern, slugify, slugifyOrEmpty, storyFolderName, withKeepSuffix,
} from './slug'

describe('slugify (cùng quy tắc backend — 11-api-contract.md §3)', () => {
  it('bỏ dấu tiếng Việt, đ→d, chữ thường, nối bằng "-"', () => {
    expect(slugify('Trà Xanh Cướp Kịch Bản Của Tôi', 5)).toBe('tra-xanh-cuop-kich-ban')
  })

  it('đ/Đ thành d', () => {
    expect(slugify('Đít Đang Audio', 12)).toBe('dit-dang-audio')
  })

  it('ký tự khác [a-z0-9] thành khoảng trắng, gộp khoảng trắng', () => {
    expect(slugify('  Kịch bản: “Tập 1” — (bản nháp)!  ', 12)).toBe('kich-ban-tap-1-ban-nhap')
  })

  it('giới hạn số từ đầu', () => {
    expect(slugify('một hai ba bốn năm sáu', 3)).toBe('mot-hai-ba')
  })

  it('rỗng → truyen-moi', () => {
    expect(slugify('', 5)).toBe('truyen-moi')
    expect(slugify('!!!', 5)).toBe('truyen-moi')
  })

  it('slugifyOrEmpty giữ rỗng khi tên không có ký tự dùng được', () => {
    expect(slugifyOrEmpty('!!!', 5)).toBe('')
    expect(slugifyOrEmpty('  —  ', 5)).toBe('')
    expect(slugifyOrEmpty('Đít Đang', 5)).toBe('dit-dang')
    expect(slugifyOrEmpty('a b c', 0)).toBe('a-b-c')
  })

  it('kẹp số từ 1–12', () => {
    expect(clampSlugWords(0)).toBe(1)
    expect(clampSlugWords(99)).toBe(12)
    expect(clampSlugWords(Number.NaN)).toBe(5)
    expect(slugify('a b c', 0)).toBe('a-b-c')
    expect(slugify('a b c')).toBe('a-b-c')
  })
})

describe('storyFolderName', () => {
  it('NN 2 chữ số + slug', () => {
    expect(storyFolderName(1, 'Trà Xanh Cướp Kịch Bản', 4)).toBe('01-tra-xanh-cuop-kich')
    expect(storyFolderName(12, 'Cư Dân', 4)).toBe('12-cu-dan')
  })
})

describe('dirNameState', () => {
  it('tên mặc định → standard', () => {
    expect(dirNameState('kich_ban', 'kich_ban')).toBe('standard')
  })
  it('đổi nhưng hợp lệ → valid', () => {
    expect(dirNameState('ban-thao', 'kich_ban')).toBe('valid')
    expect(dirNameState('a', 'kich_ban')).toBe('valid')
  })
  it('sai chuẩn → invalid', () => {
    for (const bad of ['', 'Kich_Ban', '-abc', '_abc', 'có dấu', 'a b', 'a/b', '.ainovel']) {
      expect(dirNameState(bad, 'kich_ban')).toBe('invalid')
    }
    expect(dirNameState('output', 'kich_ban')).toBe('invalid')
    expect(dirNameState('project.json', 'kich_ban')).toBe('invalid')
    expect(dirNameState('a'.repeat(101), 'kich_ban')).toBe('invalid')
  })
})

describe('renderPattern', () => {
  const base = { storyFolder: '01-tra-xanh', formats: ['txt', 'epub'] }
  it('thay mọi biến', () => {
    expect(renderPattern('tap-{tap}/{truyen}.tap-{tap}.{ext}', base))
      .toBe('tap-01/01-tra-xanh.tap-01.txt')
    expect(renderPattern('{truyen}-{tu}-{den}-{ngay}.{ext}', { ...base, from: '3', to: '9', date: '20261025' }))
      .toBe('01-tra-xanh-3-9-20261025.txt')
  })
  it('chỉ epub → ext epub', () => {
    expect(renderPattern('{truyen}.{ext}', { ...base, formats: ['epub'] })).toBe('01-tra-xanh.epub')
  })
})

describe('withKeepSuffix', () => {
  it('chèn -YYYYMMDD-HHmmss trước đuôi', () => {
    expect(withKeepSuffix('tap-01/truyen.tap-01.txt')).toBe('tap-01/truyen.tap-01-YYYYMMDD-HHmmss.txt')
    expect(withKeepSuffix('tap-01/truyen')).toBe('tap-01/truyen-YYYYMMDD-HHmmss')
  })
})
