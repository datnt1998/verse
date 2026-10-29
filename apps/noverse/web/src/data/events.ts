// Một luồng sự kiện toàn cục GET /api/events (16-feedback-refresh.md §2 · 11-api-contract.md).
// Lõi kết nối (EventSource/fetch stream, backoff, Last-Event-ID) nằm ở @noverse/ui/data;
// file này giữ phần Noverse: danh sách event, event→khoá, POST watch và probe phiên.
import { cache, createEventsClient, fetchSource, keyHash, matchesPrefix, type EventHandler, type EventsClient, type Key } from '@noverse/ui/data'
import { SERVER_EVENT_TYPES, type ServerEvent, type ServerEventType } from './eventKeys'
import { keysForEvent, watchListOf } from './eventKeys'
import { keys } from './keys'
import { ApiError, me } from '../api/client'
import type { Job, JobsResponse, Snapshot } from '../api/types'

export type { EventHandler, ServerEvent, ServerEventType }

export const events = createEventsClient({
  eventTypes: SERVER_EVENT_TYPES,
  createSource: (url) => fetchSource(url, { onUnauthorized: () => me().catch(() => {}) }),
  probe: async () => {
    try { await me(); return 'ok' }
    catch (error) { return error instanceof ApiError && error.status === 401 ? 'unauthorized' : 'ok' }
  },
  postWatch: async (subscriber, watch) => {
    const response = await fetch('/api/events/watch', { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ subscriber, keys: watch }) })
    return response.status !== 404
  },
})
export const subscribeEvents: EventsClient['subscribe'] = (type, handler) => events.subscribe(type, handler)

/** Une seule installation globale; le provider ne vit que pendant la session. */
export function startDataEvents(username?: string) {
  const pending = new Map<string, Key>()
  let timer: ReturnType<typeof setTimeout> | undefined
  const flush = () => {
    timer = undefined
    const prefixes = [...pending.values()]
    pending.clear()
    prefixes.filter((key, i) => !prefixes.some((other, j) => i !== j && other.length < key.length && matchesPrefix(key, other)))
      .forEach((key) => { void cache.invalidate(key) })
  }
  const reset = () => { clearTimeout(timer); timer = undefined; pending.clear(); void cache.invalidate() }
  const unmap = events.subscribe('*', (data, meta) => {
    const payload = data && typeof data === 'object' ? data as Record<string, unknown> : null
    // Snapshot frames already contain the resource, even if the workspace is hidden.
    if (meta.type === 'live' && payload?.kind === 'snapshot' && typeof payload.project === 'string' && typeof payload.story === 'string' && payload.data && typeof payload.data === 'object') {
      const key = keys.story(payload.project, payload.story, 'snapshot')
      pending.delete(keyHash(key))
      cache.setData(key, payload.data as Snapshot)
    }
    let prefixes = keysForEvent({ type: meta.type, data }, username)
    if (prefixes === '*') { reset(); return }
    if (meta.type.startsWith('job.') && typeof payload?.id === 'string') {
      const cached = cache.get<JobsResponse>(keys.jobs()).data
      if (cached && (meta.type === 'job.removed' || typeof payload.kind === 'string')) {
        const jobs = cached.jobs.filter(job => job.id !== payload.id)
        if (meta.type !== 'job.removed') jobs.push(payload as Job)
        cache.setData(keys.jobs(), { ...cached, jobs })
        prefixes = prefixes.filter(key => keyHash(key) !== keyHash(keys.jobs()))
      }
    }
    prefixes.forEach(key => pending.set(keyHash(key), key))
    if (pending.size && timer === undefined) timer = setTimeout(flush, 250)
  })
  const reconnect = events.onReconnect(reset)
  const watch = () => events.setWatch(watchListOf(cache.activeKeys()))
  const unwatch = cache.onActiveChange(watch)
  watch()
  events.start()
  return () => { clearTimeout(timer); pending.clear(); unmap(); reconnect(); unwatch(); events.stop(); cache.clear() }
}
