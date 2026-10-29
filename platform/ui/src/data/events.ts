// Lõi kết nối một luồng sự kiện SSE toàn cục (EventSource hoặc fetch stream).
// Nối lại bằng Last-Event-ID, backoff 1→2→4→5 s; nối lại xong → onReconnect (app
// tự invalidate các khoá đang hiển thị). `hello {subscriber}` mở đầu mỗi kết nối.
// Không có danh sách event, bảng event→khoá hay endpoint nghiệp vụ ở đây — app
// truyền `eventTypes` và các callback khi tạo client.
export type EventsStatus = {
  /** connecting: lần đầu/đang mở · open: đã nhận hello · retrying: chờ nối lại · stopped */
  state: 'connecting' | 'open' | 'retrying' | 'stopped'
  /** epoch ms lần thử kế tiếp (state = retrying) */
  reconnectAt: number | null
  /** số lần hỏng liên tiếp */
  failures: number
}

export type EventHandler = (data: unknown, meta: { type: string; id: string }) => void

type Source = { addEventListener(type: string, listener: (event: Event) => void): void; close(): void; onerror: ((event: Event) => void) | null }

export type EventsClientOptions = {
  /** Danh sách sự kiện đặt tên cần nghe (`hello` phải nằm trong đó — client mở khi nhận hello). */
  eventTypes: readonly string[]
  url?: string
  createSource?: (url: string) => Source
  /** Chỉ kiểm tra phiên sau hello nối lại; không mở vòng retry /api/me khi mất mạng. */
  probe?: () => Promise<'ok' | 'unauthorized'>
  /** thay toàn bộ danh sách theo dõi; false → subscriber đã mất, nối lại */
  postWatch?: (subscriber: string, watch: string[]) => Promise<boolean>
  backoff?: readonly number[] | (() => readonly number[])
  now?: () => number
  setTimer?: (fn: () => void, ms: number) => unknown
  clearTimer?: (handle: unknown) => void
}

export const BACKOFF_MS = [1000, 2000, 4000, 5000] as const

export interface EventsClient {
  start(): void
  stop(): void
  /** Nút “Thử ngay” / có mạng lại: bỏ chờ, nối ngay. */
  retryNow(): void
  subscribe(type: string | readonly string[] | '*', handler: EventHandler): () => void
  /** Gọi mỗi lần nối lại sau khi rớt (không gọi ở lần mở đầu tiên thành công). */
  onReconnect(fn: () => void): () => void
  onStatus(fn: () => void): () => void
  status(): EventsStatus
  /** Danh sách theo dõi (`?watch=` khi nối; đổi thì gọi `postWatch`). */
  setWatch(watch: string[]): void
}

