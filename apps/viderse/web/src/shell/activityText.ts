import type { AppNotification } from '../api/activity'
import { formatBytes, formatDuration } from '../editor/ui/export/export-settings'

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
