// /model [vai] — bảng “Model cho từng trợ lý” (CmdResults.dc.html view=model, 14-jobs-dock §2).
// Trái: 4 vai kèm model · mức đang dùng (chấm tím = chưa lưu). Phải: Nhà cung cấp → Model
// (ngữ cảnh) → Mức suy luận (chỉ các mức model hỗ trợ). Lưu gọi POST …/models + …/thinking.
import { useEffect, useMemo, useState } from 'react'
import { getModels, setModel, setThinking } from '../../api/client'
import type { ModelCatalogModel, ModelRoleKey } from '../../api/types'
import { setData, useAction, useResource } from '../../data'
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
import { Segmented } from '@noverse/ui'
import './cmdresults.css'
import {
  ROLE_META, clampThinking, formatContext, pickOf, samePick, savedSummary, thinkingFor, thinkingLabel,
  type RolePick,
} from './logic'
import { errorText } from './parts'
import { ModelContextChoices } from '../../ui/ModelContextChoices'
import { PanelRefreshBar } from '../../ui/PanelRefreshBar'

const ROLE_ORDER: ModelRoleKey[] = ['default', 'architect', 'writer', 'editor']
const isRole = (value: string | undefined): value is ModelRoleKey => !!value && (ROLE_ORDER as string[]).includes(value)