export function createEventsClient(options: EventsClientOptions): EventsClient {
  const url = options.url ?? '/api/events'
  const createSource = options.createSource ?? ((target: string) => new EventSource(target, { withCredentials: true }))
  const now = options.now ?? Date.now
  const setTimer = options.setTimer ?? ((fn: () => void, ms: number) => setTimeout(fn, ms))
  const clearTimer = options.clearTimer ?? ((handle: unknown) => clearTimeout(handle as ReturnType<typeof setTimeout>))

  const handlers = new Map<string, Set<EventHandler>>()
  const reconnectFns = new Set<() => void>()
  const statusFns = new Set<() => void>()
  let status: EventsStatus = { state: 'stopped', reconnectAt: null, failures: 0 }
  let source: Source | null = null
  let timer: unknown = null
  let watchTimer: unknown = null
  let running = false
  let lastId = ''
  let subscriber = ''
  let watch: string[] = []
  let sentWatch = ''
  /** đã từng mở → lần mở sau là “nối lại” */
  let everOpened = false
  let generation = 0

  const setStatus = (next: Partial<EventsStatus>) => {
    status = { ...status, ...next }
    statusFns.forEach((fn) => fn())
  }

  const emit = (type: string, data: unknown, id: string) => {
    const meta = { type, id }
    handlers.get(type)?.forEach((handler) => handler(data, meta))
    handlers.get('*')?.forEach((handler) => handler(data, meta))
  }

  const targetURL = () => {
    const params = new URLSearchParams()
    if (lastId) params.set('lastEventId', lastId)
    if (watch.length) params.set('watch', watch.join(','))
    const query = params.toString()
    return query ? `${url}?${query}` : url
  }

  const syncWatch = () => {
    const wanted = watch.join(',')
    if (!subscriber || !options.postWatch || wanted === sentWatch) return
    const gen = generation
    sentWatch = wanted
    void options.postWatch(subscriber, watch).then((ok) => {
      // Subscriber hết hạn (404) → nối lại với ?watch= mới.
      if (!ok && gen === generation && running) { closeSource(); connect() }
    }, () => { if (gen === generation) sentWatch = '' })
  }

  const closeSource = () => {
    generation++
    source?.close()
    source = null
    subscriber = ''
  }

  function connect() {
    clearTimer(timer)
    timer = null
    if (!running) return
    closeSource()
    const gen = generation
    const wasOpened = everOpened
    const hadFailures = status.failures > 0
    setStatus({ state: status.failures > 0 ? 'retrying' : 'connecting', reconnectAt: null })
    sentWatch = watch.join(',')
    const next = createSource(targetURL())
    source = next
    for (const type of options.eventTypes) {
      next.addEventListener(type, (event: Event) => {
        if (gen !== generation) return
        const message = event as MessageEvent<string>
        if (message.lastEventId) lastId = message.lastEventId
        let data: unknown = null
        try { data = message.data ? JSON.parse(message.data) : null } catch { data = message.data }
        if (type === 'hello') {
          const hello = data as { subscriber?: unknown } | null
          subscriber = typeof hello?.subscriber === 'string' ? hello.subscriber : ''
          everOpened = true
          setStatus({ state: 'open', failures: 0, reconnectAt: null })
          // Sự kiện bị lỡ trong lúc rớt mà server không phát lại được → làm mới tất cả.
          if (wasOpened || hadFailures) reconnectFns.forEach((fn) => fn())
          if ((wasOpened || hadFailures) && options.probe) {
            void options.probe().then((result) => {
              if (gen === generation && result === 'unauthorized') stop()
            }, () => { /* Luồng SSE tiếp tục làm chủ việc nối lại. */ })
          }
          syncWatch()
        }
        emit(type, data, message.lastEventId ?? '')
      })
    }
    next.onerror = (event) => {
      if (gen !== generation) return
      closeSource()
      if (event && 'status' in event && event.status === 401) { stop(); return }
      fail()
    }
  }

  function fail() {
    if (!running) return
    const failures = status.failures + 1
    const backoff = typeof options.backoff === 'function' ? options.backoff() : options.backoff ?? BACKOFF_MS
    const delay = backoff[Math.min(failures - 1, backoff.length - 1)]
    setStatus({ state: 'retrying', failures, reconnectAt: now() + delay })
    timer = setTimer(connect, delay)
  }

  function stop() {
    running = false
    clearTimer(timer)
    timer = null
    clearTimer(watchTimer)
    watchTimer = null
    sentWatch = ''
    closeSource()
    everOpened = false
    lastId = ''
    setStatus({ state: 'stopped', reconnectAt: null, failures: 0 })
  }

  return {
    start() {
      if (running) return
      running = true
      connect()
    },
    stop,
    retryNow() {
      if (!running || status.state === 'open') return
      connect()
    },
    subscribe(type, handler) {
      const types = typeof type === 'string' ? [type] : [...type]
      types.forEach((name) => {
        let set = handlers.get(name)
        if (!set) { set = new Set(); handlers.set(name, set) }
        set.add(handler)
      })
      return () => types.forEach((name) => handlers.get(name)?.delete(handler))
    },
    onReconnect(fn) {
      reconnectFns.add(fn)
      return () => { reconnectFns.delete(fn) }
    },
    onStatus(fn) {
      statusFns.add(fn)
      return () => { statusFns.delete(fn) }
    },
    status: () => status,
    setWatch(next) {
      watch = [...next]
      // Một lần chuyển trang có thể đổi tập khoá vài nhịp liên tiếp (màn cũ nhả khoá,
      // màn mới giữ khoá) → gộp POST watch lại thành một lần.
      clearTimer(watchTimer)
      watchTimer = setTimeout(() => { watchTimer = null; syncWatch() }, 120)
    },
  }
}

/**
 * Máy chủ gửi `: ping` mỗi 15 s. Quá ba nhịp không nhận byte nào là luồng đã chết
 * mà không báo lỗi (điện thoại đổi Wi-Fi/4G, iOS treo tab ở nền, NAT cắt kết nối):
 * fetch vẫn treo ở reader.read() nên trạng thái cứ là "open" và dữ liệu đứng yên.
 */
export const STREAM_STALL_MS = 45_000
/** Trở lại tab / có mạng lại: lỡ hơn một nhịp ping thì nối lại ngay, không chờ đủ 45 s. */
export const STREAM_WAKE_STALL_MS = 20_000

