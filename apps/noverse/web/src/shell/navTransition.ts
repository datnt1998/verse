// Trạng thái “đang chuyển trang” dùng chung cho thanh điều hướng và vạch tiến độ (Gói Q).
//
// Bấm một mục → chỉ báo trượt ngay (`beginNav`) dù trang mới chưa render xong. Quá 300 ms mà
// nội dung mới chưa có → vạch tiến độ 2 px dưới thanh trên (`slow`). Nội dung mới vẽ xong hoặc
// hết 6 s → `endNav`.
import { useSyncExternalStore } from 'react'
import type { NavKey } from './MainNav'

export type NavPhase = {
  pending: NavKey | null
  /** `location.hash` lúc bấm — chỉ kết thúc khi hash đã đổi (hoặc hết giờ). */
  from: string
  slow: boolean
  startedAt: number
}

const IDLE: NavPhase = { pending: null, from: '', slow: false, startedAt: 0 }
const SLOW_AFTER = 300
const GIVE_UP = 6000

let phase: NavPhase = IDLE
const listeners = new Set<() => void>()
let slowTimer: NodeJS.Timeout | undefined
let giveUpTimer: NodeJS.Timeout | undefined

const emit = () => { for (const listener of listeners) listener() }

function set(next: NavPhase) {
  if (next.pending === phase.pending && next.slow === phase.slow && next.from === phase.from) return
  phase = next
  emit()
}

/** Bấm/focus mục điều hướng: chỉ báo trượt tới mục đó ngay. */
export function beginNav(key: NavKey, from = location.hash) {
  clearTimers()
  set({ pending: key, from, slow: false, startedAt: Date.now() })
  slowTimer = setTimeout(() => set({ ...phase, slow: true }), SLOW_AFTER)
  giveUpTimer = setTimeout(endNav, GIVE_UP)
}

/** Trang mới đã vẽ nội dung (hoặc bỏ cuộc): trả thanh điều hướng về trạng thái thường. */
export function endNav() {
  clearTimers()
  set(IDLE)
}

function clearTimers() {
  clearTimeout(slowTimer)
  clearTimeout(giveUpTimer)
  slowTimer = undefined
  giveUpTimer = undefined
}

const subscribe = (listener: () => void) => {
  listeners.add(listener)
  return () => { listeners.delete(listener) }
}

/** Đọc trạng thái chuyển trang; chỉ vẽ lại khi `pending`/`slow` đổi. */
export function useNavPhase(): NavPhase {
  return useSyncExternalStore(subscribe, () => phase, () => IDLE)
}
