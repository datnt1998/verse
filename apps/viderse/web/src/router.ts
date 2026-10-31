// Hash router có kiểu theo design 02 §4. Hash không hợp lệ hoặc tham số query
// sai kiểu đều rơi về Bàn dựng — App chuẩn hoá lại thanh địa chỉ sau khi parse.
import { useEffect, useState } from 'react'

export type Route =
  | { name: 'workbench'; channel?: string; video?: string }            // '#/'
  | { name: 'review'; channel?: string }                               // '#/review'
  | { name: 'queue' }                                                  // '#/queue'
  | { name: 'channel'; channelId: string; tab?: ChannelTab }           // '#/channels/<id>'
  | { name: 'channel-new'; step?: ChannelNewStep }                     // '#/channels/new'
  | { name: 'explore'; rival?: string; video?: string }                // '#/explore'
  | { name: 'new' }                                                    // '#/new'
  | { name: 'library'; tab?: LibraryTab; prompt?: PromptView; nguon?: BackdropSource; kenh?: string } // '#/library'
  | { name: 'template'; templateId: string; section?: TemplateSection }// '#/templates/<id>'
  | { name: 'editor'; projectId: string; fix?: string; cue?: string }    // '#/editor/<projectId>'
  | { name: 'render'; requestId: string; token?: string }
  | { name: 'measure'; requestId: string; token?: string }
  | { name: 'settings'; section?: SettingsSection }                    // '#/settings'
  | { name: 'setup' }                                                  // '#/setup'
  | { name: 'account' }                                                // '#/account'
  | { name: 'devkit'; board?: DevKitBoard; section?: string }          // '#/dev/kit' (chỉ dev)

export const CHANNEL_TABS = ['videos', 'sources', 'templates', 'settings'] as const
export type ChannelTab = (typeof CHANNEL_TABS)[number]

export const CHANNEL_NEW_STEPS = ['create', 'done'] as const
export type ChannelNewStep = (typeof CHANNEL_NEW_STEPS)[number]

export const LIBRARY_TABS = ['templates', 'prompts', 'backdrops', 'overlays'] as const
export type LibraryTab = (typeof LIBRARY_TABS)[number]

/** Màn con của tab Prompt ảnh: danh sách, hoặc mở sẵn một prompt. */
export const PROMPT_VIEWS = ['list', 'mine', 'builtin', 'new'] as const
export type PromptView = (typeof PROMPT_VIEWS)[number]

export const TEMPLATE_SECTIONS = ['images', 'cover', 'backdrop', 'output'] as const
export type TemplateSection = (typeof TEMPLATE_SECTIONS)[number]

/** Nguồn đang mở của tab Video nền (?nguon=): 'videos' = trang Video lẻ,
 *  'channels'/vắng = trang kênh `kenh` (id kênh nền, vd "twitch:nightdrivefm"). */
export const BACKDROP_SOURCES = ['channels', 'videos'] as const
export type BackdropSource = (typeof BACKDROP_SOURCES)[number]

export const SETTINGS_SECTIONS = ['tools', 'connect', 'agents', 'accounts', 'storage', 'logs'] as const
export type SettingsSection = (typeof SETTINGS_SECTIONS)[number]

/** Alias mục cũ của bản yt-factory — link đã lưu vẫn mở đúng mục mới. */
const LEGACY_SECTIONS: Record<string, SettingsSection> = { images: 'agents' }

export const DEVKIT_BOARDS = ['kit', 'mkit'] as const
export type DevKitBoard = (typeof DEVKIT_BOARDS)[number]

export type Parsed = { route: Route; search: URLSearchParams }

const HOME: Route = { name: 'workbench' }

const oneOf = <T extends string>(value: string | null, allowed: readonly T[]): T | undefined =>
  allowed.includes(value as T) ? (value as T) : undefined
// URLSearchParams.get() đã giải mã một lần — KHÔNG decodeURIComponent nữa
// (hash `?channel=%25` sẽ ném URIError; `%2525` cũng bị giải hai lần).
const text = (value: string | null) => (value ? value : undefined)

