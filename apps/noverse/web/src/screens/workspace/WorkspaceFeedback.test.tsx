import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { ApiError } from '../../api/client'
import { WorkspaceFeedback } from './WorkspaceFeedback'

const error = new ApiError(502, 'Worker không khởi động được: chưa có model.', undefined, { needsConfig: true })
const render = (admin: boolean, failure: unknown = error) => renderToStaticMarkup(
  <WorkspaceFeedback error={failure} admin={admin} onRetry={() => {}} />,
)

describe('lỗi khởi động worker', () => {
  it('hiện nguyên văn lỗi cùng đúng một nút Thử lại', () => {
    const html = render(false)
    expect(html).toContain(error.message)
    expect(html.match(/Thử lại/g)).toHaveLength(1)
    expect(render(false, null)).toBe('')
  })

  it('chỉ quản trị viên có liên kết Nhà cung cấp khi máy chủ yêu cầu cấu hình', () => {
    expect(render(true)).toContain('#/settings/providers')
    expect(render(false)).not.toContain('#/settings/providers')
    expect(render(true, new ApiError(502, 'Worker không khởi động được.', undefined, { needsConfig: false }))).not.toContain('#/settings/providers')
  })
})
