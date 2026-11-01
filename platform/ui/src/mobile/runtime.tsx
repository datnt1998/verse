// Phạm vi pane — phần runtime dùng chung mà `keepAlive.tsx` cần.
//
// MSafe sở hữu tệp này khi chuyển runtime điện thoại của Noverse lên kit
// (`MobileRuntimeProvider`, `useTabMode`, `useMobileScroll`…); bản ở đây chỉ
// chứa đúng phạm vi pane để `MobilePanes` không phải chờ, và để hook của màn
// nằm trong pane ẩn không đẩy trạng thái lên runtime.
import { createContext, useContext, type ReactNode } from 'react'

/** `false` khi màn đang mounted nhưng ẩn (`display:none`). */
const PaneActive = createContext(true)

/** Pane chứa hook này có đang hiển thị không — hook đẩy trạng thái runtime phải hỏi trước. */
export const useMobilePaneActive = () => useContext(PaneActive)

export function MobilePaneScope({ active, children }: { active: boolean; children: ReactNode }) {
  return <PaneActive.Provider value={active}>{children}</PaneActive.Provider>
}
