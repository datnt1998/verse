// Kho nhỏ của hai màn lệnh: registry G11 lấy từ bộ nhớ đệm chung (16 §1),
// dòng đang chọn trong popover, và phím tắt của popover (Composer gọi handleSlashKey trong onKeyDown).
import { useSyncExternalStore } from 'react'
import { ApiError, getCommands } from '../api/client'
import type { CommandSpec, Snapshot } from '../api/types'
import { cache, keys, useResource } from '../data'
import {
  acceptSlash, buildSlashView, orderedSpecs, pushRecentCommand, pushRecentPaths, slashContext, tabSlash,
} from './model'

// ─── Registry lệnh ──────────────────────────────────────────────────────
export type LoadState = 'loading' | 'ready' | 'error'
const COMMANDS_KEY = [...keys.system(), 'commands'] as const

const fetchCommands = async ({ signal }: { signal: AbortSignal }): Promise<CommandSpec[]> =>
  orderedSpecs(((await getCommands(signal)) ?? []))

/** Bản đã tải (rỗng khi chưa xong) — dùng cho phím tắt, không cần render. */
export const cachedCommands = (): CommandSpec[] => cache.get<CommandSpec[]>(COMMANDS_KEY).data ?? []

/** Ép tải lại registry (nút Thử lại trên màn lỗi). */
export const loadCommands = (): Promise<unknown> => cache.refresh(COMMANDS_KEY)

export function useCommands(): {
  specs: CommandSpec[]; state: LoadState; error: string
  /** đang tải lại khi đã có dữ liệu — vạch 2 px mép trên */
  refreshing: boolean; updatedAt: number | null; retry: () => void
} {
  const resource = useResource<CommandSpec[]>(COMMANDS_KEY, fetchCommands, { staleTime: Infinity })
  return {
    specs: resource.data ?? [],
    state: resource.status,
    error: resource.error
      ? (resource.error instanceof ApiError ? resource.error.message : 'Không tải được danh sách lệnh.')
      : '',
    refreshing: resource.refreshing,
    updatedAt: resource.updatedAt,
    retry: () => { void resource.refresh() },
  }
}

// ─── Dòng đang chọn trong popover ───────────────────────────────────────
let selection = { line: '', index: 0 }
const selectionSubs = new Set<() => void>()
const emitSelection = () => selectionSubs.forEach((fn) => fn())

export const readSelection = (line: string): number => (selection.line === line ? selection.index : 0)

// Popover đã đóng bằng Esc cho đúng dòng này (17-kit: Esc đóng lớp trên cùng, bản nháp giữ nguyên).
// Đổi sang dòng khác là mở lại.
let dismissedLine: string | null = null
export const slashDismissed = (line: string): boolean => dismissedLine !== null && dismissedLine === line
export function dismissSlash(line: string): void {
  if (dismissedLine === line) return
  dismissedLine = line
  emitSelection()
}

export function writeSelection(line: string, index: number) {
  if (selection.line !== line) dismissedLine = null
  selection = { line, index }
  emitSelection()
}

export function useSelection(line: string): [number, (index: number) => void] {
  const current = useSyncExternalStore(
    (listener) => { selectionSubs.add(listener); return () => { selectionSubs.delete(listener) } },
    () => selection,
  )
  return [current.line === line ? current.index : 0, (index: number) => writeSelection(line, index)]
}

/**
 * Ghost text + lỗi cú pháp của dòng `/…` — Composer dùng để vẽ phần mờ trong ô chat
 * và chip “Chưa chạy được”; cập nhật khi registry tải xong hoặc đổi dòng/dòng chọn.
 */
export function useSlashHints(line: string, snapshot?: Snapshot | null, writable = true): { ghost: string; hasError: boolean } {
  const { specs } = useCommands()
  const [selected] = useSelection(line)
  if (!specs.length || !line.startsWith('/')) return { ghost: '', hasError: false }
  const view = buildSlashView(line, slashContext(snapshot ?? null, specs, undefined, { writable, dismissed: slashDismissed(line) }), selected)
  return { ghost: view?.ghost ?? '', hasError: view?.hasError ?? false }
}

// ─── Phím tắt (Composer gọi trước onKeyDown của nó) ─────────────────────
export type SlashKeyEvent = { key: string; preventDefault: () => void }
export type SlashKeyResult = { handled: boolean; next?: string; submit?: string }

/**
 * ↑ ↓ đổi dòng · Tab điền · Enter điền/chạy · Esc xoá `/…`.
 * Chỉ xử lý khi dòng bắt đầu bằng `/` và registry đã tải.
 */
export function handleSlashKey(
  event: SlashKeyEvent, line: string, snapshot?: Snapshot | null, known?: CommandSpec[], writable = true,
): SlashKeyResult {
  const list = known ?? cachedCommands()
  if (!list.length || !line.startsWith('/')) return { handled: false }
  const view = buildSlashView(line, slashContext(snapshot ?? null, list, undefined, { writable, dismissed: slashDismissed(line) }), readSelection(line))
  if (!view) {
    // Popover đã đóng bằng Esc: không còn phím nào của popover để xử lý, chữ vẫn giữ nguyên.
    return { handled: false }
  }
  const total = view.items.length
  const index = total ? Math.min(Math.max(readSelection(line), 0), total - 1) : 0

  if (event.key === 'ArrowDown') {
    event.preventDefault()
    writeSelection(line, total ? (index + 1) % total : 0)
    return { handled: true }
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    writeSelection(line, total ? (index - 1 + total) % total : 0)
    return { handled: true }
  }
  if (event.key === 'Tab') {
    event.preventDefault()
    return { handled: true, ...tabSlash(view, index) }
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    const result = acceptSlash(view, index)
    if (result.submit) { pushRecentCommand(view.spec.name); pushRecentPaths(result.submit, list) }
    return { handled: true, ...result }
  }
  if (event.key === 'Escape') {
    // Esc một lần: đóng popover, GIỮ chữ đang gõ (17-kit, chốt 25/10).
    event.preventDefault()
    writeSelection(line, 0)
    dismissSlash(line)
    return { handled: true }
  }
  return { handled: false }
}

/** Ghi nhớ dòng `/…` vừa gửi (lệnh dùng gần đây + đường dẫn gần đây). Composer gọi khi gửi bằng nút Gửi. */
export function rememberSlashLine(line: string): void {
  const list = cachedCommands()
  const head = line.startsWith('/') ? line.slice(1).split(/\s/)[0].toLowerCase() : ''
  const spec = list.find((entry) => entry.name === head || entry.aliases.includes(head))
  if (!spec) return
  pushRecentCommand(spec.name)
  pushRecentPaths(line, list)
}
