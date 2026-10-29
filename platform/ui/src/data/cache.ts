// Bộ nhớ đệm theo khoá dùng chung giữa các app — không phụ thuộc React để test được bằng node.
// stale-while-revalidate: tải lại không bao giờ xoá dữ liệu cũ; lỗi tải lại chỉ gắn `error`.
// Gộp yêu cầu trùng đang bay; huỷ (AbortController) khi không còn ai giữ khoá.
import { keyHash, matchesPrefix, type Key } from './keys'

export type ResourceStatus = 'loading' | 'ready' | 'error'

export type Fetcher<T> = (ctx: { signal: AbortSignal; key: Key }) => Promise<T>

/** Ảnh chụp bất biến của một mục — đổi tham chiếu mỗi khi mục đổi (useSyncExternalStore). */
export type EntrySnapshot<T = unknown> = {
  data: T | undefined
  error: unknown
  status: ResourceStatus
  /** đang tải lại khi đã có dữ liệu (vạch 2 px), không tính lần tải đầu */
  refreshing: boolean
  /** epoch ms lần tải thành công gần nhất */
  updatedAt: number | null
}

type Entry = {
  key: Key
  snap: EntrySnapshot
  listeners: Set<() => void>
  /** số nơi đang hiển thị khoá (useResource đang mount) */
  holders: number
  fetcher: Fetcher<unknown> | null
  inflight: { promise: Promise<unknown>; controller: AbortController } | null
  /** invalidate lúc đang bay → tải lại khi xong (dữ liệu đang bay có thể đã cũ) */
  again: boolean
  /** bị invalidate khi không ai giữ → lần giữ tới phải tải lại */
  stale: boolean
  /** epoch ms lần bắt đầu tải gần nhất (giới hạn focus) */
  lastFetchAt: number
  lastUsedAt: number
}

export type CacheOptions = {
  now?: () => number
  /** hẹn giờ huỷ khi hết người giữ (tách ra để StrictMode mount→unmount→mount không huỷ oan) */
  defer?: (fn: () => void) => void
  /** số mục không ai giữ tối đa còn lưu lại */
  maxIdle?: number
}

const EMPTY: EntrySnapshot = { data: undefined, error: null, status: 'loading', refreshing: false, updatedAt: null }

export interface ResourceCache {
  /** Ảnh chụp hiện tại (EMPTY khi chưa có mục). */
  get<T>(key: Key): EntrySnapshot<T>
  /** Nghe thay đổi của một khoá (không làm khoá “đang hiển thị”). */
  listen(key: Key, listener: () => void): () => void
  /**
   * Giữ khoá (màn đang hiển thị): đăng ký fetcher, tải nếu chưa có / đã cũ / quá `staleTime`.
   * Trả hàm nhả; khi hết người giữ thì yêu cầu đang bay bị huỷ.
   */
  retain<T>(key: Key, fetcher: Fetcher<T>, opts?: { staleTime?: number }): () => void
  /** Tải lại ngay (nút Thử lại / refresh()); đang bay thì tải thêm một lần khi xong. */
  refresh(key: Key): Promise<unknown>
  /** Imperative consumers share the same refresh bookkeeping and in-flight request. */
  fetch<T>(key: Key, fetcher: Fetcher<T>): Promise<unknown>
  /**
   * Nạp trước (rê chuột/focus vào mục điều hướng): tải khoá mà không cần ai “giữ”.
   * Không huỷ khi không còn người giữ — dữ liệu nằm lại trong bộ đệm cho lần mở thật.
   */
  prefetch<T>(key: Key, fetcher: Fetcher<T>, opts?: { staleTime?: number }): void
  /** Đặt dữ liệu trực tiếp (kết quả ghi / lạc quan); trả giá trị cũ để hoàn tác. */
  setData<T>(key: Key, updater: T | ((prev: T | undefined) => T)): T | undefined
  /** Trả lại dữ liệu cũ (hoàn tác lạc quan) — không đổi updatedAt. */
  restore(key: Key, data: unknown): void
  /** Khoá khớp prefix: đang hiển thị → tải lại ngầm ngay; không ai giữ → tải lại ở lần hiển thị tới. */
  invalidate(prefix?: Key): Promise<unknown>
  /** Tải lại khoá đang hiển thị, tối đa 1 lần mỗi `minInterval` mỗi khoá (focus/hiện tab/có mạng). */
  revalidateActive(minInterval: number): Promise<unknown>
  /** Khoá đang hiển thị (để báo server danh sách truyện cần theo dõi). */
  activeKeys(): Key[]
  /** Nghe khi tập khoá đang hiển thị đổi. */
  onActiveChange(listener: () => void): () => void
  /** Background refreshes are observed once by app chrome, never by resource content. */
  isRefreshing(prefix?: Key): boolean
  onRefreshingChange(listener: () => void): () => void
  /** Đăng xuất / đổi tài khoản: bỏ mọi dữ liệu (quyền khác nhau). */
  clear(): void
}
/** JSON resources retain their reference when a refresh/event carries identical data. */
export function sameData(a: unknown, b: unknown): boolean {
  if (Object.is(a, b)) return true
  if (!a || !b || typeof a !== 'object' || typeof b !== 'object' || Array.isArray(a) !== Array.isArray(b)) return false
  const left = a as Record<string, unknown>
  const right = b as Record<string, unknown>
  const names = Object.keys(left)
  return names.length === Object.keys(right).length && names.every(name => Object.hasOwn(right, name) && sameData(left[name], right[name]))
}

