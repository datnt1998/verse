import { expect, it } from 'vitest'
import type { AppNotification } from '../api/activity'
import { notificationDetail, notificationSub, notificationTitle } from './activityText'

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

it('names background-job cards by the work, video title goes to the sub line', () => {
  const review: AppNotification = { id: 'r', kind: 'review_ready', count: 3, text: '3 video chờ duyệt', actions: [{ href: '#/review', label: 'Duyệt ảnh' }], createdAt: '2026-10-28T00:00:00Z', readAt: null, detail: { channels: [{ channelId: 'a', name: 'Midnight Tales', count: 2 }, { channelId: 'b', name: 'Chuyện Lạ Đêm Khuya', count: 1 }] } }
  expect(notificationTitle(review)).toBe('3 video chờ duyệt ảnh')
  expect(notificationSub(review)).toBe('Midnight Tales 2 · Chuyện Lạ Đêm Khuya 1')
  const attention: AppNotification = { id: 'a', kind: 'needs_attention', count: 1, text: 'Cần xử lý: The Last Bus', objectRef: { type: 'video', id: 'v', label: 'The Last Bus' }, actions: [{ href: '#/settings', label: 'Đăng nhập lại' }], createdAt: '2026-10-28T00:00:00Z', readAt: null, detail: { cause: 'Phiên đăng nhập ChatGPT đã hết hạn', recovery: { label: 'Đăng nhập lại', href: '#/settings?section=connect' } } }
  expect(notificationTitle(attention)).toBe('Cần đăng nhập lại')
  expect(notificationSub(attention)).toBe('The Last Bus · Phiên đăng nhập ChatGPT đã hết hạn')
  const ready: AppNotification = { id: 'p', kind: 'ready_to_post', count: 1, text: 'The Last Bus sẵn sàng đăng', objectRef: { type: 'video', id: 'v', label: 'The Last Bus' }, actions: [], createdAt: '2026-10-28T00:00:00Z', readAt: null, detail: { mp4: { durationSec: 618, bytes: 172 * 1024 * 1024 } } }
  expect(notificationTitle(ready)).toBe('Sẵn sàng đăng')
  expect(notificationSub(ready)).toBe('The Last Bus · MP4 10:18 · 172 MB')
})
