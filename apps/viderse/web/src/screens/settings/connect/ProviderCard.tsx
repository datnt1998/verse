import * as stylex from '@stylexjs/stylex'
import { styles as settingsStyles, connBarMarker, connErrTextMarker, connManualRowMarker, connMBarMarker, connMActsMarker } from '../settings.stylex'
// ProviderCard — một thẻ nhà cung cấp trong Kết nối AI (artboard Settings
// conn=list|added|models|manual|failed + grok=signed|code|out; MSettings
// screen=connect). Wire: internal/web/ai.go
//   • hàng đầu: avatar chữ, tên + pill loại kết nối + pill năng lực, dòng phụ
//     (đăng nhập / khoá / địa chỉ, số model, lúc kiểm tra), cụm nút bên phải.
//   • hạn mức ảnh (quota) của thẻ đăng nhập gói.
//   • vùng nhập mã (Grok) hoặc chờ trình duyệt (ChatGPT) khi đang đăng nhập.
//   • ghi chú "đã kết nối, chọn model" (conn=added) và hộp lỗi cam (conn=failed).
//   • danh mục model inline (máy tính) — điện thoại mở sheet.
import { useEffect, useRef, useState } from 'react'
import { Button, Field, Icon, PasswordField, Spinner, confirm, toast } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { confirmProviderLogout } from '../../../components'
import { ApiError } from '../../../api/client'
import {
  cancelProviderLogin, deleteProvider, getProviderLogin, logoutProvider,
  startProviderLogin, submitProviderLoginCode, testProvider, updateProvider,
  type AiLogin, type AiModel, type AiProvider,
} from '../../../api/ai'
import { keys } from '../../../data/keys'
import { aiCopy, capsLine, capsPhrase, clockOf } from '../../../i18n/ai'
import { t } from '../../../i18n/settings'
import { fmtTime } from '../fmt'
import { sequentialPoll } from '../poll'
import { ModelCatalog } from './ModelCatalog'

const c = aiCopy.connect

/** Trường chuỗi trong payload lỗi (reason/field/need/agent). */
export function fieldOf(error: unknown, name: string): string | undefined {
  if (!(error instanceof ApiError)) return undefined
  const payload = error.payload
  if (!payload || typeof payload !== 'object' || !(name in payload)) return undefined
  const value = payload[name as keyof typeof payload]
  return typeof value === 'string' ? value : undefined
}

export const reasonOf = (error: unknown) => fieldOf(error, 'reason')

/** Năng lực mặc định khi chưa chọn model nào (thẻ vẫn có pill như design). */
const KIND_CAPS: Record<string, string[]> = {
  chatgpt: ['text', 'vision'],
  grok: ['image'],
  openai: ['text', 'vision', 'image'],
  gemini: ['text', 'vision', 'image'],
  openrouter: ['text', 'vision', 'image'],
  anthropic: ['text', 'vision'],
  deepseek: ['text'],
  xai: ['text', 'image'],
  ollama: ['text'],
  openai_compat: ['text'],
}

/** Tag của thẻ = hợp nhất caps model đã chọn; rỗng → theo loại. */
export function providerCaps(provider: AiProvider, draft: AiModel[]): string[] {
  const seen = new Set<string>()
  for (const m of draft) for (const cap of m.caps) seen.add(cap)
  return seen.size > 0 ? [...seen] : (KIND_CAPS[provider.kind] ?? ['text'])
}

