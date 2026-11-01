// Thanh tab + thanh tiêu đề điện thoại dùng chung (Noverse dựng trước, Viderse dùng lại).
//
// Hành vi: chế độ `default | mini | hide`, ẩn hẳn khi bàn phím mở, vạch chỉ báo lạc quan
// theo cú chạm (đặt `--tab-index` ngay, nhả sau 550ms nếu route không đổi), nút thật
// (`<button>`) + điều hướng. App truyền icon theo tên file SVG của kit, hoặc slot class
// để giữ bộ da riêng (Viderse: viên kính + FAB đỏ).
import { useEffect, useRef, type CSSProperties, type ReactNode } from 'react'
import { Icon } from '../Icon'
import type { TabMode } from './safeArea'

const RELEASE_MS = 550

export type MobileTabItem = {
  key: string
  label?: string
  /** tên icon kit (không hậu tố); tab đang chọn tự dùng bản `-bulk` nếu không có `iconActive` */
  icon?: string
  iconActive?: string
  href?: string
  /** đích riêng thay cho `href` (router của app) */
  onSelect?: () => void
  /** huy hiệu số việc — node do app vẽ */
  badge?: ReactNode
  /** chấm đỏ "có việc cần bạn" (không hiện khi đang ở tab đó) */
  dot?: boolean
  dotLabel?: string
  disabled?: boolean
  ariaLabel?: string
  /** ô giữa dành cho nút hành động (FAB) — nội dung lấy từ `centerAction` */
  center?: boolean
  className?: string
  activeClassName?: string
}

export type MobileTabBarSlots = {
  root?: string
  /** vỏ kính bọc các ô (Viderse) — có slot này thì vạch chỉ báo nằm trong vỏ */
  cap?: string
  indicator?: string
  item?: string
  itemActive?: string
  icon?: string
  iconActive?: string
  label?: string
  labelActive?: string
  badge?: string
  dot?: string
  center?: string
}

export type MobileTabBarProps = {
  items: readonly MobileTabItem[]
  activeKey?: string
  /** vị trí đang chọn; mặc định suy từ `activeKey` */
  index?: number
  mode?: TabMode
  onSelect?: (item: MobileTabItem, index: number) => void
  /** nội dung ô giữa (nút video mới của Viderse) */
  centerAction?: ReactNode
  slots?: MobileTabBarSlots
  ariaLabel?: string
  className?: string
  style?: CSSProperties
  iconSize?: number
  iconSizeActive?: number
}

const cn = (...parts: (string | false | null | undefined)[]) => parts.filter(Boolean).join(' ')

function navigateHash(href: string) {
  if (typeof window !== 'undefined' && window.location.hash !== href) window.location.hash = href
}

export function MobileTabBar({
  items, activeKey, index, mode = 'default', onSelect, centerAction, slots, ariaLabel = 'Điều hướng chính',
  className, style, iconSize = 24, iconSizeActive,
}: MobileTabBarProps) {
  const ref = useRef<HTMLElement>(null)
  const release = useRef<number | undefined>(undefined)
  const activeIndex = index ?? Math.max(0, items.findIndex((item) => item.key === activeKey))
  const committed = useRef(activeIndex)
  committed.current = activeIndex
  const hidden = mode === 'hide'
  useEffect(() => () => window.clearTimeout(release.current), [])

  const select = (item: MobileTabItem, position: number) => {
    if (hidden || item.disabled) return
    // Vạch chỉ báo lạc quan theo cú chạm ngay; React vẫn giữ tab thật cho tới khi route đổi.
    if (committed.current !== position) {
      ref.current?.style.setProperty('--tab-index', String(position))
      ref.current?.style.setProperty('--i', String(position))
    }
    if (item.onSelect) item.onSelect()
    else if (onSelect) onSelect(item, position)
    else if (item.href) navigateHash(item.href)
    window.clearTimeout(release.current)
    release.current = window.setTimeout(() => {
      if (!ref.current || committed.current === position) return
      ref.current.style.setProperty('--tab-index', String(committed.current))
      ref.current.style.setProperty('--i', String(committed.current))
    }, RELEASE_MS)
  }

  const buttons = items.map((item, position) => {
    const active = activeIndex === position
    const icon = active && item.iconActive ? item.iconActive : item.icon
    return <button
      key={item.key}
      type="button"
      className={cn(item.center ? 'm-tab-center' : 'm-tab', slots?.item, item.center && slots?.center, active && slots?.itemActive, item.className, active && item.activeClassName)}
      aria-current={active && !item.center ? 'page' : undefined}
      aria-label={item.ariaLabel ?? item.label}
      disabled={item.disabled}
      tabIndex={hidden ? -1 : 0}
      onClick={() => select(item, position)}
    >
      {item.center ? centerAction : <>
        {icon && <Icon className={cn('icon', active ? slots?.iconActive : slots?.icon)} name={icon} size={active ? iconSizeActive ?? iconSize : iconSize} />}
        {item.label && <span className={cn('m-tab-label', slots?.label, active && slots?.labelActive)}>{item.label}</span>}
        {item.badge != null && item.badge !== false && <span className={cn('m-badge', slots?.badge)}>{item.badge}</span>}
        {item.dot && !active && <span className={cn('m-dot', slots?.dot)} aria-label={item.dotLabel ?? 'Có việc cần bạn'} />}
      </>}
    </button>
  })

  return <nav
    ref={ref}
    className={cn('m-tabs', mode, slots?.root, className)}
    aria-label={ariaLabel}
    aria-hidden={hidden}
    data-tab-mode={mode}
    style={{ '--tab-index': activeIndex, '--i': activeIndex, ...style } as CSSProperties}
  >
    {slots?.cap
      ? <span className={slots.cap}>{slots.indicator && <span className={slots.indicator} aria-hidden="true" />}{buttons}</span>
      : <>{slots?.indicator && <span className={slots.indicator} aria-hidden="true" />}{buttons}</>}
  </nav>
}

export type MobileTopBarSlots = { root?: string; leading?: string; title?: string; actions?: string; back?: string }

export type MobileTopBarProps = {
  leading?: ReactNode
  title?: ReactNode
  subtitle?: ReactNode
  actions?: ReactNode
  onBack?: () => void
  backLabel?: string
  slots?: MobileTopBarSlots
  className?: string
  style?: CSSProperties
}

/** Thanh trên 52px: trái (back hoặc logo), tiêu đề giữa, hành động phải. Cao/thứ tự do CSS của app. */
export function MobileTopBar({ leading, title, subtitle, actions, onBack, backLabel, slots, className, style }: MobileTopBarProps) {
  return <header className={cn('m-topbar', slots?.root, className)} style={style}>
    {onBack && <button type="button" className={cn('m-back', slots?.back)} aria-label={backLabel ?? 'Quay lại'} onClick={onBack}>
      <Icon name="arrow-left-2" size={22} />
      {backLabel && <span>{backLabel}</span>}
    </button>}
    {leading != null && <div className={slots?.leading}>{leading}</div>}
    {title != null && <div className={cn('m-title', slots?.title)}>{title}{subtitle != null && <small>{subtitle}</small>}</div>}
    {actions != null && <div className={cn('m-actions', slots?.actions)}>{actions}</div>}
  </header>
}
