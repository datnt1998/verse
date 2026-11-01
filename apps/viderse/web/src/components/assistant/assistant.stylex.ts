// Kiểu của ô Nhắn trợ lý, thẻ Kế hoạch, thẻ xác nhận, thẻ "Chưa làm được việc này"
// (artboard Main `agent=plan`, Dialogs `assistant-confirm|assistant-unknown`,
// Kit › Trợ lý). Số đo lấy từ getComputedStyle của board ở 1440×900.
import * as stylex from '@stylexjs/stylex'
import { color, kit, line, radius, surface } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'
const WIDE = '@media (min-width: 1600px)'

export const assistantStyles = stylex.create({
  dock: { flex: '0 0 auto', display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'], minWidth: 0 },
  thread: { width: '100%', maxWidth: { default: 800, [WIDE]: 'none' }, margin: '0 auto', marginBottom: s['--vv-s-4'], display: 'flex', flexDirection: 'column', gap: s['--vv-s-8'] },
  wide: { maxWidth: 'none' },
  bubble: {
    alignSelf: 'flex-end', maxWidth: '70%', padding: `${s['--vv-s-9']} ${s['--vv-s-13']}`,
    borderRadius: `${r['--vv-r-14']} ${r['--vv-r-14']} ${r['--vv-r-4']} ${r['--vv-r-14']}`,
    backgroundColor: color['--color-surface-6'], color: color['--color-fg'], fontSize: 13.5, lineHeight: 1.5, overflowWrap: 'anywhere',
  },

  // Thẻ Kế hoạch (.vv-plan)
  plan: {
    display: 'flex', flexDirection: 'column', overflow: 'hidden', borderRadius: r['--vv-r-14'],
    backgroundColor: color['--color-surface-4'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'],
    boxShadow: '0 24px 60px -16px rgba(0,0,0,.9), 0 2px 8px rgba(0,0,0,.45), inset 0 1px 0 rgba(255,255,255,.05)',
    width: '100%', maxWidth: { default: 800, [WIDE]: 'none' }, margin: '0 auto',
  },
  planHead: { padding: `${s['--vv-s-12']} ${s['--vv-s-14']} ${s['--vv-s-8']}`, display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'] },
  planTitle: { fontSize: 14, fontWeight: 600, color: color['--color-fg'] },
  planQuote: { fontSize: 12, color: 'var(--vv-fg-note)', overflowWrap: 'anywhere' },
  step: {
    display: 'grid', gridTemplateColumns: 'auto auto minmax(0, 1fr) auto', gap: s['--vv-s-10'], alignItems: 'center',
    padding: `${s['--vv-s-9']} ${s['--vv-s-14']}`, borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--vv-hairline)',
    fontSize: 13, color: color['--color-fg-2'], cursor: 'pointer', outlineOffset: -2,
    borderRightWidth: 0, borderBottomWidth: 0, borderLeftWidth: 0, backgroundColor: 'transparent', width: '100%', textAlign: 'left', fontFamily: 'inherit',
  },
  stepLocked: { cursor: 'default' },
  stepText: { minWidth: 0 },
  stepTitle: { display: 'block', fontWeight: 500, color: color['--color-fg'], lineHeight: 1.4, overflowWrap: 'anywhere' },
  stepSub: { display: 'block', fontSize: 12, color: 'var(--vv-fg-note)', lineHeight: 1.4, overflowWrap: 'anywhere' },
  stepOff: { opacity: 0.5, textDecorationLine: 'line-through' },
  cost: { fontSize: 12, color: color['--color-fg-3'], whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' },
  state: { fontSize: 12, whiteSpace: 'nowrap', color: color['--color-fg-3'] },
  stateDone: { color: color['--color-ok'] },
  stateFailed: { color: 'var(--vv-warn-ink)' },
  check: {
    width: 18, height: 18, flex: '0 0 auto', display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
    borderRadius: r['--vv-r-5'], borderWidth: 1.5, borderStyle: 'solid', borderColor: 'var(--vv-line-28)', backgroundColor: 'transparent',
    transition: { default: 'background-color .16s, border-color .16s', [NO_MOTION]: 'none' },
  },
  checkOn: { backgroundColor: color['--color-accent'], borderColor: color['--color-accent'] },
  tick: {
    width: 5, height: 9, borderStyle: 'solid', borderColor: 'var(--vv-check-tick)', borderWidth: '0 2px 2px 0',
    transform: 'translateY(-1px) rotate(45deg)',
  },
  foot: {
    display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], padding: `${s['--vv-s-10']} ${s['--vv-s-14']}`,
    borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--vv-hairline)', backgroundColor: 'rgba(255,255,255,.02)', flexWrap: 'wrap',
  },
  footTotal: { flex: '1 1 160px', fontSize: 12.5, color: color['--color-fg-3'], minWidth: 0 },
  askQuestion: { padding: `0 ${s['--vv-s-14']} ${s['--vv-s-8']}`, fontSize: 13, color: color['--color-fg-2'] },
  ask: { display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'], padding: `0 ${s['--vv-s-14']} ${s['--vv-s-12']}` },
  chip: {
    height: 28, padding: `0 ${s['--vv-s-10']}`, display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-6'], borderRadius: r['--vv-r-8'],
    backgroundColor: color['--color-surface-1'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'],
    fontFamily: 'inherit', fontSize: 12, color: color['--color-fg-2'], whiteSpace: 'nowrap', cursor: 'pointer',
  },
  chipStatic: { cursor: 'default' },
  chipValue: { fontWeight: 500, color: color['--color-fg'] },
  chipIcon: { width: 13, height: 13, color: 'var(--vv-fg-note)' },

  // Thẻ xác nhận (Dialogs assistant-confirm) và thẻ "Chưa làm được việc này"
  card: {
    borderRadius: r['--vv-r-16'], padding: `${s['--vv-s-12']} ${s['--vv-s-14']}`, display: 'flex', flexDirection: 'column', gap: s['--vv-s-10'],
    backgroundColor: 'transparent', backgroundImage: 'linear-gradient(180deg, var(--color-surface-2) 0%, var(--color-surface-1) 100%)',
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'], boxShadow: surface['--card-shadow'],
  },
  cardConfirm: {
    borderColor: 'rgba(255,208,138,.32)',
    backgroundImage: 'linear-gradient(180deg, rgba(255,208,138,.06), rgba(255,208,138,.02))',
  },
  cardPlain: { gap: s['--vv-s-9'] },
  cardHead: { display: 'flex', alignItems: 'center', gap: s['--vv-s-10'] },
  cardTitle: { flex: 1, minWidth: 0, fontSize: 14, fontWeight: 600, color: color['--color-fg'], overflowWrap: 'anywhere' },
  detail: {
    margin: 0, padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-10'], backgroundColor: 'rgba(255,208,138,.06)',
    fontSize: 13, color: color['--color-fg-2'], lineHeight: 1.55, overflowWrap: 'anywhere',
  },
  lead: { margin: 0, fontSize: 13, color: color['--color-fg-3'], lineHeight: 1.55 },
  reruns: { display: 'flex', alignItems: 'center', gap: s['--vv-s-8'], flexWrap: 'wrap' },
  rerunLabel: { fontSize: 12, color: 'var(--vv-fg-note)' },
  grow: { flex: 1 },
  suggestions: { display: 'flex', gap: s['--vv-s-8'], flexWrap: 'wrap' },
  note: { fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.5 },
  noteError: { color: 'var(--vv-warn-ink)' },
  contents: { display: 'contents' },

  // Ô nhập (form.card của board)
  ringBox: { position: 'relative' },
  // Danh sách lệnh `/` mở ngay trên ô nhập, rộng bằng ô.
  slash: {
    position: 'absolute', insetInline: 0, bottom: '100%', marginTop: 0, marginBottom: s['--vv-s-6'], zIndex: 5, padding: s['--vv-s-4'], listStyle: 'none',
    display: 'flex', flexDirection: 'column', borderRadius: r['--vv-r-14'], backgroundColor: color['--color-surface-4'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], boxShadow: surface['--card-shadow'],
  },
  slashRow: {
    display: 'flex', alignItems: 'baseline', gap: s['--vv-s-12'], padding: `${s['--vv-s-8']} ${s['--vv-s-10']}`, borderRadius: r['--vv-r-8'],
    backgroundColor: 'transparent', cursor: 'pointer', fontSize: 13, color: color['--color-fg-3'],
  },
  slashRowOn: { backgroundColor: color['--color-surface-6'], color: color['--color-fg-2'] },
  slashUsage: { flex: '0 0 auto', minWidth: 248, fontFamily: 'var(--font-mono)', fontSize: 12.5, color: color['--color-fg'], whiteSpace: 'nowrap' },
  form: {
    position: 'relative', display: 'flex', alignItems: 'center', gap: s['--vv-s-10'], padding: `${s['--vv-s-7']} ${s['--vv-s-7']} ${s['--vv-s-7']} ${s['--vv-s-14']}`,
    borderRadius: radius['--radius-dock'], backgroundColor: color['--color-surface-3'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-2'],
    boxShadow: surface['--card-shadow'],
  },
  formIcon: { flex: '0 0 auto', color: 'var(--vv-fg-note)' },
  input: {
    flex: 1, minWidth: 0, height: 38, backgroundColor: 'transparent', borderWidth: 0, outline: 'none', color: color['--color-fg'], fontSize: 14, fontFamily: 'inherit',
  },
  consult: {
    height: 36, padding: `0 ${s['--vv-s-10']}`, display: 'inline-flex', alignItems: 'center', gap: s['--vv-s-6'], borderRadius: radius['--radius-row'],
    borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: 'transparent', color: color['--color-fg'], fontFamily: 'inherit',
    fontSize: 12.5, whiteSpace: 'nowrap', cursor: 'default',
  },
  send: {
    width: 38, height: 38, flex: '0 0 auto', borderRadius: '50%', borderWidth: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
    backgroundColor: { default: color['--color-fg'], ':hover:not(:disabled)': 'white' }, color: color['--color-bg'], cursor: 'pointer', padding: 0,
  },
  sendOff: { opacity: 0.42, cursor: 'not-allowed' },
  hints: {
    display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: `${s['--vv-s-4']} ${s['--vv-s-14']}`, padding: `0 ${s['--vv-s-6']}`,
    fontSize: 12, color: 'var(--vv-fg-note)',
  },
  kbd: { fontFamily: 'var(--font-mono)', fontSize: 11, color: color['--color-fg-3'], padding: `${s['--vv-s-1']} ${s['--vv-s-5']}`, borderRadius: r['--vv-r-4'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-4'], backgroundColor: kit['--kit-surface-15'] },
  quota: { display: 'flex', alignItems: 'center', gap: s['--vv-s-6'], whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' },
  quotaOk: { color: color['--color-ok'] },
  quotaFull: { color: 'var(--vv-warn-ink)' },
  hint: { display: 'flex', gap: s['--vv-s-6'], alignItems: 'center' },
})
