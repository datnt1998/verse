// MobileShell — khung điện thoại <768px (08-dev-prompt V10: thanh dưới 5 mục).
// Thanh trên: logo 24 + bộ đổi kênh trên Bàn dựng + JobsPill gọn + avatar.
// Thanh dưới 64px, kính mờ (vv-glass), 5 mục — mục giữa Video mới là nút đỏ tròn;
// mục đang mở nền pill đỏ nhạt, mục thường icon xám. Nội dung pad-bottom 110px.
import { Suspense, lazy, useState, type ReactNode } from 'react'
import { navigate, paths, pickChannelRoute, routePath, navSection, type NavKey, type Route } from '../router'
import { t } from '../i18n/vi'
import { LensLogo } from '../components/LensLogo'
import { Icon, MobileRuntimeProvider, MobileTabBar as KitMobileTabBar, MobileTopBar, type IconName, type MobileTabItem, type MobileTabBarSlots } from '@noverse/ui'
import { ChannelSwitcher, type SwitcherChannel } from './ChannelSwitcher'
import { JobsPill } from './JobsPill'
import type { Status } from '../api/status'
import { getActivity } from '../api/activity'
import { keys } from '../data/keys'
import { useResource } from '../data/useResource'
import * as stylex from '@stylexjs/stylex'
import { color, line, radius } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r, pipeline, visualRoles } from '../styles/viderse.stylex'

// Bảng Hàng chờ của điện thoại chỉ mở khi bấm: nạp mã riêng, không nằm trong chunk đầu.
const QueueSummarySheet = lazy(() => import('../screens/queue/QueueSummarySheet').then((m) => ({ default: m.QueueSummarySheet })))

