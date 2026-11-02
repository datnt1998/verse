// Câu lỗi hệ thống hiện cho người dùng: ưu tiên lời chẩn đoán máy chủ khi đó là
// câu tiếng Việt thật (giữ nguyên tên riêng/model), còn mã lỗi hoặc lời tiếng Anh
// chỉ nằm trong dữ liệu — trên giao diện luôn có một câu Việt đúng loại lỗi.
import type { AssistantErrorKind } from '../../api/assistant'
import { assistantCopy } from '../../i18n/assistant'

const VIETNAMESE = /[\u00c0-\u024f\u1e00-\u1eff]/i

export function systemErrorText(kind: AssistantErrorKind, message: string, duringRun = false): string {
  const text = message.trim()
  if (kind === 'failed' && text && VIETNAMESE.test(text)) return text
  if (kind === 'failed' && duringRun) return assistantCopy.states.runFailed
  return assistantCopy.states[kind]
}