/** Host gọn của địa chỉ local (“localhost:11434”). */
export function hostOf(baseUrl: string | undefined): string {
  if (!baseUrl) return ''
  return baseUrl.replace(/^https?:\/\//, '').replace(/\/.*$/, '')
}


/** Tên gói đã lưu của wire ("plus"/"pro") → "Plus"/"Pro"; rỗng giữ rỗng. */
export function planLabel(plan: string | undefined): string {
  if (!plan) return ''
  return plan.charAt(0).toUpperCase() + plan.slice(1)
}

/** Dòng phụ của thẻ — trạng thái thật từ wire, không bịa “gói Plus”. */
export function subLine(provider: AiProvider, models: number): { text: string; tone: '' | 'ok' | 'warn' | 'err' } {
  const hint = provider.keyHint || '••••'
  if (provider.status === 'auth_failed' && provider.auth === 'api_key') {
    return { text: c.subKeyFailed(hint), tone: 'err' }
  }
  if (provider.status === 'auth_failed') return { text: c.subExpired, tone: 'err' }
  if (provider.status === 'unreachable') return { text: c.subUnreachable(hostOf(provider.baseUrl) || provider.label), tone: 'err' }

  if (provider.auth === 'subscription') {
    if (provider.status === 'logged_out') {
      return { text: c.subLoggedOut(c.pkg[provider.kind as 'chatgpt' | 'grok'] ?? c.pkgFallback), tone: '' }
    }
    if (provider.status === 'login_pending') {
      return { text: c.subPending(c.pendingWhere[provider.kind as 'chatgpt' | 'grok'] ?? ''), tone: 'warn' }
    }
    // Artboard: dòng phụ ChatGPT xanh, Grok để mực phụ thường (hạn mức đã
    // mang trọng lượng); hai thẻ cùng “Đã đăng nhập” nhưng khác tông. Grok
    // không ghi gói: xAI không trả tên gói (hỏi đáp #26).
    const named = provider.kind === 'grok' ? '' : planLabel(provider.plan)
    const phrase = named ? `gói ${named}` : ''
    return { text: c.subSignedIn(models, fmtTime(provider.catalogAt), phrase), tone: provider.kind === 'grok' ? '' : 'ok' }
  }
  if (provider.auth === 'local') {
    const host = hostOf(provider.baseUrl)
    return models
      ? { text: c.subLocal(host, models), tone: '' }
      : { text: c.subLocalNone(host), tone: 'warn' }
  }
  return models
    ? { text: c.subKey(hint, models), tone: '' }
    : { text: c.subKeyNone(hint), tone: 'warn' }
}

/** Câu hạn mức ảnh và mốc đặt lại của thẻ đăng nhập gói. */
function quotaLine(provider: AiProvider): { used: string; reset: string; pct: number } | null {
  const q = provider.quota
  if (!q) return null
  const used = q.period === 'week' ? c.quotaWeek(q.used, q.limit) : c.quotaDay(q.used, q.limit)
  let reset: string = c.resetSoon
  const at = new Date(q.resetAt)
  if (!Number.isNaN(at.getTime())) {
    reset = q.period === 'week' ? c.resetWeek(at.getDay()) : c.resetDay(clockOf(q.resetAt))
  }
  return { used, reset, pct: Math.min(100, (q.used / Math.max(1, q.limit)) * 100) }
}

/** Đếm ngược hạn mã thiết bị: "9:41" / "mã đã hết hạn — đăng nhập lại". */
export function countdown(expiresAt: string, now = Date.now()): string {
  const left = Math.max(0, Date.parse(expiresAt) - now)
  if (!Number.isFinite(left) || left <= 0) return c.codeExpired
  const secs = Math.ceil(left / 1000)
  return c.codeExpiry(`${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, '0')}`)
}

/** Vùng đăng nhập subscription — poll GET /login tuần tự 2 s trong khi thẻ mở
 *  (không chồng request, abort khi đóng; 3 lỗi mạng liên tiếp → báo mất kết
 *  nối nhưng vẫn thử tiếp; phase kết thúc → dừng hẳn). */
function LoginFlow({ provider, kind, authWindow, onDone, mobile = false }: {
  provider: AiProvider
  kind: 'chatgpt' | 'grok'
  /** Cửa sổ trắng giữ chỗ từ cú click Đăng nhập — điều hướng tới authUrl khi có. */
  authWindow?: Window | null
  onDone: () => void
  mobile?: boolean
}) {
  const [login, setLogin] = useState<AiLogin | null>(null)
  const [netLost, setNetLost] = useState(false)
  const [codeInput, setCodeInput] = useState('')
  const [pasteOpen, setPasteOpen] = useState(false)
  const [codeErr, setCodeErr] = useState(false)
  const win = useRef<Window | null>(authWindow ?? null)
  const navigated = useRef('')
  const onDoneRef = useRef(onDone)
  onDoneRef.current = onDone

  useEffect(() => {
    let misses = 0
    const stop = sequentialPoll(2000, async (signal) => {
      let next: AiLogin
      try {
        next = await getProviderLogin(provider.id, signal)
      } catch {
        if (signal.aborted) return false
        misses += 1
        if (misses >= 3) setNetLost(true)
        return true
      }
      misses = 0
      setNetLost(false)
      setLogin(next)
      // Điều hướng cửa sổ đã giữ chỗ; popup bị chặn → link bên dưới vẫn mở được.
      const url = next.phase === 'authorize' ? next.authUrl : next.phase === 'code' ? next.verifyUrl : undefined
      if (url && url !== navigated.current && win.current && !win.current.closed) {
        navigated.current = url
        win.current.location.href = url
      }
      const done = next.phase === 'done' || next.phase === 'failed' || next.phase === 'cancelled' || next.phase === 'idle'
      if (next.phase === 'done') {
        await invalidate(keys.aiProviders())
        onDoneRef.current()
      }
      return !done
    })
    return () => { stop(); win.current = null }
  }, [provider.id])

  const waiting = login == null || login.phase === 'authorize' || login.phase === 'exchanging' || login.phase === 'code'
  const showCode = login?.phase === 'code' && Boolean(login.code)
  const failed = login?.phase === 'failed'

  async function pasteCode() {
    const code = codeInput.trim()
    if (!code) return
    setCodeErr(false)
    try {
      await submitProviderLoginCode(provider.id, code)
      setCodeInput('')
      setPasteOpen(false)
    } catch (error) {
      if (reasonOf(error) === 'invalid_code') setCodeErr(true)
      else toast.err(error instanceof Error ? error.message : t.errors.failed)
    }
  }

  async function cancel() {
    try { await cancelProviderLogin(provider.id) } catch { /* phiên đã chết */ }
    onDone()
  }

  async function restart() {
    navigated.current = ''
    setLogin(null)
    if (typeof window.open === 'function') {
      try { win.current = window.open('', '_blank', 'noopener') } catch { win.current = null }
    }
    try { await startProviderLogin(provider.id) } catch (error) {
      toast.err(error instanceof Error ? error.message : t.errors.failed)
    }
  }

  if (failed) {
    return (
      <div className={`conn-err ${stylex.props(settingsStyles.connErr).className}`} role="alert">
        <span className={`conn-err__text ${stylex.props(connErrTextMarker, settingsStyles.connErrText).className}`}><b {...stylex.props(settingsStyles.ConnErrTextB)}>{c.grokFailed}</b> {c.grokFailedText}</span>
        <div className={`conn-err__acts ${stylex.props(settingsStyles.connErrActs).className}`}>
          <Button className="conn-act" xstyle={[settingsStyles.connAct]} size={mobile ? 'lg' : 'compact'} variant="ghost" onClick={() => void restart()}>{c.restart}</Button>
          <Button className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} size={mobile ? 'lg' : 'compact'} variant="quiet" onClick={() => void cancel()}>{c.close}</Button>
        </div>
      </div>
    )
  }

  if (kind === 'grok') {
    return (
      <div className={`conn-code ${stylex.props(settingsStyles.connCode).className}`}>
        <span className={`conn-code__text ${stylex.props(settingsStyles.connCodeText).className}`}>{c.grokIntro}</span>
        {showCode && (
          <div className={`conn-code__row ${stylex.props(settingsStyles.connCodeRow).className}`}>
            <span className={`conn-code__val conn-num ${stylex.props(settingsStyles.connNum, settingsStyles.connCodeVal).className}`} aria-label={login!.code!.split('').join(' ')}>{login!.code}</span>
            <Button size={mobile ? 'lg' : 'compact'} variant="ghost" className="conn-act" xstyle={[settingsStyles.connAct]} icon={<Icon name="copy" size={14} />} onClick={() => void navigator.clipboard?.writeText(login!.code!)}>
              {c.copy}
            </Button>
          </div>
        )}
        <div className={`conn-code__row ${stylex.props(settingsStyles.connCodeRow).className}`}>
          {login?.verifyUrl && (
            <Button size={mobile ? 'lg' : 'compact'} variant="primary" className="conn-act" xstyle={[settingsStyles.connAct]} icon={<Icon name="link" size={14} />} onClick={() => window.open(login.verifyUrl, '_blank', 'noopener')}>
              {c.grokOpen}
            </Button>
          )}
          <Button size={mobile ? 'lg' : 'compact'} variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} onClick={() => void cancel()}>{c.cancelLogin}</Button>
          <span style={{ flex: 1 }} />
          <span className={`conn-code__wait ${stylex.props(settingsStyles.connCodeWait).className}`} role="status">
            {!netLost && <Spinner size={10} color="var(--color-fg-2)" />}
            {netLost ? c.loginReconnect : c.grokWait}
            {!netLost && login?.expiresAt && (
              <span className={`conn-num ${stylex.props(settingsStyles.connNum).className}`}>· {countdown(login.expiresAt)}</span>
            )}
          </span>
        </div>
      </div>
    )
  }

  return (
    <div className={`conn-code ${stylex.props(settingsStyles.connCode).className}`}>
      <div className={`conn-code__row ${stylex.props(settingsStyles.connCodeRow).className}`}>
        <span className={`conn-code__wait ${stylex.props(settingsStyles.connCodeWait).className}`} role="status">
          {waiting && !netLost && <Spinner size={10} color="var(--color-fg-2)" />}
          {netLost ? c.loginReconnect : c.chatgptWait}
        </span>
        <span style={{ flex: 1 }} />
        <Button size={mobile ? 'lg' : 'compact'} variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} onClick={() => void cancel()}>{c.cancelLogin}</Button>
      </div>
      {login?.phase === 'authorize' && login.authUrl && (
        // Fallback bắt buộc: popup bị chặn thì đây là đường duy nhất tới authorize.
        <a className={`conn-link ${stylex.props(settingsStyles.connLink).className}`} href={login.authUrl} target="_blank" rel="noopener noreferrer">{c.openAuth}</a>
      )}
      {waiting && (pasteOpen ? (
        <div className={`conn-manual__row ${stylex.props(connManualRowMarker, settingsStyles.connManualRow).className}`}>
          <Field
            className={`conn-edit__grow ${stylex.props(settingsStyles.connEditGrow).className}`}
            placeholder={c.codePlaceholder}
            value={codeInput}
            onChange={(e) => { setCodeInput(e.target.value); setCodeErr(false) }}
            onKeyDown={(e) => { if (e.key === 'Enter') void pasteCode() }}
            invalid={codeErr}
            aria-label={c.codePlaceholder}
          />
          <Button size={mobile ? 'lg' : 'compact'} variant="primary" className="conn-act" xstyle={[settingsStyles.connAct]} onClick={() => void pasteCode()} disabled={!codeInput.trim()}>{c.pasteCode}</Button>
        </div>
      ) : (
        <button type="button" className={`conn-link ${stylex.props(settingsStyles.connLink).className}`} onClick={() => setPasteOpen(true)}>{c.pasteCodeHint}</button>
      ))}
      {codeErr && <span className={`conn-sub conn-sub--err ${stylex.props(settingsStyles.connSub, settingsStyles.connSubErr).className}`}>{c.codeInvalid}</span>}
    </div>
  )
}

