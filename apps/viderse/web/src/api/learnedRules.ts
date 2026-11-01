// Quy tắc đã học (data-contract §19): viết (Đạo diễn, theo kênh) và Ghi chú đã học (Hoạ sĩ, theo prompt ảnh).
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: danh sách quy tắc nằm trong
// chunk của màn đầu (Bàn dựng).
import { bool, list, num, oneOf, record, str } from './guard'
import { mutate, request } from './request'

export type RuleExample = { before: string; after: string; jobId: string }

export type LearnedRule = {
  id: string
  scope: { channelId: string } | { imagePromptId: string }
  agent: 'director' | 'artist'
  text: string
  /** nhãn nguồn dựng sẵn từ server (“Học từ 3 lần bạn sửa · 26–28/10”) */
  source: string
  enabled: boolean
  createdAt: string
  learnedFrom: {
    kind: 'edits' | 'review' | 'perf' | 'manual'
    count: number
    from: string
    to: string
    examples: RuleExample[]
  }
}

const readExample = (value: unknown, path: string): RuleExample => {
  const o = record(value, path)
  return { before: str(o.before, `${path}.before`), after: str(o.after, `${path}.after`), jobId: str(o.jobId, `${path}.jobId`) }
}

const readScope = (value: unknown, path: string): LearnedRule['scope'] => {
  const o = record(value, path)
  return o.channelId === undefined
    ? { imagePromptId: str(o.imagePromptId, `${path}.imagePromptId`) }
    : { channelId: str(o.channelId, `${path}.channelId`) }
}

const readRule = (value: unknown, path: string): LearnedRule => {
  const o = record(value, path)
  const learnedFrom = record(o.learnedFrom, `${path}.learnedFrom`)
  return {
    id: str(o.id, `${path}.id`),
    scope: readScope(o.scope, `${path}.scope`),
    agent: oneOf(o.agent, ['director', 'artist'] as const, `${path}.agent`),
    text: str(o.text, `${path}.text`),
    source: str(o.source, `${path}.source`),
    enabled: bool(o.enabled, `${path}.enabled`),
    createdAt: str(o.createdAt, `${path}.createdAt`),
    learnedFrom: {
      kind: oneOf(learnedFrom.kind, ['edits', 'review', 'perf', 'manual'] as const, `${path}.learnedFrom.kind`),
      count: num(learnedFrom.count, `${path}.learnedFrom.count`),
      from: str(learnedFrom.from, `${path}.learnedFrom.from`),
      to: str(learnedFrom.to, `${path}.learnedFrom.to`),
      examples: learnedFrom.examples == null ? [] : list(learnedFrom.examples, readExample, `${path}.learnedFrom.examples`),
    },
  }
}

export type LearnedRuleList = { rules: LearnedRule[]; limit: number }

const readRuleList = (value: unknown): LearnedRuleList => {
  const o = record(value, 'rules')
  return {
    rules: o.rules == null ? [] : list(o.rules, readRule, 'rules.rules'),
    limit: num(o.limit, 'rules.limit'),
  }
}

export type RuleScope = { channelId: string } | { imagePromptId: string }
const query = (scope: RuleScope) => 'channelId' in scope ? `channelId=${encodeURIComponent(scope.channelId)}` : `imagePromptId=${encodeURIComponent(scope.imagePromptId)}`

export const listLearnedRules = async (scope: RuleScope, signal?: AbortSignal) =>
  readRuleList(await request<unknown>(`/api/learned-rules?${query(scope)}`, { signal }))

export const setLearnedRuleEnabled = async (id: string, enabled: boolean) =>
  readRule(await mutate<unknown>('PATCH', `/api/learned-rules/${encodeURIComponent(id)}`, { enabled }), 'rule')

export const deleteLearnedRule = async (id: string) => { await mutate<unknown>('DELETE', `/api/learned-rules/${encodeURIComponent(id)}`) }
