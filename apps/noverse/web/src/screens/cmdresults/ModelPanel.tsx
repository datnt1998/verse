// /model [vai] — bảng “Model cho từng trợ lý” (CmdResults.dc.html view=model, Gói AU).
// Trái: 4 vai kèm model · mức đang dùng (chấm tím = chưa lưu). Phải: Nhà cung cấp → Model
// (ngữ cảnh) → Mức suy luận (đúng các mức model hỗ trợ). Lưu ghi MỘT bản cấu hình chung cho cả app.
import { useEffect, useMemo, useState } from 'react'
import { getConfigModels, getConfigPresets } from '../../api/client'
import type { ModelRoleKey } from '../../api/types'
import { useAction, useResource } from '../../data'
import { appConfigDoc, putAppConfig, useAppConfig } from '../../data/appModels'
import { useSession } from '../../api/session'
import { saveModelContexts, useModelContext } from '../../data/modelContext'
import { contextDraft, contextThousands, type ContextDraft } from '../../lib/modelContext'
import { t } from '../../i18n/vi'
import { AgentOrb } from '@noverse/ui'
import { Kbd } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { confirmUnsavedLeave } from '../../ui/confirmPresets'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { RadioCard } from '@noverse/ui'
import './cmdresults.css'
import {
  ROLE_META, formatContext, pickOf, samePick, savedSummary, thinkingLabel,
  type RolePick,
} from './logic'
import { errorText } from './parts'
import { ModelContextChoices } from '../../ui/ModelContextChoices'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'
import { roleModel, setPath, thinkingSegments } from '../settings/lib/config'
import { providerName } from '../settings/sections/connectLogic'
import { issueOf, issueShort, retryClock } from '../settings/lib/providerError'

const ROLE_ORDER: ModelRoleKey[] = ['default', 'architect', 'writer', 'editor']
const isRole = (value: string | undefined): value is ModelRoleKey => !!value && (ROLE_ORDER as string[]).includes(value)
const pickOfRole = (config: Parameters<typeof roleModel>[0], key: ModelRoleKey): RolePick => {
  const value = roleModel(config, key)
  return { provider: value.provider, model: value.model, thinking: value.effort }
}

