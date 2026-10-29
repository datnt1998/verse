// Phiên đăng nhập: GET /api/me lúc khởi động, context cho toàn app.
// 401 ở bất cứ API nào → onUnauthorized (App lưu ainovel.next → #/login).
// Chưa có phiên + máy chủ chưa có tài khoản nào (GAP-41) → `setupNeeded` để App mở #/setup thay vì Đăng nhập.
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { getSetup, me } from './client'
import type { Session, User } from './types'

export type SessionState = {
  /** đã gọi xong /api/me lần đầu (dù có hay không có phiên) */
  checked: boolean
  user: User | null
  session: Session | null
  setUser: (user: User | null, session?: Session | null) => void
  /** gọi lại /api/me (sau đổi tên, onboarded…) */
  refresh: () => Promise<void>
  /** Máy chủ chưa có tài khoản nào → A0 Setup thay cho màn Đăng nhập. */
  setupNeeded: boolean
  setSetupNeeded: (needed: boolean) => void
  /** D15 — số bài học Não hệ thống chờ duyệt (`/api/me`, chỉ quản trị); 0 với người khác. */
  pendingLessons: number
  setPendingLessons: (n: number) => void
}

const SessionContext = createContext<SessionState>({
  checked: false, user: null, session: null,
  setUser: () => {}, refresh: async () => {},
  setupNeeded: false, setSetupNeeded: () => {},
  pendingLessons: 0, setPendingLessons: () => {},
})

export function SessionProvider({ children }: { children: ReactNode }) {
  const [checked, setChecked] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [session, setSession] = useState<Session | null>(null)
  const [setupNeeded, setSetupNeeded] = useState(false)
  const [pendingLessons, setPendingLessons] = useState(0)

  const refresh = async () => {
    try {
      const result = await me()
      setUser(result.user); setSession(result.session)
      setPendingLessons(result.pendingLessons ?? 0)
      setSetupNeeded(false)
    } catch {
      // 401 = chưa có phiên. Hỏi luôn /api/setup: máy chủ chưa có tài khoản nào thì mở A0.
      // Lỗi khác (mất kết nối) đã xử lý qua handler 401 / banner — giữ nguyên trạng thái.
      try { setSetupNeeded((await getSetup()).needed) } catch { /* banner mất kết nối lo phần này */ }
    }
  }
  useEffect(() => {
    void (async () => { await refresh(); setChecked(true) })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const value: SessionState = {
    checked, user, session, setupNeeded, setSetupNeeded, pendingLessons, setPendingLessons,
    setUser: (next, nextSession) => {
      setUser(next); setSession(nextSession === undefined ? null : nextSession)
      // Đăng nhập không đi qua /api/me: quản trị hỏi lại để có số bài học chờ duyệt (D15).
      setPendingLessons(0)
      if (next?.role === 'admin') void me().then((result) => setPendingLessons(result.pendingLessons ?? 0), () => {})
    },
    refresh,
  }
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
}

export const useSession = () => useContext(SessionContext)
