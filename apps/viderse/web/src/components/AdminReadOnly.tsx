import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { useSession } from '../api/session'

const s = stylex.create({
  note: {
    margin: 0,
    marginBottom: 'var(--vv-s-12)',
    padding: 'var(--vv-s-10) var(--vv-s-14)',
    fontSize: 13,
    lineHeight: 1.5,
    color: 'var(--color-fg-2)',
    backgroundColor: 'var(--color-surface-2)',
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: 'var(--line-2)',
    borderRadius: 'var(--vv-r-12)',
  },
  body: { opacity: 0.6 },
})

export const adminReadOnlyNote = 'Chỉ quản trị viên được thay đổi mục này. Bạn đang xem ở chế độ chỉ đọc.'

/**
 * Cấu hình toàn máy (kết nối AI, tài khoản nền tảng, công cụ…) chỉ quản trị
 * viên được ghi — máy chủ trả 403 cho thành viên. Thành viên thấy nội dung ở
 * chế độ chỉ đọc: nút và ô nhập bị khoá (inert), kèm một dòng giải thích,
 * thay vì nút bấm rồi báo lỗi.
 */
export function AdminReadOnly({ children, note = adminReadOnlyNote }: { children: ReactNode; note?: string }) {
  const { user } = useSession()
  if (!user || user.role === 'admin') return <>{children}</>
  return (
    <>
      <p role="note" {...stylex.props(s.note)}>{note}</p>
      <div inert aria-disabled="true" {...stylex.props(s.body)}>{children}</div>
    </>
  )
}
