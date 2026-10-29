// Cột phải › “Đang áp dụng cho truyện” / “Mặc định của dự án”: gom giá trị hiệu lực từ cấu hình + quy tắc.
import { getConfig, getRuleFile, getRules } from '../../api/client'
import type { Config, ConfigResponse, ConfigScope, PackPreset, RulesScope } from '../../api/types'
import { keys, useResource, type Key } from '../../data'
import { findPack, packLabel } from '../settings/lib/packs'
import { reasoningSegments } from '../settings/lib/config'
import { usd } from '../more/mobile/storySetLogic'

export type AppliedRow = { icon: string; label: string; value: string; own: boolean; section: string }

const RULE_FILE = 'quy-tac.md'

/** Mỗi quy tắc là một dòng `- …` trong quy-tac.md (20-settings-rules.md §3.3). */
export function countRules(text: string): number {
  return text.split('\n').filter((line) => /^\s*-\s+\S/.test(line)).length
}

type RuleCounts = Partial<Record<ConfigScope, number>>

export function useRuleCounts(project: string, story: string | null): RuleCounts | null {
  const scope: ConfigScope = story ? 'story' : 'project'
  const key = [...keys.rules(scope, project, story ?? undefined), 'count'] as Key
  const resource = useResource<RuleCounts>(key, async ({ signal }) => {
    const chain: RulesScope[] = (await getRules(scope, project, story ?? undefined, signal)).chain
    const counts: RuleCounts = {}
    await Promise.all(chain.filter((level) => level.files.some((file) => file.name === RULE_FILE)).map(async (level) => {
      const file = await getRuleFile(level.scope, RULE_FILE, project, story ?? undefined, signal)
      counts[level.scope] = countRules(file.text)
    }))
    return counts
  })
  return resource.data ?? null
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
  const writer = effective.roles?.writer
  const model = writer?.model || effective.model || ''
  const level = writer ? writer.reasoning_effort : effective.reasoning_effort
  const review = effective.advance_mode === 'review'
  const limit = effective.budget?.book_usd ?? 0
  // D2 — gói thể loại hiện “Gói <tên> · có chấm” thay cho tên kiểu viết (Outline.dc.html › applied).
  const pack = findPack(packs, effective.style)
  const styleValue = pack ? `Gói ${packLabel(pack)} · có chấm` : effective.style || 'Chung'
  const rulesValue = !counts ? '…'
    : story
      ? `${(counts.story ?? 0) + (counts.project ?? 0) + (counts.global ?? 0)} quy tắc`
      : `${counts.project ?? 0} quy tắc của dự án + ${counts.global ?? 0} chung`
  if (story) {
    return [
      { icon: 'edit-2', label: 'Quy tắc viết', value: rulesValue, own: false, section: 'rules' },
      { icon: 'note-text', label: 'Văn phong', value: styleValue, own: own('style'), section: 'style' },
      { icon: 'book-1', label: 'Duyệt từng chương', value: review ? 'Bật' : 'Tắt', own: own('advance_mode'), section: 'writing' },
      { icon: 'cpu', label: 'Người viết', value: model ? `${model} · ${thinkingText(level)}` : 'Theo “Mọi truyện”', own: own('roles.writer') || own('model'), section: 'roles' },
    ]
  }
  const assistantOwn = own('roles.writer') || own('model')
  return [
    { icon: 'edit-2', label: 'Quy tắc viết', value: rulesValue, own: false, section: 'rules' },
    { icon: 'note-text', label: 'Văn phong', value: own('style') ? styleValue : 'Chung', own: own('style'), section: 'style' },
    { icon: 'book-1', label: 'Duyệt từng chương', value: review ? 'Bật cho truyện mới' : 'Tắt cho truyện mới', own: own('advance_mode'), section: 'writing' },
    { icon: 'cpu', label: 'Trợ lý AI', value: assistantOwn && model ? `${model} · ${thinkingText(level)}` : 'Theo “Mọi truyện”', own: assistantOwn, section: 'roles' },
    { icon: 'coin-1', label: 'Ngân sách mỗi truyện', value: limit > 0 ? usd(limit) : '—', own: own('budget.book_usd'), section: 'budget' },
  ]
}
