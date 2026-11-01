// Lối vào M11 (Truyện mới trên điện thoại) — dùng chung cho M1 trống, M4 › Dự án & truyện và
// trang Hướng dẫn lệnh `/start`. Chọn dự án sẽ lưu truyện vào, nên mọi lối vào mở cùng một màn.
import { canCreateStory } from '../../api/client'
import type { ProjectSummary, User } from '../../api/types'
import { paths } from '../../router'

/** MMore › Dự án & truyện — nơi còn mở được khi chưa có dự án nào để tạo truyện. */
export const PROJECTS_PAGE = '/settings/mobile?page=projects'

/**
 * Dự án M11 sẽ lưu truyện vào: dự án đang mở nếu người dùng tạo được truyện ở đó, không thì
 * dự án đầu tiên họ tạo được (thư viện đã xếp dự án mở gần nhất lên trước).
 */
export function newStoryProject(projects: readonly ProjectSummary[], user: User | null, preferred?: string | null): string | null {
  if (!user) return null
  const writable = projects.filter((item) => canCreateStory(user, item.id))
  if (preferred && writable.some((item) => item.id === preferred)) return preferred
  return writable[0]?.id ?? null
}

/** Đường dẫn mở M11 với dự án điền sẵn; chưa có dự án nào thì về trang Dự án & truyện. */
export function newStoryEntry(projects: readonly ProjectSummary[], user: User | null, preferred?: string | null): string {
  const project = newStoryProject(projects, user, preferred)
  return project ? paths.newstory(project) : PROJECTS_PAGE
}
