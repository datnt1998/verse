// Quy tắc đã học (data-contract §19): viết (Đạo diễn, theo kênh) và Ghi chú đã học (Hoạ sĩ, theo prompt ảnh).
import { z } from 'zod'
import { mutate, request } from './request'

const example = z.object({ before: z.string(), after: z.string(), jobId: z.string() })

export const learnedRuleSchema = z.object({
  id: z.string(),
  scope: z.union([z.object({ channelId: z.string() }), z.object({ imagePromptId: z.string() })]),
  agent: z.enum(['director', 'artist']),
  text: z.string(),
  /** nhãn nguồn dựng sẵn từ server (“Học từ 3 lần bạn sửa · 26–28/10”) */
  source: z.string(),
  enabled: z.boolean(),
  createdAt: z.string(),
  learnedFrom: z.object({
    kind: z.enum(['edits', 'review', 'perf', 'manual']),
    count: z.number(),
    from: z.string(),
    to: z.string(),
    examples: z.array(example).nullable().transform((value) => value ?? []),
  }),
})
export type LearnedRule = z.infer<typeof learnedRuleSchema>
export type RuleExample = z.infer<typeof example>

const listSchema = z.object({ rules: z.array(learnedRuleSchema).nullable().transform((value) => value ?? []), limit: z.number() })
export type LearnedRuleList = z.infer<typeof listSchema>

export type RuleScope = { channelId: string } | { imagePromptId: string }
const query = (scope: RuleScope) => 'channelId' in scope ? `channelId=${encodeURIComponent(scope.channelId)}` : `imagePromptId=${encodeURIComponent(scope.imagePromptId)}`

export const listLearnedRules = async (scope: RuleScope, signal?: AbortSignal) =>
  listSchema.parse(await request<unknown>(`/api/learned-rules?${query(scope)}`, { signal }))

export const setLearnedRuleEnabled = async (id: string, enabled: boolean) =>
  learnedRuleSchema.parse(await mutate<unknown>('PATCH', `/api/learned-rules/${encodeURIComponent(id)}`, { enabled }))

export const deleteLearnedRule = async (id: string) => { await mutate<unknown>('DELETE', `/api/learned-rules/${encodeURIComponent(id)}`) }
