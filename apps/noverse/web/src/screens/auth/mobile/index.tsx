import { useEffect, useState } from 'react'
import type { User } from '../../../api/types'
import { useSession } from '../../../api/session'
import { ApiError, accessDeniedUser } from '../../../api/client'
import { Spinner } from '@noverse/ui'
import { useAction } from '../../../data'
import { PairScreen } from '../PairScreen'
import { loginPublic } from '../publicApi'
import './phone.css'
import { deniedAppLinks } from '../../../api/sso'
import { KickedNotice } from '../KickedNotice'

function CompactAuth({ initialUsername, onAuthenticated }: { initialUsername?: string; onAuthenticated?: (user: User) => void }) {
  const { setUser, setDeniedUser, refresh } = useSession()
  const [username, setUsername] = useState(initialUsername ?? '')
  const [password, setPassword] = useState('')
  const [scan, setScan] = useState(false)
  const [retry, setRetry] = useState(0)
  const login = useAction(loginPublic, { error: false })
  useEffect(() => {
    // 429 giữ đếm ngược; 403 app_access_denied → màn chặn giữ phiên, không báo lỗi form.
    const denied = accessDeniedUser(login.error)
    if (denied) setDeniedUser(denied, deniedAppLinks(login.error))
    else if (login.error instanceof ApiError && login.error.status === 429) setRetry(login.error.retryAfter ?? 60)
  }, [login.error])
  useEffect(() => { if (!retry) return; const timer = setTimeout(() => setRetry((left) => left - 1), 1000); return () => clearTimeout(timer) }, [retry])
  if (scan) return <div className="phone-auth"><PairScreen initialView="scan" onAuthenticated={onAuthenticated} /></div>
  return <form className="m-reauth-form" onSubmit={async (event) => {
    event.preventDefault()
    if (login.busy || retry || !username.trim() || password.length < 8) return
    const result = await login.run(username.trim().toLowerCase(), password)
    if (result) { setUser(result.user); onAuthenticated?.(result.user); void refresh() }
  }}>
    <KickedNotice />
    <label>Tên đăng nhập<input className="m-field mono" autoComplete="username" autoCapitalize="none" value={username} onChange={(event) => setUsername(event.target.value)} disabled={login.busy} /></label>
    <label>Mật khẩu<input className="m-field" type="password" autoComplete="current-password" placeholder="Ít nhất 8 ký tự" value={password} onChange={(event) => setPassword(event.target.value)} disabled={login.busy} /></label>
    {Boolean(login.error) && <p className="m-error" role="alert">{login.error instanceof Error ? login.error.message : 'Không đăng nhập được. Vui lòng thử lại.'}{retry > 0 && ` · Thử lại sau ${retry} s`}</p>}
    <button className="m-button" disabled={login.busy || retry > 0 || !username.trim() || password.length < 8} aria-busy={login.pending}>{login.pending && <Spinner size={16} />}{login.pending ? 'Đang đăng nhập…' : 'Đăng nhập'}</button>
    <button type="button" className="m-reauth-qr" onClick={() => setScan(true)} disabled={login.busy}>Quét mã QR từ máy tính</button>
  </form>
}
/** Phone.dc uses the shared real camera/QR and public password transport. */
export function MobileAuth({ code, scan = false, compact = false, initialUsername, onAuthenticated }: { code?: string; scan?: boolean; compact?: boolean; initialUsername?: string; onAuthenticated?: (user: User) => void }) {
  if (compact) return <CompactAuth initialUsername={initialUsername} onAuthenticated={onAuthenticated} />
  return <div className="phone-auth"><PairScreen key={code ?? (scan ? 'scan' : 'login')} code={code} initialView={code ? 'confirm' : scan ? 'scan' : 'login'} onAuthenticated={onAuthenticated} /></div>
}
