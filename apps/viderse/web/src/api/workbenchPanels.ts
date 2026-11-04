// Bảng bên phải của Bàn dựng: nội dung (metadata), lịch sử sửa, và clip nền
// (GET/POST /api/clone/:id/metadata|metadata/history|backdrop). Hợp đồng
// 07-data-contract §7/§20.
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: WorkbenchPanels nằm trong
// chunk của màn đầu nên tệp này không được kéo thư viện schema (và không kéo
// hợp đồng của trình dựng `lib/clone/clone-view` — chỉ bốn trường của nó được
// dùng ở đây).
import { bool, list, maybe, num, oneOf, record, str } from './guard'
import { mutate, request } from './request'
import { readReason, type SuggestionReason } from './suggestions'

const AGENTS = ['scout', 'director', 'artist', 'qa'] as const

const nullableStr = (value: unknown, path: string) => (value === null ? null : str(value, path))
const optionalStr = (value: unknown, path: string) => (value == null ? undefined : str(value, path))
const optionalBool = (value: unknown, path: string) => (value == null ? undefined : bool(value, path))

const readFlagSet = <T extends string>(value: unknown, allowed: readonly T[], path: string) => {
  const o = record(value, path)
  const pick = (key: T) => optionalBool(o[key], `${path}.${key}`)
  return Object.fromEntries(allowed.map((key) => [key, pick(key)]).filter(([, flag]) => flag !== undefined)) as Partial<Record<T, boolean>>
}

export type MetadataPartName = 'title' | 'description' | 'tags'

const PARTS = ['title', 'description', 'tags'] as const

export type Metadata = {
  title: string
  description: string
  tags: string[]
  editedBy?: string
  editedAt?: string
  edited?: Partial<Record<MetadataPartName, boolean>>
  reason?: Partial<Record<MetadataPartName, SuggestionReason>>
}

const readMetadata = (value: unknown, path: string): Metadata => {
  const o = record(value, path)
  const edited = o.edited == null ? undefined : readFlagSet(o.edited, PARTS, `${path}.edited`)
  const reason = o.reason == null ? undefined : (() => {
    const o2 = record(o.reason, `${path}.reason`)
    const out: Partial<Record<MetadataPartName, SuggestionReason>> = {}
    for (const part of PARTS) {
      const one = maybe(o2[part], readReason, `${path}.reason.${part}`)
      if (one) out[part] = one
    }
    return out
  })()
  const editedBy = optionalStr(o.editedBy, `${path}.editedBy`)
  const editedAt = optionalStr(o.editedAt, `${path}.editedAt`)
  return {
    title: str(o.title, `${path}.title`),
    description: str(o.description, `${path}.description`),
    tags: list(o.tags, str, `${path}.tags`),
    ...(editedBy === undefined ? {} : { editedBy }),
    ...(editedAt === undefined ? {} : { editedAt }),
    ...(edited === undefined ? {} : { edited }),
    ...(reason === undefined ? {} : { reason }),
  }
}

export type MetaResponse = {
  status: 'pending' | 'writing' | 'failed' | 'ai'
  metadata: Metadata | null
  revision: number
  descriptionFooter?: string | null
  alwaysTags?: string[] | null
}

const readMeta = (value: unknown): MetaResponse => {
  const o = record(value, 'metadata')
  const footer = o.descriptionFooter === null ? null : optionalStr(o.descriptionFooter, 'metadata.descriptionFooter')
  return {
    status: oneOf(o.status, ['pending', 'writing', 'failed', 'ai'] as const, 'metadata.status'),
    metadata: o.metadata == null ? null : readMetadata(o.metadata, 'metadata.metadata'),
    revision: num(o.revision, 'metadata.revision'),
    ...(footer === undefined ? {} : { descriptionFooter: footer }),
    ...(o.alwaysTags == null ? {} : { alwaysTags: list(o.alwaysTags, str, 'metadata.alwaysTags') }),
  }
}

export type History = Metadata & { at: string; by: 'director' | 'owner'; parts: MetadataPartName[] }

const readHistory = (value: unknown): History[] => {
  const o = record(value, 'history')
  if (o.history == null) return []
  return list(o.history, (entry, at) => {
    const row = record(entry, at)
    return {
      ...readMetadata(entry, at),
      at: str(row.at, `${at}.at`),
      by: oneOf(row.by, ['director', 'owner'] as const, `${at}.by`),
      parts: list(row.parts, (part, index) => oneOf(part, PARTS, `${at}.parts[${index}]`), `${at}.parts`),
    }
  }, 'history.history')
}

export type BackdropItem = {
  id: string
  channelId: string
  platform: 'twitch' | 'bilibili' | null
  channelHandle: string | null
  channelTitle: string | null
  videoId: string
  kind: 'clip' | 'highlight' | 'vod' | 'video'
  url: string
  title: string | null
  thumbnailUrl: string | null
  videoDurationSec: number | null
  startSec: number
  endSec: number
  source: 'random' | 'owner'
  status: 'pending' | 'ready' | 'failed'
  mediaId: string | null
  error: { code: string; message: string } | null
}

