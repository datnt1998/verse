// A0 · Lần chạy đầu (#/setup) — Setup.dc.html, GAP-41 (09-auth-onboard §11).
// Mã thiết lập chỉ in ở máy chủ; màn này không bao giờ nhận hay hiện lại mã.
// 8 trạng thái: code · codeWrong · codeLocked · codeExpired · account · creating · tokenExpired · taken.
import { Fragment, useCallback, useEffect, useRef, useState, type CSSProperties, type FormEvent, type InputHTMLAttributes } from 'react'
import { ApiError, createSetupAdmin, getSetup, verifySetup } from '../../../api/client'
import { useSession } from '../../../api/session'
import { t } from '../../../i18n/vi'
import { navigate } from '../../../router'
import { BrandLockup } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { OrbHero, StrengthMeter } from '../parts'
import { markFromSetup, markSetupRecovery } from '../../onboard/model'
import {
  ATTEMPT_LIMIT, CODE_LENGTH, adminFailure, canCreateAdmin, clockOf, codeBoxes, formatCode, lockClock, minutesLeft,
  normalizeCode, setupStepIndex, usernameSuggestion, usernameValid, verifyFailure,
  type SetupUiState,
} from './model'
import './setup.css'

const fieldStyle: CSSProperties = {
  height: 42, padding: '0 12px', borderRadius: 10, background: 'var(--color-surface-0)',
  border: '1px solid var(--line-4)', color: 'var(--color-fg)', fontSize: 14, width: '100%', outline: 'none',
  transition: 'border-color .2s', boxSizing: 'border-box',
}
const primaryStyle = (disabled: boolean): CSSProperties => ({
  height: 42, borderRadius: 11, border: 'none',
  background: disabled ? 'var(--line-3)' : 'var(--color-fg)',
  color: disabled ? 'var(--color-fg-4)' : 'var(--color-bg)',
  fontSize: 14, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
})

/** 3 bước của A0 (Setup.dc.html › steps): mã → tài khoản → luồng thiết lập 7 bước của A3. */
function StepBar({ index }: { index: number }) {
  return (
    <ol aria-label={t.setup.stepsAria} style={{ listStyle: 'none', margin: 0, padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10, flexWrap: 'wrap' }}>
      {t.setup.steps.map((label, i) => {
        const done = i < index
        const current = i === index
        return (
          <li key={label} aria-current={current ? 'step' : undefined} style={{
            display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, whiteSpace: 'nowrap',
            color: current ? 'var(--color-fg)' : done ? 'var(--color-ok)' : 'var(--color-fg-4)',
          }}>
            <i aria-hidden="true" style={{
              width: 20, height: 20, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontStyle: 'normal', fontSize: 11, fontWeight: 600,
              background: current ? 'var(--color-fg)' : done ? 'var(--kit-ok-12)' : 'transparent',
              color: current ? 'var(--color-bg)' : done ? 'var(--color-ok)' : 'var(--color-fg-4)',
              border: `1px solid ${current ? 'var(--color-fg)' : done ? 'var(--kit-ok-30)' : 'var(--kit-wash-14)'}`,
            }}>{done ? '✓' : i + 1}</i>
            {label}
            {i < 2 && <span aria-hidden="true" style={{ width: 22, height: 1, background: 'var(--kit-wash-12)', marginLeft: 2 }} />}
          </li>
        )
      })}
    </ol>
  )
}

/** Ô nhập 42px của artboard (`.fld`) — placeholder/focus nằm trong setup.css. */
function Field({ id, label, mono, style, ...rest }: {
  id: string; label: string; mono?: boolean; style?: CSSProperties
} & Omit<InputHTMLAttributes<HTMLInputElement>, 'id' | 'style' | 'className'>) {
  return (
    <label htmlFor={id} style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <span className="lbl">{label}</span>
      <input id={id} className={`setup-field${mono ? ' mono' : ''}`} style={{ ...fieldStyle, ...style }} {...rest} />
    </label>
  )
}

