// Đọc Quy tắc đã học của một phạm vi (kênh | prompt ảnh) và làm mới sau khi lưu/hoàn tác.
import { invalidate } from '@noverse/ui/data'
import { list, nullish, record, str } from '../../api/guard'
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

/** Các lần sửa/góp ý mà đề xuất học dựa vào (`primary.params.learnedFrom.examples`). */
export function learnExamples(suggestion: Suggestion) {
  try {
    const learnedFrom = record(suggestion.primary.params.learnedFrom, 'learnedFrom')
    if (learnedFrom.examples == null) return []
    return list(learnedFrom.examples, (entry, at) => {
      const example = record(entry, at)
      return {
        before: str(example.before, `${at}.before`),
        after: str(example.after, `${at}.after`),
        jobId: nullish(example.jobId, str, `${at}.jobId`),
      }
    }, 'learnedFrom.examples')
  } catch {
    return []
  }
}
