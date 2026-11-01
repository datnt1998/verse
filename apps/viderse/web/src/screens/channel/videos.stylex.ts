// Kiểu của Kênh › Video (artboard Channel): số liệu đầu kênh, bộ lọc, bảng,
// thanh chọn nhiều. Số đo lấy từ artboard ở 1440×900; màu qua token.
import * as stylex from '@stylexjs/stylex'
import { color, line, surface } from '@noverse/ui/tokens.stylex'
import { space, corners } from '../../styles/viderse.stylex'

const NARROW = '@media (max-width: 1380px)'
const PHONE = '@media (max-width: 700px)'
const ROW_HOVER = 'rgba(255, 255, 255, 0.035)'
const spin = stylex.keyframes({ to: { transform: 'rotate(360deg)' } })
export const videoStyles = stylex.create({
  // ── số liệu đầu kênh ────────────────────────────────────────────────
  stats: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(min(150px, 100%), 1fr))', gap: space['--vv-s-10'] },
  stat: {
    borderRadius: corners['--vv-r-12'], padding: 'var(--vv-s-10) var(--vv-s-14)', display: 'flex', flexDirection: 'column', gap: space['--vv-s-2'],
    backgroundImage: surface['--card-bg'], backgroundColor: 'transparent', borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-2'], boxShadow: surface['--card-shadow'],
  },
  statLabel: { fontSize: 12, color: 'var(--vv-fg-note)' },
  statValue: { fontSize: 20, fontWeight: 600, letterSpacing: '-0.01em', color: color['--color-fg'] },
  statValueWarn: { color: color['--color-warn'] },
  statSub: { fontSize: 11.5, color: 'var(--vv-fg-note)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },

  // ── vùng danh sách ──────────────────────────────────────────────────
  section: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-10'], minWidth: 0 },
  toolbar: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: space['--vv-s-8'] },
  filters: {
    display: 'flex', flexWrap: 'wrap', gap: space['--vv-s-2'], padding: space['--vv-s-3'], borderRadius: corners['--vv-r-10'],
    backgroundColor: color['--color-surface-1'], borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-1'],
  },
  chip: {
    height: space['--vv-s-28'], padding: '0 var(--vv-s-10)', display: 'flex', alignItems: 'center', gap: space['--vv-s-6'], borderRadius: corners['--vv-r-7'],
    borderWidth: 0, borderStyle: 'none', borderColor: 'currentColor', fontFamily: 'inherit', fontSize: 12.5, whiteSpace: 'nowrap', cursor: 'pointer',
    backgroundColor: { default: 'transparent', ':hover': color['--color-surface-4'] }, color: color['--color-fg-3'],
  },
  chipOn: { backgroundColor: color['--color-surface-6'], color: color['--color-fg'] },
  chipDot: (dot: string) => ({ width: space['--vv-s-7'], height: space['--vv-s-7'], borderRadius: '50%', backgroundColor: dot, flex: '0 0 auto' }),
  chipCount: { color: 'var(--vv-fg-note)' },
  spacer: { flex: 1 },
  toolBtn: {
    height: space['--vv-s-32'], padding: '0 var(--vv-s-10)', display: 'flex', alignItems: 'center', gap: space['--vv-s-6'], borderRadius: corners['--vv-r-8'],
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], fontFamily: 'inherit', fontSize: 13, whiteSpace: 'nowrap', cursor: 'pointer',
    backgroundColor: { default: 'transparent', ':hover:not([aria-disabled="true"])': 'var(--vv-hover)' }, color: color['--color-fg'],
  },
  toolBtnOn: { borderColor: 'var(--vv-line-30)', backgroundColor: 'var(--vv-wash-8)' },
  toolBtnLocked: { opacity: 0.42, cursor: 'not-allowed' },
  toolIcon: { color: color['--color-fg-3'], flex: '0 0 auto' },
  toolIconOn: { color: color['--color-fg'] },

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
  check: {
    position: 'absolute', left: space['--vv-s-8'], top: '50%', transform: 'translateY(-50%)', width: space['--vv-s-32'], height: space['--vv-s-32'],
    borderWidth: 0, borderStyle: 'none', borderColor: 'currentColor', backgroundColor: 'transparent', padding: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', cursor: 'pointer',
  },
  box: {
    width: space['--vv-s-18'], height: space['--vv-s-18'], borderRadius: corners['--vv-r-5'], borderWidth: 1.5, borderStyle: 'solid', borderColor: 'var(--vv-line-30)',
    display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 12, fontWeight: 800, color: color['--color-bg'], backgroundColor: 'transparent',
  },
  boxOn: { borderColor: color['--color-accent'], backgroundColor: color['--color-accent'] },
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
  act: {
    height: space['--vv-s-28'], padding: '0 var(--vv-s-10)', display: 'flex', alignItems: 'center', borderRadius: corners['--vv-r-7'], whiteSpace: 'nowrap',
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: { default: 'transparent', ':hover': 'var(--vv-hover)' },
    color: color['--color-fg'], fontFamily: 'inherit', fontSize: 12.5, fontWeight: 400, textDecoration: 'none', cursor: 'pointer',
  },
  actReview: { borderColor: 'rgba(246, 203, 140, 0.4)', color: color['--color-warn'], fontWeight: 500 },
  actFix: { borderColor: 'rgba(255, 154, 92, 0.4)', color: 'var(--vv-warn-ink)', fontWeight: 500 },
  more: { width: space['--vv-s-28'], height: space['--vv-s-28'], borderRadius: corners['--vv-r-7'], color: 'var(--vv-fg-note)' },
  empty: { padding: 'var(--vv-s-32) var(--vv-s-14)', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: space['--vv-s-8'], textAlign: 'center' },
  emptyIcon: { color: 'var(--vv-fg-note)' },
  emptyText: { fontSize: 13.5, color: color['--color-fg-2'] },
  emptyBtn: { height: space['--vv-s-30'], padding: '0 var(--vv-s-12)', borderRadius: corners['--vv-r-8'] },
  footnote: { fontSize: 12, color: 'var(--vv-fg-note)', padding: '0 var(--vv-s-2)' },
  skelRow: { alignItems: 'center', padding: 'var(--vv-s-8) var(--vv-s-14)', borderBottomWidth: space['--vv-s-1'], borderBottomStyle: 'solid', borderBottomColor: 'rgba(255, 255, 255, 0.04)' },
  skelText: { display: 'flex', flexDirection: 'column', gap: space['--vv-s-6'] },
  skelVideo: { display: 'flex', alignItems: 'center', gap: space['--vv-s-12'] },

  // ── dock dưới trang + thanh chọn nhiều ──────────────────────────────
  dock: {
    position: 'sticky', bottom: 0, marginTop: 'auto', padding: 'var(--vv-s-10) 0 var(--vv-s-14)',
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
  bulkBtn: {
    height: space['--vv-s-36'], padding: '0 var(--vv-s-11)', display: 'flex', alignItems: 'center', gap: space['--vv-s-6'], borderRadius: corners['--vv-r-10'],
    borderWidth: space['--vv-s-1'], borderStyle: 'solid', borderColor: line['--line-4'], fontFamily: 'inherit', fontSize: 13, whiteSpace: 'nowrap', cursor: 'pointer',
    backgroundColor: { default: 'transparent', ':hover:not([aria-disabled="true"])': 'var(--vv-hover)' }, color: color['--color-fg'],
  },
  bulkBtnOff: { color: 'var(--vv-fg-note)', cursor: 'not-allowed' },
  bulkMore: { width: space['--vv-s-34'], height: space['--vv-s-34'], borderRadius: corners['--vv-r-9'], color: color['--color-fg-3'] },
  bulkExit: {
    height: space['--vv-s-36'], padding: '0 var(--vv-s-12)', borderRadius: corners['--vv-r-10'], display: 'flex', alignItems: 'center', gap: space['--vv-s-6'],
    borderWidth: 0, borderStyle: 'none', borderColor: 'currentColor', backgroundColor: { default: 'transparent', ':hover': 'var(--vv-hover)' },
    color: color['--color-fg-3'], fontFamily: 'inherit', fontSize: 13, cursor: 'pointer',
  },
})