export function ModelPanel({ project, story, role, onClose }: {
  project: string; story: string
  /** `/model writer` → mở sẵn vai đó */
  role?: string
  onClose: () => void
}) {
  const resourceKey = ['config', 'story', project, story, 'models'] as const
  const resource = useResource(resourceKey, ({ signal }) => getModels(project, story, signal))
  const data = resource.data
  const loadError = resource.error ? errorText(resource.error, 'Không tải được danh sách model') : null
  const [active, setActive] = useState<ModelRoleKey>(isRole(role) ? role : 'writer')
  const [picks, setPicks] = useState<Partial<Record<ModelRoleKey, RolePick>>>({})
  const [saved, setSaved] = useState<string | null>(null)
  const admin = useSession().user?.role === 'admin'
  const [contexts, setContexts] = useState<Record<string, { provider: string; model: string; draft: ContextDraft; before: number | null; automatic: number }>>({})
  const contextChanges = Object.values(contexts).filter(entry => (entry.draft.mode === 'auto' ? null : contextThousands(entry.draft.value)) !== entry.before)
  const dirty = contextChanges.length

  const providers = useMemo(() => data?.providers ?? [], [data])
  const baseline = useMemo(() => {
    const out: Partial<Record<ModelRoleKey, RolePick>> = {}
    for (const r of data?.roles ?? []) out[r.role] = pickOf(r)
    return out
  }, [data])
  const current = (key: ModelRoleKey): RolePick => picks[key] ?? baseline[key] ?? { provider: '', model: '', thinking: '' }
  const changed = ROLE_ORDER.filter((key) => baseline[key] && picks[key] && !samePick(picks[key], baseline[key]))

  const pick = current(active)
  const setPick = (patch: Partial<RolePick>) => {
    setPicks((previous) => ({ ...previous, [active]: clampThinking(providers, { ...pick, ...patch }) }))
    setSaved(null)
  }

  const provider = providers.find((p) => p.provider === pick.provider)
  const models: ModelCatalogModel[] = useMemo(() => {
    const list = provider?.models ?? []
    if (!pick.model || list.some((m) => m.name === pick.model)) return list
    const known = data?.roles.find((r) => r.provider === pick.provider && r.model === pick.model)
    return [{ name: pick.model, contextWindow: 0, contextSource: 'default', registryWindow: 0, thinking: known?.thinkingOptions ?? [''] }, ...list]
  }, [provider, pick.model, pick.provider, data])
  const levels = models.find((m) => m.name === pick.model)?.thinking ?? thinkingFor(providers, pick.provider, pick.model)
  const context = useModelContext(pick.provider, pick.model)
  const contextId = JSON.stringify([pick.provider, pick.model])
  const draft = context.data ? contexts[contextId]?.draft ?? contextDraft(context.data) : null
  const changeContext = (draft: ContextDraft) => {
    if (!admin || !context.data) return
    setContexts(previous => ({ ...previous, [contextId]: { provider: pick.provider, model: pick.model, draft, before: previous[contextId] ? previous[contextId].before : context.data!.contextSource === 'model_config' ? context.data!.contextWindow : null, automatic: context.data!.automaticWindow } }))
    setSaved(null)
  }

  const saveAction = useAction(async () => {
    if ((changed.length === 0 && dirty === 0) || contextChanges.some(entry => entry.draft.mode === 'custom' && contextThousands(entry.draft.value) === null)) return
    for (const key of changed) {
      const next = picks[key]!
      const before = baseline[key]!
      if (next.provider !== before.provider || next.model !== before.model) await setModel(project, story, key, next.provider, next.model)
      if (next.thinking !== before.thinking || next.provider !== before.provider || next.model !== before.model) await setThinking(project, story, key, next.thinking)
    }
    await saveModelContexts(contextChanges.map(entry => ({ provider: entry.provider, model: entry.model, window: entry.draft.mode === 'auto' ? null : contextThousands(entry.draft.value) })), admin)
    const summary = changed.length ? savedSummary(changed.map((key) => ({ role: key, pick: picks[key]! }))) : t.settingsModelsW.saved
    const fresh = await getModels(project, story)
    setData(resourceKey, fresh)
    setPicks({})
    setContexts({})
    setSaved(summary)
    toast.ok(summary)
    return true
  }, { error: 'Không lưu được model', invalidate: [['config', 'story', project, story]] })
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
  const canSave = (changed.length > 0 || dirty > 0) && !saveAction.busy && !contextChanges.some(entry => entry.draft.mode === 'custom' && contextThousands(entry.draft.value) === null)

  return (
    <Dialog onClose={close} width={900} style={{ height: 600, maxHeight: '100%', overflow: 'hidden' }}>
      <div className="cr-dlg-in" style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
          <h2 id="cr-model-h" style={{ margin: 0, flex: 1, fontSize: 16, fontWeight: 600 }}>Model cho từng trợ lý</h2>
          <span className="mono" style={{ fontSize: 12, color: '#7E7E8A' }}>/model</span>
          <Kbd>Esc</Kbd>
        </div>
        <PanelRefreshBar resourceKeys={[resource.key]} />
        {loadError && data && <div role="alert" style={{ padding: '8px 18px', color: '#F6CB8C', fontSize: 12 }}>Không tải lại được · <button onClick={() => void resource.refresh()}>Thử lại</button></div>}

        {!data ? (
          <div style={{ flex: '1 1 auto', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 10, color: '#A0A0AB', fontSize: 13 }}>
            {loadError ? <><Icon name="warning-2" size={16} style={{ color: '#F6CB8C' }} />{loadError}<button onClick={() => void resource.refresh()}>Thử lại</button></> : <><span className="cr-spinner" />Đang tải model…</>}
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
                    </span>
                    {changed.includes(key) && <span title="Chưa lưu" style={{ width: 7, height: 7, borderRadius: '50%', background: '#C9A6F5', flex: '0 0 auto' }} />}
                  </button>
                )
              })}
              <span style={{ flex: 1 }} />
              <span style={{ fontSize: 11.5, color: '#7E7E8A', padding: 6 }}>“Mặc định” áp cho mọi trợ lý chưa chọn riêng.</span>
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
                  {providers.map((p) => {
                    const sel = p.provider === pick.provider
                    return (
                      <RadioCard key={p.provider} checked={sel} mark="none"
                        onChange={() => setPick({ provider: p.provider, model: p.models[0]?.name ?? '', thinking: '' })}
                        title={p.ready ? undefined : 'Chưa có khoá API hoặc chưa đăng nhập'}
                        style={{
                          height: 32, padding: '0 12px', borderRadius: 9, border: `1px solid ${sel ? 'rgba(201,166,245,0.4)' : 'rgba(255,255,255,0.1)'}`,
                          background: sel ? '#26262A' : 'transparent', color: sel ? '#EDEDEF' : '#A0A0AB', fontSize: 12.5,
                          display: 'flex', alignItems: 'center', gap: 7, whiteSpace: 'nowrap',
                        }}>
                        <span aria-hidden="true" style={{ width: 7, height: 7, borderRadius: '50%', background: p.ready ? '#A6EDC1' : '#5E5E68' }} />
                        {p.label}
                      </RadioCard>
                    )
                  })}
                </div>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                <span className="lbl" style={{ fontSize: 10.5 }}>Model</span>
                <div role="radiogroup" aria-label="Model" style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
                  {models.length === 0 && <span style={{ fontSize: 12.5, color: '#7E7E8A' }}>Nhà cung cấp này chưa khai báo model nào.</span>}
                  {models.map((m) => {
                    const sel = m.name === pick.model
                    const users = ROLE_ORDER.filter((key) => baseline[key]?.provider === pick.provider && baseline[key]?.model === m.name)
                    const pendingContext = contexts[JSON.stringify([pick.provider, m.name])]
                    const window = pendingContext ? pendingContext.draft.mode === 'custom' ? contextThousands(pendingContext.draft.value) ?? m.contextWindow : pendingContext.automatic : sel ? context.data?.contextWindow ?? m.contextWindow : m.contextWindow
                    return (
                      <RadioCard key={m.name} className="cr-crow" checked={sel} dotSize={16} onChange={() => setPick({ model: m.name })} style={{
                        alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 11,
                        border: `1px solid ${sel ? 'rgba(201,166,245,0.35)' : 'rgba(255,255,255,0.07)'}`,
                        background: sel ? 'rgba(201,166,245,0.07)' : 'transparent',
                      }}>
                        <span style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column' }}>
                          <span className="mono" style={{ fontSize: 13 }}>{m.name}</span>
                          <span style={{ fontSize: 12, color: '#8E8E99' }}>
                            {users.length > 0
                              ? `Đang dùng cho ${users.map((key) => ROLE_META[key].label).join(', ')}`
                              : m.thinking.length > 1 ? `Hỗ trợ ${m.thinking.length - 1} mức suy luận` : 'Không chỉnh mức suy luận'}
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
                <span className="lbl" style={{ fontSize: 10.5 }}>Mức suy luận</span>
                <Segmented label="Mức suy luận" value={pick.thinking}
                  onChange={(thinking) => setPick({ thinking })}
                  options={levels.map((level) => ({
                    id: level, label: thinkingLabel(level), style: { fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis' },
                  }))}
                  style={{ gap: 4 }} />
                <span style={{ fontSize: 12, color: '#7E7E8A' }}>
                  {pick.thinking === ''
                    ? 'Mặc định: dùng mức của nhà cung cấp hoặc của vai “Mặc định”.'
                    : 'Suy luận cao hơn: chương chặt chẽ hơn nhưng chậm và tốn hạn mức hơn.'}
                </span>
              </div>
            </div>
          </div>
        )}

        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 18px', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
          {saved
            ? <span className="cr-toast" role="status" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5, color: '#A6EDC1', minWidth: 0 }}><Icon name="tick-circle" size={15} />{saved}</span>
            : <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{[changed.length ? `${changed.length} trợ lý có thay đổi chưa lưu` : '', dirty ? t.settingsModelsW.dirty(dirty) : ''].filter(Boolean).join(' · ') || 'Chưa có thay đổi'}</span>}
          <span style={{ flex: 1 }} />
          <button className="cr-tbtn" type="button" onClick={close} style={{
            height: 36, padding: '0 14px', borderRadius: 9, border: '1px solid rgba(255,255,255,0.1)', background: 'transparent', color: '#EDEDEF', fontSize: 13, whiteSpace: 'nowrap',
          }}>Đóng</button>
          <button className="cr-primary" type="button" onClick={save} disabled={!canSave} aria-busy={saving} title="Lưu thay đổi (⌘S / Ctrl+S)" style={{
            height: 36, padding: '0 16px', borderRadius: 9, border: 'none', background: canSave ? '#EDEDEF' : '#222225',
            color: canSave ? '#0B0B0D' : '#7E7E8A', fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap', opacity: 1,
            display: 'flex', alignItems: 'center', gap: 8,
          }}>{saving && <span className="cr-spinner" style={{ borderTopColor: '#0B0B0D' }} />}Lưu thay đổi</button>
        </div>
      </div>
    </Dialog>
  )
}
