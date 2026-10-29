// Inspector (cột phải Bàn viết) — StyleX cho phần khung + thẻ Trợ lý/Chi phí.
// Số đo lấy từ Main.dc.html (aside 340 · tablist 30 · AgentCard orb 26/pad 12/badge 6)
// và 04-components.md (AgentCard · RewriteQueueCard · CostCard · CacheCard).
// Mọi chỗ một dòng đều nowrap + ellipsis + title ở JSX để chữ dài không đẩy panel rộng ra.
import * as stylex from '@stylexjs/stylex'
import { color, line, tint } from '@noverse/ui/tokens.stylex'

export const s = stylex.create({
  // ─── Khung panel ───────────────────────────────────────────────────────
  // Main.dc.html:377 — flex 0 0 340, max-width 340, min-height 0, viền trái line-1.
  aside: (mobile: boolean) => ({
    flex: mobile ? undefined : '0 0 340px',
    maxWidth: mobile ? '100%' : 340,
    minWidth: 0,
    minHeight: 0,
    display: 'flex',
    flexDirection: 'column',
    borderLeftWidth: mobile ? 0 : 1,
    borderLeftStyle: 'solid',
    borderLeftColor: line['--line-1'],
  }),
  // Main.dc.html:383 — padding 12 hai bên; overflow-x hidden để không bao giờ có thanh cuộn ngang.
  // 14-jobs-dock §1.5: chừa chỗ cho chấm dock 52px dưới cột agent (FDock đặt biến này).
  scroll: (mobile: boolean) => ({
    flexGrow: mobile ? 0 : 1,
    flexShrink: mobile ? 0 : 1,
    flexBasis: mobile ? 'auto' : 0,
    minHeight: 0,
    overflowX: 'hidden',
    paddingTop: mobile ? 0 : 12,
    paddingRight: mobile ? 0 : 12,
    paddingLeft: mobile ? 0 : 12,
    paddingBottom: mobile ? 0 : 'calc(12px + var(--dock-dot-space, 0px))',
    display: 'flex',
    flexDirection: 'column',
    gap: 12,
  }),

  // ─── AgentCard ────────────────────────────────────────────────────────
  // Main.dc.html:387 — card 14 · padding 12 · gap 8 · opacity 0.6 khi rảnh.
  agentCard: (idle: boolean) => ({
    position: 'relative',
    overflow: 'hidden',
    // Cột cuộn là flex column cao cố định; overflow:hidden làm min-height tự động = 0 nên 4 thẻ bị ép
    // thấp lại và dòng tiến trình bị cắt nửa. Không cho thẻ co — cột tự cuộn.
    flexShrink: 0,
    borderRadius: 14,
    padding: 12,
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
    opacity: idle ? 0.6 : 1,
  }),
  // Main.dc.html:388 — hàng đầu gap 10 (orb 26 + tên + badge).
  agentHead: { position: 'relative', display: 'flex', alignItems: 'center', gap: 10, minWidth: 0 },
  agentName: {
    fontSize: 13.5, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  // Main.dc.html:390 — 12px, fg-4.
  agentMeta: {
    fontSize: 12, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  // Badge Kit (22 · radius 6) nhưng phải chịu được nhãn dài: cắt ở 45% bề rộng hàng.
  agentBadge: { flexShrink: 0, flexGrow: 0, flexBasis: 'auto', maxWidth: '45%', minWidth: 0 },
  agentBadgeText: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  // Main.dc.html:395 — hàng ngữ cảnh: nhãn · thanh 3px · % · · scope.
  ctxRow: { display: 'flex', alignItems: 'center', gap: 8, fontSize: 12, color: color['--color-fg-4'], minWidth: 0 },
  ctxLabel: { flexShrink: 0 },
  ctxTrack: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 12 },
  ctxPct: (tone: string) => ({ flexShrink: 0, color: tone }),
  ctxScope: { flexGrow: 0, flexShrink: 1, flexBasis: 'auto', minWidth: 0, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  // 04-components › AgentCard: `context.strategy` — chỉ hiện khi có nhãn.
  ctxStrategy: {
    fontSize: 11.5, color: color['--color-fg-4'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },

  // ─── RewriteQueueCard ─────────────────────────────────────────────────
  // Main.dc.html:410-411 — “Lý do · …”, “Hiện tại · …”; chuỗi dài xuống dòng, không tràn.
  rewriteValue: { color: color['--color-fg-2'], overflowWrap: 'anywhere' },
  rewriteCurrent: { color: color['--color-warn'], overflowWrap: 'anywhere' },

  // ─── CostTab ──────────────────────────────────────────────────────────
  // Main.dc.html:418 — $ 28 + badge “tiết kiệm”; xuống dòng khi panel quá hẹp.
  costHead: { display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap', minWidth: 0 },
  savedBadge: {
    height: 22, paddingInline: 7, display: 'flex', alignItems: 'center', borderRadius: 6,
    backgroundColor: tint['--tint-ok'], color: color['--color-ok'], fontSize: 12, fontWeight: 500,
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap', minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis',
  },
  // Main.dc.html:431/434 — orb 14 · tên · token · $44.
  row: { display: 'flex', alignItems: 'center', gap: 8, paddingBlock: 3, fontSize: 12, minWidth: 0 },
  rowName: {
    flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 0,
    color: color['--color-fg-2'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  rowMeta: { flexShrink: 0, color: color['--color-fg-4'] },
  rowCost: { flexShrink: 0, width: 44, textAlign: 'right' },
  // Main.dc.html:445 — tên + % bên trái, “đọc / vào” bên phải trên một dòng.
  cacheRow: { display: 'flex', flexDirection: 'column', gap: 5, minWidth: 0 },
  cacheHead: { display: 'flex', justifyContent: 'space-between', gap: 8, fontSize: 12, minWidth: 0 },
  cacheNames: {
    minWidth: 0, color: color['--color-fg-2'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
  },
  cacheDetail: { flexShrink: 0, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  // Tab Truyện (nội dung) có module style riêng: `storyTab.stylex.ts` (Gói P2).
})
