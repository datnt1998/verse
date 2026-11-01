// MMore › Model cho từng trợ lý — MMore.dc.html.
// Model/mức suy luận dùng chung cả app: đọc/ghi cấu hình toàn cục, chỉ quản trị đổi, lưu ngay.
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { useSession } from '../../../api/session'
import { saveModelContexts, useModelContext } from '../../../data/modelContext'
import { contextDraft, type ContextDraft } from '../../../lib/modelContext'
import { t } from '../../../i18n/vi'
import { ModelContextChoices } from '../../../ui/ModelContextChoices'
import { ConflictError, getConfigModels, getConfigPresets, testConfig, swapProvider, undoSwapProvider } from '../../../api/client'
import type { Config, ConfigResponse, ModelRoleKey } from '../../../api/types'
import { useResource } from '../../../data/useResource'
import { useAction } from '../../../data/useAction'
import { appConfigDoc, putAppConfig, useAppConfig } from '../../../data/appModels'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Radio } from '@noverse/ui'
import { ROLE_META, formatContext } from '../../cmdresults/logic'
import { configForCompare, mergeConfig, roleModel, setPath, thinkingLabel, thinkingSegments } from '../../settings/lib/config'
import { providerName } from '../../settings/sections/connectLogic'
import {
  issueAction, issueLevel, issueOf, issueShort, issueTech, issueTitle, quotaRetryText, retryClock, type ProviderIssue,
} from '../../settings/lib/providerError'
import { ResourceNotice } from './parts'

export const MODEL_ROLES: ModelRoleKey[] = ['default', 'architect', 'writer', 'editor']

const sx = stylex.create({
  levelGrid: { marginInline: 16, display: 'grid', gap: 4, padding: 4, borderRadius: 14, backgroundColor: 'var(--color-card)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-1)' },
  level: { height: 44, borderWidth: 0, borderRadius: 10, backgroundColor: 'transparent', color: 'var(--color-fg-3)', fontSize: 14, whiteSpace: 'nowrap' },
  levelOn: { backgroundColor: 'var(--color-surface-7)', color: 'var(--color-fg)', fontWeight: 500 },
  banner: { margin: '0 16px', display: 'flex', flexDirection: 'column', gap: 12, padding: '14px 16px', borderRadius: 16, borderWidth: 1, borderStyle: 'solid' },
  bannerTop: { display: 'flex', gap: 12, alignItems: 'flex-start' },
  bannerIcon: { flex: '0 0 auto', width: 36, height: 36, borderRadius: 11, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  bannerCopy: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  bannerTitle: { fontSize: 15.5, fontWeight: 600, lineHeight: 1.35 },
  bannerText: { fontSize: 13.5, color: 'var(--color-fg-3)', lineHeight: 1.5 },
  chip: { alignSelf: 'flex-start', height: 22, paddingInline: 8, borderRadius: 7, fontSize: 12, fontWeight: 500, display: 'inline-flex', alignItems: 'center' },
  tech: { padding: '8px 10px', borderRadius: 10, backgroundColor: 'var(--color-bg)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--line-1)', fontSize: 11.5, color: 'var(--color-fg-3)', fontFamily: 'var(--font-mono)', overflowWrap: 'anywhere' },
  sheet: { position: 'absolute', left: 0, right: 0, bottom: 0, zIndex: 50, display: 'flex', flexDirection: 'column', gap: 12, padding: '0 16px 26px', backgroundColor: 'var(--color-surface-3)', borderRadius: '26px 26px 0 0', borderTopWidth: 1, borderTopStyle: 'solid', borderColor: 'var(--line-2)' },
})

/** Đồng hồ cho câu quota trên điện thoại. */
function useNow(active: boolean, ms = 30_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!active) return
    const timer = window.setInterval(() => setNow(Date.now()), ms)
    return () => window.clearInterval(timer)
  }, [active, ms])
  return now
}

