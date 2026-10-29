import { copy } from '../copy'
import { copyText } from '../../../lib/clipboard'

/** Trình duyệt từ chối ghi clipboard (tài liệu mất tiêu điểm, không có quyền) → luôn báo bằng tiếng Việt, không lộ lỗi thô của trình duyệt. */
export async function copyToClipboard(text: string): Promise<void> {
  try { await copyText(text) }
  catch { throw new Error(copy.export.copyFailed) }
}