export type BackdropView = {
  revision: string
  wantedCount: number
  readyCount: number
  fallback: 'none' | 'template-clips'
  randomPicks: boolean
  busy: boolean
  coverage: { mode: 'fill' | 'count'; videoSec: number | null; fixedSec: number; channelSec: number }
  items: BackdropItem[]
  warnings: string[]
  playbackSpeed: number
  fixedItems: { id: string; title: string | null; startSec: number; endSec: number; playbackSec: number }[]
  sources: { channelId: string; platform: string; handle: string; title: string | null }[]
}

const readBackdropItem = (value: unknown, path: string): BackdropItem => {
  const o = record(value, path)
  return {
    id: str(o.id, `${path}.id`),
    channelId: str(o.channelId, `${path}.channelId`),
    platform: o.platform === null ? null : oneOf(o.platform, ['twitch', 'bilibili'] as const, `${path}.platform`),
    channelHandle: nullableStr(o.channelHandle, `${path}.channelHandle`),
    channelTitle: nullableStr(o.channelTitle, `${path}.channelTitle`),
    videoId: str(o.videoId, `${path}.videoId`),
    kind: oneOf(o.kind, ['clip', 'highlight', 'vod', 'video'] as const, `${path}.kind`),
    url: str(o.url, `${path}.url`),
    title: nullableStr(o.title, `${path}.title`),
    thumbnailUrl: nullableStr(o.thumbnailUrl, `${path}.thumbnailUrl`),
    videoDurationSec: o.videoDurationSec === null ? null : num(o.videoDurationSec, `${path}.videoDurationSec`),
    startSec: num(o.startSec, `${path}.startSec`),
    endSec: num(o.endSec, `${path}.endSec`),
    source: oneOf(o.source, ['random', 'owner'] as const, `${path}.source`),
    status: oneOf(o.status, ['pending', 'ready', 'failed'] as const, `${path}.status`),
    mediaId: nullableStr(o.mediaId, `${path}.mediaId`),
    error: o.error == null ? null : (() => {
      const err = record(o.error, `${path}.error`)
      return { code: str(err.code, `${path}.error.code`), message: str(err.message, `${path}.error.message`) }
    })(),
  }
}

const readBackdrop = (value: unknown, path: string): BackdropView => {
  const o = record(value, path)
  const coverage = record(o.coverage, `${path}.coverage`)
  return {
    revision: str(o.revision, `${path}.revision`),
    wantedCount: num(o.wantedCount, `${path}.wantedCount`),
    readyCount: num(o.readyCount, `${path}.readyCount`),
    fallback: oneOf(o.fallback, ['none', 'template-clips'] as const, `${path}.fallback`),
    randomPicks: bool(o.randomPicks, `${path}.randomPicks`),
    busy: bool(o.busy, `${path}.busy`),
    coverage: {
      mode: oneOf(coverage.mode, ['fill', 'count'] as const, `${path}.coverage.mode`),
      videoSec: coverage.videoSec === null ? null : num(coverage.videoSec, `${path}.coverage.videoSec`),
      fixedSec: num(coverage.fixedSec, `${path}.coverage.fixedSec`),
      channelSec: num(coverage.channelSec, `${path}.coverage.channelSec`),
    },
    items: list(o.items, readBackdropItem, `${path}.items`),
    warnings: o.warnings == null ? [] : list(o.warnings, str, `${path}.warnings`),
    playbackSpeed: num(o.playbackSpeed, `${path}.playbackSpeed`),
    fixedItems: list(o.fixedItems, (entry, at) => {
      const item = record(entry, at)
      return {
        id: str(item.id, `${at}.id`),
        title: nullableStr(item.title, `${at}.title`),
        startSec: num(item.startSec, `${at}.startSec`),
        endSec: num(item.endSec, `${at}.endSec`),
        playbackSec: num(item.playbackSec, `${at}.playbackSec`),
      }
    }, `${path}.fixedItems`),
    sources: list(o.sources, (entry, at) => {
      const source = record(entry, at)
      return {
        channelId: str(source.channelId, `${at}.channelId`),
        platform: str(source.platform, `${at}.platform`),
        handle: str(source.handle, `${at}.handle`),
        title: nullableStr(source.title, `${at}.title`),
      }
    }, `${path}.sources`),
  }
}

const url = (id: string, suffix: string) => `/api/clone/${encodeURIComponent(id)}/${suffix}`
export async function getMetadata(id: string, signal?: AbortSignal) { return readMeta(await request<unknown>(url(id, 'metadata'), { signal })) }
export async function mutateMetadata(id: string, action: '' | 'rewrite' | 'restore', body: object) { return readMeta(await mutate<unknown>(action ? 'POST' : 'PATCH', url(id, `metadata${action ? `/${action}` : ''}`), body)) }
export async function getHistory(id: string) { return readHistory(await request<unknown>(url(id, 'metadata/history'))) }
export async function getBackdrop(id: string, signal?: AbortSignal) {
  const o = record(await request<unknown>(url(id, 'backdrop'), { signal }), 'backdrop')
  return o.backdrop == null ? null : readBackdrop(o.backdrop, 'backdrop.backdrop')
}
export async function mutateBackdrop(id: string, body: object) { return mutate('POST', url(id, 'backdrop'), body) }
