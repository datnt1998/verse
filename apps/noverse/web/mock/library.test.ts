// Mock GAP-34: phân loại thư mục của bộ chọn nơi lưu truyện.
// 23-brand §5 — thư viện cũ “Truyện ainovel” vẫn phải nhận ra: cách dò dựa vào thư mục .ainovel
// bên trong (ở mock là `kind: 'library'`), không dựa vào tên thư mục.
import { describe, expect, it } from 'vitest'
import { mockDisplay, mockInspect, mockList } from './library'

describe('mockInspect — dò thư viện không theo tên', () => {
  it('thư mục thư viện không có chữ Noverse trong tên vẫn là found', () => {
    const found = mockInspect('/mock/home/Truyen')
    expect(found.kind).toBe('found')
    expect(found.name).toBe('Truyen')
    expect(found.projects).toBe(1)
    expect(found.display).toBe('Thư mục nhà › Truyen')
  })
  it('thư mục gợi ý còn trống chỉ là empty, chưa thành thư viện', () => {
    expect(mockInspect('/mock/home/Documents/Truyện Noverse').kind).toBe('empty')
  })
})

describe('mock library — thư mục gợi ý mới', () => {
  it('Tài liệu có “Truyện Noverse” (trống), không còn tên cũ', () => {
    const listing = mockList('/mock/home/Documents')
    expect(listing.status).toBe(200)
    const rows = listing.body as { name: string; kind: string; items: number }[]
    expect(rows.find((row) => row.name === 'Truyện Noverse')).toMatchObject({ kind: 'dir', items: 0 })
    expect(rows.some((row) => row.name === 'Truyện ainovel')).toBe(false)
  })
  it('mockDisplay giữ tên thư mục người dùng đặt', () => {
    expect(mockDisplay('/mock/home/Documents/Truyện Noverse')).toBe('Tài liệu › Truyện Noverse')
  })
})
