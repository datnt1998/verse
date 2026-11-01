// Đọc Quy tắc đã học của một phạm vi (kênh | prompt ảnh) và làm mới sau khi lưu/hoàn tác.
import { invalidate } from '@noverse/ui/data'
import { z } from 'zod'
import { listLearnedRules, type RuleScope } from '../../api/learnedRules'
import type { Suggestion } from '../../api/suggestions'
import { useResource } from '../../data/useResource'

export const refreshLearned = () => {
  invalidate(['learned-rules'])
  invalidate(['suggestions'])
  invalidate(['workbench'])
  invalidate(['library', 'prompts'])
}

export const useLearnedRules = (scope: RuleScope) =>
  useResource(['learned-rules', 'channelId' in scope ? 'channel' : 'prompt', 'channelId' in scope ? scope.channelId : scope.imagePromptId], ({ signal }) => listLearnedRules(scope, signal))

const exampleSchema = z.array(z.object({ before: z.string(), after: z.string(), jobId: z.string().optional() }))
const learnedFrom = z.object({ examples: exampleSchema.nullish() })

/** Các lần sửa/góp ý mà đề xuất học dựa vào (`primary.params.learnedFrom.examples`). */
export function learnExamples(suggestion: Suggestion) {
  const parsed = learnedFrom.safeParse(suggestion.primary.params.learnedFrom)
  return parsed.success ? parsed.data.examples ?? [] : []
}
