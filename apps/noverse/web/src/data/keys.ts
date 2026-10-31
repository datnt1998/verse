// Khoá cache của Noverse. Khoá là mảng; `invalidate(prefix)` khớp mọi khoá bắt đầu
// bằng prefix, nên đặt phần chung (truyện, dự án) lên trước phần chi tiết.
// Phần lõi (Key, keyHash, matchesPrefix) nằm ở @noverse/ui/data.
import type { ConfigScope } from '../api/types'
import type { Key, KeyPart } from '@noverse/ui/data'

/** Phần dữ liệu của một truyện — trùng `parts` của sự kiện `story.changed`. */
export type StoryPart =
  | 'snapshot' | 'chapters' | 'chapter' | 'outline' | 'reviews' | 'exports' | 'meta' | 'audio' | 'rules' | 'comments'
  | 'plan' | 'facts' | 'revisions' | 'foreshadow' | 'characters' | 'address' | 'brain' | 'style-samples'
  | 'frame' | 'retention' | 'premise' | 'world-rules' | 'live-history'

/** Phần dữ liệu của một dự án — trùng `parts` của sự kiện `project.changed`. */
export type ProjectPart = 'meta' | 'stories' | 'ideas' | 'assets' | 'voices' | 'pronunciations'

export const keys = {
  library: () => ['library'] as const,
  /** Không có `part` = cả dự án (dùng để invalidate); GET /api/projects/:p là `project(p, 'meta')`. */
  project: (p: string, part?: ProjectPart, ...rest: KeyPart[]) => (part ? ['project', p, part, ...rest] : ['project', p]) as Key,
  /** `story(p, s)` = mọi khoá của truyện; `story(p, s, 'chapter', 3, 'draft')` = một chương cụ thể. */
  story: (p: string, s: string, part?: StoryPart, ...rest: KeyPart[]) => (part ? ['story', p, s, part, ...rest] : ['story', p, s]) as Key,
  /** Cấu hình theo scope. `config('project', p)` cũng là prefix của `config('story', …)`? Không — dùng `configChain`. */
  config: (scope: ConfigScope, p?: string, s?: string) => ['config', scope, ...(p ? [p] : []), ...(s ? [s] : [])] as Key,
  /** Quy tắc theo scope (danh sách + từng tệp: `[...rules(scope,p,s), 'file', name]`). */
  rules: (scope: ConfigScope, p?: string, s?: string) => ['rules', scope, ...(p ? [p] : []), ...(s ? [s] : [])] as Key,
  me: () => ['me'] as const,
  users: () => ['users'] as const,
  sessions: () => ['sessions'] as const,
  jobs: () => ['jobs'] as const,
  tts: () => ['tts'] as const,
  system: () => ['system'] as const,
}
