// Toán màu cho defineBrand: sRGB ↔ OKLab/OKLCH, trộn, tương phản WCAG, quét literal.
// File này cố ý KHÔNG chứa literal màu (#hex/rgb) — số liệu màu nằm ở tokenDefaults.ts.

export interface Rgb { r: number; g: number; b: number }
export interface Oklch { l: number; c: number; h: number }

/** Thông báo lỗi đầu vào brand — mã lỗi đọc được để consumer hiển thị lại. */
export class BrandInputError extends Error {
  readonly field: string
  constructor(field: string, message: string) {
    super(`defineBrand: ${field} — ${message}`)
    this.name = 'BrandInputError'
    this.field = field
  }
}

export function clamp01(x: number): number {
  return x < 0 ? 0 : x > 1 ? 1 : x
}

/** Chuẩn hoá hex '#rgb'/'#rrggbb' (không phân biệt hoa thường) → '#RRGGBB'. Ném BrandInputError nếu sai. */
export function parseHex(value: string, field = 'color'): Rgb {
  const m = /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.exec(value.trim())
  if (!m) throw new BrandInputError(field, `không phải mã màu hex hợp lệ: ${JSON.stringify(value)}`)
  let s = m[1]
  if (s.length === 3) s = s.split('').map((c) => c + c).join('')
  return { r: parseInt(s.slice(0, 2), 16), g: parseInt(s.slice(2, 4), 16), b: parseInt(s.slice(4, 6), 16) }
}

export function toHex({ r, g, b }: Rgb): string {
  const h = (n: number) => Math.round(clamp01(n / 255) * 255).toString(16).padStart(2, '0').toUpperCase()
  return `#${h(r)}${h(g)}${h(b)}`
}

// --- sRGB ↔ linear -----------------------------------------------------------
const lin = (c: number) => (c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4))
const gam = (c: number) => (c <= 0.0031308 ? c * 12.92 : 1.055 * Math.pow(c, 1 / 2.4) - 0.055)

export function srgbToOklch({ r, g, b }: Rgb): Oklch {
  const lr = lin(r / 255); const lg = lin(g / 255); const lb = lin(b / 255)
  const l = 0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb
  const m = 0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb
  const s = 0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb
  const l3 = Math.cbrt(l); const m3 = Math.cbrt(m); const s3 = Math.cbrt(s)
  const L = 0.2104542553 * l3 + 0.793617785 * m3 - 0.0040720468 * s3
  const a = 1.9779984951 * l3 - 2.428592205 * m3 + 0.4505937099 * s3
  const bb = 0.0259040371 * l3 + 0.7827717662 * m3 - 0.808675766 * s3
  return { l: L, c: Math.hypot(a, bb), h: ((Math.atan2(bb, a) * 180) / Math.PI + 360) % 360 }
}

/** OKLCH → sRGB, kẹp gamut bằng cách hạ chroma theo nhị phân (giữ L và hue). */
export function oklchToSrgb({ l, c, h }: Oklch): Rgb {
  const a = c * Math.cos((h * Math.PI) / 180); const bb = c * Math.sin((h * Math.PI) / 180)
  const l3 = l + 0.3963377774 * a + 0.2158037573 * bb
  const m3 = l - 0.1055613458 * a - 0.0638541728 * bb
  const s3 = l - 0.0894841775 * a - 1.291485548 * bb
  const L = l3 ** 3; const M = m3 ** 3; const S = s3 ** 3
  const lr = +4.0767416621 * L - 3.3077115913 * M + 0.2309699292 * S
  const lg = -1.2684380046 * L + 2.6097574011 * M - 0.3413193965 * S
  const lb = -0.0041960863 * L - 0.7034186147 * M + 1.707614701 * S
  const out = { r: gam(lr) * 255, g: gam(lg) * 255, b: gam(lb) * 255 }
  if (out.r >= -0.5 && out.r <= 255.5 && out.g >= -0.5 && out.g <= 255.5 && out.b >= -0.5 && out.b <= 255.5) {
    return { r: clamp01(out.r / 255) * 255, g: clamp01(out.g / 255) * 255, b: clamp01(out.b / 255) * 255 }
  }
  // Ngoài gamut: giảm chroma tới khi vào được (tối đa 24 bước).
  let lo = 0; let hi = c
  let best: Rgb = { r: l * 255, g: l * 255, b: l * 255 }
  for (let i = 0; i < 24; i++) {
    const mid = (lo + hi) / 2
    const t = oklchToSrgbRaw(l, mid, h)
    if (t) { best = t; lo = mid } else hi = mid
  }
  return best
}

