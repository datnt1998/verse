// MainNav — 5 mục giữa thanh đầu: Bàn dựng · Duyệt (badge vàng đếm chờ duyệt) ·
// Kênh · Khám phá · Video mới. Cuộn ngang khi chỗ nhìn hẹp (04-components §32;
// (04-components › AppHeader): scrollbar ẩn, mục co lại icon + chữ.
// Kiểu: stylex.create bên dưới (trước ở styles/components.css .vv-nav*).
import type { MouseEvent } from 'react'
import { navSection, paths, type NavKey, type Route } from '../router'
import { t } from '../i18n/vi'
import { Icon, type IconName } from '@noverse/ui'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
const sx = stylex.create({
  nav: {
    display: 'flex',
    gap: 'var(--vv-s-2)',
    flex: '0 1 auto',
    minWidth: 0,
    padding: 'var(--vv-s-3)',
    borderRadius: 'var(--vv-r-10)',
    backgroundColor: color['--color-surface-1'],
    borderWidth: 'var(--vv-s-1)',
    borderStyle: 'solid',
    borderColor: line['--line-1'],
    overflowX: 'auto',
    scrollbarWidth: 'none',
    whiteSpace: 'nowrap',
  },
  item: {
    position: 'relative',
    display: 'inline-flex',
    alignItems: 'center',
    height: 30,
    borderRadius: 'var(--vv-r-7)',
    fontSize: 13,
    textDecoration: 'none',
    color: { default: color['--color-fg-3'], ':hover': color['--color-fg'] },
    paddingLeft: 'var(--vv-s-10)',
    paddingRight: 'var(--vv-s-12)',
    gap: 'var(--vv-s-8)',
  },
  itemActive: {
    fontWeight: 500,
    color: { default: color['--color-fg'], ':hover': color['--color-fg'] },
    backgroundColor: color['--color-surface-6'],
  },
  review: { paddingRight: 'var(--vv-s-10)' },
  /** `.vv-badge` của kit (#14): chữ vàng --vv-st-review trên nền vàng nhạt 16%. */
  badge: {
    minWidth: 18,
    height: 18,
    paddingInline: 'var(--vv-s-5)',
    borderRadius: 'var(--vv-r-pill)',
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: 11,
    fontWeight: 600,
    fontVariantNumeric: 'tabular-nums',
    backgroundColor: 'var(--vv-badge-bg)',
    color: color['--color-warn'],
  },
})

const ITEMS: { key: NavKey; label: string; icon: IconName; path: string; badge?: 'review' }[] = [
  { key: 'workbench', label: t.nav.workbench, icon: 'workbench', path: paths.workbench() },
  { key: 'review', label: t.nav.review, icon: 'gallery', path: paths.review(), badge: 'review' },
  { key: 'channels', label: t.nav.channels, icon: 'channel', path: paths.channelNew() },
  { key: 'explore', label: t.nav.explore, icon: 'radar', path: paths.explore() },
  { key: 'new', label: t.nav.new, icon: 'video-add', path: paths.new() },
]

export function MainNav({ route, reviewCount, channelHref, onNavigate }: {
  route: Route
  /** số video chờ duyệt → badge vàng trên Duyệt */
  reviewCount: number
  /** đích mục Kênh: kênh đang xem gần nhất, hoặc '#/channels/new' khi chưa có kênh */
  channelHref?: string
  onNavigate?: (path: string, e: MouseEvent) => void
}) {
  const section = navSection(route) // null ở trang ngoài nav → không mục nào sáng
  return (
    <nav role="navigation" aria-label={t.nav.main} {...stylex.props(sx.nav)}>
      {ITEMS.map((item) => {
        const active = item.key === section
        const path = item.key === 'channels' ? channelHref ?? item.path : item.path
        return (
          <a
            key={item.key}
            href={`#${path}`}
            aria-current={active ? 'page' : undefined}
            onClick={onNavigate ? (e) => onNavigate(path, e) : undefined}
            {...stylex.props(sx.item, item.badge === 'review' && sx.review, active && sx.itemActive)}
          >
            <Icon name={item.icon} size={16} />
            {item.label}
            {item.badge === 'review' && reviewCount > 0 && (
              <span className={`vv-nav__badge num ${stylex.props(sx.badge).className}`} aria-label={`${reviewCount}`}>
                {reviewCount > 99 ? '99+' : reviewCount}
              </span>
            )}
          </a>
        )
      })}
    </nav>
  )
}