const TAB_ITEMS: { key: NavKey; label: string; icon: IconName; iconActive: IconName; path: string; badge?: 'review' }[] = [
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
  const items: MobileTabItem[] = TAB_ITEMS.map((item) => ({
    key: item.key,
    label: item.label || undefined,
    icon: item.icon,
    iconActive: item.iconActive,
    href: item.key === 'channels' ? channelHref ?? item.path : item.path,
    ariaLabel: item.key === 'new' ? t.nav.new : item.label,
    center: item.key === 'new',
    badge: item.badge === 'review' && reviewCount > 0
      ? <span className={`vv-tabbar__badge num ${stylex.props(mobileShellStyles.badge).className}`} aria-label={t.nav.reviewBadge(reviewCount)}>{reviewCount > 99 ? '99+' : reviewCount}</span>
      : undefined,
  }))
  const slots: MobileTabBarSlots = {
    root: `vv-tabbar ${stylex.props(mobileShellStyles.tabbar, !fixed && mobileShellStyles.embeddedTabbar).className}`,
    cap: `vv-tabbar__cap ${stylex.props(mobileShellStyles.cap).className}`,
    indicator: `vv-tabbar__ind ${stylex.props(mobileShellStyles.ind).className}`,
    item: `vv-tabbar__item ${stylex.props(mobileShellStyles.item).className}`,
    itemActive: stylex.props(mobileShellStyles.itemActive).className,
    icon: `vv-tabbar__icon ${stylex.props(mobileShellStyles.icon).className}`,
    iconActive: stylex.props(mobileShellStyles.iconActive).className,
    label: `vv-tabbar__label ${stylex.props(mobileShellStyles.label).className}`,
    labelActive: stylex.props(mobileShellStyles.labelActive).className,
  }
  return <KitMobileTabBar
    items={items}
    activeKey={section ?? undefined}
    slots={slots}
    ariaLabel={t.nav.bottom}
    centerAction={<span className={`vv-tabbar__new ${stylex.props(mobileShellStyles.newButton).className}`}><Icon name="add" size={22} /></span>}
    onSelect={(item) => { if (item.href) navigate(item.href) }}
  />
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
  // Chunk của bảng Việc nền (6 chunk) chỉ được tải khi người dùng mở lần đầu —
  // mount sẵn <QueueSummarySheet open={false}> sẽ kéo cả cụm vào lần vẽ đầu của shell.
  const [jobsLoaded, setJobsLoaded] = useState(false)
  /* MLibrary / MTemplate là màn con tự vẽ thanh riêng (artboard: quay lại +
     tiêu đề), không thanh app chung phía trên. */
  const subPage = route.name === 'library' || route.name === 'template'
  const [jobsTab, setJobsTab] = useState<'running' | 'inbox'>('running')
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal))
  if (route.name === 'queue') return <MobileRuntimeProvider hidden><div className={`vv-mobile-shell ${stylex.props(mobileShellStyles.shell).className}`}>{children}</div></MobileRuntimeProvider>
  const running = status?.running ?? 0
  const pickChannel = (id: string | undefined) => navigate(routePath(pickChannelRoute(route, id)))
  const showSwitcher = navSection(route) === 'workbench' || route.name === 'channel'
  /* Chấm vàng đếm đúng thông báo cần bạn chưa đọc — cùng nguồn với sheet Việc nền
     (§11 GET /api/activity), nên “Đánh dấu đã đọc” là chấm mất. */
  const unread = activity.data?.unread.length ?? 0
  const isNew = route.name === 'new'
  // MNew artboard: trang con có thanh riêng (× + tiêu đề), không thanh dưới.
  if (isNew) {
    return (
      <MobileRuntimeProvider hidden><div className={`vv-mobile-shell vv-mobile-shell--new ${stylex.props(mobileShellStyles.shell).className}`}>
        <MobileTopBar
          slots={{ root: `vv-mbar vv-mbar--new ${stylex.props(mobileShellStyles.bar, mobileShellStyles.newBar).className}`, leading: stylex.props(mobileShellStyles.newLead).className, title: `vv-mnew-title ${stylex.props(mobileShellStyles.newTitle).className}` }}
          leading={<button type="button" aria-label={t.common.close} className={`vv-mclose ${stylex.props(mobileShellStyles.close).className}`}
            onClick={() => navigate(paths.workbench())}>
            <Icon name="close" size={18} />
          </button>}
          title={t.screens.new.title}
        />
        {children}
      </div></MobileRuntimeProvider>
    )
  }
  return (
    <MobileRuntimeProvider hidden={route.name === 'channel-new'}><div className={`vv-mobile-shell ${stylex.props(mobileShellStyles.shell).className}`}>
      {!subPage &&
      <MobileTopBar
        slots={{ root: `vv-mbar ${stylex.props(mobileShellStyles.bar).className}`, leading: stylex.props(mobileShellStyles.lead).className, actions: stylex.props(mobileShellStyles.end).className }}
        leading={<>
          {/* Logo không phải nút chạm riêng (AppBar đợt 36) — bộ đổi kênh đứng cạnh nó. */}
          <a href={`#${paths.workbench()}`} aria-label={t.brand.name} {...stylex.props(mobileShellStyles.logo)}>
            <LensLogo size={24} spinning={online && running > 0} />
          </a>
          {showSwitcher && <ChannelSwitcher mobileBar channels={channels ?? []} current={route.name === 'channel' ? route.channelId : route.name === 'workbench' ? route.channel : undefined} onPick={pickChannel} />}
        </>}
        actions={<>
          <JobsPill mobileBar status={status} online={online} unread={unread} onOpen={() => { setJobsLoaded(true); setJobsTab(unread > 0 ? 'inbox' : 'running'); setJobsOpen(true) }} expanded={jobsOpen} />
          <a href={`#${paths.settings()}`} aria-label={t.shell.accountSettings} title={t.shell.settings} className={`vv-mavatar vv-mbtn vv-mbtn--av ${stylex.props(mobileShellStyles.avatar).className}`}>
            {(userName ?? t.shell.account).slice(0, 1).toLocaleUpperCase('vi-VN')}
            <span aria-hidden="true" {...stylex.props(mobileShellStyles.touchHalo)} />
          </a>
        </>}
      />}
      {children}
      {/* MCreate (tạo kênh) là màn đẩy có chân riêng — artboard không vẽ thanh tab.
          MLibrary / MTemplate cũng vậy: artboard chỉ có thanh riêng của màn. */}
      {route.name === 'channel-new' || subPage ? null : <MobileTabBar route={route} reviewCount={status?.reviewCount ?? 0} channelHref={channelHref} />}
      {jobsLoaded && <Suspense fallback={null}>
        <QueueSummarySheet open={jobsOpen} initialTab={jobsTab} unread={unread} onClose={() => setJobsOpen(false)} />
      </Suspense>}
    </div></MobileRuntimeProvider>
  )
}

