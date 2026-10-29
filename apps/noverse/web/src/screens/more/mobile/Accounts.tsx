import { useEffect, useState } from 'react'
import { useSession } from '../../../api/session'
import type { RememberedAccount, Role } from '../../../api/types'
import { BottomSheet, useMobileToast } from '../../../mobile'
import { beginAccountSwitch, readAccounts, rememberAccount, switchAccount } from '../../../mobile/accounts'
import { navigate } from '../../../router'
import { Icon } from '@noverse/ui'
import { Radio } from '@noverse/ui'
import { relativeTime } from '../../auth/parts'

export const roleLabel: Record<Role, string> = { admin: 'Quản trị', member: 'Thành viên' }
export function AccountAvatar({ name, role, small = false }: { name: string; role: Role; small?: boolean }) {
  return <span className={`mm-avatar mm-avatar-${role}${small ? ' mm-avatar-small' : ''}`}>{name.slice(0, 1).toLocaleUpperCase('vi')}</span>
}
export function RoleBadge({ role }: { role: Role }) {
  return <span className={`mm-role mm-role-${role}`}><Icon name={role === 'admin' ? 'crown' : 'profile-2user'} size={12} />{roleLabel[role]}</span>
}
export function AccountsSheet({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { user, session } = useSession()
  const toast = useMobileToast()
  const [accounts, setAccounts] = useState(readAccounts)
  const [target, setTarget] = useState<RememberedAccount | null>(null)
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)
  useEffect(() => { if (open) { if (user) rememberAccount(user); setAccounts(readAccounts()); setTarget(null); setPassword(''); setError('') } }, [open, user])
  const close = () => { if (!pending) { setPassword(''); onClose() } }
  const qr = async () => {
    if (pending) return
    setPending(true); setError('')
    try { await beginAccountSwitch(); setPassword(''); onClose(); navigate('/phone') }
    catch (failure) { setError(failure instanceof Error ? failure.message : 'Chưa đăng xuất được tài khoản đang dùng. Vui lòng thử lại.') }
    finally { setPending(false) }
  }
  const submit = async () => {
    if (!target || !password || pending || !navigator.onLine) return
    const previous = user?.name || user?.username || 'Tài khoản trước'
    setPending(true); setError('')
    try { const result = await switchAccount({ username: target.username, password }); setPassword(''); onClose(); toast(`Đã đăng nhập ${result.user.name || result.user.username} · ${previous} đã đăng xuất`) }
    catch (failure) { setError(failure instanceof Error ? failure.message : 'Chưa đăng nhập được. Vui lòng thử lại.') }
    finally { setPending(false) }
  }
  return <BottomSheet open={open} onClose={close} height="auto" label={target ? `Đăng nhập ${target.name}` : 'Tài khoản đã dùng trên máy này'}><div className="mm-accounts">
    <div className="mm-accounts-head"><h2>{target ? `Đăng nhập ${target.name}` : 'Tài khoản đã dùng trên máy này'}</h2><p className="mm-accounts-lead">{target ? `Nhập mật khẩu hoặc quét mã QR trên máy tính của ${target.name}.` : 'Chạm để đăng nhập lại. Máy không lưu mật khẩu.'}</p></div>
    {!target && error && <p className="mm-field-error" role="alert">{error}</p>}
    {!target && pending && <p className="mm-accounts-note" role="status">Đang chuyển sang đăng nhập tài khoản khác…</p>}
    {!target ? <><div className="mm-group" role="radiogroup" aria-label="Chọn tài khoản">{accounts.map((entry) => { const current = entry.username === user?.username; const remaining = session?.expiresAt ? Math.max(0, Math.ceil((Date.parse(session.expiresAt) - Date.now()) / 86400000)) : null; return <Radio className="mm-row mm-account-row" key={entry.username} mark="tick" markPosition="end" checked={current} onChange={() => { if (current) close(); else { setTarget(entry); setError('') } }} leading={<AccountAvatar name={entry.name || entry.username} role={entry.role} small />} trailing={!current ? <span className="mm-login-label">Đăng nhập</span> : undefined} style={{ padding: '8px 14px', margin: 0, gap: 12, minHeight: 64, fontSize: 15 }}><span className="mm-copy"><span style={{ fontSize: 15.5, fontWeight: 500 }}>{entry.name} <span className="mm-sub" style={{ fontSize: 12.5 }}>· {roleLabel[entry.role]}</span></span><span className={`mm-sub${current ? ' mm-current' : ''}`} style={{ fontSize: 12.5 }}>{current ? `Đang dùng${remaining !== null ? ` · phiên còn ${remaining} ngày` : ''}` : `Lần cuối ${relativeTime(entry.lastLoginAt)}`}</span></span></Radio> })}<button className="mm-row mm-account-row" type="button" onClick={qr}><span className="mm-add-account"><Icon name="user-add" size={19} /></span><span className="mm-fill">Thêm tài khoản</span><span className="mm-sub">Quét QR hoặc mật khẩu</span></button></div><p className="mm-accounts-note">Máy chỉ giữ một phiên. Chuyển tài khoản sẽ đăng xuất tài khoản đang dùng; truyện và việc nền vẫn chạy trên máy chủ.</p></> : <form className="mm-account-login" onSubmit={(event) => { event.preventDefault(); void submit() }}><div className="mm-account-target"><AccountAvatar name={target.name} role={target.role} small /><span className="mm-copy"><span>{target.name}</span><span className="mm-sub mm-mono">{target.username}</span></span><button type="button" disabled={pending} onClick={() => { setTarget(null); setPassword(''); setError('') }}>Đổi</button></div><label className="mm-field-block">Mật khẩu<input className="mm-field" type="password" autoComplete="current-password" value={password} disabled={pending} onChange={(event) => setPassword(event.target.value)} /></label>{error && <p className="mm-field-error" role="alert">{error}</p>}<div className="mm-account-actions"><button type="button" className="mm-secondary" disabled={pending} onClick={qr}><Icon name="scan-barcode" size={17} />Quét QR</button><button type="submit" className="mm-primary" disabled={!password || pending || !navigator.onLine} aria-busy={pending}>{pending ? 'Đang đăng nhập…' : 'Đăng nhập'}</button></div><p className="mm-accounts-note">Đăng nhập xong, {user?.name || user?.username} được đăng xuất khỏi máy này.</p></form>}
  </div></BottomSheet>
}
