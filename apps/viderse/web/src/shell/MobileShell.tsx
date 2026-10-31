// MobileShell — khung điện thoại <768px (08-dev-prompt V10: thanh dưới 5 mục).
// Thanh trên: logo 24 + bộ đổi kênh trên Bàn dựng + JobsPill gọn + avatar.
// Thanh dưới 64px, kính mờ (vv-glass), 5 mục — mục giữa Video mới là nút đỏ tròn;
// mục đang mở nền pill đỏ nhạt, mục thường icon xám. Nội dung pad-bottom 110px.
import { useState, type ReactNode } from 'react'
import { navigate, paths, pickChannelRoute, routePath, navSection, type Route } from '../router'
import { t } from '../i18n/vi'
import { LensLogo } from '../components/LensLogo'
import { Icon, type IconName } from '@noverse/ui'
import { ChannelSwitcher, type SwitcherChannel } from './ChannelSwitcher'
import { JobsPill } from './JobsPill'
import type { Status } from '../api/status'
import { QueueSummarySheet } from '../screens/queue/QueueSummarySheet'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r, pipeline, visualRoles } from '../styles/viderse.stylex'

const TAB_ITEMS: { key: ReturnType<typeof navSection>; label: string; icon: IconName; iconActive: IconName; path: string; badge?: 'review' }[] = [
  { key: 'workbench', label: t.nav.workbench, icon: 'workbench', iconActive: 'workbench-b', path: paths.workbench() },
  { key: 'review', label: t.nav.review, icon: 'gallery', iconActive: 'gallery-b', path: paths.review(), badge: 'review' },
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
    <nav role="navigation" aria-label={t.nav.bottom} className={`vv-tabbar ${stylex.props(mobileShellStyles.tabbar, !fixed && mobileShellStyles.embeddedTabbar).className}`}>
      <span aria-hidden="true" {...stylex.props(mobileShellStyles.tabbarSheen)} />
      {TAB_ITEMS.map((item) => {
        const path = item.key === 'channels' ? channelHref ?? item.path : item.path
        if (item.key === 'new') {
          return (
            <a key="new" href={`#${path}`} aria-label={t.nav.new} title={t.nav.new} className={`vv-tabbar__item ${stylex.props(mobileShellStyles.item).className}`}>
              <span className={`vv-tabbar__new ${stylex.props(mobileShellStyles.newButton).className}`}><Icon name="add" size={22} /></span>
            </a>
          )
        }
        const active = item.key === section
        return (
          <a key={item.key} href={`#${path}`} aria-current={active ? 'page' : undefined}
            className={`${active ? 'vv-tabbar__item vv-tabbar__item--active' : 'vv-tabbar__item'} ${stylex.props(mobileShellStyles.item, active && mobileShellStyles.itemActive).className}`}>
            {active && <span aria-hidden="true" {...stylex.props(mobileShellStyles.itemGlow)} />}
            <span className={`vv-tabbar__pill ${stylex.props(mobileShellStyles.pill, active && mobileShellStyles.pillActive).className}`}>
              <Icon name={active ? item.iconActive : item.icon} size={24} />
              {item.badge === 'review' && reviewCount > 0 && (
                <span className={`vv-tabbar__badge num ${stylex.props(mobileShellStyles.badge).className}`} aria-label={`${reviewCount}`}>{reviewCount > 99 ? '99+' : reviewCount}</span>
              )}
            </span>
            <span className={`vv-tabbar__label ${stylex.props(mobileShellStyles.label, active && mobileShellStyles.labelActive).className}`}>{item.label}</span>
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
  const [jobsOpen, setJobsOpen] = useState(false)
  if (route.name === 'queue') return <div className={`vv-mobile-shell ${stylex.props(mobileShellStyles.shell).className}`}>{children}</div>
  const running = status?.running ?? 0
  const pickChannel = (id: string | undefined) => navigate(routePath(pickChannelRoute(route, id)))
  const showSwitcher = navSection(route) === 'workbench' || route.name === 'channel'
  const isNew = route.name === 'new'
  // MNew artboard: trang con có thanh riêng (× + tiêu đề), không thanh dưới.
  if (isNew) {
    return (
      <div className={`vv-mobile-shell vv-mobile-shell--new ${stylex.props(mobileShellStyles.shell).className}`}>
        <header role="banner" className={`vv-mbar vv-mbar--new ${stylex.props(mobileShellStyles.bar, mobileShellStyles.newBar).className}`}>
          <button type="button" aria-label={t.common.close} className={`vv-mclose ${stylex.props(mobileShellStyles.close).className}`}
            onClick={() => navigate(paths.workbench())}>
            <Icon name="close" size={18} />
          </button>
          <span className={`vv-mnew-title ${stylex.props(mobileShellStyles.newTitle).className}`}>{t.screens.new.title}</span>
        </header>
        {children}
      </div>
    )
  }
  return (
    <div className={`vv-mobile-shell ${stylex.props(mobileShellStyles.shell).className}`}>
      <header role="banner" className={`vv-mbar ${stylex.props(mobileShellStyles.bar).className}`}>
        <a href={`#${paths.workbench()}`} aria-label={t.brand.name} {...stylex.props(mobileShellStyles.logo)}>
          <LensLogo size={24} spinning={online && running > 0} />
          <span aria-hidden="true" {...stylex.props(mobileShellStyles.touchHalo)} />
        </a>
        {showSwitcher && <ChannelSwitcher mobileBar channels={channels ?? []} current={route.name === 'channel' ? route.channelId : route.name === 'workbench' ? route.channel : undefined} onPick={pickChannel} />}
        <span {...stylex.props(mobileShellStyles.grow)} />
        <JobsPill mobileBar status={status} online={online} compact unread={(status?.attention.length ?? 0) > 0} onOpen={() => setJobsOpen(true)} expanded={jobsOpen} />
        <a href={`#${paths.settings()}`} aria-label={t.shell.settings} className={`vv-mavatar ${stylex.props(mobileShellStyles.avatar).className}`}>
          {(userName ?? t.shell.account).slice(0, 1).toLocaleUpperCase('vi-VN')}
          <span aria-hidden="true" {...stylex.props(mobileShellStyles.touchHalo)} />
        </a>
      </header>
      {children}
      <MobileTabBar route={route} reviewCount={status?.reviewCount ?? 0} channelHref={channelHref} />
      <QueueSummarySheet open={jobsOpen} onClose={() => setJobsOpen(false)} />
    </div>
  )
}

const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
export const mobileShellStyles = stylex.create({
  shell: { height: '100dvh', display: 'flex', flexDirection: 'column', paddingTop: 'env(safe-area-inset-top)', lineHeight: 1.5 },
  bar: { position: 'relative', zIndex: 30, flex: '0 0 52px', display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], height: 52, padding: `0 ${s['--vv-s-12']} 0 ${s['--vv-s-16']}`, backgroundColor: color['--color-bg'] },
  newBar: { padding: `0 ${s['--vv-s-6']}`, gap: 0 },
  close: { width: 44, height: 44, display: 'grid', placeItems: 'center', borderRadius: radius['--radius-tile'], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: "transparent", color: color['--color-fg-2'], cursor: 'pointer' },
  newTitle: { flex: 1, minWidth: 0, paddingLeft: s['--vv-s-2'], fontSize: 16, fontWeight: 600 },
  avatar: { width: 34, height: 34, borderRadius: '50%', flex: '0 0 auto', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 12.5, fontWeight: 600, borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-3'], color: color['--color-fg'], backgroundColor: color['--color-surface-6'], position: { default: null, [TOUCH]: 'relative' }, overflow: { default: null, [TOUCH]: 'visible' } },
  logo: { display: 'flex', alignItems: 'center', flex: '0 0 auto', position: { default: null, [TOUCH]: 'relative' } },
  grow: { flex: 1 },
  touchHalo: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2)` },
  tabbar: { position: 'fixed', left: s['--vv-s-14'], right: s['--vv-s-14'], bottom: `calc(${s['--vv-s-4']} + env(safe-area-inset-bottom))`, zIndex: 30, height: 64, display: 'flex', alignItems: 'stretch', justifyContent: 'space-around', padding: s['--vv-s-5'], borderRadius: pipeline.mobileDockRadius, backgroundColor: "var(--vv-glass)", backdropFilter: 'blur(26px) saturate(180%)', borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-4'], boxShadow: '0 22px 44px -14px rgb(from var(--vv-thumb-veil) r g b / .85), 0 2px 8px rgb(from var(--vv-thumb-veil) r g b / .35), inset 0 1px 0 color-mix(in srgb, var(--line-3), var(--line-4))' },
  embeddedTabbar: { position: 'absolute' },
  tabbarSheen: { position: 'absolute', inset: 0, borderRadius: 'inherit', pointerEvents: 'none', backgroundImage: `linear-gradient(180deg, ${line['--line-2']}, transparent 45%)` },
  item: { position: 'relative', flex: '1 1 0', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-2'], textDecoration: 'none', minWidth: 0, borderRadius: pipeline.mobileDockItemRadius, fontSize: 10.5, fontWeight: 500, color: 'var(--vv-st-neutral)' },
  itemActive: { isolation: 'isolate', color: 'var(--vv-tabbar-on)', fontWeight: 600 },
  itemGlow: { position: 'absolute', inset: `calc(-1 * ${s['--vv-s-1']})`, zIndex: -1, borderRadius: 'inherit', backgroundColor: 'var(--vv-tabbar-ind-bg)', borderWidth: '1px', borderStyle: 'solid', borderColor: 'var(--vv-tabbar-ind-bd)', boxShadow: 'inset 0 1px 0 var(--line-3), 0 0 18px -4px rgb(from var(--vv-red) r g b / .45)' },
  pill: { display: 'flex', alignItems: 'center', justifyContent: 'center', width: 44, height: 24, color: 'var(--vv-st-neutral)' },
  pillActive: { color: color['--color-accent'], transform: 'translateY(-1px) scale(1.06)' },
  label: { fontSize: 10.5, lineHeight: '16px', fontWeight: 500, color: 'var(--vv-st-neutral)' },
  labelActive: { fontWeight: 600, color: 'var(--vv-tabbar-on)' },
  badge: { position: 'absolute', top: s['--vv-s-5'], left: `calc(50% + ${s['--vv-s-6']})`, minWidth: 17, height: 17, padding: `0 ${s['--vv-s-4']}`, borderRadius: r['--vv-r-pill'], display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 10.5, fontWeight: 700, backgroundColor: color['--color-warn'], color: color['--color-bg'], boxShadow: `0 0 0 2px ${color['--color-surface-4']}` },
  newButton: { width: 46, height: 46, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--vv-red-button)', backgroundImage: visualRoles['--vv-red-button-fill'], color: 'var(--vv-red-ink)', boxShadow: 'var(--vv-shadow-fab)', transform: { default: null, ':active': 'scale(.94)' } },
})


