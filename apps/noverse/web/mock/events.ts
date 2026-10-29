import type { IncomingMessage, ServerResponse } from 'node:http'
import type { User } from '../src/api/types'

type EventRecord = { id: number; type: string; data: unknown }
type Client = { id: string; res: ServerResponse; user: User; watch: string[] }
const clients = new Map<string, Client>()
const replay: EventRecord[] = []
let sequence = 0
let subscription = 0
function allowed(client: Client, event: EventRecord) {
  const data = event.data as { project?: string; story?: string; username?: string } | null
  if (data?.username && (event.type === 'session.revoked' || event.type === 'account.changed')) return client.user.role === 'admin' || data.username === client.user.username
  if (!data?.project) return true
  return !client.user.disabled && (client.user.role === 'admin' || !!client.user.projects[data.project])
}
function send(client: Client, event: EventRecord) {
  if (allowed(client, event)) client.res.write(`id: ${event.id}\nevent: ${event.type}\ndata: ${JSON.stringify(event.data)}\n\n`)
}
export function emitGlobalEvent(type: string, data: unknown) {
  const event = { id: ++sequence, type, data }
  replay.push(event)
  if (replay.length > 500) replay.shift()
  clients.forEach((client) => send(client, event))
}
export function openGlobalEvents(req: IncomingMessage, res: ServerResponse, user: User, url: URL) {
  const id = `mock-${++subscription}`
  const client = { id, res, user, watch: (url.searchParams.get('watch') ?? '').split(',').filter(Boolean) }
  res.writeHead(200, { 'Content-Type': 'text/event-stream', 'Cache-Control': 'no-cache', Connection: 'keep-alive' })
  clients.set(id, client)
  res.write(`event: hello\ndata: ${JSON.stringify({ subscriber: id })}\n\n`)
  const last = Number(req.headers['last-event-id'] ?? url.searchParams.get('lastEventId') ?? 0)
  if (last && last < (replay[0]?.id ?? sequence) - 1) res.write('event: reset\ndata: {}\n\n')
  else if (last) replay.filter((event) => event.id > last).forEach((event) => send(client, event))
  // Bù trạng thái thực thể cho các truyện đang xem (không id, không vào ring): hợp nhất
  // theo id ở client nên mở lại trang/nối lại không xoá dữ liệu đã hiển thị.
  client.watch.forEach((key) => sendLiveBacklog(res, key))
  const timer = setInterval(() => res.write('event: ping\ndata: {}\n\n'), 20000)
  res.on('close', () => { clearInterval(timer); clients.delete(id) })
}
/** Số kết nối SSE toàn cục đang mở — dùng để đo “mỗi tab một luồng”. */
export const sseConnectionCount = () => clients.size

/** Gửi bù trạng thái thực thể của một truyện cho một kết nối (không id, không ring). */
function sendLiveBacklog(res: ServerResponse, key: string) {
  const state = live.get(key)
  if (!state) return
  const [project, story] = key.split('/')
  const frame = (kind: string, data: unknown) => {
    try { res.write(`event: live\ndata: ${JSON.stringify({ project, story, kind, data })}\n\n`) } catch { /* đóng */ }
  }
  if (state.events.length) frame('events', state.events)
  frame('clear', {})
  if (state.stream) frame('delta', { text: state.stream })
}

/** Nạp sẵn lịch sử sự kiện của truyện để tab mới mở có ngay danh sách “Vừa xong”. */
export function seedLiveEvents(project: string, story: string, events: unknown[]) {
  const key = keyOf(project, story)
  const state = live.get(key) ?? { events: [], stream: '' }
  const merged = [...state.events]
  for (const event of events) {
    const id = (event as { id?: string } | null)?.id
    if (!id) continue
    const at = merged.findIndex((item) => (item as { id?: string }).id === id)
    if (at >= 0) merged[at] = event
    else merged.push(event)
  }
  live.set(key, { ...state, events: merged.slice(-200) })
}

export function updateGlobalWatch(subscriber: string, keys: string[]) {
  const client = clients.get(subscriber)
  if (!client) return false
  const added = keys.filter((key) => !client.watch.includes(key))
  client.watch = keys
  added.forEach((key) => sendLiveBacklog(client.res, key))
  return true
}

// ─── Khung thực thể (worker) ghép vào luồng chung ───────────────────────────
// Cùng hợp đồng với máy chủ thật: event `live`, data `{project, story, kind, data}`.
// Không cấp id, không vào replay ring; mở kết nối mới thì gửi bù events + clear +
// delta như worker SSE cũ, nên dữ liệu không trắng màn khi nối lại.
type LiveState = { events: unknown[]; stream: string }
const live = new Map<string, LiveState>()
const keyOf = (project: string, story: string) => `${project}/${story}`

export function emitLiveFrame(project: string, story: string, kind: string, data: unknown) {
  const key = keyOf(project, story)
  const state = live.get(key) ?? { events: [], stream: '' }
  if (kind === 'event') {
    const event = data as { id?: string } | null
    if (event?.id) state.events = [...state.events.filter((item) => (item as { id?: string }).id !== event.id), data].slice(-200)
  } else if (kind === 'delta') {
    const text = (data as { text?: string } | null)?.text ?? ''
    state.stream = (state.stream + text).slice(-65536)
  } else if (kind === 'clear' || kind === 'reset') {
    state.stream = ''
    if (kind === 'reset') state.events = []
  }
  live.set(key, state)
  const frame = { project, story, kind, data }
  clients.forEach((client) => {
    if (!allowed(client, { id: 0, type: 'live', data: frame })) return
    try { client.res.write(`event: live\ndata: ${JSON.stringify(frame)}\n\n`) } catch { /* đóng */ }
  })
}

const versions = new Map<string, number>()
export const mockVersion = (key: string) => `mock-${versions.get(key) ?? 1}`
export function bumpMockVersion(key: string) {
  versions.set(key, (versions.get(key) ?? 1) + 1)
  return mockVersion(key)
}
export function mockConflict(req: IncomingMessage, res: ServerResponse, key: string, current: unknown, body?: Record<string, unknown>) {
  const expected = req.headers['if-match'] ?? body?.version
  if (expected === undefined || expected === mockVersion(key)) return false
  res.writeHead(409, { 'Content-Type': 'application/json; charset=utf-8' })
  res.end(JSON.stringify({ error: 'Đã có thay đổi từ nơi khác', conflict: true, version: mockVersion(key), current }))
  return true
}
