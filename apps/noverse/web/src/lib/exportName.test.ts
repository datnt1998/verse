import { describe, expect, it } from 'vitest'
import { exportExt, exportFormatOf, exportLocationLabel, friendlyExportName, inferExportVolume, sanitizeFileTitle } from './exportName'

describe('friendlyExportName', () => {
  it('ghép tên truyện · Tập N và đuôi tệp', () => {
    expect(friendlyExportName('Trà Xanh Cướp Kịch Bản', 1, 'epub')).toBe('Trà Xanh Cướp Kịch Bản · Tập 1.epub')
    expect(friendlyExportName('Trà Xanh Cướp Kịch Bản', 12, '.txt')).toBe('Trà Xanh Cướp Kịch Bản · Tập 12.txt')
  })

  it('không có tập thì bỏ phần “· Tập N”', () => {
    expect(friendlyExportName('Trà Xanh', null, 'txt')).toBe('Trà Xanh.txt')
    expect(friendlyExportName('Trà Xanh', 0, 'txt')).toBe('Trà Xanh.txt')
  })

  it('tên rỗng hoặc chỉ ký tự cấm → “Truyện”, không bao giờ rơi về slug hay đường dẫn', () => {
    expect(friendlyExportName('', 1, 'txt')).toBe('Truyện · Tập 1.txt')
    expect(friendlyExportName(undefined, null, 'epub')).toBe('Truyện.epub')
    expect(friendlyExportName('///', 2, 'txt')).toBe('Truyện · Tập 2.txt')
  })

  it('bỏ ký tự cấm trong tên tệp và gộp khoảng trắng', () => {
    expect(friendlyExportName('  A/B: "C"?  ', 1, 'txt')).toBe('A B C · Tập 1.txt')
    expect(sanitizeFileTitle('..Ngày  mai.. ')).toBe('Ngày mai')
    expect(friendlyExportName('a\\b/../c', 1, 'txt')).not.toMatch(/[\\/]/)
  })

  it('bản đọc TTS hiện “· bản đọc” trước đuôi .txt (D3)', () => {
    expect(friendlyExportName('Trà Xanh Cướp Kịch Bản', 1, 'narration')).toBe('Trà Xanh Cướp Kịch Bản · Tập 1 · bản đọc.txt')
    expect(friendlyExportName('Trà Xanh', null, 'narration')).toBe('Trà Xanh · bản đọc.txt')
  })
})

describe('exportFormatOf', () => {
  it('nhận ra bản đọc TTS bằng hậu tố -narration của tệp .txt', () => {
    expect(exportFormatOf('xuat-ban/tra-xanh-tap1-narration.txt')).toBe('narration')
    expect(exportFormatOf('tra-xanh-tap1.txt')).toBe('txt')
    expect(exportFormatOf('tap-2/A.EPUB')).toBe('epub')
  })
})

describe('inferExportVolume / exportExt / exportLocationLabel', () => {
  it('đọc tập từ đoạn đường dẫn tap-N hoặc vol-N', () => {
    expect(inferExportVolume('tap-2/tra-xanh.tap-2.epub')).toBe(2)
    expect(inferExportVolume('xuat-ban\\Vol_03\\a.txt')).toBe(3)
    expect(inferExportVolume('a.txt')).toBeNull()
  })

  it('lấy đuôi tệp chữ thường', () => {
    expect(exportExt('tap-1/A.EPUB')).toBe('epub')
    expect(exportExt('khong-co-duoi')).toBe('')
  })

  it('nhãn vị trí chỉ dùng “Bản xuất › Tập N › tên”', () => {
    expect(exportLocationLabel('Trà Xanh · Tập 1.txt', 1)).toBe('Bản xuất › Tập 1 › Trà Xanh · Tập 1.txt')
    expect(exportLocationLabel('Trà Xanh.txt', null)).toBe('Bản xuất › Trà Xanh.txt')
  })
})