export function parseHash(hash: string): Parsed {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash
  const [pathPart, queryPart] = raw.split('?')
  const search = new URLSearchParams(queryPart ?? '')
  const seg = pathPart.split('/').filter(Boolean).map((part) => {
    try { return decodeURIComponent(part) } catch { return part }
  })

  let route: Route
  if (seg.length === 0) {
    route = { name: 'workbench', channel: text(search.get('channel')), video: text(search.get('video')) }
  } else if (seg[0] === 'review' && seg.length === 1) {
    route = { name: 'review', channel: text(search.get('channel')) }
  } else if (seg[0] === 'queue' && seg.length === 1) {
    route = { name: 'queue' }
  } else if (seg[0] === 'channels' && seg[1] === 'new' && seg.length === 2) {
    route = { name: 'channel-new', step: oneOf(search.get('step'), CHANNEL_NEW_STEPS) }
  } else if (seg[0] === 'channels' && seg.length === 2 && seg[1]) {
    route = { name: 'channel', channelId: seg[1], tab: oneOf(search.get('tab'), CHANNEL_TABS) }
  } else if (seg[0] === 'explore' && seg.length === 1) {
    route = { name: 'explore', rival: text(search.get('rival')), video: text(search.get('video')) }
  } else if (seg[0] === 'new' && seg.length === 1) {
    route = { name: 'new' }
  } else if (seg[0] === 'library' && seg.length === 1) {
    route = {
      name: 'library',
      tab: oneOf(search.get('tab'), LIBRARY_TABS),
      prompt: oneOf(search.get('prompt'), PROMPT_VIEWS),
      nguon: oneOf(search.get('nguon'), BACKDROP_SOURCES),
      kenh: text(search.get('kenh')),
    }
  } else if (seg[0] === 'templates' && seg.length === 2 && seg[1]) {
    route = { name: 'template', templateId: seg[1], section: oneOf(search.get('section'), TEMPLATE_SECTIONS) }
  } else if (seg[0] === 'editor' && seg.length === 2 && seg[1]) {
    route = { name: 'editor', projectId: seg[1], ...(search.get('fix') && { fix: text(search.get('fix')) }), ...(search.get('cue') && { cue: text(search.get('cue')) }) }
  } else if ((seg[0] === 'render' || seg[0] === 'measure') && seg.length === 2 && seg[1]) {
    route = { name: seg[0], requestId: seg[1], token: text(search.get('token')) }
  } else if (seg[0] === 'settings' && seg.length === 1) {
    route = { name: 'settings', section: oneOf(search.get('section'), SETTINGS_SECTIONS) ?? LEGACY_SECTIONS[search.get('section') ?? ''] }
  } else if (seg[0] === 'setup' && seg.length === 1) {
    route = { name: 'setup' }
  } else if (seg[0] === 'account' && seg.length === 1) {
    route = { name: 'account' }
  } else if (seg[0] === 'dev' && seg[1] === 'kit' && seg.length === 2 && import.meta.env.DEV) {
    route = { name: 'devkit', board: oneOf(search.get('board'), DEVKIT_BOARDS), section: text(search.get('section')) ?? undefined }
  } else {
    route = HOME
  }
  return { route, search }
}

/** Đường dẫn chuẩn của một route — dùng để nhận ra hash đã chuẩn hoá. */
export function routePath(route: Route): string {
  const q = (pairs: readonly (readonly [string, string | undefined])[]) => {
    const params = new URLSearchParams()
    for (const [key, value] of pairs) if (value) params.set(key, value)
    const s = params.toString()
    return s ? `?${s}` : ''
  }
  switch (route.name) {
    case 'workbench': return `/${q([['channel', route.channel], ['video', route.video]])}`
    case 'review': return `/review${q([['channel', route.channel]])}`
    case 'queue': return '/queue'
    case 'channel': return `/channels/${encodeURIComponent(route.channelId)}${q([['tab', route.tab]])}`
    case 'channel-new': return `/channels/new${q([['step', route.step]])}`
    case 'explore': return `/explore${q([['rival', route.rival], ['video', route.video]])}`
    case 'new': return '/new'
    case 'library': return `/library${q([['tab', route.tab], ['prompt', route.prompt], ['nguon', route.nguon], ['kenh', route.kenh]])}`
    case 'template': return `/templates/${encodeURIComponent(route.templateId)}${q([['section', route.section]])}`
    case 'editor': return `/editor/${encodeURIComponent(route.projectId)}${q([['fix', route.fix], ['cue', route.cue]])}`
    case 'render': return `/render/${encodeURIComponent(route.requestId)}${q([['token', route.token]])}`
    case 'measure': return `/measure/${encodeURIComponent(route.requestId)}${q([['token', route.token]])}`
    case 'settings': return `/settings${q([['section', route.section]])}`
    case 'setup': return '/setup'
    case 'account': return '/account'
    case 'devkit': return `/dev/kit${q([['board', route.board], ['section', route.section]])}`
  }
}

