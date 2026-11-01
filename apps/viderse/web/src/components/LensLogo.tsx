// Logo ống kính (12-brand / artboard Brand): cầu đỏ + 4 cung màu trợ lý,
// mỗi cung ~19% vòng lệch nhau 25%. Bản nhỏ (≤30px) cung dày hơn, không vòng kính.
// Xoay 9s khi `spinning` (có job chạy); prefers-reduced-motion tắt ở styles/viderse.css.
import { useId } from 'react'
import * as stylex from '@stylexjs/stylex'

const lensRot = stylex.keyframes({ to: { transform: 'rotate(360deg)' } })
const sx = stylex.create({
  spinning: {
    transformBox: 'fill-box',
    transformOrigin: 'center',
    animationName: lensRot,
    animationDuration: '9s',
    animationTimingFunction: 'linear',
    animationIterationCount: 'infinite',
  },
})

const ARCS: { color: string; offset: number }[] = [
  { color: '#7FA3FF', offset: 0 },    // Trinh sát
  { color: '#B08AF7', offset: -25 },  // Đạo diễn
  { color: '#FFBE66', offset: -50 },  // Hoạ sĩ
  { color: '#5DDDB0', offset: -75 },  // Kiểm định
]

export function LensLogo({ size = 26, spinning = false, light = false, halo = false, hero = false }: {
  size?: number
  /** xoay 9s mỗi vòng — chỉ khi có job chạy */
  spinning?: boolean
  /** bản cho nền sáng (ít dùng — app chỉ có theme tối) */
  light?: boolean
  /** quầng đỏ mờ sau cầu — bản hero của màn chào/đăng nhập (brand/mark-hero) */
  halo?: boolean
  /** bản hero không vòng kính: cung dày 4,6 thay vì 2,6 (Onboard › step=setup) */
  hero?: boolean
}) {
  const id = useId().replace(/[^a-zA-Z0-9]/g, '')
  const small = size <= 30
  // Hero của Onboard: cung dày 4,6, không vòng kính, cầu 16 — đúng SVG artboard.
  const stroke = hero ? 4.6 : small ? (size <= 18 ? 7 : 5.4) : 2.6
  const arcR = small ? (size <= 18 ? 26 : 27) : 27
  const sphereR = hero ? 16 : small ? (size <= 18 ? 15 : 16) : 14.5
  const dash = hero || small ? '17 83' : '19 81'
  const arcColors = light ? ['#4F78F0', '#8A5BE8', '#E8922E', '#21A983'] : ARCS.map((a) => a.color)
  const mark = (
    <svg aria-hidden="true" width={size} height={size} viewBox="0 0 64 64" style={{ flex: '0 0 auto' }}>
      <defs>
        <radialGradient id={`lnSph-${id}`} cx=".34" cy=".28" r=".75">
          <stop offset="0" stopColor="#FFE6EA" />
          <stop offset=".22" stopColor="#FF8C9C" />
          <stop offset=".55" stopColor="#FF1F45" />
          <stop offset="1" stopColor="#A3001C" />
        </radialGradient>
        {!small && !hero && (
          <linearGradient id={`lnGlass-${id}`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0" stopColor="#FFFFFF" stopOpacity=".35" />
            <stop offset=".5" stopColor="#FFFFFF" stopOpacity=".04" />
            <stop offset="1" stopColor="#FFFFFF" stopOpacity=".18" />
          </linearGradient>
        )}
      </defs>
      {!small && !hero && <circle cx="32" cy="32" r="21.5" fill={light ? `url(#lnGlass-${id})` : '#141417'} stroke={light ? '#D9D9E0' : `url(#lnGlass-${id})`} strokeWidth=".8" />}
      <g {...stylex.props(spinning && sx.spinning)}>
        <g transform="rotate(-80 32 32)" fill="none" strokeWidth={stroke} strokeLinecap="round">
          {arcColors.map((color, i) => (
            <circle key={color} cx="32" cy="32" r={arcR} pathLength={100} stroke={color} strokeDasharray={dash} strokeDashoffset={ARCS[i].offset} />
          ))}
        </g>
      </g>
      <circle cx="32" cy="32" r={sphereR} fill={`url(#lnSph-${id})`} />
      {!small && !hero && <ellipse cx="27.5" cy="26" rx="4.2" ry="2.4" transform="rotate(-35 27.5 26)" fill="#FFFFFF" fillOpacity=".55" />}
    </svg>
  )
  if (!halo && !hero) return mark
  // Bản hero (brand/svg/viderse-mark-hero): quầng đỏ mờ loãng phía sau cầu.
  return (
    <span aria-hidden="true" style={{ position: 'relative', display: 'inline-flex', flex: '0 0 auto' }}>
      <span
        aria-hidden="true"
        style={{
          position: 'absolute', inset: `-${Math.round(size * 0.55)}px`, borderRadius: '50%',
          background: 'radial-gradient(circle, var(--vv-red-18) 0%, transparent 70%)',
        }}
      />
      {mark}
    </span>
  )
}
