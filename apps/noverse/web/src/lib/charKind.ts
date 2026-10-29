// Loại vai của nhân vật (Gói S · 04-components.md › *Avatar nhân vật* › *Màu theo loại vai*).
// Một nguồn cho mọi màn: avatar, nút đồ thị, chú giải, sắp xếp danh sách, nhãn a11y.
// Màu là biến token (tokens.stylex.ts › `kind`), không viết mã màu ở đây.
import type { CharacterKind } from '../api/types'
import { kind } from '@noverse/ui/tokens.stylex'

/** Thứ tự danh sách: Chính → Phản diện → Đồng minh → Phụ (04 › “Thứ tự danh sách”). */
export const KIND_ORDER: readonly CharacterKind[] = ['lead', 'antagonist', 'ally', 'support']

export type KindMeta = {
  /** nhãn trong chú giải, chip và thẻ rê */
  label: string
  /** tên biểu tượng huy hiệu góc; `null` = không có huy hiệu (Phụ) */
  icon: string | null
  border: string
  background: string
  fg: string
}

export const KIND_META: Record<CharacterKind, KindMeta> = {
  lead: {
    label: 'Chính', icon: 'magic-star',
    border: kind['--kind-lead-bd'], background: kind['--kind-lead-bg'], fg: kind['--kind-lead-fg'],
  },
  antagonist: {
    label: 'Phản diện', icon: 'danger',
    border: kind['--kind-anta-bd'], background: kind['--kind-anta-bg'], fg: kind['--kind-anta-fg'],
  },
  ally: {
    label: 'Đồng minh', icon: 'shield-tick',
    border: kind['--kind-ally-bd'], background: kind['--kind-ally-bg'], fg: kind['--kind-ally-fg'],
  },
  support: {
    label: 'Phụ', icon: null,
    border: kind['--kind-sup-bd'], background: kind['--kind-sup-bg'], fg: kind['--kind-sup-fg'],
  },
}

/** Thiếu `kind` (dữ liệu cũ) thì coi là `support` — 06 › *Bổ sung §S*. */
export const charKindOf = (value: Pick<{ kind?: CharacterKind }, 'kind'> | null | undefined): CharacterKind =>
  value?.kind !== undefined && value.kind in KIND_META ? value.kind : 'support'

/** Nhãn loại; thêm “ · chưa lộ” khi chương đang xem chưa tới `revealAt`. */
export const kindLabel = (kind: CharacterKind, hidden = false) =>
  hidden ? `${KIND_META[kind].label} · chưa lộ` : KIND_META[kind].label

/** `aria-label` của avatar: “{tên}, {loại}” (+ “, chưa lộ”). */
export const kindAriaLabel = (name: string, kind: CharacterKind, hidden = false) =>
  hidden ? `${name}, ${KIND_META[kind].label}, chưa lộ` : `${name}, ${KIND_META[kind].label}`

/** Chưa lộ với người nghe: có `revealAt` và chương đang xem nhỏ hơn chương đó. */
export const isKindHidden = (revealAt: number | null | undefined, chapter: number) =>
  typeof revealAt === 'number' && chapter > 0 && chapter < revealAt

/**
 * Sắp danh sách nhân vật theo loại; trong cùng loại giữ thứ tự xuất hiện
 * (`Array.prototype.sort` ổn định — artboard `ORDER` cũng chỉ so theo loại).
 */
export function sortByKind<T extends { kind?: CharacterKind }>(items: readonly T[]): T[] {
  const rankOf = (item: T) => {
    const index = KIND_ORDER.indexOf(charKindOf(item))
    return index < 0 ? KIND_ORDER.length : index
  }
  return [...items].sort((a, b) => rankOf(a) - rankOf(b))
}
