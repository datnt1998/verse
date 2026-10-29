// StreamReveal — bộ đệm lộ chữ theo từng cụm grapheme cho “Luồng viết trực tiếp”.
// SSE `delta` đổ vào theo cụm lớn (useStoryEvents gom theo rAF); pacer này trả chữ ra
// đều như gõ máy: tốc độ nền BASE_GS, tăng theo backlog để không bao giờ tụt lại quá
// ~LAG_SEC giây. Ranh giới ký tự dùng Intl.Segmenter nên dấu tổ hợp tiếng Việt
// (e + ◌̣ + ◌̂ → “ệ”) không bao giờ bị chẻ. flush() hiện toàn bộ ngay — gọi khi hết
// stream, đổi chương, ẩn tab hoặc prefers-reduced-motion.
import { useEffect, useRef, useState } from 'react'

/** Tốc độ nền: ~33 grapheme/giây — đọc kịp mắt, giống nhịp gõ. */
export const REVEAL_BASE_GS = 33
/** Ngân sách tụt hậu (giây): backlog lớn → tốc độ = backlog/LAG_SEC. */
export const REVEAL_LAG_SEC = 1.4
/** Khung rAF trần 120ms: tick to bất thường (GC, treo máy) không nhảy chữ. */
const MAX_DT_MS = 120

type Segmenter = { segment(input: string): Iterable<{ index: number; segment: string }> }
type SegmenterCtor = new (locale: string, options: { granularity: 'grapheme' }) => Segmenter

// TS lib của target chưa khai báo `Intl.Segmenter` → dò runtime, không đoán kiểu.
// (Cũng tránh `new (Intl as …)(…)`: esbuild bỏ ép kiểu và ném lỗi khi nạp module.)
const segmenterCtor = (): SegmenterCtor | null => {
  const candidate: unknown = typeof Intl === 'undefined' ? null : Reflect.get(Intl, 'Segmenter')
  // Đúng constructor chuẩn khi typeof là function; sai hình dạng sẽ ném ngay ở `new`.
  return typeof candidate === 'function' ? (candidate as SegmenterCtor) : null
}
const SegmenterImpl = segmenterCtor()
const segmenter: Segmenter | null = SegmenterImpl ? new SegmenterImpl('vi', { granularity: 'grapheme' }) : null

/** Điểm kết thúc của từng cụm grapheme trong `text` (offset UTF-16, dãy tăng nghiêm ngặt). */
const graphemeEnds = (text: string): number[] => {
  const ends: number[] = []
  if (segmenter) {
    for (const seg of segmenter.segment(text)) ends.push(seg.index + seg.segment.length)
  } else {
    let offset = 0
    for (const char of Array.from(text)) { offset += char.length; ends.push(offset) }
  }
  return ends
}

/** Số phần tử ≤ value (dãy đã sắp) — tìm ranh giới kế tiếp. */
const upperBound = (arr: number[], value: number): number => {
  let lo = 0
  let hi = arr.length
  while (lo < hi) {
    const mid = (lo + hi) >> 1
    if (arr[mid] <= value) lo = mid + 1
    else hi = mid
  }
  return lo
}

export class StreamReveal {
  private target = ''
  /** Offset kết thúc đã finalize (mọi cụm trừ cụm cuối — cụm cuối còn có thể nối dài). */
  private bounds: number[] = []
  /** Đuôi chưa finalize: cụm grapheme cuối cùng của target hiện tại. */
  private pending = ''
  /** Độ dài đã hiện (offset UTF-16 trong target). */
  private shownLen = 0
  /** Số bounds ≤ shownLen — index của ranh giới kế tiếp. */
  private shownIdx = 0
  /** Phần lẻ grapheme chưa đủ 1 cụm để hiện. */
  private carry = 0
  private last: number | null = null
  private reduced = false

