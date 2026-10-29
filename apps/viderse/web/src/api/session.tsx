// Phiên đăng nhập: GET /api/me lúc khởi động, context cho toàn app.
// 401 ở /api/me = chưa có phiên; hỏi tiếp /api/setup — máy chủ chưa có tài khoản
// nào thì `setupNeeded` để App mở màn thiết lập thay cho Đăng nhập.
import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { ApiError, getSetup, me } from './client'
import { onSessionExpired } from './sessionSignal'
import type { User } from './types'

export type SessionState = {
  /** đã gọi xong /api/me lần đầu (dù có hay không có phiên) */
  checked: boolean
  user: User | null
  setUser: (user: User | null) => void
  /** gọi lại /api/me + /api/setup (sau đăng nhập/đăng xuất) */
  refresh: () => Promise<void>
  /** Máy chủ chưa có tài khoản nào → màn thiết lập thay cho Đăng nhập. */
  setupNeeded: boolean
  setSetupNeeded: (needed: boolean) => void
  /** Lần kiểm phiên gần nhất có nhận được phản hồi HTTP nào từ máy chủ không. */
  serverReachable: boolean
}

const SessionContext = createContext<SessionState>({
  checked: false,
  user: null,
  setUser: () => {},
  refresh: async () => {},
  setupNeeded: false,
  setSetupNeeded: () => {},
  serverReachable: true,
})

export function SessionProvider({ children }: { children: ReactNode }) {
  const [checked, setChecked] = useState(false)
  const [user, setUser] = useState<User | null>(null)
  const [setupNeeded, setSetupNeeded] = useState(false)
  const [serverReachable, setServerReachable] = useState(true)

  const refresh = async () => {
    try {
      const result = await me()
      setUser(result.user)
      setSetupNeeded(false)
      setServerReachable(true)
    } catch {
      // Lỗi mạng hay 401 đều xử như nhau ở gate: hỏi /api/setup để chọn
      // màn thiết lập (needed) hay màn đăng nhập (không needed / không gọi được).
      setUser(null)
      try {
        setSetupNeeded((await getSetup()).needed)
        setServerReachable(true)
      } catch (error) {
        // status 0 = fetch không tới được máy chủ; mã HTTP khác vẫn là máy chủ đang trả lời.
        setServerReachable(!(error instanceof ApiError && error.status === 0))
      }
    }
  }

  useEffect(() => {
    void (async () => { await refresh(); setChecked(true) })()
    // 401 ở bất kỳ request nào (kể cả luồng SSE) → hết phiên: xoá user, App
    // trả về màn đăng nhập; việc tháo SSE/poll/cache nằm ở unmount của Root.
    return onSessionExpired(() => setUser(null))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <SessionContext.Provider value={{ checked, user, setUser, refresh, setupNeeded, setSetupNeeded, serverReachable }}>
      {children}
    </SessionContext.Provider>
  )
}

export const useSession = () => useContext(SessionContext)
