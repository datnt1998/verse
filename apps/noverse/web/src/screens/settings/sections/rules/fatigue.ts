import type { ConfigScope, RulesEffective, RuleViolationChapter } from '../../../../api/types'
import { DEFAULT_FATIGUE, type RuleEntry } from './logic'

export const fatigueLimit = (entry: RuleEntry, word: string): number | undefined => {
  const chip = entry.chips.find((item) => item.text === word && item.kind === 'max')
  return chip?.limit
}
export const fatigueRule = (word: string, limit: number) => `“${word}” tối đa ${limit} lần mỗi chương.`

/** Pending rules override the effective response immediately, before Áp dụng. */
export function fatigueRows(scope: ConfigScope, data: RulesEffective | undefined, own: RuleEntry[], learned: RuleEntry[], project: RuleEntry[], chapters: RuleViolationChapter[] = []) {
  return DEFAULT_FATIGUE.map(([word, defaultLimit]) => {
    const effective = data?.fatigueWords?.find((row) => row.word === word)
    const projectRule = scope === 'story' ? project.find((entry) => fatigueLimit(entry, word) !== undefined) : undefined
    const ownRules = own.filter((entry) => fatigueLimit(entry, word) !== undefined)
    const learnedRules = scope === 'story' ? learned.filter((entry) => fatigueLimit(entry, word) !== undefined) : []
    // AP2 preserves the lower-precedence baseline even after a saved override.
    const baseline = projectRule ? fatigueLimit(projectRule, word)! : effective?.basePerChapter ?? (
      effective && effective.source !== 'story' && effective.source !== 'learned' && !(scope === 'project' && effective.source === 'project')
        ? effective.perChapter : defaultLimit
    )
    const chosen = ownRules[0] ?? learnedRules[0]
    const originalOverride = data?.items.some((entry) => entry.scope === scope && entry.chips.some((chip) => chip.kind === 'max' && chip.text === word))
      || scope === 'story' && data?.runtime?.some((entry) => entry.chips.some((chip) => chip.kind === 'max' && chip.text === word))
    const limit = chosen ? fatigueLimit(chosen, word)! : projectRule || originalOverride ? baseline : effective?.perChapter ?? baseline
    const source = chosen ? ownRules.length ? scope : 'learned' : projectRule ? 'project' : originalOverride ? effective?.baseSource : effective?.source
    const latest = chapters.reduce<RuleViolationChapter | undefined>((a, b) => !a || b.chapter > a.chapter ? b : a, undefined)
    const used = latest?.violations.find((v) => v.rule === 'fatigue_words' && v.target === word)
    return { word, limit, baseline, source, ownRules, learnedRules, changed: !!chosen, project: !!projectRule || effective?.baseSource === 'project', over: used && used.actual > limit ? latest!.chapter : undefined }
  })
}
