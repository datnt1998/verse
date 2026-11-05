import { describe, expect, it } from 'vitest'
import { isYoutubeLink } from './performance-logic'

describe('link YouTube tuỳ chọn', () => {
  it('để trống là hợp lệ; chấp nhận watch, shorts, youtu.be; từ chối nơi khác', () => {
    expect(isYoutubeLink('  ')).toBe(true)
    expect(isYoutubeLink('https://www.youtube.com/watch?v=abc123')).toBe(true)
    expect(isYoutubeLink('youtube.com/shorts/abc123')).toBe(true)
    expect(isYoutubeLink('https://youtu.be/abc123')).toBe(true)
    expect(isYoutubeLink('https://youtube.com/watch')).toBe(false)
    expect(isYoutubeLink('https://example.com/watch?v=abc123')).toBe(false)
    expect(isYoutubeLink('không phải link')).toBe(false)
  })
})
