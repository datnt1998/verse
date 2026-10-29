// Nhãn enum engine phát ra UI — 06-data-contract.md › Snapshot → UI + 07-copy.md › Trạng thái.
// Một chỗ duy nhất cho `phase`/`flow`/`status`/`advanceMode`/`verdict`/`severity`/`agentState`:
// chip trạng thái, RunHeader, ReviewPanel, ActivityRow đều gọi `enumLabel` thay vì tự map.
import { vi } from './vi'

export type EnumKind = 'status' | 'phase' | 'flow' | 'advanceMode' | 'verdict' | 'severity' | 'agentState'

const MAPS: Record<EnumKind, Record<string, string>> = {
  // statusLabel (READY|RUNNING|REVIEW|REWRITE|COMPLETE) và runtimeState (idle|running|…)
  status: vi.status,
  phase: vi.phase,
  flow: vi.flow,
  advanceMode: vi.advanceMode,
  verdict: vi.verdict,
  severity: vi.severity,
  agentState: vi.workspace.agentState,
}

/**
 * Dịch một mã enum. `status` nhận cả chữ hoa (statusLabel) lẫn chữ thường (runtimeState).
 * Không có trong bảng → trả nguyên mã đã trim (không ẩn, không tự đoán).
 */
export function enumLabel(kind: EnumKind, value: string | null | undefined): string {
  const raw = value?.trim() ?? ''
  if (!raw) return ''
  return MAPS[kind][raw.toLowerCase()] ?? raw
}
