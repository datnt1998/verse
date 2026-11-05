// Kiểu của Kênh › số liệu đầu kênh (artboard Channel): 4 thẻ Video đã làm ·
// Đã đăng · Chờ bạn · Chờ lượt; trên điện thoại hợp thành một card 4 cột.
// Màu qua token; số đo lấy từ artboard ở 1440×900.
import * as stylex from '@stylexjs/stylex'
import { color, line, surface } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

const PHONE = '@media (max-width: 767px)'

export const videoStyles = stylex.create({
  stats: { display: 'grid', gridTemplateColumns: { default: 'repeat(auto-fit, minmax(min(150px, 100%), 1fr))', [PHONE]: 'repeat(4, minmax(0, 1fr))' }, gap: { default: space['--vv-s-10'], [PHONE]: 0 }, borderRadius: { default: null, [PHONE]: corners['--vv-r-14'] }, backgroundImage: { default: null, [PHONE]: surface['--card-bg'] }, backgroundColor: 'transparent', borderWidth: { default: null, [PHONE]: space['--vv-s-1'] }, borderStyle: { default: null, [PHONE]: 'solid' }, borderColor: { default: null, [PHONE]: line['--line-2'] }, boxShadow: { default: null, [PHONE]: surface['--card-shadow'] }, overflow: { default: null, [PHONE]: 'hidden' } },
  stat: {
    borderRadius: { default: corners['--vv-r-12'], [PHONE]: 0 }, padding: { default: 'var(--vv-s-10) var(--vv-s-14)', [PHONE]: 'var(--vv-s-10) var(--vv-s-4) var(--vv-s-9)' }, display: 'flex', flexDirection: 'column', alignItems: { default: null, [PHONE]: 'center' }, gap: { default: space['--vv-s-2'], [PHONE]: space['--vv-s-1'] },
    backgroundImage: { default: surface['--card-bg'], [PHONE]: 'none' }, backgroundColor: 'transparent', borderWidth: { default: space['--vv-s-1'], [PHONE]: 0 }, borderStyle: { default: 'solid', [PHONE]: 'none' }, borderColor: { default: line['--line-2'], [PHONE]: 'currentColor' }, boxShadow: { default: surface['--card-shadow'], [PHONE]: 'none' },
  },
  // Vách mờ giữa các cột của thẻ một mảnh trên điện thoại (MChannel › stats).
  statDivider: { borderLeftWidth: { default: 0, [PHONE]: space['--vv-s-1'] }, borderLeftStyle: { default: null, [PHONE]: 'solid' }, borderLeftColor: { default: null, [PHONE]: line['--line-1'] } },
  statLabel: { fontSize: { default: 12, [PHONE]: 11.5 }, color: 'var(--vv-fg-note)', order: { default: null, [PHONE]: 2 }, whiteSpace: { default: null, [PHONE]: 'nowrap' } },
  statValue: { fontSize: { default: 20, [PHONE]: 19 }, fontWeight: 600, letterSpacing: '-0.01em', color: color['--color-fg'], order: { default: null, [PHONE]: 1 } },
  statValueWarn: { color: color['--color-warn'] },
  statSub: { fontSize: 11.5, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', order: { default: null, [PHONE]: 3 }, display: { default: null, [PHONE]: 'none' } },
})
