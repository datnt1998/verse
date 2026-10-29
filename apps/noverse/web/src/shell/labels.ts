// Nhãn hiển thị dùng chung: tên dự án/truyện (20-settings-rules.md §1), crumb dự án (Outline.dc.html)
// và header điện thoại (Phone.dc.html › home).
// Người dùng chỉ thấy `project.json.name` và `story.json.title`; slug thư mục và tiền tố `01 ·` không bao giờ hiện.
import type { ProjectSummary } from '../api/types'
import { t } from '../i18n/vi'
import { CMD_UI } from '../i18n/commands.vi'
import type { Route } from '../router'

/** "dit-dang-audio" → "Dit Dang Audio"; bỏ tiền tố số thư mục "01-" khi `stripNumber`. */
export function humanizeSlug(slug: string, stripNumber = false): string {
  const body = stripNumber ? slug.replace(/^\d+-/, '') : slug
  return body.split(/[-_\s]+/).filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ')
}

/**
 * Tên dự án để hiện: `project.json.name`. Dự án `_` gom các truyện bố trí cũ (11-api-contract.md §3) —
 * engine trả `name: "_"`, UI hiện "Truyện cũ". Thiếu tên → tên dựng từ id, không bao giờ hiện id thô.
 */
export function projectLabel(entry: Pick<ProjectSummary, 'name' | 'legacy'> | undefined | null, id: string | null): string {
  if (!id) return '—'
  if (entry?.legacy || id === '_') return t.crumb.legacyProject
  return entry?.name?.trim() || humanizeSlug(id) || '—'
}

/** Phần tử tối thiểu để đặt tên truyện — StorySummary thoả. */
export type StoryNameSource = { story: string; title?: string | null; tempName?: boolean }

/**
 * Tên truyện để hiện: `story.json.title`. Thiếu (engine điền chính slug vào `title`, hoặc thư mục tạm
 * `NN-truyen-moi`) → "Truyện chưa đặt tên" / tên dựng từ slug, bỏ tiền tố số.
 */
export function storyLabel(story: StoryNameSource | undefined | null): string {
  if (!story) return ''
  const title = story.title?.trim() ?? ''
  const isSlugTitle = title === story.story || title === story.story.replace(/^\d+-/, '')
  if (title && !isSlugTitle) return title
  if (story.tempName || /^\d+-truyen-moi$/.test(story.story)) return t.crumb.untitledStory
  return humanizeSlug(story.story, true) || t.crumb.untitledStory
}

/** Tiêu đề tab trình duyệt: `tên truyện · tên dự án · Noverse` — tên hiển thị, không slug. */
export function documentTitle(route: Route, projectName: string | null, storyName: string | null): string {
  switch (route.name) {
    case 'settings':
      return `${t.nav.settings} · ${t.brand.name}`
    case 'story': case 'workspace': case 'activity': case 'reader': case 'audio':
      return [storyName, projectName, t.brand.name].filter(Boolean).join(' · ')
    case 'quality':
      return [t.quality.screenTitle, storyName, projectName, t.brand.name].filter(Boolean).join(' · ')
    case 'newstory':
      return [t.nav.newStory, projectName, t.brand.name].filter(Boolean).join(' · ')
    case 'project':
      return [projectName, t.brand.name].filter(Boolean).join(' · ')
    default:
      return `${t.brand.name} · Không gian viết`
  }
}

/**
 * 2 dòng header điện thoại: dòng trên là ngữ cảnh (`dự án · Tập N`), dòng dưới là tên màn.
 * Phone.dc.html › home dùng `Kênh truyện audio · Tập 1` / tên truyện — hai dòng phải khác nhau,
 * nếu không header lặp lại cùng một chữ.
 */
export function mobileHeaderCopy(
  route: Route, projectName: string | null, storyTitle: string | null,
): { meta: string; title: string } {
  const project = projectName ?? t.brand.name
  switch (route.name) {
    case 'settings':
      return { meta: t.brand.name, title: t.nav.settings }
    case 'story': case 'workspace': case 'activity': case 'reader': case 'audio':
      return { meta: `${project} · ${t.crumb.volume(1)}`, title: storyTitle || project }
    case 'newstory':
      return { meta: project, title: t.nav.newStory }
    case 'help':
      return { meta: project, title: CMD_UI.helpTitle }
    case 'project': {
      const title = projectName ?? t.crumb.library
      // Dự án trùng tên nhãn "Thư viện" → dùng tên thương hiệu để 2 dòng vẫn khác nhau
      return { meta: title === t.crumb.library ? t.brand.name : t.crumb.library, title }
    }
    default:
      return { meta: t.brand.name, title: t.crumb.library }
  }
}
