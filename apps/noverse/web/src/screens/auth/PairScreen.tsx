// A2b · Điện thoại — xác nhận đăng nhập bằng QR (Phone.dc.html).
// confirm → (POST /api/pair) → pairing → màn chính kèm lời chào; mã hỏng → invalid → login.
// Camera: scan / scanFound / scanWrong / cameraBlocked (Scanner.tsx). Không gọi /api/pair khi vừa tải trang.
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { ApiError, accessDeniedUser } from '../../api/client'
import { useSession } from '../../api/session'
import { rememberAccount } from '../../api/storage'
import type { User } from '../../api/types'
import { t } from '../../i18n/vi'
import { navigate } from '../../router'
import { BrandLockup } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { loginPublic, pairPublic } from './publicApi'
import { AuthInput, OrbHero, authDestination, detectDevice, fmtDate, useServerKind } from './parts'
import { Scanner } from './Scanner'
import { toast } from '@noverse/ui'
import { useAction } from '../../data'
import { deniedAppLinks } from '../../api/sso'
import { KickedNotice } from './KickedNotice'
import { ssoCopy } from '../../i18n/sso'

type View = 'confirm' | 'pairing' | 'invalid' | 'login' | 'scan' | 'cameraBlocked'
const SESSION_DAYS = 30
const USERNAME_RE = /^[a-z0-9._-]{2,32}$/

function PhoneHeader({ chip }: { chip?: ReactNode }) {
  return (
    <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 10, padding: '14px 16px 6px' }}>
      <BrandLockup size={20} gap={8} />
      {chip}
    </header>
  )
}

