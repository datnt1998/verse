// Phiên đăng nhập: GET /api/me lúc khởi động, context cho toàn app.
// 401 ở bất cứ API nào → onUnauthorized (App lưu ainovel.next → #/login).
// Chưa có phiên + máy chủ chưa có tài khoản nào (GAP-41) → `setupNeeded` để App mở #/setup thay vì Đăng nhập.
// 403 app_access_denied ≠ 401: phiên thật nhưng chưa được cấp quyền app → `deniedUser`
// (giữ người đang đăng nhập, KHÔNG set user — để không mount dữ liệu/SSE).
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { accessDeniedUser, getSetup, logout, me } from './client'
import type { Session, User } from './types'
import { navigate } from '../router'

/** Kết quả kiểm tra phiên qua /api/me. */
export type SessionRefresh = 'user' | 'denied' | 'anon'

export type SessionState = {
  /** đã gọi xong /api/me lần đầu (dù có hay không có phiên) */
  checked: boolean
  user: User | null
  session: Session | null
  setUser: (user: User | null, session?: Session | null) => void
  /** gọi lại /api/me (sau đổi tên, onboarded…) — trả trạng thái phiên mới. */
  refresh: () => Promise<SessionRefresh>
  /** Máy chủ chưa có tài khoản nào → A0 Setup thay cho màn Đăng nhập. */
  setupNeeded: boolean
  setSetupNeeded: (needed: boolean) => void
  /** D15 — số bài học Não hệ thống chờ duyệt (`/api/me`, chỉ quản trị); 0 với người khác. */
  pendingLessons: number
  setPendingLessons: (n: number) => void
  /**
   * Phiên hợp lệ nhưng tài khoản chưa được cấp quyền vào app này (403
   * app_access_denied): view user an toàn để màn bị chặn hiển thị. `user` vẫn
   * null — các provider dữ liệu/SSE chỉ gắn khi có user, nên chúng không chạy.
   */
  deniedUser: User | null
  setDeniedUser: (user: User | null) => void
  /** Xoá phiên phía máy chủ rồi dọn phiên UI về #/login (Đăng xuất). */
  signOut: () => Promise<void>
  /** Dọn phiên UI về #/login khi máy chủ đã thu hồi phiên (đổi mật khẩu, revoke-all). */
  endSession: () => void
}

const SessionContext = createContext<SessionState>({
  checked: false, user: null, session: null,
  setUser: () => {}, refresh: async () => 'anon',
  setupNeeded: false, setSetupNeeded: () => {},
  pendingLessons: 0, setPendingLessons: () => {},
  deniedUser: null, setDeniedUser: () => {},
  signOut: async () => {}, endSession: () => {},
})

export function SessionProvider({ children }: { children: ReactNode }) {
  const [checked, setChecked] = useState(false)
  const [user, setUserState] = useState<User | null>(null)
  const [session, setSession] = useState<Session | null>(null)
  const [setupNeeded, setSetupNeeded] = useState(false)
  const [pendingLessons, setPendingLessons] = useState(0)
  const [deniedUser, setDeniedUser] = useState<User | null>(null)

  const refresh = async (): Promise<SessionRefresh> => {
    try {
      const result = await me()
      setUserState(result.user); setSession(result.session); setDeniedUser(null)
      setPendingLessons(result.pendingLessons ?? 0)
      setSetupNeeded(false)
      return 'user'
    } catch (error) {
      // 403 app_access_denied: phiên thật, tài khoản chưa được cấp quyền vào app.
      // Giữ người đăng nhập ở deniedUser — KHÔNG set user, KHÔNG đăng xuất:
      // dữ liệu/SSE không mount, nhưng Thử lại/Đăng xuất đều dùng cookie này.
      const denied = accessDeniedUser(error)
      if (denied) { setUserState(null); setSession(null); setDeniedUser(denied); setPendingLessons(0); return 'denied' }
      // 401 = chưa có phiên. Hỏi luôn /api/setup: máy chủ chưa có tài khoản nào thì mở A0.
      // Lỗi khác (mất kết nối) đã xử lý qua handler 401 / banner — giữ nguyên trạng thái.
      try { setSetupNeeded((await getSetup()).needed) } catch { /* banner mất kết nối lo phần này */ }
      return 'anon'
    }
  }
  useEffect(() => {
    void (async () => { await refresh(); setChecked(true) })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const endSession = () => { setUserState(null); setSession(null); setDeniedUser(null); navigate('/login', true) }

  const value: SessionState = {
    checked, user, session, setupNeeded, setSetupNeeded, pendingLessons, setPendingLessons,
    deniedUser, setDeniedUser,
    setUser: (next, nextSession) => {
      setUserState(next); setSession(nextSession === undefined ? null : nextSession); setDeniedUser(null)
      // Đăng nhập không đi qua /api/me: quản trị hỏi lại để có số bài học chờ duyệt (D15).
      setPendingLessons(0)
      if (next?.role === 'admin') void me().then((result) => setPendingLessons(result.pendingLessons ?? 0), () => {})
    },
    // Máy chủ xoá cookie TRƯỚC khi trả kết quả ở nhánh revoke-phiên, nên logout
    // lỗi vẫn để phiên UI được dọn: không kẹt người dùng ở màn bị chặn.
    signOut: async () => { try { await logout() } catch { /* revoke thất bại — phiên UI vẫn dọn */ } endSession() },
    endSession,
    refresh,
  }
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export const useSession = () => useContext(SessionContext)
