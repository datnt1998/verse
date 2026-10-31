import { expect, it } from 'vitest'
import type { AppNotification } from '../api/activity'
import { notificationDetail } from './activityText'

it('renders only stored channel, cause and media details', () => {
  const n: AppNotification = { id: 'n', kind: 'review_ready', count: 3, text: '3 video chờ duyệt', actions: [{ href: '#/review', label: 'Duyệt ảnh' }], createdAt: '2026-10-28T00:00:00Z', readAt: null, detail: { channels: [{ channelId: 'a', name: 'Midnight Tales', count: 2 }, { channelId: 'b', name: 'Chuyện Lạ', count: 1 }], cause: 'Cần đăng nhập lại', mp4: { durationSec: 618, bytes: 172 * 1024 * 1024 } } }
  expect(notificationDetail(n)).toContain('Midnight Tales 2 · Chuyện Lạ 1')
  expect(notificationDetail(n)).toContain('Cần đăng nhập lại')
  expect(notificationDetail(n)).toContain('10:18')
  expect(notificationDetail(n)).toContain('172 MB')
  expect(notificationDetail({ ...n, detail: undefined })).toBe('')
})

it('omits the per-channel count when a single notification names one channel', () => {
  const n: AppNotification = { id: 'n', kind: 'ready_to_post', count: 1, text: 'Video sẵn sàng đăng', actions: [], createdAt: '2026-10-28T00:00:00Z', readAt: null, detail: { channels: [{ channelId: 'a', name: 'Midnight Tales', count: 1 }], mp4: { durationSec: 2292 } } }
  expect(notificationDetail(n)).toBe('Midnight Tales · MP4 38:12')
})
