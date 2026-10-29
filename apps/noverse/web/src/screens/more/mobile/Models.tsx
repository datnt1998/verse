import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { useSession } from '../../../api/session'
import { saveModelContexts, useModelContext } from '../../../data/modelContext'
import { contextDraft, type ContextDraft } from '../../../lib/modelContext'
import { t } from '../../../i18n/vi'
import { ModelContextChoices } from '../../../ui/ModelContextChoices'
import { getModels, setModel, setThinking } from '../../../api/client'
import type { ModelRoleKey, ModelsResponse } from '../../../api/types'
import { cache, invalidate } from '../../../data/cache'
import { keys } from '../../../data/keys'
import { useAction } from '../../../data/useAction'
import { useResource } from '../../../data/useResource'
import { AgentOrb } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Radio } from '@noverse/ui'
import { Segmented } from '@noverse/ui'
import { ROLE_META, clampThinking, formatContext, thinkingLabel } from '../../cmdresults/logic'
import { ResourceNotice } from './parts'

export const MODEL_ROLES: ModelRoleKey[] = ['default', 'architect', 'writer', 'editor']
export const modelsKey = (project: string, story: string) => [...keys.config('story', project, story), 'models']

export function Models({ project, story, active, onPick, disabled, toast }: {
  project: string; story: string; active: ModelRoleKey | null
  onPick: (role: ModelRoleKey) => void; disabled: boolean; toast: (text: string) => void
}) {
  const key = modelsKey(project, story)
  const resource = useResource(key, () => getModels(project, story))
  const data = resource.data
  const role = data?.roles.find((item) => item.role === active)
  const mutation = useAction(async (change: { provider: string; model: string } | { thinking: string }) => {
    if (!active || !role) return
    if ('thinking' in change) await setThinking(project, story, active, change.thinking)
    else {
      const next = clampThinking(data?.providers ?? [], { ...role, ...change })
      await setModel(project, story, active, next.provider, next.model)
      if (next.thinking !== role.thinking) await setThinking(project, story, active, next.thinking)
    }
    toast('thinking' in change ? `Đã đổi mức suy luận · ${thinkingLabel(change.thinking)}` : `${ROLE_META[active].label} → ${change.model}`)
  }, {
    invalidate: [keys.config('story', project, story), keys.story(project, story, 'snapshot')],
    optimistic: (change) => {
      const before = cache.get<ModelsResponse>(key).data
      if (before && active) cache.setData(key, { ...before, roles: before.roles.map((item) => item.role === active ? { ...item, ...('thinking' in change ? change : clampThinking(before.providers, { ...item, ...change })) } : item) })
      return () => { cache.restore(key, before); void invalidate(key) }
    },
  })
  const selectedModel = data?.providers.find((provider) => provider.provider === role?.provider)?.models.find((model) => model.name === role?.model)
  const levels = selectedModel?.thinking.length ? selectedModel.thinking : role?.thinkingOptions.length ? role.thinkingOptions : ['']
  const blocked = disabled || mutation.busy || !!resource.error

  return <>
    <ResourceNotice resource={resource} label="model trợ lý" />
    {mutation.error != null && <p className="mm-error" role="alert">{mutation.error instanceof Error ? mutation.error.message : 'Không đổi được model. Vui lòng thử lại.'}</p>}
    {mutation.pending && <p className="mm-note" role="status">Đang lưu lựa chọn…</p>}
    {!active && data && <>
      <div className="mm-heading">Chạm để đổi</div>
      <div className="mm-group">{MODEL_ROLES.map((key) => {
        const meta = ROLE_META[key]
        const value = data.roles.find((item) => item.role === key)
        return <button className="mm-row" type="button" key={key} disabled={!value} onClick={() => onPick(key)}>
          {meta.orb ? <AgentOrb agent={meta.orb} size={30} /> : <span className="mm-default"><Icon name="layer" size={16} /></span>}
          <span className="mm-copy"><span>{meta.label}</span><span className="mm-sub">{value?.model
            ? <><span className="mm-mono">{value.model}</span> · {thinkingLabel(value.thinking)}</>
            : 'Chưa chọn model'}</span></span>
          <Icon name="arrow-right-2" size={16} className="mm-chevron" />
        </button>
      })}</div>
      <p className="mm-note">“Mặc định” áp cho trợ lý chưa chọn riêng. Đổi có hiệu lực từ lượt viết sau.</p>
    </>}
    {active && data && <>
      {role && !selectedModel && (role.model
        ? <p className="mm-note">Đang dùng: <span className="mm-mono">{role.model}</span>{role.provider && ` · ${role.provider}`}. Model này chưa có trong danh sách nhà cung cấp.</p>
        : <p className="mm-note">Chưa chọn model cho trợ lý này. Chọn một model bên dưới.</p>)}
      {data.providers.map((provider) => <section key={provider.provider}>
        <div className="mm-heading">{provider.label}{!provider.ready && <span>Chưa kết nối</span>}</div>
        <div className="mm-group" role="radiogroup" aria-label={provider.label}>
          {provider.models.map((model) => {
            const selected = role?.provider === provider.provider && role.model === model.name
            return <Radio key={model.name} className="mm-row" mark="tick" markPosition="end" checked={selected}
              disabled={blocked || !provider.ready || !role} aria-busy={mutation.pending}
              onChange={() => { if (!selected) void mutation.run({ provider: provider.provider, model: model.name }) }}
              label={<span className="mm-mono" style={{ fontSize: 14.5 }}>{model.name}</span>}
              description={<span style={{ fontSize: 13 }}>{formatContext(model.contextWindow) || 'Model đã cấu hình'}{!provider.ready ? ' · Kết nối trên máy tính' : ''}</span>}
              style={{ padding: '8px 14px', margin: 0, gap: 12, minHeight: 54, fontSize: 15, lineHeight: 1.35 }} />
          })}
          {!provider.models.length && <p className="mm-empty">Chưa có model. Thêm model trên máy tính.</p>}
        </div>
      </section>)}
      {!data.providers.length && <p className="mm-note">Chưa có nhà cung cấp AI. Thiết lập nhà cung cấp trên máy tính.</p>}
      {role && <>
        <MobileModelContext key={`${role.provider}/${role.model}`} provider={role.provider} model={role.model} disabled={disabled} toast={toast} />
        <div className="mm-heading">Mức suy luận</div>
        <Segmented label="Mức suy luận" value={role.thinking} onChange={(level) => { if (role.thinking !== level) void mutation.run({ thinking: level }) }}
          columns={4} size={42} radius={10} thumbStyle={{ boxShadow: 'none' }}
          style={{ display: 'grid', gap: 4, padding: 4, borderRadius: 14, background: 'var(--color-card)', border: '1px solid var(--line-1)', margin: '0 16px' }}
          options={levels.map((level) => ({ id: level, label: thinkingLabel(level), disabled: blocked, style: { fontSize: 14 } }))} />
        {!levels.includes(role.thinking) && <p className="mm-note">Mức đang dùng: {thinkingLabel(role.thinking)}. Chọn một mức được model hỗ trợ.</p>}
      </>}
    </>}
    {disabled && <p className="mm-note">Bạn cần quyền viết và kết nối mạng để đổi model hoặc mức suy luận.</p>}
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
