// KeepAlive + MobilePanes — giữ màn đã mở khi người dùng rời đi (điện thoại: tab;
// máy tính: các màn chính) thay vì gỡ rồi dựng lại.
//
// `<Activity mode="hidden">` giữ state, DOM và vị trí cuộn của màn nhưng gỡ effect —
// nên SSE, poll và timer của màn ẩn tự dừng, và React vẫn dựng trước màn ẩn khi rảnh.
// Trước đây chỗ này tự ẩn bằng `display:none` (không dừng effect) hoặc gỡ rồi dựng
// lại (mất state, nháy khung chờ).
import { Activity, type ReactNode } from 'react'
import { MobilePaneScope } from './runtime'

export function KeepAlive({ active, children }: { active: boolean; children: ReactNode }) {
  return <Activity mode={active ? 'visible' : 'hidden'}>{children}</Activity>
}

export type MobilePanesProps = {
  /** `[id, node]` theo thứ tự hiển thị; node giữ nguyên tham chiếu để pane ẩn không render lại. */
  panes: [string, ReactNode][]
  /** pane đang hiển thị; `null` = không pane nào (màn đẩy phủ lên trên). */
  active: string | null
  /** Lớp của khung pane — app đặt tên riêng (Noverse dùng `m-pane` cho CSS cũ). */
  paneClassName?: string
}

/**
 * Các pane của thanh tab: pane đang mở hiện, pane đã mở khác giữ trong
 * `<Activity mode="hidden">` — state, DOM và vị trí cuộn còn nguyên, effect
 * (SSE, poll, timer) tự dừng khi ẩn. Mỗi pane là một cột flex tự cuộn bên trong
 * (màn tự đặt vùng cuộn của nó), nên vị trí cuộn không dùng chung giữa các tab.
 * Pane đang mở mang `data-pane-active` để CSS của app nhắm đúng pane hiện hành.
 */
export function MobilePanes({ panes, active, paneClassName }: MobilePanesProps) {
  return <>{panes.map(([id, node]) => {
    const on = id === active
    return <KeepAlive key={id} active={on}>
      <div
        data-pane={id}
        data-pane-active={on ? '' : undefined}
        className={paneClassName}
        aria-hidden={on ? undefined : true}
        style={{
          display: on ? 'flex' : 'none',
          flexDirection: 'column',
          flexGrow: 1,
          flexShrink: 1,
          flexBasis: 0,
          minHeight: 0,
          overflow: 'hidden',
        }}
      >
        <MobilePaneScope active={on}>{node}</MobilePaneScope>
      </div>
    </KeepAlive>
  })}</>
}
