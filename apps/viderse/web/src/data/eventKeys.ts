// Bảng sự kiện SSE của Viderse → prefix khoá cần invalidate.
// Sự kiện của app là { rev, ref? }: `status` khi revision đầu trang đổi;
// `reset` khi số resume rơi khỏi vòng 500 sự kiện → làm mới mọi khoá.
import type { Key } from '@noverse/ui/data'
import { keys } from './keys'

export const SERVER_EVENT_TYPES = ['hello', 'reset', 'status'] as const
export type ServerEventType = (typeof SERVER_EVENT_TYPES)[number]

export type ServerEvent = { type: ServerEventType; data: unknown; id: string }

/** `'*'` = invalidate mọi khoá (reset — mất dấu sự kiện). */
export type EventKeys = readonly Key[] | '*'

export function keysForEvent(event: { type: string; data: unknown }): EventKeys {
  if (event.type === 'reset') return '*'
  if (event.type === 'status') return [keys.status()]
  return []
}
