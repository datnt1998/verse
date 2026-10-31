// Cột phải › “Đang áp dụng cho truyện” / “Mặc định của dự án”: gom giá trị hiệu lực từ cấu hình + quy tắc.
import { getConfig, getRulesEffective, putConfig } from '../../api/client'
import type { Config, ConfigResponse, ConfigScope, PackPreset } from '../../api/types'
import { cache, invalidate, keys, useResource } from '../../data'
import { findPack, packLabel } from '../settings/lib/packs'
import { reasoningSegments } from '../settings/lib/config'
import { usd } from '../more/mobile/storySetLogic'
import { configForSave, mergeConfig } from '../settings/lib/config'
import { effectiveKey } from '../settings/sections/rules/useRulesEditor'

/** PUT chỉ tệp của cấp đích; không sao chép giá trị hiệu lực từ cấp trên. */
export async function saveQuickConfig(scope: 'story' | 'project', project: string, story: string | undefined, previous: ConfigResponse, patch: Config): Promise<ConfigResponse> {
  const next = await putConfig(scope, configForSave(mergeConfig(previous.target.config, patch)), project, story, previous.version)
  cache.setData(keys.config(scope, project, story), next)
  void invalidate(['config'])
  return next
}

export type AppliedRow = { icon: string; label: string; value: string; own: boolean; section: string }


type RuleCounts = Partial<Record<ConfigScope, number>>

export function useRuleCounts(project: string, story: string | null): RuleCounts | null {
  const scope: ConfigScope = story ? 'story' : 'project'
  const resource = useResource(effectiveKey(scope, project, story ?? ''), ({ signal }) => getRulesEffective(scope, project, story ?? undefined, signal))
  if (!resource.data) return null
  const counts: RuleCounts = { story: resource.data.runtime?.length ?? 0 }
  for (const item of resource.data.items) counts[item.scope] = (counts[item.scope] ?? 0) + 1
  return counts
}

export function useEffectiveConfig(project: string, story: string | null): ConfigResponse | null {
  const scope: ConfigScope = story ? 'story' : 'project'
  const resource = useResource(keys.config(scope, project, story ?? undefined), ({ signal }) => getConfig(scope, project, story ?? undefined, signal))
  return resource.data ?? null
}

/** “suy nghĩ vừa” — nhãn mức suy nghĩ của Settings, viết thường. */
function thinkingText(level: string | undefined): string {
  const id = level ?? ''
  return `suy nghĩ ${(reasoningSegments(id).find((segment) => segment.id === id)?.label ?? id).toLowerCase()}`
}

export function appliedRows(config: ConfigResponse, counts: RuleCounts | null, story: boolean, packs?: PackPreset[]): AppliedRow[] {
  const effective: Config = config.effective
  const own = (path: string) => config.sources[path] === (story ? 'story' : 'project')
  const writer = effective.roles?.writer ?? effective.roles?.default
  const model = writer?.model || effective.model || ''
  const level = writer ? writer.reasoning_effort : effective.reasoning_effort
  const review = effective.advance_mode === 'review'
  const limit = effective.budget?.book_usd ?? 0
  // D2 — gói thể loại hiện “Gói <tên> · có chấm” thay cho tên kiểu viết (Outline.dc.html › applied).
  const pack = findPack(packs, effective.style)
  const styleValue = pack ? `Gói ${packLabel(pack)} · có chấm` : effective.style || 'Chung'
  const rulesValue = !counts ? '…'
    : story
      ? `${(counts.story ?? 0) + (counts.project ?? 0)} quy tắc`
      : `${counts.project ?? 0} quy tắc của dự án`
  if (story) {
    return [
      { icon: 'edit-2', label: 'Quy tắc', value: rulesValue, own: false, section: 'rules' },
      { icon: 'note-text', label: 'Văn phong', value: styleValue, own: own('style'), section: 'style' },
      { icon: 'book-1', label: 'Duyệt từng chương', value: review ? 'Bật' : 'Tắt', own: own('advance_mode'), section: 'writing' },
      { icon: 'cpu', label: 'Người viết', value: model ? `${model} · ${thinkingText(level)}` : 'Theo dự án', own: own('roles.writer') || own('model'), section: 'roles' },
    ]
  }
  const assistantOwn = own('roles.writer') || own('model')
  return [
    { icon: 'edit-2', label: 'Quy tắc', value: rulesValue, own: false, section: 'rules' },
    { icon: 'note-text', label: 'Văn phong', value: own('style') ? styleValue : 'Chung', own: own('style'), section: 'style' },
    { icon: 'book-1', label: 'Duyệt từng chương', value: review ? 'Bật cho truyện mới' : 'Tắt cho truyện mới', own: own('advance_mode'), section: 'writing' },
    { icon: 'cpu', label: 'Trợ lý AI', value: assistantOwn && model ? `${model} · ${thinkingText(level)}` : 'Theo Tuỳ chỉnh', own: assistantOwn, section: 'roles' },
    { icon: 'coin-1', label: 'Ngân sách mỗi truyện', value: limit > 0 ? usd(limit) : '—', own: own('budget.book_usd'), section: 'budget' },
  ]
}
