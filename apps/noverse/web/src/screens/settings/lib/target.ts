// Mục + cấp áp dụng của Tuỳ chỉnh đọc từ đường dẫn: `#/settings/<mục>?scope=&project=&story=`
// (đúng dạng `settingsLink` của màn Dự án tạo ra); `?section=` được chấp nhận như đường dẫn dự phòng.
import type { ConfigScope } from '../../../api/types'

export const SECTION_IDS = ['rules', 'writing', 'models', 'budget', 'info', 'members', 'connect', 'export', 'notify', 'app', 'adv'] as const
export type SectionId = (typeof SECTION_IDS)[number]
/** Giọng đọc không có trong artboard Settings: chỉ mở bằng liên kết từ màn Audio, không có mục trong menu. */
export type PageId = SectionId | 'voice'

/** Nhóm HỆ THỐNG · QUẢN TRỊ của menu trái. */
export const SYSTEM_SECTIONS: readonly SectionId[] = ['members', 'connect', 'export', 'notify', 'app', 'adv']

/** Tên mục cũ (liên kết đã phát hành) → mục mới. */
const LEGACY: Record<string, PageId> = { providers: 'connect', roles: 'models', library: 'export', system: 'adv', style: 'writing', voice: 'voice' }

export const isSystemSection = (id: PageId): boolean => (SYSTEM_SECTIONS as readonly string[]).includes(id)

export function resolveSection(raw: string | null | undefined, admin: boolean): PageId {
  const id = raw ? (LEGACY[raw] ?? raw) : 'rules'
  const known = (SECTION_IDS as readonly string[]).includes(id) || id === 'voice'
  if (!known) return 'rules'
  if (isSystemSection(id as PageId) && !admin) return 'rules'
  return id as PageId
}

export type SettingsTarget = { section: string | null; scope: ConfigScope | null; project: string; story: string }

const SCOPES: readonly string[] = ['story', 'project', 'global']

export function parseSettingsTarget(section: string | null, search: URLSearchParams): SettingsTarget {
  const scope = search.get('scope')
  return {
    section: section ?? search.get('section'),
    scope: scope && SCOPES.includes(scope) ? (scope as ConfigScope) : null,
    project: search.get('project') ?? '',
    story: search.get('story') ?? '',
  }
}
