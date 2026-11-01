import type { CommandSpec, Snapshot, User } from '../api/types'
import { canWriteStory } from '../api/client'
import { isGate } from '../screens/workspace/GateCard'
import { PHONE_MAX_NOVERSE, phoneQuery } from '@noverse/ui'

export const MOBILE_QUERY = phoneQuery(PHONE_MAX_NOVERSE)
export const TABLET_QUERY = '(min-width: 641px) and (max-width: 1099px)'
export type LayoutMode = 'phone' | 'tablet' | 'desktop'
export { tabModeFor, pickToastBottom, type TabMode, type ToastBottomEntry } from '@noverse/ui'
export const MOBILE_BACKOFF_MS = [1000, 2000, 4000, 5000] as const
export const mobileBackoff = (failure: number) => MOBILE_BACKOFF_MS[Math.min(Math.max(0, failure), MOBILE_BACKOFF_MS.length - 1)]
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
