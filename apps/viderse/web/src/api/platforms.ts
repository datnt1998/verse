// Client khu vực nền tảng — hợp đồng 07 §24/§24.1, khớp
// `internal/web/platforms.go`: platform {id,label,uses,kinds,targetHeight,
// login{type,saved,masked,account,browserProfile,method,renewable,
// lastRefreshAt,refreshError},state,pausedUntil,pauseReason,failureCount,
// lastError{code,at,jobId},usage{channels,videos,rivals}}; QR {qrcodeKey,
// qrImage (data URL), loginUrl, expiresAt} + poll theo qrcodeKey.
import { mutate, request } from './request'

export type PlatformId = 'youtube' | 'twitch' | 'bilibili'

export type PlatformState = 'ready' | 'paused' | 'needs_login'

export type PlatformLoginType = 'none' | 'browser_cookie' | 'cookie_string' | 'qr'

export type PlatformLogin = {
  type: PlatformLoginType
  saved: boolean
  masked?: string
  account?: string
  /** "Chrome:Kênh chính" — hồ sơ đã chọn cho YouTube. */
  browserProfile?: string
  method?: string
  renewable: boolean
  lastRefreshAt?: string
  refreshError?: string
}

export type PlatformLastError = { code: string; at: string; jobId?: string }

export type PlatformUsage = { channels: number; videos: number; rivals?: number }

export type Platform = {
  id: PlatformId | string
  label: string
  uses: string[]
  kinds: string[]
  targetHeight: number
  login: PlatformLogin
  state: PlatformState
  pausedUntil?: string
  pauseReason?: string
  failureCount?: number
  lastError?: PlatformLastError
  usage: PlatformUsage
}

export type BrowserProfile = { browser: string; profile: string; name: string; email?: string }

export type QrSession = { qrcodeKey: string; qrImage: string; loginUrl: string; expiresAt: string }

export type QrStatus = 'waiting' | 'scanned' | 'expired' | 'success' | 'rejected'

export type QrPoll = { status: QrStatus; account?: string }

// ---------- Danh sách ----------

export const listPlatforms = (signal?: AbortSignal) => request<Platform[]>('/api/platforms', { signal })

/** Hồ sơ trình duyệt để lấy cookie (refresh=1 buộc quét lại; mặc định dùng cache). */
export const listBrowserProfiles = (id: string, opts?: { refresh?: boolean; signal?: AbortSignal }) =>
  request<BrowserProfile[]>(`/api/platforms/${id}/login/profiles${opts?.refresh ? '?refresh=1' : ''}`, { signal: opts?.signal })

// ---------- Đăng nhập ----------

/** YouTube: {browser,profile}; Bilibili: {cookie} → login mới. */
export function putPlatformLogin(id: string, input: { browser?: string; profile?: string; cookie?: string }): Promise<PlatformLogin> {
  return mutate('PUT', `/api/platforms/${id}/login`, input)
}

/** Kiểm cookie đã lưu (banner bot "Thử lại") — không lấy lại cookie. */
export function checkPlatformLogin(id: string): Promise<PlatformLogin> {
  return mutate('POST', `/api/platforms/${id}/login/check`)
}

/** Gia hạn đăng nhập đang có (QR có refresh token) → {…login, status}. */
export function refreshPlatformLogin(id: string): Promise<PlatformLogin & { status: string }> {
  return mutate('POST', `/api/platforms/${id}/login/refresh`)
}

export function deletePlatformLogin(id: string): Promise<PlatformLogin> {
  return mutate('DELETE', `/api/platforms/${id}/login`)
}

/** Bilibili: tạo phiên QR mới. */
export function createPlatformQr(id: string): Promise<QrSession> {
  return mutate('POST', `/api/platforms/${id}/login/qr`)
}

/** Poll trạng thái QR theo qrcodeKey — chỉ gọi khi dialog/sheet đang mở. */
export const getPlatformQr = (id: string, key: string, signal?: AbortSignal) =>
  request<QrPoll>(`/api/platforms/${id}/login/qr/${encodeURIComponent(key)}`, { signal })
