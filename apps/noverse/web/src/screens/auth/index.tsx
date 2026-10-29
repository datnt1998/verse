// screens/auth — A1 · Đăng nhập & tài khoản (Login.dc.html) + A2b · xác nhận QR (Phone.dc.html).
// Trạng thái A1: login (chọn tài khoản nhớ trên máy) · password · other · expired;
// lỗi none/wrong/limited (429 + đếm ngược); phiên 30 ngày; “Máy dùng chung”.
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { ApiError, login } from '../../api/client'
import { useSession } from '../../api/session'
import { forgetAccount, getRemembered, rememberAccount } from '../../api/storage'
import type { RememberedAccount, User } from '../../api/types'
import { t } from '../../i18n/vi'
import { navigate } from '../../router'
import { RoleBadge } from '../../shell/AccountMenu'
import { useMediaQuery } from '../../shell/AppShell'
import { Avatar, Button } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import {
  AuthInput, AuthShell, CheckLine, HintLine, OrbHero,
  authDestination, detectDevice, fmtDate, fmtShort, nextRouteLabel,
} from './parts'
import { useAction } from '../../data'
import { RecoveryForm } from './RecoveryForm'

type View = 'login' | 'password' | 'other'

const USERNAME_RE = /^[a-z0-9._-]{2,32}$/
const SESSION_DAYS = 30

export { PairScreen } from './PairScreen'

