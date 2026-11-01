// Hash router — bảng route chuẩn ở docs/design/v1/02-ia-flows.md.
// Hộp thoại phủ dùng query trong hash: #/p/x/s/y/work?dlg=palette|reopen|locked|qr|devices
import { useEffect, useState } from 'react'
import { flushSync } from 'react-dom'

export type Route =
  | { name: 'project'; project: string | null }
  | { name: 'projbrain'; project: string }
  | { name: 'story'; project: string; story: string }
  | { name: 'workspace'; project: string; story: string }
  | { name: 'activity'; project: string; story: string }
  | { name: 'brain'; project: string; story: string }
  | { name: 'quality'; project: string; story: string }
  | { name: 'reader'; project: string; story: string; chapter: number | null }
  | { name: 'audio'; project: string; story: string; volume: number | null; mode: AudioMode }
  | { name: 'newstory'; project: string }
  | { name: 'settings'; section: string | null }
  | { name: 'sysbrain' }
  | { name: 'help'; cmd: string | null }
  | { name: 'notifications' }
  | { name: 'command'; job: string }
  | { name: 'mobile-link'; kind: 'read' | 'write' | 'listen'; id: string; chapter: number | null }
  | { name: 'login' }
  | { name: 'setup' }
  | { name: 'pair'; code: string }
  | { name: 'welcome'; step: string | null }
  | { name: 'kit' }
  | { name: 'missing' }

/** 02-ia-flows.md: bản đọc (review) · xác nhận/đang tạo (make) · nghe & xuất (listen) */
export type AudioMode = 'review' | 'make' | 'listen'

export type Parsed = { route: Route; dialog: string | null; search: URLSearchParams }

const dec = (part: string) => { try { return decodeURIComponent(part) } catch { return part } }

