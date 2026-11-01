import type { CommandSpec, Snapshot, User } from '../api/types'
import { canWriteStory } from '../api/client'
import { isGate } from '../screens/workspace/GateCard'

export const MOBILE_QUERY = '(max-width: 640px)'
export const TABLET_QUERY = '(min-width: 641px) and (max-width: 1099px)'
export type LayoutMode = 'phone' | 'tablet' | 'desktop'
export type TabMode = 'default' | 'mini' | 'hide'
export const MOBILE_BACKOFF_MS = [1000, 2000, 4000, 5000] as const
export const mobileBackoff = (failure: number) => MOBILE_BACKOFF_MS[Math.min(Math.max(0, failure), MOBILE_BACKOFF_MS.length - 1)]
export function tabModeFor({ keyboard = 0, hidden = false, immersive = false, down = false }: { keyboard?: number; hidden?: boolean; immersive?: boolean; down?: boolean }): TabMode {
  return keyboard > 0 || hidden || immersive ? 'hide' : down ? 'mini' : 'default'
}
/** Only an actionable chapter review gate earns the orange Read-tab dot. */
export function showReviewDot(snapshot: Snapshot | null, user: User | null, storyId: string): boolean {
  return !!user && canWriteStory(user, storyId) && isGate(snapshot)
}
export function shouldCloseSheet(distance: number, height: number): boolean {
  return height > 0 && distance > height * 0.3
}
const DESKTOP_COMMANDS: Record<string, true> = { config: true, start: true, simulate: true, importsim: true }
export function mobileCommands(specs: CommandSpec[]): CommandSpec[] {
  return specs.filter((spec) => !DESKTOP_COMMANDS[spec.name])
}

export type ToastBottomEntry = { px: number; priority: number }
/** Khoảng cách đáy của toast: ưu tiên cao thắng, cùng ưu tiên thì mục đăng ký sau thắng; không có mục nào → null (CSS mặc định). */
export function pickToastBottom(entries: ToastBottomEntry[]): number | null {
  let best: ToastBottomEntry | null = null
  for (const entry of entries) if (!best || entry.priority >= best.priority) best = entry
  return best ? best.px : null
}
/** `bottom` của toast theo từng artboard điện thoại (MWork 214 · MReader 100 · MAudio 168 · MMore 100 · MCmd 112 · MLog 30 · MNotify 30). */
export function toastBottomFor(route: string): number | null {
  switch (route) {
    case 'workspace': case 'story': return 214
    case 'reader': return 100
    case 'audio': return 168
    case 'command': return 112
    case 'activity': case 'notifications': return 30
    case 'project': case 'help': case 'settings': return 100
    // M11: nút dính đáy (50) + lý do khoá + safe area — toast nằm trên cả hai.
    case 'newstory': return 120
    default: return null
  }
}
