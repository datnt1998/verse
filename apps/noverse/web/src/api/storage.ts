// Khóa lưu cục bộ — 11-api-contract.md §8. Không bao giờ lưu mật khẩu/cookie.
import type { RememberedAccount } from './types'

const read = <T>(key: string, fallback: T): T => {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : (JSON.parse(raw) as T)
  } catch { return fallback }
}
const write = (key: string, value: unknown) => { try { localStorage.setItem(key, JSON.stringify(value)) } catch { /* đầy */ } }

// ainovel.accounts — tài khoản đã dùng trên máy (chỉ thông tin hiển thị)
export const getRemembered = (): RememberedAccount[] => {
  const entries = read<unknown>('ainovel.accounts', [])
  if (!Array.isArray(entries)) return []
  return entries.filter((item): item is RememberedAccount => Boolean(item && typeof item === 'object' &&
    typeof item.username === 'string' && typeof item.name === 'string' && typeof item.lastLoginAt === 'string' &&
    ['admin', 'member'].includes(item.role))).map(({ username, name, role, lastLoginAt }) => ({ username, name, role, lastLoginAt })).slice(0, 8)
}
export const rememberAccount = ({ username, name, role, lastLoginAt }: RememberedAccount) =>
  write('ainovel.accounts', [{ username, name, role, lastLoginAt }, ...getRemembered().filter((item) => item.username !== username)].slice(0, 8))
export const forgetAccount = (username: string) =>
  write('ainovel.accounts', getRemembered().filter((item) => item.username !== username))

// ainovel.next — route quay lại sau đăng nhập (sessionStorage). Giá trị đọc ra
// luôn qua safeHashPath: chỉ chấp nhận đường trong app, không bao giờ điều hướng
// ra ngoài (router chỉ đổi location.hash, nhưng giá trị lạ vẫn bị loại).
export const setNextRoute = (route: string) => { try { sessionStorage.setItem('ainovel.next', route) } catch { /* */ } }
export const takeNextRoute = (): string | null => {
  try { const v = safeHashPath(sessionStorage.getItem('ainovel.next')); sessionStorage.removeItem('ainovel.next'); return v } catch { return null }
}

/**
 * Đường đích nội bộ hợp lệ để quay lại: bắt đầu bằng một `/` (tùy chọn `#` đứng
 * trước), ký tự thứ hai không phải `/` hay `\`, và sau khi giải mã URL một lần
 * vẫn giữ dạng đó. Loại `//evil`, `\\evil`, `javascript:` và mọi bypass đã mã
 * hoá — chúng không phải route trong app. Trả path không có `#`, hoặc null.
 */
export function safeHashPath(value: string | null | undefined): string | null {
  const raw = (value ?? '').trim().replace(/^#/, '')
  if (!raw.startsWith('/') || raw[1] === '/' || raw[1] === '\\') return null
  let decoded: string
  try { decoded = decodeURIComponent(raw) } catch { return null }
  if (!decoded.startsWith('/') || decoded[1] === '/' || decoded[1] === '\\') return null
  // Segment mã hoá giấu ký tự điều hướng: %2f/%5c/%00 mở ra `//`, `\\`, null-byte —
  // trình định tuyến decode khi match nên chặn ngay tại nguồn.
  if (/%(2f|5c|00)/i.test(raw)) return null
  return raw
}

// ─── Phiên hết hạn (09-auth-onboard.md › F1b) ───────────────────────────
/**
 * 401 có phải "phiên đã hết hạn"? Chỉ khi trình duyệt này TỪNG có phiên:
 * `/api/me` đã thành công trong vòng đời app này (`hadSession`), hoặc máy đã từng đăng
 * nhập — `ainovel.accounts` còn tài khoản (phần tử đầu là lần đăng nhập gần nhất).
 * 401 ở trình duyệt sạch nghĩa là chưa đăng nhập, không phải hết hạn.
 */
export const sessionExpired = (hadSession: boolean, rememberedCount = getRemembered().length) =>
  hadSession || rememberedCount > 0

/** Màn đăng nhập tương ứng: chỉ thêm `?expired=1` khi phiên thật sự hết hạn. */
export const loginPath = (expired: boolean) => (expired ? '/login?expired=1' : '/login')

// ainovel.panels — ẩn/hiện 2 panel Workspace
export type PanelState = { left: boolean; right: boolean }
export const getPanels = () => read<PanelState>('ainovel.panels', { left: true, right: true })
export const setPanels = (panels: PanelState) => write('ainovel.panels', panels)

// ainovel.readerSize — cỡ chữ Trình đọc 14–24
export const getReaderSize = () => Math.min(24, Math.max(14, read<number>('ainovel.readerSize', 17)))
export const setReaderSize = (size: number) => write('ainovel.readerSize', Math.min(24, Math.max(14, size)))

// ainovel.readerFont — Tuỳ chỉnh › Giao diện ứng dụng. (27/10: v1 chỉ nền tối, không còn ainovel.theme.)
export const getReaderFont = () => read<string>('ainovel.readerFont', 'literata')
export const setReaderFont = (font: string) => write('ainovel.readerFont', font)

// ainovel.lastRoute.<username> — route cuối trước khi thoát
export const setLastRoute = (username: string, route: string) => write(`ainovel.lastRoute.${username}`, route)
export const getLastRoute = (username: string) => read<string | null>(`ainovel.lastRoute.${username}`, null)

// ainovel.lastStory.<username> — truyện mở gần nhất; điện thoại mở thẳng Viết của truyện này khi vào `#/` (18-mobile không quy định, chọn có chủ đích)
export type LastStory = { project: string; story: string }
export const setLastStory = (username: string, value: LastStory) => write(`ainovel.lastStory.${username}`, value)
export const getLastStory = (username: string): LastStory | null => {
  const value = read<Partial<LastStory> | null>(`ainovel.lastStory.${username}`, null)
  return value && typeof value.project === 'string' && typeof value.story === 'string' && value.project && value.story ? { project: value.project, story: value.story } : null
}
