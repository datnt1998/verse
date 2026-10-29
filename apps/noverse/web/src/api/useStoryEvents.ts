// Sự kiện của một truyện: snapshot · event · delta · clear · done · cocreate · job.
// KHÔNG mở EventSource riêng: mọi khung worker được máy chủ ghép vào luồng toàn cục
// /api/events (`live` + `{project, story, kind, data}`), nên mỗi tab chỉ giữ MỘT
// kết nối SSE — HTTP/1.1 giới hạn 6 kết nối/nguồn, mở thêm luồng là request xếp hàng
// và màn hình trông như mất dữ liệu rồi có lại.
// Delta vẫn gom theo requestAnimationFrame; dữ liệu cũ được giữ nguyên khi rớt
// mạng (cache SWR) và bản ghi cuối được dùng lại khi máy chủ không phát lại được.
import { useCallback, useEffect, useRef, useState } from 'react'
import { ApiError, getSnapshot } from './client'
import type { HostEvent, JobEvent, Snapshot } from './types'
import { cache, keys, setData, subscribeEvents, useConnection, useResource } from '../data'
import { deduplicateUserEvents, isInternalEvent } from '../lib/activity'

export type StoryEvents = {
  snapshot: Snapshot | null
  events: HostEvent[]
  streamText: string
  /** delta đồng sáng tác (SSE `cocreate`), xóa khi reset/commit */
  cocreateText: string
  /** job id → trạng thái mới nhất (SSE `job`) */
  jobs: Map<string, JobEvent>
  connected: boolean
  /** worker từ chối: 'locked' (409 errBookLocked) · 'forbidden' (403) — không retry */
  blocked: 'locked' | 'forbidden' | null
  /** epoch ms lần kết nối lại kế tiếp; null khi đang kết nối */
  reconnectAt: number | null
  /** tăng mỗi sự kiện `done` → refetch dữ liệu phụ thuộc */
  revision: number
  /** ép thử kết nối lại ngay (nút Thử ngay của ConnectionBanner) */
  retryNow: () => void
  loading: boolean
  refreshing: boolean
  updatedAt: number | null
  error: unknown
  refresh: () => Promise<unknown>
}

const MAX_EVENTS = 1000

/** `delta`/`cocreate` mang `{text}`; giá trị lạ (khung hỏng) coi như rỗng. */
const streamTextOf = (data: unknown): string => {
  if (!data || typeof data !== 'object' || !('text' in data)) return ''
  return typeof data.text === 'string' ? data.text : ''
}

/** Khung thực thể do máy chủ ghép vào luồng chung: `{project, story, kind, data}`. */
export type LiveFrame = { project: string; story: string; kind: string; data: unknown }

export const readLive = (payload: unknown): LiveFrame | null => {
  if (!payload || typeof payload !== 'object') return null
  if (!('project' in payload) || !('story' in payload) || !('kind' in payload)) return null
  const { project, story, kind } = payload
  if (typeof project !== 'string' || typeof story !== 'string' || typeof kind !== 'string') return null
  return { project, story, kind, data: 'data' in payload ? payload.data : null }
}

/**
 * Phần khung chỉ chạm cache (snapshot · event · events). Không bao giờ xoá dữ liệu
 * đã có: nối lại chỉ hợp nhất theo id, nên danh sách không trắng rồi mới đầy lại.
 * Trả về 'stream-clear'/'done' để hook xử lý phần state cục bộ.
 */
export function applyLiveToCache(p: string, s: string, frame: LiveFrame): 'stream-clear' | 'done' | null {
  const eventKey = ['story', p, s, 'events']
  const merge = (batch: HostEvent[]) => {
    if (!batch.length) return
    setData<HostEvent[]>(eventKey, (cached) => {
      const byID = new Map((cached ?? []).map((item) => [item.id, item]))
      batch.forEach((item) => byID.set(item.id, item))
      return deduplicateUserEvents([...byID.values()].filter((item) => !isInternalEvent(item))).slice(-MAX_EVENTS)
    })
  }
  switch (frame.kind) {
    case 'snapshot':
      if (frame.data && typeof frame.data === 'object') setData(keys.story(p, s, 'snapshot'), frame.data as Snapshot)
      return null
    case 'event':
      if (frame.data && typeof frame.data === 'object') merge([frame.data as HostEvent])
      return null
    case 'events': {
      if (!Array.isArray(frame.data)) return null
      merge(frame.data.filter((item): item is HostEvent =>
        !!item && typeof item === 'object' && 'id' in item && typeof item.id === 'string'))
      return null
    }
    case 'clear':
    case 'reset':
      return 'stream-clear'
    case 'done':
      return 'done'
    default:
      return null
  }
}

