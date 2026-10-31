// Ánh xạ sự kiện toàn cục (/api/events, 16-feedback-refresh.md §2) → prefix khoá cần invalidate.
import type { Job } from '../api/types'
import { matchesPrefix, type Key } from '@noverse/ui/data'
import { keys, type StoryPart } from './keys'

export type ServerEventType =
  | 'hello' | 'reset' | 'ping' | 'live'
  | 'library.changed' | 'project.changed' | 'story.changed' | 'config.changed' | 'comments.reanchored'
  | 'account.changed' | 'session.revoked' | 'notify.changed'
  | 'job.created' | 'job.progress' | 'job.waiting' | 'job.failed' | 'job.done' | 'job.removed'

export const SERVER_EVENT_TYPES: readonly ServerEventType[] = [
  'hello', 'reset', 'ping',
  'live', // khung thật (audio/story) đi kèm /api/events — một kết nối cho mỗi tab
  'library.changed', 'project.changed', 'story.changed', 'config.changed', 'comments.reanchored',
  'account.changed', 'session.revoked', 'notify.changed',
  'job.created', 'job.progress', 'job.waiting', 'job.failed', 'job.done', 'job.removed',
]

export type ServerEvent = { type: ServerEventType; data: unknown; id: string }

/** `'*'` = invalidate mọi khoá (đổi quyền / mất dấu sự kiện). */
export type EventKeys = Key[] | '*'

const str = (value: unknown) => (typeof value === 'string' && value ? value : undefined)
const record = (value: unknown): Record<string, unknown> => (value && typeof value === 'object' ? value as Record<string, unknown> : {})

/** Phần của `story.changed` → các khoá con. `chapters` kéo theo từng chương và dữ liệu phụ của chương. */
const STORY_PART_KEYS: Record<string, StoryPart[]> = {
  snapshot: ['snapshot', 'premise', 'world-rules'],
  // Gói S: sửa nhân vật (loại vai/lộ vai) ghi characters.json — Não truyện đọc cùng tệp.
  characters: ['characters', 'brain'],
  chapters: ['chapters', 'chapter', 'plan', 'facts', 'reviews', 'revisions', 'foreshadow', 'characters', 'retention'],
  // D5: bản đồ giữ chân là dẫn xuất của đại cương, khung truyện nằm trong story_retention.json
  // mà Kiến trúc sư ghi khi lập dàn ý — cả hai làm mới theo `outline`.
  outline: ['outline', 'plan', 'frame', 'retention', 'premise'],
  reviews: ['reviews'],
  exports: ['exports'],
  meta: ['meta'],
  audio: ['audio'],
  rules: ['rules'],
  comments: ['comments'],
  // GAP-35: máy chủ phát `parts:["style"]` sau khi thêm/xoá văn mẫu.
  style: ['style-samples'],
  frame: ['frame'],
  retention: ['retention'],
  brain: ['brain', 'world-rules'],
  premise: ['premise'],
  'world-rules': ['world-rules'],
}

/**
 * Sự kiện → prefix khoá. `me` là tên đăng nhập hiện tại (account.changed của chính mình = quyền
 * có thể đã đổi → làm mới tất cả).
 */
