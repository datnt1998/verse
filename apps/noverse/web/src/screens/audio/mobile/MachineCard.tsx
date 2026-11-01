// §AT — thẻ Máy đọc đầu màn Nghe (MAudio.dc.html › tweak `machine`).
// Cùng khung với Bản đọc/Phòng thu trên máy tính; câu chữ và biến thể lấy từ
// `machineView` (../machine) — điện thoại chỉ vẽ lại cho khổ 390 px, không tự
// đặt câu, không bao giờ hiện mã lỗi.
import * as stylex from '@stylexjs/stylex'
import { Icon, Spinner } from '@noverse/ui'
import { color, kit, line } from '@noverse/ui/tokens.stylex'
import type { MachineActionKind, MachineTone, MachineView } from '../machine'

const s = stylex.create({
  card: {
    display: 'flex', flexDirection: 'column', gap: 10, margin: '6px 16px 12px', padding: 14,
    borderRadius: 18, backgroundColor: color['--color-card'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'],
  },
  cardInfo: { borderColor: kit['--kit-info-25'] },
  cardDanger: { backgroundColor: kit['--kit-errf-5'], borderColor: kit['--kit-errf-30'] },
  cardWarn: { borderColor: kit['--kit-warn-25'] },
  cardOk: { borderColor: kit['--kit-ok-25'] },
  head: { display: 'flex', alignItems: 'flex-start', gap: 12 },
  tile: {
    flexShrink: 0, width: 38, height: 38, borderRadius: 12, display: 'flex', alignItems: 'center', justifyContent: 'center',
    backgroundColor: kit['--kit-wash-6'], color: color['--color-fg-3'],
  },
  tileInfo: { backgroundColor: kit['--kit-info-12'], color: kit['--kit-info-fg'] },
  tileDanger: { backgroundColor: kit['--kit-errf-12'], color: color['--color-err'] },
  tileWarn: { backgroundColor: kit['--kit-warn-12'], color: color['--color-warn'] },
  tileOk: { backgroundColor: kit['--kit-ok-12'], color: color['--color-ok'] },
  body: { flexGrow: 1, flexBasis: 0, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 },
  title: { fontSize: 15.5, fontWeight: 600, lineHeight: 1.35 },
  chip: { alignSelf: 'flex-start', height: 22, padding: '0 8px', borderRadius: 7, fontSize: 12, fontWeight: 500, display: 'inline-flex', alignItems: 'center' },
  chipWarn: { backgroundColor: kit['--kit-warn-14'], color: color['--color-warn'] },
  chipError: { backgroundColor: kit['--kit-errf-14'], color: color['--color-err'] },
  sentence: { fontSize: 13.5, color: color['--color-fg-3'], lineHeight: 1.5 },
  bar: { display: 'flex', flexDirection: 'column', gap: 6 },
  track: { height: 6, borderRadius: 3, backgroundColor: kit['--kit-wash-10'], overflow: 'hidden' },
  fill: { height: '100%', borderRadius: 3, backgroundImage: 'linear-gradient(90deg, var(--kit-info-dot), var(--kit-info-fg))' },
  barNote: { fontSize: 12.5, color: color['--color-fg-3'] },
  spinNote: { fontSize: 12.5, color: color['--color-fg-3'] },
  action: {
    height: 44, borderRadius: 12, fontSize: 15, fontWeight: 600, display: 'flex', alignItems: 'center', justifyContent: 'center',
    backgroundColor: kit['--kit-wash-7'], color: color['--color-fg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-12'],
  },
  actionPrimary: { backgroundColor: color['--color-fg'], color: color['--color-bg'], borderColor: 'transparent' },
})

const TONE_CARD = { neutral: null, info: s.cardInfo, danger: s.cardDanger, warn: s.cardWarn, ok: s.cardOk } as const
const TONE_TILE = { neutral: null, info: s.tileInfo, danger: s.tileDanger, warn: s.tileWarn, ok: s.tileOk } as const

const tile = (tone: MachineTone) => stylex.props(s.tile, TONE_TILE[tone])

/** Thẻ đầu màn Nghe: 5 biến thể của §8.1/§8.3. `onAction` chỉ nhận `retry` (Thử lại) trên điện thoại. */
export function MachineCard({ view, onAction }: {
  view: MachineView
  onAction?: (kind: MachineActionKind) => void
}) {
  const percent = view.progress?.percent ?? null
  const primary = view.kind === 'provisionFailed'
  return <section role="status" aria-label="Máy đọc" {...stylex.props(s.card, TONE_CARD[view.tone])}>
    <div {...stylex.props(s.head)}>
      <span {...tile(view.tone)}>
        {view.spinning ? <Spinner size={17} color={kit['--kit-info-fg']} /> : <Icon name={view.icon} size={19} />}
      </span>
      <div {...stylex.props(s.body)}>
        <b {...stylex.props(s.title)}>{view.title}</b>
        {view.chip && <span {...stylex.props(s.chip, view.chip.tone === 'error' ? s.chipError : s.chipWarn)}>{view.chip.text}</span>}
        <span {...stylex.props(s.sentence)}>{view.cardText || view.sentence}</span>
      </div>
    </div>
    {view.progress && percent !== null && <div {...stylex.props(s.bar)}>
      <div role="progressbar" aria-label="Chuẩn bị bộ đọc" aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.round(percent)} {...stylex.props(s.track)}>
        <div {...stylex.props(s.fill)} style={{ width: `${Math.min(100, Math.max(0, percent))}%` }} />
      </div>
      <span {...stylex.props(s.barNote)}>{view.progress.label}</span>
    </div>}
    {view.progress && percent === null && <span {...stylex.props(s.spinNote)}>{view.progress.label}</span>}
    {(view.cardActions.length > 0 ? view.cardActions : view.actions).map((action) => <button key={action.kind} type="button" {...stylex.props(s.action, primary && s.actionPrimary)}
      onClick={() => onAction?.(action.kind)}>{action.label}</button>)}
  </section>
}