export function useStoryEvents(p: string, s: string, _onUnauthorized: () => void): StoryEvents {
  const resource = useResource(keys.story(p, s, 'snapshot'), ({ signal }) => getSnapshot(p, s, signal))
  const snapshot = resource.data ?? null
  const eventResource = useResource<HostEvent[]>(['story', p, s, 'events'], async () =>
    cache.get<HostEvent[]>(['story', p, s, 'events']).data ?? [])
  const events = eventResource.data ?? []
  const connection = useConnection()
  const [streamText, setStreamText] = useState('')
  const [cocreateText, setCoCreateText] = useState('')
  const [jobs, setJobs] = useState<Map<string, JobEvent>>(() => new Map())
  const [revision, setRevision] = useState(0)

  useEffect(() => {
    // rAF gom delta để mỗi khung hình chỉ render một lần.
    let pendingDelta = ''
    let pendingCo = ''
    let raf: number | undefined
    const flush = () => {
      raf = undefined
      if (pendingDelta) { const text = pendingDelta; pendingDelta = ''; setStreamText((prev) => prev + text) }
      if (pendingCo) { const text = pendingCo; pendingCo = ''; setCoCreateText((prev) => prev + text) }
    }
    const queueDelta = (text: string, cocreate: boolean) => {
      if (!text) return
      if (cocreate) pendingCo += text; else pendingDelta += text
      if (raf === undefined) raf = requestAnimationFrame(flush)
    }
    const apply = (frame: LiveFrame) => {
      switch (frame.kind) {
        case 'delta':
          queueDelta(streamTextOf(frame.data), false)
          break
        case 'cocreate':
          queueDelta(streamTextOf(frame.data), true)
          break
        case 'job':
          if (frame.data && typeof frame.data === 'object') {
            const job = frame.data as JobEvent
            setJobs((previous) => { const next = new Map(previous); next.set(job.job, job); return next })
          }
          break
        case 'jobs.reset':
          if (frame.data && typeof frame.data === 'object' && 'jobs' in frame.data && Array.isArray(frame.data.jobs)) {
            const list: JobEvent[] = frame.data.jobs
            setJobs(() => new Map(list.map((job) => [job.job, job])))
          }
          break
        default: {
          const effect = applyLiveToCache(p, s, frame)
          if (effect === 'stream-clear') {
            pendingDelta = ''
            setStreamText('')
          } else if (effect === 'done') {
            setRevision((value) => value + 1)
          }
        }
      }
    }
    const offLive = subscribeEvents('live', (payload) => {
      const frame = readLive(payload)
      if (!frame || frame.project !== p || frame.story !== s) return
      apply(frame)
    })
    // Máy chủ bỏ qua một đoạn lịch sử (ring đã cuốn) → bỏ phần văn bản đang stream,
    // danh sách sự kiện vẫn giữ bản cuối và tự làm mới qua cache.
    const offReset = subscribeEvents('reset', () => {
      pendingDelta = ''
      pendingCo = ''
      setStreamText('')
      setCoCreateText('')
    })
    return () => {
      offLive()
      offReset()
      if (raf !== undefined) cancelAnimationFrame(raf)
    }
  }, [p, s])

  const retryRef = useRef(connection.retry)
  retryRef.current = connection.retry
  const retryNow = useCallback(() => retryRef.current(), [])
  // 403/409 khi lấy snapshot (truyện bị khoá / không có quyền) là trạng thái cuối,
  // không phải lỗi mạng — màn hình tự hiển thị lời giải thích.
  const blocked: StoryEvents['blocked'] = resource.error instanceof ApiError && (resource.error.status === 403 || resource.error.status === 409)
    ? (resource.error.status === 409 ? 'locked' : 'forbidden')
    : null

  return {
    snapshot, events, streamText, cocreateText, jobs,
    connected: connection.state === 'open',
    blocked,
    reconnectAt: connection.reconnectAt,
    revision, retryNow,
    loading: resource.initialLoading, refreshing: resource.refreshing, updatedAt: resource.updatedAt,
    error: resource.error, refresh: resource.refresh,
  }
}
