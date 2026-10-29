// ThinkingOrb: quả cầu 150 chấm Fibonacci — “đang nghĩ” cuối luồng chữ (17-kit.md › ThinkingOrb).
// Port nguyên startOrbit()/canvas.orbit-cv của artboards/Main.dc.html: phối cảnh F=3.8, trục
// nghiêng −20° (−0.36 rad), quay 0.35 rad/s (~18s/vòng), sóng sáng lan 1 nhịp/(1/0.55)s từ điểm
// lang thang, chấm trên sóng nhô 7% và sáng lên. Không quầng, không vòng sáng, không vệt đuôi.
// ~30fps (vẽ khi now−last>30), DPR ≤ 3, dừng khi ngoài màn hình hoặc tab ẩn,
// prefers-reduced-motion → một khung đứng yên (t=1.2).
import { useEffect, useRef, type CSSProperties } from 'react'
import { getKitLabels } from './labels'

const N = 150
const GA = Math.PI * (3 - Math.sqrt(5))
const POINTS: [number, number, number][] = []
for (let i = 0; i < N; i++) {
  const y = 1 - (2 * (i + 0.5)) / N
  const r = Math.sqrt(1 - y * y)
  const th = GA * i
  POINTS.push([Math.cos(th) * r, y, Math.sin(th) * r])
}

// Màu lấy từ token --orb-dot-back/front (tokens.stylex.ts) qua getComputedStyle; kênh dự phòng
// khi môi trường không đọc được CSS var (test). Mặt sau 150·150·255 mờ 0.1 · mặt trước 240·222·255 rõ ~0.85.
const FALLBACK_BACK: [number, number, number] = [150, 150, 255]
const FALLBACK_FRONT: [number, number, number] = [240, 222, 255]

const channels = (value: string): [number, number, number] | null => {
  const m = /^\s*#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})\s*$/i.exec(value)
  return m ? [parseInt(m[1], 16), parseInt(m[2], 16), parseInt(m[3], 16)] : null
}

const hexOf = (r: number, g: number, b: number): string =>
  '#' + ((1 << 24) | (r << 16) | (g << 8) | b).toString(16).slice(1)

export type ThinkingOrbProps = {
  /** cạnh canvas CSS px — mặc định 22 (inline sau chữ cuối) */
  size?: number
  /** thinking: quay + sóng sáng; idle: chỉ quay chậm */
  state?: 'thinking' | 'idle'
  /** aria-label — mặc định 'Người viết đang viết' */
  label?: string
}

// orbit-cv của Main.dc.html: inline-block, lệch 6px trái, vertical-align -6px.
const orbStyle = (px: number): CSSProperties => ({
  display: 'inline-block', width: px, height: px, marginLeft: 6, verticalAlign: -6,
})

export function ThinkingOrb({ size = 22, state = 'thinking', label }: ThinkingOrbProps) {
  const labelText = label ?? getKitLabels().orbs.thinking
  const ref = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const cv = ref.current
    if (!cv) return
    const g = cv.getContext('2d')
    if (!g) return
    const style = getComputedStyle(cv)
    const back = channels(style.getPropertyValue('--orb-dot-back')) ?? FALLBACK_BACK
    const front = channels(style.getPropertyValue('--orb-dot-front')) ?? FALLBACK_FRONT
    const thinking = state === 'thinking'
    const mq = typeof window.matchMedia === 'function' ? window.matchMedia('(prefers-reduced-motion: reduce)') : null
    const dpr = Math.min(3, window.devicePixelRatio || 1)
    let visible = true
    let raf = 0
    let last = 0
    const draw = (now: number) => {
      const t = (mq ? mq.matches : false) ? 1.2 : now / 1000
      const ay = t * 0.35, cy = Math.cos(ay), sy = Math.sin(ay)
      const tilt = -0.36, cx = Math.cos(tilt), sxv = Math.sin(tilt), F = 3.8
      // tâm sóng đi vòng quanh cầu; idle → không sóng
      const wa = t * 0.7
      const wc: [number, number, number] = [Math.cos(wa) * 0.8, Math.sin(t * 0.45) * 0.6, Math.sin(wa) * 0.8]
      const wl = Math.hypot(wc[0], wc[1], wc[2])
      wc[0] /= wl; wc[1] /= wl; wc[2] /= wl
      const phase = (t * 0.55) % 1
      const css = cv.clientWidth || size
      const W = Math.round(css * dpr)
      if (cv.width !== W) { cv.width = W; cv.height = W }
      const c = W / 2, R = W * 0.4, base = 0.55 * Math.pow(css / 22, 0.45) * dpr
      g.clearRect(0, 0, W, W)
      for (const [x, y, z] of POINTS) {
        let wv = 0
        if (thinking) {
          const ang = Math.acos(Math.max(-1, Math.min(1, x * wc[0] + y * wc[1] + z * wc[2]))) / Math.PI
          wv = Math.exp(-Math.pow(ang - phase, 2) / 0.006) * (1 - phase)
        }
        const k = 1 + 0.07 * wv
        const X = x * k * cy + z * k * sy
        let Z = -x * k * sy + z * k * cy
        const Y = y * k * cx - Z * sxv
        Z = y * k * sxv + Z * cx
        const p = F / (F - Z), d = (Z + 1) / 2
        g.globalAlpha = Math.min(1, 0.1 + 0.75 * d * d + 0.6 * wv * (0.35 + 0.65 * d))
        g.fillStyle = hexOf(
          Math.round(back[0] + (front[0] - back[0]) * d + 15 * wv),
          Math.round(back[1] + (front[1] - back[1]) * d + 20 * wv),
          Math.round(back[2] + (front[2] - back[2]) * d),
        )
        g.beginPath()
        g.arc(c + X * R * p, c + Y * R * p, base * (0.7 + 0.45 * d) * p, 0, 6.2832)
        g.fill()
      }
    }

    const tick = (now: number) => {
      // ngoài màn hình hoặc tab ẩn → bỏ khung hình (mẫu artboard lọc bằng bounding rect)
      if (visible && !document.hidden && now - last > 30) { last = now; draw(now) }
      raf = requestAnimationFrame(tick)
    }
    const io = typeof IntersectionObserver === 'function'
      ? new IntersectionObserver((entries) => { visible = entries[0]?.isIntersecting ?? true })
      : null
    io?.observe(cv)
    // reduced-motion: draw vẫn chạy qua tick nhưng đọc mq.matches trực tiếp → khung tĩnh t=1.2;
    // vẽ một frame ngay để canvas không trống khi chờ rAF đầu.
    draw(0)
    raf = requestAnimationFrame(tick)
    return () => { cancelAnimationFrame(raf); io?.disconnect() }
  }, [state, size])

  return <canvas ref={ref} className="orbit-cv" style={orbStyle(size)} role="img" aria-label={labelText} />
}
