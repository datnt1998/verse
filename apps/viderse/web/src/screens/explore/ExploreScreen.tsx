import * as stylex from '@stylexjs/stylex'
import { exploreStyles, exploreRootMarker } from './explore.stylex'
// Khám phá (artboard Explore, gói V5): đối thủ → lưới video nổi trội → panel
// đối thủ/video + Quét lịch sử + Lịch đăng. Phạm vi "Tất cả" hoặc một kênh.
// Lưới ảo hoá thủ công (≤ ~120 thẻ DOM cho 20.000 video), SSE `rivals` làm mới
// trang đầu. Ô "Tìm tiêu đề" (q, debounce 250 ms) cộng với bộ lọc; không có kết
// quả → khối thông báo + "Xoá tìm".
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { Icon, useTabMode } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
import type { Route } from '../../router'
import { navigate, paths } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { useDebounced } from '../../lib/useDebounced'
import { ActionButton, Orb } from '../../components'
import {
  getPostingSlots, getRivalScan, listRivals,
  type PostingZone, type Rival, type Video, type VideoSort,
} from '../../api/rivals'
import { listChannels } from '../../api/channels'
import { enqueueClone } from '../../api/clone'
import { toast } from '@noverse/ui'
import { RivalSidebar, rivalColor } from './RivalSidebar'
import { AddRivalForm } from './AddRivalForm'
import { FilterBar, WITHIN } from './FilterBar'
import { SearchBox } from './SearchBox'
import { VideoGrid } from './VideoGrid'
import { BulkBar } from './BulkBar'
import { RivalCard } from './RivalCard'
import { ScanCard } from './ScanCard'
import { PostingCard } from './PostingCard'
import { PreviewPanel } from './PreviewPanel'
import { useRivalVideos, type VideosFilter } from './useRivalVideos'
import { GRID_MAX_W } from './media'
import { GRID_GAP_Y, useVirtualGrid } from '../../lib/useVirtualGrid'

type Scope = 'all' | string

const num = new Intl.NumberFormat('vi-VN')

/** ?rival= của /explore là handle (@…): đối chiếu handle rồi mới tới id. */
const scopeOf = (param: string | null | undefined, rivals: Rival[]): Scope => {
  if (!param) return 'all'
  const hit = rivals.find((r) => r.handle === param || r.id === param)
  return hit ? hit.id : 'all'
}

