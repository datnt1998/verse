// ChannelAvatar — avatar kênh, luôn tròn (Luật hình). Đã có ảnh → ảnh;
// chưa có → hình tròn phẳng với 2 chữ cái đầu. Kênh của tôi: nền tối ám màu
// kênh + viền 1.5px màu kênh; đối thủ: nền đặc không viền (04-components).
// Màu kênh là dữ liệu (mỗi kênh một tint/ring trong DB) nên nhận qua prop.
import type { CSSProperties } from 'react'

export function ChannelAvatar({ name, initials, src, color, size = 26, stacked = false, ringWidth = 1.5, ringColor = 'var(--color-surface-1)', fontSize, style, className }: {
  name: string
  /** 2 chữ cái đầu — mặc định lấy từ `name` */
  initials?: string
  /** URL ảnh avatar kênh khi đã có */
  src?: string
  /** Màu kênh { bg, ring } — đối thủ truyền chỉ `bg` (nền đặc, không viền) */
  color?: { bg: string; ring?: string }
  size?: number
  /** chồng lên ảnh/avatar khác → viền tách màu nền */
  stacked?: boolean
  /** bề dày vòng tách khi chồng (thanh trên điện thoại dùng 2px, máy tính 1.5px) */
  ringWidth?: number
  /** màu vòng tách — phải trùng nền chỗ đặt (thanh dùng surface-1, menu surface-2) */
  ringColor?: string
  /** cỡ chữ chữ cái; mặc định theo cỡ avatar */
  fontSize?: number
  style?: CSSProperties
  /** lớp ngoài (stylex) — chồng avatar trong bộ đổi kênh dùng để lệch âm */
  className?: string
}) {
  const letters = initials ?? name.trim().split(/\s+/).map((w) => w[0]).join('').slice(0, 2).toUpperCase()
  const base: CSSProperties = {
    width: size, height: size, borderRadius: '50%', flex: '0 0 auto',
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    fontSize: fontSize ?? Math.max(7.5, size * 0.36), fontWeight: 600, lineHeight: 1,
    color: 'var(--color-fg)', overflow: 'hidden', boxSizing: 'border-box',
    ...(stacked ? { boxShadow: `0 0 0 ${ringWidth}px ${ringColor}` } : {}),
    ...style,
  }
  if (src) {
    return (
      <span role="img" aria-label={name} className={className} style={{
        ...base,
        // Nền kênh nằm dưới ảnh: ảnh chưa tải hoặc lỗi vẫn thấy hình tròn.
        backgroundColor: color?.bg ?? 'var(--color-surface-2)',
        backgroundImage: `url(${JSON.stringify(src)})`, backgroundSize: 'cover', backgroundPosition: 'center',
        border: color?.ring ? `1.5px solid ${color.ring}` : undefined,
      }} />
    )
  }
  return (
    <span role="img" aria-label={name} className={className} style={{
      ...base,
      background: color?.bg ?? 'var(--color-surface-2)',
      border: color?.ring ? `1.5px solid ${color.ring}` : `1.5px solid var(--line-2)`,
    }}>
      {letters}
    </span>
  )
}