export function ProviderCard({ provider, draft, mobile = false, isNew, revealKey, onToggleModel, onOpenModels, onAssign }: {
  provider: AiProvider
  draft: AiModel[]
  /** Điện thoại: danh mục model mở bằng sheet (MSettings), không inline. */
  mobile?: boolean
  /** Vừa thêm → ghi chú chọn model + catalog mở sẵn, lọc Gợi ý (conn=added). */
  isNew?: boolean
  /** Đổi giá trị (đếm lên) mỗi lần người dùng chọn lại nguồn này ở panel Thêm. */
  revealKey?: number
  onToggleModel: (provider: AiProvider, id: string, on: boolean, meta?: AiModel) => void
  onOpenModels?: (provider: AiProvider) => void
  onAssign?: () => void
}) {
  const cardRef = useRef<HTMLElement | null>(null)
  const [open, setOpen] = useState(Boolean(isNew))
  const [edit, setEdit] = useState<null | 'key' | 'url'>(null)
  const [editValue, setEditValue] = useState('')
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<null | { ok: boolean; ms: number; error?: string }>(null)
  const [loginOpen, setLoginOpen] = useState(provider.status === 'login_pending')
  /** Cửa sổ trắng giữ chỗ trong cú click Đăng nhập (popup chặn chỉ chặn mở
   *  SAU await — mở trước rồi điều hướng sau thì luôn được phép). */
  const authWindow = useRef<Window | null>(null)

  useEffect(() => { if (isNew) setOpen(true) }, [isNew])
  // Chọn lại nguồn đã có ở panel Thêm: cuộn tới thẻ và mở sẵn Đổi khoá (hỏi đáp #27).
  useEffect(() => {
    if (!revealKey || provider.auth !== 'api_key') return
    setEdit('key')
    setEditValue('')
    cardRef.current?.scrollIntoView?.({ block: 'center', behavior: 'smooth' })
  }, [revealKey, provider.auth])

  const subscription = provider.auth === 'subscription'
  const loginKind = provider.kind === 'grok' ? 'grok' : 'chatgpt'
  const failed = provider.status === 'auth_failed' || provider.status === 'unreachable'
  const loggedOut = provider.status === 'logged_out'
  const caps = providerCaps(provider, draft)
  // Đang mở luồng đăng nhập trong UI cũng là “đang đăng nhập” với thẻ: viền
  // hổ phách + câu hướng dẫn, dù server chưa kịp đổi status (artboard grok=code).
  const pendingLogin = loginOpen || provider.status === 'login_pending'
  const sub = loginOpen && loggedOut
    ? { text: c.subPending(c.pendingWhere[provider.kind as 'chatgpt' | 'grok'] ?? ''), tone: 'warn' as const }
    : subLine(provider, draft.length)
  const quota = quotaLine(provider)

  async function runTest() {
    setTesting(true)
    setTestResult(null)
    try {
      setTestResult(await testProvider(provider.id))
    } catch {
      setTestResult({ ok: false, ms: 0 })
    } finally {
      setTesting(false)
      void invalidate(keys.aiProviders())
    }
  }

  async function saveEdit() {
    const value = editValue.trim()
    if (!value) return
    try {
      await updateProvider(provider.id, edit === 'key' ? { apiKey: value } : { baseUrl: value })
      await invalidate(keys.aiProviders())
      setEdit(null)
      setEditValue('')
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.errors.failed)
    }
  }

  async function remove() {
    const guard = (error: unknown) => {
      if (reasonOf(error) === 'in_use') toast.err(c.inUseError)
      else toast.err(error instanceof Error ? error.message : t.errors.failed)
    }
    if (subscription) {
      const doLogout = async () => {
        await logoutProvider(provider.id)
        await invalidate(keys.aiProviders())
      }
      void confirmProviderLogout({ provider: provider.label, fallback: 'OpenRouter', onConfirm: doLogout }).catch(guard)
      return
    }
    // Provider khoá/địa chỉ: xoá hẳn cấu hình → cam xác nhận trước khi DELETE.
    const doDelete = async () => {
      await deleteProvider(provider.id)
      await invalidate(keys.aiProviders())
      toast.ok(c.removed(provider.label))
    }
    void confirm({
      tone: 'warn',
      title: c.removeTitle(provider.label),
      body: c.removeBody,
      cancelLabel: c.cancel,
      confirmLabel: c.removeOk,
      size: 'compact',
      onConfirm: doDelete,
    }).catch(guard)
  }

  async function startLogin() {
    // Giữ chỗ cửa sổ NGAY trong gesture — LoginFlow điều hướng tới authUrl sau.
    if (loginKind === 'chatgpt' && typeof window.open === 'function') {
      try { authWindow.current = window.open('', '_blank', 'noopener') } catch { authWindow.current = null }
    }
    try {
      await startProviderLogin(provider.id)
      setLoginOpen(true)
      await invalidate(keys.aiProviders())
    } catch (error) {
      toast.err(error instanceof Error ? error.message : t.errors.failed)
    }
  }

  // Nút phụ theo loại kết nối: Đăng xuất · Đổi khoá · Đổi địa chỉ.
  const secondary = subscription ? c.logout : provider.auth === 'api_key' ? c.changeKey : c.changeAddress
  const secondaryGo = () => {
    if (subscription) { void remove(); return }
    const kind = provider.auth === 'api_key' ? 'key' : 'url'
    setEdit(edit === kind ? null : kind)
    setEditValue(kind === 'key' ? '' : (provider.baseUrl ?? ''))
  }

  const modelsButton = (buttonStyle?: stylex.StyleXStyles) => (
    draft.length
      ? <Button size={mobile ? 'lg' : 'compact'} variant="ghost" xstyle={[buttonStyle, mobile && settingsStyles.ConnMActs_]} onClick={() => (mobile ? onOpenModels?.(provider) : setOpen((v) => !v))}>
          {c.modelsCount(draft.length)}
          {!mobile && <Icon name="arrow-d" size={12} style={{ transform: open ? 'rotate(180deg)' : undefined }} />}
        </Button>
      : <Button size={mobile ? 'lg' : 'compact'} variant="ghost" className="conn-act--empty" xstyle={[settingsStyles.connActEmpty, mobile && settingsStyles.ConnMActs_]} onClick={() => (mobile ? onOpenModels?.(provider) : setOpen((v) => !v))}>
          {c.modelsPick}
          {!mobile && <Icon name="arrow-d" size={12} style={{ transform: open ? 'rotate(180deg)' : undefined }} />}
        </Button>
  )

  const authButton = loggedOut
    ? <Button size={mobile ? 'lg' : 'compact'} variant="primary" className="conn-act" xstyle={[settingsStyles.connAct]} onClick={() => void startLogin()}>
        {loginKind === 'grok' ? c.loginGrok : c.loginChatGPT}
      </Button>
    : <Button size={mobile ? 'lg' : 'compact'} variant="ghost" xstyle={mobile && settingsStyles.ConnMActs_} onClick={secondaryGo}>{secondary}</Button>

  return (
    <section
      ref={cardRef}
      data-kind={provider.kind}
      className={`conn-card ${stylex.props(settingsStyles.card, settingsStyles.connCard, ((((((failed) && (isNew)) && (pendingLogin)) || (((failed) && (isNew)) && (!(pendingLogin)))) || (((failed) && (!(isNew))) && (pendingLogin))) || (((failed) && (!(isNew))) && (!(pendingLogin)))) && settingsStyles.connCardFail, ((((((failed) && (isNew)) && (pendingLogin)) || (((failed) && (isNew)) && (!(pendingLogin)))) || (((!(failed)) && (isNew)) && (pendingLogin))) || (((!(failed)) && (isNew)) && (!(pendingLogin)))) && settingsStyles.connCardNew, ((((((failed) && (isNew)) && (pendingLogin)) || (((failed) && (!(isNew))) && (pendingLogin))) || (((!(failed)) && (isNew)) && (pendingLogin))) || (((!(failed)) && (!(isNew))) && (pendingLogin))) && settingsStyles.connCardPending).className}`}
      aria-label={provider.label}
    >
      <div className={`conn-head ${stylex.props(settingsStyles.connHead).className}`}>
        <span className={`conn-av ${stylex.props(settingsStyles.connAv, ((provider.kind) === "chatgpt") && settingsStyles.connAvChatgpt, ((provider.kind) === "grok") && settingsStyles.connAvGrok, ((provider.kind) === "gemini") && settingsStyles.connAvGemini, ((provider.kind) === "ollama") && settingsStyles.connAvOllama, ((provider.kind) === "openrouter") && settingsStyles.connAvOpenrouter, ((provider.kind) === "openai") && settingsStyles.connAvOpenai, ((provider.kind) === "deepseek") && settingsStyles.connAvDeepseek, ((provider.kind) === "xai") && settingsStyles.connAvXai, ((provider.kind) === "anthropic") && settingsStyles.connAvAnthropic, ((provider.kind) === "openai_compat") && settingsStyles.connAvOpenaiCompat).className}`} aria-hidden="true">{provider.label.trim()[0]?.toUpperCase() ?? '?'}</span>
        <div className={`conn-id ${stylex.props(settingsStyles.connId).className}`}>
          <div className={`conn-title ${stylex.props(settingsStyles.connTitle).className}`}>
            <span className={`conn-name ${stylex.props(settingsStyles.connName).className}`}>{provider.label}</span>
            <span className={`conn-pill ${stylex.props(settingsStyles.connPill).className}`}>{mobile ? c.kindShort[provider.auth] : c.kind[provider.auth]}</span>
            {!mobile && <span className={`conn-pill ${stylex.props(settingsStyles.connPill).className}`}>{capsLine(caps)}</span>}
          </div>
          <span className={`conn-sub ${stylex.props(settingsStyles.connSub, mobile ? mobileTone(provider) : sub.tone === 'ok' ? settingsStyles.connSubOk : sub.tone === 'warn' ? settingsStyles.connSubWarn : sub.tone === 'err' ? settingsStyles.connSubErr : null).className}`}>
            {mobile ? mobileSub(provider, caps, draft.length) : sub.text}
          </span>
        </div>
        {!mobile && !loginOpen && (
          <div className={`conn-acts ${stylex.props(settingsStyles.connActs).className}`}>
            {loggedOut ? authButton : (
              <>
                {/* Artboard conn=failed: header ẩn Model; nút test giữ câu
                    “Thử kết nối” — “Thử lại” sống trong hộp lỗi. */}
                {!failed && modelsButton([settingsStyles.connAct, open && settingsStyles.connActOpen])}
                <Button size={mobile ? 'lg' : 'compact'} variant="ghost" className="conn-act" xstyle={[settingsStyles.connAct]} pending={testing} pendingLabel={c.testing} onClick={() => void runTest()}>
                  {c.test}
                </Button>
                <Button size={mobile ? 'lg' : 'compact'} variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} onClick={secondaryGo}>{secondary}</Button>
              </>
            )}
          </div>
        )}
      </div>

      {mobile && quota && (
        <span className={`conn-m__quota ${stylex.props(settingsStyles.connMQuota).className}`}>
          <span className={`conn-m__bar ${stylex.props(connMBarMarker, settingsStyles.connMBar).className}`} aria-hidden="true"><span style={{ width: `${quota.pct}%` }} {...stylex.props(settingsStyles.ConnMBarSpan)} /></span>
          <span className={`conn-m__quota-row ${stylex.props(settingsStyles.connMQuotaRow).className}`}>
            <span className={`conn-num ${stylex.props(settingsStyles.connNum).className}`} style={{ color: 'var(--color-fg-2)' }}>{quota.used}</span>
            <span>{quota.reset}</span>
          </span>
        </span>
      )}

      {mobile && !loginOpen && !loggedOut && (
        <div className={`conn-m__acts ${stylex.props(connMActsMarker, settingsStyles.connMActs).className}`}>
          {modelsButton()}
          {subscription
            ? <Button size={mobile ? 'lg' : 'compact'} variant="ghost" className="conn-m__sec" xstyle={[settingsStyles.ConnMActs_, settingsStyles.connMSec]} onClick={secondaryGo}>{secondary}</Button>
            : authButton}
        </div>
      )}
      {mobile && !loginOpen && loggedOut && authButton}

      {quota && !mobile && (
        <div className={`conn-quota ${stylex.props(settingsStyles.connQuota).className}`}>
          <span className={`conn-bar ${stylex.props(connBarMarker, settingsStyles.connBar).className}`} aria-hidden="true"><span style={{ width: `${quota.pct}%` }} {...stylex.props(settingsStyles.ConnBarSpan)} /></span>
          <span className={`conn-num ${stylex.props(settingsStyles.connNum).className}`} style={{ color: 'var(--color-fg-2)' }}>{quota.used}</span>
          <span className={`conn-num ${stylex.props(settingsStyles.connNum).className}`}>{quota.reset}</span>
        </div>
      )}

      {testResult && (
        <span className={`conn-sub ${stylex.props(settingsStyles.connSub, (testResult.ok) && settingsStyles.connSubOk, (!(testResult.ok)) && settingsStyles.connSubErr).className}`} role="status">
          {testResult.ok ? c.testOk(testResult.ms) : (testResult.error || c.testFail)}
        </span>
      )}

      {/* Đang đăng nhập: hiện luồng ở MỌI trạng thái — thẻ đang “chưa đăng nhập”
          mới là thẻ hay bấm Đăng nhập nhất (artboard grok=code). */}
      {subscription && loginOpen && (
        <LoginFlow provider={provider} kind={loginKind} authWindow={authWindow.current} mobile={mobile} onDone={() => setLoginOpen(false)} />
      )}

      {edit && (
        <div className={`conn-edit ${stylex.props(settingsStyles.connEdit).className}`}>
          {edit === 'key'
            ? <PasswordField className={`conn-edit__grow ${stylex.props(settingsStyles.connEditGrow).className}`} placeholder={c.keyLabel} value={editValue} onChange={(e) => setEditValue(e.target.value)} aria-label={c.keyLabel} autoFocus />
            : <Field className="conn-edit__grow" xstyle={[settingsStyles.connEditGrow]} placeholder={c.addressPlaceholder} value={editValue} onChange={(e) => setEditValue(e.target.value)} aria-label={c.addressLabel} autoFocus />}
          <Button size={mobile ? 'lg' : 'compact'} variant="primary" className="conn-act" xstyle={[settingsStyles.connAct]} onClick={() => void saveEdit()} disabled={!editValue.trim()}>{c.saveKey}</Button>
          <Button size={mobile ? 'lg' : 'compact'} variant="quiet" className="conn-act conn-act--quiet" xstyle={[settingsStyles.connActQuiet, settingsStyles.connAct]} onClick={() => setEdit(null)}>{c.cancelEdit}</Button>
          {!subscription && (
            <Button size={mobile ? 'lg' : 'compact'} variant="quiet" className="conn-act" xstyle={[settingsStyles.connAct]} onClick={() => void remove()}>{c.removeOk}</Button>
          )}
        </div>
      )}

      {isNew && !failed && <div className={`conn-note ${stylex.props(settingsStyles.connNote).className}`}>{c.addedNote(provider.label, provider.catalogCount)}</div>}

      {/* conn=failed: thẻ chỉ có hộp lỗi, không mở danh mục rỗng (artboard). */}
      {isNew && !failed && !mobile && (
        <ModelCatalog
          provider={provider}
          draft={draft}
          autoFilter="recommended"
          onToggle={(id, on, meta) => onToggleModel(provider, id, on, meta)}
          onAssign={onAssign}
        />
      )}

      {failed && (
        <div className={`conn-err ${stylex.props(settingsStyles.connErr).className}`} role="alert">
          <span className={`conn-err__text ${stylex.props(connErrTextMarker, settingsStyles.connErrText).className}`}>
            <b {...stylex.props(settingsStyles.ConnErrTextB)}>{c.failedTitle}</b> {c.failedText(provider.label)}
          </span>
          <div className={`conn-err__acts ${stylex.props(settingsStyles.connErrActs).className}`}>
            <Button size={mobile ? 'lg' : 'compact'} variant="warnGhost" className="conn-act" xstyle={[settingsStyles.connAct]} onClick={secondaryGo}>{c.changeKey}</Button>
            <Button size={mobile ? 'lg' : 'compact'} variant="ghost" className="conn-act" xstyle={[settingsStyles.connAct]} icon={<Icon name="refresh" size={14} />} pending={testing} onClick={() => void runTest()}>{c.retry}</Button>
            <Button size={mobile ? 'lg' : 'compact'} variant="quiet" className="conn-act conn-act--text" xstyle={[settingsStyles.connAct]} onClick={() => { setOpen(true); onOpenModels?.(provider) }}>{c.addManualShort}</Button>
          </div>
        </div>
      )}

      {open && !isNew && !mobile && (
        <ModelCatalog
          provider={provider}
          draft={draft}
          onToggle={(id, on, meta) => onToggleModel(provider, id, on, meta)}
          onAssign={onAssign}
        />
      )}
    </section>
  )
}

