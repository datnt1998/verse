// Tuỳ chỉnh › Trợ lý AI — Settings.dc.html #models + 11-api-contract.md §6.
// Mặc định = provider/model/reasoning_effort cấp trên cùng; roles.<vai> = {provider, model, fallbacks[], reasoning_effort}.
// Trọng tài luôn dùng Mặc định nên không có hàng riêng (artboard).
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { AgentOrb, type AgentKind } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { KitSelect, type Option } from '@noverse/ui'
import { getConfigModels } from '../../../api/client'
import { useResource } from '../../../data'
import { saveModelContexts } from '../../../data/modelContext'
import { t } from '../../../i18n/vi'
import { toast } from '@noverse/ui'
import { ContextSelect } from './ContextSelect'
import { formatContextK } from '../../../lib/modelContext'
import type { Config } from '../../../api/types'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { mergeConfig, modelOptionId, modelOptions, reasoningSegments, type ModelOption } from '../lib/config'
import { providerName } from './connectLogic'
import { providerGroups } from './modelPick'
import { OvrBadge, Page, SecCard, SectionHead, Segc } from '../parts'
import type { SettingsContext } from '../types'
import { RunCard } from './RunCard'

type RoleKey = 'default' | 'architect' | 'writer' | 'editor'
type RoleSpec = { key: RoleKey; name: string; does: string; orb: AgentKind | null }

const ROLES: RoleSpec[] = [
  { key: 'default', name: 'Mặc định', does: 'Trọng tài & trợ lý chưa chọn riêng', orb: null },
  { key: 'architect', name: 'Kiến trúc sư', does: 'Dàn ý, kế hoạch chương', orb: 'architect' },
  { key: 'writer', name: 'Người viết', does: 'Viết từng chương', orb: 'writer' },
  { key: 'editor', name: 'Biên tập viên', does: 'Soát và chấm chương', orb: 'editor' },
]
const FALLBACK_ROLES: RoleKey[] = ['architect', 'writer', 'editor']
const INHERIT = '__inherit__'
/** Tiền tố value của hàng điều hướng “Chưa chọn model · Chọn” trong ô Model. */
const PICK_PREFIX = 'pick:'

const sx = stylex.create({
  row: {
    display: 'flex',
    flexDirection: 'column',
    gap: 10,
    paddingTop: 16,
    paddingBottom: 16,
    borderTopWidth: 1,
    borderTopStyle: 'solid',
    borderTopColor: line['--line-1'],
  },
  first: { borderTopWidth: 0, paddingTop: 4 },
  top: { display: 'flex', alignItems: 'center', gap: 12, width: '100%', minWidth: 0 },
  fields: { display: 'grid', width: '100%', minWidth: 0, gridTemplateColumns: { default: 'minmax(0, 1fr) auto 176px', '@media (max-width: 800px)': 'minmax(0, 1fr)' }, gap: 12, paddingLeft: { default: 42, '@media (max-width: 800px)': 0 }, alignItems: 'end' },
  field: { display: 'flex', flexDirection: 'column', gap: 5, minWidth: 0 },
  label: { color: color['--color-fg-4'], fontSize: 11.5 },
  scope: { marginLeft: 'auto', flexShrink: 0 },
  defaultOrb: { width: 30, height: 30, borderRadius: '50%', backgroundColor: color['--color-surface-6'], display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, color: color['--color-fg-3'] },
  chip: { flexShrink: 0, height: 20, paddingInline: 7, borderRadius: 6, display: 'flex', alignItems: 'center', backgroundColor: kit['--kit-wash-5'], color: color['--color-fg-3'], fontSize: 11.5, whiteSpace: 'nowrap' },
  footer: { width: '100%', minHeight: 38, display: 'flex', alignItems: 'center', gap: 10, borderWidth: 0, borderRadius: 9, backgroundColor: 'transparent', color: color['--color-fg-2'], fontSize: 13, paddingBlock: 6, paddingLeft: 10, paddingRight: 8, cursor: 'pointer' },
})

/** Khoá cấu hình của một hàng: Mặc định nằm ở cấp trên cùng, vai trò nằm trong `roles.<vai>`. */
const pathOf = (key: RoleKey, field: 'provider' | 'model' | 'reasoning_effort' | 'fallbacks') => (key === 'default' ? field : `roles.${key}.${field}`)
const rowPaths = (key: RoleKey) => (['provider', 'model', 'reasoning_effort'] as const).map((field) => pathOf(key, field))

