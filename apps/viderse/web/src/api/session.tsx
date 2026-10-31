// Phiên đăng nhập trên kho tài khoản chung: GET /api/me lúc khởi động, context cho toàn app.
// Phân loại phản hồi /api/me một cách chặt chẽ — chỉ 401 mới đường tới
// setup/login; mọi lỗi khác giữ nguyên bản chất của nó:
//   · 200 → user (+ grants, + mustChangePassword) đi cùng phiên
//   · 401 → chưa có phiên: hỏi /api/setup để biết store có thật sự trống không
//   · 403 code:"app_access_denied" → phiên THẬT nhưng chưa được cấp quyền
//     Viderse: giữ user trong trạng thái accessDenied, màn bị chặn (không phải
//     login, không mount dữ liệu/SSE — Root chặn trước startDataEvents)
//   · 403 code:"password_change_required" (hoặc user.mustChangePassword) →
//     phiên THẬT bị ép đổi mật khẩu tạm: màn đổi mật khẩu, route hash không vượt được
//   · 503 / status 0 / 5xx → kho tài khoản hoặc máy chủ không trả lời được:
//     serverReachable=false, màn lỗi có Thử lại — KHÔNG BAO GIỜ màn Setup/Login
//     giả (Setup giả sẽ mời tạo admin trong khi kho đang có tài khoản)
//   · getSetup thất bại sau 401 → cùng coi như không xác định được trạng thái
//     store: màn lỗi + Thử lại, không đoán needed.
// sessionSignal chỉ phát: session-expired (401), access-denied (403 có mã),
// password-required (403 có mã) — listener ở đây đổi trạng thái cổng cho mọi
// request, kể cả caller không đọc payload.
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { ApiError, accessDeniedUser, getSetup, login as loginApi, logout, me, passwordChangeRequired } from './client'
import { onAccessDenied, onPasswordChangeRequired, onSessionExpired } from './sessionSignal'
import type { User } from './types'

/** Kết quả một lần kiểm phiên qua /api/me. */
export type SessionRefresh = 'user' | 'denied' | 'password' | 'anon' | 'down'

export type SessionState = {
  /** đã gọi xong /api/me lần đầu (dù có hay không có phiên) */
  checked: boolean
  user: User | null
  setUser: (user: User | null) => void
  /** gọi lại /api/me + /api/setup khi cần; trả trạng thái cổng mới. */
  refresh: () => Promise<SessionRefresh>
  /** Máy chủ chưa có tài khoản nào → màn thiết lập thay cho Đăng nhập. */
  setupNeeded: boolean
  setSetupNeeded: (needed: boolean) => void
  /**
   * Phiên hợp lệ nhưng chưa được cấp quyền Viderse (403 app_access_denied):
   * giữ view user an toàn để màn bị chặn hiển thị và để Thử lại/Đăng xuất
   * vẫn dùng đúng cookie phiên này.
   */
  accessDenied: User | null
  setAccessDenied: (user: User | null) => void
  /** Server đánh dấu phải đổi mật khẩu tạm — cổng riêng, không nút bỏ qua. */
  mustChangePassword: boolean
  /** Lần kiểm phiên gần nhất có nhận được phản hồi HTTP nào từ máy chủ không. */
  serverReachable: boolean
  /** Đăng nhập qua /api/login của Viderse: 2xx → vào app; 403 no-grant → màn bị chặn. */
  signIn: (username: string, password: string) => Promise<void>
  /** Revoke phiên trên máy chủ rồi dọn phiên UI (Đăng xuất — thoát cả hai app). */
  signOut: () => Promise<void>
  /** Dọn phiên UI về Đăng nhập khi máy chủ đã thu hồi phiên (đổi mật khẩu, revoke-all, 401). */
  endSession: () => void
}

const SessionContext = createContext<SessionState>({
  checked: false,
  user: null,
  setUser: () => {},
  refresh: async () => 'anon',
  setupNeeded: false,
  setSetupNeeded: () => {},
  accessDenied: null,
  setAccessDenied: () => {},
  mustChangePassword: false,
  serverReachable: true,
  signIn: async () => {},
  signOut: async () => {},
  endSession: () => {},
})