export const paths = {
  workbench: (opts?: { channel?: string; video?: string }) => routePath({ name: 'workbench', ...opts }),
  review: (opts?: { channel?: string }) => routePath({ name: 'review', ...opts }),
  queue: () => '/queue',
  channel: (channelId: string, tab?: ChannelTab) => routePath({ name: 'channel', channelId, tab }),
  channelNew: (step?: ChannelNewStep) => routePath({ name: 'channel-new', step }),
  explore: (opts?: { rival?: string; video?: string }) => routePath({ name: 'explore', ...opts }),
  new: () => '/new',
  library: (tab?: LibraryTab, prompt?: PromptView, backdrop?: { nguon?: BackdropSource; kenh?: string }) =>
    routePath({ name: 'library', tab, prompt, nguon: backdrop?.nguon, kenh: backdrop?.kenh }),
  template: (templateId: string, section?: TemplateSection) => routePath({ name: 'template', templateId, section }),
  editor: (projectId: string) => routePath({ name: 'editor', projectId }),
  settings: (section?: SettingsSection) => routePath({ name: 'settings', section }),
  setup: () => '/setup',
  account: () => '/account',
  devkit: (board?: DevKitBoard, section?: string) => routePath({ name: 'devkit', board, section }),
}

/** Mục MainNav tô sáng cho route đang mở; null = trang ngoài nav (Hàng chờ,
 *  Cài đặt, Tài khoản, Editor) — không mục nào sáng (artboard Queue/Settings).
 */
export type NavKey = 'workbench' | 'review' | 'channels' | 'explore' | 'new'
export function navSection(route: Route): NavKey | null {
  switch (route.name) {
    case 'workbench': return 'workbench'
    case 'review': return 'review'
    case 'channel': case 'channel-new': case 'template': case 'library': return 'channels'
    case 'explore': return 'explore'
    case 'new': return 'new'
    default: return null
  }
}

/**
 * Bộ đổi kênh (01 W5): ở Bàn dựng → đổi phạm vi = query `channel`
 * (đổi phạm vi không đóng video đang mở → giữ `video`); ở trang Kênh → đổi
 * kênh đang xem; "Mọi kênh" từ trang Kênh → về Bàn dựng.
 */
export function pickChannelRoute(route: Route, channelId: string | undefined): Route {
  if (route.name === 'channel' && channelId !== undefined) return { name: 'channel', channelId }
  if (route.name === 'channel') return { name: 'workbench' }
  return { name: 'workbench', channel: channelId, video: route.name === 'workbench' ? route.video : undefined }
}

/**
 * replaceState không bắn `hashchange` nên dispatch tay (giống Noverse);
 * gán location.hash thì trình duyệt tự bắn sự kiện.
 */
export function navigate(path: string, replace = false) {
  if (replace) {
    const next = new URL(location.href)
    next.hash = `#${path}`
    history.replaceState(null, '', next)
    dispatchEvent(new HashChangeEvent('hashchange'))
  } else {
    location.hash = `#${path}`
  }
}

export function useRoute(): Parsed {
  const [hash, setHash] = useState(location.hash)
  useEffect(() => {
    const change = () => setHash(location.hash)
    addEventListener('hashchange', change)
    return () => removeEventListener('hashchange', change)
  }, [])
  return parseHash(hash)
}
