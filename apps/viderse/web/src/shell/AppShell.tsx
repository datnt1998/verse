// AppShell — khung ứng dụng cho người đã đăng nhập: AppHeader (+ dải offline
// ngay dưới đầu), vùng cuộn của màn hình, và MobileShell ở bề ngang <768px.
// Nội dung mờ 42% khi mất kết nối (System artboard · state=offline).
import { useEffect, useRef, useState, type ReactNode } from 'react'
import type { Route } from '../router'
import { useMediaQuery } from '../lib/useMediaQuery'
import { useStatus, useOnline } from '../data/status'
import { useEventsStatus } from '../data/events'
import { AppHeader } from './AppHeader'
import { MobileShell } from './MobileShell'
import { OfflineStrip } from './OfflineStrip'
import type { SwitcherChannel } from './ChannelSwitcher'
import { useResource } from '../data/useResource'
import { keys } from '../data/keys'
import { listChannels } from '../api/channels'
import { getChannelSummaries } from '../api/workbenchChannels'
import { onboardDismissed } from '../lib/onboard'
import { channelsHref } from '../lib/lastChannel'
import { channelTint } from '../lib/channelTint'
import type { User } from '../api/types'
import { navigate } from '../router'
import { getSetup } from '../api/setup'
import { vi as t } from '../i18n/vi'
import { AdminStorageStrip } from './AdminStorageStrip'
import { ActivityBridge } from './ActivityBridge'

export function AppShell({ route, user, children }: {
  route: Route
  user: User
  children: ReactNode
}) {
  const mobile = useMediaQuery('(max-width: 767px)')
  const status = useStatus()
  const online = useOnline()
  const connecting = useEventsStatus().state === 'connecting'

  // Kênh của tôi cho Bộ đổi kênh + luật Onboard. Lỗi/không có gì → danh sách
  // rỗng, không chặn khung (test shell và máy chưa có kênh vẫn chạy).
  const channelList = useResource(
    keys.channels(),
    ({ signal }) => listChannels(signal).catch(() => ({ channels: [] })),
  )
  const summaries = useResource(['channels', 'summary'], ({ signal }) => getChannelSummaries(signal))
  const channelHref = channelsHref(channelList.data?.channels ?? [])
  const channels: SwitcherChannel[] = (channelList.data?.channels ?? []).map((channel) => {
    const summary = summaries.data?.find((item) => item.channelId === channel.id)
    return {
      id: channel.id,
      name: channel.name || t.channels.page.noName,
      initials: summary?.initials,
      src: summary?.avatarUrl ?? channel.avatarUrl ?? undefined,
      color: { bg: channelTint(channel.id), ring: channelTint(channel.id) },
      pace: summary ? `Hôm nay ${summary.today.done}/${summary.today.cap}` : undefined,
      waiting: (summary?.waitingYou ?? 0) > 0,
    }
  })

  const inSetup = route.name === 'setup'
  const offline = !online && !connecting
  const strip = <>{offline ? <OfflineStrip /> : null}{!mobile || route.name !== 'settings' ? <AdminStorageStrip user={user} status={status.data} /> : null}</>

  // Máy chưa chuẩn bị xong thì mở #/setup một lần mỗi phiên (desktop); vào
  // thẳng từ sau khi đăng nhập, không bắt người dùng đã bỏ qua quay lại.
  const setupChecked = useRef(false)
  // Đọc cờ skip một lần (đặt từ nút Quay lại của #/setup).
  const [setupSkipped] = useState(() => {
    try { return sessionStorage.getItem('viderse:setup-skipped') === '1' } catch { return false }
  })
  useEffect(() => {
    if (mobile || setupChecked.current || route.name === 'setup') return
    if (sessionStorage.getItem('viderse:setup-checked') || sessionStorage.getItem('viderse:setup-skipped')) return
    setupChecked.current = true
    const controller = new AbortController()
    // Chỉ đánh dấu "đã kiểm" khi CÓ quyết định từ server: fetch lỗi thì lần đổi
    // route sau được kiểm lại, không khoá cả phiên.
    void getSetup({ signal: controller.signal }).then((s) => {
      if (controller.signal.aborted) return
      sessionStorage.setItem('viderse:setup-checked', '1')
      if (!s.complete) navigate('/setup')
    }).catch(() => { setupChecked.current = false })
    return () => controller.abort()
  }, [mobile, route.name])

  // Chuẩn bị máy là màn riêng thời điểm mở app (W1): không header, không tab
  // dưới — Onboard chỉ có một việc là cài máy.
  // Lần đầu mở: máy đã cài xong, chưa có kênh nào, đang ở bước Tạo kênh — mở
  // khung riêng như Onboard (không nav). Huỷ/Tạo xong ghi cờ phiên để không lặp.
  const onboard = !mobile && route.name === 'channel-new' && (route.step ?? 'create') === 'create'
    && !onboardDismissed() && channelList.data !== undefined && channels.length === 0
  if (inSetup || onboard || (mobile && route.name === 'account')) {
    return <div style={{ minHeight: '100dvh', display: 'flex', flexDirection: 'column' }}>{strip}{children}</div>
  }

  if (mobile) {
    return (
      <MobileShell route={route} status={status.data} online={online} userName={user.name || user.username} channels={channels} channelHref={channelHref}>
        <ActivityBridge />
        {route.name !== 'queue' && strip}
        <div inert={offline && route.name !== 'queue' || undefined} style={{ flex: '1 1 auto', minWidth: 0, minHeight: 0, display: 'flex', flexDirection: 'column', overflowY: route.name === 'queue' ? 'hidden' : 'auto', paddingBottom: route.name === 'new' || route.name === 'queue' ? 0 : 'var(--vv-tabbar-pad)', ...(offline && route.name !== 'queue' ? { opacity: 0.42 } : {}) }}>
          {children}
        </div>
      </MobileShell>
    )
  }
  return (
    // Khung cao đúng màn: header + các dải cảnh báo lấy phần của mình, vùng
    // nội dung nhận phần còn lại và tự cuộn — màn có cột cố định (Cài đặt)
    // không bị đẩy xuống khi dải offline/chưa-sẵn-sàng hiện.
    <div style={{ height: '100dvh', display: 'flex', flexDirection: 'column' }}>
      <ActivityBridge />
      <AppHeader route={route} status={status.data} online={online} userName={user.name || user.username} channels={channels} channelHref={channelHref} />
      {setupSkipped && !inSetup ? (
        // Cảnh báo bền "máy chưa sẵn sàng" — có thể làm việc tiếp, không chặn.
        <button type="button" onClick={() => navigate('/setup')}
          style={{ display: 'flex', alignItems: 'center', gap: 'var(--vv-s-8)', padding: 'var(--vv-s-6) var(--vv-s-12)', border: 'none', borderBottom: 'var(--vv-s-1) solid var(--line-2)', background: 'var(--color-warn-bg, var(--color-surface-2))', color: 'var(--color-fg)', fontSize: '12.5px', cursor: 'pointer', width: '100%', textAlign: 'left' }}>
          <span style={{ flex: 1 }}>{t.prep.skippedWarn}</span>
          <span style={{ color: 'var(--color-fg-4)' }}>{t.prep.skippedAction} →</span>
        </button>
      ) : null}
      {strip}
      <div inert={offline || undefined} style={{ flex: '1 1 auto', minWidth: 0, minHeight: 0, overflowY: 'auto', display: 'flex', flexDirection: 'column', ...(offline ? { opacity: 0.42 } : {}) }}>
        {children}
      </div>
    </div>
  )
}
