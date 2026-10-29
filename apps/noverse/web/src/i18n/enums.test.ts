// Nhãn enum engine → UI: mọi giá trị thật của snapshot (06-data-contract.md › Snapshot → UI)
// và verdict (05-screens.md › Thẩm duyệt) phải ra tiếng Việt, mã lạ giữ nguyên.
import { describe, expect, it } from 'vitest'
import { enumLabel } from './enums'

describe('enumLabel', () => {
  it('statusLabel engine phát chữ hoa (snapshot thật: COMPLETE)', () => {
    expect(enumLabel('status', 'COMPLETE')).toBe('Hoàn thành')
    expect(enumLabel('status', 'READY')).toBe('Sẵn sàng')
    expect(enumLabel('status', 'RUNNING')).toBe('Đang chạy')
    expect(enumLabel('status', 'REVIEW')).toBe('Thẩm duyệt')
    expect(enumLabel('status', 'REWRITE')).toBe('Viết lại')
  })

  it('runtimeState chữ thường', () => {
    expect(enumLabel('status', 'idle')).toBe('Đang rảnh')
    expect(enumLabel('status', 'running')).toBe('Đang chạy')
    expect(enumLabel('status', 'pausing')).toBe('Đang tạm dừng')
    expect(enumLabel('status', 'paused')).toBe('Đã tạm dừng')
    expect(enumLabel('status', 'completed')).toBe('Hoàn thành')
  })

  it('phase và flow theo bảng 06', () => {
    expect(enumLabel('phase', 'complete')).toBe('Hoàn thành')
    expect(enumLabel('phase', 'writing')).toBe('Sáng tác')
    expect(enumLabel('phase', 'premise')).toBe('Tiền đề')
    expect(enumLabel('flow', 'writing')).toBe('Sáng tác')
    expect(enumLabel('flow', 'rewriting')).toBe('Viết lại')
    expect(enumLabel('flow', 'polishing')).toBe('Gọt giũa')
  })

  it('advanceMode, verdict, severity, agentState', () => {
    expect(enumLabel('advanceMode', 'auto')).toBe('Tự động')
    expect(enumLabel('advanceMode', 'review')).toBe('Nghiệm thu từng chương')
    expect(enumLabel('verdict', 'accept')).toBe('Đạt')
    expect(enumLabel('verdict', 'polish')).toBe('Cần sửa')
    expect(enumLabel('verdict', 'rewrite')).toBe('Viết lại')
    expect(enumLabel('verdict', 'reject')).toBe('Không đạt')
    expect(enumLabel('severity', 'critical')).toBe('Nghiêm trọng')
    expect(enumLabel('severity', 'error')).toBe('Lỗi')
    expect(enumLabel('severity', 'warning')).toBe('Cảnh báo')
    expect(enumLabel('severity', 'info')).toBe('Thông tin')
    expect(enumLabel('agentState', 'running')).toBe('Đang chạy')
    expect(enumLabel('agentState', 'standby')).toBe('Gọi khi cần')
    expect(enumLabel('agentState', 'working')).toBe('Đang chạy')
  })

  it('rỗng → chuỗi rỗng; mã lạ giữ nguyên văn (không ẩn)', () => {
    expect(enumLabel('status', null)).toBe('')
    expect(enumLabel('phase', undefined)).toBe('')
    expect(enumLabel('flow', '   ')).toBe('')
    expect(enumLabel('verdict', 'unknown-code')).toBe('unknown-code')
  })
})
