// Icon: render SVG trong src/icons (chép từ docs/design/v1/icons).
// Màu theo currentColor qua mask-image — giống cách artboard dùng .ic + .i-*.
// Tên: 'category' (linear) | 'category-bulk'. variant='bulk' tự thêm hậu tố.
// Tên không có SVG: bản dev báo lỗi lớn trong console và vẽ ô đỏ gạch đứt (không im lặng hiện vòng tròn);
// Icon.test.tsx quét mã nguồn để mọi tên viết thẳng trong src đều phải có file.
import type { CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { IconName } from './iconNames'

const urls = import.meta.glob('./icons/*.svg', { eager: true, query: '?url', import: 'default' }) as Record<string, string>
const byName = new Map<string, string>()
for (const [path, url] of Object.entries(urls)) {
  byName.set(path.slice(path.lastIndexOf('/') + 1, -4), url)
}

export type IconProps = {
  /** tên file không đuôi, vd 'category' hoặc 'category-bulk' — union IconName nhưng hở: tên bất kỳ vẫn chạy */
  name: IconName | (string & {})
  variant?: 'linear' | 'bulk'
  size?: number
  /** nhãn a11y; để trống/undefined → aria-hidden */
  label?: string
  style?: CSSProperties
  className?: string
  xstyle?: stylex.StyleXStyles
}

const sx = stylex.create({
  base: (size: string) => ({
    display: 'inline-block', flex: '0 0 auto', width: size, height: size,
    backgroundColor: 'currentColor',
    WebkitMaskPosition: 'center', maskPosition: 'center',
    WebkitMaskSize: 'contain', maskSize: 'contain',
    WebkitMaskRepeat: 'no-repeat', maskRepeat: 'no-repeat',
  }),
})

const reported = new Set<string>()
/** Dev: mỗi tên thiếu chỉ báo một lần để console không ngập, nhưng ô đỏ vẫn hiện ở mọi chỗ dùng. */
function reportMissing(name: string) {
  if (!import.meta.env.DEV || reported.has(name)) return
  reported.add(name)
  console.error(`[Icon] Missing icon "${name}" — add the SVG to src/icons (docs/design/v1/icons) or fix the name.`)
}

export function Icon({ name, variant, size = 18, label, style, className, xstyle }: IconProps) {
  const file = variant === 'bulk' && !name.endsWith('-bulk') ? `${name}-bulk` : name
  const url = byName.get(file) ?? byName.get(name)
  if (!url) reportMissing(name)
  // URL phải nằm trong dấu nháy (data URI SVG chứa ký tự đặc biệt); ô đỏ dev giữ literal để test markup nhận diện.
  const missing: CSSProperties = import.meta.env.DEV
    ? { background: 'var(--kit-dev-miss-bg)', borderRadius: 4, border: '1px dashed var(--kit-dev-miss)' }
    : { background: 'transparent', borderRadius: '50%', border: '1px dashed currentColor' }
  const p = stylex.props(sx.base(`${size}px`), xstyle)
  const cls = [p.className, className].filter(Boolean).join(' ')
  const merged = {
    ...p.style,
    ...(url ? { WebkitMaskImage: `url(${JSON.stringify(url)})`, maskImage: `url(${JSON.stringify(url)})` } : missing),
    ...style,
  }
  const title = !url && import.meta.env.DEV ? `Missing icon: ${name}` : undefined
  if (label) return <span className={cls} role="img" aria-label={label} title={title} style={merged} />
  return <span className={cls} aria-hidden="true" title={title} style={merged} />
}

/** kiểm tra tên icon tồn tại (dùng chọn biến thể bulk khi có) */
export const hasIcon = (name: string) => byName.has(name)