export type FetchSourceOptions = {
  /** SSE trả 401: chạy handler phiên chung của app (làm mới cookie/đăng xuất) rồi báo lỗi để client dừng. */
  onUnauthorized?: () => unknown
  /** không nhận byte nào trong khoảng này ⇒ coi luồng đã chết (mặc định STREAM_STALL_MS = 45 s). */
  stallMs?: number
  /** nhịp kiểm tra watchdog (mặc định 5 s). */
  stallCheckMs?: number
  /** headers của response không về sau khoảng này ⇒ coi kết nối không tới được (mặc định STREAM_STALL_MS). */
  connectTimeoutMs?: number
}

/** Adaptateur fetch SSE: Last-Event-ID reste un en-tête même après reconnexion. */
export function fetchSource(url: string, options: FetchSourceOptions = {}): Source {
  const stallMs = options.stallMs ?? STREAM_STALL_MS
  const checkMs = options.stallCheckMs ?? 5000
  const connectTimeout = options.connectTimeoutMs ?? STREAM_STALL_MS
  const controller = new AbortController()
  const target = new EventTarget()
  let stalled = false
  let lastByte = Date.now()
  let responded = false
  let reader: ReadableStreamDefaultReader<Uint8Array> | null = null
  const stall = (limit: number) => {
    if (controller.signal.aborted || Date.now() - lastByte < limit) return
    stalled = true
    controller.abort()
    // Không trông vào việc abort lan tới body: huỷ reader để read() đang treo trả về ngay.
    void reader?.cancel().catch(() => {})
  }
  const watchdog = setInterval(() => stall(stallMs), checkMs)
  // Headers không về là kết nối không tới được — đừng chờ đủ một nhịp stall.
  const connectTimer = setTimeout(() => { if (!responded) { stalled = true; controller.abort() } }, connectTimeout)
  // Trở lại tab / có mạng lại: nối lại sớm nếu quá nhịp ping (nhưng không sớm hơn stallMs).
  const wake = () => { if (document.visibilityState === 'visible') stall(Math.min(STREAM_WAKE_STALL_MS, stallMs)) }
  document.addEventListener('visibilitychange', wake)
  window.addEventListener('online', wake)
  const release = () => {
    clearInterval(watchdog)
    clearTimeout(connectTimer)
    document.removeEventListener('visibilitychange', wake)
    window.removeEventListener('online', wake)
  }
  const result: Source = {
    addEventListener: target.addEventListener.bind(target),
    close: () => { release(); controller.abort() },
    onerror: null,
  }
  const endpoint = new URL(url, location.href)
  const lastId = endpoint.searchParams.get('lastEventId')
  endpoint.searchParams.delete('lastEventId')
  void (async () => {
    const response = await fetch(endpoint, { credentials: 'same-origin', signal: controller.signal, headers: { Accept: 'text/event-stream', ...(lastId ? { 'Last-Event-ID': lastId } : {}) } })
    lastByte = Date.now()
    responded = true
    clearTimeout(connectTimer)
    // Máy chủ đã trả 401 (không phải outage): dùng handler phiên chung, rồi dừng SSE.
    if (response.status === 401) {
      release()
      await Promise.resolve(options.onUnauthorized?.()).catch(() => {})
      if (!controller.signal.aborted) result.onerror?.(Object.assign(new Event('error'), { status: 401 }))
      return
    }
    if (!response.ok || !response.body) throw new Error('Không mở được luồng sự kiện')
    reader = response.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let type = 'message'
    let id = lastId ?? ''
    let data: string[] = []
    try {
      while (!controller.signal.aborted) {
        const chunk = await reader.read()
        if (chunk.done) break
        lastByte = Date.now()
        buffer += decoder.decode(chunk.value, { stream: true })
        let end: number
        while ((end = buffer.indexOf('\n')) >= 0) {
          const line = buffer.slice(0, end).replace(/\r$/, '')
          buffer = buffer.slice(end + 1)
          if (!line) {
            if (data.length) target.dispatchEvent(new MessageEvent(type, { data: data.join('\n'), lastEventId: id }))
            data = []; type = 'message'
          } else if (!line.startsWith(':')) {
            const colon = line.indexOf(':')
            const field = colon < 0 ? line : line.slice(0, colon)
            const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '')
            if (field === 'event') type = value
            else if (field === 'id' && !value.includes('\0')) id = value
            else if (field === 'data') data.push(value)
          }
        }
      }
    } finally { reader.releaseLock() }
    if (stalled || !controller.signal.aborted) throw new Error('Luồng sự kiện đã ngắt')
  })().catch(() => {
    // Luồng treo bị watchdog cắt vẫn phải báo lỗi để client nối lại (và làm mới dữ liệu).
    if (!controller.signal.aborted || stalled) { release(); result.onerror?.(new Event('error')) }
  })
  return result
}