/** Tông dòng phụ của thẻ điện thoại — artboard: ChatGPT đăng nhập gói xanh,
 *  Grok để mực phụ thường, lỗi/hết hạn cam. */
function mobileTone(provider: AiProvider): stylex.StyleXStyles {
  if (provider.status === 'auth_failed' || provider.status === 'unreachable') return settingsStyles.connSubErr
  if (provider.auth === 'subscription' && provider.status !== 'logged_out' && provider.kind !== 'grok') return settingsStyles.connSubOk
  return null
}

/** Dòng phụ bản điện thoại: chưa có “gói Plus” trong wire → trạng thái + năng lực. */
function mobileSub(provider: AiProvider, caps: string[], models: number): string {
  const phrase = capsPhrase(caps)
  const hint = provider.keyHint || '••••'
  if (provider.status === 'auth_failed' || provider.status === 'unreachable') return subLine(provider, models).text
  if (provider.status === 'logged_out') return `Chưa đăng nhập · ${phrase}`
  if (provider.auth === 'subscription') {
    const plan = provider.kind === 'grok' ? '' : planLabel(provider.plan)
    return `${plan ? `Gói ${plan}` : 'Đã đăng nhập'} · ${phrase} · ${models} model`
  }
  if (provider.auth === 'local') return `${hostOf(provider.baseUrl)} · ${phrase} · miễn phí`
  return `Khoá ${hint} · ${phrase} · ${models} model`
}