export function ModelPanel({ role, onClose }: {
  /** `/model writer` → mở sẵn vai đó */
  role?: string
  onClose: () => void
}) {
  const resource = useAppConfig()
  const response = resource.data
  const loadError = resource.error ? errorText(resource.error, 'Không tải được danh sách model') : null
  const [active, setActive] = useState<ModelRoleKey>(isRole(role) ? role : 'writer')
  const [picks, setPicks] = useState<Partial<Record<ModelRoleKey, RolePick>>>({})
  const [saved, setSaved] = useState<string | null>(null)
  const [reset, setReset] = useState<Partial<Record<ModelRoleKey, string>>>({})
  const admin = useSession().user?.role === 'admin'
  const [contexts, setContexts] = useState<Record<string, { provider: string; model: string; draft: ContextDraft; before: number | null; automatic: number }>>({})
  const contextChanges = Object.values(contexts).filter(entry => (entry.draft.mode === 'auto' ? null : contextThousands(entry.draft.value)) !== entry.before)
  const dirty = contextChanges.length

  const doc = appConfigDoc(response)
  const presets = useResource(['config-presets'], ({ signal }) => getConfigPresets(signal), { staleTime: 60_000 })
  const nameOf = (provider: string) => providerName(provider, presets.data?.providers.find((preset) => preset.id === provider) ?? null)
  const providers = useMemo(() => Object.entries(doc.providers ?? {}).map(([provider, entry]) => ({
    provider,
    label: nameOf(provider),
    ready: !!entry.api_key_set || !!response?.accounts?.[provider]?.connected || entry.type === 'ollama',
    models: (entry.models ?? []).filter((model) => model.name),
    issue: issueOf(response, provider),
  })), [doc, response, presets.data])
  const catalog = useResource(['app-model-catalog', JSON.stringify(providers.map((provider) => [provider.provider, provider.models.map((model) => model.name)]))], async () => {
    const entries = await Promise.all(providers.map(async ({ provider, models }) => [provider, (await getConfigModels(provider, models.map((model) => model.name))).models] as const))
    return Object.fromEntries(entries)
  }, { enabled: providers.length > 0 })
  const metaOf = (provider: string, model: string) => catalog.data?.[provider]?.find((entry) => entry.name === model)
  const levelsOf = (provider: string, model: string): string[] => metaOf(provider, model)?.thinking ?? ['']

  const baseline = useMemo(() => {
    const out: Partial<Record<ModelRoleKey, RolePick>> = {}
    for (const key of ROLE_ORDER) out[key] = pickOfRole(doc, key)
    return out
  }, [doc])
  const current = (key: ModelRoleKey): RolePick => picks[key] ?? baseline[key] ?? { provider: '', model: '', thinking: '' }
  const changed = ROLE_ORDER.filter((key) => { const next = picks[key]; const base = baseline[key]; return !!next && !!base && !samePick(next, base) })

  const pick = current(active)
  const setPick = (patch: Partial<RolePick>) => {
    setPicks((previous) => ({ ...previous, [active]: { ...pick, ...patch } }))
    setSaved(null)
  }
  const chooseModel = (provider: string, model: string) => {
    const previous = pick.thinking
    const levels = levelsOf(provider, model)
    const invalid = !!previous && !levels.includes(previous)
    setPick({ provider, model, thinking: invalid ? '' : previous })
    setReset((notes) => ({ ...notes, [active]: invalid ? previous : undefined }))
  }

  const provider = providers.find((item) => item.provider === pick.provider)
  const models = provider?.models ?? []
  const levels = levelsOf(pick.provider, pick.model)
  const context = useModelContext(pick.provider, pick.model)
  const contextId = JSON.stringify([pick.provider, pick.model])
  const draft = context.data ? contexts[contextId]?.draft ?? contextDraft(context.data) : null
  const changeContext = (next: ContextDraft) => {
    if (!admin || !context.data) return
    setContexts(previous => ({ ...previous, [contextId]: { provider: pick.provider, model: pick.model, draft: next, before: previous[contextId] ? previous[contextId].before : context.data!.contextSource === 'model_config' ? context.data!.contextWindow : null, automatic: context.data!.automaticWindow } }))
    setSaved(null)
  }

  const saveAction = useAction(async () => {
    if ((changed.length === 0 && dirty === 0) || contextChanges.some(entry => entry.draft.mode === 'custom' && contextThousands(entry.draft.value) === null)) return
    if (changed.length > 0) {
      if (!response) return
      let next = doc
      for (const key of changed) {
        const value = picks[key]!
        next = setPath(next, key === 'default' ? 'provider' : `roles.${key}.provider`, value.provider)
        next = setPath(next, key === 'default' ? 'model' : `roles.${key}.model`, value.model)
        next = setPath(next, key === 'default' ? 'reasoning_effort' : `roles.${key}.reasoning_effort`, value.thinking)
      }
      await putAppConfig(response, next)
    }
    await saveModelContexts(contextChanges.map(entry => ({ provider: entry.provider, model: entry.model, window: entry.draft.mode === 'auto' ? null : contextThousands(entry.draft.value) })), admin)
    const summary = changed.length ? `${savedSummary(changed.map((key) => ({ role: key, pick: picks[key]! })))} · cho cả app` : t.settingsModelsW.saved
    setPicks({})
    setContexts({})
    setReset({})
    setSaved(summary)
    void resource.refresh()
    toast.ok(summary)
    return true
  }, { error: 'Không lưu được model', invalidate: [['config', 'global']] })
  const saving = saveAction.pending
  const save = () => { if (changed.length > 0 || dirty > 0) void saveAction.run() }
  const close = () => {
    if (!changed.length && !dirty) { onClose(); return }
    void confirmUnsavedLeave({
      body: [changed.length ? `${changed.length} trợ lý có thay đổi chưa lưu` : '', dirty ? t.settingsModelsW.dirty(dirty) : ''].filter(Boolean).join(' · '),
      successMessage: undefined,
      onConfirm: async () => {
        if (await saveAction.run()) onClose()
        else throw new Error('Không lưu được model — thay đổi của bạn vẫn được giữ lại.')
      },
      secondary: { label: 'Bỏ thay đổi', onConfirm: onClose },
    })
  }
  useEffect(() => {
    const beforeUnload = (event: BeforeUnloadEvent) => { if (changed.length || dirty) event.preventDefault() }
    const keydown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') { event.preventDefault(); save() }
    }
    window.addEventListener('beforeunload', beforeUnload)
    window.addEventListener('keydown', keydown)
    return () => { window.removeEventListener('beforeunload', beforeUnload); window.removeEventListener('keydown', keydown) }
  })

  const meta = ROLE_META[active]
  const canSave = admin && (changed.length > 0 || dirty > 0) && !saveAction.busy && !contextChanges.some(entry => entry.draft.mode === 'custom' && contextThousands(entry.draft.value) === null)
  const levelCount = levels.filter((level) => level !== '').length
  const thinkSegments = thinkingSegments(levels, pick.thinking)
  const resetLevel = reset[active]

  return (
    <Dialog padBody={false} onClose={close} width={900} style={{ height: 600, maxHeight: '100%', overflow: 'hidden' }}>
      <div className="cr-dlg-in" style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
          <h2 id="cr-model-h" style={{ margin: 0, flex: 1, fontSize: 16, fontWeight: 600 }}>Model cho từng trợ lý</h2>
          <span className="mono" style={{ fontSize: 12, color: '#7E7E8A' }}>/model</span>
          <Kbd>Esc</Kbd>
        </div>
        <PanelRefreshBar resourceKeys={[resource.key]} />
        {loadError && response && <div role="alert" style={{ padding: '8px 18px', color: 'var(--color-warn)', fontSize: 12 }}>Không tải lại được · <button onClick={() => void resource.refresh()}>Thử lại</button></div>}

        {!response ? (
          <div style={{ flex: '1 1 auto', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10, color: '#A0A0AB', fontSize: 13 }}>
            {loadError ? <><Icon name="warning-2" size={16} style={{ color: 'var(--color-warn)' }} />{loadError}<button onClick={() => void resource.refresh()}>Thử lại</button></> : <><span className="cr-spinner" />Đang tải model…</>}
          </div>
        ) : (
          <div style={{ flex: '1 1 auto', minHeight: 0, display: 'flex' }}>
            <div role="tablist" aria-orientation="vertical" aria-label="Vai trợ lý" style={{
              flex: '0 0 270px', minWidth: 0, borderRight: '1px solid rgba(255,255,255,0.06)', padding: 10, display: 'flex', flexDirection: 'column', gap: 4,
            }}>
              {ROLE_ORDER.map((key) => {
                const sel = key === active
                const value = current(key)
                const roleMeta = ROLE_META[key]
                const rowIssue = issueOf(response, value.provider)
                return (
                  <button key={key} className="cr-crow" type="button" role="tab" aria-selected={sel} onClick={() => setActive(key)} style={{
                    display: 'flex', alignItems: 'center', gap: 12, padding: 10, borderRadius: 11,
                    border: `1px solid ${sel ? 'rgba(255,255,255,0.1)' : 'transparent'}`, background: sel ? '#1F1F23' : 'transparent',
                    color: '#EDEDEF', textAlign: 'left',
                  }}>
                    {roleMeta.orb ? <AgentOrb agent={roleMeta.orb} size={28} /> : (
                      <span aria-hidden="true" style={{ width: 28, height: 28, borderRadius: '50%', background: '#222225', display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>
                        <Icon name="layer" size={15} style={{ color: '#A0A0AB' }} />
                      </span>
                    )}
                    <span style={{ minWidth: 0, flex: '1 1 auto', display: 'flex', flexDirection: 'column' }}>
                      <span style={{ fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap' }}>{roleMeta.label}</span>
                      <span style={{ fontSize: 11.5, color: sel ? '#C9A6F5' : '#7E7E8A', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        <span className="mono">{value.model || '—'}</span> · {thinkingLabel(value.thinking)}
                      </span>
                      {!!rowIssue && <span style={{ fontSize: 11.5, color: rowIssue.kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        {nameOf(value.provider)} {issueShort(rowIssue.kind)}{rowIssue.kind === 'quota' ? ` · tự thử lại lúc ${retryClock(rowIssue.retryAt) || '—'}` : ''}
                      </span>}
                    </span>
                    {changed.includes(key) && <span title="Chưa lưu" style={{ width: 7, height: 7, borderRadius: '50%', background: '#C9A6F5', flex: '0 0 auto' }} />}
                  </button>
                )
              })}
              <span style={{ flex: 1 }} />
              <span style={{ fontSize: 11.5, color: '#7E7E8A', padding: 6, lineHeight: 1.5 }}>“Mặc định” áp cho trợ lý chưa chọn riêng. Đổi ở đây là đổi cho cả app, có hiệu lực từ lượt viết kế tiếp.</span>
            </div>

            <div className="scroll" role="tabpanel" aria-label={meta.label} style={{
              flex: '1 1 auto', minWidth: 0, overflowY: 'auto', padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: 18,
            }}>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                <span style={{ fontSize: 15, fontWeight: 600 }}>{meta.label}</span>
                <span style={{ fontSize: 12.5, color: '#A0A0AB' }}>{meta.job}</span>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <span className="lbl" style={{ fontSize: 10.5 }}>Nhà cung cấp</span>
                <div role="radiogroup" aria-label="Nhà cung cấp" style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  {providers.length === 0 && <span style={{ fontSize: 12.5, color: '#7E7E8A' }}>Chưa cấu hình nhà cung cấp nào — mở /config để thêm.</span>}
                  {providers.map((item) => {
                    const sel = item.provider === pick.provider
                    return (
                      <RadioCard key={item.provider} checked={sel} mark="none" disabled={!admin}
                        onChange={() => chooseModel(item.provider, item.models[0]?.name ?? '')}
                        title={item.ready ? undefined : 'Chưa có khoá API hoặc chưa đăng nhập'}
                        style={{
                          height: 32, padding: '0 12px', borderRadius: 9, border: `1px solid ${sel ? 'rgba(201,166,245,0.4)' : 'rgba(255,255,255,0.1)'}`,
                          background: sel ? '#26262A' : 'transparent', color: sel ? '#EDEDEF' : '#A0A0AB', fontSize: 12.5,
                          display: 'flex', alignItems: 'center', gap: 7, whiteSpace: 'nowrap',
                        }}>
                        <span aria-hidden="true" style={{ width: 7, height: 7, borderRadius: '50%', background: item.ready ? 'var(--color-ok)' : 'var(--color-fg-4)' }} />
                        {item.label}{item.issue ? ` · ${issueShort(item.issue.kind)}` : ''}
                      </RadioCard>
                    )
                  })}
                </div>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <span className="lbl" style={{ fontSize: 10.5 }}>Model</span>
                <div role="radiogroup" aria-label="Model" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {models.length === 0 && <span style={{ fontSize: 12.5, color: '#7E7E8A' }}>Nhà cung cấp này chưa khai báo model nào.</span>}
                  {models.map((model) => {
                    const sel = model.name === pick.model
                    const users = ROLE_ORDER.filter((key) => baseline[key]?.provider === pick.provider && baseline[key]?.model === model.name)
                    const pendingContext = contexts[JSON.stringify([pick.provider, model.name])]
                    const window = pendingContext ? pendingContext.draft.mode === 'custom' ? contextThousands(pendingContext.draft.value) ?? metaOf(pick.provider, model.name)?.contextWindow ?? 0 : pendingContext.automatic : sel ? context.data?.contextWindow ?? metaOf(pick.provider, model.name)?.contextWindow ?? 0 : metaOf(pick.provider, model.name)?.contextWindow ?? 0
                    const meta = metaOf(pick.provider, model.name)
                    return (
                      <RadioCard key={model.name} className="cr-crow" checked={sel} dotSize={16} disabled={!admin} onChange={() => chooseModel(pick.provider, model.name)} style={{
                        alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 11,
                        border: `1px solid ${sel ? 'rgba(201,166,245,0.35)' : 'rgba(255,255,255,0.07)'}`,
                        background: sel ? 'rgba(201,166,245,0.07)' : 'transparent',
                      }}>
                        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                          <span className="mono" style={{ fontSize: 13 }}>{model.name}</span>
                          <span style={{ fontSize: 12, color: '#8E8E99' }}>
                            {users.length > 0
                              ? `Đang dùng cho ${users.map((key) => ROLE_META[key].label).join(', ')}`
                              : ((meta?.thinking?.length ?? 0) > 1) ? `Hỗ trợ ${(meta?.thinking?.length ?? 1) - 1} mức suy luận` : 'Không chỉnh mức suy luận'}
                          </span>
                        </span>
                        <span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-4)', whiteSpace: 'nowrap' }}>{formatContext(window)}</span>
                      </RadioCard>
                    )
                  })}
                </div>
              </div>
              {context.data && draft && <section style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <span className="lbl" style={{ fontSize: 10.5 }}>{t.settingsModelsW.contextFor} <span className="mono" style={{ textTransform: 'none', letterSpacing: 0 }}>{pick.model}</span></span>
                <ModelContextChoices model={context.data} draft={draft} onChange={changeContext} disabled={!admin || saving} />
              </section>}
              {!!context.error && <p role="alert">{t.settingsModelsW.loadError}<button type="button" onClick={() => void context.refresh()}>{t.settingsModelsW.retry}</button></p>}

              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <span className="lbl" style={{ fontSize: 10.5 }}>Mức suy luận <span style={{ textTransform: 'none', letterSpacing: 0, fontWeight: 400 }}>· {levelCount > 0 ? `${levelCount} mức của ${pick.model}` : `${pick.model || 'Model này'} không có mức suy luận`}</span></span>
                <div role="radiogroup" aria-label="Mức suy luận" style={{ display: 'flex', flexWrap: 'wrap', gap: 4, padding: 3, borderRadius: 10, background: 'var(--color-surface-0)', border: '1px solid var(--line-3)' }}>
                  {thinkSegments.map((segment) => {
                    const sel = segment.id === pick.thinking
                    return (
                      <button key={segment.id || 'auto'} type="button" role="radio" aria-checked={sel} disabled={!admin}
                        onClick={() => { setPick({ thinking: segment.id }); setReset((notes) => ({ ...notes, [active]: undefined })) }}
                        style={{
                          flex: '1 1 64px', minWidth: 64, height: 30, padding: '0 8px', border: 'none', borderRadius: 7,
                          background: sel ? 'var(--color-surface-7)' : 'transparent', color: sel ? 'var(--color-fg)' : 'var(--color-fg-3)', fontSize: 12, whiteSpace: 'nowrap',
                        }}>
                        {segment.label}
                      </button>
                    )
                  })}
                </div>
                {!!resetLevel && <span style={{ fontSize: 12, color: 'var(--color-warn)' }}>Mức {thinkingLabel(resetLevel)} không có ở {pick.model} — đã về Tự động.</span>}
                <span style={{ fontSize: 12, color: '#7E7E8A' }}>
                  Tự động: dùng mức mặc định của model. Đổi là đổi cho cả app, hiện ngay ở danh sách trợ lý.
                </span>
              </div>
            </div>
          </div>
        )}

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 18px', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
          {saved
            ? <span className="cr-toast" role="status" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: '#A6EDC1', minWidth: 0 }}><Icon name="tick-circle" size={15} />{saved}</span>
            : <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{[changed.length ? `${changed.length} trợ lý có thay đổi chưa lưu` : '', dirty ? t.settingsModelsW.dirty(dirty) : ''].filter(Boolean).join(' · ') || (admin ? 'Chưa có thay đổi' : 'Chỉ quản trị đổi được model dùng chung.')}</span>}
          <span style={{ flex: 1 }} />
          <button className="cr-tbtn" type="button" onClick={close} style={{
            height: 36, padding: '0 14px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 13, whiteSpace: 'nowrap',
          }}>Đóng</button>
          <button className="cr-primary" type="button" onClick={save} disabled={!canSave} aria-busy={saving} title="Lưu cho cả app (⌘S / Ctrl+S)" style={{
            height: 36, padding: '0 16px', borderRadius: 9, border: 'none', background: canSave ? '#EDEDEF' : '#222225',
            color: canSave ? '#0B0B0D' : '#7E7E8A', fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap', opacity: 1,
            display: 'flex', alignItems: 'center', gap: 8,
          }}>{saving && <span className="cr-spinner" style={{ borderTopColor: 'var(--color-bg)' }} />}Lưu cho cả app</button>
        </div>
      </div>
    </Dialog>
  )
}
