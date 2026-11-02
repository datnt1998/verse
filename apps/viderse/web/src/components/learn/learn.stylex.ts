// Kiểu banner học (.vv-learn, .vv-banner--ready) và dòng quy tắc (.vv-rule) — số đo từ
// getComputedStyle của artboard Main `agent=learn`, Review `state=learn`, Channel `tab=settings`.
import * as stylex from '@stylexjs/stylex'
import { color, kit } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'

const NO_MOTION = '@media (prefers-reduced-motion: reduce)'

export const learnStyles = stylex.create({
  // Banner "Đạo diễn/Hoạ sĩ nhận ra…"
  learn: {
    display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr)', gap: `${s['--vv-s-6']} ${s['--vv-s-10']}`, alignItems: 'start',
    padding: `${s['--vv-s-10']} ${s['--vv-s-12']}`, borderRadius: r['--vv-r-12'], fontSize: 12.5, lineHeight: 1.5,
    backgroundColor: 'var(--vv-director-notice-fill)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--vv-director-notice-line)',
    color: 'var(--vv-director-notice-ink)',
  },
  learnArtist: { backgroundColor: 'var(--vv-artist-notice-fill)', borderColor: 'var(--vv-artist-notice-line)', color: 'var(--vv-artist-notice-ink)' },
  orb: { display: 'flex', marginTop: s['--vv-s-2'] },
  text: { minWidth: 0, overflowWrap: 'anywhere' },
  actions: { gridColumn: 2, display: 'flex', flexWrap: 'wrap', gap: s['--vv-s-6'] },
  notice: { gridColumn: 2, color: 'var(--vv-warn-ink)' },
  // Banner đã lưu (.vv-banner--ready)
  saved: {
    display: 'flex', alignItems: 'center', gap: s['--vv-s-12'], padding: `${s['--vv-s-8']} ${s['--vv-s-10']}`, borderRadius: r['--vv-r-12'],
    fontSize: 12.5, backgroundColor: 'var(--vv-ok-6)', borderWidth: 1, borderStyle: 'solid', borderColor: 'var(--vv-ok-20)', color: color['--color-fg'],
  },
  savedIcon: { display: 'flex', flex: '0 0 auto', marginTop: s['--vv-s-2'], color: 'var(--vv-prov-sub-ok)' },
  savedText: { flex: 1, minWidth: 0, overflowWrap: 'anywhere' },
  // Ví dụ trước → sau
  examples: { gridColumn: 2, margin: 0, padding: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: s['--vv-s-6'] },
  example: {
    display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], padding: `${s['--vv-s-6']} ${s['--vv-s-8']}`, borderRadius: r['--vv-r-8'],
    backgroundColor: 'var(--vv-hover-row)', color: color['--color-fg-2'],
  },
  exampleRow: { display: 'flex', gap: s['--vv-s-8'], alignItems: 'baseline', overflowWrap: 'anywhere' },
  exampleTag: { flex: '0 0 auto', minWidth: 28, fontSize: 11, color: color['--color-fg-3'] },

  // Khối Quy tắc đã học (Kênh › Cài đặt) / Ghi chú đã học (Thư viện › Prompt ảnh)
  rules: {
    display: 'flex', flexDirection: 'column', borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--vv-hairline)', paddingTop: s['--vv-s-10'],
  },
  rulesHead: { display: 'flex', alignItems: 'baseline', gap: s['--vv-s-8'], flexWrap: 'wrap', paddingBottom: s['--vv-s-4'] },
  rulesTitle: { fontSize: 12.5, color: color['--color-fg-3'] },
  rulesHelp: { flex: '1 1 0', fontSize: 11.5, lineHeight: 1.5, color: 'var(--vv-fg-note)' },
  counter: { fontSize: 11, color: 'var(--vv-fg-note)', fontVariantNumeric: 'tabular-nums' },
  rule: {
    display: 'grid', gridTemplateColumns: 'auto minmax(0, 1fr) auto', gap: `${s['--vv-s-2']} ${s['--vv-s-12']}`, alignItems: 'start',
    padding: `${s['--vv-s-10']} 0`, borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--vv-hairline)',
  },
  ruleMobile: { alignItems: 'center', minHeight: 56 },
  ruleSwitchMobile: { gridRow: 'span 2', width: 56, height: 44, marginLeft: -3, display: 'flex', alignItems: 'center' },
  ruleSwitchMobileHit: { width: 56, marginBlock: 0, marginInline: 0 },
  ruleSwitch: {
    gridRow: 'span 2', marginTop: s['--vv-s-1'], position: 'relative', width: 36, height: 22, flex: '0 0 auto', borderWidth: 0, padding: 0,
    borderRadius: r['--vv-r-pill'], backgroundColor: color['--color-surface-7'], cursor: 'pointer',
    transition: { default: 'background-color .2s', [NO_MOTION]: 'none' },
    ':disabled': { opacity: 0.4, cursor: 'not-allowed' },
  },
  ruleSwitchOn: { backgroundColor: kit['--kit-switch-on'] },
  knob: {
    position: 'absolute', top: s['--vv-s-3'], left: s['--vv-s-3'], width: 16, height: 16, borderRadius: '50%', backgroundColor: color['--color-fg'],
    boxShadow: 'var(--vv-knob-shadow)', transition: { default: 'left .3s ease', [NO_MOTION]: 'none' },
  },
  knobOn: { left: 17 },
  ruleText: { fontSize: 13.5, lineHeight: 1.45, color: color['--color-fg'], overflowWrap: 'anywhere' },
  ruleSrc: { fontSize: 12, lineHeight: 1.55, color: 'var(--vv-fg-note)' },
  ruleTrash: { width: 30, paddingInline: 0 },
  ruleActions: { gridRow: 'span 2', display: 'flex', gap: s['--vv-s-4'] },
  ruleExamples: { gridColumn: '2 / -1' },
  setRow: { display: 'flex', alignItems: 'center', gap: s['--vv-s-12'], padding: `${s['--vv-s-12']} ${s['--vv-s-16']}`, borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: 'var(--line-1)' },
  setSub: { paddingLeft: s['--vv-s-28'] },
  setText: { flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], fontSize: 13, color: color['--color-fg'] },
  setDesc: { fontSize: 12, lineHeight: 1.5, color: 'var(--vv-fg-note)' },
  setChoose: { fontSize: 12.5, fontWeight: 400 },
  state: { fontSize: 12, lineHeight: 1.5, color: color['--color-fg-3'], padding: `${s['--vv-s-6']} 0` },
  stateAction: { alignSelf: 'flex-start' },
  error: { fontSize: 12, color: 'var(--vv-warn-ink)', padding: `${s['--vv-s-4']} 0` },
})
