// Mã kiểm tra → lời thường (20-settings-rules.md §4 › “Lời cho từng mã kiểm tra”).
// `message`/`path` của máy chủ chứa tên tệp và thư mục nên không bao giờ hiện cho người dùng.
import type { LibraryConfig, StorySummary, ValidationIssue } from '../../api/types'
import { storyLabel } from '../../shell/labels'

export type PlainIssue = { key: string; code: string; text: string; fixable: boolean }

export type PlainContext = {
  /** Tên hiển thị của dự án (không phải id/thư mục). */
  projectName: string
  dirs: LibraryConfig['dirs']
  stories: Pick<StorySummary, 'story' | 'folder' | 'title' | 'tempName'>[]
}

/** `invalid-name`: tên thư mục không theo chuẩn mới nhưng vẫn dùng được — chỉ có trong “Chi tiết cho quản trị”. */
const HIDDEN_CODE = 'invalid-name'

const GENERIC = 'Có một điểm chưa đúng chuẩn trong dự án. Nhờ quản trị kiểm tra.'

function sectionOf(path: string, dirs: LibraryConfig['dirs']): string | null {
  const first = path.split('/')[0]
  if (first === dirs.ideas) return 'Ý tưởng'
  if (first === dirs.assets) return 'Tư liệu'
  if (first === dirs.stories) return 'Truyện'
  if (path.endsWith('rules')) return 'Quy tắc viết'
  return null
}

/** Truyện của một đường dẫn `kich_ban/<thư mục truyện>/…` (hoặc `<thư mục truyện>/…` ở dự án cũ). */
function storyNameOf(path: string, context: PlainContext): string {
  const segments = path.split('/')
  const known = context.stories.find((story) => path === story.folder || path.startsWith(`${story.folder}/`) || segments.includes(story.story))
  if (known) return storyLabel(known)
  const folder = segments[0] === context.dirs.stories ? segments[1] : segments[0]
  return storyLabel({ story: folder ?? '' })
}

export function plainIssues(issues: ValidationIssue[], context: PlainContext): PlainIssue[] {
  const out: PlainIssue[] = []
  const legacy: ValidationIssue[] = []
  const symlinks: ValidationIssue[] = []
  for (const issue of issues) {
    if (issue.code === HIDDEN_CODE) continue
    const key = `${issue.code}:${issue.path}`
    const entry = (text: string): PlainIssue => ({ key, code: issue.code, text, fixable: issue.fixable })
    switch (issue.code) {
      case 'missing-project-json':
        out.push(entry(`Dự án chưa có thông tin — Noverse sẽ tạo mới với tên “${context.projectName}”.`)); break
      case 'invalid-project-json':
        out.push(entry('Thông tin dự án bị hỏng, Noverse không đọc được. Nhờ quản trị kiểm tra.')); break
      case 'missing-dir': {
        const section = sectionOf(issue.path, context.dirs)
        out.push(entry(section ? `Thiếu mục ${section} — sẽ tạo mục trống.` : 'Thiếu một mục trong dự án — sẽ tạo mục trống.'))
        break
      }
      case 'missing-story-json':
        out.push(entry(`Truyện “${storyNameOf(issue.path, context)}” chưa có thông tin trạng thái — sẽ tạo mới với trạng thái “Đang viết”.`)); break
      case 'invalid-story-json':
        out.push(entry(`Thông tin của truyện “${storyNameOf(issue.path, context)}” bị hỏng. Nhờ quản trị kiểm tra.`)); break
      case 'legacy-rules': legacy.push(issue); break
      case 'rules-symlink': symlinks.push(issue); break
      default: out.push(entry(GENERIC))
    }
  }
  // Mỗi tệp quy tắc cũ là một mã riêng; artboard gộp thành một dòng có số đếm.
  if (legacy.length > 0) {
    out.push({
      key: 'legacy-rules', code: 'legacy-rules', fixable: legacy.every((issue) => issue.fixable),
      text: `Có ${legacy.length} quy tắc viết kiểu cũ — sẽ chuyển vào Quy tắc viết của dự án.`,
    })
  }
  if (symlinks.length > 0) {
    out.push({
      key: 'rules-symlink', code: 'rules-symlink', fixable: symlinks.every((issue) => issue.fixable),
      text: 'Quy tắc của truyện đang trỏ sang dự án — sẽ gỡ để tránh áp hai lần.',
    })
  }
  return out
}

/** Mã gửi lên `POST …/validate/fix`: chỉ những mã sửa được (`fixable=true`), mỗi mã một lần. */
export function fixableCodes(issues: PlainIssue[]): string[] {
  return [...new Set(issues.filter((issue) => issue.fixable).map((issue) => issue.code))]
}
