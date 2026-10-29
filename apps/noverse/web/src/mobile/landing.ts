import { getLastStory } from '../api/storage'
import { verifyTarget } from '../commands/target'
import { paths } from '../router'

/**
 * Màn đầu tiên trên điện thoại khi vào `#/` (18-mobile không quy định): Viết của truyện mở gần nhất —
 * nếu truyện còn trong thư viện — còn không thì ở lại Thêm › Dự án & truyện. Trả `null` = ở lại.
 */
export async function phoneLandingPath(username: string): Promise<string | null> {
  const last = getLastStory(username)
  if (!last || !(await verifyTarget(last))) return null
  return paths.workspace(last.project, last.story)
}
