// MainNav — 5 mục giữa thanh đầu: Bàn dựng · Duyệt (badge vàng đếm chờ duyệt) ·
// Kênh · Khám phá · Video mới. Cuộn ngang khi chỗ nhìn hẹp (04-components §32;
// (04-components › AppHeader): scrollbar ẩn, mục co lại icon + chữ.
// Kiểu: styles/components.css .vv-nav*.
import type { MouseEvent } from 'react'
import { navSection, paths, type Route } from '../router'
import { t } from '../i18n/vi'
import { Icon, type IconName } from '@noverse/ui'
import * as stylex from '@stylexjs/stylex'
const sx = stylex.create({
  item: { paddingLeft: 10, paddingRight: 12, gap: 8 },
  review: { paddingRight: 10 },
})

const ITEMS: { key: ReturnType<typeof navSection>; label: string; icon: IconName; path: string; badge?: 'review' }[] = [
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
    <nav role="navigation" aria-label={t.nav.main} className="vv-nav">
      {ITEMS.map((item) => {
        const active = item.key === section
        const path = item.key === 'channels' ? channelHref ?? item.path : item.path
        return (
          <a
            key={item.key}
            href={`#${path}`}
            aria-current={active ? 'page' : undefined}
            onClick={onNavigate ? (e) => onNavigate(path, e) : undefined}
            className={`${active ? 'vv-nav__item vv-nav__item--active' : 'vv-nav__item'} ${stylex.props(sx.item, item.badge === 'review' && sx.review).className}`}
          >
            <Icon name={item.icon} size={16} />
            {item.label}
            {item.badge === 'review' && reviewCount > 0 && (
              <span className="vv-nav__badge num" aria-label={`${reviewCount}`}>
                {reviewCount > 99 ? '99+' : reviewCount}
              </span>
            )}
          </a>
        )
      })}
    </nav>
  )
}
