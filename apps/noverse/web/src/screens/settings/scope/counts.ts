// Số quy tắc riêng của một dự án/truyện cho danh sách chọn (Gói R). Đọc tệp `quy-tac.md` — cùng tệp
// màn Quy tắc viết ghi — nên không gọi mô hình để chuẩn hoá (Effective có thể gọi LLM), chỉ đếm dòng.
import { ApiError, getRuleFile } from '../../../api/client'
import { useResource } from '../../../data'
import { RULES_FILE, parseRuleLines } from '../sections/rules/logic'

/** Tệp chưa có (dự án/truyện chưa chỉnh quy tắc) coi như 0 dòng. */
const empty = { name: RULES_FILE, text: '' }

/** Số quy tắc riêng ở đúng cấp đó; `undefined` khi đang đọc. */
export function useOwnRuleCount(scope: 'project' | 'story', project: string, story: string, enabled = true): number | undefined {
  const resource = useResource(
    ['settings-scope-rules', scope, project, story],
    ({ signal }) => getRuleFile(scope, RULES_FILE, project || undefined, story || undefined, signal)
      .catch((error: unknown) => {
        if (error instanceof ApiError && (error.status === 404 || error.status === 403)) return empty
        throw error
      }),
    { enabled: enabled && !!project && (scope === 'project' || !!story) },
  )
  if (resource.error) return 0
  return resource.data ? parseRuleLines(resource.data.text).length : undefined
}
