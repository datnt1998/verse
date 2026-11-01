// Kiểu của Bàn dựng điện thoại (artboard MMain). Mọi khoảng cách/góc lấy token
// --vv-s-*/--vv-r-*; màu lấy token kit để luật thiết kế không bắt hex.
import * as stylex from '@stylexjs/stylex'
import { color, line, kit } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../../styles/viderse.stylex'

export const mw = stylex.create({
  listRoot: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-16'],
    paddingTop: s['--vv-s-12'], paddingInline: s['--vv-s-16'], paddingBottom: s['--vv-s-8'],
  },
  detailRoot: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-14'],
    paddingTop: s['--vv-s-8'], paddingInline: s['--vv-s-16'], paddingBottom: s['--vv-s-8'],
  },
  recapSheet: { top: 0, height: '100dvh', '--m-sheet-max-height': '100dvh', borderRadius: 0, paddingBottom: 'max(8px, var(--m-safe-bottom))' },
  recapHeader: { position: 'sticky', top: 0, zIndex: 1, backgroundColor: color['--color-surface-3'], paddingTop: `calc(var(--m-safe-top) + ${s['--vv-s-8']})`, paddingBottom: s['--vv-s-8'] },
  recapHeading: { margin: 0, fontSize: 18, fontWeight: 600, color: color['--color-fg'] },
  recapSubtitle: { margin: `${s['--vv-s-4']} 0 0`, fontSize: 12.5, color: color['--color-fg-3'] },
  recapRoot: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-16'],
    paddingTop: s['--vv-s-8'], paddingInline: s['--vv-s-16'], paddingBottom: s['--vv-s-8'],
  },
  headRow: { display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: s['--vv-s-12'] },
  h1: { margin: 0, fontSize: 24, lineHeight: 1.25, fontWeight: 600, color: color['--color-fg'], letterSpacing: '-.01em' },
  dayBlock: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], marginTop: s['--vv-s-6'] },
  dayLabel: { flexShrink: 0, fontSize: 13, color: color['--color-fg-3'], fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap' },
  dayCount: { fontWeight: 600, color: color['--color-fg'] },
  dayMeter: { flex: 1, minWidth: 16 },
  daySub: { flexShrink: 0, fontSize: 13, color: color['--color-fg-3'], whiteSpace: 'nowrap' },
  section: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  sectionHead: { display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: s['--vv-s-8'] },
  sectionTitle: { margin: 0, fontSize: 13, fontWeight: 600, color: color['--color-fg-3'], letterSpacing: '.01em' },
  sectionCount: { fontSize: 12, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },

  // Thẻ Bản tin
  recapCard: {
    width: '100%', display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], textAlign: 'left',
    padding: s['--vv-s-14'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
    borderRadius: r['--vv-r-16'], backgroundColor: color['--color-surface-1'], color: color['--color-fg'],
    cursor: 'pointer',
  },
  recapTop: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'] },
  recapTitle: { fontSize: 12.5, fontWeight: 600, letterSpacing: '.04em', textTransform: 'uppercase', color: color['--color-fg-3'] },
  recapWhen: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  recapSummary: { fontSize: 13.5, lineHeight: 1.5, color: color['--color-fg-2'], display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' },
  recapCta: { fontSize: 12.5, fontWeight: 600, color: color['--color-fg'] },

  // Rail ngang (đề xuất, kênh)
  rail: {
    display: 'flex', gap: s['--vv-s-10'], overflowX: 'auto', overflowY: 'hidden',
    marginInline: `calc(-1 * ${s['--vv-s-16']})`, paddingInline: s['--vv-s-16'], paddingBottom: s['--vv-s-2'],
    scrollSnapType: 'x mandatory', scrollbarWidth: 'none',
  },
  railItem: { flex: '0 0 auto', scrollSnapAlign: 'start' },
  suggestCard: { width: 288, minWidth: 288 },
  channelCard: {
    flex: '0 0 158px', display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], textAlign: 'left',
    padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
    borderRadius: r['--vv-r-14'], backgroundColor: color['--color-surface-1'], color: color['--color-fg'],
    cursor: 'pointer',
  },
  channelCardOn: { borderColor: kit['--kit-acc-55'], boxShadow: `inset 0 0 0 1px ${kit['--kit-acc-55']}` },
  channelTop: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], minWidth: 0, width: '100%' },
  channelName: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, fontWeight: 600 },
  channelMeta: { fontSize: 12, color: color['--color-fg-3'], fontVariantNumeric: 'tabular-nums' },
  channelToday: { flex: 1, fontSize: 12, color: color['--color-fg-3'], fontVariantNumeric: 'tabular-nums' },
  channelWarn: { color: 'var(--vv-warn-ink)' },
  channelBar: { display: 'block', width: '100%', height: 3, borderRadius: r['--vv-r-pill'], backgroundColor: color['--color-surface-2'], overflow: 'hidden' },
  channelBarFill: (pct: number) => ({ display: 'block', width: `${pct}%`, height: '100%', backgroundColor: color['--color-fg-3'] }),
  allStack: { display: 'flex', alignItems: 'center' },
  allStackItem: { marginLeft: `calc(-1 * ${s['--vv-s-6']})` },
  allStackFirst: { marginLeft: 0 },

  // Danh sách video
  jobList: { display: 'flex', flexDirection: 'column' },
  jobMain: {
    flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: s['--vv-s-12'],
    padding: 0, borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', cursor: 'pointer',
  },
  jobThumb: {
    flex: '0 0 auto', width: 80, height: 45, borderRadius: r['--vv-r-8'], backgroundColor: color['--color-surface-2'],
    overflow: 'hidden',
  },
  jobThumbImg: { width: '100%', height: '100%', objectFit: 'cover', display: 'block' },
  jobText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'] },
  jobTitle: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13.5, fontWeight: 500 },
  jobStatus: { minWidth: 0, display: 'flex', alignItems: 'center', gap: s['--vv-s-4'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 11.5, color: color['--color-fg-3'] },
  jobAction: { flex: '0 0 auto', minWidth: 32, minHeight: 32, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: color['--color-fg-3'], textDecoration: 'none' },
  jobWrap: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], paddingBlock: s['--vv-s-8'], borderBottomWidth: 1, borderBottomStyle: 'solid', borderBottomColor: line['--line-1'] },
  jobRow: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'], paddingBlock: s['--vv-s-2'] },
  jobMeter: { paddingLeft: 'calc(80px + var(--vv-s-12))' },
  moreLink: {
    display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-4'], alignSelf: 'flex-start',
    minHeight: 36, paddingInline: s['--vv-s-2'], borderWidth: 0, backgroundColor: 'transparent',
    color: color['--color-fg-2'], fontSize: 12.5, textDecoration: 'none', cursor: 'pointer',
  },
  state: { padding: `${s['--vv-s-24']} ${s['--vv-s-16']}`, textAlign: 'center', fontSize: 13, lineHeight: 1.6, color: 'var(--vv-fg-note)' },
  stateTitle: { display: 'block', marginBottom: s['--vv-s-4'], fontSize: 15, fontWeight: 600, color: color['--color-fg'] },
  emptyActions: { display: 'flex', justifyContent: 'center', marginTop: s['--vv-s-12'] },

  // Chi tiết
  backRow: { display: 'flex', alignItems: 'center', gap: s['--vv-s-4'] },
  detailTitle: { margin: 0, fontSize: 20, lineHeight: 1.35, fontWeight: 600, color: color['--color-fg'] },
  detailTitleSm: { fontSize: 15 },
  detailMeta: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: s['--vv-s-6'], fontSize: 12.5, color: color['--color-fg-3'] },
  dot: { color: 'var(--vv-fg-note)' },
  chips: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'] },
  chip: {
    padding: `${s['--vv-s-4']} ${s['--vv-s-8']}`, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
    borderRadius: r['--vv-r-7'], backgroundColor: color['--color-surface-1'], color: color['--color-fg-3'], fontSize: 11.5,
  },
  banner: {
    display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'],
    padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    borderRadius: r['--vv-r-12'], color: color['--color-fg-2'], fontSize: 13, lineHeight: 1.5,
  },
  actionRow: { display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: s['--vv-s-8'] },
  card: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], padding: `${s['--vv-s-12']} ${s['--vv-s-14']}`,
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: r['--vv-r-14'],
    backgroundColor: color['--color-surface-1'],
  },
  cardHead: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'] },
  cardTitle: { margin: 0, fontSize: 14, fontWeight: 600, color: color['--color-fg'] },
  cardHint: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  agentGrid: { display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: s['--vv-s-8'] },
  agentTile: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'], padding: s['--vv-s-10'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: r['--vv-r-12'],
    backgroundColor: color['--color-surface-2'], minWidth: 0,
  },
  agentTop: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], minWidth: 0 },
  agentName: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12.5, fontWeight: 600 },
  agentState: { fontSize: 11.5, color: color['--color-fg-3'] },
  agentNow: { fontSize: 12, lineHeight: 1.45, color: color['--color-fg-2'], display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden' },
  agentMetric: { fontSize: 11.5, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },

  imageGrid: { display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: s['--vv-s-8'] },
  imageTile: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'], padding: 0, borderWidth: 0,
    backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', cursor: 'pointer', minWidth: 0,
  },
  imageCover: { gridColumn: '1 / -1', order: -1 },
  imageFrame: {
    position: 'relative', aspectRatio: '16 / 9', display: 'grid', placeItems: 'center',
    overflow: 'hidden', borderRadius: r['--vv-r-10'], backgroundColor: color['--color-surface-2'],
    color: 'var(--vv-fg-note)', fontSize: 12,
  },
  image: { width: '100%', height: '100%', objectFit: 'cover' },
  imageDrawingStatus: { position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', gap: s['--vv-s-6'], fontSize: 12 },
  imageBadge: {
    position: 'absolute', left: s['--vv-s-6'], top: s['--vv-s-6'], padding: `${s['--vv-s-2']} ${s['--vv-s-6']}`,
    borderRadius: r['--vv-r-6'], backgroundColor: 'var(--kit-scrim)', color: 'white', fontSize: 10.5,
  },
  imageCap: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 11.5, color: color['--color-fg-3'] },
  imageCapRow: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'], fontSize: 11.5, color: color['--color-fg-3'] },
  imageTime: { flexShrink: 0, fontVariantNumeric: 'tabular-nums' },
  promptLine: { fontSize: 12, lineHeight: 1.5, color: color['--color-fg-3'], overflowWrap: 'anywhere' },
  promptTag: { color: 'var(--vv-warn-ink)' },

  // Bản tin
  recapLead: { margin: 0, fontSize: 20, lineHeight: 1.4, fontWeight: 600, color: color['--color-fg'] },
  recapGroup: { fontSize: 11.5, fontWeight: 600, letterSpacing: '.06em', textTransform: 'uppercase', color: color['--color-fg-3'] },
  recapRow: {
    width: '100%', display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`,
    borderWidth: 0, borderRadius: r['--vv-r-10'], backgroundColor: color['--color-surface-1'],
    color: color['--color-fg-2'], fontSize: 13, textAlign: 'left', cursor: 'pointer',
  },
  recapRowText: { flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  recapRowState: { fontSize: 12, color: color['--color-fg-3'], whiteSpace: 'nowrap' },
  tiles: { display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: s['--vv-s-8'] },
  tile: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`,
    borderRadius: r['--vv-r-12'], backgroundColor: color['--color-surface-1'],
  },
  tileValue: { fontSize: 18, fontWeight: 600, color: color['--color-fg'], fontVariantNumeric: 'tabular-nums' },
  tileLabel: { fontSize: 11.5, color: color['--color-fg-3'] },
  autoLine: {
    display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], minHeight: 40,
    padding: `${s['--vv-s-6']} ${s['--vv-s-10']}`, borderRadius: r['--vv-r-10'],
    backgroundColor: color['--color-surface-1'], color: color['--color-fg-2'], fontSize: 13,
  },
  autoText: { flex: 1, minWidth: 0, overflowWrap: 'anywhere' },

  // Sheet nội dung đăng
  sheetBody: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-12'] },
  field: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  fieldHead: { display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: s['--vv-s-8'] },
  fieldLabel: { fontSize: 12, fontWeight: 600, color: color['--color-fg-2'] },
  edited: { fontSize: 11.5, color: 'var(--vv-warn-ink)' },
  input: {
    width: '100%', boxSizing: 'border-box', minHeight: 40, padding: `${s['--vv-s-8']} ${s['--vv-s-10']}`,
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], borderRadius: r['--vv-r-9'],
    backgroundColor: color['--color-surface-2'], color: color['--color-fg'], fontFamily: 'inherit', fontSize: 13.5,
  },
  textarea: { minHeight: 112, resize: 'vertical', lineHeight: 1.5 },
  tagRow: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'] },
  tag: {
    display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-4'], padding: `${s['--vv-s-4']} ${s['--vv-s-8']}`,
    borderRadius: r['--vv-r-pill'], backgroundColor: color['--color-surface-3'], color: color['--color-fg-2'], fontSize: 12,
  },
  tagRemove: { width: 24, minWidth: 24, height: 24, minHeight: 24, padding: 0 },
  metaActions: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-8'] },
  notice: { fontSize: 12.5, lineHeight: 1.5, color: 'var(--vv-fg-note)' },
  promptText: { margin: 0, fontSize: 12.5, lineHeight: 1.6, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', color: color['--color-fg-2'] },
  error: { fontSize: 12.5, lineHeight: 1.5, color: color['--color-err'] },
  historyRow: {
    display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], paddingBlock: s['--vv-s-8'],
    borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'],
  },
  historyText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'] },
  historyWho: { fontSize: 11.5, color: color['--color-fg-3'] },
  historyTitle: { fontSize: 13, color: color['--color-fg'], overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  historyWhat: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  rewriteBox: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'], padding: s['--vv-s-10'], borderRadius: r['--vv-r-12'], backgroundColor: color['--color-surface-2'] },
  chipRow: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'] },

  // Sheet ảnh
  viewer: { width: '100%', aspectRatio: '16 / 9', objectFit: 'contain', borderRadius: r['--vv-r-12'], backgroundColor: color['--color-surface-2'] },
  filmstrip: { display: 'flex', gap: s['--vv-s-6'], overflowX: 'auto', paddingBottom: s['--vv-s-4'] },
  filmThumb: {
    flex: '0 0 auto', width: 76, height: 44, padding: 0, overflow: 'hidden', borderWidth: 2,
    borderStyle: 'solid', borderColor: 'transparent', borderRadius: r['--vv-r-8'],
    backgroundColor: color['--color-surface-2'], cursor: 'pointer',
  },
  filmThumbOn: { borderColor: color['--color-fg'] },
  filmImage: { width: '100%', height: '100%', objectFit: 'cover' },
  versionRow: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'] },

  // Kế hoạch
  planBody: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-12'] },
  planHead: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-4'] },
  planTitle: { margin: 0, fontSize: 17, fontWeight: 600, color: color['--color-fg'] },
  planQuote: { margin: 0, fontSize: 12.5, lineHeight: 1.5, color: 'var(--vv-fg-note)', fontStyle: 'italic' },
  planStep: {
    display: 'flex', alignItems: 'flex-start', gap: s['--vv-s-10'], paddingBlock: s['--vv-s-8'],
    borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: line['--line-1'],
  },
  planStepText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'] },
  planStepTitle: { fontSize: 13.5, fontWeight: 600, color: color['--color-fg'] },
  planStepSub: { fontSize: 12, lineHeight: 1.45, color: color['--color-fg-3'] },
  planCost: { fontSize: 11.5, color: 'var(--vv-fg-note)' },
  planFooter: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'] },
  planTotal: { flex: 1, minWidth: 0, fontSize: 12.5, color: color['--color-fg-2'] },
  planAsk: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  runRow: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], fontSize: 12.5, color: color['--color-fg-2'] },
  runState: { marginLeft: 'auto', fontSize: 11.5, color: color['--color-fg-3'] },

  // Composer dính đáy (nằm trên thanh tab, hoặc trên safe-area khi ẩn tab)
  composerFoot: {
    position: 'sticky', bottom: 'var(--m-float-bottom)', zIndex: 6,
    display: 'flex', alignItems: 'center', gap: s['--vv-s-8'],
    marginTop: s['--vv-s-4'], paddingBlock: s['--vv-s-8'],
    backgroundColor: color['--color-bg'],
  },
  composerSheet: {
    position: 'sticky', bottom: 'max(8px, var(--m-safe-bottom))', zIndex: 6,
    display: 'flex', alignItems: 'center', gap: s['--vv-s-8'],
    marginTop: s['--vv-s-4'], paddingBlock: s['--vv-s-8'],
    backgroundColor: color['--color-bg'],
  },
  composerInput: {
    flex: 1, minWidth: 0, height: 44, boxSizing: 'border-box', paddingInline: s['--vv-s-12'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], borderRadius: r['--vv-r-12'],
    backgroundColor: color['--color-surface-2'], color: color['--color-fg'], fontFamily: 'inherit', fontSize: 14,
  },
  srOnly: { position: 'absolute', width: 1, height: 1, padding: 0, margin: -1, overflow: 'hidden', clip: 'rect(0 0 0 0)', whiteSpace: 'nowrap', borderWidth: 0 },
  kit: { color: kit['--kit-fg-sub'] },
})
