import { describe, expect, it } from 'vitest'
import { pickToastBottom, tabModeFor, visualViewportLayout, PHONE_MAX_NOVERSE, PHONE_MAX_VIDERSE, phoneQuery } from './safeArea'


describe('nhịp điện thoại', () => {
  it('mốc bề ngang là hằng số có tên', () => {
    expect(phoneQuery(PHONE_MAX_NOVERSE)).toBe('(max-width: 640px)')
    expect(phoneQuery(PHONE_MAX_VIDERSE)).toBe('(max-width: 767px)')
  })

  it('hide thắng mini; bàn phím mở luôn ẩn', () => {
    expect(tabModeFor({})).toBe('default')
    expect(tabModeFor({ down: true })).toBe('mini')
    expect(tabModeFor({ down: true, immersive: true })).toBe('hide')
    expect(tabModeFor({ down: true, hidden: true })).toBe('hide')
    expect(tabModeFor({ keyboard: 320 })).toBe('hide')
  })

  it('neo toast: ưu tiên cao thắng, cùng ưu tiên thì mục sau thắng', () => {
    expect(pickToastBottom([])).toBeNull()
    expect(pickToastBottom([{ px: 96, priority: 0 }])).toBe(96)
    expect(pickToastBottom([{ px: 96, priority: 0 }, { px: 148, priority: 1 }])).toBe(148)
    expect(pickToastBottom([{ px: 96, priority: 1 }, { px: 148, priority: 1 }])).toBe(148)
  })
})

describe('khung nhìn thật: đáy shell và thanh tab', () => {
  it('iOS 26 còn offsetTop sau khi đóng bàn phím: đáy vẫn ở mép thiết bị, không phải visualViewport.height', () => {
    // Safari 26 để lại đúng trạng thái này: innerHeight 844, visualViewport.height 820,
    // offsetTop 24. Neo vào `height` sẽ đẩy thanh tab lên 24px; đáy nhìn thấy là 844.
    expect(visualViewportLayout({ innerHeight: 844, height: 820, offsetTop: 24, scale: 1, editing: false }))
      .toEqual({ keyboard: 0, bottom: 844 })
  })

  it('visualViewport hụt nhẹ mà không có offsetTop: vẫn lấy innerHeight làm đáy', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 820, offsetTop: 0, scale: 1, editing: false }))
      .toEqual({ keyboard: 0, bottom: 844 })
  })

  it('bàn phím thật khi ô nhập đang focus: báo bàn phím và không lấy đáy đó làm shell', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 520, offsetTop: 24, scale: 1, editing: true }))
      .toEqual({ keyboard: 300, bottom: null })
  })

  it('thanh địa chỉ co giãn khi không focus ô nhập: KHÔNG phải bàn phím, không ẩn nav', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 760, offsetTop: 0, scale: 1, editing: false }))
      .toEqual({ keyboard: 0, bottom: 844 })
  })

  it('blur xong bàn phím còn đóng dở: không ẩn nav, shell vẫn đủ đáy', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 520, offsetTop: 24, scale: 1, editing: false }))
      .toEqual({ keyboard: 0, bottom: 844 })
  })

  it('đang thu phóng: không nhận bàn phím, không suy ra chiều cao shell', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 600, offsetTop: 120, scale: 1.8, editing: true }))
      .toEqual({ keyboard: 0, bottom: null })
    expect(visualViewportLayout({ innerHeight: 844, height: 1055, offsetTop: 0, scale: .8, editing: false }))
      .toEqual({ keyboard: 0, bottom: null })
  })

  it('số đo rỗng hoặc offsetTop âm: không nhận bàn phím sai', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 0, offsetTop: 0, scale: 1, editing: false }))
      .toEqual({ keyboard: 0, bottom: null })
    expect(visualViewportLayout({ innerHeight: 844, height: 600, offsetTop: -120, scale: 1, editing: false }))
      .toEqual({ keyboard: 0, bottom: 844 })
  })

  it('đáy vùng nhìn thấy vượt innerHeight (offsetTop cộng dồn): shell nới tới đáy đó', () => {
    expect(visualViewportLayout({ innerHeight: 844, height: 844, offsetTop: 24, scale: 1, editing: false }).bottom).toBe(868)
  })
})
