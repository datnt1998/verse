// Tab Truyện của Bàn viết (Main.dc.html › tab Truyện + Gói P2 · 21-dev-prompt.md:624-635).
// StyleX cho mọi khối của tab; màu lấy từ token. Chỗ duy nhất còn inline là màu/nét theo dữ liệu
// (màu kiểu quan hệ, màu xưng hô) và màu chữ của thẻ <a> — base.css đặt `a { color }` ngoài layer.
import * as stylex from '@stylexjs/stylex'
import { brain, color, kit, line } from '@noverse/ui/tokens.stylex'

export const s = stylex.create({
  // ─── Hai ô đầu: Bản nộp mới nhất · Thẩm duyệt mới nhất ─────────────────
  tiles: { display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 8 },
  tile: {
    borderRadius: 12, paddingBlock: 10, paddingInline: 12, display: 'flex', flexDirection: 'column', gap: 4,
    color: color['--color-fg'],
  },
  tileLabel: { fontSize: 12, color: color['--color-fg-4'] },
  tileValue: {
    fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },

  // ─── Thẻ Nhân vật ─────────────────────────────────────────────────────
  castCard: {
    borderRadius: 14, paddingBlock: 12, paddingInline: 14, display: 'flex', flexDirection: 'column', gap: 8,
    position: 'relative',
  },
  castHead: { display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8 },
  castHeadRight: { display: 'flex', alignItems: 'baseline', gap: 8, flexShrink: 0 },
  castCount: { fontSize: 12, color: color['--color-fg-4'] },
  castLink: { fontSize: 12 },
  /** Nút Đồ thị (Main.dc.html › “…Đồ thị”): ghost có viền, icon routing + chữ. */
  graphBtn: {
    height: 26, paddingLeft: 6, paddingRight: 8, borderRadius: 7, borderWidth: 1, borderStyle: 'solid',
    borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg-2'],
    display: 'flex', alignItems: 'center', gap: 5, fontSize: 12, alignSelf: 'center', cursor: 'pointer',
  },
  castRow: {
    position: 'relative', display: 'flex', alignItems: 'center', gap: 10, minWidth: 0,
    paddingBlock: 4, paddingInline: 6, marginInline: -6, marginBlock: 0, border: 'none',
    backgroundColor: 'transparent', borderRadius: 8,
    color: color['--color-fg'], textAlign: 'left', cursor: 'pointer',
  },
  rowText: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 0, overflow: 'hidden', display: 'flex', gap: 6, alignItems: 'baseline' },
  // Main.dc.html:465 — tên không bao giờ cắt (flexShrink 0); ellipsis chỉ là phòng khi tên quá dài hơn cả dòng.
  rowName: {
    flexShrink: 0, minWidth: 0, fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  rowRole: {
    flexGrow: 1, flexShrink: 1, flexBasis: 'auto', minWidth: 0, fontSize: 12, color: color['--color-fg-4'],
    whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  recentSupporting: { fontSize: 12, color: color['--color-fg-4'] },

  // ─── Thẻ rê (hover/focus) — Main.dc.html › .castrow > .hc: rộng hơn hàng 6 px mỗi bên.
  peek: {
    position: 'absolute', top: 'calc(100% + 4px)', insetInline: -6, zIndex: 6,
    display: 'flex', flexDirection: 'column', gap: 8, paddingBlock: 10, paddingInline: 12,
    borderRadius: 12, backgroundColor: color['--color-surface-4'], borderWidth: 1, borderStyle: 'solid',
    borderColor: line['--line-4'], boxShadow: brain['--brain-pop-shadow'], cursor: 'pointer',
  },
  peekHead: {
    fontSize: 11.5, color: color['--color-fg-4'], display: 'flex', alignItems: 'center', gap: 8, justifyContent: 'space-between',
  },
  peekKind: { marginLeft: 'auto', flexShrink: 0, fontSize: 11, whiteSpace: 'nowrap' },
  peekRel: { display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, minWidth: 0 },
  peekLine: { width: 10, height: 3, borderRadius: 2, flexShrink: 0 },
  peekLabel: { whiteSpace: 'nowrap' },
  peekOther: { minWidth: 0, color: color['--color-fg'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  peekSince: { marginLeft: 'auto', flexShrink: 0, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  peekXh: {
    fontSize: 12.5, color: color['--color-fg-2'], paddingTop: 6, borderTopWidth: 1, borderTopStyle: 'solid',
    borderTopColor: line['--line-1'],
  },
  peekHint: { fontSize: 11.5, color: brain['--brain-muted'] },

  // ─── Thẻ “Đổi ở chương N” ─────────────────────────────────────────────
  changeCard: {
    borderRadius: 14, paddingBlock: 12, paddingInline: 14, display: 'flex', flexDirection: 'column', gap: 8,
  },
  changeHead: { display: 'flex', alignItems: 'baseline', gap: 8, minWidth: 0 },
  changeTitle: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  changeCount: { flexShrink: 0, fontSize: 12, color: color['--color-fg-4'] },
  changeRow: { display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5, minWidth: 0 },
  changeTag: {
    flexShrink: 0, paddingBlock: 1, paddingInline: 6, borderRadius: 5, backgroundColor: kit['--kit-warn-12'],
    color: color['--color-warn'], fontSize: 11,
  },
  changeText: { minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  changeEmpty: { fontSize: 12.5, color: color['--color-fg-4'] },
  changeAction: {
    alignSelf: 'flex-start', height: 26, paddingInline: 6, marginLeft: -6, border: 'none',
    borderRadius: 7, backgroundColor: 'transparent', color: color['--color-accent'], fontSize: 12.5, cursor: 'pointer',
  },

  // ─── Giới thiệu tóm tắt ───────────────────────────────────────────────
  synopsisCard: {
    borderRadius: 14, paddingBlock: 12, paddingInline: 14, display: 'flex', flexDirection: 'column', gap: 6,
  },
  synopsisBody: {
    display: 'flex', flexDirection: 'column', gap: 6, fontSize: 13, lineHeight: 1.65, color: color['--color-fg-3'],
  },
  premiseBody: {
    display: 'flex', flexDirection: 'column', gap: 6, fontSize: 13, lineHeight: 1.65, color: color['--color-fg-2'],
  },
  premiseLabel: { fontSize: 12, color: color['--color-fg-4'] },
  empty: { color: color['--color-fg-4'] },
  frameLink: { fontSize: 12 },

  // ─── Tóm tắt gần đây ──────────────────────────────────────────────────
  summaryCard: {
    borderRadius: 14, paddingBlock: 12, paddingInline: 14, display: 'flex', flexDirection: 'column', gap: 8,
  },
  summaryRow: {
    display: 'block', marginInline: -6, paddingBlock: 3, paddingInline: 6, borderRadius: 8,
    fontSize: 13, color: color['--color-fg-2'], overflowWrap: 'anywhere',
  },
  summaryNum: { color: color['--color-fg-4'] },
  summaryHint: { fontSize: 12, color: brain['--brain-muted'] },
})