export function parseHash(hash: string): Parsed {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash
  const [pathPart, queryPart] = raw.split('?')
  const search = new URLSearchParams(queryPart ?? '')
  const seg = pathPart.split('/').filter(Boolean).map(dec)
  const dialog = search.get('dlg')
  const fail: Parsed = { route: { name: 'missing' }, dialog, search }

  if (seg.length === 0) return { route: { name: 'project', project: null }, dialog, search }
  if (import.meta.env.DEV && seg[0] === 'dev' && seg[1] === 'kit') return { route: { name: 'kit' }, dialog, search }
  if (seg[0] === 'login') return { route: { name: 'login' }, dialog, search }
  // GAP-41 · A0: máy chủ chưa có tài khoản nào → trang thiết lập thay cho Đăng nhập.
  if (seg[0] === 'setup') return { route: { name: 'setup' }, dialog, search }
  if (seg[0] === 'phone') { search.set('scan', '1'); return { route: { name: 'login' }, dialog, search } }
  if (seg[0] === 'write' && !seg[1]) return { route: { name: 'workspace', project: '', story: '' }, dialog, search }
  if (seg[0] === 'pair' && seg[1]) return { route: { name: 'pair', code: seg[1] }, dialog, search }
  if (seg[0] === 'welcome') return { route: { name: 'welcome', step: seg[1] ?? null }, dialog, search }
  if (seg[0] === 'settings') return { route: { name: 'settings', section: seg[1] ?? null }, dialog, search }
  // D15 · Não hệ thống (chỉ quản trị) — mở từ Tuỳ chỉnh › Hệ thống.
  if (seg[0] === 'system' && seg[1] === 'brain' && seg.length === 2) return { route: { name: 'sysbrain' }, dialog, search }
  if (seg[0] === 'notifications') return { route: { name: 'notifications' }, dialog, search }
  if (seg[0] === 'cmd' && seg[1]) return { route: { name: 'command', job: seg[1] }, dialog, search }
  if (seg[0] === 'more') {
    if (seg[1]) search.set('page', seg[1])
    return { route: { name: 'settings', section: 'mobile' }, dialog, search }
  }
  if ((seg[0] === 'read' || seg[0] === 'write' || seg[0] === 'listen') && seg[1]) {
    const chapter = Number(seg[2])
    return { route: { name: 'mobile-link', kind: seg[0], id: seg[1], chapter: Number.isInteger(chapter) && chapter > 0 ? chapter : null }, dialog, search }
  }
  // 13-commands.md §2: #/help/commands/:name? (không có tên = phần Cách dùng lệnh)
  if (seg[0] === 'help') {
    const cmd = seg[1] === 'commands' ? seg[2] ?? null : null
    return { route: { name: 'help', cmd }, dialog, search }
  }
  // Tương thích ngược: #/book/:id → #/p/_/s/:id/work (useRoute đổi hash trong effect)
  if (seg[0] === 'book' && seg[1]) {
    return { route: { name: 'workspace', project: '_', story: seg[1] }, dialog, search }
  }
  if (seg[0] === 'p' && seg[1]) {
    const project = seg[1]
    if (seg[2] === 'new' && seg.length === 3) return { route: { name: 'newstory', project }, dialog, search }
    if (seg[2] === 'brain' && seg.length === 3) return { route: { name: 'projbrain', project }, dialog, search }
    if (seg[2] === 's' && seg[3]) {
      const story = seg[3]
      switch (seg[4]) {
        case undefined: return { route: { name: 'story', project, story }, dialog, search }
        case 'work': return { route: { name: 'workspace', project, story }, dialog, search }
        case 'activity': return { route: { name: 'activity', project, story }, dialog, search }
        // D10 · Não truyện — chương đang xem đi qua query `ch` (giữ được khi mở từ trang Đọc).
        case 'brain': return { route: { name: 'brain', project, story }, dialog, search }
        // D13 · Chất lượng truyện — cách xem/bộ lọc đi qua query `view`, `level`, `scope`.
        case 'quality': return { route: { name: 'quality', project, story }, dialog, search }
        case 'read': {
          const n = seg[5] ? Number.parseInt(seg[5], 10) : NaN
          return { route: { name: 'reader', project, story, chapter: Number.isFinite(n) && n > 0 ? n : null }, dialog, search }
        }
        case 'audio': {
          const volume = seg[5] ? Number.parseInt(seg[5], 10) : NaN
          const tail = seg[6] ?? (seg[5] === 'make' || seg[5] === 'listen' ? seg[5] : undefined)
          if (tail && tail !== 'make' && tail !== 'listen') return fail
          return {
            route: {
              name: 'audio', project, story,
              volume: Number.isFinite(volume) && volume > 0 ? volume : null,
              mode: tail === 'make' || tail === 'listen' ? tail : 'review',
            },
            dialog, search,
          }
        }
        default: return fail
      }
    }
    if (seg.length === 2) return { route: { name: 'project', project }, dialog, search }
  }
  return fail
}

const hashOf = (path: string, dialog?: string | null) => `#${path}${dialog ? `?dlg=${dialog}` : ''}`

/** Tab taps publish the new hash in the click itself, without waiting for a hashchange task. */
export function navigateNow(path: string, replace = false, dialog?: string | null) {
  const next = new URL(location.href)
  next.hash = hashOf(path, dialog)
  if (next.href === location.href) return
  if (replace) history.replaceState(null, '', next)
  else history.pushState(null, '', next)
  // Batch all route subscribers into one commit, rather than flushing each listener's
  // whole screen tree separately.
  flushSync(() => dispatchEvent(new HashChangeEvent('hashchange')))
}

export function navigate(path: string, replace = false, dialog?: string | null) {
  const url = hashOf(path, dialog)
  if (replace) {
    const next = new URL(location.href); next.hash = url
    history.replaceState(null, '', next)
    dispatchEvent(new HashChangeEvent('hashchange'))
  } else {
    location.hash = url
  }
}

/** Mở/đóng hộp thoại trên route hiện tại (thay/bỏ ?dlg=). */
export function setDialog(dialog: string | null) {
  const { search } = parseHash(location.hash)
  const raw = (location.hash.startsWith('#') ? location.hash.slice(1) : location.hash).split('?')[0]
  if (dialog) search.set('dlg', dialog); else search.delete('dlg')
  const query = search.toString()
  location.hash = `#${raw}${query ? `?${query}` : ''}`
}

const PUBLIC_ROUTES: Partial<Record<Route['name'], true>> = { login: true, setup: true, pair: true, missing: true, kit: true }

/** Route công khai — không cần phiên đăng nhập. */
export const isPublicRoute = (route: Route) => PUBLIC_ROUTES[route.name] === true

