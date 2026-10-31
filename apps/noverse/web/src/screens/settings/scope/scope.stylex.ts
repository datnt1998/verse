// StyleX cho thanh “Áp dụng cho” (Gói R): hộp hai nút, popover 360 px, dòng danh sách, hộp cam “chưa lưu”,
// dải xanh “Đang xem…”. Màu chỉ lấy từ tokens.stylex.ts.
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, tint } from '@noverse/ui/tokens.stylex'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

export const sx = stylex.create({
  // ─── Thanh ────────────────────────────────────────────────────────────
  bar: {
    display: 'flex', alignItems: 'center', gap: 6, padding: 4, borderRadius: 15,
    backgroundColor: color['--color-scope-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    width: 'max-content', maxWidth: '100%', flexWrap: 'wrap',
  },
  lead: { padding: '0 10px 0 8px', fontSize: 12.5, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  box: {
    position: 'relative', display: 'flex', alignItems: 'stretch', borderRadius: 12,
    backgroundColor: 'transparent', borderWidth: 0,
    transitionProperty: 'background-color, border-color, box-shadow',
    transitionDuration: '.15s', transitionTimingFunction: 'ease', [NO_MOTION]: { transitionDuration: '0s' },
  },
  boxOn: {
    backgroundColor: color['--color-scope-selected'], borderColor: kit['--kit-acc-55'], boxShadow: kit['--kit-shadow-segment'],
  },
  boxHover: { backgroundColor: line['--line-1'] },
  level: {
    display: 'flex', alignItems: 'center', gap: 10, padding: '0 12px', minHeight: 52, borderRadius: 11,
    borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', textAlign: 'left', cursor: 'pointer',
    color: color['--color-fg'], fontFamily: 'inherit',
  },
  levelDisabled: { opacity: .4, cursor: 'not-allowed' },
  levelText: { display: 'flex', flexDirection: 'column', lineHeight: 1.25, minWidth: 0 },
  levelLabel: { fontSize: 13.5, fontWeight: 500, whiteSpace: 'nowrap' },
  levelSub: { fontSize: 11.5, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 190 },
  levelIcon: { color: color['--color-fg-4'] },
  levelIconOn: { color: color['--color-accent'] },
  note: { margin: '10px 2px 0', fontSize: 12.5, color: color['--color-fg-3'] },
  caret: {
    display: 'flex', alignItems: 'center', justifyContent: 'center', width: 28,
    borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', color: color['--color-fg-4'],
    cursor: 'pointer', borderRadius: 10,
  },
  // ─── Popover ──────────────────────────────────────────────────────────
  pop: {
    position: 'absolute', zIndex: 40, top: 'calc(100% + 8px)', left: 0, width: 360,
    borderRadius: 14, backgroundColor: color['--color-surface-2'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    boxShadow: kit['--kit-shadow-pop'], padding: 8, display: 'flex', flexDirection: 'column', gap: 6,
  },
  head: { display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 8, padding: '4px 6px 2px' },
  headTitle: { fontSize: 12.5, fontWeight: 600, color: color['--color-fg-2'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  headCount: { fontSize: 11.5, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  search: {
    height: 34, padding: '0 10px', borderRadius: 9, backgroundColor: color['--color-surface-0'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-3'], color: color['--color-fg'],
    fontSize: 12.5, width: '100%', fontFamily: 'inherit',
  },
  list: { display: 'flex', flexDirection: 'column', gap: 1, maxHeight: 320, overflowY: 'auto', outline: 'none' },
  row: {
    display: 'flex', alignItems: 'center', gap: 10, width: '100%', padding: '7px 8px', borderRadius: 9,
    borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', textAlign: 'left', cursor: 'pointer',
    color: color['--color-fg'], fontFamily: 'inherit',
  },
  rowHi: { backgroundColor: color['--color-surface-4'] },
  rowOn: { backgroundColor: color['--color-surface-6'] },
  rowIcon: { color: color['--color-fg-4'], flex: '0 0 auto' },
  rowText: { display: 'flex', flexDirection: 'column', minWidth: 0, flex: '1 1 auto', lineHeight: 1.3 },
  rowName: { fontSize: 13.5, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  rowSub: { fontSize: 11.5, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  chip: {
    display: 'inline-flex', alignItems: 'center', height: 18, padding: '0 6px', borderRadius: 6,
    backgroundColor: tint['--tint-ok'], color: color['--color-ok'], fontSize: 10.5, fontWeight: 600, whiteSpace: 'nowrap',
  },
  empty: { padding: '10px 8px', fontSize: 12.5, color: color['--color-fg-4'] },
  sep: { height: 1, backgroundColor: line['--line-1'], margin: '4px 0' },
  footer: {
    display: 'flex', alignItems: 'center', gap: 8, width: '100%', padding: '8px', borderRadius: 9,
    borderWidth: 0, borderStyle: 'none', backgroundColor: 'transparent', color: color['--color-fg-2'],
    fontSize: 12.5, textAlign: 'left', cursor: 'pointer', fontFamily: 'inherit',
  },
  footerText: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  // Gói R: nội dung picker trên điện thoại, giữ hình học hàng của Settings.
  sheetContent: { display: 'flex', flexDirection: 'column', gap: 16, padding: '2px 16px 0' },
  sheetSection: { display: 'flex', flexDirection: 'column' },
  sheetHead: { padding: '6px 10px 8px' },
  sheetHeadTitle: { fontSize: 12, fontWeight: 400, color: color['--color-fg-3'] },
  sheetSearch: {
    display: 'flex', alignItems: 'center', gap: 8, height: 34, flexShrink: 0, padding: '0 10px', marginBottom: 6, borderRadius: 9,
    backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-acc-35'],
  },
  sheetSearchInput: {
    flex: '1 1 auto', minWidth: 0, width: '100%', height: '100%', padding: 0, borderWidth: 0, outline: 'none',
    backgroundColor: 'transparent', color: color['--color-fg'], fontFamily: 'inherit', fontSize: 13,
  },
  sheetList: { maxHeight: 300 },
  sheetRow: { display: 'grid', gridTemplateColumns: '18px minmax(0,1fr) auto', padding: '8px 10px' },
  sheetRowText: { lineHeight: 1.35 },
  sheetRowName: { fontSize: 13, fontWeight: 500 },
  sheetMarks: { display: 'flex', alignItems: 'center', gap: 6 },
  // ─── Hộp cam “chưa lưu” ───────────────────────────────────────────────
  unsaved: {
    display: 'flex', flexDirection: 'column', gap: 8, padding: '10px 12px', borderRadius: 10,
    backgroundColor: kit['--kit-warn-6'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-warn-20'],
  },
  unsavedText: { fontSize: 12.5, color: kit['--kit-warn-text'], lineHeight: 1.45 },
  unsavedActions: { display: 'flex', gap: 6, flexWrap: 'wrap' },
  action: {
    height: 28, padding: '0 10px', borderRadius: 8, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-warn-30'],
    backgroundColor: 'transparent', color: kit['--kit-warn-text'], fontSize: 12, cursor: 'pointer', fontFamily: 'inherit',
  },
  actionPrimary: { backgroundColor: kit['--kit-warn-12'], fontWeight: 500 },
  // ─── Dải xanh “Đang xem…” ─────────────────────────────────────────────
  strip: {
    display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', maxWidth: 820, marginTop: 10,
    padding: '8px 12px', borderRadius: 12, backgroundColor: kit['--kit-up-bg'],
    borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-up-bd'],
  },
  stripText: { flex: '1 1 320px', minWidth: 0, fontSize: 12.5, color: kit['--kit-up-badge-fg'], lineHeight: 1.45 },
  stripGo: {
    height: 28, padding: '0 10px', borderRadius: 8, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-up-bd'],
    backgroundColor: 'transparent', color: kit['--kit-info-fg'], fontSize: 12, cursor: 'pointer', whiteSpace: 'nowrap', fontFamily: 'inherit',
  },
})

export { NO_MOTION }
