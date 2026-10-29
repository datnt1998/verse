// BrandMark — bốn quả cầu chụm lại (23-brand.md §1, §4). Hình lấy từ brand/svg/noverse-mark*.svg,
// cắt đúng khung nhìn thấy được (bỏ khoảng trống 24 đơn vị của tệp gốc) để `size` là chiều cao thật.
// Gradient/mask mang id riêng của từng lần dùng: hai biểu tượng trên một trang không tranh id của nhau.
import { useId, type CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import { brand, font } from './tokens.stylex'
import { getKitLabels } from './labels'

/** Khung nhìn thấy của biểu tượng: 560×410 đơn vị (cao 22px → rộng 30px; artboard vẽ 30,2px). */
const ASPECT = 560 / 410

const sx = stylex.create({
  mark: { display: 'block', flexShrink: 0, flexGrow: 0 },
  lockup: { display: 'flex', alignItems: 'center', flex: '0 0 auto' },
  word: { display: 'inline-flex', alignItems: 'baseline', gap: 6, lineHeight: 1 },
  noverse: { fontFamily: font['--font-brand'], fontSize: 16, fontWeight: 700, letterSpacing: '-0.012em' },
  studio: { fontSize: 13, fontWeight: 500, color: 'var(--color-fg-4)' },
})

export type BrandMarkVariant = 'color' | 'flat' | 'mono'

type Sphere = { key: string; cx: number; cy: number; r: number; stops: readonly string[] }

/** Thứ tự vẽ từ dưới lên: Kiến trúc · Người viết · Biên tập · Trọng tài (23-brand.md §1).
    Màu (sáng → tối) là token thương hiệu trong tokens.stylex.ts. */
const SPHERES: Sphere[] = [
  {
    key: 'b', cx: 115, cy: 206, r: 114,
    stops: [brand['--brand-architect-1'], brand['--brand-architect-2'], brand['--brand-architect-3'], brand['--brand-architect-4']],
  },
  {
    key: 'p', cx: 281, cy: 134, r: 134,
    stops: [brand['--brand-writer-1'], brand['--brand-writer-2'], brand['--brand-writer-3'], brand['--brand-writer-4']],
  },
  {
    key: 'o', cx: 447, cy: 216, r: 114,
    stops: [brand['--brand-editor-1'], brand['--brand-editor-2'], brand['--brand-editor-3'], brand['--brand-editor-4']],
  },
  {
    key: 'g', cx: 281, cy: 308, r: 102,
    stops: [brand['--brand-arbiter-1'], brand['--brand-arbiter-2'], brand['--brand-arbiter-3'], brand['--brand-arbiter-4']],
  },
]

/** Khe hở trong suốt của bản một màu: quả sau che quả trước (toạ độ tệp gốc, đã bù 24). */
const MONO_MASKS = [
  { key: 'b', holes: [[281, 134, 145], [447, 216, 125], [281, 308, 113]] },
  { key: 'p', holes: [[447, 216, 125], [281, 308, 113]] },
  { key: 'o', holes: [[281, 308, 113]] },
]

export function BrandMark({ size, variant = 'color', label, style }: {
  /** chiều cao biểu tượng, px */
  size: number
  /** `mono` dùng currentColor; mặc định tự chuyển sang bản phẳng khi size < 32 */
  variant?: BrandMarkVariant
  /** đứng một mình thì truyền nhãn (role=img); cạnh chữ “Noverse” thì bỏ trống (aria-hidden) */
  label?: string
  style?: CSSProperties
}) {
  const bare = useId().replace(/[^a-zA-Z0-9_-]/g, '')
  const shape = variant === 'color' && size < 32 ? 'flat' : variant
  const mark = stylex.props(sx.mark)
  const box = { ...mark.style, height: size, width: size * ASPECT, ...style }
  const a11y = label
    ? { role: 'img' as const, 'aria-label': label }
    : { 'aria-hidden': true as const }
  const svg = { className: mark.className, style: box, ...a11y }
  if (shape === 'mono') return (
    <svg viewBox="1 0 560 410" fill="currentColor" {...svg}>
      <defs>
        {MONO_MASKS.map((mask) => (
          <mask key={mask.key} id={`${bare}mm${mask.key}`} maskUnits="userSpaceOnUse" x="-20" y="-20" width="602" height="450">
            <rect x="-20" y="-20" width="602" height="450" fill="white" />
            {mask.holes.map(([cx, cy, r]) => <circle key={`${cx}-${cy}`} cx={cx} cy={cy} r={r} fill="black" />)}
          </mask>
        ))}
      </defs>
      {SPHERES.map((sphere) => (
        <circle key={sphere.key} cx={sphere.cx} cy={sphere.cy} r={sphere.r}
          mask={sphere.key === 'g' ? undefined : `url(#${bare}mm${sphere.key})`} />
      ))}
    </svg>
  )

  const flat = shape === 'flat'
  return (
    <svg viewBox="1 0 560 410" overflow="visible" {...svg}>
      <defs>
        {SPHERES.map((sphere) => (
          <radialGradient key={sphere.key} id={`${bare}m${sphere.key}`} cx=".34" cy=".28" fx=".34" fy=".28" r=".95">
            {(flat ? sphere.stops.slice(1) : sphere.stops).map((stop, i) => (
              <stop key={stop} offset={flat ? ['0', '0.55', '1'][i] : ['0', '0.22', '0.58', '1'][i]}
                style={{ stopColor: stop }} />
            ))}
          </radialGradient>
        ))}
        {!flat && (
          <>
            <radialGradient id={`${bare}mrim`} cx=".42" cy=".38" r=".66">
              <stop offset=".62" stopOpacity="0" style={{ stopColor: brand['--brand-rim'] }} />
              <stop offset="1" stopOpacity=".30" style={{ stopColor: brand['--brand-rim'] }} />
            </radialGradient>
            <radialGradient id={`${bare}mhl`} cx=".34" cy=".27" r=".24">
              <stop offset="0" stopColor="white" stopOpacity=".35" />
              <stop offset="1" stopColor="white" stopOpacity="0" />
            </radialGradient>
            <filter id={`${bare}msh`} x="-30%" y="-30%" width="160%" height="170%">
              <feDropShadow dx="0" dy="10" stdDeviation="12" style={{ floodColor: brand['--brand-shadow'] }} floodOpacity=".32" />
            </filter>
          </>
        )}
      </defs>
      {SPHERES.map((sphere) => {
        const lit = !flat && sphere.key !== 'b'
        const body = (
          <>
            <circle cx={sphere.cx} cy={sphere.cy} r={sphere.r} fill={`url(#${bare}m${sphere.key})`} />
            {!flat && <circle cx={sphere.cx} cy={sphere.cy} r={sphere.r} fill={`url(#${bare}mrim)`} />}
            {!flat && <circle cx={sphere.cx} cy={sphere.cy} r={sphere.r} fill={`url(#${bare}mhl)`} />}
          </>
        )
        // Chỉ ba quả sau đổ bóng — quả trái nằm dưới cùng nên không cần (tệp gốc).
        return lit ? <g key={sphere.key} filter={`url(#${bare}msh)`}>{body}</g> : <g key={sphere.key}>{body}</g>
      })}
    </svg>
  )
}

/** Logo thanh trên cùng: biểu tượng + “Noverse” (+ “Studio” khi artboard vẽ). */
export function BrandLockup({ size = 22, gap = 10, studio = false, style }: {
  size?: number
  /** khoảng cách biểu tượng → chữ (artboard: 10 ở Login/Setup, 14 ở Onboard, 8 ở điện thoại) */
  gap?: number
  studio?: boolean
  style?: CSSProperties
}) {
  return (
    <span className={stylex.props(sx.lockup).className} style={{ gap, ...style }}>
      <BrandMark size={size} />
      <span className={stylex.props(sx.word).className}>
        <span className={stylex.props(sx.noverse).className}>{getKitLabels().brand.name}</span>
        {studio && <span className={stylex.props(sx.studio).className}>{getKitLabels().brand.studio}</span>}
      </span>
    </span>
  )
}
