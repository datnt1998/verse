// Tuỳ chỉnh › Trợ lý AI — Settings.dc.html #models + 11-api-contract.md §6.
// Mặc định = provider/model/reasoning_effort cấp trên cùng; roles.<vai> = {provider, model, fallbacks[], reasoning_effort}.
// Model dùng chung cả app, chỉ quản trị đổi, lưu NGAY (không có thanh Lưu) — mỗi thao tác
// là một bản cấu hình hoàn chỉnh, lỗi thì trả về lựa chọn đã lưu và báo lỗi có cách xử lý.
// Trọng tài luôn dùng Mặc định nên không có hàng riêng (artboard).
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { AgentOrb, type AgentKind } from '@noverse/ui'
import { Button, Icon } from '@noverse/ui'
import { KitSelect, type Option } from '@noverse/ui'
import { getConfigModels, swapProvider, undoSwapProvider } from '../../../api/client'
import { useResource } from '../../../data'
import { saveModelContexts } from '../../../data/modelContext'
import { t } from '../../../i18n/vi'
import { toast } from '@noverse/ui'
import { ContextSelect } from './ContextSelect'
import { formatContextK } from '../../../lib/modelContext'
import type { Config } from '../../../api/types'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { mergeConfig, modelOptionId, modelOptions, setPath, thinkingLabel, thinkingSegments, type ModelOption } from '../lib/config'
import { providerName } from './connectLogic'
import { providerGroups } from './modelPick'
import { Page, SecCard, SectionHead } from '../parts'
import type { SettingsContext } from '../types'
import { hasOwn } from '../lib/labels'
import {
  issueAction, issueLevel, issueOf, issueShort, issueTech, issueText, issueTitle, quotaRetryText, retryClock, type ProviderIssue,
} from '../lib/providerError'

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
  fields: {
    display: 'grid',
    width: '100%',
    minWidth: 0,
    gridTemplateColumns: { default: 'minmax(0, 1fr) 200px', '@media (max-width: 800px)': 'minmax(0, 1fr)' },
    gap: 12,
    paddingLeft: { default: 42, '@media (max-width: 800px)': 0 },
    alignItems: 'end',
  },
  full: { gridColumn: '1 / -1' },
  field: { display: 'flex', flexDirection: 'column', gap: 5, minWidth: 0 },
  label: { color: color['--color-fg-4'], fontSize: 11.5 },
  defaultOrb: { width: 30, height: 30, borderRadius: '50%', backgroundColor: color['--color-surface-6'], display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, color: color['--color-fg-3'] },
  chip: { flexShrink: 0, height: 20, paddingInline: 7, borderRadius: 6, display: 'flex', alignItems: 'center', backgroundColor: kit['--kit-wash-5'], color: color['--color-fg-3'], fontSize: 11.5, whiteSpace: 'nowrap' },
  footer: { width: '100%', minHeight: 38, display: 'flex', alignItems: 'center', gap: 10, borderWidth: 0, borderRadius: 9, backgroundColor: 'transparent', color: color['--color-fg-2'], fontSize: 13, paddingBlock: 6, paddingLeft: 10, paddingRight: 8, cursor: 'pointer' },
  banner: { display: 'flex', flexDirection: 'column', gap: 12, padding: '16px 18px', borderRadius: 16, borderWidth: 1, borderStyle: 'solid' },
  bannerTop: { display: 'flex', alignItems: 'flex-start', gap: 12 },
  bannerIcon: { flex: '0 0 auto', width: 36, height: 36, borderRadius: 10, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  bannerCopy: { flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  bannerTitleRow: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  bannerTitle: { fontSize: 15, fontWeight: 600 },
  bannerText: { fontSize: 13, color: color['--color-fg-3'], lineHeight: 1.55 },
  bannerActions: { marginLeft: 48, display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  swapList: { marginLeft: 48, display: 'flex', flexDirection: 'column', padding: '4px 12px', borderRadius: 11, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'] },
  swapRow: { display: 'flex', alignItems: 'center', gap: 8, paddingBlock: 5, fontSize: 12, minWidth: 0 },
  tech: { marginLeft: 48, padding: '8px 10px', borderRadius: 8, backgroundColor: color['--color-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], fontSize: 11.5, color: color['--color-fg-3'], fontFamily: 'var(--font-mono)' },
  warnRow: { display: 'flex', alignItems: 'center', gap: 8, padding: '6px 10px', borderRadius: 9, fontSize: 12.5 },
  levelPicker: { display: 'flex', flexWrap: 'wrap', gap: 2, padding: 3, borderRadius: 10, backgroundColor: color['--color-surface-1'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], width: 'max-content', maxWidth: '100%' },
  level: { height: 28, paddingInline: 10, borderWidth: 0, borderRadius: 7, backgroundColor: 'transparent', color: color['--color-fg-3'], fontSize: 12.5, cursor: 'pointer', whiteSpace: 'nowrap' },
  levelOn: { backgroundColor: color['--color-surface-7'], color: color['--color-fg'], fontWeight: 500 },
  resetNote: { fontSize: 12, color: color['--color-warn'] },
})

/** Khoá cấu hình của một hàng: Mặc định nằm ở cấp trên cùng, vai trò nằm trong `roles.<vai>`. */
const pathOf = (key: RoleKey, field: 'provider' | 'model' | 'reasoning_effort' | 'fallbacks') => (key === 'default' ? field : `roles.${key}.${field}`)

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

/** Đồng hồ cho câu quota: chỉ chạy khi thật sự có mốc thử lại. */
function useNow(active: boolean, ms = 30_000): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!active) return
    const timer = window.setInterval(() => setNow(Date.now()), ms)
    return () => window.clearInterval(timer)
  }, [active, ms])
  return now
}

