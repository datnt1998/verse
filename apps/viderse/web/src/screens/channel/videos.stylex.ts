// Kiểu của Kênh › Video (artboard Channel): số liệu đầu kênh, bộ lọc, bảng,
// thanh chọn nhiều. Số đo lấy từ artboard ở 1440×900; màu qua token.
import * as stylex from '@stylexjs/stylex'
import { color, line, surface } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

const NARROW = '@media (max-width: 1380px)'
const PHONE = '@media (max-width: 767px)'
const ROW_HOVER = 'rgba(255, 255, 255, 0.035)'
const spin = stylex.keyframes({ to: { transform: 'rotate(360deg)' } })
export const videoStyles = stylex.create({
  // ── số liệu đầu kênh ────────────────────────────────────────────────
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

  // ── vùng danh sách ──────────────────────────────────────────────────
  section: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-10'], minWidth: 0 },
  toolbar: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: space['--vv-s-8'] },
  chipDot: (dot: string) => ({ width: space['--vv-s-7'], height: space['--vv-s-7'], borderRadius: '50%', backgroundColor: dot, flex: '0 0 auto' }),
  chipCount: { color: 'var(--vv-fg-note)' },
  spacer: { flex: 1 },

  // ── bảng ────────────────────────────────────────────────────────────
  card: {
    borderRadius: corners['--vv-r-14'], overflowX: 'auto',
    backgroundImage: surface['--card-bg'], backgroundColor: 'transparent', borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], boxShadow: surface['--card-shadow'],
  },
  table: { display: 'flex', flexDirection: 'column', minWidth: { default: null, [PHONE]: 720 } },
  grid: {
    display: 'grid', gap: space['--vv-s-12'],
    gridTemplateColumns: { default: 'minmax(0, 1fr) 168px 132px 128px', [NARROW]: 'minmax(0, 1fr) 156px 124px' },
  },
  head: {
    padding: 'var(--vv-s-9) var(--vv-s-14)', borderBottomWidth: space['--vv-s-1'], borderBottomStyle: 'solid', borderBottomColor: line['--line-1'],
    fontSize: 12, color: 'var(--vv-fg-note)',
  },
  template: { display: { default: null, [NARROW]: 'none' } },
  right: { textAlign: 'right' },
  row: {
    position: 'relative', alignItems: 'center', padding: 'var(--vv-s-8) var(--vv-s-14)',
    borderBottomWidth: space['--vv-s-1'], borderBottomStyle: 'solid', borderBottomColor: 'rgba(255, 255, 255, 0.04)',
    backgroundColor: { default: 'transparent', ':hover': ROW_HOVER },
  },
  rowSelecting: { paddingLeft: space['--vv-s-46'] },
  rowChosen: { backgroundColor: ROW_HOVER },
  check: { position: 'absolute', left: space['--vv-s-8'], top: '50%', transform: 'translateY(-50%)' },
  video: { display: 'flex', alignItems: 'center', gap: space['--vv-s-12'], minWidth: 0, color: 'inherit', textDecoration: 'none' },
  thumb: {
    position: 'relative', width: 80, height: 45, borderRadius: corners['--vv-r-7'], flex: '0 0 auto', overflow: 'hidden',
    backgroundColor: color['--color-surface-4'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-1'],
  },
  thumbImg: { width: '100%', height: '100%', objectFit: 'cover', display: 'block' },
  duration: {
    position: 'absolute', right: space['--vv-s-4'], bottom: space['--vv-s-3'], fontSize: 10, padding: '0 var(--vv-s-4)', borderRadius: corners['--vv-r-4'],
    backgroundColor: 'rgba(0, 0, 0, 0.72)',
  },
  titles: { minWidth: 0, display: 'flex', flexDirection: 'column', gap: space['--vv-s-1'] },
  title: { fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  sub: { fontSize: 12, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  status: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-1'], minWidth: 0 },
  statusRow: { display: 'flex', alignItems: 'center', gap: space['--vv-s-7'], fontSize: 13 },
  statusDot: { width: space['--vv-s-7'], height: space['--vv-s-7'], borderRadius: '50%', backgroundColor: 'currentColor', flex: '0 0 auto' },
  spinner: {
    width: space['--vv-s-11'], height: space['--vv-s-11'], borderRadius: '50%', flex: '0 0 auto', boxSizing: 'border-box',
    borderWidth: 1.5, borderStyle: 'solid', borderColor: 'var(--vv-spinner-line)', borderTopColor: 'currentColor', animationName: spin, animationDuration: '0.9s', animationTimingFunction: 'linear', animationIterationCount: 'infinite',
  },
  tone: (value: string) => ({ color: value }),
  statusDetail: { fontSize: 12, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', paddingLeft: space['--vv-s-14'] },
  tpl: { fontSize: 12.5, color: color['--color-fg-2'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  actions: { display: 'flex', justifyContent: 'flex-end', gap: space['--vv-s-4'] },
  actionLink: { display: 'inline-flex', alignItems: 'center', color: color['--color-fg-2'], textDecorationLine: 'underline' },
  empty: { padding: 'var(--vv-s-32) var(--vv-s-14)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: space['--vv-s-8'], textAlign: 'center' },
  emptyIcon: { color: 'var(--vv-fg-note)' },
  emptyText: { fontSize: 13.5, color: color['--color-fg-2'] },
  footnote: { fontSize: 12, color: 'var(--vv-fg-note)', padding: '0 var(--vv-s-2)' },
  skelRow: { alignItems: 'center', padding: 'var(--vv-s-8) var(--vv-s-14)', borderBottomWidth: space['--vv-s-1'], borderBottomStyle: 'solid', borderBottomColor: 'rgba(255, 255, 255, 0.04)' },
  skelText: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'] },
  skelVideo: { display: 'flex', alignItems: 'center', gap: space['--vv-s-12'] },

  // ── dock dưới trang + thanh chọn nhiều ──────────────────────────────
  dock: {
    position: 'sticky', bottom: 'calc(-1 * var(--vv-s-26))', marginTop: 'auto', marginBottom: 'calc(-1 * var(--vv-s-26))', padding: 'var(--vv-s-10) 0 var(--vv-s-14)',
    backgroundImage: 'linear-gradient(0deg, var(--color-bg) 70%, transparent)', display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'],
  },
  kbd: {
    fontSize: 11, color: color['--color-fg-3'], padding: 'var(--vv-s-1) var(--vv-s-5)', borderRadius: corners['--vv-r-4'],
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'rgb(21, 21, 23)', whiteSpace: 'nowrap',
  },
  bulk: {
    borderRadius: corners['--vv-r-16'], padding: 'var(--vv-s-7) var(--vv-s-7) var(--vv-s-7) var(--vv-s-14)', display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: space['--vv-s-6'],
    backgroundColor: 'rgb(28, 28, 31)', backgroundImage: 'none', borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: 'var(--vv-line-14)', boxShadow: surface['--card-shadow'],
  },
  bulkCount: { fontSize: 13.5, fontWeight: 600, marginRight: space['--vv-s-6'], whiteSpace: 'nowrap' },
})
