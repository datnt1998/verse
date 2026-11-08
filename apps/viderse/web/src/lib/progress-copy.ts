// Câu chữ của Nhật ký tiến độ — một chỗ duy nhất đổi dữ liệu bước (step,
// status, progress.phase, error) thành tiếng Việt hiện trên Bàn dựng.
//
// Máy chủ đã phát `progress.phase` bằng mã cố định; bảng dưới đây là danh sách
// mã đã biết, không suy đoán ngôn ngữ từ chữ. Mã lạ rơi về câu tiếng Việt theo
// nhãn bước, còn mã gốc vẫn nằm nguyên trong `step.progress.phase` của dữ liệu,
// log và bản xuất hỗ trợ.
//
// `error.message` là chẩn đoán của máy chủ (việc cũ có thể còn tiếng Anh trên
// đĩa). Nhật ký KHÔNG hiện nguyên văn trường này: câu hiện chọn theo `error.code`
// đã biết, mã lạ rơi về câu lỗi chung. Bản gốc vẫn nằm trong JSON job, log và
// gói hỗ trợ nên không mất nguyên nhân.
import type { JobStep } from '../api/workbench'
import { progressCopy } from '../i18n/screens'
import { cloneWaitReason, cloneWaitText } from './clone/clone-wait'

/** Phần dữ liệu của một bước mà nhật ký thật sự đọc — `CloneStep` trên dây. */
export type ProgressStep = Pick<JobStep, 'step' | 'status' | 'progress' | 'error'>

const lowerFirst = (text: string) => text.charAt(0).toLocaleLowerCase('vi') + text.slice(1)

// Mã lỗi kết nối nhà cung cấp (aiconn báo `auth_failed`; imagegen trả
// `provider_auth` khi phiên đăng nhập của nguồn vẽ đã hết hạn). Giữ phân biệt
// hết phiên đăng nhập với khoá bị từ chối — hạn mức đã có câu riêng trong
// `progressCopy.errors.quota`. Bản cũ trên đĩa có thể còn câu tiếng Anh, nên
// câu hiện vẫn chọn theo mã, không đọc `error.message`.
const providerExpired = 'Nhà cung cấp đã hết hạn đăng nhập. Đăng nhập lại rồi chạy tiếp.'
const providerKeyRejected = 'Nhà cung cấp từ chối khoá API. Sửa khoá ở Kết nối AI rồi chạy tiếp.'
const providerErrors: Record<string, string> = {
  provider_auth: providerExpired,
  login_required: providerExpired,
  login_expired: providerExpired,
  auth_expired: providerExpired,
  auth_failed: providerKeyRejected,
  provider_key: providerKeyRejected,
  invalid_api_key: providerKeyRejected,
}

/** Câu "đang làm" khi chưa biết pha: dùng nhãn bước, không đoán việc cụ thể. */
function runningFallback(step: string): string {
  const label = progressCopy.steps[step]
  return label ? `Đang ${lowerFirst(label)}` : progressCopy.unknownRunning
}

/**
 * Câu mô tả bước trong nhật ký, luôn tiếng Việt và không rỗng.
 *
 * - Có `error.code`: câu theo mã (nguyên nhân + việc làm tiếp, kể cả hết phiên
 *   đăng nhập / khoá bị từ chối / hạn mức của nhà cung cấp) hoặc câu lỗi chung
 *   khi mã lạ; không hiện `error.message`/mã trần.
 * - Đang chạy: câu của `progress.phase` đã biết; mã lạ → "Đang <nhãn bước>".
 * - Các trạng thái khác: nhãn bước, hoặc câu huỷ/cũ theo trạng thái.
 *
 * Không kèm chữ trạng thái — dùng thêm `progressState` khi cần ("Xong", "Chờ lượt").
 */
export function progressText(step: ProgressStep): string {
  const code = step.error?.code?.trim()
  const waitReason = cloneWaitReason(step.step, step.error)
  if (waitReason) return cloneWaitText(waitReason)
  if (code) return providerErrors[code] ?? progressCopy.errors[code] ?? progressCopy.failed
  if (step.status === 'failed') return progressCopy.failed
  if (step.status === 'cancelled') return progressCopy.cancelled
  if (step.status === 'stale') return progressCopy.stale
  const label = progressCopy.steps[step.step] ?? progressCopy.unknownStep
  if (step.status !== 'running') return label
  const phase = step.progress?.phase?.trim() ?? ''
  if (!phase) return runningFallback(step.step)
  const mapped = progressCopy.phases[`${step.step}:${phase}`] ?? progressCopy.phases[lowerFirst(phase)]
  return mapped ?? runningFallback(step.step)
}

/**
 * Nhãn trạng thái bước. Trạng thái lạ trả "" để chỗ gọi bỏ hẳn nhãn — không
 * bao giờ hiện mã enum tiếng Anh thay cho câu Việt.
 */
export function progressState(status: string): string {
  const key = status?.trim().toLocaleLowerCase('en') ?? ''
  return progressCopy.states[key] ?? ''
}