export function SessionProvider({ children }: { children: ReactNode }) {
  const [checked, setChecked] = useState(false)
  const [user, setUserState] = useState<User | null>(null)
  const [setupNeeded, setSetupNeeded] = useState(false)
  const [accessDenied, setAccessDeniedState] = useState<User | null>(null)
  const [mustChangePassword, setMustChangePassword] = useState(false)
  const [serverReachable, setServerReachable] = useState(true)

  const applyUser = (next: User | null) => {
    setUserState(next)
    setMustChangePassword(next?.mustChangePassword === true)
    if (next) setAccessDeniedState(null)
  }
  const endSession = () => {
    setUserState(null)
    setAccessDeniedState(null)
    setMustChangePassword(false)
  }
  const setAccessDenied = (denied: User | null) => {
    if (denied) {
      // Giữ người đang đăng nhập trong user để màn bị chặn hiển thị và các
      // route auth-neutral (logout, /api/me/sessions, đổi mật khẩu) vẫn đi
      // bằng cookie này. Root chặn dữ liệu/SSE qua cờ accessDenied.
      setUserState(denied)
      setAccessDeniedState(denied)
      setMustChangePassword(false)
    } else {
      setAccessDeniedState(null)
    }
  }

  const refresh = async (): Promise<SessionRefresh> => {
    try {
      const result = await me()
      applyUser(result.user)
      setSetupNeeded(false)
      setServerReachable(true)
      return result.user.mustChangePassword === true ? 'password' : 'user'
    } catch (error) {
      const denied = accessDeniedUser(error)
      if (denied) {
        setAccessDenied(denied)
        setSetupNeeded(false)
        setServerReachable(true)
        return 'denied'
      }
      if (passwordChangeRequired(error)) {
        // /api/me lẽ ra là auth-neutral; nếu server vẫn trả mã này thì tôn
        // trọng nó: đưa về cổng đổi mật khẩu với user hiện có (nếu đã có).
        setMustChangePassword(true)
        setSetupNeeded(false)
        setServerReachable(true)
        return 'password'
      }
      if (error instanceof ApiError && error.status !== 401 && (error.status === 0 || error.status >= 500)) {
        // Mất kết nối hoặc kho tài khoản 503: không suy ra "chưa đăng nhập"
        // hay "store trống" — chỉ báo không kiểm được và giữ nguyên phiên UI.
        setServerReachable(false)
        return 'down'
      }
      // 401 (và mọi lỗi 4xx còn lại — coi như không có phiên): dọn phiên UI
      // rồi hỏi /api/setup để phân biệt "store trống" với "chưa đăng nhập".
      endSession()
      try {
        setSetupNeeded((await getSetup()).needed)
        setServerReachable(true)
      } catch (setupError) {
        if (setupError instanceof ApiError && (setupError.status === 0 || setupError.status >= 500)) {
          // getSetup 503 ≠ "trống": không hiện màn Setup/Login giả.
          setSetupNeeded(false)
          setServerReachable(false)
          return 'down'
        }
        setSetupNeeded(false)
      }
      return 'anon'
    }
  }

  useEffect(() => {
    void (async () => { await refresh(); setChecked(true) })()
    // 401 ở bất kỳ request nào (kể cả luồng SSE) → hết phiên: dọn UI về cổng.
    // Hai tín hiệu 403 cập nhật cổng từ mọi request chứ không chỉ /api/me.
    const offExpired = onSessionExpired(endSession)
    const offDenied = onAccessDenied(setAccessDenied)
    const offPassword = onPasswordChangeRequired(() => setMustChangePassword(true))
    return () => { offExpired(); offDenied(); offPassword() }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const value: SessionState = {
    checked, user, refresh, setupNeeded, setSetupNeeded,
    accessDenied, setAccessDenied, mustChangePassword, serverReachable,
    setUser: applyUser,
    signIn: async (username, password) => {
      // login() đã phát access-denied qua transport; vẫn xử lý tại đây để
      // không phụ thuộc listener khi provider chưa sẵn sàng trong test.
      try {
        applyUser((await loginApi(username, password)).user)
      } catch (error) {
        const denied = accessDeniedUser(error)
        if (denied) { setAccessDenied(denied); return }
        throw error
      }
    },
    // Server revoke trước khi xoá cookie: lỗi revoke thì ném ra để caller hiện
    // lỗi (không báo giả đã đăng xuất); 401 = phiên đã mất → dọn UI luôn.
    signOut: async () => {
      try { await logout() } catch (error) {
        if (!(error instanceof ApiError && error.status === 401)) throw error
      }
      endSession()
    },
    endSession,
  }
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export const useSession = () => useContext(SessionContext)
