// Hook nhỏ cho chuyển động luồng viết: đồng hồ round đang chạy (mỗi giây) và
// đếm ngược retry. Tôn trọng prefers-reduced-motion ở tầng CSS.
import { useEffect, useState } from 'react'

/** Giây kể từ lần `resetKey` đổi (round/tool mới). */
export function useElapsed(active: boolean, resetKey: string): number {
  const [seconds, setSeconds] = useState(0)
  useEffect(() => {
    setSeconds(0)
    if (!active) return
    const started = performance.now()
    const id = setInterval(() => setSeconds((performance.now() - started) / 1000), 1000)
    return () => clearInterval(id)
  }, [active, resetKey])
  return seconds
}

/** Đồng hồ chung cập nhật mỗi `interval` ms — dùng cho đếm ngược `retryAt`. */
export function useNow(interval = 1000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), interval)
    return () => clearInterval(id)
  }, [interval])
  return now
}

