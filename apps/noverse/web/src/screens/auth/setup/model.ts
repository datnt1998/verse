// A0 · Setup.dc.html — logic thuần của màn Lần chạy đầu (GAP-41).
// Mã thiết lập chỉ in ở máy chủ; mọi thứ ở đây chỉ xử lý chuỗi và mã lỗi của API.

export const CODE_LENGTH = 8
export const ATTEMPT_LIMIT = 5
export const MIN_PASSWORD = 8
/** Tên đăng nhập: chữ thường không dấu, số, “.”, “_”, “-” · 2–32 ký tự (09-auth-onboard §4). */
export const USERNAME_RE = /^[a-z0-9._-]{2,32}$/

/** 8 trạng thái của artboard `state`: code · codeWrong · codeLocked · codeExpired · account · creating · tokenExpired · taken. */
export type SetupUiState =
  | 'code' | 'codeWrong' | 'codeLocked' | 'codeExpired'
  | 'account' | 'creating' | 'tokenExpired' | 'taken'

/** Bỏ gạch nối/khoảng trắng, viết hoa, tối đa 8 ký tự — gõ hay dán (có/không gạch) đều được. */
export const normalizeCode = (raw: string): string => raw.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, CODE_LENGTH)

/** `XXXX-XXXX` cho ô nhập ẩn; 8 ô bên dưới tự tách ký tự. */
export const formatCode = (raw: string): string => {
  const code = normalizeCode(raw)
  return code.length > 4 ? `${code.slice(0, 4)}-${code.slice(4)}` : code
}

export type CodeBox = { ch: string; dash: boolean; cls: '' | 'cur' | 'bad' | 'ok' }

/** 8 ô ký tự + gạch phân cách sau ô 4, viền theo trạng thái (Setup.dc.html › boxes). */
export function codeBoxes(raw: string, options: { bad?: boolean; checking?: boolean; accepted?: boolean } = {}): CodeBox[] {
  const code = normalizeCode(raw)
  const full = code.length === CODE_LENGTH
  return Array.from({ length: CODE_LENGTH }, (_, index) => ({
    ch: code[index] ?? '',
    dash: index === 3,
    cls: options.bad && full ? 'bad' : index === code.length && !options.checking ? 'cur' : full && options.accepted ? 'ok' : '',
  }))
}

/** Gợi ý tên đăng nhập từ tên hiển thị: bỏ dấu, đ → d, chữ thường, ký tự lạ → “.”, tối đa 32. */
export const usernameSuggestion = (name: string): string =>
  name.normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .replace(/đ/g, 'd').replace(/Đ/g, 'D')
    .toLowerCase().replace(/[^a-z0-9]+/g, '.')
    .replace(/^\.+|\.+$/g, '').slice(0, 32)

export const usernameValid = (value: string): boolean => USERNAME_RE.test(value)

/** Phút còn lại để tạo tài khoản sau khi mã đúng (làm tròn lên) — “Còn 27 phút”. */
export function minutesLeft(expiresAt: string | null | undefined, now = Date.now()): number {
  if (!expiresAt) return 0
  const ms = new Date(expiresAt).getTime() - now
  return Number.isNaN(ms) ? 0 : Math.max(0, Math.ceil(ms / 60_000))
}

/** Khoá 10 phút của máy chủ (GAP-41) — dùng khi phản hồi thiếu `lockedUntil`. */
export const LOCK_MINUTES = 10

/**
 * Mốc “Thử lại lúc 21:44”: giờ:phút của `lockedUntil`; máy chủ không kèm mốc
 * (nhánh 400 hết lượt) thì lấy bây giờ + 10 phút, đúng thời lượng khoá của GAP-41.
 */
export function lockClock(lockedUntil: string | null | undefined, now = Date.now()): string {
  const date = lockedUntil ? new Date(lockedUntil) : null
  const at = date && !Number.isNaN(date.getTime()) ? date : new Date(now + LOCK_MINUTES * 60_000)
  return `${String(at.getHours()).padStart(2, '0')}:${String(at.getMinutes()).padStart(2, '0')}`
}

/** Giờ:phút của mốc “lúc 21:04” trong thẻ taken. */
export function clockOf(value: string | null | undefined): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

const payloadField = (payload: unknown, key: string): unknown =>
  payload && typeof payload === 'object' && key in payload ? (payload as Record<string, unknown>)[key] : undefined

export type VerifyFailure =
  | { state: 'codeWrong'; attemptsLeft: number }
  /** Mã sai định dạng: máy chủ không tính lượt thử (GAP-41 bổ sung). */
  | { state: 'codeFormat' }
  | { state: 'codeLocked'; lockedUntil: string }
  | { state: 'codeExpired' }
  | { state: 'taken'; at: string }

/**
 * POST /api/setup/verify: 400 còn lượt · 423 khoá · 410 hết hạn · 409 đã có quản trị.
 * 400 `{format:true}` = mã sai định dạng, **không** trừ lượt thử nên không bao giờ khoá.
 */
export function verifyFailure(status: number, payload: unknown): VerifyFailure | null {
  if (status === 423) return { state: 'codeLocked', lockedUntil: String(payloadField(payload, 'lockedUntil') ?? '') }
  if (status === 410) return { state: 'codeExpired' }
  if (status === 409) return { state: 'taken', at: String(payloadField(payload, 'at') ?? '') }
  if (status === 400) {
    if (payloadField(payload, 'format') === true) return { state: 'codeFormat' }
    const left = Number(payloadField(payload, 'attemptsLeft') ?? 0)
    // Hết lượt (0) → khoá; server cũng trả 423 cho lần sai thứ 5, nhánh này là lưới an toàn.
    return left > 0 ? { state: 'codeWrong', attemptsLeft: left } : { state: 'codeLocked', lockedUntil: '' }
  }
  return null
}

export type AdminFailure =
  | { state: 'taken'; at: string }
  | { state: 'tokenExpired' }
  | { state: 'field'; field: 'name' | 'username' | 'password'; message: string }

/** POST /api/setup/admin: 410 token hết hạn/dùng rồi · 409 đã có quản trị · 400 theo trường. */
export function adminFailure(status: number, payload: unknown, message = ''): AdminFailure | null {
  if (status === 410) return { state: 'tokenExpired' }
  if (status === 409) return { state: 'taken', at: String(payloadField(payload, 'at') ?? '') }
  if (status === 400) {
    const field = String(payloadField(payload, 'field') ?? '')
    return { state: 'field', field: field === 'username' || field === 'password' ? field : 'name', message }
  }
  return null
}

/** Nút “Tạo tài khoản và tiếp tục” bật khi đủ ba ô và hai mật khẩu giống nhau (Setup.dc.html › accDisabled). */
export const canCreateAdmin = (input: { name: string; username: string; password: string; repeat: string }): boolean =>
  Boolean(input.name.trim()) && usernameValid(input.username) &&
  input.password.length >= MIN_PASSWORD && input.password === input.repeat

/** 3 bước của A0 — bước 3 là A3 luồng quản trị (7 bước); bước 1 là mọi trạng thái của mã. */
export const setupStepIndex = (state: SetupUiState): number =>
  state === 'code' || state === 'codeWrong' || state === 'codeLocked' || state === 'codeExpired' ? 0 : 1
