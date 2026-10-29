import { Activity, type ReactNode } from 'react'

/**
 * Giữ một màn đã mở khi người dùng chuyển đi chỗ khác (điện thoại: tab; máy tính: 5 màn chính).
 *
 * `<Activity mode="hidden">` giữ state, DOM và vị trí cuộn của màn, nhưng gỡ effect — nên SSE,
 * poll và timer của màn ẩn tự dừng, và React vẫn dựng trước màn ẩn khi rảnh. Trước đây chỗ này
 * tự ẩn bằng `display:none` (không dừng effect) hoặc gỡ rồi dựng lại (mất state, nháy khung chờ).
 */
export function KeepAlive({ active, children }: { active: boolean; children: ReactNode }) {
  return <Activity mode={active ? 'visible' : 'hidden'}>{children}</Activity>
}
