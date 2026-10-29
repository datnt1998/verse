import { describe, expect, it } from 'vitest'
import { exampleFileName, toggleFormat } from './exportLogic'

describe('Xuất tự động', () => {
  it('toggleFormat: bật/tắt giữ thứ tự txt → epub, không trùng', () => {
    expect(toggleFormat(['epub'], 'txt', true)).toEqual(['txt', 'epub'])
    expect(toggleFormat(['txt', 'epub'], 'txt', false)).toEqual(['epub'])
    expect(toggleFormat(['txt'], 'txt', true)).toEqual(['txt'])
    expect(toggleFormat(['txt'], 'txt', false)).toEqual([])
  })

  it('exampleFileName: tên thân thiện không đường dẫn, .epub khi có chọn epub', () => {
    expect(exampleFileName('Trà Xanh Cướp Kịch Bản', ['txt', 'epub'])).toBe('Trà Xanh Cướp Kịch Bản · Tập 1.epub')
    expect(exampleFileName('Trà Xanh Cướp Kịch Bản', ['txt'])).toBe('Trà Xanh Cướp Kịch Bản · Tập 1.txt')
    expect(exampleFileName('', ['epub'])).toBe('Truyện · Tập 1.epub')
  })
})
