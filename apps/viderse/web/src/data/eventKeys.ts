// Bảng sự kiện SSE của Viderse → prefix khoá cần invalidate.
// Sự kiện của app là { rev, ref? }: `status` khi revision đầu trang đổi;
// `reset` khi số resume rơi khỏi vòng 500 sự kiện → làm mới mọi khoá.
import type { Key } from '@noverse/ui/data'
import { keys } from './keys'

export const SERVER_EVENT_TYPES = ['hello', 'reset', 'status', 'ai', 'platforms', 'tools', 'activity', 'storage', 'rivals'] as const
export type ServerEventType = (typeof SERVER_EVENT_TYPES)[number]

export type ServerEvent = { type: ServerEventType; data: unknown; id: string }

/** `'*'` = invalidate mọi khoá (reset — mất dấu sự kiện). */
export type EventKeys = readonly Key[] | '*'

export function keysForEvent(event: { type: string; data: unknown }): EventKeys {
  if (event.type === 'reset') return '*'
  if (event.type === 'status') return [keys.status()]
  if (event.type === 'ai') return [['ai']]
  if (event.type === 'platforms') return [['platforms']]
  if (event.type === 'tools') return [keys.tools(), keys.setup()]
  if (event.type === 'activity') return [keys.activity(), keys.setup(), keys.storage()]
  if (event.type === 'storage') return [keys.storage(), keys.activity()]
  // `rivals` {rev, ref=id kênh|'': đổi mức danh sách} → mọi khoá dưới ['rivals']
  // (danh sách đối thủ, video, quét lịch sử, lịch đăng của từng kênh).
  if (event.type === 'rivals') return [['rivals']]
  return []
}