/** Màn đăng nhập. expired=true → thẻ "Phiên đăng nhập đã hết hạn" (hash ?expired=1). */
export function AuthScreen({ expired = false }: { expired?: boolean }) {
  const { setUser, refresh } = useSession()
  const narrow = useMediaQuery('(max-width: 640px)')
  const remembered = useMemo(() => getRemembered(), [])
  const initialAccount = remembered[0] ?? null
  const [view, setView] = useState<View>(() => {
    if (expired) return initialAccount ? 'password' : 'other'
    return remembered.length > 0 ? 'login' : 'other'
  })
  const [accounts, setAccounts] = useState<RememberedAccount[]>(remembered)
  const [picked, setPicked] = useState<string | null>(initialAccount?.username ?? null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [showPw, setShowPw] = useState(false)
  const [forgot, setForgot] = useState(false)
  const [recoverOpen, setRecoverOpen] = useState(false)
  const [shared, setShared] = useState(false)
  // Lỗi đăng nhập là lỗi theo trường (09 §F1): chỉ hiện dưới ô mật khẩu, không toast trùng (16 §3).
  const loginAction = useAction(login, { error: false })
  const busy = loginAction.busy
  const [errorKind, setErrorKind] = useState<'none' | 'wrong' | 'limited' | 'msg'>('none')
  const [errorMsg, setErrorMsg] = useState('')
  const [retryLeft, setRetryLeft] = useState(0)
  const passRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (retryLeft <= 0) return
    const timer = setTimeout(() => setRetryLeft((n) => Math.max(0, n - 1)), 1000)
    return () => clearTimeout(timer)
  }, [retryLeft])

  useEffect(() => {
    if (errorKind === 'wrong') passRef.current?.focus()
  }, [errorKind])

  const account = accounts.find((a) => a.username === picked) ?? (view === 'password' ? initialAccount : null)
  const isOther = view === 'other'
  const formUsername = isOther ? username.trim().toLowerCase() : (account?.username ?? '')
  const usernameOk = !isOther || USERNAME_RE.test(formUsername)
  const limited = retryLeft > 0
  const disabled = busy || limited || password.length === 0 || !usernameOk
  const expires = useMemo(() => new Date(Date.now() + SESSION_DAYS * 86_400_000), [])
  const h1 = expired && view !== 'login' ? 'Đăng nhập lại'
    : view === 'password' && account ? t.auth.welcomeName(account.name)
    : view === 'other' ? t.auth.login
    : t.auth.welcomeBack
  const sub = view === 'login' ? t.auth.chooseAccount
    : view === 'other' ? t.auth.otherSub
    : t.auth.enterPassword
  const errorText = errorKind === 'limited' ? t.auth.errorLimited
    : errorKind === 'wrong' ? t.auth.errorWrong
    : errorMsg
  const hasError = errorKind !== 'none'

  const pick = (entry: RememberedAccount) => {
    setPicked(entry.username); setPassword(''); setErrorKind('none'); setForgot(false); setView('password')
  }
  const remove = (entry: RememberedAccount) => {
    forgetAccount(entry.username)
    setAccounts(getRemembered())
    if (picked === entry.username) setPicked(null)
  }
  // Vào ứng dụng như đăng nhập thường (cả đăng nhập bằng mật khẩu lẫn đặt lại bằng mã khôi phục).
  const enter = (user: User) => {
    if (shared) forgetAccount(user.username)
    else rememberAccount({
      username: user.username, name: user.name || user.username, role: user.role,
      lastLoginAt: new Date().toISOString(), device: detectDevice(),
    })
    setUser(user)
    void refresh()
    navigate(authDestination(user), true)
  }
  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (disabled) return
    setErrorKind('none'); setErrorMsg('')
    const result = await loginAction.run(formUsername, password)
    if (result) enter(result.user)
  }
  useEffect(() => {
    if (!loginAction.error) return
    const err = loginAction.error instanceof ApiError ? loginAction.error : new ApiError(0, 'Không kết nối được máy chủ. Vui lòng thử lại.')
    if (err.status === 429) { setErrorKind('limited'); setRetryLeft(err.retryAfter ?? 60) }
    else if (err.status === 401) setErrorKind('wrong')
    else { setErrorKind('msg'); setErrorMsg(err.message) }
  }, [loginAction.error])

  const expiredFrom = account?.lastLoginAt ? new Date(account.lastLoginAt) : null
  const expiredTo = expiredFrom ? new Date(expiredFrom.getTime() + SESSION_DAYS * 86_400_000) : null
  const returnTo = nextRouteLabel()

  return (
    <AuthShell>
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, textAlign: 'center' }}>
        <OrbHero />
        <div>
          <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{h1}</h1>
          <p style={{ margin: '6px 0 0', fontSize: 14, color: 'var(--color-fg-3)' }}>{sub}</p>
        </div>
      </div>

      {expired && view !== 'login' && (
        <div role="status" className="card" style={{
          borderRadius: 14, padding: '12px 14px', display: 'flex', gap: 12,
          borderColor: 'rgba(246,203,140,0.3)',
        }}>
          <Icon name="clock" size={18} style={{ color: 'var(--color-warn)', marginTop: 2 }} />
          <div style={{ display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13 }}>
            <span style={{ fontWeight: 500, color: 'var(--color-fg)' }}>{t.auth.expiredTitle}</span>
            <span style={{ color: 'var(--color-fg-3)' }}>
              {account && expiredFrom && expiredTo
                ? t.auth.expiredBody(account.name, fmtShort(account.lastLoginAt), fmtShort(expiredTo.toISOString()))
                : t.auth.expiredBodyAnon}
            </span>
            {returnTo && (
              <span style={{ display: 'flex', alignItems: 'center', gap: 6, color: 'var(--color-fg-4)', fontSize: 12 }}>
                <Icon name="arrow-right-2" size={13} />{t.auth.expiredReturn(returnTo)}
              </span>
            )}
          </div>
        </div>
      )}

      {view === 'login' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: '0 2px' }}>
            <span className="lbl">{t.auth.rememberedLabel}</span>
            <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }} className="num">{accounts.length}</span>
          </div>
          <div className="card" style={{ borderRadius: 14, padding: 6, display: 'flex', flexDirection: 'column', gap: 2 }}>
            {accounts.map((entry) => (
              <div key={entry.username} className="hov-row" style={{ display: 'flex', alignItems: 'center', gap: 4, borderRadius: 10 }}>
                <button type="button" onClick={() => pick(entry)}
                  style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 12, padding: 10, border: 'none', background: 'transparent', color: 'var(--color-fg)', textAlign: 'left', borderRadius: 10 }}>
                  <Avatar name={entry.name || entry.username} size={38} />
                  <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                    <span style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 14, fontWeight: 500 }}>
                      {entry.name || entry.username}<RoleBadge role={entry.role} />
                    </span>
                    <span style={{ fontSize: 12, color: 'var(--color-fg-4)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      <span className="mono">@{entry.username}</span> · lần cuối {fmtShort(entry.lastLoginAt)}{!narrow && <> · {entry.device || detectDevice()}</>}
                    </span>
                  </span>
                  <Icon name="arrow-right-2" size={16} style={{ color: 'var(--color-fg-4)' }} />
                </button>
                <Button variant="icon-only" tooltip="Gỡ khỏi máy này" onClick={() => remove(entry)} aria-label={`Gỡ ${entry.name || entry.username} khỏi máy này`} style={{ width: 30, height: 30, marginRight: 6, borderRadius: 8 }}><Icon name="close-circle-bulk" size={16} /></Button>
              </div>
            ))}
            <button type="button" className="hov-row" onClick={() => { setView('other'); setPicked(null); setPassword(''); setErrorKind('none') }}
              style={{ display: 'flex', alignItems: 'center', gap: 12, padding: 10, border: 'none', borderRadius: 10, background: 'transparent', color: 'var(--color-fg-2)', textAlign: 'left' }}>
              <span aria-hidden="true" style={{
                width: 38, height: 38, borderRadius: '50%', border: '1.5px dashed rgba(255,255,255,0.2)',
                display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-fg-3)',
              }}><Icon name="user-add" size={17} /></span>
              <span style={{ flex: 1, fontSize: 14 }}>{t.auth.otherAccount}</span>
            </button>
          </div>
        </div>
      )}

      {view !== 'login' && (
        <form onSubmit={(event) => void submit(event)} style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          {account && (
            <div className="card" style={{ borderRadius: 14, padding: '10px 10px 10px 12px', display: 'flex', alignItems: 'center', gap: 12 }}>
              <Avatar name={account.name || account.username} size={38} />
              <span style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                <span style={{ fontSize: 14, fontWeight: 500 }}>{account.name || account.username}</span>
                <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}><span className="mono">@{account.username}</span> · {t.role[account.role]}</span>
              </span>
              <button type="button" className="hov-ghost" onClick={() => { setView('login'); setPicked(null); setPassword(''); setErrorKind('none'); setForgot(false) }}
                style={{ height: 30, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg-2)', fontSize: 12 }}>
                Đổi
              </button>
            </div>
          )}

          {isOther && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
              <label htmlFor="lg-user" className="lbl">{t.auth.username}</label>
              <AuthInput id="lg-user" mono type="text" autoComplete="username" autoCapitalize="none" spellCheck={false}
                placeholder="vd: linh" value={username} invalid={hasError && !limited} style={{ height: 40 }}
                onChange={(value) => setUsername(value.trim().toLowerCase())} />
              <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{t.auth.usernameHint}</span>
            </div>
          )}

          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline' }}>
              <label htmlFor="lg-pass" className="lbl">{t.auth.password}</label>
              <button type="button" onClick={() => { setForgot((v) => !v); setRecoverOpen(false) }} aria-expanded={forgot}
                style={{ border: 'none', background: 'transparent', padding: 0, color: 'var(--color-fg-3)', fontSize: 12 }}>
                {t.auth.forgot}
              </button>
            </div>
            <div style={{ position: 'relative' }}>
              <AuthInput id="lg-pass" ref={passRef} type={showPw ? 'text' : 'password'} autoComplete="current-password"
                placeholder={t.auth.passwordHint} value={password} invalid={hasError && !limited}
                aria-describedby={hasError ? 'lg-err' : undefined}
                style={{ height: 40, paddingRight: 44 }} onChange={setPassword} />
              <Button variant="icon-only" tooltip={showPw ? t.auth.hidePassword : t.auth.showPassword} onClick={() => setShowPw((v) => !v)} aria-label={showPw ? t.auth.hidePassword : t.auth.showPassword} style={{ position: 'absolute', right: 4, top: 4, width: 32, height: 32, borderRadius: 8 }}><Icon name={showPw ? 'eye-slash' : 'eye'} size={17} /></Button>
            </div>
            {hasError && (
              <span id="lg-err" role="alert" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-err)' }}>
                <Icon name="warning-2" size={14} />{errorText}
              </span>
            )}
            {limited && (
              <span className="sr-only" aria-live="polite">
                {retryLeft === 60 || retryLeft === 10 ? `${t.auth.errorLimited} Còn ${retryLeft} giây.` : ''}
              </span>
            )}
          </div>

          {forgot && (
            <div style={{ padding: 12, borderRadius: 12, background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', display: 'flex', flexDirection: 'column', gap: 8, fontSize: 13, color: 'var(--color-fg-3)' }}>
              <span>{t.auth.forgotHelp}</span>
              {(isOther || account?.role === 'admin') && (
                <>
                  <div style={{ height: 1, background: 'var(--line-1)' }} />
                  <span style={{ fontSize: 12.5 }}>{t.auth.forgotAdminPre}<span style={{ color: 'var(--color-fg)' }}>{t.auth.forgotAdminCode}</span>{t.auth.forgotAdminPost}</span>
                  {recoverOpen ? (
                    <RecoveryForm username={formUsername} usernameOk={usernameOk} onRecovered={enter} />
                  ) : (
                    <button type="button" className="hov-ghost" onClick={() => setRecoverOpen(true)} style={{
                      alignSelf: 'flex-start', height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.12)',
                      background: 'transparent', color: 'var(--color-fg)', fontSize: 13, display: 'flex', alignItems: 'center', gap: 6,
                    }}>
                      <Icon name="key" size={15} style={{ color: 'var(--color-accent)' }} />{t.auth.useRecovery}
                    </button>
                  )}
                </>
              )}
            </div>
          )}

          <button type="submit" className="hov-primary" aria-busy={loginAction.pending} disabled={disabled} style={{
            height: 42, borderRadius: 11, border: 'none',
            background: disabled && !busy ? 'var(--color-surface-7)' : 'var(--color-fg)',
            color: disabled && !busy ? 'var(--color-fg-4)' : 'var(--color-bg)',
            fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
          }}>
            {loginAction.pending && <Spinner size={14} style={{ borderColor: 'rgba(0,0,0,0.15)', borderTopColor: 'var(--color-bg)' }} />}
            {!loginAction.pending && !limited && <Icon name={loginAction.success ? 'tick-circle-bulk' : 'login'} size={17} />}
            {limited ? `Thử lại sau ${Math.floor(retryLeft / 60)}:${String(retryLeft % 60).padStart(2, '0')}` : loginAction.pending ? t.auth.loggingIn : t.auth.login}
          </button>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 12, color: 'var(--color-fg-3)' }}>
            <HintLine icon="shield-security"><span>{t.auth.sessionNote}<span className="num" style={{ color: 'var(--color-fg-2)' }}>{fmtDate(expires)}</span></span></HintLine>
            <CheckLine checked={shared} onChange={setShared}>{t.auth.sharedDevice}</CheckLine>
          </div>

          {isOther && accounts.length > 0 && (
            <button type="button" onClick={() => { setView('login'); setErrorKind('none') }}
              style={{ alignSelf: 'center', display: 'flex', alignItems: 'center', gap: 6, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 13 }}>
              <Icon name="arrow-left-2" size={15} />Tài khoản đã lưu
            </button>
          )}
        </form>
      )}
    </AuthShell>
  )
}
