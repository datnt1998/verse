// Danh sách video của lưới Khám phá: trang đầu đi qua cache khoá (SSE `rivals`
// invalidate → tải lại trong nền, không giật màn); các trang sau nạp thêm bằng
// cursor khi cửa sổ cuộn gần đáy. Khi trang đầu đổi (SSE), giữ mọi video đã
// nạp mà trang đầu mới không mang theo → video mới của sort "Mới nhất" tự trôi
// lên đầu mà lựa chọn và vị trí cuộn không mất.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { listRivalVideos, type Video, type VideoPage, type VideoQuery, type VideoSort } from '../../api/rivals'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'

export const VIDEO_PAGE_LIMIT = 200

export type VideosFilter = { scope: string; sort: VideoSort; long: boolean; within: 0 | 7 | 30 | 90; q: string }
const queryOf = (f: VideosFilter, cursor: string | null): VideoQuery => ({
  sort: f.sort,
  long: f.long ? 1 : 0,
  within: f.within,
  q: f.q || undefined,
  cursor,
  limit: VIDEO_PAGE_LIMIT,
})

export type RivalVideos = {
  items: Video[]
  /** `total` của trang đầu mới nhất (sau lọc và tìm) — hiện "24 / 1.284 video". */
  total: number | undefined
  /** chuỗi tìm của dữ liệu đang hiện: khi đổi tìm, dữ liệu cũ còn hiện tới khi
   *  trang mới về, nên dòng đếm và khối "không có kết quả" đọc chuỗi này. */
  shownQ: string
  initialLoading: boolean
  error: unknown
  loadingMore: boolean
  hasMore: boolean
  needMore: () => void
  /** Ảnh chụp trang đầu đổi (SSE đưa video mới lên đầu) — dùng để neo cuộn. */
  revision: VideoPage | undefined
  /** Tải lại trang đầu (nút Thử lại của trạng thái lỗi/mất kết nối). */
  refresh: () => Promise<unknown>
}

export function useRivalVideos(filter: VideosFilter): RivalVideos {
  const first = useResource(
    keys.rivalVideos(filter.scope, filter.sort, filter.long ? 1 : 0, filter.within, filter.q),
    ({ signal }) => listRivalVideos(filter.scope, queryOf(filter, null), signal),
    { keepPrevious: true },
  )
  const [extra, setExtra] = useState<Video[]>([])
  const [cursor, setCursor] = useState<string | null>(null)
  const [moreError, setMoreError] = useState<unknown>(null)
  const seq = useRef(0)
  const cursorRef = useRef<string | null>(null)
  const scopeRef = useRef(filter)
  cursorRef.current = cursor
  scopeRef.current = filter

  // Bộ lọc đổi → bỏ hết trang cũ; dữ liệu mới (hoặc trang đầu do SSE làm mới)
  // đặt lại cursor nối tiếp. Đang hiện tạm dữ liệu của bộ lọc trước thì chưa có
  // cursor: cursor đó thuộc truy vấn khác.
  const firstData = first.data
  const placeholder = first.placeholder
  useEffect(() => {
    setExtra([])
    setCursor(placeholder ? null : firstData?.nextCursor ?? null)
    setMoreError(null)
  }, [filter.scope, filter.sort, filter.long, filter.within, filter.q, firstData, placeholder])
  // Cập nhật ngay trong lượt render có dữ liệu mới (mẫu "lưu từ render trước"
  // của React) để dòng đếm không chớp chuỗi cũ một khung hình.
  const [shownQ, setShownQ] = useState(filter.q)
  if (!placeholder && firstData !== undefined && shownQ !== filter.q) setShownQ(filter.q)

  const [loadingMore, setLoadingMore] = useState(false)
  const busyRef = useRef(false)
  const needMore = useCallback(() => {
    const cur = cursorRef.current
    const f = scopeRef.current
    if (!cur || busyRef.current) return
    busyRef.current = true
    const ticket = ++seq.current
    setLoadingMore(true)
    listRivalVideos(f.scope, queryOf(f, cur))
      .then((page) => {
        // Bỏ trang trễ: cursor đã đổi (đã nạp hoặc reset từ trang đầu mới).
        if (ticket !== seq.current || cursorRef.current !== cur) return
        setExtra((prev) => {
          const seen = new Set(prev.map((v) => v.id))
          return [...prev, ...page.videos.filter((v) => !seen.has(v.id))]
        })
        setCursor(page.nextCursor)
      })
      .catch((err) => { if (ticket === seq.current) setMoreError(err) })
      .finally(() => {
        busyRef.current = false
        if (ticket === seq.current) setLoadingMore(false)
      })
  }, [])

  const items = useMemo(() => {
    const seen = new Set<string>()
    const head = firstData?.videos ?? []
    const tail: Video[] = []
    for (const v of head) if (!seen.has(v.id)) { seen.add(v.id); tail.push(v) }
    for (const v of extra) if (!seen.has(v.id)) { seen.add(v.id); tail.push(v) }
    return tail
  }, [firstData, extra])

  return {
    items,
    /** `total` của trang đầu mới nhất (sau lọc và tìm) — hiện "24 / 1.284 video". */
    total: firstData?.total,
    shownQ,
    initialLoading: first.initialLoading,
    error: first.error ?? moreError,
    loadingMore,
    hasMore: cursor !== null,
    needMore,
    /** Ảnh chụp trang đầu đổi (SSE đưa video mới lên đầu) — dùng để neo cuộn. */
    revision: firstData,
    refresh: first.refresh,
  }
}
