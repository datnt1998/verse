// Tuỳ chỉnh › Kết nối AI — Settings.dc.html sec connect: mỗi nhà cung cấp một thẻ (ô chữ cái 40px · tên · dòng trạng thái ·
// Model · n / Chọn model · Thử kết nối · Đăng xuất/Đổi khoá) + nút nét đứt “Thêm nhà cung cấp”. Chỉnh sửa vào bản nháp chung của SettingsScreen
// (thanh Lưu, If-Match, 409). Bảng chọn model mở trong thẻ (ModelPicker); base URL, giao thức, ngắt khi im lặng ở Nâng cao › Tinh chỉnh model.
import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import { cancelOAuth, completeOAuth, getOAuthStatus, logoutOAuth, startOAuth, testConfig } from '../../../api/client'
import { PASTE_CODE_EMPTY_ERROR, PASTE_CODE_PLACEHOLDER, parseAuthCode } from '../../../lib/authCode'
import type { ConfigAccount, ConfigTestResult, OAuthStart, ProviderConfig, ProviderPreset } from '../../../api/types'
import { invalidate, keys, useAction, useResource } from '../../../data'
import { Button } from '@noverse/ui'
import { Card } from '@noverse/ui'
import { TextField } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { CopyButton } from '../components'
import { configForSave, oauthAccountName, testModelFor } from '../lib/config'
import { Page, SectionHead } from '../parts'
import type { SettingsContext } from '../types'
import { SUB_COLOR, avatarOf, providerName, subLine, testedLabel } from './connectLogic'
import { ModelPicker } from './ModelPicker'

/** Nút viền của thẻ (artboard “Thử kết nối”). */
const outlineBtn: CSSProperties = {
  height: 32, padding: '0 12px', borderRadius: 9, border: '1px solid var(--line-4)', background: 'transparent',
  color: 'var(--color-fg)', fontSize: 12.5, display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
}
/** Nút nền sáng (artboard “Chọn model” khi chưa có model nào). */
const solidBtn: CSSProperties = { ...outlineBtn, border: '1px solid transparent', background: '#EDEDEF', color: '#0B0B0D', fontWeight: 600 }
/** Nút chữ của thẻ (artboard “Đăng xuất” / “Đổi khoá”). */
const textBtn: CSSProperties = {
  height: 32, padding: '0 12px', borderRadius: 9, border: 'none', background: 'transparent',
  color: 'var(--color-fg-3)', fontSize: 12.5, whiteSpace: 'nowrap',
}

function CardButton({ kind, onClick, pending, disabled, label, children, expanded }: {
  kind: 'outline' | 'text' | 'solid'; onClick: () => void; pending?: boolean; disabled?: boolean; label?: string; expanded?: boolean; children: ReactNode
}) {
  const base = kind === 'outline' ? outlineBtn : kind === 'solid' ? solidBtn : textBtn
  return (
    <button type="button" className="hov-ghost" onClick={onClick} disabled={disabled || pending} aria-busy={pending || undefined} aria-expanded={expanded}
      aria-label={label} style={{ ...base, ...(expanded && kind === 'outline' ? { border: '1px solid var(--kit-acc-55)', background: 'rgba(201,166,245,.08)' } : null), opacity: disabled && !pending ? 0.45 : 1 }}>
      {pending && <Spinner size={13} color="currentColor" />}{children}
    </button>
  )
}