/** Nút mức suy luận: 1–8 lựa chọn, tự xuống dòng trên màn hẹp thay vì cắt mất mức. */
function LevelPicker({ value, options, label, disabled, onPick }: {
  value: string; options: { id: string; label: string }[]; label: string; disabled: boolean; onPick: (id: string) => void
}) {
  return (
    <div role="radiogroup" aria-label={label} {...stylex.props(sx.levelPicker)}>
      {options.map((option) => {
        const on = option.id === value
        return (
          <button key={option.id || 'auto'} type="button" role="radio" aria-checked={on} disabled={disabled}
            onClick={() => onPick(option.id)}
            className={`hov-ghost ${stylex.props(sx.level, on && sx.levelOn).className ?? ''}`}>
            {option.label}
          </button>
        )
      })}
    </div>
  )
}

export function RolesSection({ ctx }: { ctx: SettingsContext }) {
  // Ô Model nhóm theo nhà cung cấp, chỉ gồm model đã chọn (bản nháp + hiệu lực để thấy ngay model vừa chọn).
  const merged = mergeConfig(ctx.effective, ctx.draft)
  const options = modelOptions(merged)
  // Gồm cả lựa chọn vừa đổi: chọn model là danh mục (mô tả, ngữ cảnh, mức suy luận) được tải ngay.
  const defaults = roleValue(merged, 'default')
  const defaultId = defaults.provider && defaults.model ? `${defaults.provider}/${defaults.model}` : ''
  const selected = ROLES.map((role) => roleValue(merged, role.key))
  const providers = [...new Set([...Object.keys(merged.providers ?? {}), ...selected.map((role) => role.provider), defaults.provider].filter(Boolean))].sort()
  // Tải metadata của MỌI model đã cấu hình (không chỉ model đang chọn) để đổi model là biết ngay
  // mức suy luận hợp lệ — nếu chỉ tải model đang chọn, lần đổi đầu sẽ không phát hiện mức cũ sai.
  const requested = providers.map((provider) => ({
    provider,
    models: [...new Set([
      ...(merged.providers?.[provider]?.models ?? []).map((model) => model.name),
      ...selected.filter((role) => (role.provider || defaults.provider) === provider).map((role) => role.model || defaults.model || ''),
    ].filter(Boolean))].sort(),
  }))
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
  /** Mức suy luận model thật sự nhận (chưa tải xong → undefined = chỉ “Tự động”). */
  const levelsFor = (provider: string, model: string): string[] | undefined =>
    catalog.data?.[provider]?.find((entry) => entry.name === model)?.thinking
  const canEdit = ctx.admin && ctx.canEdit
  const presetName = (provider: string) => providerName(provider, ctx.presets?.providers.find((preset) => preset.id === provider) ?? null)

  // Ghi ngay: một bản cấu hình chung hoàn chỉnh cho mỗi thao tác (xem index.tsx › saveAppModels).
  const writeRole = (key: RoleKey, provider: string, model: string, effort: string) => ctx.saveAppModels((config) => {
    let next = setPath(config, pathOf(key, 'provider'), provider)
    next = setPath(next, pathOf(key, 'model'), model)
    return setPath(next, pathOf(key, 'reasoning_effort'), effort)
  })
  /** Mức vừa bị mất khi đổi model (hiện ghi chú “đã về Tự động”). */
  const [resetNotes, setResetNotes] = useState<Partial<Record<RoleKey, string>>>({})
  /** Hàng đang mở bảng chọn model từ nút “Đổi model” của dải cảnh báo. */
  const [openRow, setOpenRow] = useState<{ key: RoleKey; nonce: number } | null>(null)

  // Options nhóm theo nhà cung cấp (label = tên hiển thị); nhóm rỗng có hàng “Chưa chọn model · Chọn”.
  const groupsOf = providerGroups(merged).map((group) => ({
    provider: group.provider,
    label: presetName(group.provider),
    models: group.models,
  }))

  const pickModel = (spec: RoleSpec, id: string) => {
    setOpenRow(null)
    // “Theo Mặc định”: ghi rỗng (không xoá khoá) để hàng đổi ngay; configForSave bỏ hẳn vai rỗng.
    if (id === INHERIT) {
      void writeRole(spec.key, '', '', '')
      setResetNotes((notes) => ({ ...notes, [spec.key]: undefined }))
      return
    }
    // Hàng “Chưa chọn model · Chọn” của nhóm rỗng: mở thẳng bảng của nhà cung cấp ở Kết nối AI.
    if (id.startsWith(PICK_PREFIX)) { ctx.goSection('connect', id.slice(PICK_PREFIX.length)); return }
    const [provider, ...rest] = id.split('/')
    const model = rest.join('/')
    const current = roleValue(merged, spec.key).effort
    const levels = levelsFor(provider, model)
    // Đổi model → mức đang chọn có thể không còn hợp lệ: về Tự động và nói rõ mức cũ.
    const invalid = !!levels && !!current && !levels.includes(current)
    void writeRole(spec.key, provider, model, invalid ? '' : current)
    setResetNotes((notes) => ({ ...notes, [spec.key]: invalid ? current : undefined }))
    toast(`Đã đổi model ${spec.name} sang ${nameOf(id)} · áp dụng từ lượt kế tiếp`, 'ok')
  }
  const pickEffort = (spec: RoleSpec, effort: string) => {
    setResetNotes((notes) => ({ ...notes, [spec.key]: undefined }))
    void ctx.saveAppModels((config) => {
      let next = config
      // Vai đang “Theo Mặc định” mà chọn mức riêng: phải có provider/model của Mặc định.
      if (spec.key !== 'default') {
        const mine = roleValue(config, spec.key)
        const base = roleValue(config, 'default')
        if (!mine.provider && !mine.model && base.provider && base.model) {
          next = setPath(next, pathOf(spec.key, 'provider'), base.provider)
          next = setPath(next, pathOf(spec.key, 'model'), base.model)
        }
      }
      return setPath(next, pathOf(spec.key, 'reasoning_effort'), effort)
    })
    toast(`Mức suy luận ${spec.name}: ${thinkingLabel(effort)} · áp dụng từ lượt kế tiếp`, 'ok')
  }

  // Nhà cung cấp dùng chung đang lỗi: một thẻ duy nhất, nói rõ loại lỗi và việc nên làm.
  const primary = defaults.provider
  const issue = issueOf(ctx.response, primary)
  const badName = presetName(primary)
  const using = ROLES.filter((spec) => (roleValue(merged, spec.key).provider || primary) === primary)
  const alternative = options.find((option) => option.provider !== primary && !issueOf(ctx.response, option.provider))
  const [swapping, setSwapping] = useState(false)
  const [swapped, setSwapped] = useState<{ undoToken: string; count: number; rows: { role: string; from: string; to: string; resetEffort: string }[] } | null>(null)
  const [techOpen, setTechOpen] = useState(false)
  const now = useNow(issue?.kind === 'quota')

  const doSwap = async () => {
    if (!alternative || !ctx.response || swapping) return
    setSwapping(true)
    try {
      const result = await swapProvider(primary, alternative.provider, alternative.model, ctx.response.version)
      setSwapped({
        undoToken: result.undoToken,
        count: result.swapped.length,
        // máy chủ tự về Tự động các mức model mới không hỗ trợ → hiện ghi chú đã về Tự động.
        rows: result.swapped.map((row) => ({
          role: row.role,
          from: nameOf(`${row.from.provider}/${row.from.model}`),
          to: nameOf(`${row.to.provider}/${row.to.model}`),
          resetEffort: row.from.reasoning_effort && row.from.reasoning_effort !== row.to.reasoning_effort ? row.from.reasoning_effort : '',
        })),
      })
      ctx.refreshConfig()
      toast(`Đã chuyển ${result.swapped.length} trợ lý · áp dụng từ lượt kế tiếp`, 'ok')
      if (result.warning) toast(result.warning, 'warn')
    } catch (error) {
      toast(error instanceof Error ? error.message : 'Không chuyển được nhà cung cấp — thử lại.', 'err')
    } finally {
      setSwapping(false)
    }
  }
  const undoSwap = async () => {
    if (!swapped || swapping) return
    setSwapping(true)
    try {
      const restored = await undoSwapProvider(swapped.undoToken)
      setSwapped(null)
      ctx.refreshConfig()
      toast('Đã hoàn tác · áp dụng từ lượt kế tiếp', 'ok')
      if (restored.warning) toast(restored.warning, 'warn')
    } catch (error) {
      toast(error instanceof Error ? error.message : 'Không hoàn tác được — thử lại.', 'err')
    } finally {
      setSwapping(false)
    }
  }

  const issueColor = (kind: ProviderIssue['kind']) => (kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)')
  const bannerVisible = !!swapped || (!!issue && using.length > 0)
  const bannerKind = issue?.kind ?? 'key'
  const bannerTitle = swapped ? `Đã chuyển ${swapped.count} trợ lý sang nhà cung cấp khác` : issue ? issueTitle(badName, issue.kind) : ''
  const bannerText = swapped
    ? 'Áp dụng từ lượt viết kế tiếp ở mọi truyện. Bạn vẫn có thể hoàn tác để dùng lại lựa chọn trước.'
    : issue && using.length > 0
      ? `${using.length} trợ lý đang dùng model của ${badName}: ${using.map((spec) => spec.name).join(', ')}. ${issue.kind === 'quota'
        ? quotaRetryText(issue.retryAt, now)
        : alternative ? 'Đổi một chạm sang nhà cung cấp còn dùng được, hoặc ' : ''}${issue.kind === 'quota' ? '' : issue.kind === 'key' ? 'nhập khoá mới.' : 'gia hạn rồi đăng nhập lại.'}`
      : ''

  return (
    <Page maxWidth={860}>
      <SectionHead title="Trợ lý AI" note="Dùng chung cho cả app — mọi dự án, mọi truyện. Đổi là có hiệu lực từ lượt viết kế tiếp. Trọng tài dùng model “Mặc định”." />
      {!ctx.admin && (
        <div role="note" style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', borderRadius: 14, background: 'var(--color-surface-1)', border: '1px solid var(--line-1)' }}>
          <Icon name="lock" size={16} style={{ color: 'var(--color-fg-3)', flex: '0 0 auto' }} />
          <span style={{ flex: '1 1 auto', fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.5 }}>Model dùng chung cho mọi người nên chỉ quản trị đổi được. Bạn vẫn xem được trợ lý nào đang dùng model nào.</span>
        </div>
      )}
      {!!ctx.appModelsError && (
        <div role="alert" style={{ display: 'flex', alignItems: 'flex-start', gap: 10, padding: '12px 14px', borderRadius: 14, background: 'color-mix(in srgb, var(--color-err) 8%, transparent)', border: '1px solid color-mix(in srgb, var(--color-err) 30%, transparent)' }}>
          <Icon name="warning-2" size={17} style={{ color: 'var(--color-err)', marginTop: 2, flex: '0 0 auto' }} />
          <span style={{ flex: '1 1 auto', fontSize: 12.5, color: 'var(--color-fg-2)', lineHeight: 1.5 }}>{ctx.appModelsError}</span>
          <button type="button" className="hov-ghost" onClick={ctx.retryAppModels}
            style={{ height: 28, padding: '0 10px', borderRadius: 8, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12, whiteSpace: 'nowrap' }}>
            Thử lại
          </button>
        </div>
      )}
      {bannerVisible && (
        <section role="status" {...stylex.props(sx.banner)}
          style={swapped
            ? { borderColor: 'color-mix(in srgb, var(--color-ok) 25%, transparent)', background: 'var(--color-surface-2)' }
            : issue?.kind === 'quota'
              ? { borderColor: 'color-mix(in srgb, var(--color-warn) 30%, transparent)', background: 'color-mix(in srgb, var(--color-warn) 4%, transparent)' }
              : { borderColor: 'color-mix(in srgb, var(--color-err) 32%, transparent)', background: 'color-mix(in srgb, var(--color-err) 4%, transparent)' }}>
          <div {...stylex.props(sx.bannerTop)}>
            <span aria-hidden="true" {...stylex.props(sx.bannerIcon)} style={{ background: `color-mix(in srgb, ${swapped ? 'var(--color-ok)' : issueColor(bannerKind)} 12%, transparent)`, color: swapped ? 'var(--color-ok)' : issueColor(bannerKind) }}>
              <Icon name={swapped ? 'tick-circle' : 'warning-2'} size={18} />
            </span>
            <div {...stylex.props(sx.bannerCopy)}>
              <div {...stylex.props(sx.bannerTitleRow)}>
                <b {...stylex.props(sx.bannerTitle)}>{bannerTitle}</b>
                {!swapped && issue && <span className="st-uchip" style={{ background: `color-mix(in srgb, ${issueColor(issue.kind)} 14%, transparent)`, color: issue.kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)' }}>{issueLevel(issue.kind)}</span>}
              </div>
              <span {...stylex.props(sx.bannerText)}>{bannerText}</span>
            </div>
          </div>
          {!swapped && issue && issue.kind !== 'quota' && using.length > 0 && (
            <div {...stylex.props(sx.swapList)}>
              {using.map((spec) => {
                const from = roleValue(merged, spec.key)
                const fromId = from.provider && from.model ? `${from.provider}/${from.model}` : defaultId
                return (
                  <div key={spec.key} {...stylex.props(sx.swapRow)}>
                    <span style={{ color: 'var(--color-fg-3)', minWidth: 104 }}>{spec.name}</span>
                    <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--color-fg-4)', textDecoration: 'line-through' }}>{nameOf(fromId)}</span>
                    <Icon name="arrow-right-2" size={13} style={{ color: 'var(--color-fg-4)', flex: '0 0 auto' }} />
                    <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--color-fg)' }}>{alternative ? nameOf(`${alternative.provider}/${alternative.model}`) : '—'}</span>
                  </div>
                )
              })}
            </div>
          )}
          {swapped && (
            <div {...stylex.props(sx.swapList)}>
              {swapped.rows.map((row) => (
                <div key={row.role} {...stylex.props(sx.swapRow)}>
                  <span style={{ color: 'var(--color-fg-3)', minWidth: 104 }}>{t.settingsModelsW.swapRoles[row.role] ?? 'Trợ lý'}</span>
                  <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--color-fg-4)', textDecoration: 'line-through' }}>{row.from}</span>
                  <Icon name="arrow-right-2" size={13} style={{ color: 'var(--color-fg-4)', flex: '0 0 auto' }} />
                  <span style={{ fontFamily: 'var(--font-mono)', color: 'var(--color-fg)' }}>{row.to}</span>
                  {!!row.resetEffort && (
                    <span style={{ color: 'var(--color-warn)' }}>· Mức {thinkingLabel(row.resetEffort)} không có — đã về Tự động</span>
                  )}
                </div>
              ))}
            </div>
          )}
          <div {...stylex.props(sx.bannerActions)}>
            {!swapped && issue && issue.kind !== 'quota' && canEdit && using.length > 0 && alternative && (
              <Button size="sm" variant="primary" disabled={swapping} onClick={() => void doSwap()}>
                Chuyển {using.length} trợ lý sang {presetName(alternative.provider)}
              </Button>
            )}
            {!swapped && issue && issue.kind !== 'quota' && canEdit && (
              <Button size="sm" onClick={() => ctx.goSection('connect', primary)}>
                {issueAction(issue.kind)} {badName}
              </Button>
            )}
            {swapped && (
              <Button size="sm" disabled={swapping} onClick={() => void undoSwap()}>Hoàn tác</Button>
            )}
            <span style={{ flex: 1 }} />
            {!swapped && issue && (
              <Button size="sm" variant="quiet" aria-expanded={techOpen} onClick={() => setTechOpen((open) => !open)}>
                Chi tiết kỹ thuật
              </Button>
            )}
          </div>
          {!swapped && issue && techOpen && <div {...stylex.props(sx.tech)}>{issueTech(primary, issue)}</div>}
        </section>
      )}
      <SecCard style={{ gap: 0 }}>
        {ROLES.map((spec, index) => {
          const mine = roleValue(merged, spec.key)
          const shown = mine.provider && mine.model ? `${mine.provider}/${mine.model}` : defaultId
          const [shownProvider, ...shownRest] = shown.split('/')
          const shownModel = shownRest.join('/')
          const effort = mine.effort
          const levels = levelsFor(shownProvider, shownModel)
          const known = options.some((option) => modelOptionId(option) === shown)
          const info = catalog.data?.[shownProvider]?.find((model) => model.name === shownModel)
          const rowIssue = issueOf(ctx.response, shownProvider)
          const reset = resetNotes[spec.key]
          const picker: Option[] = [
            ...(spec.key !== 'default' && mine.provider && mine.model ? [{ value: INHERIT, label: 'Theo Mặc định' }] : []),
            ...groupsOf.flatMap((group): Option[] => group.models.length
              ? group.models.map((option) => {
                  const meta = catalog.data?.[option.provider]?.find((model) => model.name === option.model)
                  return { value: modelOptionId(option), label: nameOf(modelOptionId(option)), group: group.label, description: meta?.description, meta: meta ? <span {...stylex.props(sx.chip)}>{formatContextK(meta.contextWindow)}</span> : undefined }
                })
              // Nhà cung cấp chưa chọn model: tiêu đề nhóm + hàng “Chưa chọn model · Chọn” mở bảng của nhà cung cấp đó.
              : [{ value: `${PICK_PREFIX}${group.provider}`, label: 'Chưa chọn model · Chọn', group: group.label, icon: <Icon name="add" size={15} style={{ color: 'var(--kit-warn-text)' }} /> }]),
            ...(shown && !known ? [{ value: shown, label: nameOf(shown), group: presetName(shownProvider), description: info?.description, meta: info && <span {...stylex.props(sx.chip)}>{formatContextK(info.contextWindow)}</span> }] : []),
            ...(!shown ? [{ value: '', label: '—' }] : []),
          ]
          const cells = stylex.props(sx.row, index === 0 && sx.first)
          const levelCount = (levels ?? []).filter((level) => level !== '').length
          return (
            <div key={spec.key} data-role={spec.key} className={cells.className} style={cells.style}>
              <div {...stylex.props(sx.top)}>
                {spec.orb ? <AgentOrb agent={spec.orb} size={30} /> : <span aria-hidden="true" {...stylex.props(sx.defaultOrb)}><Icon name="layer" size={15} /></span>}
                <div className="st-stx" style={{ flexDirection: 'row', alignItems: 'baseline', gap: 8, flexWrap: 'wrap' }}><b>{spec.name}</b><span>{spec.does}</span></div>
              </div>
              {!!rowIssue && (
                <div role="status" {...stylex.props(sx.warnRow)}
                  style={{ marginLeft: 42, background: rowIssue.kind === 'quota' ? 'color-mix(in srgb, var(--color-warn) 8%, transparent)' : 'color-mix(in srgb, var(--color-err) 8%, transparent)', color: rowIssue.kind === 'quota' ? 'var(--color-warn)' : 'var(--color-err)' }}>
                  <Icon name="warning-2" size={14} style={{ flex: '0 0 auto' }} />
                  <span style={{ flex: '1 1 auto', minWidth: 0 }}>{presetName(shownProvider)} {issueShort(rowIssue.kind)}{rowIssue.kind === 'quota' ? ` · tự thử lại lúc ${retryClock(rowIssue.retryAt) || '—'}` : ' · trợ lý này không chạy được'}</span>
                  {canEdit && rowIssue.kind !== 'quota' && (
                    <button type="button" className="hov-ghost" onClick={() => setOpenRow({ key: spec.key, nonce: (openRow?.nonce ?? 0) + 1 })}
                      style={{ height: 26, padding: '0 8px', borderRadius: 7, border: '1px solid var(--line-4)', background: 'transparent', color: 'var(--color-fg)', fontSize: 12, whiteSpace: 'nowrap' }}>
                      Đổi model
                    </button>
                  )}
                </div>
              )}
              <div {...stylex.props(sx.fields)}>
                <div {...stylex.props(sx.field)}><span {...stylex.props(sx.label)}>{t.settingsModelsW.model}</span>
                  <KitSelect key={`${spec.key}-${openRow?.key === spec.key ? openRow.nonce : 0}`} defaultOpen={openRow?.key === spec.key} inlineDescription aria-label={`Model cho ${spec.name}`} value={shown} options={picker} disabled={!canEdit} onChange={(id) => pickModel(spec, id)}
                    footer={<button type="button" data-select-action {...stylex.props(sx.footer)} onClick={() => ctx.goSection('connect')}><Icon name="add" size={15} style={{ color: 'var(--color-accent)' }} />Chọn thêm model hoặc nhà cung cấp ở Kết nối AI</button>} />
                </div>
                <div {...stylex.props(sx.field)}><span {...stylex.props(sx.label)}>{t.settingsModelsW.context}</span>
                  {info ? <ContextSelect key={shown} provider={shownProvider} model={info} label={`${t.settingsModelsW.context} ${spec.name}`} admin={ctx.admin} onSave={(window) => saveContext(shownProvider, shownModel, window)} />
                    : <span role="status" {...stylex.props(sx.label)}>{catalog.error ? t.settingsModelsW.loadError : catalog.initialLoading ? t.settingsModelsW.loading : t.settingsModelsW.missing}{!!catalog.error && <button type="button" {...stylex.props(sx.footer)} onClick={() => void catalog.refresh()}>{t.settingsModelsW.retry}</button>}</span>}
                </div>
                <div {...stylex.props(sx.field, sx.full)}>
                  <span {...stylex.props(sx.label)}>Mức suy luận · {levelCount > 0 ? `${levelCount} mức của ${shownModel}` : `${shownModel || 'Model này'} không có mức suy luận`}</span>
                  <LevelPicker label={`Mức suy luận của ${spec.name}`} value={effort} disabled={!canEdit}
                    options={thinkingSegments(levels, effort)} onPick={(next) => pickEffort(spec, next)} />
                  {!!reset && <span {...stylex.props(sx.resetNote)}>Mức {thinkingLabel(reset)} không có ở {shownModel} — đã về Tự động.</span>}
                </div>
              </div>
            </div>
          )
        })}
      </SecCard>
      <FallbackCard ctx={ctx} options={options} nameOf={nameOf} presetName={presetName} />
    </Page>
  )
}

