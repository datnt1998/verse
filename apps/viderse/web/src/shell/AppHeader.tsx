// AppHeader — thanh 48px dán đỉnh: [logo] [Bộ đổi kênh | “Cài đặt · máy này”] |
// [nav giữa] | [JobsPill] [bánh răng] [avatar]. Logo bấm về Bàn dựng; bánh răng
// dẫn Cài đặt và có chấm đỏ khi có mục attention; ở màn Cài đặt bánh răng đang mở
// (nền --color-surface-6) và chỗ bộ đổi kênh là nhãn “Cài đặt · máy này”.
// Kiểu: styles/components.css .vv-header/.vv-gear.
import { useState, type MouseEvent } from 'react'
import { navigate, paths, pickChannelRoute, routePath, type Route, navSection } from '../router'
import { t } from '../i18n/vi'
import { LensLogo } from '../components/LensLogo'
import { Icon } from '@noverse/ui'
import { ChannelSwitcher, type SwitcherChannel } from './ChannelSwitcher'
import { ExploreCrumb } from './ExploreCrumb'
import { JobsPill } from './JobsPill'
import { MainNav } from './MainNav'
import type { Status } from '../api/status'
import { useResource } from '../data/useResource'
import { getServerStorage } from '../api/storageServer'
import { serverStorageCopy } from '../i18n/storageServer'
import { AccountMenu } from './AccountMenu'
import { ActivityPopover } from './ActivityPopover'
import { getActivity } from '../api/activity'
import { keys } from '../data/keys'
import * as stylex from '@stylexjs/stylex'
const sx = stylex.create({
  header: { paddingInline: 16 },
  logo: { marginInline: 1 },
  settings: { width: 32, height: 32, borderRadius: 'var(--vv-r-8)' },
  activityAnchor: { position: 'relative', flex: '0 0 auto' },
})

export function AppHeader({ route, status, online, userName, channels, channelHref }: {
  route: Route
  status: Status | undefined
  online: boolean
  userName?: string
  /** kênh của tôi để hiện trong bộ đổi kênh (chưa có dữ liệu → []) */
  channels?: SwitcherChannel[]
  /** đích mục nav Kênh (kênh gần nhất / tạo kênh khi chưa có) */
  channelHref?: string
}) {
  const running = status?.running ?? 0
  const [activityOpen, setActivityOpen] = useState(false)
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal))
  const attention = status?.attention ?? []
  const showSwitcher = navSection(route) === 'workbench' || route.name === 'channel'

  const home = (e: MouseEvent) => { e.preventDefault(); navigate(paths.workbench()) }
  const pickChannel = (id: string | undefined) => navigate(routePath(pickChannelRoute(route, id)))
  const inSettings = route.name === 'settings'
  const server = useResource(['storage', 'server'], ({ signal }) => getServerStorage(signal), { enabled: inSettings })
  const settingsTarget = attention.find((a) => a.href.includes('#/settings'))?.href
  const settings = (e: MouseEvent) => {
    e.preventDefault()
    if (settingsTarget) { window.location.hash = settingsTarget; return }
    navigate(paths.settings())
  }

  return (
    <header role="banner" className={`vv-header ${stylex.props(sx.header).className}`}>
      <div className="vv-header__zone">
        <a href={`#${paths.workbench()}`} aria-label={t.brand.name} onClick={home} {...stylex.props(sx.logo)} style={{ display: 'flex', alignItems: 'center', flex: '0 0 auto' }}>
          <LensLogo size={26} spinning={online && running > 0} />
        </a>
        {route.name === 'account'
          ? <span className="vv-header__crumb">{t.account.title}</span>
          : inSettings
          ? <span className="vv-header__crumb">{server.data ? serverStorageCopy.serverTitle : t.shell.settingsCrumb}</span>
          : route.name === 'explore'
            ? <ExploreCrumb scope={route.rival} />
            : showSwitcher && <ChannelSwitcher channels={channels ?? []} current={route.name === 'channel' ? route.channelId : route.name === 'workbench' ? route.channel : undefined} onPick={pickChannel} />}
      </div>
      <MainNav route={route} reviewCount={status?.reviewCount ?? 0} channelHref={channelHref} />
      <div className="vv-header__zone vv-header__zone--right">
        <div {...stylex.props(sx.activityAnchor)}>
          <JobsPill status={status} online={online} unread={(activity.data?.unread ?? []).some((n) => n.kind === 'review_ready' || n.kind === 'needs_attention')} onOpen={() => setActivityOpen((open) => !open)} expanded={activityOpen} />
          {activityOpen && <ActivityPopover activity={activity.data} error={activity.error} retry={() => void activity.refresh()} close={() => setActivityOpen(false)} />}
        </div>
        <a
          href={settingsTarget ?? `#${paths.settings()}`}
          aria-label={attention.length ? t.shell.settingsAttention(attention.length) : t.shell.settings}
          title={t.shell.settings}
          onClick={settings}
          className={`vv-gear ${stylex.props(sx.settings).className}`}
          aria-current={inSettings ? 'page' : undefined}
        >
          <Icon name="setting" size={18} />
          {attention.length > 0 && <span role="img" aria-label={`${attention.length}`} className="vv-gear__dot" />}
        </a>
        <AccountMenu fallbackName={userName} />
      </div>
    </header>
  )
}
