// Nhãn kế thừa (“Riêng truyện này” · “Theo dự án”…), đếm thay đổi cho thanh nổi đáy, chương có hiệu lực.
import type { Config, ConfigScope, Snapshot } from '../../../api/types'
import { getPath, isJsonObject, pruneEmpty, type JsonObject } from './config'

export type Ovr = { cls: 'own' | 'inh'; text: string; reset: boolean }

/**
 * Nhãn của một thiết lập (Settings.dc.html `ovr`): cấp toàn cục luôn “Mặc định”; cấp dự án/truyện có khoá
 * ghi đè ở chính cấp đó → “Riêng …” kèm nút *Theo dự án* / *Theo “Mọi truyện”* (xoá khoá); còn lại là nhãn xám theo
 * cấp thật sự cung cấp giá trị (`ConfigResponse.sources`).
 */
export function ovrOf(scope: ConfigScope, own: boolean, inheritedFrom?: ConfigScope): Ovr {
  if (scope === 'global') return { cls: 'inh', text: 'Mặc định', reset: false }
  if (own) return { cls: 'own', text: scope === 'story' ? 'Riêng truyện này' : 'Riêng dự án', reset: true }
  const fromGlobal = scope === 'project' || inheritedFrom === 'global'
  return { cls: 'inh', text: fromGlobal ? 'Theo “Mọi truyện”' : 'Theo dự án', reset: false }
}

export const resetLabel = (scope: ConfigScope): string => (scope === 'story' ? 'Theo dự án' : 'Theo “Mọi truyện”')

const RANK: Record<ConfigScope, number> = { global: 0, project: 1, story: 2 }

/**
 * Cấp đang cung cấp giá trị kế thừa cho nhóm khoá: ưu tiên `ConfigResponse.sources` (cấp khác cấp đang sửa); nếu không
 * (khoá vừa bị xoá khỏi bản nháp nên nguồn vẫn là chính cấp này) thì quét chuỗi `chain` từ cấp gần nhất xuống.
 */
export function inheritedFrom(
  sources: Record<string, ConfigScope> | undefined, paths: string[], scope: ConfigScope,
  chain: { scope: ConfigScope; config: Config }[] = [],
): ConfigScope | undefined {
  for (const path of paths) {
    const found = sources?.[path]
    if (found && found !== scope) return found
  }
  const below = chain.filter((doc) => RANK[doc.scope] < RANK[scope]).sort((a, b) => RANK[b.scope] - RANK[a.scope])
  for (const doc of below) if (paths.some((path) => getPath(doc.config, path) !== undefined)) return doc.scope
  return undefined
}

/** Có khoá nào trong `paths` được ghi ở tệp của cấp đang chọn (bản nháp) không. */
export const hasOwn = (draft: Config, paths: string[]): boolean => paths.some((path) => getPath(draft, path) !== undefined)

function flatten(value: unknown, prefix: string, out: Map<string, string>) {
  if (isJsonObject(value)) {
    for (const [key, child] of Object.entries(value as JsonObject)) flatten(child, prefix ? `${prefix}.${key}` : key, out)
    return
  }
  out.set(prefix, JSON.stringify(value))
}

/** `roles.writer.model` → `roles.writer`; `providers.openai.models` → `providers.openai`; khoá khác → khoá gốc. */
const settingOf = (path: string): string => {
  const parts = path.split('.')
  return parts[0] === 'roles' || parts[0] === 'providers' ? parts.slice(0, 2).join('.') : parts[0]
}

/** Số thiết lập đã đổi giữa tệp đã lưu và bản nháp. */
export function countChanges(baseline: unknown, draft: unknown): number {
  const before = new Map<string, string>()
  const after = new Map<string, string>()
  flatten(pruneEmpty(baseline), '', before)
  flatten(pruneEmpty(draft), '', after)
  const changed = new Set<string>()
  for (const key of new Set([...before.keys(), ...after.keys()])) if (before.get(key) !== after.get(key)) changed.add(settingOf(key))
  return changed.size
}

/** Chương mà quy tắc/thiết lập mới có hiệu lực: sau chương đang viết dở (nếu có) hoặc chương đã xong cuối cùng. */
export const effectiveFromChapter = (snapshot: Pick<Snapshot, 'completedCount' | 'inProgressChapter'> | null | undefined): number =>
  Math.max(snapshot?.completedCount ?? 0, snapshot?.inProgressChapter ?? 0) + 1
