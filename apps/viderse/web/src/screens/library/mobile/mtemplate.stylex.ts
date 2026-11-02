// Nhịp khung MTemplate (artboard MTemplate.dc.html, đợt 38): danh sách mục,
// thẻ xem thử, danh sách sẵn sàng, thanh "Đã lưu". Chỉ token.
import * as stylex from '@stylexjs/stylex'
import { color, kit, surface as kitSurface, line } from '@noverse/ui/tokens.stylex'
import { corners, space as s } from '../../../styles/viderse.stylex'
import { libraryVars } from '../libraryTokens.stylex'

export const mtpl = stylex.create({
  headerTitle: {
    display: 'block',
    fontSize: s['--vv-s-14'],
    lineHeight: 1.3,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  scroll: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-18'],
    paddingTop: s['--vv-s-4'],
    paddingInline: s['--vv-s-16'],
    paddingBottom: s['--vv-s-28'],
    lineHeight: 1.55,
  },
  sectionTitle: {
    margin: 0,
    color: kit['--kit-fg-sub'],
    paddingTop: s['--vv-s-4'],
    paddingBottom: s['--vv-s-6'],
    fontSize: s['--vv-s-12'],
    fontWeight: 600,
    letterSpacing: '.06em',
    textTransform: 'uppercase',
  },

  // ── Thẻ xem thử ─────────────────────────────────────────────────────────
  card: {
    display: 'flex',
    flexDirection: 'column',
    overflow: 'hidden',
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    borderRadius: corners['--vv-r-16'],
    backgroundImage: kitSurface['--card-bg'],
    boxShadow: kitSurface['--card-shadow'],
  },
  frame: {
    position: 'relative',
    aspectRatio: '16 / 9',
    backgroundImage: 'linear-gradient(135deg, var(--color-surface-6), var(--color-surface-3))',
    overflow: 'hidden',
  },
  frameArt: {
    position: 'absolute',
    left: '18%',
    right: '18%',
    top: '26%',
    bottom: '18%',
    borderRadius: corners['--vv-r-12'],
    backgroundColor: 'var(--vv-thumb-veil)',
    opacity: .35,
  },
  body: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-8'],
    paddingTop: s['--vv-s-10'],
    paddingInline: s['--vv-s-12'],
    paddingBottom: s['--vv-s-12'],
  },
  stateRow: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-8'],
    color: kit['--kit-fg-sub'],
    fontSize: libraryVars.bodySmallFontSize,
  },
  previewState: {
    display: 'inline-flex',
    alignItems: 'center',
    gap: s['--vv-s-6'],
    color: color['--color-ok'],
    minWidth: 0,
  },
  previewWarn: { color: color['--color-warn'] },
  builtAt: {
    marginInlineStart: 'auto',
    flexShrink: 0,
    fontVariantNumeric: 'tabular-nums',
  },
  dot: {
    flexShrink: 0,
    width: s['--vv-s-7'],
    height: s['--vv-s-7'],
    borderRadius: '50%',
    backgroundColor: color['--color-ok'],
  },
  dotWarn: {
    backgroundColor: color['--color-warn'],
  },
  help: {
    margin: 0,
    color: kit['--kit-fg-sub'],
    fontSize: libraryVars.captionFontSize,
    lineHeight: 1.5,
  },
  previewProgress: {
    margin: 0,
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-12'],
    fontVariantNumeric: 'tabular-nums',
  },
  previewError: {
    margin: 0,
    color: color['--color-err'],
    fontSize: s['--vv-s-12'],
  },
  rebuild: {
    width: '100%',
    minHeight: s['--vv-s-44'],
    borderRadius: corners['--vv-r-12'],
    fontSize: s['--vv-s-14'],
  },

  // ── Danh sách mục ───────────────────────────────────────────────────────
  settings: {
    display: 'flex',
    flexDirection: 'column',
  },
  navRows: {
    display: 'flex',
    flexDirection: 'column',
  },
  navRow: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-12'],
    width: '100%',
    minHeight: s['--vv-s-56'],
    paddingBlock: s['--vv-s-8'],
    paddingInline: s['--vv-s-2'],
    borderWidth: 0,
    borderBottomWidth: 1,
    borderBottomStyle: 'solid',
    borderBottomColor: kit['--kit-wash-5'],
    borderRadius: 0,
    backgroundColor: 'transparent',
    color: color['--color-fg'],
    fontFamily: 'inherit',
    lineHeight: 1.55,
    textAlign: 'start',
    cursor: 'pointer',
    ':hover': { backgroundColor: 'var(--vv-hover-row)' },
    ':focus-visible': {
      outlineWidth: 2,
      outlineStyle: 'solid',
      outlineColor: line['--line-focus'],
      outlineOffset: -2,
    },
  },
  navIcon: {
    flexShrink: 0,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: s['--vv-s-36'],
    height: s['--vv-s-36'],
    borderRadius: corners['--vv-r-10'],
    backgroundColor: color['--color-surface-4'],
    color: color['--color-fg-2'],
  },
  navText: {
    flex: 1,
    minWidth: 0,
    display: 'flex',
    flexDirection: 'column',
  },
  navLabel: {
    fontSize: s['--vv-s-15'],
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  navHint: {
    fontSize: libraryVars.bodySmallFontSize,
    color: kit['--kit-fg-sub'],
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  navChevron: { flexShrink: 0, color: kit['--kit-fg-sub'] },

  // ── Danh sách sẵn sàng ──────────────────────────────────────────────────
  ready: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-6'],
    paddingBlock: s['--vv-s-12'],
    paddingInline: s['--vv-s-14'],
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    borderRadius: corners['--vv-r-14'],
    backgroundImage: kitSurface['--card-bg'],
    boxShadow: kitSurface['--card-shadow'],
  },
  readyTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-8'],
    color: color['--color-ok'],
    fontSize: libraryVars.controlFontSize,
    fontWeight: 600,
  },
  readyItem: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-8'],
    color: color['--color-fg-2'],
    fontSize: s['--vv-s-13'],
    lineHeight: 1.55,
  },
  tick: {
    flexShrink: 0,
    color: color['--color-ok'],
  },
  readyNeutral: { flexShrink: 0, color: color['--color-fg-3'] },

  // ── Thanh lưu ───────────────────────────────────────────────────────────
  footer: {
    paddingInline: 0,
    paddingTop: 0,
    backgroundImage: 'none',
  },
  saveBar: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-8'],
    paddingBlock: s['--vv-s-8'],
    paddingInline: s['--vv-s-16'],
    borderTopWidth: 1,
    borderTopStyle: 'solid',
    borderTopColor: line['--line-2'],
    color: color['--color-fg-3'],
    fontSize: libraryVars.bodySmallFontSize,
    lineHeight: 1.4,
  },
  saveText: {
    flex: 1,
    minWidth: 0,
  },

  // ── Màn con ─────────────────────────────────────────────────────────────
  section: {
    paddingTop: s['--vv-s-4'],
    paddingInline: s['--vv-s-16'],
    paddingBottom: s['--vv-s-28'],
  },

  // ── Hộp đổi tên ─────────────────────────────────────────────────────────
  field: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-6'],
  },
  fieldLabel: {
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-12'],
  },
  loading: {
    padding: s['--vv-s-24'],
    textAlign: 'center',
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-13'],
  },
})
