import { paths } from '../../router'

/**
 * Liên kết sang Tuỳ chỉnh đúng cấp (20-settings-rules.md §4): dòng của truyện → cấp truyện, dòng của dự án → cấp dự án.
 * Cấp được truyền bằng chuỗi truy vấn `?scope=&project=&story=` (router đã đọc `search`).
 */
export function settingsLink(section: string | undefined, target: { project: string; story?: string }): string {
  const query = new URLSearchParams({ scope: target.story ? 'story' : 'project', project: target.project })
  if (target.story) query.set('story', target.story)
  return `${paths.settings(section)}?${query}`
}
