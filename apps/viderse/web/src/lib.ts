import type { Brief, Run, RunStatus } from './api'

// Validate phía form: topic bắt buộc, đúng một dạng độ dài dương — cùng luật
// server (script.Brief.Validate) để báo lỗi sớm, không chờ round-trip.
export function validateBrief(input: {
  topic: string
  audience: string
  lengthKind: 'seconds' | 'words'
  lengthValue: string
  wpm: string
}): { brief?: Brief; error?: string } {
  const topic = input.topic.trim()
  if (!topic) return { error: 'Thiếu chủ đề' }
  const length = Number(input.lengthValue)
  if (!Number.isFinite(length) || length <= 0) {
    return { error: 'Độ dài mục tiêu phải là số dương' }
  }
  const wpm = input.wpm.trim() === '' ? 0 : Number(input.wpm)
  if (!Number.isFinite(wpm) || wpm < 0) {
    return { error: 'Tốc độ đọc phải là số không âm' }
  }
  const brief: Brief = { topic }
  if (input.audience.trim()) brief.audience = input.audience.trim()
  if (input.lengthKind === 'seconds') brief.target_seconds = Math.round(length)
  else brief.target_words = Math.round(length)
  if (wpm > 0) brief.words_per_minute = wpm
  return { brief }
}

export function isActive(status: RunStatus): boolean {
  return status === 'queued' || status === 'running'
}

// hasActiveRun báo list còn run chưa xong → tiếp tục polling.
export function hasActiveRun(runs: Run[]): boolean {
  return runs.some((r) => isActive(r.status))
}

export function statusLabel(status: RunStatus): string {
  switch (status) {
    case 'queued':
      return 'Đang chờ'
    case 'running':
      return 'Đang chạy'
    case 'succeeded':
      return 'Thành công'
    case 'failed':
      return 'Thất bại'
  }
}

export function formatDuration(seconds?: number): string {
  if (seconds === undefined) return '—'
  const s = Math.round(seconds)
  const m = Math.floor(s / 60)
  return m > 0 ? `${m}m${s % 60}s` : `${s}s`
}

export function formatCost(cost?: number): string {
  if (cost === undefined) return '—'
  return `$${cost.toFixed(4)}`
}

export function formatTime(iso?: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  return d.toLocaleString('vi-VN', { hour12: false })
}

// briefSummary tóm tắt brief để hiển thị trong danh sách run.
export function briefSummary(brief?: Brief): string {
  if (!brief) return '—'
  const len = brief.target_seconds
    ? formatDuration(brief.target_seconds)
    : brief.target_words
      ? `${brief.target_words} từ`
      : ''
  return len ? `${brief.topic} · ${len}` : brief.topic
}
