import { describe, expect, it } from 'vitest'
import type { InboxNotification } from '../../api/types'
import { groupNotifications, needsHomeScreenInstall, notificationPath } from './logic'

const item = (id: string, createdAt: string): InboxNotification => ({ id, createdAt, type: 'gate', story: 'p/s', title: id, body: '', url: '', actions: [], read: false })

describe('hộp thư thông báo', () => {
  it('nhóm theo ngày lịch địa phương, qua ranh giới tháng, sắp xếp mới trước và không làm đổi dữ liệu nguồn', () => {
    const input = [item('cũ', new Date(2026, 8, 29, 21).toISOString()), item('hôm qua', new Date(2026, 8, 30, 23, 59).toISOString()), item('sáng', new Date(2026, 9, 1, 0, 1).toISOString()), item('mới', new Date(2026, 9, 1, 10).toISOString())]
    const grouped = groupNotifications(input, new Date(2026, 9, 1, 12))
    expect(grouped.map((group) => group.label)).toEqual(['Hôm nay', 'Hôm qua', new Date(2026, 8, 29).toLocaleDateString('vi-VN', { day: 'numeric', month: 'long', year: 'numeric' })])
    expect(grouped.map((group) => group.items.map((value) => value.id))).toEqual([['mới', 'sáng'], ['hôm qua'], ['cũ']])
    expect(input.map((value) => value.id)).toEqual(['cũ', 'hôm qua', 'sáng', 'mới'])
    expect(groupNotifications([], new Date(2026, 9, 1))).toEqual([])
  })

  it('ánh xạ mọi loại tới đích sâu, giữ query và đường tải nội bộ, từ chối nguồn bên ngoài', () => {
    const notification = { ...item('n', ''), chapter: 25, job: 'p/s/import/1' }
    expect(notificationPath(notification)).toBe('/read/p%2Fs/25?gate=1')
    expect(notificationPath({ ...notification, type: 'error' })).toBe('/write/p%2Fs')
    expect(notificationPath({ ...notification, type: 'job' })).toBe('/listen/p%2Fs')
    expect(notificationPath({ ...notification, type: 'done', chapter: 20 })).toBe('/read/p%2Fs/20')
    expect(notificationPath({ ...notification, type: 'security' })).toBe('/more/devices')
    for (const type of ['import', 'sync', 'cocreate'] as const) expect(notificationPath({ ...notification, type })).toBe('/cmd/p%2Fs%2Fimport%2F1')
    expect(notificationPath({ ...notification, url: '/read/p%2Fs/25?gate=1&rewrite=1' })).toBe('/read/p%2Fs/25?gate=1&rewrite=1')
    expect(notificationPath({ ...notification, type: 'job', url: '/api/jobs/x/download' })).toBe('/api/jobs/x/download')
    expect(notificationPath({ ...notification, url: '//evil.example/path' })).toBe('/read/p%2Fs/25?gate=1')
    expect(notificationPath({ ...notification, url: 'https://evil.example/path' })).toBe('/read/p%2Fs/25?gate=1')
  })
})

describe('lưu ý Thêm vào Màn hình chính trên iPhone', () => {
  const base = { userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Chrome/130', platform: 'MacIntel', maxTouchPoints: 0, standalone: false }
  const iphone = { ...base, userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 Version/17.5 Mobile/15E148 Safari/604.1', platform: 'iPhone', maxTouchPoints: 5 }

  it('hiện cho iPhone/iPad chưa cài vào Màn hình chính, kể cả iPadOS tự nhận là Mac', () => {
    expect(needsHomeScreenInstall(iphone)).toBe(true)
    expect(needsHomeScreenInstall({ ...base, maxTouchPoints: 5 })).toBe(true)
  })

  it('không hiện khi đã chạy từ Màn hình chính, trên Mac thật hay Android', () => {
    expect(needsHomeScreenInstall({ ...iphone, standalone: true })).toBe(false)
    expect(needsHomeScreenInstall(base)).toBe(false)
    expect(needsHomeScreenInstall({ ...base, userAgent: 'Mozilla/5.0 (Linux; Android 14; Pixel 8) Chrome/130 Mobile', platform: 'Linux armv81', maxTouchPoints: 5 })).toBe(false)
  })
})
