// Kiểu thẻ Đề xuất (kit.css .vv-suggest*, .vv-why, .vv-count) và nút "Đề xuất N"
// của cột trái Bàn dựng. Số đo lấy từ getComputedStyle của artboard Main
// `agent=suggest|recap` ở 1440×900.
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'

export const suggestStyles = stylex.create({
  card: {
    display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr)', gap: `${s['--vv-s-4']} ${s['--vv-s-10']}`,
    padding: s['--vv-s-12'], borderRadius: r['--vv-r-12'], backgroundColor: color['--color-surface-1'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], minWidth: 0,
  },
  cardCompact: { padding: s['--vv-s-10'], gap: `${s['--vv-s-2']} ${s['--vv-s-8']}` },
  orb: { gridRow: 'span 3', marginTop: s['--vv-s-2'] },
  who: { fontSize: 11.5, fontWeight: 600, lineHeight: 1.55 },
  ink: (value: string) => ({ color: value }),
  title: {
    fontSize: 13.5, lineHeight: 1.45, color: color['--color-fg'], display: '-webkit-box',
    WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', overflowWrap: 'anywhere',
  },
  titleCompact: { fontSize: 13 },
  evidence: {
    fontSize: 12, lineHeight: 1.45, color: color['--color-fg-3'], display: 'flex', flexWrap: 'wrap',
    gap: `0 ${s['--vv-s-8']}`, alignItems: 'center', minWidth: 0,
  },
  evidenceText: { minWidth: 0, overflowWrap: 'anywhere' },
  why: {
    display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-4'], height: 22, padding: `0 ${s['--vv-s-2']}`,
    borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg-2'], fontFamily: 'inherit', fontSize: 12,
    whiteSpace: 'nowrap', textDecorationLine: 'underline', textDecorationStyle: 'dotted',
    textDecorationColor: 'rgba(255,255,255,.35)', textUnderlineOffset: 3, cursor: 'pointer',
    ':hover': { color: color['--color-fg'] },
  },
  whyOpen: { color: color['--color-fg'] },
  actions: {
    gridColumn: '1 / -1', display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], marginTop: s['--vv-s-6'], minWidth: 0,
  },
  primary: { flex: '0 1 auto', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' },
  more: { marginLeft: 'auto' },
  reason: { gridColumn: '1 / -1', marginTop: s['--vv-s-6'] },
  notice: { gridColumn: '1 / -1', fontSize: 12, color: 'var(--vv-warn-ink)' },
  // Nhóm trong cột trái: chữ “Đề xuất” + số, các thẻ xếp dọc cách 6px.
  group: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'] },
  groupHead: {
    display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', padding: `${s['--vv-s-10']} ${s['--vv-s-8']} ${s['--vv-s-4']}`,
    margin: 0, fontSize: 11, fontWeight: 600, letterSpacing: '.08em', textTransform: 'uppercase', color: color['--color-fg-3'],
  },
  seeAll: {
    width: '100%', display: 'flex', alignItems: 'center', padding: `${s['--vv-s-7']} ${s['--vv-s-8']}`, borderWidth: 0,
    borderRadius: r['--vv-r-8'], backgroundColor: 'transparent', color: color['--color-fg-2'], fontFamily: 'inherit',
    fontSize: 12.5, textAlign: 'left', cursor: 'pointer',
  },
  state: { padding: `${s['--vv-s-6']} ${s['--vv-s-8']}`, fontSize: 12, lineHeight: 1.5, color: color['--color-fg-3'] },
  stateAction: { alignSelf: 'flex-start' },
  // Hàng hai nút đầu cột trái: Bản tin + Đề xuất N (cùng rộng, cách 6px).
  toggles: { display: 'flex', gap: s['--vv-s-6'], padding: `${s['--vv-s-2']} ${s['--vv-s-2']} 0` },
  toggle: { flex: '1 1 0', minWidth: 0 },
  toggleOn: { backgroundColor: color['--color-surface-4'] },
  orbs: { display: 'flex', alignItems: 'center' },
  orbStack: { marginLeft: -3 },
  count: {
    minWidth: 18, height: 18, padding: `0 ${s['--vv-s-5']}`, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    borderRadius: r['--vv-r-pill'], backgroundColor: 'rgba(246,203,140,.16)', color: 'var(--vv-warn-ink)', fontSize: 11,
    fontWeight: 600, fontVariantNumeric: 'tabular-nums',
  },
  // Bản tin: nhóm "Đề xuất · 3 trong 5"
  recapSection: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  recapGroup: { fontSize: 11, fontWeight: 600, letterSpacing: '.08em', textTransform: 'uppercase', color: color['--color-fg-3'] },
  recapList: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  // Bản tin: dòng “Máy đã tự làm”
  autoLine: {
    display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], minHeight: 40, padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`,
    borderRadius: r['--vv-r-10'], backgroundColor: color['--color-surface-1'], fontSize: 13, color: color['--color-fg-2'],
  },
  autoText: { flex: 1, minWidth: 0, overflowWrap: 'anywhere' },
  autoTag: {
    height: 22, padding: `0 ${s['--vv-s-8']}`, display: 'inline-flex', alignItems: 'center', borderRadius: r['--vv-r-6'],
    backgroundColor: 'rgba(157,236,201,.08)', color: 'var(--vv-qa-ink)', fontSize: 11.5, whiteSpace: 'nowrap',
  },
})
