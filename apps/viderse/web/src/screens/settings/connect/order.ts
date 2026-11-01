import type { AiProvider } from '../../../api/ai'

/** Thẻ đang lỗi (khoá bị từ chối / không với tới) — hiện cam, lên đầu danh sách. */
export const isFailed = (p: Pick<AiProvider, 'status'>) => p.status === 'auth_failed' || p.status === 'unreachable'

/** Thứ tự thẻ Kết nối AI (hỏi đáp #27): thẻ vừa thêm đứng đầu tới khi rời
 *  trang, rồi thẻ lỗi, rồi phần còn lại. Server trả theo thứ tự thêm (cũ
 *  trước) và sắp ổn định giữ nguyên thứ tự đó trong từng nhóm. */
export function orderProviders(list: readonly AiProvider[], pinnedId: string | null): AiProvider[] {
  const rank = (p: AiProvider) => (p.id === pinnedId ? 0 : isFailed(p) ? 1 : 2)
  return [...list].sort((a, b) => rank(a) - rank(b))
}

/** Ollama và Tương thích OpenAI được nhiều thẻ khi khác địa chỉ — chọn lại ô
 *  của chúng vẫn mở form thêm; mọi loại khác đã có thẻ thì chuyển tới thẻ đó. */
export const MULTI_CARD_KINDS: ReadonlySet<string> = new Set(['ollama', 'openai_compat'])

/** Id thẻ đã có của `kind` mà ô nguồn phải dẫn tới, hoặc undefined khi được thêm thẻ mới. */
export function existingCardFor(list: readonly AiProvider[], kind: string): string | undefined {
  if (MULTI_CARD_KINDS.has(kind)) return undefined
  return list.find((p) => p.kind === kind)?.id
}
