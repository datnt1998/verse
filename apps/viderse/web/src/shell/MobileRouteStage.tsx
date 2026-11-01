// MobileRouteStage — khung nội dung điện thoại (<768px) giữ các màn gốc của
// thanh tab trong pane giữ-alive. Đổi tab chỉ ẩn pane cũ (`<Activity>`, kit
// `MobilePanes`): state, DOM và vị trí cuộn còn nguyên, effect (SSE/poll/timer)
// của pane ẩn tự dừng, nên quay lại tab đã mở không dựng lại màn và không gọi
// lại API khi dữ liệu SWR còn mới.
// Màn đẩy (Cài đặt, Hàng chờ, Thư viện, template…) phủ lên trên; các pane vẫn
// nằm dưới nên quay ra là đúng chỗ cũ.
import { Suspense, memo, useEffect, useRef, type ReactNode } from 'react'
import { MobilePanes } from '@noverse/ui'
import { cache } from '@noverse/ui/data'
import * as stylex from '@stylexjs/stylex'
import { RouteScreen } from '../screens/RouteScreen'
import { routePath, type Route } from '../router'
import { keys } from '../data/keys'
import { PANE_STALE_MS, PaneStaleContext } from '../data/paneStale'
import { getWorkbench } from '../api/workbench'
import { getChannelSummaries } from '../api/workbenchChannels'
import { listRivals, listRivalVideos } from '../api/rivals'
import { listChannels } from '../api/channels'
import { request } from '../api/request'
import { VIDEO_PAGE_LIMIT } from '../screens/explore/useRivalVideos'
import { stageStyles } from './mobileStage.stylex'

/** Ảnh chụp một pane: route lúc pane được mở lần cuối + dấu nội dung. */
export type PaneSnapshot = { route: Route; signature: string }

/**
 * Pane của một route; `null` = màn đẩy (Cài đặt, Hàng chờ, Thư viện, template,
 * Editor…) — phủ lên trên, các pane đã mở vẫn nằm nguyên bên dưới.
 * Kênh và Tạo kênh chung một pane vì thanh tab chỉ có một mục Kênh.
 */
export function paneKeyOf(route: Route): string | null {
  switch (route.name) {
    case 'workbench': return 'workbench'
    case 'review': return 'review'
    case 'new': return 'new'
    case 'channel':
    case 'channel-new': return 'channels'
    case 'explore': return 'explore'
    default: return null
  }
}

/**
 * Cập nhật ảnh chụp pane, giữ nguyên object cũ khi route không đổi — nhờ vậy
 * `memo` của PaneView không bị phá mỗi lần router parse lại hash.
 */
export function keepPane(panes: Map<string, PaneSnapshot>, id: string, route: Route): PaneSnapshot {
  const signature = routePath(route)
  const previous = panes.get(id)
  if (previous && previous.signature === signature) return previous
  const next: PaneSnapshot = { route, signature }
  panes.set(id, next)
  return next
}

/** Nội dung một pane: vùng cuộn riêng của pane (không dùng chung với tab khác). */
const PaneView = memo(function PaneView({ pane, renderScreen }: { pane: PaneSnapshot; renderScreen: (route: Route) => ReactNode }) {
  // Video mới là trang con có thanh riêng, không thanh tab dưới → không cần đệm đáy.
  const noPad = pane.route.name === 'new'
  return <div {...stylex.props(stageStyles.scroll, noPad && stageStyles.noPad)}>
    <PaneStaleContext.Provider value={PANE_STALE_MS}>
      <Suspense fallback={null}>{renderScreen(pane.route)}</Suspense>
    </PaneStaleContext.Provider>
  </div>
})

/**
 * Nạp trước dữ liệu của các tab chưa mở (sau khung hình đầu, lúc máy rảnh) để
 * lần chạm tab đầu tiên vào thẳng dữ liệu thật thay vì khung chờ — mẫu của
 * Noverse (`mobile/index.tsx`). Tab ẩn/không nhìn thì để lúc quay lại.
 */
function useWarmTabs() {
  const warmed = useRef(false)
  useEffect(() => {
    if (warmed.current) return
    warmed.current = true
    const warm = () => {
      if (document.visibilityState !== 'visible') return
      cache.prefetch(['workbench', 'list', 'all'], ({ signal }) => getWorkbench('all', signal))
      cache.prefetch(['workbench', 'channels'], ({ signal }) => getChannelSummaries(signal))
      cache.prefetch(['review', 'counts'], ({ signal }) => request<{ approvedToday: number | null }>('/api/clone/counts', { signal }))
      cache.prefetch(keys.rivals(), ({ signal }) => listRivals(signal))
      cache.prefetch(keys.channels(), ({ signal }) => listChannels(signal))
      // Lưới Khám phá mở ở bộ lọc mặc định (Tất cả · nổi bật · 30 ngày).
      cache.prefetch(keys.rivalVideos('all', 'outlier', 0, 30, ''), ({ signal }) =>
        listRivalVideos('all', { sort: 'outlier', long: 0, within: 30, limit: VIDEO_PAGE_LIMIT }, signal))
    }
    const idle = typeof window.requestIdleCallback === 'function'
    const handle = idle ? window.requestIdleCallback(warm, { timeout: 1500 }) : window.setTimeout(warm, 250)
    return () => { if (idle) window.cancelIdleCallback(handle); else window.clearTimeout(handle) }
  }, [])
}

/** Màn đẩy tự cuộn bên trong và không có thanh tab dưới (Hàng chờ, Thư viện, sửa template). */
const SELF_SCROLL: Partial<Record<Route['name'], true>> = { queue: true, library: true, template: true }

export function MobileRouteStage({ route, renderScreen = (next: Route) => <RouteScreen route={next} /> }: {
  route: Route
  /** Màn của một route — tham số để test khung pane độc lập với bảng route. */
  renderScreen?: (route: Route) => ReactNode
}) {
  const panes = useRef(new Map<string, PaneSnapshot>()).current
  const active = paneKeyOf(route)
  if (active) keepPane(panes, active, route)
  useWarmTabs()
  const pushed = active === null
  return <div {...stylex.props(stageStyles.stage)}>
    <div {...stylex.props(stageStyles.panes)}>
      <MobilePanes
        panes={[...panes].map(([id, pane]) => [id, <PaneView key={id} pane={pane} renderScreen={renderScreen} />] as [string, ReactNode])}
        active={active}
      />
    </div>
    {pushed && <div {...stylex.props(stageStyles.push, SELF_SCROLL[route.name] && stageStyles.fullBleed)}>
      <Suspense fallback={null}>{renderScreen(route)}</Suspense>
    </div>}
  </div>
}
