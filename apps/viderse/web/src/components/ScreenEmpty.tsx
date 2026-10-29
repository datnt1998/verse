// ScreenEmpty — trạng thái trống chung của các màn chưa có dữ liệu (design:
// EmptyState kiểu frame, căn giữa vùng nội dung). Tách khỏi route Duyệt để
// màn khác không phụ thuộc module màn hình.
// Kiểu: styles/components.css .vv-page-empty.
import type { ReactNode } from 'react'
import { EmptyState } from '@noverse/ui'

export function ScreenEmpty({ icon = 'magic-star', title, hint, action }: { icon?: string; title: string; hint: string; action?: ReactNode }) {
  return (
    <div className="vv-page-empty">
      <EmptyState variant="frame" icon={icon} title={title} hint={hint} action={action} />
    </div>
  )
}
