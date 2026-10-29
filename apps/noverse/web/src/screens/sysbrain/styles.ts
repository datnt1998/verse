// SysBrain.dc.html — số đo lấy nguyên inline style + lớp .card/.chip2/.btn2/.seg của artboard.
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, surface, sysbrain, tint } from '@noverse/ui/tokens.stylex'

export const styles = stylex.create({
  root: {
    flex: '1 1 auto', minHeight: 0, overflowY: 'auto', paddingTop: 18, paddingInline: 24, paddingBottom: 24,
    display: 'flex', flexDirection: 'column', gap: 14,
  },
  head: { display: 'flex', alignItems: 'flex-end', gap: 16 },
  intro: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  // Màu đặt inline ở <a>: quy tắc `a { color }` của base.css thắng lớp StyleX.
  back: { display: 'flex', alignItems: 'center', gap: 6, fontSize: 12.5 },
  h1: {
    margin: 0, fontSize: 22, fontWeight: 600, letterSpacing: '-0.01em', display: 'flex', alignItems: 'center', gap: 10,
  },
  sub: { fontSize: 13, color: color['--color-fg-3'] },
  stats: { display: 'flex', gap: 10 },
  stat: {
    borderRadius: 12, paddingBlock: 8, paddingInline: 12, display: 'flex', flexDirection: 'column', minWidth: 96,
  },
  statValue: { fontSize: 17, fontWeight: 600, fontVariantNumeric: 'tabular-nums' },
  statLabel: { fontSize: 11.5, color: kit['--kit-fg-icon'] },
  grid: { display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) 420px', gap: 14, alignItems: 'start' },
  column: { display: 'flex', flexDirection: 'column', gap: 10 },
  aside: { display: 'flex', flexDirection: 'column', gap: 12 },
  toolbar: { display: 'flex', alignItems: 'center', gap: 10 },
  grow: { flex: 1 },
  note: {
    fontSize: 12, color: color['--color-fg-4'], minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  empty: { borderRadius: 16, padding: 30, textAlign: 'center', fontSize: 13, color: kit['--kit-fg-icon'] },
  // .card của artboard
  card: {
    backgroundImage: surface['--card-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    boxShadow: surface['--card-shadow'],
  },
  lesson: { borderRadius: 16, paddingBlock: 14, paddingInline: 16, display: 'flex', flexDirection: 'column', gap: 10 },
  chipRow: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  // .chip2
  chip: {
    height: 20, paddingInline: 7, borderRadius: 6, display: 'inline-flex', alignItems: 'center',
    fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap',
  },
  chipReview: { backgroundColor: kit['--kit-acc-12'], color: kit['--kit-badge-run'] },
  chipUser: { backgroundColor: tint['--tint-ok'], color: sysbrain['--sysbrain-user-fg'] },
  chipRef: { backgroundColor: kit['--kit-wash-5'], color: color['--color-fg-2'] },
  chipEvidence: { backgroundColor: kit['--kit-wash-4'], color: color['--color-fg-3'], fontWeight: 400 },
  chipAdmin: { backgroundColor: kit['--kit-acc-12'], color: kit['--kit-badge-run'] },
  when: { fontSize: 12, color: color['--color-fg-4'] },
  text: { margin: 0, fontSize: 14.5, color: color['--color-fg'], lineHeight: 1.55 },
  textarea: {
    width: '100%', minHeight: 64, paddingBlock: 10, paddingInline: 12, borderRadius: 10,
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-acc-45'],
    color: color['--color-fg'], font: 'inherit', fontSize: 14, lineHeight: 1.55, resize: 'vertical',
  },
  evidence: {
    display: 'flex', flexDirection: 'column', gap: 6, paddingBlock: 10, paddingInline: 12, borderRadius: 12,
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
  },
  evidenceSource: { fontSize: 12, color: color['--color-fg-3'] },
  evidenceChips: { display: 'flex', flexWrap: 'wrap', gap: 4 },
  actions: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  appliesTo: { fontSize: 12.5, color: kit['--kit-fg-icon'] },
  done: { display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5 },
  doneKept: { color: color['--color-ok'] },
  doneDropped: { color: color['--color-fg-3'] },
  doneText: { flex: 1 },
  // aside
  panel: { borderRadius: 16, paddingBlock: 14, paddingInline: 16, display: 'flex', flexDirection: 'column', gap: 10 },
  panelHead: { display: 'flex', alignItems: 'center', gap: 8 },
  panelTitle: { fontSize: 14, fontWeight: 600, flex: 1 },
  violation: { display: 'grid', gridTemplateColumns: '90px minmax(0, 1fr) 64px', gap: 10, alignItems: 'center' },
  refCell: { display: 'flex', flexDirection: 'column', gap: 2, minWidth: 0 },
  refChip: { alignSelf: 'flex-start' },
  nameCell: { display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 },
  name: { fontSize: 12.5, color: color['--color-fg-2'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  track: { height: 6, borderRadius: 3, backgroundColor: kit['--kit-wash-6'] },
  bar: { display: 'block', height: 6, borderRadius: 3 },
  barError: { backgroundColor: color['--color-err'] },
  barWarning: { backgroundColor: color['--color-warn'] },
  countCell: { display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 1 },
  countLine: { display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 6 },
  count: { fontSize: 12.5, color: color['--color-fg'], fontVariantNumeric: 'tabular-nums' },
  scopeAll: { fontSize: 11.5, color: kit['--kit-fg-icon'], whiteSpace: 'nowrap' },
  alert: { display: 'inline-flex', color: kit['--kit-err-soft'] },
  legend: { display: 'flex', flexWrap: 'wrap', rowGap: 4, columnGap: 14, fontSize: 11.5, color: kit['--kit-fg-icon'] },
  legendItem: { display: 'flex', alignItems: 'center', gap: 5 },
  swatch: { width: 10, height: 6, borderRadius: 3 },
  reminder: {
    borderRadius: 16, paddingBlock: 14, paddingInline: 16, display: 'flex', flexDirection: 'column', gap: 8,
    borderColor: kit['--kit-errf-25'],
  },
  reminderTitle: { display: 'flex', alignItems: 'center', gap: 8, fontSize: 13.5, fontWeight: 600 },
  reminderLead: { fontSize: 12.5, color: color['--color-fg-3'] },
  reminderText: {
    margin: 0, paddingBlock: 10, paddingInline: 12, borderRadius: 12, backgroundColor: color['--color-surface-0'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], fontSize: 13, color: kit['--kit-fg-note'], lineHeight: 1.6,
  },
  footnote: { margin: 0, fontSize: 12, color: kit['--kit-fg-faint'], lineHeight: 1.5 },
  error: { display: 'flex', alignItems: 'center', gap: 10, fontSize: 13, color: kit['--kit-err-soft'] },
})

/** .seg của artboard (nền --color-card, viền line-2, mục chọn --sysbrain-seg-on, không khoảng hở) đặt lên Segmented của kit. */
export const segGroup = { gap: 0, background: 'var(--color-card)', border: '1px solid var(--line-2)' } as const
export const segThumb = { background: 'var(--sysbrain-seg-on)', boxShadow: 'none' } as const
export const segOption = (on: boolean) => ({ flex: 'none', padding: '0 11px', fontWeight: on ? 500 : 400 }) as const
/** Mục chọn gói: tên gói thật có thể dài → co đều trong khung, cắt “…”, toàn văn ở `title`. */
export const packOption = (on: boolean) => ({ ...segOption(on), flex: '1 1 0', minWidth: 0 }) as const
/** .btn2: cao 32, đệm 12, bo 9, chữ 12.5, khoảng 6. */
export const btn2 = { height: 32, paddingInline: 12, borderRadius: 9, fontSize: 12.5, gap: 6 } as const
export const btn2Bare = { ...btn2, border: 'none', color: 'var(--color-fg-3)', fontWeight: 400 } as const
export const btn2Ghost = { ...btn2, borderColor: 'var(--line-4)', fontWeight: 400 } as const
export const btn2Primary = { ...btn2, borderColor: 'var(--color-fg)' } as const
