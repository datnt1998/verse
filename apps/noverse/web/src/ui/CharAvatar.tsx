// Avatar nhân vật — component duy nhất cho mọi danh sách Nhân vật (04-components.md ›
// *Avatar nhân vật*, Gói S). Màu theo loại vai (token `kind`), viền nét đứt khi chưa lộ,
// huy hiệu góc cho Chính/Phản diện/Đồng minh, ẩn huy hiệu ở cỡ ≤ 20 px.
import type { CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import type { CharacterKind } from '../api/types'
import { charInitials } from '../lib/cast'
import { KIND_META, KIND_ORDER, kindAriaLabel, kindLabel, type KindMeta } from '../lib/charKind'
import { brain, kind } from '@noverse/ui/tokens.stylex'
import { Icon } from '@noverse/ui'

/**
 * Hình học huy hiệu góc, suy từ cỡ avatar (artboard `.cav-b` / `.cav.lg .cav-b`):
 * 13 px viền 1,5 ở cỡ < 40 px và 16 px ở cỡ ≥ 40 px; đặt lệch 3 / 2 px tính từ **hộp đệm**
 * (trong viền 1,5 px của avatar) nên mép ngoài huy hiệu nhô ra 1,5 / 0,5 px ở góc dưới phải.
 * `dx` = khoảng cách tâm huy hiệu → tâm avatar theo mỗi trục, dùng chung cho DOM và SVG.
 * `null` = không vẽ huy hiệu (avatar ≤ 20 px: chip kế hoạch / chip nộp chương).
 */
export const badgeGeometry = (px: number) => {
  if (px <= 20) return null
  const large = px >= 40
  const size = large ? 16 : 13
  const offset = large ? 2 : 3
  return { size, icon: large ? 10 : 8, offset, border: 1.5, dx: px / 2 - 1.5 + offset - size / 2 }
}

/** `sm` 20 px (chip kế hoạch / chip nộp chương) · `md` 26 px (danh sách) · `lg` 40 px (ngăn chi tiết). */
export type CharAvatarSize = 'sm' | 'md' | 'lg'

const PX: Record<CharAvatarSize, number> = { sm: 20, md: 26, lg: 40 }
const FONT: Record<CharAvatarSize, number> = { sm: 8, md: 10, lg: 13 }

const s = stylex.create({
  base: {
    position: 'relative', flexShrink: 0, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    borderRadius: '50%', fontWeight: 600, lineHeight: 1, borderWidth: 1.5, borderStyle: 'solid',
    boxSizing: 'border-box', overflow: 'visible',
  },
  lead: { borderColor: kind['--kind-lead-bd'], backgroundColor: kind['--kind-lead-bg'], color: kind['--kind-lead-fg'] },
  antagonist: { borderColor: kind['--kind-anta-bd'], backgroundColor: kind['--kind-anta-bg'], color: kind['--kind-anta-fg'] },
  ally: { borderColor: kind['--kind-ally-bd'], backgroundColor: kind['--kind-ally-bg'], color: kind['--kind-ally-fg'] },
  support: { borderColor: kind['--kind-sup-bd'], backgroundColor: kind['--kind-sup-bg'], color: kind['--kind-sup-fg'] },
  /** Chưa lộ với người nghe: viền nét đứt, vẫn giữ màu loại. */
  hidden: { borderStyle: 'dashed' },
  /**
   * Nút không liên quan khi chọn một nút ở đồ thị: giữ nền đặc rồi làm mờ bằng filter —
   * KHÔNG giảm opacity (04 › “Mờ đi”), vì cạnh phía sau sẽ lộ qua.
   */
  dim: { filter: 'saturate(0) brightness(.55)' },
  badge: {
    position: 'absolute', borderRadius: '50%', backgroundColor: kind['--kind-badge-bg'],
    borderWidth: 1.5, borderStyle: 'solid', boxSizing: 'border-box', flexShrink: 0, zIndex: 1,
  },
  /** Icon căn giữa tuyệt đối theo tâm huy hiệu — không phụ thuộc baseline/flex của chữ. */
  badgeIcon: { position: 'absolute', inset: 0, margin: 'auto', display: 'block' },
})

export type CharAvatarProps = {
  name: string
  /** chữ viết tắt người dùng tự đặt (thắng luật tính từ tên) */
  ini?: string | null
  kind?: CharacterKind
  /** chưa lộ với người nghe ở chương đang xem → viền nét đứt */
  hidden?: boolean
  /** nút ngoài mục chọn ở đồ thị Não truyện → nền đặc + filter */
  dim?: boolean
  size?: CharAvatarSize | number
  style?: CSSProperties
  className?: string
}

export function CharAvatar({
  name, ini, kind: kindKey = 'support', hidden = false, dim = false, size = 'md', style, className,
}: CharAvatarProps) {
  const px = typeof size === 'number' ? size : PX[size]
  const meta: KindMeta = KIND_META[kindKey]
  const sx = stylex.props(s.base, s[kindKey], hidden && s.hidden, dim && s.dim)
  const badge = meta.icon ? badgeGeometry(px) : null
  const badgeBox = stylex.props(s.badge)
  const badgeIcon = stylex.props(s.badgeIcon)
  return (
    <span
      role="img"
      aria-label={kindAriaLabel(name, kindKey, hidden)}
      className={[sx.className, className].filter(Boolean).join(' ')}
      style={{
        ...sx.style,
        width: px,
        height: px,
        // 26 → 10 px, 40 → 13 px; cỡ số khác (44, 46, 52) theo cùng tỉ lệ, sàn 8 px cho chip 18–20 px.
        fontSize: typeof size === 'number' ? Math.max(8, Math.round(px * 0.31)) : FONT[size],
        ...style,
      }}
    >
      {charInitials(name, ini)}
      {badge && meta.icon && (
        <span aria-hidden="true" className={badgeBox.className}
          style={{
            ...badgeBox.style,
            width: badge.size, height: badge.size, right: -badge.offset, bottom: -badge.offset,
            borderColor: meta.border, color: meta.border,
          }}>
          <Icon name={meta.icon} size={badge.icon} className={badgeIcon.className} style={badgeIcon.style} />
        </span>
      )}
    </span>
  )
}

/**
 * Chip loại vai cạnh tên trong ngăn chi tiết (artboard: viền + chữ cùng màu loại,
 * bo tròn 999, 11 px, “· chưa lộ” khi chương đang xem chưa tới `revealAt`).
 */
export function KindChip({ kind: kindKey, hidden = false }: { kind: CharacterKind; hidden?: boolean }) {
  const meta = KIND_META[kindKey]
  const sx = stylex.props(chipStyles.base)
  return (
    <span className={sx.className}
      style={{ ...sx.style, borderColor: meta.border, color: meta.fg }}>
      {kindLabel(kindKey, hidden)}
    </span>
  )
}

const chipStyles = stylex.create({
  base: {
    display: 'inline-flex', alignItems: 'center', paddingInline: 7, paddingBlock: 1, borderRadius: 999,
    borderWidth: 1, borderStyle: 'solid', fontSize: 11, fontWeight: 500, whiteSpace: 'nowrap',
  },
})

const legendStyles = stylex.create({
  row: {
    display: 'flex', flexWrap: 'wrap', gap: '4px 12px', paddingTop: 6, borderTopWidth: 1, borderTopStyle: 'solid',
    fontSize: 11, color: brain['--brain-muted'],
  },
  item: { display: 'inline-flex', alignItems: 'center', gap: 5, whiteSpace: 'nowrap' },
  dot: {
    width: 9, height: 9, borderRadius: '50%', borderWidth: 1.5, borderStyle: 'solid', boxSizing: 'border-box',
  },
})

const LEGEND_ROWS = [
  ...KIND_ORDER.map((key) => ({ key, label: KIND_META[key].label, meta: KIND_META[key], hidden: false })),
  { key: 'support' as const, label: 'Chưa lộ', meta: KIND_META.support, hidden: true },
]

/**
 * Chú giải một dòng “Chính · Phản diện · Đồng minh · Phụ · Chưa lộ” — dưới danh sách
 * Nhân vật và trong ngăn kéo Não truyện (artboard `.kdot`, 04 › “Chú giải”).
 */
export function KindLegend({ showHidden = true, style }: { showHidden?: boolean; style?: CSSProperties }) {
  const row = stylex.props(legendStyles.row)
  return (
    <div aria-hidden="true" className={row.className} style={{ ...row.style, ...style }}>
      {LEGEND_ROWS.filter((entry) => showHidden || !entry.hidden).map(({ key, label, meta, hidden }) => (
        <span key={`${key}-${label}`} {...stylex.props(legendStyles.item)}>
          <span {...stylex.props(legendStyles.dot)}
            style={{ backgroundColor: meta.background, borderColor: meta.border, borderStyle: hidden ? 'dashed' : 'solid' }} />
          {label}
        </span>
      ))}
    </div>
  )
}
