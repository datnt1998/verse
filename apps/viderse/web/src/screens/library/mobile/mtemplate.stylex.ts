// Nhịp khung MTemplate (artboard MTemplate.dc.html, đợt 38): danh sách mục,
// thẻ xem thử, danh sách sẵn sàng, thanh "Đã lưu". Chỉ token.
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { corners, space as s } from '../../../styles/viderse.stylex'

export const mtpl = stylex.create({
  scroll: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-12'],
    paddingInline: s['--vv-s-16'],
    paddingBottom: s['--vv-s-24'],
  },
  sectionTitle: {
    margin: 0,
    color: color['--color-fg-4'],
    fontSize: s['--vv-s-11'],
    fontWeight: 600,
    letterSpacing: '.04em',
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
    backgroundColor: color['--color-card'],
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
  frameState: {
    position: 'absolute',
    left: s['--vv-s-10'],
    bottom: s['--vv-s-10'],
    paddingInline: s['--vv-s-8'],
    paddingBlock: s['--vv-s-3'],
    borderRadius: corners['--vv-r-pill'],
    backgroundColor: 'var(--vv-thumb-veil)',
    color: color['--color-fg'],
    fontSize: s['--vv-s-11'],
  },
  body: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-8'],
    padding: s['--vv-s-12'],
  },
  stateRow: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-6'],
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-12'],
  },
  dot: {
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
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-12'],
    lineHeight: 1.5,
  },
  actions: {
    display: 'flex',
    gap: s['--vv-s-8'],
  },

  // ── Danh sách mục ───────────────────────────────────────────────────────
  navCard: {
    overflow: 'hidden',
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    borderRadius: corners['--vv-r-16'],
    backgroundColor: color['--color-card'],
  },
  navHead: {
    paddingInline: s['--vv-s-12'],
    paddingTop: s['--vv-s-10'],
    paddingBottom: s['--vv-s-4'],
  },
  navRows: {
    display: 'flex',
    flexDirection: 'column',
  },

  // ── Danh sách sẵn sàng ──────────────────────────────────────────────────
  ready: {
    display: 'flex',
    flexDirection: 'column',
    gap: s['--vv-s-8'],
    padding: s['--vv-s-12'],
    borderWidth: 1,
    borderStyle: 'solid',
    borderColor: line['--line-2'],
    borderRadius: corners['--vv-r-16'],
    backgroundColor: color['--color-card'],
  },
  readyTitle: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-8'],
    color: color['--color-fg'],
    fontSize: s['--vv-s-14'],
    fontWeight: 600,
  },
  readyItem: {
    display: 'flex',
    alignItems: 'flex-start',
    gap: s['--vv-s-8'],
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-12'],
    lineHeight: 1.45,
  },
  tick: {
    flexShrink: 0,
    color: color['--color-ok'],
  },

  // ── Thanh lưu ───────────────────────────────────────────────────────────
  saveBar: {
    display: 'flex',
    alignItems: 'center',
    gap: s['--vv-s-8'],
    color: color['--color-fg-3'],
    fontSize: s['--vv-s-12'],
    lineHeight: 1.4,
  },
  saveText: {
    flex: 1,
    minWidth: 0,
  },

  // ── Màn con ─────────────────────────────────────────────────────────────
  section: {
    paddingInline: s['--vv-s-16'],
    paddingBottom: s['--vv-s-24'],
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
