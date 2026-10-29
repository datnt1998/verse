import { useEffect, useState } from 'react'
import { addParagraphComment, ApiError, sendInput } from '../api/client'
import type { ParagraphCommentInput } from '../api/types'
import { cache, keys } from '../data'

export type QueuedComment = { id: string; account: string; project: string; story: string; chapter: number; input: ParagraphCommentInput; createdAt: string; kind: 'comment' }
export type QueuedInput = { id: string; account: string; project: string; story: string; text: string; createdAt: string; kind: 'input' }
type Entry = QueuedComment | QueuedInput
let username = ''
let database: Promise<IDBDatabase> | undefined
const listeners = new Set<() => void>()
let draining: Promise<void> | null = null
let sending: AbortController | null = null
function db() {
  return database ??= new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open('ainovel.mobile', 1)
    request.onupgradeneeded = () => { request.result.createObjectStore('outbox', { keyPath: 'id' }) }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => { database = undefined; reject(new Error('Không lưu được góp ý chờ gửi. Giữ nội dung và thử lại.')) }
  })
}
async function transaction<T>(mode: IDBTransactionMode, action: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const database = await db()
  return new Promise<T>((resolve, reject) => {
    const tx = database.transaction('outbox', mode)
    const request = action(tx.objectStore('outbox'))
    tx.oncomplete = () => resolve(request.result)
    tx.onerror = () => reject(tx.error ?? new Error('Không lưu được góp ý chờ gửi.'))
    tx.onabort = () => reject(tx.error ?? new Error('Không lưu được góp ý chờ gửi.'))
  })
}
const changed = () => listeners.forEach((listener) => listener())
export function setOutboxAccount(account: string) {
  if (account !== username) sending?.abort()
  username = account; changed()
}
export async function readOutbox(): Promise<Entry[]> {
  const entries = await transaction('readonly', (store) => store.getAll()) as Entry[]
  return entries.filter((entry) => entry.account === username).sort((a, b) => a.createdAt.localeCompare(b.createdAt))
}
export async function enqueueComment(input: Omit<QueuedComment, 'id' | 'account' | 'createdAt' | 'kind'>) {
  if (!username) throw new Error('Đăng nhập để giữ góp ý chờ gửi.')
  await transaction('readwrite', (store) => store.put({ ...input, id: crypto.randomUUID(), account: username, createdAt: new Date().toISOString(), kind: 'comment' } satisfies QueuedComment))
  changed()
}
export async function enqueueInput(input: Pick<QueuedInput, 'project' | 'story' | 'text'>) {
  if (!username) throw new Error('Đăng nhập để giữ góp ý chờ gửi.')
  await transaction('readwrite', (store) => store.put({ ...input, id: crypto.randomUUID(), account: username, createdAt: new Date().toISOString(), kind: 'input' } satisfies QueuedInput))
  changed()
}
export function drainOutbox(): Promise<void> {
  if (draining) return draining
  const active = username
  draining = (async () => {
    for (const entry of await readOutbox()) {
      if (!navigator.onLine || username !== active) break
      try {
        sending = new AbortController()
        if (entry.kind === 'comment') {
          await addParagraphComment(entry.project, entry.story, entry.chapter, entry.input, sending.signal)
          await cache.invalidate(keys.story(entry.project, entry.story, 'comments', entry.chapter))
        } else await sendInput(entry.project, entry.story, entry.text, sending.signal)
        await transaction('readwrite', (store) => store.delete(entry.id))
        changed()
      } catch { break } // Keep the original entry, including non-network errors, for explicit retry.
      finally { sending = null }
    }
  })().finally(() => {
    draining = null
    if (username && username !== active && navigator.onLine) void drainOutbox().catch(() => {})
  })
  return draining
}
export async function submitQueuedComment(input: Omit<QueuedComment, 'id' | 'account' | 'createdAt' | 'kind'>) {
  if (navigator.onLine) {
    try { return { queued: false, comment: await addParagraphComment(input.project, input.story, input.chapter, input.input) } }
    catch (error) { if (!(error instanceof ApiError) || error.status !== 0) throw error }
  }
  await enqueueComment(input)
  return { queued: true }
}
function useOutbox(project: string, story: string, chapter?: number) {
  const [items, setItems] = useState<Entry[]>([])
  useEffect(() => {
    let active = true
    const update = () => { void readOutbox().then((entries) => { if (active) setItems(entries.filter((entry) => entry.project === project && entry.story === story && (chapter === undefined ? entry.kind === 'input' : entry.kind === 'comment' && entry.chapter === chapter))) }).catch(() => {}) }
    listeners.add(update); update()
    return () => { active = false; listeners.delete(update) }
  }, [project, story, chapter])
  return items
}
export const useCommentOutbox = (project: string, story: string, chapter: number) => useOutbox(project, story, chapter) as QueuedComment[]
export const useInputOutbox = (project: string, story: string) => useOutbox(project, story) as QueuedInput[]