/** Khung cột 430px: điện thoại đầy màn hình, máy tính vẫn là cột hẹp giữa màn hình. */
function PhoneShell({ children, footer, header }: { children: ReactNode; footer?: ReactNode; header?: ReactNode }) {
  return (
    <div style={{ minHeight: '100vh', height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--color-bg)', color: 'var(--color-fg)', width: '100%', maxWidth: 430, margin: '0 auto' }}>
      {header}
      <main className="scroll" style={{ flex: 1, minHeight: 0, overflowY: 'auto', padding: '22px 16px 16px', display: 'flex', flexDirection: 'column', gap: 22 }}>
        {children}
      </main>
      {footer && (
        <footer style={{ flex: '0 0 auto', padding: '12px 16px 26px', display: 'flex', flexDirection: 'column', gap: 8, background: 'linear-gradient(180deg, rgba(11,11,13,0) 0%, var(--color-bg) 30%)' }}>
          {footer}
        </footer>
      )}
    </div>
  )
}

export function PairScreen({ code = '', initialView, onAuthenticated }: { code?: string; initialView?: 'login' | 'scan' | 'confirm'; onAuthenticated?: (user: User) => void }) {
  const { setUser, setDeniedUser, refresh, sharedAccounts, kicked } = useSession()
  const { label: serverLabel } = useServerKind()
  const [view, setView] = useState<View>(initialView ?? (code ? 'confirm' : 'login'))
  const [activeCode, setActiveCode] = useState(code)
  // Lỗi hiện tại chỗ (thẻ invalid / dòng lỗi dưới form) — không toast trùng (16 §3).
  const pairAction = useAction(pairPublic, { error: false })
  const loginAction = useAction(loginPublic, { error: false })
  const busy = pairAction.busy || loginAction.busy
  const pending = pairAction.pending || loginAction.pending
  const [error, setError] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const expires = useMemo(() => new Date(Date.now() + SESSION_DAYS * 86_400_000), [])

  const complete = useCallback((user: User) => {
    rememberAccount({
      username: user.username, name: user.name || user.username, role: user.role,
      lastLoginAt: new Date().toISOString(), device: detectDevice(),
    })
    setUser(user)
    void refresh()
    if (onAuthenticated) { onAuthenticated(user); return }
    // Phone.dc › home: vào thẳng màn chính, chào bằng thông báo thành công.
    toast(t.pair.hello(user.name || user.username, fmtDate(expires)), 'ok')
    navigate(authDestination(user), true)
  }, [setUser, refresh, onAuthenticated, expires])

  const confirm = async () => {
    setError('')
    const result = await pairAction.run(activeCode)
    if (result) complete(result.user)
  }
  const submitLogin = async () => {
    if (password.length === 0 || !USERNAME_RE.test(username.trim().toLowerCase())) return
    setError('')
    const result = await loginAction.run(username.trim().toLowerCase(), password)
    if (result) complete(result.user)
  }
  useEffect(() => {
    if (!pairAction.error) return
    const failure = pairAction.error instanceof ApiError ? pairAction.error : new ApiError(0, 'Không kết nối được máy chủ. Vui lòng thử lại.')
    // Mã hợp lệ nhưng tài khoản chưa được cấp quyền app → màn chặn, không báo mã hỏng.
    const denied = accessDeniedUser(failure)
    if (denied) { setDeniedUser(denied, deniedAppLinks(failure)); return }
    setError(failure.message); setView('invalid')
  }, [pairAction.error])
  useEffect(() => {
    if (!loginAction.error) return
    const failure = loginAction.error instanceof ApiError ? loginAction.error : new ApiError(0, 'Không kết nối được máy chủ. Vui lòng thử lại.')
    const denied = accessDeniedUser(failure)
    if (denied) { setDeniedUser(denied, deniedAppLinks(failure)); return }
    setError(failure.status === 429 ? `${t.auth.errorLimited} (${failure.retryAfter ?? 60}s)` : failure.message)
  }, [loginAction.error])

  const onBlocked = useCallback(() => setView('cameraBlocked'), [])
  const onCode = useCallback((next: string) => { setActiveCode(next); setView('confirm') }, [])

  if (view === 'scan') {
    return (
      <div style={{ minHeight: '100vh', height: '100vh', display: 'flex', flexDirection: 'column', background: 'var(--color-bg)', width: '100%', maxWidth: 430, margin: '0 auto' }}>
        <Scanner onClose={() => setView('login')} onPassword={() => setView('login')} onBlocked={onBlocked} onCode={onCode} />
      </div>
    )
  }

  if (view === 'cameraBlocked') {
    return (
      <PhoneShell
        header={(
          <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', gap: 10, padding: '14px 14px 6px' }}>
            <button type="button" className="hov-ghost" onClick={() => setView('login')} aria-label="Quay lại"
              style={{ width: 40, height: 40, borderRadius: '50%', border: 'none', background: 'var(--color-surface-1)', color: 'var(--color-fg)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Icon name="arrow-left-2" size={18} />
            </button>
            <span style={{ fontSize: 15, fontWeight: 600 }}>Quét mã đăng nhập</span>
          </header>
        )}
        footer={(
          <>
            <button type="button" className="hov-primary" onClick={() => setView('scan')} style={{
              height: 50, borderRadius: 14, border: 'none', background: 'var(--color-fg)', color: 'var(--color-bg)',
              fontSize: 16, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            }}><Icon name="refresh-2" size={18} />Thử mở camera lại</button>
            <button type="button" className="hov-ghost" onClick={() => setView('login')} style={{
              height: 44, borderRadius: 14, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 15,
            }}>Đăng nhập bằng mật khẩu</button>
          </>
        )}
      >
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 12, textAlign: 'center' }}>
          <span aria-hidden="true" style={{
            width: 64, height: 64, borderRadius: 20, background: 'var(--color-surface-5)',
            display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-warn)',
          }}><Icon name="camera-slash" size={30} /></span>
          <div>
            <h1 style={{ margin: 0, fontSize: 21, fontWeight: 600 }}>Chưa mở được camera ở đây</h1>
            <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--color-fg-3)' }}>
              Không sao — ứng dụng Camera của điện thoại quét được mã này, còn nhanh hơn.
            </p>
          </div>
        </div>
        <div className="card" style={{ borderRadius: 16, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14, fontWeight: 600 }}>
            <Icon name="camera-bulk" size={18} style={{ color: 'var(--color-accent)' }} />Dùng ứng dụng Camera
          </span>
          <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 10, fontSize: 14 }}>
            {['Mở ứng dụng Camera của điện thoại', 'Hướng vào mã QR trên máy tính', 'Chạm vào đường liên kết Noverse hiện ra'].map((step, index) => (
              <li key={step} style={{ display: 'flex', gap: 10 }}>
                <span className="num" style={{
                  width: 22, height: 22, flex: '0 0 auto', borderRadius: '50%', background: 'var(--color-surface-6)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 11, fontWeight: 600,
                }}>{index + 1}</span>
                <span>{step}</span>
              </li>
            ))}
          </ol>
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '12px 14px', borderRadius: 14, border: '1px solid var(--line-2)', fontSize: 12.5, color: 'var(--color-fg-3)' }}>
          <span style={{ fontSize: 13, fontWeight: 500, color: 'var(--color-fg-2)' }}>Vì sao không mở được?</span>
          <span>• Bạn đã chặn quyền camera cho trang này — bật lại trong phần cài đặt trang web của trình duyệt.</span>
          <span>• Trình duyệt chỉ cho dùng camera khi kết nối được bảo mật, nên khi mở Noverse qua Wi-Fi nhà có thể không dùng được.</span>
        </div>
      </PhoneShell>
    )
  }

  const isConfirm = view === 'confirm' || view === 'pairing'
  const isInvalid = view === 'invalid'
  const isLogin = view === 'login'
  const title = isInvalid ? t.pair.invalid : isLogin ? 'Đăng nhập Noverse' : t.pair.title
  const subtitle = isInvalid ? t.auth.errorPair : isLogin ? 'Thư viện truyện trên máy tính của bạn' : t.pair.sub

  return (
    <PhoneShell
      header={<PhoneHeader chip={(
        <span style={{
          display: 'flex', alignItems: 'center', gap: 6, height: 28, padding: '0 10px', borderRadius: 999,
          background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', fontSize: 11.5, color: 'var(--color-fg-3)',
        }}><Icon name={serverLabel === 'Tailscale' ? 'global' : 'wifi'} size={13} />{serverLabel}</span>
      )} />}
      footer={(
        <>
          {isLogin ? (
            <button type="submit" form="phone-login" className="hov-primary" aria-busy={pending} disabled={busy} style={{
              height: 50, borderRadius: 14, border: 'none',
              background: busy ? 'var(--color-surface-7)' : 'var(--color-fg)',
              color: busy ? 'var(--color-fg-3)' : 'var(--color-bg)',
              fontSize: 16, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            }}>
              {pending && <Spinner size={16} />}
              {!pending && <Icon name="login" size={18} />}
              {pending ? t.auth.loggingIn : t.auth.login}
            </button>
          ) : (
            <button type="button" className="hov-primary" aria-busy={pending} onClick={() => { if (isConfirm) void confirm(); else setView('login') }} disabled={busy} style={{
              height: 50, borderRadius: 14, border: 'none',
              background: busy ? 'var(--color-surface-7)' : 'var(--color-fg)',
              color: busy ? 'var(--color-fg-3)' : 'var(--color-bg)',
              fontSize: 16, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
            }}>
              {pending && <Spinner size={16} />}
              {!pending && !isInvalid && <Icon name="login" size={18} />}
              {pending ? t.auth.loggingIn : isInvalid ? t.pair.loginWithPassword : t.pair.login}
            </button>
          )}
          {isConfirm && (
            <button type="button" className="hov-ghost" onClick={() => navigate('/login', true)} style={{
              height: 44, borderRadius: 14, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 15,
            }}>{t.pair.cancel}</button>
          )}
          {isInvalid && (
            <button type="button" className="hov-ghost" onClick={() => navigate('/login', true)} style={{
              height: 44, borderRadius: 14, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 15,
            }}>Đóng</button>
          )}
          <span style={{ textAlign: 'center', fontSize: 12, color: 'var(--color-fg-4)' }}>{t.pair.sessionNote}</span>
        </>
      )}
    >
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 14, textAlign: 'center' }}>
        <OrbHero variant={isInvalid ? 'error' : 'default'} />
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{title}</h1>
          <p style={{ margin: '8px 0 0', fontSize: 14, color: 'var(--color-fg-3)' }}>{subtitle}</p>
          {isLogin && (sharedAccounts || kicked) && <p style={{ margin: '6px 0 0', fontSize: 13, color: 'var(--color-fg-4)' }}>{ssoCopy.subtitle}</p>}
        </div>
      </div>
      {isLogin && <KickedNotice />}

      {isConfirm && (
        <>
          <div className="card" style={{ borderRadius: 16, padding: '4px 14px' }}>
            {[
              { icon: 'book-1', key: t.pair.library, value: 'Noverse', mono: false },
              { icon: 'mobile', key: t.pair.device, value: detectDevice(), mono: false },
              { icon: 'clock', key: t.pair.session, value: `${t.pair.days30} · đến ${fmtDate(expires)}`, mono: true },
            ].map((fact, index) => (
              <div key={fact.key} style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 0', borderTop: index ? '1px solid var(--line-1)' : 'none' }}>
                <Icon name={fact.icon} size={18} style={{ color: 'var(--color-fg-3)' }} />
                <span style={{ flex: 1, fontSize: 13, color: 'var(--color-fg-3)', textAlign: 'left' }}>{fact.key}</span>
                <span className={fact.mono ? 'num' : undefined} style={{ fontSize: 13, color: 'var(--color-fg)', textAlign: 'right' }}>{fact.value}</span>
              </div>
            ))}
          </div>
          <div style={{ display: 'flex', gap: 10, padding: '12px 14px', borderRadius: 14, background: 'rgba(246,203,140,0.06)', border: '1px solid rgba(246,203,140,0.18)', fontSize: 13, color: 'var(--color-fg-2)' }}>
            <Icon name="shield-security" size={16} style={{ color: 'var(--color-warn)', marginTop: 2 }} />
            <span>{t.pair.warning}</span>
          </div>
        </>
      )}

      {isInvalid && (
        <div className="card" style={{ borderRadius: 16, padding: 14, display: 'flex', flexDirection: 'column', gap: 12 }}>
          <span className="lbl">Lấy mã mới trên máy tính</span>
          <ol style={{ margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 10, fontSize: 14 }}>
            {[
              <>Bấm ảnh đại diện → <b style={{ fontWeight: 500 }}>Đăng nhập điện thoại bằng QR</b></>,
              <>Bấm <b style={{ fontWeight: 500 }}>Tạo mã mới</b>, quét lại bằng Camera</>,
            ].map((step, index) => (
              <li key={index} style={{ display: 'flex', gap: 10 }}>
                <span className="num" style={{
                  width: 22, height: 22, flex: '0 0 auto', borderRadius: '50%', background: 'var(--color-surface-6)',
                  display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 11, fontWeight: 600,
                }}>{index + 1}</span>
                <span>{step}</span>
              </li>
            ))}
          </ol>
          <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>Mỗi mã chỉ dùng một lần và hết hạn sau 5 phút.</span>
        </div>
      )}

      {isLogin && (
        <form id="phone-login" onSubmit={(event) => { event.preventDefault(); void submitLogin() }} style={{ display: 'flex', flexDirection: 'column', gap: 18 }}>
          <button type="button" className="card" onClick={() => setView('scan')} style={{
            borderRadius: 16, padding: 14, display: 'flex', gap: 12, alignItems: 'center',
            color: 'var(--color-fg)', textAlign: 'left', borderColor: 'rgba(201,166,245,0.3)',
          }}>
            <span aria-hidden="true" style={{
              width: 44, height: 44, borderRadius: 12, background: 'var(--tint-accent)',
              display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-accent)',
            }}><Icon name="scan-barcode" size={22} /></span>
            <span style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span style={{ fontSize: 14.5, fontWeight: 600 }}>Quét mã QR để đăng nhập</span>
              <span style={{ fontSize: 12.5, color: 'var(--color-fg-3)' }}>Nhanh hơn, không cần gõ mật khẩu</span>
            </span>
            <Icon name="arrow-right-2" size={18} style={{ color: 'var(--color-fg-4)' }} />
          </button>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, fontSize: 12, color: 'var(--color-fg-4)' }}>
            <span style={{ flex: 1, height: 1, background: 'var(--line-2)' }} />hoặc dùng mật khẩu<span style={{ flex: 1, height: 1, background: 'var(--line-2)' }} />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <label htmlFor="m-user" className="lbl">{t.auth.username}</label>
              <AuthInput id="m-user" mono type="text" autoComplete="username" autoCapitalize="none" spellCheck={false}
                placeholder="vd: dat" value={username} style={{ height: 48, borderRadius: 12, fontSize: 16 }}
                onChange={(value) => setUsername(value.trim().toLowerCase())} />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <label htmlFor="m-pass" className="lbl">{t.auth.password}</label>
              <AuthInput id="m-pass" type="password" autoComplete="current-password" placeholder={t.auth.passwordHint}
                value={password} style={{ height: 48, borderRadius: 12, fontSize: 16 }}
                onChange={setPassword} />
            </div>
            {error && (
              <span role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: 'var(--color-err)' }}>
                <Icon name="warning-2" size={14} />{error}
              </span>
            )}
          </div>
        </form>
      )}
    </PhoneShell>
  )
}