export function SetupScreen() {
  const { user, setUser, refresh, setSetupNeeded } = useSession()
  const [ui, setUi] = useState<SetupUiState>('code')
  const [code, setCode] = useState('')
  const [attemptsLeft, setAttemptsLeft] = useState(ATTEMPT_LIMIT)
  const [lockedUntil, setLockedUntil] = useState('')
  const [token, setToken] = useState('')
  const [tokenExpiresAt, setTokenExpiresAt] = useState('')
  const [takenAt, setTakenAt] = useState('')
  const [version, setVersion] = useState('')
  const [name, setName] = useState(user?.name ?? '')
  const [username, setUsername] = useState(user ? usernameSuggestion(user.name || user.username) : '')
  const [autoUsername, setAutoUsername] = useState(true)
  const [password, setPassword] = useState('')
  const [repeat, setRepeat] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [newCode, setNewCode] = useState('')
  const [checking, setChecking] = useState<'code' | 'newCode' | null>(null)
  const [shake, setShake] = useState(false)
  const [error, setError] = useState('')
  const [fieldError, setFieldError] = useState<{ field: 'name' | 'username' | 'password'; message: string } | null>(null)
  const booted = useRef(false)
  const tried = useRef('')
  const inputRef = useRef<HTMLInputElement>(null)

  // Máy chủ đã có tài khoản (trang tự đóng) → về Đăng nhập; còn khoá/hết hạn thì mở đúng trạng thái.
  useEffect(() => {
    if (booted.current) return
    booted.current = true
    void (async () => {
      try {
        const state = await getSetup()
        setAttemptsLeft(state.attemptsLeft)
        setVersion(state.version ?? '')
        if (!state.needed) {
          setSetupNeeded(false)
          navigate(user ? '/' : '/login', true)
          return
        }
        // Hết hạn thắng khoá (Câu 1): khởi động lại giải quyết cả hai, nên hiện codeExpired trước.
        if (state.expired === true) { setUi('codeExpired'); return }
        if (state.lockedUntil && new Date(state.lockedUntil).getTime() > Date.now()) { setLockedUntil(state.lockedUntil); setUi('codeLocked'); return }
        setUi('code')
      } catch { setUi('code') }
    })()
  }, [setSetupNeeded, user])

  useEffect(() => {
    if (ui === 'code' || ui === 'codeWrong') inputRef.current?.focus()
  }, [ui])

  useEffect(() => {
    if (!shake) return
    const timer = window.setTimeout(() => setShake(false), 460)
    return () => window.clearTimeout(timer)
  }, [shake])

  const verify = useCallback(async (raw: string, mode: 'code' | 'newCode') => {
    setChecking(mode); setError('')
    try {
      const result = await verifySetup(formatCode(raw))
      setToken(result.setupToken); setTokenExpiresAt(result.expiresAt)
      setAttemptsLeft(ATTEMPT_LIMIT); setLockedUntil('')
      setCode(''); setNewCode('')
      setUi('account')
    } catch (cause) {
      const failure = cause instanceof ApiError ? verifyFailure(cause.status, cause.payload) : null
      if (!failure) { setError(cause instanceof Error ? cause.message : t.setup.offline); return }
      if (failure.state === 'codeWrong') {
        setAttemptsLeft(failure.attemptsLeft)
        if (mode === 'newCode') setError(t.setup.wrongBody(failure.attemptsLeft))
        else { setUi('codeWrong'); setCode(raw); setShake(true) }
      } else if (failure.state === 'codeFormat') {
        // Mã sai định dạng: máy chủ không trừ lượt (Câu 1) → giữ nguyên `attemptsLeft`, chỉ báo sai.
        // Không xoá `tried`: cùng một mã không tự gửi lại (sửa mã thì `changeCode` mở khoá).
        if (mode === 'newCode') setError(t.setup.wrongBody(attemptsLeft))
        else { setUi('codeWrong'); setCode(raw); setShake(true) }
      } else if (failure.state === 'codeLocked') { setLockedUntil(failure.lockedUntil); setUi('codeLocked') }
      else if (failure.state === 'codeExpired') setUi('codeExpired')
      else { setTakenAt(failure.at); setUi('taken'); setSetupNeeded(false) }
    } finally { setChecking(null) }
  }, [attemptsLeft, setSetupNeeded])

  // Đủ 8 ký tự thì tự kiểm (Setup.dc.html › submitCode, 09 §11); cùng một mã chỉ thử một lần.
  useEffect(() => {
    if (ui !== 'code' && ui !== 'codeWrong') return
    if (checking) return
    const normalized = normalizeCode(code)
    if (normalized.length !== CODE_LENGTH || tried.current === normalized) return
    tried.current = normalized
    void verify(normalized, 'code')
  }, [code, ui, checking, verify])

  const changeCode = (raw: string) => {
    const next = formatCode(raw)
    if (normalizeCode(next) !== tried.current) tried.current = ''
    setCode(next)
    if (ui === 'codeWrong') setUi('code')
    setError('')
  }

  const create = async (event?: FormEvent) => {
    event?.preventDefault()
    if (ui === 'creating' || ui === 'tokenExpired') return
    if (!canCreateAdmin({ name, username, password, repeat })) return
    setUi('creating'); setError(''); setFieldError(null)
    try {
      const result = await createSetupAdmin({ setupToken: token, name: name.trim(), username, password })
      setSetupNeeded(false)
      markFromSetup()
      markSetupRecovery(result.recoveryCode)
      setUser(result.user)
      await refresh()
      navigate('/welcome/profile', true)
    } catch (cause) {
      const failure = cause instanceof ApiError ? adminFailure(cause.status, cause.payload, cause.message) : null
      if (failure?.state === 'taken') { setTakenAt(failure.at); setUi('taken'); setSetupNeeded(false) }
      else if (failure?.state === 'tokenExpired') setUi('tokenExpired')
      else if (failure?.state === 'field') { setFieldError({ field: failure.field, message: failure.message }); setUi('account') }
      else { setUi('account'); setError(cause instanceof Error ? cause.message : t.setup.offline) }
    }
  }

  const stepIndex = setupStepIndex(ui)
  const normalized = normalizeCode(code)
  const locked = ui === 'codeLocked' || ui === 'codeExpired'
  const codeDisabled = Boolean(checking) || locked || normalized.length < CODE_LENGTH
  const creating = ui === 'creating'
  const tokenExpired = ui === 'tokenExpired'
  const accountDisabled = creating || tokenExpired || !canCreateAdmin({ name, username, password, repeat })
  const mismatch = Boolean(repeat) && repeat !== password
  const isCode = stepIndex === 0
  const message = ui === 'codeWrong'
    ? { icon: 'warning-2', fg: 'var(--kit-err-soft)', bg: 'var(--kit-errf-6)', bd: 'var(--kit-errf-25)', title: t.setup.wrongTitle, text: t.setup.wrongBody(attemptsLeft) }
    : ui === 'codeLocked'
      ? {
        icon: 'lock', fg: 'var(--kit-err-soft)', bg: 'var(--kit-errf-6)', bd: 'var(--kit-errf-25)', title: t.setup.lockedTitle,
        text: t.setup.lockedBody(lockClock(lockedUntil)),
      }
      : ui === 'codeExpired'
        ? { icon: 'clock', fg: 'var(--kit-warn-text)', bg: 'var(--kit-warn-6)', bd: 'var(--kit-warn-25)', title: t.setup.expiredTitle, text: t.setup.expiredBody }
        : null
  const userHint = !username ? t.setup.usernameEmpty : usernameValid(username) ? t.setup.usernameOk(username) : t.setup.usernameBad
  const fieldMessage = (field: 'name' | 'username' | 'password') => fieldError?.field === field
    ? <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)', marginTop: -6 }}>{fieldError.message}</span>
    : null
  const title = ui === 'taken' ? t.setup.titleTaken : isCode ? t.setup.titleCode : t.setup.titleAccount
  const sub = ui === 'taken' ? t.setup.subTaken : isCode ? t.setup.subCode : t.setup.subAccount

  return (
    <div style={{
      position: 'relative', minHeight: '100vh', height: '100vh', overflow: 'hidden',
      background: 'var(--kit-glow-page), var(--color-bg)', color: 'var(--color-fg)',
      fontSize: 14, lineHeight: 1.55, display: 'flex', flexDirection: 'column',
    }}>
      <header style={{ flex: '0 0 auto', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, padding: '14px 20px' }}>
        <BrandLockup size={22} studio />
        <div style={{
          display: 'flex', alignItems: 'center', gap: 8, height: 30, padding: '0 12px', borderRadius: 999,
          background: 'var(--color-surface-1)', border: '1px solid var(--line-2)', fontSize: 12, color: 'var(--color-fg-3)',
          whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>
          <span aria-hidden="true" style={{ width: 7, height: 7, borderRadius: '50%', background: 'var(--color-warn)', boxShadow: '0 0 0 3px var(--kit-warn-14)', flex: '0 0 auto' }} />
          {t.setup.headerChip}
        </div>
      </header>

      <main className="scroll" style={{ flex: 1, minHeight: 0, overflowY: 'auto', display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}>
        <div style={{ width: 'min(440px, 100%)', display: 'flex', flexDirection: 'column', gap: 22 }}>
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16, textAlign: 'center' }}>
            <OrbHero />
            <div>
              <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.015em' }}>{title}</h1>
              <p style={{ margin: '6px 0 0', fontSize: 14, color: 'var(--color-fg-3)' }}>{sub}</p>
            </div>
          </div>

          <StepBar index={stepIndex} />

          {/* ===== BƯỚC 1 · MÃ THIẾT LẬP ===== */}
          {isCode && (
            <form onSubmit={(event) => { event.preventDefault(); if (!codeDisabled) void verify(normalized, 'code') }} className="card"
              style={{ borderRadius: 18, padding: 20, display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <label htmlFor="setup-code" className="lbl">{t.setup.codeLabel}</label>
                <span style={{ fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>{t.setup.codeHelp}</span>
              </div>
              <div className={shake ? 'setup-shake' : undefined} style={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 6 }}>
                {codeBoxes(code, { bad: ui === 'codeWrong', checking: Boolean(checking) }).map((box, index) => (
                  <Fragment key={index}>
                    <span aria-hidden="true" style={{
                      width: 42, height: 50, borderRadius: 10, background: box.cls === 'bad' ? 'var(--kit-errf-5)' : 'var(--color-surface-0)',
                      border: `1px solid ${box.cls === 'bad' ? 'var(--kit-errf-60)' : box.cls === 'cur' ? 'var(--kit-acc-70)' : box.cls === 'ok' ? 'var(--kit-ok-50)' : 'var(--kit-wash-12)'}`,
                      boxShadow: box.cls === 'cur' ? 'var(--kit-ring-acc)' : undefined,
                      display: 'flex', alignItems: 'center', justifyContent: 'center',
                      fontFamily: 'var(--font-mono)', fontSize: 20, fontWeight: 500, color: 'var(--color-fg)',
                      transition: 'border-color .2s, background .2s',
                    }}>{box.ch}</span>
                    {box.dash && <span aria-hidden="true" style={{ width: 10, height: 2, borderRadius: 2, background: 'var(--kit-dash)' }} />}
                  </Fragment>
                ))}
                <input
                  ref={inputRef} id="setup-code" type="text" inputMode="text" autoComplete="one-time-code" autoCapitalize="characters"
                  spellCheck={false} maxLength={9} value={code} onChange={(event) => changeCode(event.target.value)}
                  aria-label={t.setup.codeAria} aria-describedby={message ? 'setup-msg' : undefined}
                  style={{
                    position: 'absolute', inset: 0, width: '100%', height: '100%', opacity: 0, border: 'none',
                    background: 'transparent', color: 'transparent', caretColor: 'transparent', cursor: 'text', fontSize: 16,
                  }}
                />
              </div>
              {message && (
                <div id="setup-msg" role="alert" aria-live="polite" style={{
                  display: 'flex', gap: 10, alignItems: 'flex-start', padding: '10px 12px', borderRadius: 12,
                  background: message.bg, border: `1px solid ${message.bd}`, fontSize: 13, lineHeight: 1.5,
                }}>
                  <Icon name={message.icon} size={16} style={{ marginTop: 2, color: message.fg }} />
                  <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    <span style={{ color: message.fg, fontWeight: 500 }}>{message.title}</span>
                    <span style={{ color: 'var(--color-fg-3)' }}>{message.text}</span>
                  </span>
                </div>
              )}
              <button type="submit" className={codeDisabled ? undefined : 'hov-primary'} disabled={codeDisabled} style={primaryStyle(codeDisabled)}>
                {checking === 'code' && <Spinner size={14} color="var(--color-bg)" style={{ borderColor: 'var(--kit-spin-dark)', borderTopColor: 'var(--color-bg)' }} />}
                {checking === 'code' ? t.setup.checking : locked ? t.setup.needNewCode : t.setup.continue}
              </button>
              <details style={{ fontSize: 13, color: 'var(--color-fg-3)' }}>
                <summary style={{ cursor: 'pointer', color: 'var(--color-fg-2)' }}>{t.setup.whereTitle}</summary>
                <ul style={{ margin: '8px 0 0', paddingLeft: 18, display: 'flex', flexDirection: 'column', gap: 4, lineHeight: 1.55 }}>
                  {t.setup.where.map((line) => <li key={line}>{line}</li>)}
                </ul>
              </details>
              {error && <span role="alert" style={{ fontSize: 12.5, color: 'var(--color-err)' }}>{error}</span>}
            </form>
          )}

          {/* ===== BƯỚC 2 · TÀI KHOẢN QUẢN TRỊ ===== */}
          {!isCode && ui !== 'taken' && (
            <form onSubmit={(event) => void create(event)} className="card"
              style={{ borderRadius: 18, padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
              {!tokenExpired && (
                <div style={{
                  display: 'flex', gap: 10, alignItems: 'flex-start', padding: '10px 12px', borderRadius: 12,
                  background: 'var(--kit-ok-6)', border: '1px solid var(--kit-ok-16)', fontSize: 13,
                }}>
                  <Icon name="tick-circle-bulk" size={16} style={{ marginTop: 2, color: 'var(--color-ok)' }} />
                  <span style={{ color: 'var(--color-fg-2)' }}>
                    {t.setup.codeOkPre}<span className="num">{minutesLeft(tokenExpiresAt)} phút</span>{t.setup.codeOkPost}
                  </span>
                </div>
              )}
              {tokenExpired && (
                <div role="alert" style={{
                  display: 'flex', flexDirection: 'column', gap: 10, padding: 12, borderRadius: 12,
                  background: 'var(--kit-warn-6)', border: '1px solid var(--kit-warn-28)', fontSize: 13,
                }}>
                  <div style={{ display: 'flex', gap: 10, alignItems: 'flex-start' }}>
                    <Icon name="clock" size={16} style={{ marginTop: 2, color: 'var(--kit-warn-text)' }} />
                    <span style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                      <b style={{ fontWeight: 600, color: 'var(--kit-warn-text)' }}>{t.setup.tokenExpiredTitle}</b>
                      <span style={{ color: 'var(--color-fg-2)', lineHeight: 1.5 }}>{t.setup.tokenExpiredBody}</span>
                    </span>
                  </div>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <input
                      id="setup-newcode" aria-label={t.setup.newCodeAria} placeholder="XXXX-XXXX" autoComplete="one-time-code"
                      className="mono" value={newCode} onChange={(event) => { setNewCode(formatCode(event.target.value)); setError('') }}
                      onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); if (normalizeCode(newCode).length === CODE_LENGTH) void verify(newCode, 'newCode') } }}
                      style={{
                        flex: 1, minWidth: 0, height: 38, padding: '0 12px', borderRadius: 10, background: 'var(--color-surface-0)',
                        border: '1px solid var(--kit-wash-12)', color: 'var(--color-fg)', fontSize: 15, letterSpacing: '.14em',
                        textTransform: 'uppercase', boxSizing: 'border-box',
                      }}
                    />
                    <button type="button" onClick={() => void verify(newCode, 'newCode')}
                      disabled={normalizeCode(newCode).length < CODE_LENGTH || checking === 'newCode'}
                      style={{
                        height: 38, padding: '0 14px', borderRadius: 10, border: '1px solid var(--kit-wash-16)', background: 'transparent',
                        color: normalizeCode(newCode).length < CODE_LENGTH ? 'var(--kit-fg-faint)' : 'var(--color-fg)',
                        fontSize: 13, fontWeight: 500, display: 'flex', alignItems: 'center', gap: 6,
                      }}>
                      {checking === 'newCode' && <Spinner size={13} />}{t.setup.checkCode}
                    </button>
                  </div>
                  {error && <span role="alert" style={{ fontSize: 12.5, color: 'var(--color-err)' }}>{error}</span>}
                </div>
              )}

              <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', gap: 12 }}>
                <Field id="setup-name" label={t.setup.name} value={name} autoComplete="name" placeholder={t.setup.namePlaceholder}
                  onChange={(event) => { const value = event.target.value; setName(value); if (autoUsername) setUsername(usernameSuggestion(value)) }} />
                <label htmlFor="setup-username" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  <span className="lbl">{t.setup.username}</span>
                  <span style={{ position: 'relative', display: 'block' }}>
                    <span aria-hidden="true" style={{ position: 'absolute', left: 12, top: 11, color: 'var(--kit-fg-off)' }}>@</span>
                    <input id="setup-username" className="setup-field mono" type="text" autoComplete="username" spellCheck={false}
                      value={username} aria-describedby="setup-username-hint"
                      onChange={(event) => { setAutoUsername(false); setUsername(event.target.value.toLowerCase()) }}
                      style={{ ...fieldStyle, paddingLeft: 28 }} />
                  </span>
                </label>
              </div>
              {fieldMessage('name')}
              <span id="setup-username-hint" style={{ fontSize: 12, marginTop: -6, color: username && !usernameValid(username) ? 'var(--color-err)' : 'var(--color-fg-4)' }}>{userHint}</span>
              {fieldMessage('username')}

              <label htmlFor="setup-password" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                <span className="lbl">{t.setup.password}</span>
                <span style={{ position: 'relative', display: 'block' }}>
                  <input id="setup-password" className="setup-field" type={showPassword ? 'text' : 'password'} autoComplete="new-password"
                    value={password} onChange={(event) => setPassword(event.target.value)} aria-describedby="setup-password-strength"
                    style={{ ...fieldStyle, paddingRight: 44 }} />
                  <button type="button" className="hov-ghost" onClick={() => setShowPassword((value) => !value)}
                    aria-label={showPassword ? t.auth.hidePassword : t.auth.showPassword}
                    style={{
                      position: 'absolute', right: 6, top: 6, width: 30, height: 30, borderRadius: 8, border: 'none',
                      background: 'transparent', color: 'var(--color-fg-3)', display: 'flex', alignItems: 'center', justifyContent: 'center',
                    }}>
                    <Icon name={showPassword ? 'eye-slash' : 'eye'} size={16} />
                  </button>
                </span>
              </label>
              <div id="setup-password-strength" style={{ display: 'flex', flexDirection: 'column', gap: 6, marginTop: -4 }}>
                <StrengthMeter value={password} />
              </div>
              {fieldMessage('password')}

              <Field id="setup-repeat" label={t.setup.repeatPassword} type={showPassword ? 'text' : 'password'} autoComplete="new-password"
                value={repeat} onChange={(event) => setRepeat(event.target.value)}
                style={{ borderColor: mismatch ? 'var(--kit-errf-55)' : 'var(--line-4)' }} />
              {mismatch && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)', marginTop: -6 }}>{t.setup.mismatch}</span>}

              <div style={{
                display: 'flex', gap: 10, alignItems: 'flex-start', padding: '10px 12px', borderRadius: 12,
                background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', fontSize: 12.5,
                color: 'var(--color-fg-3)', lineHeight: 1.5,
              }}>
                <Icon name="crown" size={15} style={{ marginTop: 2, color: 'var(--color-accent)' }} />
                <span>{t.setup.adminNote}</span>
              </div>

              <button type="submit" className={accountDisabled ? undefined : 'hov-primary'} disabled={accountDisabled} style={primaryStyle(accountDisabled)}>
                {creating && <Spinner size={14} color="var(--color-bg)" style={{ borderColor: 'var(--kit-spin-dark)', borderTopColor: 'var(--color-bg)' }} />}
                {creating ? t.setup.creating : tokenExpired ? t.setup.createNeedsCode : t.setup.create}
              </button>
              {error && !tokenExpired && <span role="alert" style={{ fontSize: 12.5, color: 'var(--color-err)' }}>{error}</span>}
            </form>
          )}

          {/* ===== ĐÃ CÓ QUẢN TRỊ ===== */}
          {ui === 'taken' && (
            <div role="alert" className="card" style={{ borderRadius: 18, padding: 20, display: 'flex', flexDirection: 'column', gap: 14 }}>
              <div style={{ display: 'flex', gap: 12, alignItems: 'flex-start' }}>
                <span style={{
                  width: 38, height: 38, borderRadius: 11, background: 'var(--tint-info)', display: 'flex',
                  alignItems: 'center', justifyContent: 'center', flex: 'none',
                }}>
                  <Icon name="shield-tick" size={19} style={{ color: 'var(--color-info)' }} />
                </span>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                  <span style={{ fontSize: 15, fontWeight: 600 }}>{t.setup.takenTitle}</span>
                  <span style={{ fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>
                    {t.setup.takenBody(clockOf(takenAt) || '—')}
                  </span>
                </div>
              </div>
              <a href="#/login" className="hov-primary" onClick={() => setSetupNeeded(false)} style={{
                height: 42, borderRadius: 11, background: 'var(--color-fg)', color: 'var(--color-bg)', fontSize: 14,
                fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
              }}>
                {t.setup.toLogin}<Icon name="arrow-right-2" size={15} />
              </a>
            </div>
          )}

          {ui !== 'taken' && (
            <p style={{ margin: 0, textAlign: 'center', fontSize: 12, color: 'var(--kit-fg-faint)' }}>
              {`${t.setup.foot}${version ? ` · Noverse ${version}` : ''}`}
            </p>
          )}
        </div>
      </main>
    </div>
  )
}
