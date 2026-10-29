// AppShell — khung ứng dụng cho người đã đăng nhập: AppHeader (+ dải offline
// ngay dưới đầu), vùng cuộn của màn hình, và MobileShell ở bề ngang <768px.
// Nội dung mờ 42% khi mất kết nối (System artboard · state=offline).
import type { ReactNode } from 'react'
import type { Route } from '../router'
import { useMediaQuery } from '../lib/useMediaQuery'
import { useStatus, useOnline } from '../data/status'
import { useEventsStatus } from '../data/events'
import { AppHeader } from './AppHeader'
import { MobileShell } from './MobileShell'
import { OfflineStrip } from './OfflineStrip'
import type { SwitcherChannel } from './ChannelSwitcher'
import type { User } from '../api/types'

export function AppShell({ route, user, children }: {
  route: Route
  user: User
  children: ReactNode
}) {
  const mobile = useMediaQuery('(max-width: 767px)')
  const status = useStatus()
  const online = useOnline()
  const connecting = useEventsStatus().state === 'connecting'

  const offline = !online && !connecting
  const strip = offline ? <OfflineStrip /> : undefined

  if (mobile) {
    return (
      <MobileShell route={route} status={status.data} online={online} userName={user.name || user.username}>
        {strip}
        <div inert={offline || undefined} style={{ flex: '1 1 auto', minWidth: 0, paddingBottom: 'var(--vv-tabbar-pad)', ...(offline ? { opacity: 0.42 } : {}) }}>
          {children}
        </div>
      </MobileShell>
    )
  }
  return (
    <div style={{ minHeight: '100dvh', display: 'flex', flexDirection: 'column' }}>
      <AppHeader route={route} status={status.data} online={online} userName={user.name || user.username} />
      {strip}
      <div inert={offline || undefined} style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', ...(offline ? { opacity: 0.42 } : {}) }}>
        {children}
      </div>
    </div>
  )
}
