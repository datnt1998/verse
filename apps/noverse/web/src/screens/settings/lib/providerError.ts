// Gói AU — lỗi nhà cung cấp dùng chung một cách nói ở mọi màn (Settings, MMore, CmdResults,
// Bàn viết, MNewStory). Máy chủ phân loại expired · key · quota; quota là tạm thời nên không
// bao giờ được nói như hết hạn gói.
import type { ConfigResponse } from '../../../api/types'

export type ProviderIssueKind = 'expired' | 'key' | 'quota'
export type ProviderIssue = { kind: ProviderIssueKind; message: string; retryAt?: string }

/** `/api/config` trả `providerErrors[id] = {kind, message, retryAt?}`. Bản máy chủ cũ trả chuỗi
 *  (“khoá không dùng được”) — vẫn đọc được, coi là lỗi khoá, không bịa thêm loại. */
export function providerIssue(errors: unknown, provider: string): ProviderIssue | null {
  if (!provider) return null
  const raw = (errors as Record<string, unknown> | undefined)?.[provider]
  if (!raw) return null
  if (typeof raw === 'string') return raw ? { kind: 'key', message: raw } : null
  const value = raw as { kind?: unknown; message?: unknown; retryAt?: unknown }
  const kind: ProviderIssueKind = value.kind === 'expired' || value.kind === 'quota' ? value.kind : 'key'
  return { kind, message: typeof value.message === 'string' ? value.message : '', retryAt: typeof value.retryAt === 'string' ? value.retryAt : undefined }
}

/** Tài khoản hết hạn gói + lỗi khoá/quota của một nhà cung cấp. */
export function issueOf(response: ConfigResponse | null | undefined, provider: string): ProviderIssue | null {
  if (!provider) return null
  if (response?.accounts?.[provider]?.expired) return { kind: 'expired', message: '' }
  return providerIssue(response?.providerErrors, provider)
}

export const issueTitle = (name: string, kind: ProviderIssueKind): string =>
  kind === 'expired' ? `${name} đã hết hạn gói`
    : kind === 'key' ? `Khoá ${name} không còn dùng được`
      : `${name} tạm hết lượt`

export const issueShort = (kind: ProviderIssueKind): string =>
  kind === 'expired' ? 'hết hạn gói' : kind === 'key' ? 'khoá không dùng được' : 'tạm hết lượt'

/** Nhãn mức trong thẻ lỗi: hai lỗi phải sửa, quota chỉ là chờ. */
export const issueLevel = (kind: ProviderIssueKind): string => (kind === 'quota' ? 'Cảnh báo' : 'Bắt sửa')

export const issueAction = (kind: ProviderIssueKind): string =>
  kind === 'expired' ? 'Đăng nhập lại' : kind === 'key' ? 'Nhập khoá mới' : ''

export const issueText = (kind: ProviderIssueKind): string =>
  kind === 'expired' ? 'Viết tiếp bằng nhà cung cấp khác, hoặc gia hạn rồi đăng nhập lại.'
    : kind === 'key' ? 'Khoá đã bị thu hồi hoặc hết hạn. Nhập khoá mới, hoặc viết tiếp bằng nhà cung cấp khác.'
      : 'Nhà cung cấp tạm giới hạn số lần gọi. Noverse tự thử lại sau ít phút.'

/** “21:58” theo giờ máy; rỗng khi máy chủ không cho giờ thử lại. */
export function retryClock(retryAt: string | undefined): string {
  if (!retryAt) return ''
  const at = new Date(retryAt)
  if (Number.isNaN(at.getTime())) return ''
  return `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`
}

/** “khoảng 4 phút nữa” — dùng cho câu quota; rỗng khi không có mốc. */
export function retryIn(retryAt: string | undefined, now = Date.now()): string {
  if (!retryAt) return ''
  const at = new Date(retryAt).getTime()
  if (Number.isNaN(at)) return ''
  const minutes = Math.max(0, Math.round((at - now) / 60_000))
  if (minutes <= 0) return 'ngay bây giờ'
  return minutes === 1 ? 'khoảng 1 phút nữa' : `khoảng ${minutes} phút nữa`
}

/** Câu đầy đủ cho quota: “Noverse tự thử lại lúc 21:58 (khoảng 4 phút nữa); không cần đổi gì.” */
export function quotaRetryText(retryAt: string | undefined, now = Date.now()): string {
  const clock = retryClock(retryAt)
  if (!clock) return 'Noverse tự thử lại sau ít phút; không cần đổi gì.'
  const left = retryIn(retryAt, now)
  return `Noverse tự thử lại lúc ${clock}${left ? ` (${left})` : ''}; không cần đổi gì.`
}

/** Chi tiết kỹ thuật chỉ nói điều máy chủ thật sự gửi — không bịa mã HTTP. */
export function issueTech(provider: string, issue: ProviderIssue): string {
  const retry = issue.retryAt ? ` · retryAt=${issue.retryAt}` : ''
  return `provider=${provider} · kind=${issue.kind}${retry}`
}