export function Models({ active, onPick, disabled, toast }: {
  active: ModelRoleKey | null
  onPick: (role: ModelRoleKey) => void
  disabled: boolean
  toast: (text: string) => void
}) {
  const session = useSession()
  const admin = session.user?.role === 'admin'
  const resource = useAppConfig()
  const response = resource.data
  const [draft, setDraft] = useState<Config | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const failed = useRef<Config | null>(null)
  const [reset, setReset] = useState('')
  const [swapping, setSwapping] = useState(false)
  const [swapped, setSwapped] = useState<{ undoToken: string; count: number; to: string; resets: { role: string; effort: string }[] } | null>(null)
  const [techOpen, setTechOpen] = useState(false)
  const [keyOpen, setKeyOpen] = useState(false)
  const [key, setKey] = useState('')
  const [keyBusy, setKeyBusy] = useState(false)
  const [keyError, setKeyError] = useState('')

  // Máy chủ xác nhận xong thì bỏ bản nháp (draft) để hiển thị đúng bản đã lưu.
  useEffect(() => { setDraft(null); setError(''); failed.current = null }, [response?.version])

  const doc = draft ?? mergeConfig({}, appConfigDoc(response))
  const role = active ? roleModel(doc, active) : null
  const canEdit = admin && !disabled
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60_000 })
  const nameOf = (provider: string) => providerName(provider, presets.data?.providers.find((preset) => preset.id === provider) ?? null)

  const providers = Object.entries(doc.providers ?? {}).map(([provider, entry]) => ({
    provider,
    label: nameOf(provider),
    models: (entry.models ?? []).filter((model) => model.name),
  }))
  const catalog = useResource(['app-model-catalog', JSON.stringify(providers.map((provider) => [provider.provider, provider.models.map((model) => model.name)]))], async () => {
    const entries = await Promise.all(providers.map(async ({ provider, models }) => [provider, (await getConfigModels(provider, models.map((model) => model.name))).models] as const))
    return Object.fromEntries(entries)
  }, { enabled: providers.length > 0 })
  const levelsFor = (provider: string, model: string): string[] | undefined =>
    catalog.data?.[provider]?.find((entry) => entry.name === model)?.thinking

  // Nối tiếp từng lần ghi để hai lần chạm liên tiếp không gửi hai bản cũ đè nhau; mỗi lần vẫn là
  // MỘT bản cấu hình chung hoàn chỉnh. 409 → áp lại đúng thay đổi này lên bản mới nhất, thử một lần.
  const chain = useRef<Promise<unknown>>(Promise.resolve())
  const latest = useRef<ConfigResponse | null>(response)
  const docRef = useRef(doc)
  docRef.current = doc
  useEffect(() => { latest.current = response }, [response])
  const save = (mutate: (config: Config) => Config): Promise<boolean> => {
    const task = chain.current.then(async () => {
      const base = latest.current
      if (!base) return false
      const next = mutate(docRef.current)
      docRef.current = next
      setDraft(next)
      setSaving(true)
      setError('')
      const attempt = async (current: ConfigResponse, docToSave: Config): Promise<ConfigResponse> => {
        try {
          return await putAppConfig(current, docToSave)
        } catch (cause) {
          if (cause instanceof ConflictError && cause.current && typeof cause.current === 'object' && 'target' in cause.current) {
            const fresh = cause.current as ConfigResponse
            return await putAppConfig(fresh, mutate(configForCompare(fresh.target.config)))
          }
          throw cause
        }
      }
      try {
        latest.current = await attempt(base, next)
        failed.current = null
        return true
      } catch (cause) {
        docRef.current = appConfigDoc(latest.current)
        setDraft(null)
        failed.current = next
        setError(`${cause instanceof Error ? cause.message : 'Không kết nối được máy chủ.'} Đang dùng lựa chọn đã lưu trước đó — bấm Thử lại để lưu lại.`)
        return false
      } finally {
        setSaving(false)
      }
    })
    chain.current = task.catch(() => {})
    return task
  }
  const retry = () => {
    const next = failed.current
    if (!next) return
    failed.current = null
    void save(() => next)
  }

  const primary = doc.provider ?? ''
  const primaryIssue = issueOf(response, primary)
  const using = MODEL_ROLES.filter((key) => roleModel(doc, key).provider === primary)
  const alternative = providers.flatMap((provider) => provider.models.map((model) => ({ provider: provider.provider, model: model.name })))
    .find((option) => option.provider !== primary && !issueOf(response, option.provider))
  const now = useNow(primaryIssue?.kind === 'quota')
  const issue = swapped ? null : primaryIssue

  const pickModel = (key: ModelRoleKey, provider: string, model: string) => {
    const previous = roleModel(doc, key).effort
    const levels = levelsFor(provider, model)
    const invalid = !!levels && !!previous && !levels.includes(previous)
    setReset(invalid ? previous : '')
    void save((config) => {
      let next = setPath(config, key === 'default' ? 'provider' : `roles.${key}.provider`, provider)
      next = setPath(next, key === 'default' ? 'model' : `roles.${key}.model`, model)
      return setPath(next, key === 'default' ? 'reasoning_effort' : `roles.${key}.reasoning_effort`, invalid ? '' : previous)
    }).then((saved) => {
      if (saved) toast(`${ROLE_META[key].label} → ${model} · cho cả app`)
      else setReset('')
    })
  }
  const pickLevel = (key: ModelRoleKey, level: string) => {
    setReset('')
    void save((config) => {
      let next = config
      if (key !== 'default') {
        const mine = config.roles?.[key]
        if (!mine?.provider && !mine?.model && config.provider && config.model) {
          next = setPath(next, `roles.${key}.provider`, config.provider)
          next = setPath(next, `roles.${key}.model`, config.model)
        }
      }
      return setPath(next, key === 'default' ? 'reasoning_effort' : `roles.${key}.reasoning_effort`, level)
    }).then((saved) => {
      if (saved) toast(`Mức suy luận ${ROLE_META[key].label}: ${thinkingLabel(level)} · cho cả app`)
    })
  }

  const doSwap = async () => {
    if (!alternative || !response || swapping) return
    setSwapping(true)
    try {
      const result = await swapProvider(primary, alternative.provider, alternative.model, response.version)
      setSwapped({ undoToken: result.undoToken, count: result.swapped.length, to: nameOf(alternative.provider),
        resets: result.swapped.filter((row) => row.from.reasoning_effort && row.from.reasoning_effort !== row.to.reasoning_effort)
          .map((row) => ({ role: t.settingsModelsW.swapRoles[row.role] ?? 'Trợ lý', effort: row.from.reasoning_effort })) })
      setDraft(null)
      void resource.refresh()
      toast(`Đã chuyển ${result.swapped.length} trợ lý sang ${nameOf(alternative.provider)} · cho cả app`)
      if (result.warning) toast(result.warning)
    } catch (cause) {
      toast(cause instanceof Error ? cause.message : 'Không chuyển được nhà cung cấp — thử lại.')
    } finally {
      setSwapping(false)
    }
  }
  const undo = async () => {
    if (!swapped || swapping) return
    setSwapping(true)
    try {
      const restored = await undoSwapProvider(swapped.undoToken)
      setSwapped(null)
      void resource.refresh()
      toast('Đã hoàn tác · cho cả app')
      if (restored.warning) toast(restored.warning)
    } catch (cause) {
      toast(cause instanceof Error ? cause.message : 'Không hoàn tác được — thử lại.')
    } finally {
      setSwapping(false)
    }
  }
  const saveKey = async () => {
    if (!response || !primaryIssue || !key.trim()) return
    setKeyBusy(true)
    setKeyError('')
    try {
      const result = await testConfig(primary, undefined, { provider: primary, providers: { [primary]: { api_key: key.trim() } } })
      if (!result.ok) { setKeyError(result.message || 'Khoá chưa kết nối được.'); return }
      const next = setPath(appConfigDoc(response), `providers.${primary}.api_key`, key.trim())
      await putAppConfig(response, next)
      setKeyOpen(false); setKey('')
      void resource.refresh()
      toast(`Đã lưu khoá ${nameOf(primary)} mới · kết nối được`)
    } catch (cause) {
      setKeyError(cause instanceof Error ? cause.message : 'Không lưu được khoá — thử lại.')
    } finally {
      setKeyBusy(false)
    }
  }

  const selected = active ? role : null
  const selectedLevels = selected ? levelsFor(selected.provider, selected.model) : undefined
  const segments = selected ? thinkingSegments(selectedLevels, selected.effort) : []
  const levelCount = (selectedLevels ?? []).filter((level) => level !== '').length
  const color = primaryIssue?.kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)'

  return <>
    <ResourceNotice resource={resource} label="model trợ lý" />
    {!!error && <div className="mm-error" role="alert">{error}<button type="button" className="mm-revoke" style={{ marginLeft: 8 }} onClick={retry}>Thử lại</button></div>}
    {saving && <p className="mm-note" role="status">Đang lưu lựa chọn…</p>}
    {!admin && <p className="mm-note">Model dùng chung cho mọi người nên chỉ quản trị đổi được. Bạn vẫn xem được trợ lý nào đang dùng model nào.</p>}

    {(!!swapped || (!!primaryIssue && using.length > 0)) && (
      <section role="status" {...stylex.props(sx.banner)}
        style={{ borderColor: `color-mix(in srgb, ${swapped ? 'var(--color-ok)' : color} 30%, transparent)`, background: `color-mix(in srgb, ${swapped ? 'var(--color-ok)' : color} 5%, transparent)` }}>
        <div {...stylex.props(sx.bannerTop)}>
          <span aria-hidden="true" {...stylex.props(sx.bannerIcon)} style={{ background: `color-mix(in srgb, ${swapped ? 'var(--color-ok)' : color} 12%, transparent)`, color: swapped ? 'var(--color-ok)' : color }}>
            <Icon name={swapped ? 'tick-circle' : 'warning-2'} size={18} />
          </span>
          <span {...stylex.props(sx.bannerCopy)}>
            <b {...stylex.props(sx.bannerTitle)}>{swapped ? `Đã chuyển ${swapped.count} trợ lý sang ${swapped.to}` : issue ? issueTitle(nameOf(primary), issue.kind) : ''}</b>
            {issue && <span {...stylex.props(sx.chip)} style={{ background: `color-mix(in srgb, ${color} 14%, transparent)`, color }}>{issueLevel(issue.kind)}</span>}
            <span {...stylex.props(sx.bannerText)}>{swapped
              ? `Áp dụng cho cả app từ lượt viết kế tiếp.${swapped.resets.map((row) => ` ${row.role}: Mức ${thinkingLabel(row.effort)} không có — đã về Tự động.`).join('')}`
              : issue ? `${using.length} trợ lý đang dùng ${nameOf(primary)}${issue.kind === 'quota' ? `. ${quotaRetryText(issue.retryAt, now)}` : ' nên không chạy được.'}` : ''}</span>
          </span>
        </div>
        {issue && issue.kind !== 'quota' && canEdit && using.length > 0 && alternative && (
          <button className="mbtn pri tap" type="button" disabled={swapping} onClick={() => void doSwap()} style={{ height: 46, fontSize: 15 }}>
            Chuyển {using.length} trợ lý sang {nameOf(alternative.provider)}
          </button>
        )}
        {issue?.kind === 'key' && canEdit && (
          <button className="mbtn gho tap" type="button" onClick={() => setKeyOpen(true)} style={{ height: 44, fontSize: 15 }}>
            Nhập khoá mới {nameOf(primary)}
          </button>
        )}
        {issue?.kind === 'expired' && canEdit && (
          <p className="mm-note" style={{ margin: '0 30px' }}>Gia hạn {nameOf(primary)} trên máy tính rồi đăng nhập lại; điện thoại không đăng nhập được.</p>
        )}
        {swapped && <button className="mbtn gho tap" type="button" disabled={swapping} onClick={() => void undo()} style={{ height: 44, fontSize: 15 }}>Hoàn tác</button>}
        {issue && <>
          <button type="button" className="tap" aria-expanded={techOpen} onClick={() => setTechOpen((open) => !open)}
            style={{ alignSelf: 'flex-start', height: 36, padding: 0, border: 'none', background: 'transparent', color: 'var(--color-fg-4)', fontSize: 13 }}>
            Chi tiết kỹ thuật
          </button>
          {techOpen && <div {...stylex.props(sx.tech)}>{issueTech(primary, issue)}</div>}
        </>}
      </section>
    )}

    {!active && response && <>
      <p className="mm-note">Dùng chung cho cả app — mọi dự án, mọi truyện. Đổi là lưu ngay, áp dụng từ lượt viết kế tiếp.</p>
      <div className="mm-heading">{admin ? 'Chạm để đổi' : 'Đang dùng'}</div>
      <div className="mm-group">{MODEL_ROLES.map((key) => {
        const meta = ROLE_META[key]
        const value = roleModel(doc, key)
        const rowIssue = issueOf(response, value.provider)
        return <button className="mm-row" type="button" key={key} disabled={!admin || disabled} onClick={() => onPick(key)}>
          {meta.orb ? <AgentOrb agent={meta.orb} size={30} /> : <span className="mm-default"><Icon name="layer" size={16} /></span>}
          <span className="mm-copy"><span>{meta.label}</span><span className="mm-sub">{value.model
            ? <><span className="mm-mono">{value.model}</span> · {thinkingLabel(value.effort)}</>
            : 'Chưa chọn model'}</span>
          {!!rowIssue && <span className="mm-sub" style={{ color: rowIssue.kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)' }}>
            <Icon name="warning-2" size={13} /> {nameOf(value.provider)} {issueShort(rowIssue.kind)}{rowIssue.kind === 'quota' ? ` · tự thử lại lúc ${retryClock(rowIssue.retryAt) || '—'}` : ''}
          </span>}
          </span>
          {admin && <Icon name="arrow-right-2" size={16} className="mm-chevron" />}
        </button>
      })}</div>
      <p className="mm-note">“Mặc định” áp cho trợ lý chưa chọn riêng.</p>
    </>}

    {active && selected && response && <>
      {!selected.model && <p className="mm-note">Chưa chọn model cho trợ lý này. Chọn một model bên dưới.</p>}
      {!!selected.model && !providers.some((provider) => provider.provider === selected.provider && provider.models.some((model) => model.name === selected.model)) && (
        <p className="mm-note">Đang dùng: <span className="mm-mono">{selected.model}</span>{selected.provider && ` · ${selected.provider}`}. Model này chưa có trong danh sách nhà cung cấp.</p>
      )}
      {providers.map((provider) => {
        const providerIssue = issueOf(response, provider.provider)
        return <section key={provider.provider}>
          <div className="mm-heading">{provider.label}{providerIssue && <span style={{ color: providerIssue.kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)' }}> · {issueShort(providerIssue.kind)}</span>}</div>
          <div className="mm-group" role="radiogroup" aria-label={provider.label}>
            {provider.models.map((model) => {
              const isSelected = selected.provider === provider.provider && selected.model === model.name
              const info = catalog.data?.[provider.provider]?.find((entry) => entry.name === model.name)
              return <Radio key={model.name} className="mm-row" mark="tick" markPosition="end" checked={isSelected}
                disabled={!canEdit || saving}
                onChange={() => { if (!isSelected) pickModel(active, provider.provider, model.name) }}
                label={<span className="mm-mono" style={{ fontSize: 14.5 }}>{model.name}</span>}
                description={<span style={{ fontSize: 13 }}>{formatContext(info?.contextWindow ?? 0) || 'Model đã cấu hình'}</span>}
                style={{ padding: '8px 14px', margin: 0, gap: 12, minHeight: 54, fontSize: 15, lineHeight: 1.35 }} />
            })}
            {!provider.models.length && <p className="mm-empty">Chưa có model. Thêm model trên máy tính.</p>}
          </div>
        </section>
      })}
      {!providers.length && <p className="mm-note">Chưa có nhà cung cấp AI. Thiết lập nhà cung cấp trên máy tính.</p>}
      {selected.provider && selected.model && <MobileModelContext provider={selected.provider} model={selected.model} disabled={!canEdit || saving} toast={toast} />}
      <div className="mm-heading">Mức suy luận · {levelCount > 0 ? `${levelCount} mức của ${selected.model}` : `${selected.model || 'Model này'} không có mức suy luận`}</div>
      {segments.length > 1 && (
        <div role="radiogroup" aria-label="Mức suy luận" {...stylex.props(sx.levelGrid)} style={{ gridTemplateColumns: `repeat(${Math.min(4, segments.length)}, minmax(0, 1fr))` }}>
          {segments.map((segment) => {
            const on = segment.id === selected.effort
            return <button key={segment.id || 'auto'} type="button" role="radio" aria-checked={on} disabled={!canEdit || saving}
              onClick={() => { if (!on) pickLevel(active, segment.id) }}
              className={`tap ${stylex.props(sx.level, on && sx.levelOn).className ?? ''}`}>
              {segment.label}
            </button>
          })}
        </div>
      )}
      {!!reset && <p className="mm-note" style={{ color: 'var(--color-warn)' }}>Mức {thinkingLabel(reset)} không có ở {selected.model} — đã về Tự động.</p>}
      <p className="mm-note">Tự động: dùng mức mặc định của model. Đổi là đổi cho cả app, hiện ngay ở danh sách trợ lý.</p>
    </>}

    {keyOpen && primaryIssue && <>
      <div className="scrim" aria-hidden="true" onClick={() => { setKeyOpen(false); setKeyError('') }} />
      <section role="dialog" aria-label="Nhập khoá mới" {...stylex.props(sx.sheet)}>
        <div className="grab" aria-hidden="true" />
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <b style={{ flex: '1 1 auto', fontSize: 17, fontWeight: 600 }}>Khoá {nameOf(primary)} mới</b>
          <button type="button" className="tap" onClick={() => { setKeyOpen(false); setKeyError('') }} style={{ height: 40, border: 'none', background: 'transparent', color: 'var(--color-fg-3)', fontSize: 15 }}>Huỷ</button>
        </div>
        <input className="mfield mono" type="password" value={key} aria-label="Khoá API" autoComplete="off"
          onChange={(event) => setKey(event.target.value)} placeholder="Dán khoá API mới" />
        <span style={{ fontSize: 13, color: 'var(--color-fg-4)', lineHeight: 1.5 }}>Khoá lưu trên máy chủ, không hiện lại đầy đủ. Noverse thử kết nối trước khi lưu.</span>
        {!!keyError && <p className="mm-error" role="alert">{keyError}</p>}
        <button className="mbtn pri tap" type="button" disabled={keyBusy || !key.trim()} onClick={() => void saveKey()}>
          {keyBusy ? 'Đang thử kết nối…' : 'Thử và lưu'}
        </button>
      </section>
    </>}
  </>
}

const contextStyles = stylex.create({
  group: { marginInline: 16, minWidth: 0 },
  heading: { fontSize: 12.5, color: 'var(--color-fg-4)', paddingTop: 18, paddingInline: 30, paddingBottom: 8, letterSpacing: '.02em' },
  modelName: { fontSize: 12.5, fontFamily: 'var(--font-mono)' },
})

function MobileModelContext({ provider, model, disabled, toast }: { provider: string; model: string; disabled: boolean; toast: (text: string) => void }) {
  const admin = useSession().user?.role === 'admin'
  const resource = useModelContext(provider, model)
  const [draft, setDraft] = useState<ContextDraft | null>(null)
  const mutation = useAction(async (window: number | null) => {
    await saveModelContexts([{ provider, model, window }], admin)
    setDraft(null)
    toast(t.settingsModelsW.saved)
  })
  const info = resource.data
  if (!provider || !model) return null
  return <>
    <div {...stylex.props(contextStyles.heading)}>{t.settingsModelsW.contextFor} <span {...stylex.props(contextStyles.modelName)}>{model}</span></div>
    {info && <div {...stylex.props(contextStyles.group)}>
      <ModelContextChoices model={info} draft={draft ?? contextDraft(info)} onChange={setDraft} disabled={!admin || disabled || mutation.busy} variant="mobile" onCommit={window => void mutation.run(window)} />
    </div>}
    <ResourceNotice resource={resource} label="ngữ cảnh model" />
    {mutation.error != null && <p className="mm-error" role="alert">{mutation.error instanceof Error ? mutation.error.message : t.settingsModelsW.loadError}</p>}
    {!admin && <p className="mm-note">{t.settingsModelsW.applies}</p>}
  </>
}
