import { describe, expect, it } from 'vitest'
import { kickedReason, readAppLinks } from './sso'

describe('lý do phiên chung kết thúc', () => {
  it.each([
    [{ reason: 'logout', app: 'noverse' }, 'self'],
    [{ reason: 'logout', app: 'viderse' }, 'other'],
    [{ reason: 'logout' }, 'unknown'],
    [{ reason: 'password_changed' }, 'password'],
    [{ reason: 'admin_changed' }, 'admin'],
    [{ reason: 'locked' }, 'admin'],
    [{ reason: 'expired' }, 'expired'],
    [{ reason: 'unknown' }, 'unknown'],
    [{ reason: 'anything', app: 'viderse' }, 'unknown'],
    [null, 'unknown'],
  ])('%j ánh xạ sang %s', (payload, expected) => {
    expect(kickedReason(payload)).toBe(expected)
  })
})
it('chỉ nhận app đã biết với địa chỉ gốc http an toàn', () => {
  expect(readAppLinks([
    { id: 'viderse', name: 'Viderse', origin: 'https://video.example.test' },
    { id: 'viderse', name: 'Viderse', origin: 'javascript:alert(1)' },
    { id: 'other', name: 'Other', origin: 'https://example.com' },
    { id: 'noverse', name: 'Noverse', origin: 'https://example.com/path' },
  ])).toEqual([{ id: 'viderse', name: 'Viderse', origin: 'https://video.example.test' }])
})
