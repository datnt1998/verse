// MobileShell — khung điện thoại <768px (08-dev-prompt V10: thanh dưới 5 mục).
// Thanh trên: logo 24 + bộ đổi kênh trên Bàn dựng + JobsPill gọn + avatar.
// Thanh dưới 60px, kính mờ (vv-glass), 5 mục — mục giữa Video mới là nút đỏ tròn;
// mục đang mở nền pill đỏ nhạt, mục thường icon xám. Nội dung pad-bottom 110px.
// Kiểu: styles/components.css .vv-mbar/.vv-tabbar*/.vv-mavatar.
import type { ReactNode } from 'react'
import { navigate, paths, pickChannelRoute, routePath, navSection, type Route } from '../router'
import { t } from '../i18n/vi'
import { LensLogo } from '../components/LensLogo'
import { Icon, type IconName } from '@noverse/ui'
import { ChannelSwitcher, type SwitcherChannel } from './ChannelSwitcher'
import { JobsPill } from './JobsPill'
import type { Status } from '../api/status'

const TAB_ITEMS: { key: ReturnType<typeof navSection>; label: string; icon: IconName; iconActive: IconName; path: string; badge?: 'review' }[] = [
  { key: 'workbench', label: t.nav.workbench, icon: 'workbench', iconActive: 'workbench-b', path: paths.workbench() },
  { key: 'review', label: t.nav.review, icon: 'flag', iconActive: 'flag', path: paths.review(), badge: 'review' },
  { key: 'new', label: '', icon: 'add', iconActive: 'add', path: paths.new() }, // nút + tròn giữa
  { key: 'channels', label: t.nav.channels, icon: 'channel', iconActive: 'channel-b', path: paths.channelNew() },
  { key: 'explore', label: t.nav.explore, icon: 'radar', iconActive: 'radar-b', path: paths.explore() },
]

export function MobileTabBar({ route, reviewCount, channelHref, fixed = true }: {
  route: Route
  reviewCount: number
  /** đích tab Kênh (kênh gần nhất / tạo kênh khi chưa có) */
  channelHref?: string
  /** false khi nhúng trong khung demo (dev kit) — đứng cuối khung thay vì dán màn hình */
  fixed?: boolean
}) {
  const section = navSection(route)
  return (
    <nav role="navigation" aria-label={t.nav.bottom} className="vv-tabbar" style={fixed ? undefined : { position: 'absolute' }}>
      {TAB_ITEMS.map((item) => {
        const path = item.key === 'channels' ? channelHref ?? item.path : item.path
        if (item.key === 'new') {
          return (
            <a key="new" href={`#${path}`} aria-label={t.nav.new} title={t.nav.new}
              style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', minWidth: 56 }}>
              <span className="vv-tabbar__new"><Icon name="add" size={22} /></span>
            </a>
          )
        }
        const active = item.key === section
        return (
          <a key={item.key} href={`#${path}`} aria-current={active ? 'page' : undefined}
            className={active ? 'vv-tabbar__item vv-tabbar__item--active' : 'vv-tabbar__item'}>
            <span className="vv-tabbar__pill">
              <Icon name={active ? item.iconActive : item.icon} size={20} />
              {item.badge === 'review' && reviewCount > 0 && (
                <span className="vv-tabbar__badge num" aria-label={`${reviewCount}`}>{reviewCount > 99 ? '99+' : reviewCount}</span>
              )}
            </span>
            <span className="vv-tabbar__label">{item.label}</span>
          </a>
        )
      })}
    </nav>
  )
}

export function MobileShell({ route, status, online, userName, channels, channelHref, children }: {
  route: Route
  status: Status | undefined
  online: boolean
  userName?: string
  channels?: SwitcherChannel[]
  /** đích tab Kênh (kênh gần nhất / tạo kênh khi chưa có) */
  channelHref?: string
  children: ReactNode
}) {
  const running = status?.running ?? 0
  const pickChannel = (id: string | undefined) => navigate(routePath(pickChannelRoute(route, id)))
  const showSwitcher = navSection(route) === 'workbench' || route.name === 'channel'
  const isNew = route.name === 'new'
  // MNew artboard: trang con có thanh riêng (× + tiêu đề), không thanh dưới.
  if (isNew) {
    return (
      <div className="vv-mobile-shell vv-mobile-shell--new">
        <header role="banner" className="vv-mbar vv-mbar--new">
          <button type="button" aria-label={t.common.close} className="vv-mclose"
            onClick={() => navigate(paths.workbench())}>
            <Icon name="close" size={18} />
          </button>
          <span className="vv-mnew-title">{t.screens.new.title}</span>
        </header>
        {children}
      </div>
    )
  }
  return (
    <div className="vv-mobile-shell">
      <header role="banner" className="vv-mbar">
        <a href={`#${paths.workbench()}`} aria-label={t.brand.name} style={{ display: 'flex', alignItems: 'center', flex: '0 0 auto' }}>
          <LensLogo size={24} spinning={online && running > 0} />
        </a>
        {showSwitcher && <ChannelSwitcher channels={channels ?? []} current={route.name === 'channel' ? route.channelId : route.name === 'workbench' ? route.channel : undefined} onPick={pickChannel} />}
        <span style={{ flex: 1 }} />
        <JobsPill status={status} online={online} compact unread={(status?.attention.length ?? 0) > 0} />
        <a href={`#${paths.account()}`} aria-label={userName ?? t.shell.account} className="vv-mavatar">
          {(userName ?? t.shell.account).slice(0, 1).toLocaleUpperCase('vi-VN')}
        </a>
      </header>
      {children}
      <MobileTabBar route={route} reviewCount={status?.reviewCount ?? 0} channelHref={channelHref} />
    </div>
  )
}