/**
 * Chưa onboarding hoặc còn mật khẩu tạm → ép về #/welcome, TRỪ khi đang ở chính
 * luồng welcome (mọi bước) hoặc route công khai. Nếu không loại trừ welcome, mỗi
 * bước onboarding bị đẩy về bước 1. Mật khẩu tạm: máy chủ chặn mọi API khác nên
 * màn nào ngoài bước đổi mật khẩu cũng chỉ ra lỗi.
 */
export const needsWelcomeRedirect = (route: Route, user: { onboardedAt: string | null; mustChangePassword?: boolean } | null): boolean =>
  Boolean(user && (!user.onboardedAt || user.mustChangePassword) && !isPublicRoute(route) && route.name !== 'welcome')

type NavigationBlocker = () => boolean
const navigationBlockers = new Set<NavigationBlocker>()

/** Trình chặn trả true khi đã giữ URL cũ; router không đổi route cho hashchange đó. */
export function addNavigationBlocker(blocker: NavigationBlocker) {
  navigationBlockers.add(blocker)
  return () => { navigationBlockers.delete(blocker) }
}

export function useRoute(): Parsed {
  const [hash, setHash] = useState(location.hash)
  useEffect(() => {
    const change = () => {
      for (const blocked of navigationBlockers) if (blocked()) return
      // Urgent update: never defer a visited route behind a transition. Effect-driven
      // redirects keep normal batching; tab clicks flush the complete broadcast once.
      setHash(location.hash)
    }
    addEventListener('hashchange', change)
    return () => removeEventListener('hashchange', change)
  }, [])
  // #/book/:id cũ → thay hash sau khi render, không dispatch trong lúc parse
  useEffect(() => {
    const legacy = /^#?\/book\/([^/?]+)/.exec(hash)
    if (legacy) navigate(`/p/_/s/${legacy[1]}/work`, true)
  }, [hash])
  return parseHash(hash)
}

// Đường dẫn tiện ích — id dự án/truyện được encode (tên cũ có thể chứa ký tự lạ).
const enc = encodeURIComponent

export const paths = {
  setup: () => '/setup',
  project: (p: string) => `/p/${enc(p)}`,
  projectBrain: (p: string) => `/p/${enc(p)}/brain`,
  story: (p: string, s: string) => `/p/${enc(p)}/s/${enc(s)}`,
  workspace: (p: string, s: string) => `/p/${enc(p)}/s/${enc(s)}/work`,
  activity: (p: string, s: string) => `/p/${enc(p)}/s/${enc(s)}/activity`,
  /** D10 · Não truyện. `chapter` = chương đang xem (`?ch=N`), MBrain mở đúng chương đang đọc. */
  brain: (p: string, s: string, chapter?: number | null) => `/p/${enc(p)}/s/${enc(s)}/brain${chapter ? `?ch=${chapter}` : ''}`,
  /** D13 · Chất lượng truyện (Quality.dc.html). */
  quality: (p: string, s: string) => `/p/${enc(p)}/s/${enc(s)}/quality`,
  reader: (p: string, s: string, chapter?: number) => `/p/${enc(p)}/s/${enc(s)}/read${chapter ? `/${chapter}` : ''}`,
  /** Câu 11 — mở Trình đọc ở chương đó với con trỏ đặt sẵn trong ô bình luận chương (`?note=1`). */
  readerNote: (p: string, s: string, chapter: number) => `${paths.reader(p, s, chapter)}?note=1`,
  audio: (p: string, s: string, volume?: number | null, mode: AudioMode = 'review') =>
    `/p/${enc(p)}/s/${enc(s)}/audio${volume ? `/${volume}` : ''}${mode === 'review' ? '' : `/${mode}`}`,
  newstory: (p: string) => `/p/${enc(p)}/new`,
  settings: (section?: string) => `/settings${section ? `/${enc(section)}` : ''}`,
  /** D15 · Não hệ thống (chỉ quản trị). */
  sysbrain: () => '/system/brain',
  /** 13-commands.md §2 — không có tên lệnh = phần “Cách dùng lệnh”. */
  help: (cmd?: string | null) => `/help/commands${cmd ? `/${enc(cmd)}` : ''}`,
}