function shareData<T>(previous: T, next: T): T {
  if (sameData(previous, next)) return previous
  if (!previous || !next || typeof previous !== 'object' || typeof next !== 'object') return next
  if (Array.isArray(previous) && Array.isArray(next)) return next.map((value, index) => shareData(previous[index], value)) as T
  if (Array.isArray(previous) || Array.isArray(next)) return next
  const before = previous as Record<string, unknown>
  const result: Record<string, unknown> = {}
  for (const [key, value] of Object.entries(next)) result[key] = shareData(before[key], value)
  return result as T
}

export function createCache(options: CacheOptions = {}): ResourceCache {
  const now = options.now ?? Date.now
  const defer = options.defer ?? ((fn) => { setTimeout(fn, 0) })
  const maxIdle = options.maxIdle ?? 200
  const entries = new Map<string, Entry>()
  const activeListeners = new Set<() => void>()
  const refreshingListeners = new Set<() => void>()
  let refreshingCount = 0

  const entryOf = (key: Key): Entry => {
    const hash = keyHash(key)
    let entry = entries.get(hash)
    if (!entry) {
      entry = {
        key, snap: EMPTY, listeners: new Set(), holders: 0, fetcher: null, inflight: null,
        again: false, stale: false, lastFetchAt: 0, lastUsedAt: now(),
      }
      entries.set(hash, entry)
    }
    return entry
  }

  const update = (entry: Entry, patch: Partial<EntrySnapshot>) => {
    if ('data' in patch && sameData(entry.snap.data, patch.data)) patch.data = entry.snap.data
    if (Object.keys(patch).every(name => Object.is(entry.snap[name as keyof EntrySnapshot], patch[name as keyof EntrySnapshot]))) return
    const refreshingChanged = patch.refreshing !== undefined && patch.refreshing !== entry.snap.refreshing
    if (refreshingChanged) refreshingCount += patch.refreshing ? 1 : -1
    entry.snap = { ...entry.snap, ...patch }
    if (refreshingChanged) refreshingListeners.forEach((listener) => listener())
    entry.listeners.forEach((listener) => listener())
  }

  function load(entry: Entry): Promise<unknown> {
    if (entry.inflight) return entry.inflight.promise
    const fetcher = entry.fetcher
    if (!fetcher) return Promise.resolve(entry.snap.data)
    const controller = new AbortController()
    entry.lastFetchAt = now()
    entry.stale = false
    if (entry.snap.data !== undefined) update(entry, { refreshing: true })
    else if (entry.snap.status !== 'loading') update(entry, { status: 'loading' })
    const promise = fetcher({ signal: controller.signal, key: entry.key }).then(
      (data) => {
        if (entry.inflight?.controller !== controller || controller.signal.aborted) return entry.snap.data
        entry.inflight = null
        const shared = shareData(entry.snap.data, data)
        update(entry, { data: shared, error: null, status: 'ready', refreshing: false, updatedAt: now() })
        return shared
      },
      (error: unknown) => {
        if (entry.inflight?.controller !== controller || controller.signal.aborted) return entry.snap.data
        entry.inflight = null
        // SWR: có dữ liệu cũ thì giữ, chỉ gắn lỗi (StaleBanner); chưa có → trạng thái lỗi của panel.
        update(entry, {
          error, refreshing: false,
          status: entry.snap.data !== undefined ? 'ready' : 'error',
        })
        return entry.snap.data
      },
    ).finally(() => {
      if (entry.again && entry.holders > 0) { entry.again = false; void load(entry) }
    })
    entry.inflight = { promise, controller }
    return promise
  }

  const notifyActive = () => activeListeners.forEach((listener) => listener())

  function prune() {
    const idle = [...entries.entries()].filter(([, entry]) => entry.holders === 0 && entry.listeners.size === 0 && !entry.inflight)
    if (idle.length <= maxIdle) return
    idle.sort((a, b) => a[1].lastUsedAt - b[1].lastUsedAt)
    idle.slice(0, idle.length - maxIdle).forEach(([hash]) => entries.delete(hash))
  }

  return {
    /** Ảnh chụp hiện tại (EMPTY khi chưa có mục). */
    get<T>(key: Key): EntrySnapshot<T> {
      return (entries.get(keyHash(key))?.snap ?? EMPTY) as EntrySnapshot<T>
    },

    /** Nghe thay đổi của một khoá (không làm khoá “đang hiển thị”). */
    listen(key: Key, listener: () => void): () => void {
      const entry = entryOf(key)
      entry.listeners.add(listener)
      return () => { entry.listeners.delete(listener) }
    },

    /**
     * Giữ khoá (màn đang hiển thị): đăng ký fetcher, tải nếu chưa có / đã cũ / quá `staleTime`.
     * Trả hàm nhả; khi hết người giữ thì yêu cầu đang bay bị huỷ.
     */
    retain<T>(key: Key, fetcher: Fetcher<T>, opts: { staleTime?: number } = {}): () => void {
      const entry = entryOf(key)
      entry.fetcher = fetcher as Fetcher<unknown>
      entry.holders++
      entry.lastUsedAt = now()
      if (entry.holders === 1) notifyActive()
      const staleTime = opts.staleTime ?? 2000
      const old = entry.snap.updatedAt === null || now() - entry.snap.updatedAt >= staleTime
      if (entry.stale || old || entry.snap.data === undefined) void load(entry)
      let released = false
      return () => {
        if (released) return
        released = true
        entry.holders--
        entry.lastUsedAt = now()
        if (entry.holders > 0) return
        defer(() => {
          if (entry.holders > 0) return
          notifyActive()
          if (entry.inflight) {
            entry.inflight.controller.abort()
            entry.inflight = null
            entry.again = false
            // Huỷ giữa chừng → lần sau phải tải lại; giữ nguyên dữ liệu cũ.
            entry.stale = true
            if (entry.snap.refreshing) update(entry, { refreshing: false })
          }
          prune()
        })
      }
    },

    /** Tải lại ngay (nút Thử lại / refresh()). */
    refresh(key: Key): Promise<unknown> {
      const entry = entryOf(key)
      if (entry.inflight) { entry.again = true; return entry.inflight.promise }
      return load(entry)
    },

    fetch<T>(key: Key, fetcher: Fetcher<T>) {
      const entry = entryOf(key)
      entry.fetcher = fetcher as Fetcher<unknown>
      return load(entry)
    },

    prefetch<T>(key: Key, fetcher: Fetcher<T>, opts: { staleTime?: number } = {}) {
      const entry = entryOf(key)
      entry.fetcher = fetcher as Fetcher<unknown>
      entry.lastUsedAt = now()
      if (entry.inflight) return
      const staleTime = opts.staleTime ?? 15000
      const old = entry.snap.updatedAt === null || now() - entry.snap.updatedAt >= staleTime
      if (entry.stale || old || entry.snap.data === undefined) void load(entry)
    },

    /**
     * Đặt dữ liệu trực tiếp (kết quả ghi / cập nhật lạc quan). Trả giá trị cũ để hoàn tác.
     * `updater` nhận dữ liệu hiện tại (có thể undefined).
     */
    setData<T>(key: Key, updater: T | ((prev: T | undefined) => T)): T | undefined {
      const entry = entryOf(key)
      const prev = entry.snap.data as T | undefined
      const incoming = typeof updater === 'function' ? (updater as (prev: T | undefined) => T)(prev) : updater
      const next = shareData(prev, incoming)
      // Dữ liệu đặt tay mới hơn mọi yêu cầu đang bay → bỏ kết quả đó.
      if (entry.inflight) { entry.inflight.controller.abort(); entry.inflight = null }
      update(entry, { data: next, error: null, status: next === undefined ? 'loading' : 'ready', refreshing: false, updatedAt: sameData(prev, next) ? entry.snap.updatedAt : now() })
      return prev
    },

    /** Trả lại dữ liệu cũ (hoàn tác lạc quan) — không đổi updatedAt. */
    restore(key: Key, data: unknown) {
      const entry = entryOf(key)
      update(entry, { data, status: data === undefined ? 'loading' : 'ready' })
    },

    /**
     * Đánh dấu cũ mọi khoá khớp prefix: khoá đang hiển thị tải lại ngầm ngay,
     * khoá không ai giữ tải lại ở lần hiển thị tới. Prefix rỗng = tất cả.
     */
    invalidate(prefix: Key = []): Promise<unknown> {
      const work: Promise<unknown>[] = []
      entries.forEach((entry) => {
        if (!matchesPrefix(entry.key, prefix)) return
        if (entry.holders === 0) { entry.stale = true; return }
        if (entry.inflight) { entry.again = true; work.push(entry.inflight.promise); return }
        work.push(load(entry))
      })
      return Promise.all(work)
    },

    /** Focus/hiện tab/có mạng: tải lại khoá đang hiển thị, tối đa 1 lần mỗi `minInterval` mỗi khoá. */
    revalidateActive(minInterval: number): Promise<unknown> {
      const work: Promise<unknown>[] = []
      const at = now()
      entries.forEach((entry) => {
        if (entry.holders === 0 || entry.inflight || at - entry.lastFetchAt < minInterval) return
        work.push(load(entry))
      })
      return Promise.all(work)
    },

    /** Khoá đang hiển thị (để báo server danh sách truyện cần theo dõi). */
    activeKeys(): Key[] {
      return [...entries.values()].filter((entry) => entry.holders > 0).map((entry) => entry.key)
    },

    /** Nghe khi tập khoá đang hiển thị đổi. */
    onActiveChange(listener: () => void): () => void {
      activeListeners.add(listener)
      return () => { activeListeners.delete(listener) }
    },

    isRefreshing(prefix) {
      if (!prefix) return refreshingCount > 0
      for (const entry of entries.values()) if (entry.snap.refreshing && matchesPrefix(entry.key, prefix)) return true
      return false
    },
    onRefreshingChange(listener) {
      refreshingListeners.add(listener)
      return () => { refreshingListeners.delete(listener) }
    },

    /** Đăng xuất / đổi tài khoản: bỏ mọi dữ liệu (quyền khác nhau). */
    clear() {
      entries.forEach((entry) => {
        entry.inflight?.controller.abort()
        entry.inflight = null
        entry.stale = true
        update(entry, EMPTY)
      })
      entries.forEach((entry, hash) => { if (entry.holders === 0 && entry.listeners.size === 0) entries.delete(hash) })
    },
  }
}

/** Cache dùng chung của app. */
export const cache = createCache()

export const invalidate = (prefix?: Key) => cache.invalidate(prefix)
export const setData = <T>(key: Key, updater: T | ((prev: T | undefined) => T)) => cache.setData<T>(key, updater)
export const getData = <T>(key: Key) => cache.get<T>(key).data