export function ProvidersSection({ ctx, open }: { ctx: SettingsContext; open?: string }) {
  const draft = ctx.draft.providers ?? {}
  const ids = ctx.scope === 'global' ? Object.keys(draft) : Array.from(new Set([...Object.keys(ctx.effective.providers ?? {}), ...Object.keys(draft)]))
  const [adding, setAdding] = useState(false)
  const [added, setAdded] = useState<string[]>([])
  // Một bảng chọn model mở tại một thời điểm; `open` = id nhà cung cấp do Trợ lý AI chuyển tới.
  const [openPicker, setOpenPicker] = useState<string | null>(open ?? null)
  const unused = (ctx.presets?.providers ?? []).filter((preset) => !ids.includes(preset.id))

  const addProvider = (preset: ProviderPreset) => {
    let id = preset.id
    if (id === 'custom') {
      let index = 1
      while (ids.includes(`provider-${index}`)) index++
      id = `provider-${index}`
    }
    ctx.setField(`providers.${id}`, {
      ...(preset.type ? { type: preset.type } : { type: 'openai' }),
      ...(preset.baseURL ? { base_url: preset.baseURL } : {}),
      ...(preset.api ? { api: preset.api } : {}),
      models: [],
    })
    setAdded((prev) => [...prev, id])
    // Không cần khoá/đăng nhập (Ollama…) → mở bảng chọn model ngay ở lọc Gợi ý; cần khoá → mở khi đã dán khoá (ProviderCard);
    // gói thuê bao → mở khi đăng nhập xong.
    if (!preset.oauth && !preset.keyRequired) setOpenPicker(id)
    setAdding(false)
  }

  return (
    <Page top={4}>
      <SectionHead title="Kết nối AI" note="Tài khoản và khoá dùng chung cho cả máy chủ. Chỉ quản trị thấy mục này." />
      {ids.map((id, index) => (
        <ProviderCard key={`${ctx.scope}:${id}`} ctx={ctx} id={id} index={index} fresh={added.includes(id)}
          pickerOpen={openPicker === id} setPicker={(v) => setOpenPicker(v ? id : null)} />
      ))}
      {ctx.canEdit && (
        <button type="button" className="hov-ghost" onClick={() => setAdding(!adding)} aria-expanded={adding} style={{
          height: 44, borderRadius: 14, border: '1px dashed rgba(255,255,255,0.14)', background: 'transparent', color: '#C9C9D1',
          fontSize: 13.5, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 8,
        }}>
          <Icon name="add" size={16} />Thêm nhà cung cấp — OpenAI, Anthropic, Gemini, OpenRouter, DeepSeek, Qwen, Ollama…
        </button>
      )}
      {adding && ctx.canEdit && (
        <Card radius={16} padding="16px 18px" style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
          <span style={{ fontSize: 13, fontWeight: 600 }}>Chọn nhà cung cấp</span>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3,minmax(0,1fr))', gap: 8 }}>
            {unused.map((preset) => <Button key={preset.id} onClick={() => addProvider(preset)} style={{ height: 'auto', minHeight: 58, padding: '10px 12px', borderRadius: 12, whiteSpace: 'normal', textAlign: 'left' }}><span style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-start', gap: 2, width: '100%' }}><span style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 13 }}>{preset.label}{preset.oauth && <Icon name="login" size={13} style={{ color: 'var(--color-accent)' }} />}</span><span style={{ fontSize: 11.5, color: 'var(--color-fg-4)', fontWeight: 400 }}>{preset.oauth ? 'Gói thuê bao · đăng nhập OAuth' : preset.keyRequired ? 'API Key' : 'API Key không bắt buộc'}</span></span></Button>)}
          </div>
          {!unused.length && <span style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>Đã thêm mọi nhà cung cấp có sẵn.</span>}
        </Card>
      )}
    </Page>
  )
}

function ProviderCard({ ctx, id, index, fresh, pickerOpen, setPicker }: {
  ctx: SettingsContext; id: string; index: number; fresh: boolean
  pickerOpen: boolean; setPicker: (open: boolean) => void
}) {
  const preset = ctx.presets?.providers.find((entry) => entry.id === id) ?? null
  const inherited = ctx.effective.providers?.[id] ?? {}
  const own = ctx.draft.providers?.[id] ?? {}
  const provider: ProviderConfig = ctx.scope === 'global' ? own : { ...inherited, ...own }
  // Nhà cung cấp vừa thêm bằng khoá API: mở sẵn ô nhập khoá.
  const [editingKey, setEditingKey] = useState(fresh && !preset?.oauth)
  const [flow, setFlow] = useState<OAuthStart | null>(null)
  const [accountOverride, setAccountOverride] = useState<{ account: ConfigAccount | null } | null>(null)
  const [oauthFailure, setOauthFailure] = useState('')
  const [pasteCode, setPasteCode] = useState('')
  const [pasteError, setPasteError] = useState('')
  const [testResult, setTestResult] = useState<ConfigTestResult | null>(null)
  const account = accountOverride ? accountOverride.account : ctx.response?.accounts?.[id] ?? null
  const models = provider.models ?? []
  const canKey = ctx.canEdit && ctx.scope === 'global'
  const keySet = !own.clear_api_key && (own.api_key_set ?? inherited.api_key_set)
  const oauthStatus = useResource([...keys.config('global'), 'oauth', flow?.id ?? null], () => getOAuthStatus(flow!.id), { enabled: !!flow, staleTime: 0 })

  // Chỉ đóng ô khoá khi máy chủ trả cấu hình mới (sau Lưu) — không chạy lúc mount, nếu không thẻ vừa thêm mất ô nhập khoá.
  const seenResponse = useRef(ctx.response)
  useEffect(() => {
    if (seenResponse.current === ctx.response) return
    seenResponse.current = ctx.response
    setEditingKey(false); setAccountOverride(null)
  }, [ctx.response])
  // Vừa thêm bằng khoá API: dán khoá xong thì mở bảng chọn model ở Gợi ý.
  const hasNewKey = !!own.api_key
  useEffect(() => { if (fresh && hasNewKey) setPicker(true) }, [fresh, hasNewKey])
  useEffect(() => {
    if (!flow) return
    const deadline = Date.now() + 10 * 60_000
    const timer = setInterval(() => {
      if (Date.now() >= deadline) { setOauthFailure('Đã hết thời gian đăng nhập'); setFlow(null); return }
      void oauthStatus.refresh()
    }, 2000)
    return () => clearInterval(timer)
  }, [flow?.id, oauthStatus.refresh])
  useEffect(() => {
    const status = oauthStatus.data
    if (!flow || !status || status.state === 'pending') return
    if (status.state === 'done' && status.account) {
      setAccountOverride({ account: { ...status.account, connected: true, expiresAt: null } })
      // Đăng nhập xong thì mở bảng chọn model (lọc Gợi ý khi chưa có model).
      setPicker(true)
      invalidate(keys.config('global'))
    } else setOauthFailure(status.error || 'Không đăng nhập được')
    setFlow(null)
  }, [flow, oauthStatus.data])

  const test = useAction(async () => {
    setTestResult(null)
    const result = await testConfig(id, testModelFor(models), configForSave({ provider: id, providers: { [id]: provider } }))
    if (!result.ok) throw new Error(result.message || 'Không kết nối được')
    setTestResult(result)
    return result
  })
  const connect = useAction(async (device: boolean) => {
    setOauthFailure(''); setPasteCode(''); setPasteError('')
    const started = await startOAuth(id, device)
    setFlow(started)
    if (started.mode === 'browser' && started.url) window.open(started.url, '_blank', 'noopener')
    return started
  })
  const disconnect = useAction(async () => {
    await logoutOAuth(id)
    setAccountOverride({ account: null })
    setFlow(null)
    return true
  }, { success: 'Đã đăng xuất', invalidate: [keys.config('global')] })
  const cancel = useAction(async () => {
    if (flow) await cancelOAuth(flow.id)
    setPasteCode(''); setPasteError('')
    setFlow(null)
    return true
  })
  const complete = useAction(async () => {
    if (!flow) return false
    if (!parseAuthCode(pasteCode)) { setPasteError(PASTE_CODE_EMPTY_ERROR); return false }
    try {
      await completeOAuth(flow.id, pasteCode)
      setPasteCode('')
      setPasteError('')
      void oauthStatus.refresh()
      return true
    } catch (error) {
      setPasteError(error instanceof Error ? error.message : 'Mã không dùng được')
      return false
    }
  }, { error: false })
  const actionError = test.error || connect.error || disconnect.error || cancel.error || oauthStatus.error
  const set = (key: keyof ProviderConfig, value: unknown) => ctx.setField(`providers.${id}.${key}`, value)
  const local = /^https?:\/\/(localhost|127\.0\.0\.1)([:/]|$)/.test(provider.base_url ?? '')

  const name = providerName(id, preset)
  const avatar = avatarOf(name, index)
  const sub = subLine({
    oauth: !!preset?.oauth,
    account: preset && account?.connected ? { name: oauthAccountName(preset, account), plan: account.plan } : null,
    keySet: !!keySet,
    keyHint: own.api_key_hint ?? inherited.api_key_hint,
    local,
    models: models.length,
  })
  const keyPending = !!own.api_key || !!own.clear_api_key
  const keyPanel = canKey && !preset?.oauth && (editingKey || keyPending)

  return (
    <section className="card" style={{ borderRadius: 16, padding: '14px 16px', display: 'flex', flexDirection: 'column', gap: 12, ...(models.length === 0 ? { border: '1px solid var(--kit-warn-35)' } : null) }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <span aria-hidden="true" style={{ width: 40, height: 40, borderRadius: 12, background: avatar.bg, color: '#0B0B0D', fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>{avatar.ini}</span>
        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', lineHeight: 1.4 }}>
          <span style={{ fontSize: 14.5, fontWeight: 500 }}>{name}</span>
          <span style={{ fontSize: 12.5, color: SUB_COLOR[sub.tone] }}>{sub.text}</span>
        </span>
        {/* “Model · n” mở bảng chọn model; 0 model → nút sáng “Chọn model”. */}
        <CardButton kind={models.length === 0 && !pickerOpen ? 'solid' : 'outline'} expanded={pickerOpen}
          label={models.length === 0 ? `Chọn model của ${name}` : `Model của ${name}`} onClick={() => setPicker(!pickerOpen)}>
          <Icon name="cpu" size={14} />{models.length === 0 ? 'Chọn model' : `Model · ${models.length}`}
          <Icon name="arrow-down-2" size={13} style={{ transform: pickerOpen ? 'rotate(180deg)' : 'none' }} />
        </CardButton>
        {ctx.admin && <CardButton kind="outline" pending={test.pending} disabled={test.busy} label={`Thử kết nối ${name}`} onClick={() => { void test.run() }}>
          {testResult ? testedLabel(testResult.latencyMs) : 'Thử kết nối'}
        </CardButton>}
        {ctx.admin && preset?.oauth && account?.connected && (
          <CardButton kind="text" pending={disconnect.pending} disabled={disconnect.busy} label={`Đăng xuất ${name}`} onClick={() => { void disconnect.run() }}>Đăng xuất</CardButton>
        )}
        {ctx.admin && preset?.oauth && !account?.connected && (
          <CardButton kind="text" pending={connect.pending} disabled={connect.busy || !!flow} label={`Đăng nhập ${name}`} onClick={() => { void connect.run(!!preset.deviceOnly) }}>Đăng nhập</CardButton>
        )}
        {ctx.admin && preset?.oauth && !account?.connected && !preset.deviceOnly && (
          <CardButton kind="text" pending={connect.pending} disabled={connect.busy || !!flow} label={`Đăng nhập ${name} bằng mã thiết bị`} onClick={() => { void connect.run(true) }}>Mã thiết bị</CardButton>
        )}
        {canKey && !preset?.oauth && (
          <CardButton kind="text" label={`Đổi khoá ${name}`} onClick={() => setEditingKey(!editingKey)}>Đổi khoá</CardButton>
        )}
      </div>
      {actionError instanceof Error && <div role="alert" style={{ paddingLeft: 54, color: 'var(--color-err)', fontSize: 12 }}>{actionError.message}</div>}
      {keyPanel && (
        <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8, paddingLeft: 54 }}>
          <TextField type="password" aria-label={`Khoá API mới cho ${name}`} autoComplete="off" placeholder="Dán khoá API mới" value={own.api_key ?? ''}
            onChange={(value) => set('api_key', value)} style={{ width: 260, height: 34, fontFamily: 'var(--font-mono)', fontSize: 12 }} />
          {keySet && !own.clear_api_key && (
            <CardButton kind="text" onClick={() => { set('clear_api_key', true); set('api_key', undefined) }}>Xoá khoá</CardButton>
          )}
          {own.clear_api_key && <span style={{ fontSize: 12.5, color: '#8E8E99' }}>Khoá sẽ bị xoá khi lưu.</span>}
          <CardButton kind="text" onClick={() => { set('api_key', undefined); set('clear_api_key', undefined); setEditingKey(false) }}>
            {keyPending ? 'Bỏ thay đổi' : 'Đóng'}
          </CardButton>
        </div>
      )}
      {preset && (flow || oauthFailure) && <div style={{ paddingLeft: 54, display: 'flex', flexDirection: 'column', gap: 8 }}>
        {flow && <><span style={{ fontSize: 13 }}>Đăng nhập {name} bằng tài khoản của bạn — Noverse sẽ tự gia hạn, không phải đăng nhập lại.</span><div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}><a href={flow.verifyURL || flow.url} target="_blank" rel="noreferrer" style={{ fontSize: 12, color: 'var(--color-accent)' }}>Mở trang đăng nhập</a>{flow.mode === 'paste' && <CopyButton value={flow.url} label="Sao chép liên kết" />}{flow.userCode && <><code className="mono">{flow.userCode}</code><CopyButton value={flow.userCode} label="Sao chép mã" /></>}<span role="status" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: 'var(--color-fg-3)' }}><Spinner size={13} />Đang chờ bạn xác nhận…</span><Button size="sm" pending={cancel.pending} disabled={cancel.busy} onClick={() => { void cancel.run() }}>Huỷ</Button></div>{flow.mode === 'paste' && <><span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>Đăng nhập Claude rồi dán mã hiển thị (dạng code#state) hoặc cả URL chuyển hướng vào ô bên dưới.</span><div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 8 }}><TextField aria-label={PASTE_CODE_PLACEHOLDER} aria-invalid={!!pasteError} autoComplete="off" spellCheck={false} placeholder={PASTE_CODE_PLACEHOLDER} value={pasteCode} onChange={(value) => { setPasteCode(value); setPasteError('') }} onKeyDown={(event) => { if (event.key === 'Enter' && pasteCode.trim()) { event.preventDefault(); void complete.run() } }} style={{ flex: '1 1 240px', height: 34, fontFamily: 'var(--font-mono)', fontSize: 12 }} /><Button variant="primary" size="sm" pending={complete.pending} disabled={!pasteCode.trim() || complete.busy} onClick={() => { void complete.run() }} style={{ height: 30, fontSize: 12 }}>Xác nhận</Button></div>{pasteError && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>{pasteError}</span>}</>}</>}
        {oauthFailure && <span role="alert" style={{ fontSize: 12, color: 'var(--color-err)' }}>{oauthFailure}</span>}
      </div>}
      {pickerOpen && (
        <ModelPicker ctx={ctx} id={id} name={name} provider={provider} preset={preset}
          local={local} oauth={!!preset?.oauth && !!account?.connected} draftModels={models}
          intro={fresh && models.length === 0} onEditKey={() => setEditingKey(true)} />
      )}
    </section>
  )
}
