import type { AppNotification } from '../api/activity'
import { formatBytes, formatDuration } from '../lib/units'

export function notificationDetail(n: AppNotification) {
  const detail = n.detail
  if (!detail) return ''
  const parts: string[] = []
  if (detail.channels?.length) parts.push(n.count > 1 ? detail.channels.map((channel) => `${channel.name} ${channel.count}`).join(' · ') : detail.channels.map((channel) => channel.name).join(' · '))
  if (detail.cause) parts.push(detail.cause)
  if (detail.mp4?.durationSec != null) parts.push(`MP4 ${formatDuration(detail.mp4.durationSec)}`)
  if (detail.mp4?.bytes != null) parts.push(formatBytes(detail.mp4.bytes))
  return parts.join(' · ')
}

// Copy Toast r35: tiêu đề nói VIỆC một dòng, tên video nằm ở câu phụ.
export function notificationTitle(n: AppNotification) {
  if (n.kind === 'review_ready') return n.count > 1 ? `${n.count} video chờ duyệt ảnh` : 'Video chờ duyệt ảnh'
  if (n.kind === 'needs_attention') {
    const recovery = n.detail?.recovery?.label?.trim().toLowerCase()
    return recovery ? `Cần ${recovery}` : 'Cần xử lý'
  }
  if (n.kind === 'ready_to_post') return 'Sẵn sàng đăng'
  // task_done: text của máy chủ đã là câu một dòng ("Đã tạo nhận diện cho …").
  return n.text.replace(/\.\s*$/, '')
}

export function notificationSub(n: AppNotification) {
  const detail = n.detail
  if (n.kind === 'needs_attention') {
    const label = n.objectRef?.label || n.text.replace(/^Cần xử lý:\s*/, '').replace(/\.\s*$/, '')
    return [label, detail?.cause].filter(Boolean).join(' · ')
  }
  if (n.kind === 'ready_to_post') {
    const parts = [n.objectRef?.label]
    if (detail?.mp4?.durationSec != null) parts.push(`MP4 ${formatDuration(detail.mp4.durationSec)}`)
    if (detail?.mp4?.bytes != null) parts.push(formatBytes(detail.mp4.bytes))
    return parts.filter(Boolean).join(' · ')
  }
  return notificationDetail(n)
}
