// @vitest-environment happy-dom
// Chữ chạy ánh sáng không được khởi động lại khi nhãn công cụ đổi hoặc snapshot đẩy re-render:
// animation CSS chỉ chạy tiếp nếu phần tử DOM giữ nguyên (React không remount). Test này chốt
// node identity qua 2 lần render khác nội dung, và chốt không có thuộc tính nào reset animation
// (`key`, `animation`, `animation-name` nội tuyến).
import { act } from 'react'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { Shimmer } from './feedback'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
afterEach(() => { document.body.replaceChildren() })

describe('Shimmer: ánh sáng chạy liên tục', () => {
  it('đổi chữ không thay phần tử DOM (không khởi động lại animation)', async () => {
    const host = document.createElement('div')
    document.body.appendChild(host)
    const root = createRoot(host)

    await act(async () => { root.render(<Shimmer style={{ fontSize: 12 }}>Đang lập kế hoạch chương</Shimmer>) })
    const first = document.querySelector('.shimmer')
    expect(first?.textContent).toBe('Đang lập kế hoạch chương')

    await act(async () => { root.render(<Shimmer style={{ fontSize: 12 }}>Đang viết bản nháp</Shimmer>) })
    const second = document.querySelector('.shimmer')

    expect(second).toBe(first)
    expect(second?.textContent).toBe('Đang viết bản nháp')
    expect(second?.getAttribute('style')).toBe('font-size: 12px;')
    expect(second?.className).toBe('shimmer')

    await act(async () => { root.unmount() })
  })
})