function roleValue(config: Config, key: RoleKey) {
  if (key === 'default') return { provider: config.provider ?? '', model: config.model ?? '', effort: config.reasoning_effort ?? '' }
  const role = config.roles?.[key]
  return { provider: role?.provider ?? '', model: role?.model ?? '', effort: role?.reasoning_effort ?? '' }
}

/** Tên ngắn của model; trùng tên giữa hai nhà cung cấp thì thêm tên nhà cung cấp để phân biệt. */
export function shortNames(options: ModelOption[]): Map<string, string> {
  const count = new Map<string, number>()
  for (const option of options) count.set(option.model, (count.get(option.model) ?? 0) + 1)
  return new Map(options.map((option) => [modelOptionId(option), (count.get(option.model) ?? 0) > 1 ? `${option.model} (${option.provider})` : option.model]))
}

export function RolesSection({ ctx }: { ctx: SettingsContext }) {
  // Ô Model nhóm theo nhà cung cấp, chỉ gồm model đã chọn (bản nháp + hiệu lực để thấy ngay model vừa chọn).
  const merged = mergeConfig(ctx.effective, ctx.draft)
  const options = modelOptions(merged)
  const selected = ROLES.map((role) => roleValue(ctx.effective, role.key))
  const providers = [...new Set([...Object.keys(merged.providers ?? {}), ...selected.map((role) => role.provider)].filter(Boolean))].sort()
  const requested = providers.map((provider) => ({ provider, models: [...new Set(selected.filter((role) => (role.provider || ctx.effective.provider) === provider).map((role) => role.model || ctx.effective.model || '').filter(Boolean))].sort() }))
  const catalog = useResource(['settings-model-catalog', JSON.stringify(requested)], async () => {
    const entries = await Promise.all(requested.map(async ({ provider, models }) => [provider, (await getConfigModels(provider, models)).models] as const))
    return Object.fromEntries(entries)
  }, { enabled: providers.length > 0 })
  const saveContext = async (provider: string, model: string, window: number | null) => {
    if (!ctx.admin) return
    await saveModelContexts([{ provider, model, window }], ctx.admin)
    ctx.refreshConfig()
    toast(t.settingsModelsW.saved, 'ok')
  }
  const names = shortNames(options)
  const nameOf = (id: string) => names.get(id) ?? id.split('/').slice(1).join('/') ?? id
  const defaults = roleValue(ctx.effective, 'default')
  const defaultId = defaults.provider && defaults.model ? `${defaults.provider}/${defaults.model}` : ''

  // Options nhóm theo nhà cung cấp (label = tên hiển thị); nhóm rỗng có hàng “Chưa chọn model · Chọn”.
  const groupsOf = providerGroups(merged).map((group) => ({
    provider: group.provider,
    label: providerName(group.provider, ctx.presets?.providers.find((preset) => preset.id === group.provider) ?? null),
    models: group.models,
  }))
  /** Vai trò chỉ hợp lệ khi có đủ provider + model (BuildConfig kiểm sau khi trộn): khi mới tạo, chép model đang hiệu lực. */
  const seedRole = (key: RoleKey) => {
    if (key === 'default') return
    const eff = roleValue(ctx.effective, key)
    const mine = roleValue(ctx.draft, key)
    if (!mine.provider && !eff.provider && defaults.provider) ctx.setField(pathOf(key, 'provider'), defaults.provider)
    if (!mine.model && !eff.model && defaults.model) ctx.setField(pathOf(key, 'model'), defaults.model)
  }
  const pickModel = (key: RoleKey, id: string) => {
    if (id === INHERIT) { ctx.resetField(pathOf(key, 'provider')); ctx.resetField(pathOf(key, 'model')); return }
    // Hàng “Chưa chọn model · Chọn” của nhóm rỗng: mở thẳng bảng của nhà cung cấp ở Kết nối AI.
    if (id.startsWith(PICK_PREFIX)) { ctx.goSection('connect', id.slice(PICK_PREFIX.length)); return }
    const [provider, ...rest] = id.split('/')
    ctx.setField(pathOf(key, 'provider'), provider)
    ctx.setField(pathOf(key, 'model'), rest.join('/'))
  }
  const pickEffort = (key: RoleKey, effort: string) => {
    if (effort) seedRole(key)
    ctx.setField(pathOf(key, 'reasoning_effort'), effort || undefined)
  }
  const reset = (key: RoleKey) => rowPaths(key).forEach((path) => ctx.resetField(path))

  return (
    <Page maxWidth={860}>
      <SectionHead title="Trợ lý AI" note="Chọn model, mức suy nghĩ và ngữ cảnh cho từng trợ lý. Trọng tài luôn dùng model “Mặc định”." />
      <SecCard style={{ gap: 0 }}>
        {ROLES.map((spec, index) => {
          const eff = roleValue(ctx.effective, spec.key)
          const mine = roleValue(ctx.draft, spec.key)
          const own = mine.provider && mine.model ? `${mine.provider}/${mine.model}` : ''
          const shown = eff.provider && eff.model ? `${eff.provider}/${eff.model}` : defaultId
          const effort = mine.effort || eff.effort
          const known = options.some((option) => modelOptionId(option) === shown)
          const ovr = ctx.ovr(rowPaths(spec.key))
          const info = catalog.data?.[eff.provider || defaults.provider]?.find((model) => model.name === (eff.model || defaults.model))
          const picker: Option[] = [
            ...(ctx.scope === 'global' && spec.key !== 'default' && own ? [{ value: INHERIT, label: 'Theo Mặc định' }] : []),
            ...groupsOf.flatMap((group): Option[] => group.models.length
              ? group.models.map((option) => {
                  const meta = catalog.data?.[option.provider]?.find((model) => model.name === option.model)
                  return { value: modelOptionId(option), label: nameOf(modelOptionId(option)), group: group.label, description: meta?.description, meta: meta ? <span {...stylex.props(sx.chip)}>{formatContextK(meta.contextWindow)}</span> : undefined }
                })
              // Nhà cung cấp chưa chọn model: tiêu đề nhóm + hàng “Chưa chọn model · Chọn” mở bảng của nhà cung cấp đó.
              : [{ value: `${PICK_PREFIX}${group.provider}`, label: 'Chưa chọn model · Chọn', group: group.label, icon: <Icon name="add" size={15} style={{ color: 'var(--kit-warn-text)' }} /> }]),
            ...(shown && !known ? [{ value: shown, label: nameOf(shown), group: providerName(eff.provider || defaults.provider, ctx.presets?.providers.find((preset) => preset.id === (eff.provider || defaults.provider)) ?? null), description: info?.description, meta: info && <span {...stylex.props(sx.chip)}>{formatContextK(info.contextWindow)}</span> }] : []),
            ...(!shown ? [{ value: '', label: '—' }] : []),
          ]
          const cells = stylex.props(sx.row, index === 0 && sx.first)
          return (
            <div key={spec.key} data-role={spec.key} className={cells.className} style={cells.style}>
              <div {...stylex.props(sx.top)}>
                {spec.orb ? <AgentOrb agent={spec.orb} size={30} /> : <span aria-hidden="true" {...stylex.props(sx.defaultOrb)}><Icon name="layer" size={15} /></span>}
                <div className="st-stx" style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}><b>{spec.name}</b><span>{spec.does}</span></div>
                <div {...stylex.props(sx.scope)}><OvrBadge ovr={ovr} scope={ctx.scope} width={100} disabled={!ctx.canEdit} onReset={() => reset(spec.key)} /></div>
              </div>
              <div {...stylex.props(sx.fields)}>
                <div {...stylex.props(sx.field)}><span {...stylex.props(sx.label)}>{t.settingsModelsW.model}</span>
                  <KitSelect inlineDescription aria-label={`Model cho ${spec.name}`} value={shown} options={picker} disabled={!ctx.canEdit} onChange={(id) => pickModel(spec.key, id)}
                    footer={<button type="button" data-select-action {...stylex.props(sx.footer)} onClick={() => ctx.goSection('connect')}><Icon name="add" size={15} style={{ color: 'var(--color-accent)' }} />Chọn thêm model hoặc nhà cung cấp ở Kết nối AI</button>} />
                </div>
                <div {...stylex.props(sx.field)}><span {...stylex.props(sx.label)}>{t.settingsModelsW.effort}</span>
                  <Segc stretch label="Mức suy nghĩ" value={effort} disabled={!ctx.canEdit} options={reasoningSegments(effort)} onPick={(next) => pickEffort(spec.key, next)} />
                </div>
                <div {...stylex.props(sx.field)}><span {...stylex.props(sx.label)}>{t.settingsModelsW.context}</span>
                  {info ? <ContextSelect key={shown} provider={eff.provider || defaults.provider} model={info} label={`${t.settingsModelsW.context} ${spec.name}`} admin={ctx.admin} onSave={(window) => saveContext(eff.provider || defaults.provider, eff.model || defaults.model, window)} />
                    : <span role="status" {...stylex.props(sx.label)}>{catalog.error ? t.settingsModelsW.loadError : catalog.initialLoading ? t.settingsModelsW.loading : t.settingsModelsW.missing}{!!catalog.error && <button type="button" {...stylex.props(sx.footer)} onClick={() => void catalog.refresh()}>{t.settingsModelsW.retry}</button>}</span>}
                </div>
              </div>
            </div>
          )
        })}
      </SecCard>
      <RunCard ctx={ctx} />
      <FallbackCard ctx={ctx} options={options} nameOf={nameOf} />
    </Page>
  )
}

