// Gói R — ngữ cảnh “Áp dụng cho” của Tuỳ chỉnh: mục nào có thanh, mục tiêu đang xem, câu phụ của từng dòng.
// Tách khỏi React để ghim bằng test: chọn cấp/dự án/truyện, dải “Đang xem…”, nhãn “theo …”.
import type { ConfigScope, ProjectSummary, StorySummary } from '../../../api/types'
import type { PageId } from '../lib/target'
import { storyLabel } from '../../../shell/labels'

/** Mục có 3 cấp (21-dev-prompt.md › Gói R, R3): chỉ những mục này mới hiện thanh “Áp dụng cho”. */
export const SCOPED_SECTIONS: readonly PageId[] = ['writing', 'models', 'budget', 'info']

export const showsScope = (section: PageId): boolean => SCOPED_SECTIONS.includes(section)

/** Mục tiêu đang xem ở Tuỳ chỉnh. `scope` là cấp sẽ ghi khi bấm Áp dụng/Lưu. */
export type ScopeTarget = { scope: ConfigScope; project: string; story: string }

/** Truyện đang mở ở Bàn viết — thanh đường dẫn không đổi khi xem chỗ khác. */
export type OpenTarget = { project: string; story: string; title: string } | null

export const targetKey = (target: ScopeTarget): string => `${target.scope}/${target.project}/${target.story}`

export const sameTarget = (a: ScopeTarget, b: ScopeTarget): boolean => targetKey(a) === targetKey(b)

/** Artboard cắt tên truyện ở 22 ký tự rồi thêm “…”. */
export const clipLabel = (text: string, max = 22): string => (text.length > max ? `${text.slice(0, max)}…` : text)

/** Nhãn ô “Truyện”: “Truyện này” khi đúng truyện đang mở, “Truyện” khi xem truyện khác (R1). */
export const storyBoxLabel = (target: Pick<ScopeTarget, 'project' | 'story'>, open: OpenTarget): string =>
  open && open.project === target.project && open.story === target.story ? 'Truyện này' : 'Truyện'

/** Dòng phụ của một truyện trong danh sách: số quy tắc riêng, hoặc “theo dự án” (R2). */
export const storyRuleSub = (ownRules: number | undefined): string =>
  ownRules === undefined ? 'Đang đếm quy tắc…' : ownRules > 0 ? `${ownRules} quy tắc riêng` : 'theo dự án'

/** Dòng phụ của một dự án: “N truyện · M quy tắc riêng” hoặc “N truyện · theo “Mọi truyện”” (R2). */
export const projectRuleSub = (stories: number, ownRules: number | undefined): string =>
  ownRules === undefined ? `${stories} truyện · Đang đếm quy tắc…`
    : ownRules > 0 ? `${stories} truyện · ${ownRules} quy tắc riêng`
      : `${stories} truyện · theo “Mọi truyện”`

export type ScopeTargetNames = { projectName: string; storyTitle: string; open: OpenTarget }

/**
 * Dải xanh dưới thanh (R3): chỉ khi đang xem một dự án khác, hoặc một truyện khác truyện đang mở.
 * Cấp “Mọi truyện” là lựa chọn có chủ đích nên không có dải.
 */
export function viewingElsewhereText(target: ScopeTarget, names: ScopeTargetNames): string | null {
  const { open } = names
  if (!open || target.scope === 'global') return null
  if (target.scope === 'project') {
    if (target.project === open.project) return null
    return `Đang xem quy tắc của dự án “${names.projectName}”. Truyện đang mở ở Bàn viết vẫn là “${open.title}”.`
  }
  if (target.project === open.project && target.story === open.story) return null
  return `Đang xem quy tắc của truyện “${names.storyTitle}”. Truyện đang mở ở Bàn viết vẫn là “${open.title}”.`
}

/** Đối tượng trong câu “Có N thay đổi chưa lưu cho …” của hộp cam (R3). */
export function pendingObject(target: ScopeTarget, projectName: string, storyTitle: string): string {
  if (target.scope === 'project') return `dự án “${projectName}”`
  if (target.scope === 'story') return `truyện “${storyTitle}”`
  return '“Mọi truyện”'
}

export const unsavedText = (count: number, object: string): string =>
  `Có ${count} thay đổi chưa lưu cho ${object}. Lưu trước khi chuyển?`

/** Truyện của một dự án sau khi đổi dự án: truyện đang mở nếu cùng dự án, không thì truyện đầu (R3). */
export function storyForProject(project: string, stories: readonly StorySummary[], open: OpenTarget): string {
  if (open && open.project === project && stories.some((item) => item.story === open.story)) return open.story
  return stories[0]?.story ?? ''
}

/** Ứng viên thay thế khi ngữ cảnh đã lưu không còn thật: truyện đang mở, không thì dự án đầu (R3). */
function fallbackProject(projects: readonly ProjectSummary[], open: OpenTarget): string {
  if (open && projects.some((item) => item.id === open.project)) return open.project
  return (projects.find((item) => !item.legacy) ?? projects[0])?.id ?? ''
}

/**
 * Gói R — ngữ cảnh Tuỳ chỉnh đọc từ đường dẫn/localStorage của máy khác có thể trỏ vào
 * dự án hoặc truyện không có trên máy chủ này. Trả ngữ cảnh thay thế (truyện đang mở
 * trước, rồi dự án đầu tiên) để màn hình bỏ giá trị hỏng; `null` = ngữ cảnh còn đúng
 * hoặc chưa kết luận được (danh sách truyện đang tải).
 */
export function staleScopeFallback(
  current: { project: string; story: string },
  ctx: {
    projects: readonly ProjectSummary[]
    open: OpenTarget
    /** truyện của một dự án; `null` khi chi tiết dự án chưa tải xong */
    storiesOf: (project: string) => readonly StorySummary[] | null
  },
): { project: string; story: string } | null {
  const { projects, open, storiesOf } = ctx
  if (!current.project || projects.length === 0) return null
  if (!projects.some((item) => item.id === current.project)) {
    const project = fallbackProject(projects, open)
    const stories = storiesOf(project)
    return {
      project,
      // Chưa tải xong danh sách: tin truyện đang mở; tải rồi thì theo đúng danh sách.
      story: stories === null
        ? (open?.project === project ? open.story : '')
        : storyForProject(project, stories, open),
    }
  }
  const stories = storiesOf(current.project)
  if (stories === null) return null
  if (current.story && !stories.some((item) => item.story === current.story)) {
    return { project: current.project, story: storyForProject(current.project, stories, open) }
  }
  return null
}

/** Cấp hợp lệ khi ngữ cảnh thiếu: không có truyện → dự án, không có dự án → mọi truyện (R3). */
export const validScope = (scope: ConfigScope, project: string, story: string): ConfigScope =>
  scope === 'story' && (!project || !story) ? (project ? 'project' : 'global')
    : scope === 'project' && !project ? 'global' : scope

/** Tên truyện hiển thị: `storyLabel` (bỏ tiêu đề trùng slug / thư mục tạm), không bao giờ hiện slug thô. */
export const displayStoryTitle = (summary: StorySummary | null | undefined, fallback = ''): string =>
  summary ? storyLabel(summary) : fallback
