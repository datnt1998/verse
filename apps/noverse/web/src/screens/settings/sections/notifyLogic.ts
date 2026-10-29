// Logic thuần của Tuỳ chỉnh › Báo động máy chủ: `config.notify.events` ⇄ công tắc từng sự kiện.
// Quy ước của máy chủ (internal/notify, bootstrap.NotifyConfig): bỏ khoá `events` = mọi sự kiện bật;
// `events` không rỗng = chỉ các sự kiện liệt kê; `enabled: false` = tắt cả kênh báo động.
import type { NotifyConfig } from '../../../api/types'

/** Mọi sự kiện của `notify.Kinds()` (internal/notify/notify.go) — theo đúng thứ tự khai báo ở đó. */
export const NOTIFY_KINDS = ['run_end', 'budget', 'advance_gate', 'stop_guard', 'plan_start', 'deadlock', 'worker_failure'] as const
export type NotifyKind = typeof NOTIFY_KINDS[number]

/** Hàng hiển thị theo artboard (`evRows`). `stop_guard` không có hàng trong artboard nên không hiện. */
export const NOTIFY_ROWS: { kind: NotifyKind; label: string }[] = [
  { kind: 'run_end', label: 'Truyện chạy xong' },
  { kind: 'budget', label: 'Sắp hết ngân sách' },
  { kind: 'advance_gate', label: 'Chương chờ duyệt' },
  { kind: 'deadlock', label: 'Trợ lý bị kẹt lặp lại' },
  { kind: 'plan_start', label: 'Bắt đầu kế hoạch tập mới' },
  { kind: 'worker_failure', label: 'Tiến trình nền bị lỗi' },
]

export type NotifyStates = Record<NotifyKind, boolean>

/** Trạng thái bật/tắt của từng sự kiện suy ra từ cấu hình đã lưu. */
export function notifyStates(notify: NotifyConfig | undefined): NotifyStates {
  const events = notify?.events ?? []
  const all = notify?.enabled === false ? false : events.length === 0
  return Object.fromEntries(NOTIFY_KINDS.map((kind) => [kind, all || (notify?.enabled !== false && events.includes(kind))])) as NotifyStates
}

/** Giá trị ghi xuống `notify.enabled` / `notify.events`; `undefined` = xoá khoá. */
export type NotifyPatch = { enabled: false | undefined; events: string[] | undefined }

/**
 * Bật/tắt một sự kiện: mọi sự kiện bật → xoá `events` (và `enabled`); có sự kiện tắt → ghi danh sách tường minh
 * (tất cả trừ các sự kiện tắt); tắt hết → `enabled: false` (danh sách rỗng sẽ bị máy chủ hiểu là “bật hết”).
 */
export function toggleNotify(notify: NotifyConfig | undefined, kind: NotifyKind, on: boolean): NotifyPatch {
  const states = { ...notifyStates(notify), [kind]: on }
  const enabledKinds = NOTIFY_KINDS.filter((entry) => states[entry])
  if (enabledKinds.length === NOTIFY_KINDS.length) return { enabled: undefined, events: undefined }
  if (enabledKinds.length === 0) return { enabled: false, events: undefined }
  return { enabled: undefined, events: enabledKinds }
}
