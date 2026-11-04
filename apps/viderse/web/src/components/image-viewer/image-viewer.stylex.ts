// Lớp CSS của Trình xem ảnh dùng chung: khung tối, ảnh contain không phóng quá
// cỡ gốc, dải ảnh đủ ảnh, ô chưa vẽ là sọc. Chỉ dùng token sẵn có của Viderse
// và kit; không có mã màu mới.
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { corners as r, space as s, typography as type, imageViewer as layout } from '../../styles/viderse.stylex'

const NARROW = '@media (max-width: 1100px)'
const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

export const sx = stylex.create({
  // ── Modal ───────────────────────────────────────────────────────────────
  scrim: { backgroundColor: 'var(--vv-scrim)', backdropFilter: `blur(${s['--vv-s-4']})` },
  head: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], minWidth: 0, width: '100%' },
  headIcon: { flex: '0 0 auto', color: color['--color-fg-4'] },
  headTitle: { fontSize: type['--vv-fs-title-small'], fontWeight: 600, whiteSpace: 'nowrap' },
  headPos: { flex: '0 0 auto', fontSize: type['--vv-fs-title-small'], color: color['--color-fg-3'], fontVariantNumeric: 'tabular-nums' },
  headSub: { minWidth: 0, fontSize: type['--vv-fs-title-small'], color: color['--color-fg-3'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  headSpacer: { flex: 1 },
  tag: {
    flex: '0 0 auto', height: s['--vv-s-22'], paddingInline: s['--vv-s-8'], display: 'flex', alignItems: 'center',
    borderRadius: r['--vv-r-6'], borderWidth: 0, backgroundColor: kit['--kit-wash-6'],
    fontSize: type['--vv-fs-small'], color: color['--color-info'], whiteSpace: 'nowrap',
  },
  tagSample: { color: color['--color-fg-2'] },
  sideTitle: { fontSize: type['--vv-fs-body'], fontWeight: 600 },
  sideLink: { color: color['--color-fg-3'], textDecorationLine: 'underline', textUnderlineOffset: s['--vv-s-2'] },
  caption: { fontSize: type['--vv-fs-group'], fontWeight: 600, textTransform: 'uppercase', letterSpacing: '.06em', color: color['--color-fg-3'], margin: 0 },
  body: { minHeight: 0, display: 'grid', gridTemplateColumns: `minmax(0, 1fr) ${layout.side}`, gridTemplateRows: 'minmax(0, 1fr)' },
  bodySolo: { gridTemplateColumns: 'minmax(0, 1fr)' },
  stageCol: { minWidth: 0, minHeight: 0, padding: s['--vv-s-16'], display: 'flex', flexDirection: 'column', gap: s['--vv-s-12'] },
  stageBox: {
    position: 'relative', flex: '1 1 auto', minHeight: 0, display: 'grid', placeItems: 'center',
    overflow: 'hidden', borderRadius: r['--vv-r-12'], backgroundColor: layout.stageBackground,
  },
  // Neo vào khung đã tính xong để trần 100% không phụ thuộc kích thước nội tại của ảnh.
  // `auto` giữ tỉ lệ và cỡ gốc; ảnh dọc/ngang chỉ thu nhỏ khi chạm mép khung.
  image: { position: 'absolute', inset: 0, margin: 'auto', display: 'block', width: 'auto', height: 'auto', maxWidth: '100%', maxHeight: '100%', objectFit: 'contain' },
  stageNotice: { display: 'flex', flexDirection: 'column', alignItems: 'center', gap: s['--vv-s-8'], padding: s['--vv-s-16'], color: color['--color-fg-3'], fontSize: type['--vv-fs-ui'], textAlign: 'center' },
  nav: {
    position: 'absolute', top: '50%', transform: 'translateY(-50%)', width: s['--vv-s-38'], height: s['--vv-s-38'], borderRadius: '50%',
    borderWidth: s['--vv-s-1'], borderStyle: 'solid', borderColor: 'var(--vv-line-28)', backgroundColor: 'var(--vv-scrim-60)',
  },
  prev: { left: s['--vv-s-12'] },
  next: { right: s['--vv-s-12'] },
  prevIcon: { transform: 'rotate(180deg)' },

  // ── Dải ảnh ─────────────────────────────────────────────────────────────
  strip: { flex: '0 0 auto', display: 'flex', gap: s['--vv-s-8'], minWidth: 0, overflowX: 'auto', paddingBlockEnd: s['--vv-s-2'] },
  thumb: {
    position: 'relative', flex: `0 0 calc((100% - ${s['--vv-s-48']}) / 7)`, minWidth: 0, aspectRatio: '16 / 9', padding: 0, borderWidth: 0, borderStyle: 'none',
    borderRadius: r['--vv-r-7'], overflow: 'hidden', backgroundColor: color['--color-surface-3'],
    opacity: .55, cursor: 'pointer', transition: 'opacity 160ms, box-shadow 160ms',
    [NO_MOTION]: { transition: 'none' },
    ':hover': { opacity: .85 },
    ':focus-visible': { outline: '2px solid var(--vv-red-line)', outlineOffset: 2 },
  },
  thumbOn: { opacity: 1, outline: '2px solid var(--color-fg)', outlineOffset: -2 },
  thumbWait: {
    opacity: 1, cursor: 'default', borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
    backgroundImage: 'repeating-linear-gradient(135deg, var(--color-surface-1) 0 8px, var(--color-surface-2) 8px 16px)',
  },
  thumbImage: { position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover' },
  thumbKey: {
    position: 'absolute', left: s['--vv-s-5'], top: s['--vv-s-4'], fontSize: type['--vv-fs-badge'], fontWeight: 600,
    color: color['--color-fg'], textShadow: '0 1px 3px var(--vv-scrim)',
  },

  // ── Cột phải ────────────────────────────────────────────────────────────
  side: {
    minHeight: 0, overflowY: 'auto', padding: `${s['--vv-s-14']} ${s['--vv-s-16']}`, fontSize: type['--vv-fs-detail'], lineHeight: 1.55,
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-16'],
    borderInlineStartWidth: 1, borderInlineStartStyle: 'solid', borderInlineStartColor: line['--line-1'],
  },
  top: {
    display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], minWidth: 0,
    paddingInlineStart: s['--vv-s-16'], paddingInlineEnd: s['--vv-s-10'],
    borderBottomWidth: s['--vv-s-1'], borderBottomStyle: 'solid', borderBottomColor: line['--line-1'],
  },
  footer: {
    display: 'flex', alignItems: 'center', minWidth: 0, paddingInline: s['--vv-s-16'],
    borderTopWidth: s['--vv-s-1'], borderTopStyle: 'solid', borderTopColor: line['--line-1'],
  },
  footerRow: { flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'], fontSize: type['--vv-fs-detail'], lineHeight: 1.4 },

  // ── Inline (màn Duyệt ba cột) ───────────────────────────────────────────
  inlineRoot: { display: 'contents' },
})