function oklchToSrgbRaw(l: number, c: number, h: number): Rgb | null {
  const a = c * Math.cos((h * Math.PI) / 180); const bb = c * Math.sin((h * Math.PI) / 180)
  const l3 = l + 0.3963377774 * a + 0.2158037573 * bb
  const m3 = l - 0.1055613458 * a - 0.0638541728 * bb
  const s3 = l - 0.0894841775 * a - 1.291485548 * bb
  const L = l3 ** 3; const M = m3 ** 3; const S = s3 ** 3
  const lr = +4.0767416621 * L - 3.3077115913 * M + 0.2309699292 * S
  const lg = -1.2684380046 * L + 2.6097574011 * M - 0.3413193965 * S
  const lb = -0.0041960863 * L - 0.7034186147 * M + 1.707614701 * S
  const r = gam(lr); const g = gam(lg); const b = gam(lb)
  if (r < -0.002 || r > 1.002 || g < -0.002 || g > 1.002 || b < -0.002 || b > 1.002) return null
  return { r: clamp01(r) * 255, g: clamp01(g) * 255, b: clamp01(b) * 255 }
}

// --- Phép toán trên OKLCH -----------------------------------------------------

/** Trộn hai màu trong OKLCH (hue nội suy theo cung ngắn). t=0 → a, t=1 → b. */
export function mix(a: Rgb, b: Rgb, t: number): Rgb {
  const ca = srgbToOklch(a); const cb = srgbToOklch(b)
  let dh = cb.h - ca.h
  if (dh > 180) dh -= 360
  if (dh < -180) dh += 360
  return oklchToSrgb({ l: ca.l + (cb.l - ca.l) * t, c: ca.c + (cb.c - ca.c) * t, h: (ca.h + dh * t + 360) % 360 })
}

export function lighten(rgb: Rgb, amount: number): Rgb {
  const { l, c, h } = srgbToOklch(rgb)
  return oklchToSrgb({ l: clamp01(l + amount), c, h })
}
export function darken(rgb: Rgb, amount: number): Rgb {
  return lighten(rgb, -amount)
}
export function rotate(rgb: Rgb, degrees: number): Rgb {
  const { l, c, h } = srgbToOklch(rgb)
  return oklchToSrgb({ l, c, h: (h + degrees + 360) % 360 })
}
/** Đổi chroma trong OKLCH (hue giữ nguyên). */
export function withChroma(rgb: Rgb, c: number): Rgb {
  const { l, h } = srgbToOklch(rgb)
  return oklchToSrgb({ l, c: Math.max(0, c), h })
}
/** Hue/chroma của `tint`, lightness của `rgb` — dùng để nhuộm thang trung tính. */
export function withTint(rgb: Rgb, tint: Rgb): Rgb {
  const { l } = srgbToOklch(rgb)
  const t = srgbToOklch(tint)
  return oklchToSrgb({ l, c: Math.min(t.c, 0.08), h: t.h })
}

// --- Tương phản WCAG ----------------------------------------------------------

function relLum({ r, g, b }: Rgb): number {
  return 0.2126 * lin(r / 255) + 0.7152 * lin(g / 255) + 0.0722 * lin(b / 255)
}
export function contrastRatio(a: Rgb, b: Rgb): number {
  const la = relLum(a); const lb = relLum(b)
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05)
}

