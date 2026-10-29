// StreamReveal — nhịp lộ chữ của Luồng viết trực tiếp. Test điều khiển thời gian
// bằng tay (tick(now) nhận timestamp kiểu requestAnimationFrame), không cần DOM.
import { describe, expect, it } from 'vitest'
import { REVEAL_BASE_GS, REVEAL_LAG_SEC, StreamReveal } from './streamReveal'

const FRAME = 1000 / 60

/** Chạy `frames` khung 60fps kể từ `from`; `check` chạy ngay trong từng khung
 * (trước khung kế tiếp) nên đọc được trạng thái đúng của khung đó. */
const drive = (reveal: StreamReveal, from: number, frames: number, check?: (shown: string) => void): number[] => {
  const lengths: number[] = []
  for (let i = 1; i <= frames; i++) {
    const shown = reveal.tick(from + i * FRAME)
    lengths.push(shown.length)
    check?.(shown)
  }
  return lengths
}

/** Ranh giới cụm grapheme của `text` (offset UTF-16) — dùng Intl.Segmenter như pacer. */
const boundaries = (text: string): Set<number> => {
  const set = new Set<number>([0])
  for (const seg of new Intl.Segmenter('vi', { granularity: 'grapheme' }).segment(text)) {
    set.add(seg.index + seg.segment.length)
  }
  return set
}

const PROSE = 'Tôi chuyển phí tư vấn. Ứng dụng ngân hàng báo số dư còn lại thấp hơn tiền đặt cọc cần phòng. '
  + 'Tin nhắn chủ nhà vẫn nằm trên màn hình khoá, chờ tôi trả lời. '
  + 'Tôi nhắn: “Xin lỗi, hôm nay tôi không chốt được.” Rồi gửi cho Chi toàn bộ những gì mình tìm thấy.'

describe('StreamReveal — lộ chữ theo nhịp', () => {
  it('không lùi: độ dài hiện ra tăng dần và luôn là tiền tố của văn bản đích', () => {
    const reveal = new StreamReveal()
    const chunks = [PROSE.slice(0, 40), PROSE.slice(0, 120), PROSE]
    let previous = 0
    let now = 0
    for (const chunk of chunks) {
      reveal.push(chunk)
      drive(reveal, now, 40, (shown) => {
        expect(shown.length).toBeGreaterThanOrEqual(previous)
        expect(chunk.startsWith(shown)).toBe(true)
        previous = shown.length
      })
      now += 40 * FRAME
    }
    drive(reveal, now, 400)
    expect(reveal.current).toBe(PROSE)
    expect(reveal.done).toBe(true)
  })

  it('không chẻ cụm grapheme: “ệ” dựng từ e + dấu móc + dấu mũ không bao giờ hiện nửa chừng', () => {
    const reveal = new StreamReveal()
    // “ệ” dạng NFD: e + U+0323 (dấu móc) + U+0302 (dấu mũ) — SSE có thể cắt giữa cụm.
    const target = `Vi\u0065\u0323\u0302t`
    let received = ''
    let now = 0
    for (const unit of target) {
      received += unit
      reveal.push(received)
      const allowed = boundaries(received)
      drive(reveal, now, 3, (shown) => {
        expect(allowed.has(shown.length)).toBe(true)
      })
      now += 3 * FRAME
    }
    drive(reveal, now, 120)
    expect(reveal.current).toBe(target)
    expect(reveal.current).toBe('Việt'.normalize('NFD'))
  })

  it('không chẻ emoji ZWJ và cụm dấu rời khi văn bản đích nối dài dần', () => {
    const reveal = new StreamReveal()
    const target = 'Gia đình 👨‍👩‍👧 ở Huế̂'
    let received = ''
    let now = 0
    for (const unit of Array.from(target)) {
      received += unit
      reveal.push(received)
      const allowed = boundaries(received)
      drive(reveal, now, 2, (shown) => {
        expect(allowed.has(shown.length)).toBe(true)
      })
      now += 2 * FRAME
    }
    drive(reveal, now, 200)
    expect(reveal.current).toBe(target)
    expect(reveal.done).toBe(true)
  })

  it('bắt kịp theo backlog: cụm lớn về một lúc vẫn đuổi kịp trong vài giây', () => {
    const reveal = new StreamReveal()
    const burst = PROSE.repeat(4)
    reveal.push(burst)
    const lags: number[] = []
    const now = 0
    drive(reveal, now, 300, (shown) => lags.push(burst.length - shown.length))
    const atOneLag = lags[Math.round(REVEAL_LAG_SEC * 60) - 1]
    const atTwoLag = lags[Math.round(REVEAL_LAG_SEC * 120) - 1]
    // Tụt hậu giảm dần và đã hụt dưới 40% sau 1 LAG, dưới 15% sau 2 LAG.
    expect(atOneLag).toBeLessThan(burst.length * 0.4)
    expect(atTwoLag).toBeLessThan(burst.length * 0.15)
    expect(lags.every((value, index) => index === 0 || value <= lags[index - 1])).toBe(true)
    const after = drive(reveal, 300 * FRAME, 600)
    expect(after[after.length - 1]).toBe(burst.length)
    expect(reveal.current).toBe(burst)
  })

  it('tốc độ nền khi backlog nhỏ: gõ chậm như người, không nhảy cả đoạn', () => {
    const reveal = new StreamReveal()
    const target = 'Một hai ba bốn năm sáu bảy tám chín mười.'
    reveal.push(target)
    const lengths = drive(reveal, 0, 60)
    expect(lengths[59]).toBeGreaterThan(0)
    expect(lengths[59]).toBeLessThanOrEqual(Math.ceil(REVEAL_BASE_GS * 1.05))
    expect(lengths[59]).toBeLessThan(target.length)
  })

  it('flush: hiện toàn bộ ngay khi hết stream / đổi chương / ẩn tab', () => {
    const reveal = new StreamReveal()
    reveal.push(PROSE)
    drive(reveal, 0, 5)
    expect(reveal.current.length).toBeLessThan(PROSE.length)
    reveal.flush()
    expect(reveal.current).toBe(PROSE)
    expect(reveal.done).toBe(true)
  })

  it('prefers-reduced-motion: trả nguyên văn, không cần khung hình', () => {
    const reveal = new StreamReveal()
    reveal.setReduced(true)
    reveal.push(PROSE)
    expect(reveal.current).toBe(PROSE)
    reveal.push(`${PROSE} Thêm.`)
    expect(reveal.current).toBe(`${PROSE} Thêm.`)
    expect(reveal.tick(16).length).toBe(`${PROSE} Thêm.`.length)
    // Tắt giảm chuyển động → gõ dần phần mới từ vị trí đã hiện, không lộ lại.
    reveal.setReduced(false)
    reveal.push(`${PROSE} Thêm. Nữa.`)
    expect(reveal.current).toBe(`${PROSE} Thêm.`)
    drive(reveal, 16, 60)
    expect(reveal.current).toBe(`${PROSE} Thêm. Nữa.`)
  })

  it('đổi chương (văn bản mới không nối tiếp) → gõ lại từ đầu', () => {
    const reveal = new StreamReveal()
    reveal.push(PROSE)
    drive(reveal, 0, 30)
    expect(reveal.current.length).toBeGreaterThan(0)
    reveal.push('Chương khác mở bằng một câu khác.')
    expect(reveal.current).toBe('')
    drive(reveal, 30 * FRAME, 120)
    expect(reveal.current).toBe('Chương khác mở bằng một câu khác.')
  })
})
