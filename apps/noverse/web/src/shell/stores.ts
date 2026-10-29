// Stores nhỏ của shell: trạng thái kết nối SSE (banner) + ẩn/hiện panel.
// useStoryEvents → screens gọi setConnection; AppShell render ConnectionBanner.
import { useEffect, useState } from 'react'
import { getPanels, setPanels as persistPanels } from '../api/storage'

// ─── Kết nối ───
type Conn = { reconnectAt: number | null; retry: () => void; locked: boolean }
let conn: Conn = { reconnectAt: null, retry: () => {}, locked: false }
const connSubs = new Set<() => void>()
const emit = () => connSubs.forEach((fn) => fn())

export function setConnection(reconnectAt: number | null, retry?: () => void) {
  conn = { ...conn, reconnectAt, retry: retry ?? conn.retry }
  emit()
}
/** 409 errBookLocked từ worker → AppShell bật LockedDialog (Dialogs.dc.html › locked). */
export function setLocked(locked: boolean) {
  if (conn.locked === locked) return
  conn = { ...conn, locked }
  emit()
}
export function useConnection(): Conn {
  const [state, setState] = useState(conn)
  useEffect(() => {
    const update = () => setState({ ...conn })
    connSubs.add(update)
    return () => { connSubs.delete(update) }
  }, [])
  return state
}

// ─── Nháp ô chat: QuickActions Trình đọc / IssueCard → Workspace ───
// F2 gọi setComposerDraft() NGAY TRƯỚC navigate(paths.workspace(p, s)).
// WorkspaceScreen đọc một lần khi mount: useState(() => takeComposerDraft(p, s)).
// Không tự gửi (04-components.md › QuickActions); sessionStorage = chỉ tab hiện tại.
const draftKey = (p: string, s: string) => `ainovel.composer.${p}/${s}`

export function setComposerDraft(project: string, story: string, text: string) {
  try { sessionStorage.setItem(draftKey(project, story), text) } catch { /* chế độ riêng tư */ }
}

/** Đọc và xoá nháp — dùng một lần để quay lại Workspace không prefill lại nháp cũ. */
export function takeComposerDraft(project: string, story: string): string {
  const key = draftKey(project, story)
  try {
    const text = sessionStorage.getItem(key) ?? ''
    sessionStorage.removeItem(key)
    return text
  } catch { return '' }
}

// ─── Panel Workspace (ainovel.panels) ───
type PanelListener = (p: { left: boolean; right: boolean }) => void
let panels = getPanels()
const panelSubs = new Set<PanelListener>()
export function togglePanel(side: 'left' | 'right') {
  panels = { ...panels, [side]: !panels[side] }
  persistPanels(panels)
  panelSubs.forEach((fn) => fn(panels))
}
export function usePanels() {
  const [state, setState] = useState(panels)
  useEffect(() => {
    const update = (p: typeof panels) => setState({ ...p })
    panelSubs.add(update)
    return () => { panelSubs.delete(update) }
  }, [])
  return { ...state, toggle: togglePanel }
}