export function ExploreScreen({ route }: { route: Route }) {
  // Màn này chỉ mount cho route 'explore' — thu hẹp để đọc rival/video.
  const eroute = route.name === 'explore' ? route : { name: 'explore' as const }
  const tt = t.screens.explore
  const narrow = useMediaQuery('(max-width: 767px)')

  const list = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const rivals = useMemo(() => list.data?.rivals ?? [], [list.data])
  // Kênh của tôi: có ≥ 1 kênh thì mở khoá các điều khiển chỉ cần kênh (chọn
  // kênh, liên kết đối thủ); nút xếp hàng vẫn khoá nhưng đổi lý do.
  const channelList = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const channels = useMemo(
    () => (channelList.data?.channels ?? []).map((channel) => ({
      id: channel.id,
      name: channel.name || t.channels.page.noName,
      sources: channel.sources,
      templateName: channel.templates?.[channel.templateCursor % channel.templates.length]?.name,
    })),
    [channelList.data],
  )
  const channelRefs = useMemo(() => channels.map(({ id, name, templateName }) => ({ id, name, templateName })), [channels])

  // Phạm vi lưới từ route (?rival= là handle); rail giữ kênh cuối cùng được
  // chọn để "Tất cả" vẫn còn ngữ cảnh đối thủ (kênh đầu danh sách khi chưa
  // chọn bao giờ).
  const scope: Scope = scopeOf(eroute.rival, rivals)
  const [railId, setRailId] = useState<string | null>(null)
  const railRival = useMemo(() => {
    const id = railId ?? (scope !== 'all' ? scope : rivals[0]?.id ?? null)
    return rivals.find((r) => r.id === id)
  }, [railId, scope, rivals])
  const railIndex = Math.max(0, railRival ? rivals.indexOf(railRival) : 0)

  const pickRival = useCallback((id: Scope) => {
    if (id !== 'all') setRailId(id)
    const handle = id === 'all' ? undefined : (rivals.find((r) => r.id === id)?.handle ?? id)
    navigate(paths.explore({ rival: handle }), true)
  }, [rivals])

  // Bộ lọc lưới — state cục bộ (route chỉ chuẩn hoá rival/video).
  const [sort, setSort] = useState<VideoSort>('outlier')
  const [long, setLong] = useState(false)
  const [within, setWithin] = useState<0 | 7 | 30 | 90>(30)
  // Ô tìm: gõ thì chờ 250 ms mới hỏi API; xoá trắng thì áp ngay (và xả luôn
  // giá trị chờ để gõ lại không hỏi chuỗi cũ).
  const [query, setQuery] = useState('')
  const typedQ = query.trim()
  const debouncedQ = useDebounced(typedQ, typedQ === '' ? 0 : 250)
  const q = typedQ === '' ? '' : debouncedQ
  const filter: VideosFilter = useMemo(
    () => ({ scope, sort, long, within, q }),
    [scope, sort, long, within, q],
  )
  const videos = useRivalVideos(filter)
  const items = videos.items
  // Dòng đếm và khối không có kết quả: số video đã quét của phạm vi + bộ lọc đang bật.
  const scopeRival = scope === 'all' ? undefined : rivals.find((r) => r.id === scope)
  const scanned = scopeRival ? scopeRival.videoCount : list.data?.totalVideos ?? 0
  const withinLabel = WITHIN.find((w) => w.value === within)?.label
  const activeFilters = [
    ...(long ? [tt.noMatchLong] : []),
    ...(within > 0 && withinLabel ? [tt.noMatchWithin(withinLabel)] : []),
  ]

  // Chọn nhiều + video đang xem ở cột phải.
  const [selectMode, setSelectMode] = useState(false)
  // Chế độ chọn nhiều thay thanh tab bằng thanh chọn (artboard MExplore) — ẩn hẳn thanh tab.
  useTabMode(selectMode ? 'hide' : undefined)
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set())
  const [pickedId, setPickedId] = useState<string | null>(eroute.video ?? null)
  const [zone, setZone] = useState<PostingZone>('vn')

  // Bộ lọc đổi → dọn lựa chọn/panel (dữ liệu cũ không còn hợp lệ); lần mount
  // giữ picked từ route.video nên bỏ qua.
  const firstMount = useRef(true)
  useEffect(() => {
    if (firstMount.current) { firstMount.current = false; return }
    setCheckedIds(new Set())
    setPickedId(null)
  }, [filter])

  const picked = useMemo(() => (pickedId ? items.find((v) => v.id === pickedId) ?? null : null), [pickedId, items])
  const toggleChecked = useCallback((video: Video) => {
    setCheckedIds((prev) => {
      const next = new Set(prev)
      if (next.has(video.id)) next.delete(video.id)
      else next.add(video.id)
      return next
    })
  }, [])
  const selectTop = useCallback((n: number) => {
    setCheckedIds(new Set(items.slice(0, n).map((v) => v.id)))
  }, [items])
  const exitSelect = useCallback(() => { setSelectMode(false); setCheckedIds(new Set()) }, [])

  // Lưới ảo hoá gắn vào chính cột giữa (nó là vùng cuộn).
  const grid = useVirtualGrid({ count: items.length, maxWidth: GRID_MAX_W })

  // Nạp trang tiếp khi cửa sổ render gần hết dữ liệu đã tải.
  useEffect(() => {
    if (!videos.hasMore) return
    if (grid.range.end * grid.cols >= items.length - grid.cols * 2) videos.needMore()
  }, [grid.range.end, grid.cols, items.length, videos.hasMore, videos.needMore])

  // SSE `rivals` → trang đầu đổi: giữ lựa chọn (Set theo id đã giữ) và neo cuộn
  // — item đầu tiên đang thấy giữ nguyên vị trí, video mới chen vào trước nó.
  const anchor = useRef<{ id: string; top: number } | null>(null)
  const prevItems = useRef(items)
  if (prevItems.current !== items) {
    const el = grid.ref.current
    if (el && items.length > 0 && !narrow) {
      const lead = Math.max(0, Math.floor(el.scrollTop / (grid.rowHeight + GRID_GAP_Y)) * grid.cols)
      const id = items[Math.min(lead, items.length - 1)]?.id
      const oldIndex = prevItems.current.findIndex((v) => v.id === id)
      if (id && oldIndex >= 0 && oldIndex !== Math.min(lead, items.length - 1)) {
        anchor.current = { id, top: el.scrollTop % (grid.rowHeight + GRID_GAP_Y) }
      }
    }
    prevItems.current = items
  }
  useLayoutEffect(() => {
    const a = anchor.current
    anchor.current = null
    const el = grid.ref.current
    if (!a || !el) return
    const index = items.findIndex((v) => v.id === a.id)
    if (index < 0) return
    const row = Math.floor(index / grid.cols)
    el.scrollTop = row * (grid.rowHeight + GRID_GAP_Y) + a.top
  }, [items, grid.cols, grid.rowHeight, grid.ref])
  // Cột phải: scan + posting-slots của kênh rail.
  const railScan = useResource(
    keys.rivalScan(railRival?.id ?? '_none'),
    ({ signal }) => getRivalScan(railRival!.id, signal),
    { enabled: !!railRival, keepPrevious: true },
  )
  const railPosting = useResource(
    keys.rivalPosting(railRival?.id ?? '_none', zone),
    ({ signal }) => getPostingSlots(railRival!.id, zone, signal),
    { enabled: !!railRival, keepPrevious: true },
  )

  const selectedVideos = useMemo(() => items.filter((v) => checkedIds.has(v.id)), [items, checkedIds])

  const [bulkBusy, setBulkBusy] = useState(false)
  const bulkEnqueue = useCallback(async (channelId: string) => {
    if (bulkBusy || !channels.some((channel) => channel.id === channelId) || selectedVideos.length === 0) return
    setBulkBusy(true)
    try {
      await enqueueClone({ channelId, urls: selectedVideos.map((v) => v.url), priority: 'batch' })
      toast(t.screens.new.toasts.made, 'ok', { label: t.screens.new.toasts.openQueue, onClick: () => navigate(paths.queue()) })
      exitSelect()
    } catch {
      toast.err(t.screens.new.toasts.failed)
    } finally { setBulkBusy(false) }
  }, [bulkBusy, channels, selectedVideos, exitSelect])

  // ─── Trạng thái trống: chưa theo dõi đối thủ nào ───
  // Artboard vẫn giữ cột đối thủ 252 px (nhóm "Đối thủ 0" + câu hướng dẫn)
  // và bỏ rail; chỉ cột giữa đổi thành khối giới thiệu. Ô thêm đối thủ chỉ có
  // một chỗ (giữa), nên cột trái không vẽ lại form.
  const empty = !list.initialLoading && rivals.length === 0 && list.error == null
  if (empty) {
    return (
      <div className={`vx-shell vx-shell--empty ${stylex.props(exploreStyles.shell, exploreStyles.shellEmpty, exploreRootMarker).className}`}>
        <RivalSidebar list={list.data} showAdd={false} selected="all" onPick={pickRival} onAdded={(rival) => pickRival(rival.id)} />
        <main className={`vx-empty ${stylex.props(exploreStyles.empty).className}`} aria-label={tt.title}>
          <div className={`vx-empty__box ${stylex.props(exploreStyles.emptyBox).className}`}>
            <Orb of="scout" size={40} live={false} />
            <h1 className={`vx-empty__title ${stylex.props(exploreStyles.emptyTitle).className}`}>{tt.emptyTitle}</h1>
            <p className={`vx-empty__body ${stylex.props(exploreStyles.emptyBody).className}`}>{tt.emptyBody}</p>
            <AddRivalForm onAdded={(rival) => pickRival(rival.id)} onOpen={pickRival} />
            <div className={`vx-empty__ex ${stylex.props(exploreStyles.emptyEx).className}`} aria-label={tt.example}>
              <span>{tt.example}</span>
              {tt.emptyExamples.map((example) => <code className={`vx-tag ${stylex.props(exploreStyles.tag).className}`} key={example}>{example}</code>)}
            </div>
          </div>
        </main>
      </div>
    )
  }

  return (
    <div className={`vx-shell ${stylex.props(exploreStyles.shell, exploreRootMarker).className}`}>
      <RivalSidebar
        list={list.data}
        selected={scope}
        onPick={pickRival}
        onAdded={(rival) => pickRival(rival.id)}
      />
      <main className={`vx-center vv-wide vv-wide--grid ${stylex.props(exploreStyles.center, selectMode && exploreStyles.selectPad).className}`} ref={grid.ref} onScroll={grid.onScroll} aria-label={tt.title}>
        <div className={`vx-head ${stylex.props(exploreStyles.head).className}`}>
          <h1 className={`vx-title ${stylex.props(exploreStyles.title).className}`}>{tt.outlierTitle}</h1>
          <span className={`vx-sub ${stylex.props(exploreStyles.sub).className}`}>{tt.outlierHint}</span>
          <span className={`vx-grow ${stylex.props(exploreStyles.grow).className}`} />
          <SearchBox value={query} onChange={setQuery} />
          <ActionButton className={stylex.props(exploreStyles.touchTarget).className}
            variant="outline"
            size="compact"
            icon={<Icon name="tick" size={15} style={{ color: selectMode ? 'var(--color-fg)' : 'var(--color-fg-3)' }} />}
            aria-pressed={selectMode}
            style={selectMode ? { background: 'var(--vv-wash-8)', borderColor: 'var(--vv-line-30)' } : undefined}
            onClick={() => (selectMode ? exitSelect() : setSelectMode(true))}
          >
            {selectMode ? tt.exitSelect : tt.selectMany}
          </ActionButton>
        </div>
        <FilterBar
          sort={sort}
          long={long}
          within={within}
          onSort={setSort}
          onLong={setLong}
          onWithin={setWithin}
          total={videos.total}
          scanned={scanned}
          q={videos.shownQ}
          hasChannel={channels.length > 0}
        />
        {videos.error != null && items.length === 0 ? (
          <p className={`vx-side-empty ${stylex.props(exploreStyles.sideEmpty).className}`}>{tt.videosError}</p>
        ) : items.length === 0 && !videos.initialLoading && videos.shownQ ? (
          <div className={`vx-nomatch ${stylex.props(exploreStyles.nomatch).className}`} role="status">
            <Icon name="search" size={22} className={`vx-nomatch__ic ${stylex.props(exploreStyles.nomatchIc).className}`} />
            <span className={`vx-nomatch__title ${stylex.props(exploreStyles.nomatchTitle).className}`}>{tt.noMatchTitle(videos.shownQ)}</span>
            <span className={`vx-nomatch__sub ${stylex.props(exploreStyles.nomatchSub).className}`}>{tt.noMatchWhere(num.format(scanned), scopeRival?.title ?? tt.noMatchAll, activeFilters)}</span>
            <ActionButton className={stylex.props(exploreStyles.touchTarget).className} size="sm" onClick={() => setQuery('')}>{tt.searchClear}</ActionButton>
          </div>
        ) : items.length === 0 && !videos.initialLoading ? (
          <p className={`vx-side-empty ${stylex.props(exploreStyles.sideEmpty).className}`}>{tt.videosEmpty}</p>
        ) : (
          <VideoGrid
            items={items}
            cols={grid.cols}
            start={grid.itemStart}
            end={grid.itemEnd}
            padTop={grid.padTop}
            padBottom={grid.padBottom}
            pickedId={pickedId}
            selectMode={selectMode}
            checkedIds={checkedIds}
            onOpen={(v) => { setSelectMode(false); setPickedId(v.id) }}
            onToggle={toggleChecked}
            hasChannel={channels.length > 0}
          />
        )}
        {selectMode && checkedIds.size > 0 && (
          <BulkBar
            count={checkedIds.size}
            selected={selectedVideos}
            onTop={selectTop}
            onClear={() => setCheckedIds(new Set())}
            onExit={exitSelect}
            channels={channelRefs}
            busy={bulkBusy}
            onMake={bulkEnqueue}
          />
        )}
      </main>
      <aside className={`vx-rail ${stylex.props(exploreStyles.rail).className}`} aria-label={picked && !selectMode ? tt.previewLabel : tt.rivalDetail}>
        {picked && !selectMode ? (
          <PreviewPanel
            video={picked}
            rival={rivals.find((r) => r.id === picked.channelId) ?? railRival}
            channels={channelRefs}
            onClose={() => setPickedId(null)}
          />
        ) : (
          railRival && (
            <>
              <RivalCard
                rival={railRival}
                color={rivalColor(railIndex)}
                rescanHours={railScan.data?.rescanHours ?? 6}
                perWeek={railPosting.data?.perWeek}
                channels={channelRefs}
                linkedChannelName={channels.find((channel) => channel.sources.some((source) => source.rivalId === railRival.id))?.name}
                onRemoved={() => {
                  setRailId(null)
                  pickRival('all')
                  void invalidate(keys.rivalScan(railRival.id))
                }}
              />
              {railScan.data && <ScanCard status={railScan.data} rivalId={railRival.id} />}
              <PostingCard schedule={railPosting.data} zone={zone} onZone={setZone} />
            </>
          )
        )}
      </aside>
    </div>
  )
}

export default ExploreScreen
