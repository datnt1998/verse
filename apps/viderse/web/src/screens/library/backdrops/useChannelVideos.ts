// Danh mục video của một kênh nền: trang đầu đi qua cache (SSE `backdrop`
// invalidate → tự nạp lại), trang sau nạp tay theo con trỏ rồi ghép — mục
// trùng lấy bản mới hơn. Đổi kênh hoặc đổi bộ lọc kind thì đặt lại từ đầu.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import { listChannelVideos, type BackdropVideoPage, type ChannelVideoItem } from '../../../api/backdrop'

export const CHANNEL_PAGE_LIMIT = 24

export type ChannelVideos = {
  items: ChannelVideoItem[]
  /** counts/total của trang đầu (đã mới nhất) */
  counts: Partial<Record<string, number>>
  total: number
  unavailable: number
  hasMore: boolean
  initialLoading: boolean
  loadingMore: boolean
  error: unknown
  loadMore: () => void
}

export function useChannelVideos(channelId: string | undefined, kinds: string[]): ChannelVideos {
  const kindKey = kinds.join(',')
  const first = useResource(
    channelId ? keys.backdropChannelVideos(channelId, kindKey) : ['backdrop', 'none'],
    ({ signal }) => channelId
      ? listChannelVideos(channelId, { kinds, limit: CHANNEL_PAGE_LIMIT }, signal)
      : Promise.resolve<BackdropVideoPage>({ items: [], counts: {}, total: 0, unavailable: 0 }),
    { enabled: channelId !== undefined },
  )
  const [extra, setExtra] = useState<ChannelVideoItem[]>([])
  const [cursor, setCursor] = useState<string | undefined>(undefined)
  const [moreError, setMoreError] = useState<unknown>(null)
  const [loadingMore, setLoadingMore] = useState(false)
  // Chỉ nhận dữ liệu của đúng kênh + bộ lọc đang hiện (response cũ về sau).
  const scope = useRef('')
  const cursorRef = useRef<string | undefined>(undefined)

  useEffect(() => {
    scope.current = `${channelId ?? ''}:${kindKey}`
    setExtra([])
    setCursor(undefined)
    setMoreError(null)
    cursorRef.current = first.data?.nextCursor
  }, [channelId, kindKey])

  // Con trỏ mới nhất của trang đầu khi SSE nạp lại — trang sau đi tiếp từ đó.
  useEffect(() => {
    if (scope.current === `${channelId ?? ''}:${kindKey}`) cursorRef.current = first.data?.nextCursor ?? cursorRef.current
    if (first.data) setCursor(first.data.nextCursor)
  }, [first.data, channelId, kindKey])

  const loadMore = useCallback(() => {
    const next = cursorRef.current
    if (!channelId || !next || loadingMore) return
    const mine = scope.current
    setLoadingMore(true)
    setMoreError(null)
    void listChannelVideos(channelId, { kinds: kindKey ? kindKey.split(',') : undefined, cursor: next, limit: CHANNEL_PAGE_LIMIT })
      .then((page) => {
        if (scope.current !== mine) return
        cursorRef.current = page.nextCursor
        setCursor(page.nextCursor)
        setExtra((prev) => [...prev, ...page.items])
      })
      .catch((err) => { if (scope.current === mine) setMoreError(err) })
      .finally(() => { if (scope.current === mine) setLoadingMore(false) })
  }, [channelId, kindKey, loadingMore])

  const items = useMemo(() => {
    const seen = new Set<string>()
    const out: ChannelVideoItem[] = []
    for (const item of [...(first.data?.items ?? []), ...extra]) {
      if (seen.has(item.videoId)) continue
      seen.add(item.videoId)
      out.push(item)
    }
    return out
  }, [first.data, extra])

  const data = first.data
  return {
    items,
    counts: data?.counts ?? {},
    total: data?.total ?? 0,
    unavailable: data?.unavailable ?? 0,
    hasMore: cursor !== undefined,
    initialLoading: first.initialLoading,
    loadingMore,
    error: moreError ?? first.error,
    loadMore,
  }
}