  /** Cập nhật văn bản đích (append-only; nội dung khác hẳn → reset và gõ lại từ đầu). */
  push(target: string): void {
    if (target === this.target) return
    if (!target.startsWith(this.target)) {
      this.target = ''
      this.bounds = []
      this.pending = ''
      this.shownLen = 0
      this.shownIdx = 0
      this.carry = 0
    }
    if (this.reduced) {
      this.target = target
      this.bounds = []
      this.pending = ''
      this.shownLen = target.length
      this.shownIdx = 0
      return
    }
    const base = this.target.length - this.pending.length
    // Đuôi rỗng sau flush/giảm chuyển động: mốc nối vẫn là ranh giới hợp lệ.
    if (this.pending === '' && base > 0) this.bounds.push(base)
    const combined = this.pending + target.slice(this.target.length)
    const ends = graphemeEnds(combined)
    for (let i = 0; i < ends.length - 1; i++) this.bounds.push(base + ends[i])
    this.pending = ends.length ? combined.slice(ends[ends.length - 2] ?? 0) : ''
    this.target = target
    this.shownIdx = upperBound(this.bounds, this.shownLen)
    // Cụm cuối vừa dài ra (dấu tổ hợp / ZWJ mới tới): hiện nốt cụm đó thay vì
    // đứng giữa cụm — “e” + dấu móc vừa tới thì hiện “ẹ”, không chờ dấu mũ.
    const onBoundary = this.shownLen === 0 || this.shownLen === target.length
      || this.bounds[this.shownIdx - 1] === this.shownLen
    if (!onBoundary) {
      this.shownLen = this.shownIdx < this.bounds.length ? this.bounds[this.shownIdx] : target.length
      this.shownIdx = upperBound(this.bounds, this.shownLen)
    }
  }

  /** Một khung hình: tiến `dt = now - khung trước`, trả về phần chữ cần hiện. */
  tick(now: number): string {
    if (this.last === null) {
      this.last = now
      return this.current
    }
    const dt = Math.min(Math.max(0, now - this.last), MAX_DT_MS)
    this.last = now
    if (this.done || this.reduced) return this.current
    const rate = Math.max(REVEAL_BASE_GS, this.backlog / REVEAL_LAG_SEC)
    let n = Math.floor(this.carry + (rate * dt) / 1000)
    this.carry = this.carry + (rate * dt) / 1000 - n
    while (n > 0 && this.shownIdx < this.bounds.length) {
      this.shownLen = this.bounds[this.shownIdx]
      this.shownIdx++
      n--
    }
    if (n > 0) this.shownLen = this.target.length
    return this.current
  }

  /** Hiện toàn bộ văn bản đã nhận ngay (hết stream / đổi chương / ẩn tab). */
  flush(): void {
    this.shownLen = this.target.length
    this.shownIdx = this.bounds.length
    this.carry = 0
  }

  /** prefers-reduced-motion → mọi push/tick đều trả toàn văn, không gõ dần. */
  setReduced(on: boolean): void {
    this.reduced = on
    if (on) this.flush()
  }

  get done(): boolean {
    return this.shownLen >= this.target.length
  }

  /** Số grapheme chưa hiện (dùng cho tốc độ bắt kịp; test đọc trực tiếp). */
  get backlog(): number {
    if (this.done) return 0
    return this.bounds.length - this.shownIdx + 1
  }

  get current(): string {
    return this.target.slice(0, this.shownLen)
  }
}

/**
 * Hook quanh StreamReveal: đẩy `target` vào pacer, chạy bằng requestAnimationFrame,
 * `shown` là phần chữ đang hiện. `live=false` hoặc reduced-motion → trả `target`
 * nguyên vẹn, không tạo khung hình. Ẩn tab (visibilitychange) → flush ngay.
 */
export function useStreamReveal(target: string, live: boolean, reduced: boolean): string {
  const [shown, setShown] = useState('')
  const ref = useRef<StreamReveal | null>(null)
  const pacer = (ref.current ??= new StreamReveal())
  pacer.push(target)
  pacer.setReduced(reduced)

  useEffect(() => {
    if (!live || reduced) {
      pacer.flush()
      return
    }
    let raf = 0
    let alive = true
    const step = (now: number) => {
      if (!alive) return
      setShown(pacer.tick(now))
      raf = requestAnimationFrame(step)
    }
    raf = requestAnimationFrame(step)
    const onVisibility = () => pacer.flush()
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      alive = false
      cancelAnimationFrame(raf)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [live, reduced, pacer])

  if (!live || reduced || pacer.done) return target
  return shown
}