/** Sáng hoá màu trong OKLCH tới khi đạt tương phản `target` trên `background` (nền tối). */
export function ensureAA(rgb: Rgb, background: Rgb, target = 4.5): Rgb {
  // Biên +0.02: giá trị trả về là số thực, toHex() làm tròn có thể rơi xuống dưới ngưỡng.
  const goal = target + 0.02
  if (contrastRatio(rgb, background) >= goal) return rgb
  const { c, h } = srgbToOklch(rgb)
  let lo = srgbToOklch(rgb).l; let hi = 1
  let best = oklchToSrgb({ l: hi, c, h })
  for (let i = 0; i < 24; i++) {
    const mid = (lo + hi) / 2
    const t = oklchToSrgb({ l: mid, c, h })
    if (contrastRatio(t, background) >= goal) { best = t; hi = mid } else lo = mid
  }
  return best
}

/** Chữ trên nền màu: trả về màu chữ có tương phản cao hơn giữa hai ứng viên. */
export function readableOn(bg: Rgb, dark: Rgb, light: Rgb): Rgb {
  return contrastRatio(dark, bg) >= contrastRatio(light, bg) ? dark : light
}
// --- Thang tông OKLCH -----------------------------------------------------------

export const TONE_STOPS = [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950] as const
const TONE_LIGHTNESS = [0.96, 0.91, 0.84, 0.76, 0.67, 0.58, 0.48, 0.38, 0.28, 0.18, 0.11]

/** Thang 11 nấc OKLCH từ một màu gốc — hue giữ, chroma hạ dần ở hai đầu. */
export function tonalScale(base: Rgb): Record<(typeof TONE_STOPS)[number], string> {
  const { c, h } = srgbToOklch(base)
  const out = {} as Record<(typeof TONE_STOPS)[number], string>
  TONE_STOPS.forEach((stop, i) => {
    const edge = Math.abs(i - 5) / 5 // 0 ở nấc giữa → 1 ở đầu mút
    out[stop] = toHex(oklchToSrgb({ l: TONE_LIGHTNESS[i], c: Math.min(c, 0.34) * (1 - edge * 0.45), h }))
  })
  return out
}
// --- Quét literal trong giá trị token -----------------------------------------

const HEX = /#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})\b/g
const TRI = /\b(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\b/g

export interface LitSpan { start: number; end: number; text: string; rgb: Rgb }

/** Liệt kê mọi literal màu (hex + bộ ba r,g,b) trong một giá trị token. */
export function colorSpans(value: string): LitSpan[] {
  const out: LitSpan[] = []
  for (const m of value.matchAll(HEX)) out.push({ start: m.index!, end: m.index! + m[0].length, text: m[0], rgb: parseHex(m[0]) })
  for (const m of value.matchAll(TRI)) {
    const rgb = { r: +m[1], g: +m[2], b: +m[3] }
    if (rgb.r > 255 || rgb.g > 255 || rgb.b > 255) continue
    out.push({ start: m.index!, end: m.index! + m[0].length, text: m[0], rgb })
  }
  return out.sort((a, b) => a.start - b.start)
}

/** Thay literal màu theo hàm map — giữ nguyên định dạng (hex ↔ hex, bộ ba ↔ bộ ba). */
export function mapColors(value: string, map: (rgb: Rgb, text: string) => Rgb | null): string {
  const spans = colorSpans(value)
  if (spans.length === 0) return value
  let out = ''; let pos = 0
  for (const s of spans) {
    const next = map(s.rgb, s.text)
    if (next == null) continue
    out += value.slice(pos, s.start)
    out += s.text.startsWith('#')
      ? toHex(next)
      : `${Math.round(next.r)},${s.text.includes(', ') ? ' ' : ''}${Math.round(next.g)},${s.text.includes(', ') ? ' ' : ''}${Math.round(next.b)}`
    pos = s.end
  }
  return out + value.slice(pos)
}
