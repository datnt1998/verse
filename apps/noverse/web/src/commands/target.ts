// Ngữ cảnh truyện cho trang Hướng dẫn lệnh: nhớ workspace đang mở để nút
// “Điền vào ô chat Workspace” (13-commands.md §2) quay lại đúng truyện.
// Chỉ lưu trong tab hiện tại; chỉ hỏi máy chủ danh sách truyện của dự án để biết truyện còn tồn tại.
import { useState } from 'react'
import { ApiError, getSnapshot, listStories } from '../api/client'
import type { Snapshot, StorySummary } from '../api/types'
import { cache, keys, useResource } from '../data'

const TARGET_KEY = 'ainovel.commands.target'
/** Một truyện đã được máy chủ xác nhận có thật được tin trong chừng này ms rồi mới hỏi lại. */
const VERDICT_TTL = 60_000

export type CommandTarget = { project: string; story: string }

const STORY_HASH = /^\/p\/([^/]+)\/s\/([^/]+)\/(?:work|activity|read|audio)/

const listeners = new Set<() => void>()
/** Nghe khi một mục tiêu đã nhớ bị bỏ vì truyện không còn trong thư viện. */
export function subscribeHelpTarget(listener: () => void): () => void {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

/** Danh sách truyện của dự án đã có trong bộ nhớ đệm chung → truyện có thật hay không; chưa biết thì `undefined`. */
function cachedVerdict(target: CommandTarget): boolean | undefined {
  const cached = cache.get<{ stories: StorySummary[] }>(keys.project(target.project, 'stories')).data
  return cached ? cached.stories.some((item) => item.story === target.story) : undefined
}

const verdicts = new Map<string, { ok: boolean; at: number }>()
const pending = new Map<string, Promise<boolean>>()

/**
 * Truyện có trong thư viện không? Hỏi danh sách truyện của dự án (một lần mỗi phút mỗi truyện);
 * dự án hoặc truyện không tồn tại → false. Lỗi mạng/máy chủ khác không chứng minh gì → true.
 */
export function verifyTarget(target: CommandTarget): Promise<boolean> {
  const known = cachedVerdict(target)
  if (known !== undefined) return Promise.resolve(known)
  const id = `${target.project}/${target.story}`
  const verdict = verdicts.get(id)
  if (verdict && Date.now() - verdict.at < VERDICT_TTL) return Promise.resolve(verdict.ok)
  let inflight = pending.get(id)
  if (!inflight) {
    inflight = listStories(target.project)
      .then((result) => result.stories.some((item) => item.story === target.story))
      .catch((error: unknown) => !(error instanceof ApiError && error.status === 404))
      .then((ok) => { verdicts.set(id, { ok, at: Date.now() }); return ok })
      .finally(() => { pending.delete(id) })
    pending.set(id, inflight)
  }
  return inflight
}

function readTarget(): CommandTarget | null {
  try {
    const raw = sessionStorage.getItem(TARGET_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return null
    const { project, story } = parsed as Partial<CommandTarget>
    return typeof project === 'string' && typeof story === 'string' && project && story ? { project, story } : null
  } catch { return null }
}

const notify = () => queueMicrotask(() => listeners.forEach((listener) => listener()))

function dropTarget(target: CommandTarget): void {
  const stored = readTarget()
  if (!stored || stored.project !== target.project || stored.story !== target.story) return
  try { sessionStorage.removeItem(TARGET_KEY) } catch { /* chế độ riêng tư */ }
  notify()
}

function verifyStored(target: CommandTarget): void {
  void verifyTarget(target).then((ok) => { if (!ok) dropTarget(target) })
}

/**
 * Gọi trước khi điều hướng sang Hướng dẫn: hash hiện tại là màn truyện thì ghi nhớ.
 * Truyện không có trong thư viện (đường dẫn cũ, truyện đã xoá) không được nhớ: nếu bộ nhớ đệm đã biết
 * thì từ chối ngay, chưa biết thì nhớ tạm rồi bỏ khi máy chủ báo không có.
 */
export function rememberHelpTarget(hash: string = location.hash): void {
  const match = STORY_HASH.exec(hash.replace(/^#/, ''))
  if (!match) return
  const target = { project: decodeURIComponent(match[1]), story: decodeURIComponent(match[2]) }
  if (cachedVerdict(target) === false) { notify(); return }
  try { sessionStorage.setItem(TARGET_KEY, JSON.stringify(target)) } catch { /* chế độ riêng tư */ }
  verifyStored(target)
}

export function helpTarget(): CommandTarget | null {
  const target = readTarget()
  if (!target) return null
  if (cachedVerdict(target) === false) { dropTarget(target); return null }
  verifyStored(target)
  return target
}

/** Snapshot của truyện đã nhớ (bộ nhớ đệm chung) — cho busy + tiến độ. */
export function useCommandSnapshot(): {
  target: CommandTarget | null; snapshot: Snapshot | null
  loading: boolean; refreshing: boolean; updatedAt: number | null
} {
  const [target] = useState<CommandTarget | null>(() => helpTarget())
  const resource = useResource(
    keys.story(target?.project ?? '', target?.story ?? '', 'snapshot'),
    ({ signal }) => getSnapshot(target!.project, target!.story, signal),
    { enabled: !!target },
  )
  return {
    target,
    snapshot: resource.data ?? null,
    loading: resource.status === 'loading',
    refreshing: resource.refreshing,
    updatedAt: resource.updatedAt,
  }
}
