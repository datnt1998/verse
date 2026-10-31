import { describe, expect, it } from 'vitest'
import type { LibraryInspect } from '../../api/types'
import { displayLeaf, footerView, inspectNote, inspectView } from './libraryInspect'

const base: LibraryInspect = { kind: 'empty', projects: 0, stories: 0, items: 0, place: 'Tài liệu', display: 'Tài liệu › Truyện Noverse', name: 'Truyện Noverse' }

describe('inspectView (09 §10.1/§10.4)', () => {
  it('found gọi tên dự án khi chỉ có một', () => {
    const view = inspectView({ ...base, kind: 'found', projects: 1, stories: 1, projectName: 'Kênh truyện audio' })
    expect(view.text).toBe('Đã có 1 dự án Noverse (“Kênh truyện audio”) với 1 truyện — Noverse dùng luôn, không đổi gì.')
    expect(view.tone).toBe('ok')
  })
  it('found nhiều dự án không gọi tên', () => {
    expect(inspectView({ ...base, kind: 'found', projects: 2, stories: 5, projectName: 'A' }).text).toContain('Đã có 2 dự án Noverse với 5 truyện')
  })
  it('empty trong iCloud nhắc “Luôn giữ trên máy”, nơi khác thì không', () => {
    expect(inspectView({ ...base, place: 'iCloud Drive' }).text).toContain('Luôn giữ trên máy')
    expect(inspectView({ ...base, place: 'Google Drive' }).text).toContain('Luôn giữ trên máy')
    expect(inspectView(base).text).toBe('Thư mục trống, ghi được.')
  })
  it('other cảnh báo vàng, vẫn dùng được', () => {
    const view = inspectView({ ...base, kind: 'other', items: 4 })
    expect(view.tone).toBe('warn')
    expect(view.text).toBe('Có tệp khác không phải truyện — Noverse không đụng tới, nhưng nên chọn thư mục riêng')
  })
  it('readonly giải thích và là màu lỗi', () => {
    const view = inspectView({ ...base, kind: 'readonly' })
    expect(view.tone).toBe('err')
    expect(view.text).toContain('không ghi được')
  })
})

describe('footerView / inspectNote / displayLeaf', () => {
  it('chân bộ duyệt: readonly đỏ, found xanh', () => {
    expect(footerView({ ...base, kind: 'readonly' })).toMatchObject({ fg: '#FF9AA2', text: 'Không ghi được vào thư mục này' })
    expect(footerView({ ...base, kind: 'found' }).fg).toBe('#A6EDC1')
  })
  it('ghi chú thẻ', () => {
    expect(inspectNote({ ...base, kind: 'found', projects: 1, stories: 3 }).text).toBe('1 dự án · 3 truyện')
    expect(inspectNote({ ...base, kind: 'readonly' }).text).toBe('Không ghi được')
    expect(inspectNote(base).text).toBe('Thư mục trống')
  })
  it('displayLeaf lấy đoạn cuối, không bao giờ trả đường dẫn', () => {
    expect(displayLeaf('iCloud Drive › Truyện của Dat')).toBe('Truyện của Dat')
    expect(displayLeaf('Thư mục nhà')).toBe('Thư mục nhà')
  })
})

// 23-brand §5: thư viện cũ “Truyện ainovel” vẫn phải hiện là thư viện có sẵn — cách dò nằm ở
// máy chủ (internal/library/inspect.go → Open(dir), tức thư mục .ainovel bên trong), không dựa vào tên.
describe('thư viện cũ tên “Truyện ainovel”', () => {
  it('vẫn là found, chỉ câu chữ đổi sang Noverse', () => {
    const legacy: LibraryInspect = {
      kind: 'found', projects: 1, stories: 2, items: 3, place: 'Tài liệu',
      display: 'Tài liệu › Truyện ainovel', name: 'Truyện ainovel', projectName: 'Kênh truyện audio',
    }
    const view = inspectView(legacy)
    expect(view.tone).toBe('ok')
    expect(view.text).toBe('Đã có 1 dự án Noverse (“Kênh truyện audio”) với 2 truyện — Noverse dùng luôn, không đổi gì.')
    expect(footerView(legacy).text).toBe('Thư viện Noverse có sẵn — dùng luôn')
  })
})

it('warns about ephemeral container storage in both selection presentations', () => {
  const info = { ...base, storageWarning: 'Thư mục này sẽ mất khi cập nhật container — hãy gắn volume.' }
  expect(inspectView(info)).toMatchObject({ tone: 'warn', text: info.storageWarning })
  expect(footerView(info).text).toBe(info.storageWarning)
})