/** “Khi model chính lỗi”: một danh sách thứ tự, áp cho Kiến trúc sư · Người viết · Biên tập viên (mỗi vai lưu `roles.<vai>.fallbacks`). */
function FallbackCard({ ctx, options, nameOf }: { ctx: SettingsContext; options: ModelOption[]; nameOf: (id: string) => string }) {
  const [adding, setAdding] = useState(false)
  const listOf = (config: Config) => FALLBACK_ROLES.map((key) => config.roles?.[key]?.fallbacks ?? []).find((list) => list.length > 0) ?? []
  const list = listOf(ctx.draft).length > 0 ? listOf(ctx.draft) : listOf(ctx.effective)
  const ids = list.map((item) => `${item.provider}/${item.model}`)
  const defaults = roleValue(ctx.effective, 'default')

  const write = (next: { provider: string; model: string }[]) => {
    for (const key of FALLBACK_ROLES) {
      const eff = roleValue(ctx.effective, key)
      const mine = roleValue(ctx.draft, key)
      if (next.length > 0) {
        if (!mine.provider && !eff.provider && defaults.provider) ctx.setField(pathOf(key, 'provider'), defaults.provider)
        if (!mine.model && !eff.model && defaults.model) ctx.setField(pathOf(key, 'model'), defaults.model)
      }
      ctx.setField(pathOf(key, 'fallbacks'), next.length > 0 ? next : undefined)
    }
  }
  const candidates = options.filter((option) => !ids.includes(modelOptionId(option)))
  return (
    <SecCard>
      <div className="st-srow">
        <div className="st-stx"><b>Khi model chính lỗi</b><span>Tự chuyển sang model dự phòng theo thứ tự, không dừng truyện.</span></div>
        <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
          {list.map((item, index) => (
            <span key={`${item.provider}/${item.model}`} className="st-uchip" style={{ background: 'rgba(255,255,255,0.06)', color: '#C9C9D1' }}>
              {index + 1} · {nameOf(`${item.provider}/${item.model}`)}
              {ctx.canEdit && (
                <button type="button" className="hov-ghost" aria-label={`Bỏ dự phòng ${nameOf(`${item.provider}/${item.model}`)}`}
                  onClick={() => write(list.filter((_, position) => position !== index))}
                  style={{ width: 16, height: 16, border: 'none', borderRadius: 4, background: 'transparent', color: '#8E8E99', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', padding: 0 }}>
                  <Icon name="close-circle" size={12} />
                </button>
              )}
            </span>
          ))}
          {ctx.canEdit && !adding && candidates.length > 0 && <button type="button" className="st-tpl" onClick={() => setAdding(true)}>+ Thêm</button>}
          {ctx.canEdit && adding && (
            <span style={{ width: 180 }} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setAdding(false) }}>
              <KitSelect autoFocus defaultOpen aria-label="Thêm model dự phòng" value="" placeholder="Chọn model…" height={30}
                options={candidates.map((option) => ({ value: modelOptionId(option), label: nameOf(modelOptionId(option)) }))}
                onChange={(value) => {
                  const [provider, ...rest] = value.split('/')
                  if (provider) write([...list, { provider, model: rest.join('/') }])
                  setAdding(false)
                }} />
            </span>
          )}
        </span>
      </div>
    </SecCard>
  )
}