const TOUCH = '@media (max-width: 767px) and (pointer: coarse)'
export const mobileShellStyles = stylex.create({
  shell: { height: '100dvh', display: 'flex', flexDirection: 'column', paddingTop: 'var(--m-top)', lineHeight: 1.5 },
  /* AppBar đợt 36: một hàng 52px, đệm 0 12 0 16, khe 10; phải đẩy sát mép khe 8. */
  bar: { position: 'relative', zIndex: 30, flex: '0 0 52px', display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], height: 52, padding: `0 ${s['--vv-s-12']} 0 ${s['--vv-s-16']}`, backgroundColor: color['--color-bg'] },
  newBar: { padding: `0 ${s['--vv-s-6']}`, gap: 0 },
  close: { width: 44, height: 44, display: 'grid', placeItems: 'center', borderRadius: radius['--radius-tile'], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: "transparent", color: color['--color-fg-2'], cursor: 'pointer' },
  newTitle: { flex: 1, minWidth: 0, paddingLeft: s['--vv-s-2'], fontSize: 16, fontWeight: 600 },
  /* Nhóm trái: logo 24 + bộ đổi kênh, khe 10 như hàng thanh trên. */
  lead: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], minWidth: 0, flex: '0 1 auto' },
  newLead: { display: 'flex', alignItems: 'center', flex: '0 0 auto' },
  /* Nhóm phải: nút Việc nền + avatar, không co, đẩy sát mép phải. */
  end: { marginInlineStart: 'auto', display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], flex: '0 0 auto' },
  /* Mọi phần tử thanh trên cao 36, nền surface-1, viền line-2 (artboard .vv-mbtn). */
  avatar: { width: 36, height: 36, borderRadius: '50%', flex: '0 0 auto', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 13, fontWeight: 600, letterSpacing: '.02em', borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-2'], color: color['--color-fg'], backgroundColor: color['--color-surface-6'], textDecoration: 'none', position: { default: null, [TOUCH]: 'relative' }, overflow: { default: null, [TOUCH]: 'visible' } },
  /* Logo 24px không phải nút chạm riêng — chỉ canh giữa theo hàng 36px. */
  logo: { display: 'flex', alignItems: 'center', flex: '0 0 auto', height: 36 },
  touchHalo: { display: { default: 'none', [TOUCH]: 'block' }, position: 'absolute', inset: `calc((${s['--vv-s-44']} - 100%) / -2)` },
  /* Thanh dưới = vỏ kính 64px cách đáy var(--m-tabbar-bottom) (Viderse 30px khi không có
     home indicator), vạch chỉ báo trượt theo --i (artboard kit.css .vv-tabbar*). */
  tabbar: { position: 'fixed', left: s['--vv-s-14'], right: s['--vv-s-14'], bottom: 'var(--m-tabbar-bottom)', zIndex: 30, height: 'var(--m-tabbar-h)', transitionProperty: 'transform, opacity', transitionDuration: '.42s, .3s', transitionTimingFunction: 'cubic-bezier(.2,.9,.25,1), ease' },
  embeddedTabbar: { position: 'absolute' },
  cap: { position: 'relative', height: '100%', width: '100%', display: 'flex', padding: s['--vv-s-5'], borderRadius: pipeline.mobileDockRadius, backgroundColor: "var(--vv-glass)", backdropFilter: 'blur(26px) saturate(180%)', borderWidth: "1px", borderStyle: "solid", borderColor: line['--line-4'], boxShadow: '0 22px 44px -14px rgb(from var(--vv-thumb-veil) r g b / .85), 0 2px 8px rgb(from var(--vv-thumb-veil) r g b / .35), inset 0 1px 0 color-mix(in srgb, var(--line-3), var(--line-4))' },
  ind: { position: 'absolute', top: s['--vv-s-5'], bottom: s['--vv-s-5'], width: 'calc((100% - 10px) / 5)', left: 'calc(5px + var(--i, 0) * (100% - 10px) / 5)', borderRadius: pipeline.mobileDockItemRadius, backgroundColor: 'var(--vv-tabbar-ind-bg)', borderWidth: '1px', borderStyle: 'solid', borderColor: 'var(--vv-tabbar-ind-bd)', boxShadow: 'inset 0 1px 0 var(--line-3), 0 0 18px -4px rgb(from var(--vv-red) r g b / .45)', pointerEvents: 'none', transitionProperty: 'left', transitionDuration: '.45s', transitionTimingFunction: 'cubic-bezier(.2,.9,.25,1.15)' },
  item: { position: 'relative', zIndex: 1, flex: '1 1 0', minWidth: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-2'], padding: 0, borderRadius: pipeline.mobileDockItemRadius, borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', fontFamily: 'inherit', fontSize: 10.5, fontWeight: 500, color: 'var(--vv-st-neutral)', textDecoration: 'none', cursor: 'pointer' },
  itemActive: { color: 'var(--vv-tabbar-on)', fontWeight: 600 },
  icon: { transitionProperty: 'transform', transitionDuration: '.35s', transitionTimingFunction: 'cubic-bezier(.2,.9,.3,1.4)' },
  iconActive: { color: color['--color-accent'], transform: 'translateY(-1px) scale(1.06)' },
  label: { maxHeight: 16, lineHeight: '16px', whiteSpace: 'nowrap', transitionProperty: 'opacity, max-height, margin', transitionDuration: '.2s, .3s, .3s' },
  labelActive: { fontWeight: 600, color: 'var(--vv-tabbar-on)' },
  badge: { position: 'absolute', top: s['--vv-s-5'], left: `calc(50% + ${s['--vv-s-6']})`, minWidth: 17, height: 17, padding: `0 ${s['--vv-s-4']}`, borderRadius: r['--vv-r-pill'], display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: 10.5, fontWeight: 700, backgroundColor: color['--color-warn'], color: color['--color-bg'], boxShadow: `0 0 0 2px ${color['--color-surface-4']}` },
  /* FAB đỏ 46px: hai bevel inset (sáng trên, tối dưới) + quầng đỏ, đúng artboard. */
  newButton: { width: 46, height: 46, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: 'var(--vv-red-button)', backgroundImage: visualRoles['--vv-red-button-fill'], color: 'var(--vv-red-ink)', boxShadow: '0 8px 20px -6px rgb(from var(--vv-red) r g b / .7), inset 0 1px 0 rgb(255 255 255 / .35), inset 0 -2px 4px rgb(0 0 0 / .25)', transform: { default: null, ':active': 'scale(.94)' } },
})


