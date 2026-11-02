import * as stylex from '@stylexjs/stylex'

/** Exact Editor.dc.html surface values; all consumer styles reference these vars. */
export const v13 = stylex.defineVars({
  muted: '#8B8B97', danger: '#FF9A5C', dangerText: '#FFC09A',
  dangerBorder: 'rgba(255,154,92,.28)', dangerTint: 'rgba(255,154,92,.04)',
  dangerChipBorder: 'rgba(255,154,92,.35)', dangerChipTint: 'rgba(255,154,92,.06)',
  gateListTint: 'rgba(255,255,255,.03)',
  controlRadius: '10px', ghostBorder: 'rgba(255,255,255,.12)',
  focusOutline: '2px solid #E6002E',
  primary: '#EDEDEF', primaryHover: '#FFFFFF', primaryFg: '#0B0B0D',
  readyBorder: 'rgba(166,237,193,.3)', readyTint: 'rgba(166,237,193,.06)',
  choiceRadius: '11px', fieldBorder: 'rgba(255,255,255,.10)', stage: '#111113',
  /* Editor surfaces and track inks from Editor.dc.html. */
  text: '#EDEDEF', text2: '#C9C9D1', text3: '#A0A0AB', textHint: '#8E8E99',
  faint: '#6E6E78', hairline: 'rgba(255,255,255,.06)', rowHover: 'rgba(255,255,255,.04)',
  selectedRow: 'rgba(255,255,255,.08)', surface1: '#131315', surface2: '#161618',
  stageAlt: '#0D0D0F', scope: '#0F0F11', ipBox: '#101012',
  caption: '#D6BE5A', scene: '#C49BEA', overlay: '#E59DBB', video: '#7FB4E6', audio: '#78C99A',
  captionFill: 'rgba(214,190,90,.22)', captionBorder: 'rgba(214,190,90,.85)',
  sceneFill: 'rgba(196,155,234,.22)', sceneBorder: 'rgba(196,155,234,.85)',
  overlayFill: 'rgba(229,157,187,.22)', overlayBorder: 'rgba(229,157,187,.85)',
  videoFill: 'rgba(127,180,230,.22)', videoBorder: 'rgba(127,180,230,.85)',
  audioFill: 'rgba(120,201,154,.18)', audioBorder: 'rgba(120,201,154,.85)',
  accent: '#FF4D63', accentStrong: '#E6002E', ok: '#A6EDC1', warn: '#F6CB8C',
  warnText: '#FFC09A', warnFill: 'rgba(255,154,92,.07)', okFill: 'rgba(166,237,193,.06)',
  info: '#9DB0F7', imageBadge: 'rgba(11,11,13,.8)',
})
