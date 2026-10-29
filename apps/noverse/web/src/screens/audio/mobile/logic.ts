import type { AudioChapter } from '../../../api/types'

export type SleepMode = 'off' | '15' | '30' | 'chapter'
export type SleepTimer = { mode: SleepMode; deadline: number | null }
export const SPEEDS = [1, 1.25, 1.5, 0.8] as const
export const SLEEP_LABELS: Record<SleepMode, string> = { off: 'Hẹn giờ tắt', '15': '15 phút', '30': '30 phút', chapter: 'Hết chương' }

/** Only complete, non-empty renders enter the listening queue. Order follows chapter numbers. */
export function playerQueue(chapters: readonly AudioChapter[]): number[] {
  return chapters.filter(({ audio }) => audio && !audio.rendering && audio.segments > 0 && audio.segmentsDone >= audio.segments)
    .map(({ chapter }) => chapter).sort((a, b) => a - b)
}

/** No wrap: a finished book stops, and an unfinished chapter is never silently played. */
export function stepChapter(queue: readonly number[], current: number | null, direction: 1 | -1): number | null {
  if (current === null) return direction === 1 ? queue[0] ?? null : queue[queue.length - 1] ?? null
  if (direction === 1) return queue.find((chapter) => chapter > current) ?? null
  for (let index = queue.length - 1; index >= 0; index--) if (queue[index] < current) return queue[index]
  return null
}

export function startSleep(mode: SleepMode, now: number): SleepTimer {
  return { mode, deadline: mode === '15' || mode === '30' ? now + Number(mode) * 60_000 : null }
}
export function sleepExpired(timer: SleepTimer, now: number, chapterEnded = false): boolean {
  return timer.mode === 'chapter' ? chapterEnded : timer.deadline !== null && now >= timer.deadline
}
export function nextSleep(timer: SleepTimer, now: number): SleepTimer {
  const modes: SleepMode[] = ['off', '15', '30', 'chapter']
  return startSleep(modes[(modes.indexOf(timer.mode) + 1) % modes.length], now)
}

export function chapterEnded(queue: readonly number[], current: number | null, timer: SleepTimer, now: number): { next: number | null; sleep: boolean } {
  const sleep = sleepExpired(timer, now, true)
  return { next: sleep ? null : stepChapter(queue, current, 1), sleep }
}

/** M3 đồng hồ: `5:12`, `13:42` — phút không đệm số 0 (khớp MAudio.dc.html `fmt`). */
export function formatMinutes(seconds: number): string {
  const total = Number.isFinite(seconds) ? Math.max(0, Math.round(seconds)) : 0
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`
}

/** Dòng cảnh báo ở sheet xuất khi còn chương chưa có giọng (MAudio: "Còn 3 chương đang tạo giọng (khoảng 18 phút)…"). */
export function pendingNotice(missing: number, ready: number, jobs: readonly { state: string; etaSec: number }[]): string {
  const running = jobs.filter((job) => job.state === 'queued' || job.state === 'rendering')
  const tail = `Xuất ngay sẽ chỉ gồm ${ready} chương đã xong.`
  if (!running.length) return `Còn ${missing} chương chưa có giọng. ${tail}`
  const minutes = Math.max(1, Math.round(running.reduce((sum, job) => sum + Math.max(0, job.etaSec), 0) / 60))
  return `Còn ${missing} chương đang tạo giọng (khoảng ${minutes} phút). ${tail}`
}
