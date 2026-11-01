// Mốc "còn mới" cho màn nằm trong pane giữ-alive của thanh tab.
//
// Pane ẩn bị `<Activity>` gỡ effect, nên khi quay lại tab các hook `useResource`
// giữ lại khoá và — với mốc mặc định 2 s — tải lại toàn bộ dữ liệu của màn vừa
// mở. Trong pane, đổi tab không phải là "mount lại": dữ liệu đã có vẫn dùng,
// SSE (`data/events.ts`) và `refresh()` sau thao tác ghi mới là đường làm mới.
// Phạm vi hẹp có chủ đích: hộp thoại, sheet và màn máy tính vẫn giữ mốc mặc định.
import { createContext } from 'react'

/** 30 s — đủ dài cho một vòng đổi tab, đủ ngắn để mở lại app vẫn thấy dữ liệu mới. */
export const PANE_STALE_MS = 30_000

export const PaneStaleContext = createContext<number | undefined>(undefined)