export function keysForEvent(event: { type: string; data: unknown }, me?: string): EventKeys {
  const data = record(event.data)
  const project = str(data.project)
  const story = str(data.story)
  const parts = Array.isArray(data.parts) ? data.parts.filter((part): part is string => typeof part === 'string') : []
  switch (event.type) {
    case 'reset':
      return '*'
    case 'live': {
      if (!project || !story) return []
      if (data.kind === 'event' && record(data.data).category === 'TOOL') return [keys.story(project, story, 'live-history')]
      if (data.kind !== 'done') return []
      return (['snapshot', 'chapters', 'outline', 'reviews', 'live-history'] as const).map(part => keys.story(project, story, part))
    }
    case 'library.changed':
      return [keys.library()]
    case 'project.changed': {
      if (!project) return [keys.library()]
      // Tên/mô tả dự án hiện ở thư viện; danh sách truyện cũng là số liệu của thư viện.
      const out: Key[] = [keys.library()]
      if (parts.length === 0) out.push(keys.project(project))
      else parts.forEach((part) => out.push(keys.project(project, part as Parameters<typeof keys.project>[1])))
      return out
    }
    case 'story.changed': {
      if (!project) return []
      // Từ điển/giọng cấp dự án dùng chung cho mọi truyện, không có story cụ thể.
      if (!story) return [['story', project], keys.project(project, 'voices'), keys.project(project, 'pronunciations')]
      if (parts.length === 0) return [keys.story(project, story), keys.project(project, 'stories')]
      const out: Key[] = []
      parts.forEach((part) => {
        if (part.startsWith('audio/')) {
          const name = part.slice(6)
          out.push(keys.story(project, story, 'audio', name === 'scripts' ? 'script' : name))
        } else {
          (STORY_PART_KEYS[part] ?? [part as StoryPart]).forEach((sub) => out.push(keys.story(project, story, sub)))
        }
      })
      // Tiêu đề/trạng thái truyện hiện ở danh sách truyện của dự án.
      if (parts.includes('meta') || parts.includes('snapshot')) out.push(keys.project(project, 'stories'))
      if (parts.includes('rules')) out.push(keys.rules('story', project, story))
      return out.filter((key, index) => !out.some((other, otherIndex) =>
        otherIndex !== index && matchesPrefix(key, other) && (other.length < key.length || otherIndex < index)))
    }
    // §AI — chương vừa viết lại theo góp ý: làm mới mốc neo của chương đó và danh sách
    // chương (badge trên hàng); chữ chương đi qua story.changed 'chapters' cùng lúc.
    case 'comments.reanchored': {
      if (!project || !story) return []
      const out: Key[] = [keys.story(project, story, 'chapters'), keys.story(project, story, 'comments')]
      const chapter = typeof data.chapter === 'number' ? data.chapter : null
      if (chapter !== null) out.push(keys.story(project, story, 'comments', chapter))
      return out
    }
    case 'config.changed': {
      const scope = str(data.scope)
      const head = data.section === 'rules' ? 'rules' : 'config'
      // Cấu hình hiệu lực của scope dưới gộp từ scope trên → toàn cục đổi thì mọi scope đổi.
      if (scope === 'oauth') return []
      if (scope === 'global' || !project) return head === 'rules' ? [[head]] : [[head], keys.tts()]
      if (scope === 'project') return [[head, 'project', project], [head, 'story', project]]
      return story ? [[head, 'story', project, story]] : [[head, 'story', project]]
    }
    case 'account.changed': {
      const username = str(data.username)
      if (me && username === me) return '*'
      return [keys.users(), keys.sessions(), ['admin-audit']]
    }
    case 'session.revoked':
      // DataProvider xử lý phiên hiện tại trực tiếp; phiên khác chỉ làm mới danh sách thiết bị.
      return [keys.sessions(), ['admin-audit']]
    case 'notify.changed':
      return [['notifications'], ['notify-prefs'], ['push-subscriptions']]
    case 'job.created': case 'job.progress': case 'job.waiting': case 'job.failed': case 'job.removed':
      return [keys.jobs()]
    case 'job.done':
      return [keys.jobs(), ...jobDoneKeys(event.data as Partial<Job>)]
    default:
      return []
  }
}

/** Việc xong → dữ liệu nó tạo ra (máy chủ cũng phát story.changed; đây là lưới an toàn). */
function jobDoneKeys(job: Partial<Job>): Key[] {
  if (!job.project || !job.story) return job.kind === 'audio' ? [keys.tts()] : []
  switch (job.kind) {
    case 'export': return [keys.story(job.project, job.story, 'exports'), keys.story(job.project, job.story, 'audio', 'exports')]
    case 'audio': return ['volume', 'queue', 'script', 'peaks'].map(part => keys.story(job.project!, job.story!, 'audio', part))
    default: return []
  }
}

/** Khoá đang hiển thị → danh sách theo dõi gửi server (`p/s` cho truyện, `p` cho dự án). */
export function watchListOf(active: readonly Key[]): string[] {
  const out = new Set<string>()
  active.forEach((key) => {
    if (key[0] === 'story' && typeof key[1] === 'string' && typeof key[2] === 'string') out.add(`${key[1]}/${key[2]}`)
    else if (key[0] === 'project' && typeof key[1] === 'string') out.add(key[1])
    else if ((key[0] === 'rules' || key[0] === 'config') && typeof key[2] === 'string') {
      if (key[1] === 'story' && typeof key[3] === 'string') out.add(`${key[2]}/${key[3]}`)
      else if (key[1] === 'project') out.add(key[2])
    }
  })
  return [...out].sort()
}
