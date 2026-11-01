// Hiệu ứng vẽ ảnh — 04-components › DevelopEffect (artboard Review, .vv-develop).
//
// Nền = bản trước mờ đậm, vệt sáng chéo màu Hoạ sĩ chạy 2,8 s, hạt nhiễu 10%
// overlay, thẻ trạng thái kính mờ ở giữa (orb, tiêu đề, giờ, 3 bước). Có
// `previewUrl` thì ảnh xem dần hiện mờ 5px. Vẽ xong: lộ ảnh blur(14px)→0,
// scale(1.025)→1 trong 1 s và viền sáng chạy một lần 1,6 s.
// Dải ảnh, Phòng vẽ ở Bàn dựng và ô ảnh trong Editor dùng bản `compact`.
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { corners, space } from '../styles/viderse.stylex'

const sweepFrames = stylex.keyframes({ from: { transform: 'translateX(-35%)' }, to: { transform: 'translateX(35%)' } })
const grainFrames = stylex.keyframes({
  '0%': { transform: 'translate(0, 0)' },
  '33%': { transform: 'translate(-3%, 2%)' },
  '66%': { transform: 'translate(2%, -3%)' },
})
const pulseFrames = stylex.keyframes({ '50%': { boxShadow: '0 0 0 6px var(--vv-develop-clear)' } })
const spinFrames = stylex.keyframes({ to: { transform: 'rotate(360deg)' } })
const revealFrames = stylex.keyframes({
  from: { filter: 'blur(14px) brightness(1.2) saturate(.7)', transform: 'scale(1.025)' },
  to: { filter: 'none', transform: 'none' },
})
const fadeFrames = stylex.keyframes({ from: { opacity: 0 }, to: { opacity: 1 } })
const glintFrames = stylex.keyframes({
  '0%': { boxShadow: 'inset 0 0 0 2px var(--vv-develop-clear)' },
  '25%': { boxShadow: 'inset 0 0 0 2px var(--vv-develop-glint), 0 0 48px var(--vv-develop-glow)' },
  '100%': { boxShadow: 'inset 0 0 0 2px var(--vv-develop-clear)' },
})

export const sx = stylex.create({
  root: { position: 'absolute', inset: 0, overflow: 'hidden', pointerEvents: 'none' },
  base: { position: 'absolute', inset: -24, width: 'calc(100% + 48px)', height: 'calc(100% + 48px)', objectFit: 'cover', filter: 'blur(16px) saturate(.7) brightness(.72)', transition: 'filter .9s cubic-bezier(.2,.8,.2,1)' },
  basePartial: { filter: 'blur(5px) saturate(.9) brightness(.85)' },
  baseCompact: { inset: -6, width: 'calc(100% + 12px)', height: 'calc(100% + 12px)', filter: 'blur(5px) saturate(.5) brightness(.55)' },
  preview: { position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', filter: 'blur(5px)' },
  sweep: { position: 'absolute', inset: '-60% -60%', backgroundImage: 'var(--vv-develop-sweep)', animationName: sweepFrames, animationDuration: '2.8s', animationTimingFunction: 'cubic-bezier(.45,0,.25,1)', animationIterationCount: 'infinite' },
  grain: { backgroundImage: 'var(--vv-develop-grain)', position: 'absolute', inset: '-10%', opacity: .1, mixBlendMode: 'overlay', animationName: grainFrames, animationDuration: '.9s', animationTimingFunction: 'steps(3)', animationIterationCount: 'infinite' },
  card: { position: 'absolute', left: '50%', top: '50%', transform: 'translate(-50%, -50%)', minWidth: 300, paddingBlock: space['--vv-s-14'], paddingInline: space['--vv-s-16'], borderRadius: corners['--vv-r-14'], backgroundColor: 'var(--vv-glass)', borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], backdropFilter: 'blur(10px)', display: 'flex', flexDirection: 'column', gap: space['--vv-s-10'], pointerEvents: 'auto' },
  cardInline: { position: 'absolute', left: space['--vv-s-8'], right: space['--vv-s-8'], bottom: space['--vv-s-8'], paddingBlock: space['--vv-s-8'], paddingInline: space['--vv-s-10'], borderRadius: corners['--vv-r-10'], backgroundColor: 'var(--vv-glass)', display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'], pointerEvents: 'auto' },
  cardInlineHead: { display: 'flex', alignItems: 'center', gap: space['--vv-s-8'], fontSize: 12.5, fontWeight: 600, color: color['--color-fg'] },
  cardInlineTimer: { marginInlineStart: 'auto', fontWeight: 400, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  stepsInline: { display: 'flex', gap: space['--vv-s-6'], flexWrap: 'wrap' },
  cardHead: { display: 'flex', alignItems: 'center', gap: space['--vv-s-10'] },
  cardText: { flex: '1 1 0', minWidth: 0, display: 'flex', flexDirection: 'column' },
  cardTitle: { fontSize: 14, fontWeight: 600, color: color['--color-fg'] },
  cardSub: { fontSize: 12, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  cancel: { flex: '0 0 auto' },
  steps: { display: 'flex', alignItems: 'center', gap: space['--vv-s-10'] },
  step: { display: 'flex', alignItems: 'center', gap: space['--vv-s-6'], fontSize: 12, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap' },
  stepDone: { color: color['--color-ok'] },
  stepOn: { color: 'var(--vv-artist-ink)' },
  stepDot: { width: space['--vv-s-7'], height: space['--vv-s-7'], borderRadius: '50%', backgroundColor: 'currentColor', opacity: .5 },
  stepDotOn: { opacity: 1, boxShadow: '0 0 0 3px var(--vv-develop-pulse)', animationName: pulseFrames, animationDuration: '1.2s', animationTimingFunction: 'ease-in-out', animationIterationCount: 'infinite' },
  stepLine: { width: space['--vv-s-18'], height: space['--vv-s-1'], backgroundColor: line['--line-3'] },
  hint: { fontSize: 12, color: 'var(--vv-fg-note)', lineHeight: 1.5 },
  ring: { position: 'absolute', right: space['--vv-s-5'], top: space['--vv-s-5'], width: space['--vv-s-14'], height: space['--vv-s-14'], borderRadius: '50%', borderWidth: space['--vv-s-2'], borderStyle: 'solid', borderColor: 'var(--vv-develop-ring)', borderTopColor: 'var(--vv-artist-ink)', animationName: spinFrames, animationDuration: '.9s', animationTimingFunction: 'linear', animationIterationCount: 'infinite' },
  reveal: { position: 'absolute', inset: 0, display: 'block', animationName: revealFrames, animationDuration: '1s', animationTimingFunction: 'cubic-bezier(.2,.8,.2,1)', animationFillMode: 'both' },
  glint: { position: 'absolute', inset: 0, borderRadius: 'inherit', pointerEvents: 'none', animationName: glintFrames, animationDuration: '1.6s', animationTimingFunction: 'ease-out', animationDelay: '.25s', animationFillMode: 'both' },
  /** prefers-reduced-motion: vệt sáng, hạt, nhấp nháy, vòng chờ đứng yên. */
  still: { animationName: 'none' },
  /** prefers-reduced-motion: lộ ảnh thành đổi tức thì (chỉ mờ dần 0,2 s). */
  revealStill: { animationName: fadeFrames, animationDuration: '.2s' },
  /** Không vẽ hạt nhiễu khi giảm chuyển động. */
  hidden: { display: 'none' },
})
