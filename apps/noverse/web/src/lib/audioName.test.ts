// A7 · 10-audio.md — quy tắc tên tệp/tên ngắn tải về (khớp AudioMeta.dc.html renderVals).
import { describe, expect, it } from 'vitest'
import { audioChapterFileName, audioZipFileName, audioVolumeFileName, chapterTrackTitle, shortStoryTitle, volumeTrackTitle, asciiFileName } from './audioName'

const STORY = 'Trà Xanh Cướp Kịch Bản Của Tôi, Tôi Lên Show Thực Tế Đọc Bản Gốc'
const SHORT = 'Trà Xanh Cướp Kịch Bản'

describe('shortStoryTitle', () => {
  it('ưu tiên tên ngắn đã lưu', () => {
    expect(shortStoryTitle(STORY, SHORT)).toBe(SHORT)
  })
  it('tên ngắn lưu bị ký tự cấm vẫn được làm sạch', () => {
    expect(shortStoryTitle(STORY, 'Trà Xanh: Phần/2')).toBe('Trà Xanh Phần 2')
  })
  it('cắt tên truyện ở ranh giới từ khi dài hơn 32', () => {
    const short = shortStoryTitle(STORY)
    expect(short.length).toBeLessThanOrEqual(32)
    expect(short.endsWith(' ')).toBe(false)
    expect(STORY.startsWith(short)).toBe(true)
    expect(short).toBe('Trà Xanh Cướp Kịch Bản Của Tôi,')
  })
  it('giữ nguyên tên vừa 32 ký tự trở xuống', () => {
    expect(shortStoryTitle(SHORT)).toBe(SHORT)
  })
  it('bỏ ký tự cấm trong tên tệp trước khi cắt', () => {
    expect(shortStoryTitle('A/B\\C:D')).toBe('A B C D')
  })
  it('không còn gì sau khi làm sạch thì về tên dự phòng', () => {
    expect(shortStoryTitle('//\\:*?')).toBe('Truyện')
    expect(shortStoryTitle(null)).toBe('Truyện')
  })
  it.each([
    ['RULES KỊCH BẢN - “Khi sếp học Vibecode”', 'Khi sếp học Vibecode'],
    ['RULES KỊCH BẢN: “KHI SẾP HỌC VIBECODE”', 'KHI SẾP HỌC VIBECODE'],
    ['RULES KỊCH\u00a0BẢN — ‘Khi sếp học Vibecode’', 'Khi sếp học Vibecode'],
    ['"Khi sếp học Vibecode"', 'Khi sếp học Vibecode'],
    ['Khi sếp – học Vibecode', 'Khi sếp – học Vibecode'],
    ['Khi/sếp:học Vibecode', 'Khi sếp học Vibecode'],
    ['Khi sếp học Vibecode và cả công ty cùng học lại từ đầu', 'Khi sếp học Vibecode và cả công'],
    ['12345678901234567890123456789 ab tiếp', '12345678901234567890123456789 ab'],
    ['Một truyện “có tên dài hơn ba mươi hai ký tự”', 'Một truyện'],
    ['a'.repeat(40), 'Truyện'],
    ['', 'Truyện'],
  ])('không lấy tiền tố nhập hoặc cắt dở từ/dấu ngoặc: %s', (title, expected) => {
    expect(shortStoryTitle(title)).toBe(expected)
    expect(audioVolumeFileName(shortStoryTitle(title), 1, 'mp3', '-ch1-8')).toBe(`${expected} · Tập 1-ch1-8.mp3`)
  })
  it('bỏ dấu ngoặc bọc tên ngắn đã lưu', () => {
    expect(shortStoryTitle(STORY, '“Tên riêng”')).toBe('Tên riêng')
  })
})

describe('tên bài phát', () => {
  it('chương theo mẫu “Chương N · tên”', () => {
    expect(chapterTrackTitle(4, 'Bảy mươi ba giây')).toBe('Chương 4 · Bảy mươi ba giây')
  })
  it('bản thử thêm nhãn', () => {
    expect(chapterTrackTitle(4, 'Bảy mươi ba giây', true)).toBe('Chương 4 · Bảy mươi ba giây (bản thử)')
  })
  it('tệp cả tập dùng tên ngắn', () => {
    expect(volumeTrackTitle(SHORT, 1)).toBe('Trà Xanh Cướp Kịch Bản · Tập 1')
  })
})

describe('tên tệp tải về (A7)', () => {
  it('theo chương: số chương 2 chữ số', () => {
    expect(audioChapterFileName(SHORT, 1, 4, 'Bảy mươi ba giây', 'mp3'))
      .toBe('Trà Xanh Cướp Kịch Bản · Tập 1 · Chương 04 · Bảy mươi ba giây.mp3')
  })
  it('bản thử thêm “ (bản thử)” trước đuôi', () => {
    expect(audioChapterFileName(SHORT, 1, 4, 'Bảy mươi ba giây', 'mp3', true))
      .toBe('Trà Xanh Cướp Kịch Bản · Tập 1 · Chương 04 · Bảy mươi ba giây (bản thử).mp3')
  })
  it('cả tập', () => {
    expect(audioVolumeFileName(SHORT, 1, 'mp3')).toBe('Trà Xanh Cướp Kịch Bản · Tập 1.mp3')
  })
  it('xuất một phần giữ hậu tố -chN', () => {
    expect(audioVolumeFileName(SHORT, 1, 'm4b', '-ch1-3_5')).toBe('Trà Xanh Cướp Kịch Bản · Tập 1-ch1-3_5.m4b')
  })
  it('nhiều chương gộp zip', () => {
    expect(audioZipFileName(SHORT, 1, 3)).toBe('Trà Xanh Cướp Kịch Bản · Tập 1 · 3 chương.zip')
  })
  it('làm sạch ký tự cấm trong tên chương', () => {
    expect(audioChapterFileName(SHORT, 1, 4, 'Ai nói: “thế này”?', 'mp3'))
      .toBe('Trà Xanh Cướp Kịch Bản · Tập 1 · Chương 04 · Ai nói “thế này”.mp3')
  })
})

describe('asciiFileName — filename= dự phòng (§AF)', () => {
  it('bỏ dấu, đổi · thành -', () => {
    expect(asciiFileName('Trà Xanh Cướp Kịch Bản · Tập 1 · Chương 04 · Bảy mươi ba giây (bản thử).mp3'))
      .toBe('Tra Xanh Cuop Kich Ban - Tap 1 - Chuong 04 - Bay muoi ba giay (ban thu).mp3')
  })
  it('đ/Đ → d/D, ký tự cấm → _', () => {
    expect(asciiFileName('Đít Đang · Tập 1.mp3')).toBe('Dit Dang - Tap 1.mp3')
  })
})