/** “Khi model chính lỗi”: một danh sách thứ tự, áp cho Kiến trúc sư · Người viết · Biên tập viên (mỗi vai lưu `roles.<vai>.fallbacks`). */
function FallbackCard({ ctx, options, nameOf, presetName }: {
  ctx: SettingsContext; options: ModelOption[]; nameOf: (id: string) => string; presetName: (provider: string) => string
}) {
  const [adding, setAdding] = useState(false)
  const merged = mergeConfig(ctx.effective, ctx.draft)
  const listOf = (config: Config) => FALLBACK_ROLES.map((key) => config.roles?.[key]?.fallbacks ?? []).find((list) => list.length > 0) ?? []
  const list = listOf(merged)
  const ids = list.map((item) => `${item.provider}/${item.model}`)
  const defaults = roleValue(merged, 'default')
  const canEdit = ctx.admin && ctx.canEdit

  const write = (next: { provider: string; model: string }[]) => {
    void ctx.saveAppModels((config) => {
      let doc = config
      for (const key of FALLBACK_ROLES) {
        const mine = roleValue(config, key)
        const base = roleValue(config, 'default')
        if (next.length > 0 && !mine.provider && !mine.model && base.provider && base.model) {
          doc = setPath(doc, pathOf(key, 'provider'), base.provider)
          doc = setPath(doc, pathOf(key, 'model'), base.model)
        }
        doc = setPath(doc, pathOf(key, 'fallbacks'), next.length > 0 ? next : undefined)
      }
      return doc
    })
  }
  // Artboard: dự phòng phải ở nhà cung cấp khác — hết hạn gói/khoá hỏng thường
  // xảy ra theo cả nhà cung cấp, dự phòng cùng chỗ cũng hỏng theo.
  const candidates = options.filter((option) => option.provider !== defaults.provider && !ids.includes(modelOptionId(option)))
  return (
    <SecCard>
      <div className="st-srow">
        <div className="st-stx"><b>Model dự phòng</b><span>Dùng chung cho cả app. Khi nhà cung cấp chính hết hạn gói, khoá hỏng hoặc tạm hết lượt quá 2 phút, Noverse tự chuyển sang dự phòng theo thứ tự và báo trên Bàn viết. Dự phòng phải ở nhà cung cấp khác.</span></div>
        <span style={{ display: 'flex', gap: 6, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
          {list.map((item, index) => {
            const issue = issueOf(ctx.response, item.provider)
            const label = nameOf(`${item.provider}/${item.model}`)
            return (
              <span key={`${item.provider}/${item.model}`} className="st-uchip" style={{ background: issue ? 'color-mix(in srgb, var(--color-err) 12%, transparent)' : 'var(--color-surface-2)', color: issue ? 'var(--color-err)' : 'var(--color-fg-2)' }}>
                {index + 1} · {label}{issue ? ` · ${issueShort(issue.kind)}` : ''}
                {canEdit && (
                  <button type="button" className="hov-ghost" aria-label={`Bỏ dự phòng ${label}`}
                    onClick={() => write(list.filter((_, position) => position !== index))}
                    style={{ width: 16, height: 16, border: 'none', borderRadius: 4, background: 'transparent', color: 'var(--color-fg-4)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', padding: 0 }}>
                    <Icon name="close-circle" size={12} />
                  </button>
                )}
              </span>
            )
          })}
          {canEdit && !adding && candidates.length > 0 && <button type="button" className="st-tpl" onClick={() => setAdding(true)}>+ Thêm</button>}
          {canEdit && adding && (
            <span style={{ width: 180 }} onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setAdding(false) }}>
              <KitSelect autoFocus defaultOpen aria-label="Thêm model dự phòng" value="" placeholder="Chọn model…" height={30}
                options={candidates.map((option) => ({ value: modelOptionId(option), label: nameOf(modelOptionId(option)), group: presetName(option.provider) }))}
                onChange={(value) => {
                  const [provider, ...rest] = value.split('/')
                  if (provider) write([...list, { provider, model: rest.join('/') }])
                  setAdding(false)
                }} />
            </span>
          )}
          {list.length === 0 && !adding && <span style={{ fontSize: 12.5, color: 'var(--color-fg-4)' }}>{candidates.length > 0 ? 'Chưa có dự phòng — máy chủ báo lỗi là dừng viết.' : 'Chưa có nhà cung cấp khác để làm dự phòng.'}</span>}
        </span>
      </div>
    </SecCard>
  )
}
