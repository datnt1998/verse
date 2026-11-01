// Trạng thái vùng danh sách của Khám phá (artboard Explore/MExplore › list).
// Tách khỏi React để chọn trạng thái theo dữ liệu API: đang quét thì chưa được
// báo “không khớp bộ lọc”, mất kết nối đứng trước lỗi (lỗi do mất kết nối).
import type { Rival } from '../../api/rivals'

export type ExploreListState = 'loading' | 'firstScan' | 'offline' | 'error' | 'filterEmpty' | 'nomatch' | 'ok'

/** Trang đầu khi thêm đối thủ: 30 video mới nhất (baseline của Trinh sát). */
export const FIRST_SCAN_TOTAL = 30

/** Đối thủ trong phạm vi đang được Trinh sát quét (lịch sử hoặc video mới). */
export function scanningRival(rivals: Rival[], scope: string): Rival | undefined {
  const pool = scope === 'all' ? rivals : rivals.filter((rival) => rival.id === scope)
  return pool.find((rival) =>
    rival.scan.state === 'queued' || rival.scan.state === 'running' ||
    rival.scan.state === 'backoff' || rival.scan.discoveryRunning)
}

export function scanProgress(rival: Rival): { done: number; total: number } {
  return { done: Math.min(rival.videoCount, FIRST_SCAN_TOTAL), total: FIRST_SCAN_TOTAL }
}

export type ListStateInput = {
  /** số video đang hiện (đã qua lọc + tìm) */
  count: number
  /** trang đầu đang về lần đầu, chưa có dữ liệu nào */
  loading: boolean
  error: unknown
  /** mất kết nối máy chủ (SSE đang thử lại) */
  offline: boolean
  /** đối thủ trong phạm vi còn đang quét */
  scanning: Rival | undefined
  /** ô tìm đang có chuỗi */
  q: string
}

export function exploreListState({ count, loading, error, offline, scanning, q }: ListStateInput): ExploreListState {
  // Còn video thì giữ danh sách kể cả khi mất kết nối — SSE sẽ tự làm mới.
  if (count > 0) return 'ok'
  if (offline) return 'offline'
  if (error) return 'error'
  if (loading) return 'loading'
  // Đang quét: video hiện dần, chưa kết luận được là không khớp bộ lọc.
  if (scanning) return 'firstScan'
  if (q) return 'nomatch'
  return 'filterEmpty'
}
