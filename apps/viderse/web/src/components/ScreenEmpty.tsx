// ScreenEmpty — trạng thái trống chung của các màn chưa có dữ liệu (design:
// EmptyState kiểu frame, căn giữa vùng nội dung). Tách khỏi route Duyệt để
// màn khác không phụ thuộc module màn hình.
// Kiểu: stylex.create bên dưới (trước ở styles/components.css .vv-page-empty).
import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { EmptyState } from '@noverse/ui'

const sx = stylex.create({
  page: { display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: '50vh', padding: 'var(--vv-s-26)' },
})

export function ScreenEmpty({ icon = 'magic-star', title, hint, action }: { icon?: string; title: string; hint: string; action?: ReactNode }) {
  return (
    <div {...stylex.props(sx.page)}>
      <EmptyState variant="frame" icon={icon} title={title} hint={hint} action={action} />
    </div>
  )
}
