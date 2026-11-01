import { useCallback, useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, frame, font, kit, line, tint } from '@noverse/ui/tokens.stylex'
import { getLiveHistory } from '../../api/client'
import type { LiveHistoryOperation, LiveHistoryRound } from '../../api/types'
import { useStoryEvents } from '../../api/useStoryEvents'
import { formatTimer } from '../../lib/format'
import { runCardModel } from '../../lib/runCardModel'
import type { RunCardBlock } from '../../lib/runCardModel'
import { mergeRunEvents, mergeRunHistory } from '../../lib/runHistoryMerge'
import { MobilePage } from '../../mobile/primitives'
import { AgentOrb, Icon, Shimmer, Spinner, Sheet as BottomSheet } from '@noverse/ui'
import type { AgentKind } from '@noverse/ui'
import { navigate, paths, useRoute } from '../../router'
import { formatRunDuration } from './format'
import { useNow } from './hooks'

const noop = () => {}
const roles: Record<string, string> = { writer: 'Người viết', architect: 'Kiến trúc sư', editor: 'Biên tập viên', arbiter: 'Trọng tài' }
/** Nhóm lọc của khung luồng viết (Main.dc.html › lvRoles): chỉ bốn vai có vòng trong lịch sử. */
const filters = [['all', 'Tất cả'], ['architect', roles.architect], ['writer', roles.writer], ['editor', roles.editor]] as const
const roleFg: Record<string, string> = { architect: 'var(--color-architect)', writer: 'var(--color-writer)', editor: 'var(--color-editor)', arbiter: 'var(--color-arbiter)' }
const AGENTS: AgentKind[] = ['architect', 'writer', 'editor', 'arbiter']
const agentKind = (role: string): AgentKind => AGENTS.find(kind => kind === role) ?? 'writer'
const icons: Record<string, string> = { novel_context: 'note-text', read_chapter: 'book-1', plan_chapter: 'routing-2', draft_chapter: 'edit-2', edit_chapter: 'magicpen', check_consistency: 'shield-tick', commit_chapter: 'folder-open', audit_foundation: 'shield-tick', revise_outline: 'routing-2', resolve_outline_feedback: 'tick-circle', save_book: 'document-text', save_foundation: 'lamp-on', save_review: 'chart', save_arc_summary: 'note-text', save_volume_summary: 'book-1' }
const clock = (at: string) => { const d = new Date(at); return Number.isNaN(d.valueOf()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' }) }
const s = stylex.create({
  button: { borderWidth: 0, backgroundColor: 'transparent', color: color['--color-fg'], cursor: 'pointer' },
  column: { display: 'flex', flexDirection: 'column', gap: 4, minWidth: 0 },
  label: { fontSize: 12, color: color['--color-fg-4'] },
  chips: { display: 'flex', flexWrap: 'wrap', gap: 6 },
  kv: { padding: '3px 8px', borderRadius: 6, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'], fontSize: 12, color: color['--color-fg-4'], lineHeight: 1.45, overflowWrap: 'anywhere', backgroundColor: color['--color-surface-0'] },
  value: { color: color['--color-fg'] },
  toneB: { backgroundColor: tint['--tint-info'] }, toneW: { backgroundColor: kit['--kit-warn-12'] }, toneR: { backgroundColor: tint['--tint-err'] }, toneG: { backgroundColor: tint['--tint-ok'] },
  fgB: { color: frame['--frame-info-fg'] }, fgW: { color: kit['--kit-warn-soft'] }, fgR: { color: kit['--kit-err-soft'] }, fgG: { color: color['--color-ok'] },
  list: { margin: 0, paddingLeft: 16, display: 'flex', flexDirection: 'column', gap: 3, fontSize: 13, color: color['--color-fg-2'], lineHeight: 1.55 },
  text: { margin: 0, fontSize: 13, color: color['--color-fg'], lineHeight: 1.6 },
  quote: { margin: 0, paddingLeft: 12, borderLeftWidth: 2, borderLeftStyle: 'solid', borderLeftColor: kit['--kit-acc-40'], fontFamily: font['--font-read'], fontSize: 14, color: kit['--kit-fg-note'], lineHeight: 1.65 },
  diff: { display: 'flex', flexDirection: 'column', borderRadius: 10, overflow: 'hidden', borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'], fontSize: 13, lineHeight: 1.6 },
  diffRow: { display: 'flex', gap: 10, padding: '8px 12px' }, diffOld: { backgroundColor: kit['--kit-errf-7'] }, diffNew: { backgroundColor: kit['--kit-ok-6'] },
  minus: { color: color['--color-err'], flexShrink: 0 }, plus: { color: color['--color-ok'], flexShrink: 0 },
  oldText: { color: color['--color-fg-2'], textDecorationLine: 'line-through', textDecorationColor: kit['--kit-errf-55'] },
  issues: { display: 'flex', flexDirection: 'column', gap: 8 },
  issue: { display: 'flex', flexDirection: 'column', gap: 4, padding: '8px 12px', borderRadius: 10, backgroundColor: color['--color-surface-0'], color: color['--color-fg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'] },
  issueHead: { display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
  chip: { height: 20, padding: '0 7px', display: 'inline-flex', alignItems: 'center', borderRadius: 6, fontSize: 11.5, fontWeight: 500, whiteSpace: 'nowrap', backgroundColor: color['--color-surface-0'], color: color['--color-fg'] },
  issueE: { backgroundColor: kit['--kit-errf-14'] }, issueW: { backgroundColor: kit['--kit-warn-14'] }, issueI: { backgroundColor: kit['--kit-up-badge-bg'], color: kit['--kit-up-badge-fg'] },
  meta: { fontSize: 12, color: kit['--kit-fg-meta'] }, issueText: { fontSize: 13, color: color['--color-fg'], lineHeight: 1.5 },
  issueFix: { fontSize: 12.5, color: color['--color-fg-3'], lineHeight: 1.5 }, fg2: { color: color['--color-fg-2'] },
  cast: { height: 26, maxWidth: '100%', padding: '0 10px 0 4px', display: 'inline-flex', alignItems: 'center', gap: 6, borderRadius: 999, backgroundColor: color['--color-surface-4'], color: color['--color-fg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-10'], fontSize: 12 },
  avatar: { width: 18, height: 18, borderRadius: '50%', backgroundColor: kit['--kit-acc-16'], color: color['--color-accent-soft'], fontStyle: 'normal', fontSize: 8.5, fontWeight: 700, display: 'flex', alignItems: 'center', justifyContent: 'center' },
  truncate: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  tag: { display: 'flex', gap: 10, alignItems: 'flex-start', padding: '8px 12px', borderRadius: 10, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'] },
  noShrink: { flexShrink: 0 }, tagText: { fontSize: 13, color: color['--color-fg-2'], lineHeight: 1.5, overflowWrap: 'anywhere' },
  scores: { display: 'grid', gridTemplateColumns: 'repeat(4,minmax(0,1fr))', gap: 6 }, scoresMobile: { gridTemplateColumns: 'repeat(2,minmax(0,1fr))' },
  score: { display: 'flex', flexDirection: 'column', gap: 2, padding: '7px 10px', borderRadius: 9, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'], fontSize: 12, color: kit['--kit-fg-meta'], minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' },
  scoreValue: { fontSize: 15, fontWeight: 600, color: color['--color-fg'], fontVariantNumeric: 'tabular-nums' },
  ordered: { listStyleType: 'none', margin: 0, padding: 0, display: 'grid', gridTemplateColumns: 'repeat(2,minmax(0,1fr))', gap: '3px 18px', fontSize: 13, color: color['--color-fg-2'] }, orderedMobile: { gridTemplateColumns: '1fr' },
  orderedItem: { display: 'flex', gap: 8, minWidth: 0 }, orderedNumber: { color: kit['--kit-fg-faint'], width: 14, flexShrink: 0, fontVariantNumeric: 'tabular-nums' },
  note: { margin: 0, fontSize: 12.5, color: kit['--kit-fg-meta'], lineHeight: 1.55 },
  link: { display: 'inline-flex', fontSize: 12.5, lineHeight: 1.5, color: color['--color-accent'], textDecorationLine: 'none', alignSelf: 'flex-start' },
  body: { padding: '2px 14px 12px 40px', fontSize: 13, lineHeight: 1.5, color: color['--color-fg-2'], display: 'flex', flexDirection: 'column', gap: 10 }, bodyMobile: { padding: '4px 18px 8px' },
  errorBody: { margin: 0, padding: '12px 14px', borderRadius: 10, backgroundColor: tint['--tint-err'], color: color['--color-err'], lineHeight: 1.5 },
  technical: { borderTopWidth: 1, borderTopStyle: 'solid', borderTopColor: kit['--kit-wash-6'], paddingTop: 8 },
  technicalHead: { display: 'flex', alignItems: 'center', gap: 6, height: 24, color: color['--color-fg-4'], fontSize: 12, cursor: 'pointer', listStyleType: 'none' },
  technicalCode: { marginTop: 6, padding: '8px 10px', borderRadius: 8, backgroundColor: color['--color-bg'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'], fontFamily: 'monospace', fontSize: 11.5, color: color['--color-fg-3'], lineHeight: 1.6, display: 'flex', flexDirection: 'column', gap: 2, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' },
  opIcon: { color: color['--color-ok'], flexShrink: 0 }, opCopy: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 0, display: 'flex', flexDirection: 'row', alignItems: 'center', gap: 12, textAlign: 'left' }, opCopyMobile: { flexDirection: 'column', alignItems: 'stretch', gap: 0 },
  opName: { fontSize: 13, whiteSpace: 'nowrap' }, opSummary: { overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12, color: color['--color-fg-4'] }, errorText: { color: color['--color-err'] }, opDuration: { fontSize: 12, color: color['--color-fg-4'], whiteSpace: 'nowrap' },
  opRow: { width: '100%', height: 40, padding: '0 12px', display: 'flex', alignItems: 'center', gap: 10 }, opRowMobile: { height: 58 },
  root: { display: 'flex', flexDirection: 'column', minHeight: 0, height: '100%', position: 'relative' },
  filters: { display: 'flex', gap: 8, overflowX: 'auto', padding: '8px 16px' }, filter: { padding: '7px 12px', whiteSpace: 'nowrap', borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: 30, backgroundColor: color['--color-surface-1'] }, filterSelected: { backgroundColor: color['--color-surface-5'] },
  scroll: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minHeight: 0, overflowY: 'auto', padding: '8px 7px 24px' }, scrollMobile: { padding: '8px 16px 24px' },
  round: { marginTop: 18 }, roundTitle: { fontSize: 12, fontWeight: 500, color: color['--color-fg-3'] },
  roundCards: { display: 'flex', flexDirection: 'column', gap: 6 }, roundCardsMobile: { gap: 0, borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'], borderRadius: 16, backgroundColor: color['--color-surface-1'], overflow: 'hidden' },
  floating: { position: 'absolute', bottom: 34, left: '50%', transform: 'translateX(-50%)', width: 243, height: 40, borderRadius: 30, backgroundColor: color['--color-surface-5'] },
  sheet: { maxHeight: 'min(760px, 90vh) !important' }, sheetContent: { overflowY: 'auto', paddingBottom: 8 }, sheetMeta: { padding: '0 18px', color: color['--color-fg-4'], fontSize: 12 }, sheetSummary: { padding: '0 18px', color: color['--color-fg-3'], fontSize: 13 },
  pagePanel: { borderRadius: 16, padding: '10px 14px 14px' },
})

// Gói AS — luồng viết trực tiếp gọn (Main.dc.html › phase rewrite · plan): hàng lọc cao 24,
// vòng là dòng 30 px có orb 11 px cùng hàng với chữ và đường dọc nối xuống thẻ, thẻ cao 36,
// thẻ đang chạy dính đáy khung.
const lv = stylex.create({
  bar: { flexShrink: 0, display: 'flex', alignItems: 'center', gap: 10, padding: '2px 0 4px' },
  seg: { display: 'flex', gap: 2, padding: 2, borderRadius: 9, backgroundColor: color['--color-surface-0'], borderWidth: 1, borderStyle: 'solid', borderColor: line['--line-1'] },
  segButton: { height: 24, padding: '0 10px', display: 'flex', alignItems: 'center', gap: 6, borderWidth: 0, borderRadius: 7, backgroundColor: 'transparent', color: kit['--kit-fg-icon'], fontSize: 12, whiteSpace: 'nowrap', cursor: 'pointer', ':hover': { color: color['--color-fg'] } },
  segSelected: { backgroundColor: color['--color-surface-6'], color: color['--color-fg'], boxShadow: 'inset 0 1px 0 var(--kit-wash-5)' },
  dot: { width: 7, height: 7, borderRadius: '50%', flexShrink: 0 },
  grow: { flexGrow: 1, flexShrink: 1, flexBasis: 0 },
  count: { fontSize: 12, color: color['--color-fg-4'], fontVariantNumeric: 'tabular-nums' },
  history: { display: 'flex', alignItems: 'center', gap: 3, fontSize: 12, color: color['--color-fg-3'], textDecorationLine: 'none' },
  rounds: { display: 'flex', flexDirection: 'column', paddingTop: 4 },
  pageRoot: { height: 'auto', display: 'flex', flexDirection: 'column' },
  pageScroll: { display: 'flex', flexDirection: 'column', paddingTop: 8 },
  more: { alignSelf: 'center', height: 26, margin: '4px 0 8px', padding: '0 10px', display: 'flex', alignItems: 'center', gap: 6, borderRadius: 999, borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-7'], backgroundColor: 'transparent', color: color['--color-fg-3'], fontSize: 12, cursor: 'pointer' },
  roundRow: { display: 'flex', flexDirection: 'column' },
  roundRowNext: { marginTop: 6 },
  roundHead: { width: '100%', height: 30, padding: '0 4px 0 0', display: 'flex', alignItems: 'center', gap: 8, borderWidth: 0, borderRadius: 8, backgroundColor: 'transparent', color: kit['--kit-fg-icon'], fontSize: 12, textAlign: 'left', cursor: 'pointer' },
  roundNumber: { fontSize: 12.5, fontWeight: 600, color: color['--color-fg-2'], whiteSpace: 'nowrap' },
  roundTitle: { minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' },
  roundOld: { color: kit['--kit-fg-faint'] },
  roundRule: { flex: '1 1 12px', height: 1, backgroundColor: kit['--kit-wash-6'] },
  roundTime: { fontSize: 12, fontVariantNumeric: 'tabular-nums' },
  ops: {
    position: 'relative', display: 'flex', flexDirection: 'column', gap: 4, padding: '2px 0 4px 22px',
    '::before': { content: '""', position: 'absolute', left: 5, top: 0, bottom: 6, width: 1, backgroundImage: `linear-gradient(${kit['--kit-wash-10']}, ${kit['--kit-wash-4']})` },
  },
  card: { borderRadius: 10, backgroundColor: color['--color-surface-1'], borderWidth: 1, borderStyle: 'solid', borderColor: kit['--kit-wash-6'], transitionProperty: 'border-color, background-color', transitionDuration: '.2s' },
  cardHover: { ':hover': { borderColor: kit['--kit-wash-10'] } },
  cardOpen: { backgroundColor: color['--color-surface-2'], borderColor: kit['--kit-wash-12'] },
  cardError: { backgroundColor: kit['--kit-errf-4'], borderColor: kit['--kit-errf-32'] },
  cardLive: { position: 'sticky', bottom: 0, zIndex: 2, borderColor: kit['--kit-acc-38'], backgroundColor: color['--color-card'], backgroundImage: `linear-gradient(90deg, ${kit['--kit-acc-10']}, ${kit['--kit-acc-3']} 60%)`, boxShadow: `0 -10px 18px -6px ${color['--color-surface-1']}` },
  cardHead: { width: '100%', height: 36, padding: '0 10px 0 11px', display: 'flex', alignItems: 'center', gap: 10, borderWidth: 0, borderRadius: 10, backgroundColor: 'transparent', color: color['--color-fg'], textAlign: 'left', cursor: 'pointer' },
  name: { flexShrink: 0, fontSize: 13, fontWeight: 500, whiteSpace: 'nowrap' },
  summary: { flexGrow: 1, flexShrink: 1, flexBasis: 0, minWidth: 0, fontSize: 12.5, color: kit['--kit-fg-icon'], whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' },
  summaryError: { color: kit['--kit-err-soft'] },
  duration: { flexShrink: 0, fontSize: 12, color: color['--color-fg-4'], fontVariantNumeric: 'tabular-nums' },
  empty: { display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6, padding: '28px 12px', textAlign: 'center' },
  emptyTitle: { fontSize: 13, color: color['--color-fg-2'] },
  emptyHint: { fontSize: 12, color: color['--color-fg-4'] },
})
const toneBg = { b: s.toneB, w: s.toneW, r: s.toneR, g: s.toneG }
const toneFg = { b: s.fgB, w: s.fgW, r: s.fgR, g: s.fgG }
const initials = (name: string) => name.split(/\s+/).map(w => [...w][0]).slice(-2).join('').toLocaleUpperCase('vi')

function Block({ block, project, story, mobile }: { block: RunCardBlock; project: string; story: string; mobile: boolean }) {
  switch (block.kind) {
    case 'kv': return <div {...stylex.props(s.chips)}>{block.items.map(([k, v, tone], i) => <span key={i} title={v} {...stylex.props(s.kv, tone && toneBg[tone])}>{k} <span {...stylex.props(s.value, tone && toneFg[tone])}>{v}</span></span>)}</div>
    case 'list': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{block.label}</span><ul {...stylex.props(s.list)}>{block.items.map((x, i) => <li key={i}>{x}</li>)}</ul></div>
    case 'text': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{block.label}</span><p {...stylex.props(s.text)}>{block.text}</p></div>
    case 'quote': return <p {...stylex.props(s.quote)}>{block.text}</p>
    case 'diff': return <div {...stylex.props(s.diff)}>
      <div {...stylex.props(s.diffRow, s.diffOld)}><span aria-label="Cũ" {...stylex.props(s.minus)}>−</span><span {...stylex.props(s.oldText)}>{block.old}</span></div>
      <div {...stylex.props(s.diffRow, s.diffNew)}><span aria-label="Mới" {...stylex.props(s.plus)}>+</span><span {...stylex.props(s.value)}>{block.next}</span></div>
    </div>
    case 'issues': return <div {...stylex.props(s.issues)}>{block.items.map((i, x) => {
      const title = i.level === 'e' ? 'Bắt sửa' : i.level === 'w' ? 'Cảnh báo' : 'Lưu ý'
      return <div key={x} {...stylex.props(s.issue)}>
        <span {...stylex.props(s.issueHead)}><span {...stylex.props(s.chip, i.level === 'e' ? s.fgR : i.level === 'w' ? s.fgW : s.fgB, i.level === 'e' ? s.issueE : i.level === 'w' ? s.issueW : s.issueI)}>{title}</span><span {...stylex.props(s.meta)}>{i.ref}</span></span>
        <span {...stylex.props(s.issueText)}>{i.text}</span>
        {i.fix && <span {...stylex.props(s.issueFix)}><span {...stylex.props(s.fg2)}>Gợi ý:</span> {i.fix}</span>}
      </div> })}</div>
    case 'cast': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>Nhân vật</span><span {...stylex.props(s.chips)}>{block.names.map((n, i) => <span key={i} title={n} {...stylex.props(s.cast)}><i aria-hidden="true" {...stylex.props(s.avatar)}>{initials(n)}</i><span {...stylex.props(s.truncate)}>{n}</span></span>)}</span></div>
    case 'tags': return <div {...stylex.props(s.column)}><span {...stylex.props(s.label)}>{block.label}</span>{block.items.map(([a, t, tone], i) => <div key={i} {...stylex.props(s.tag)}><span {...stylex.props(s.chip, tone && toneBg[tone], tone && toneFg[tone], s.noShrink)}>{a}</span><span {...stylex.props(s.tagText)}>{t}</span></div>)}</div>
    case 'scores': return <div {...stylex.props(s.scores, mobile && s.scoresMobile)}>{block.items.map(([k, v], i) => <span key={i} title={k} {...stylex.props(s.score)}>{k}<strong {...stylex.props(s.scoreValue)}>{v}</strong></span>)}</div>
    case 'ol': return <ol {...stylex.props(s.ordered, mobile && s.orderedMobile)}>{block.items.map((t, i) => <li key={i} {...stylex.props(s.orderedItem)}><span {...stylex.props(s.orderedNumber)}>{i + 1}</span><span title={t} {...stylex.props(s.truncate)}>{t}</span></li>)}</ol>
    case 'note': return <p {...stylex.props(s.note)}>{block.text}</p>
    case 'link': {
      if (!project || !story) return null
      let href: string
      if (block.href === 'premise') href = `${paths.story(project, story)}?tab=premise`
      else {
        if (!block.chapter) return null
        href = paths.reader(project, story, block.chapter)
      }
      return <a href={`#${href}`} {...stylex.props(s.link)}>{block.text}</a>
    }
  }
}

export function RunOperationBody({ operation, mobile = false }: { operation: LiveHistoryOperation; mobile?: boolean }) {
  const [techOpen, setTechOpen] = useState(false)
  const { route } = useRoute()
  const card = runCardModel(operation.key, operation.params, operation.result)
  const story = 'story' in route ? route.story ?? '' : ''
  const project = 'project' in route ? route.project ?? '' : ''
  return <div {...stylex.props(s.body, mobile && s.bodyMobile)}>
    {operation.status === 'running' ? <p {...stylex.props(s.note)}>Bắt đầu lúc {clock(operation.startedAt)}. Thẻ tự cập nhật khi xong; không cần tải lại.</p>
      : operation.status === 'error' ? <p role="alert" {...stylex.props(s.errorBody)}>{operation.error}</p>
      : card.blocks.map((block, i) => <div key={i} data-run-block={block.kind}><Block block={block} project={project} story={story} mobile={mobile} /></div>)}
    <details onToggle={event => setTechOpen(event.currentTarget.open)} {...stylex.props(s.technical)}>
      <summary {...stylex.props(s.technicalHead)}>Chi tiết kỹ thuật</summary>
      {techOpen && <pre {...stylex.props(s.technicalCode)}>{`op: ${operation.key}\nargs: ${operation.params}\nresult: ${operation.result}${operation.error ? `\nerror: ${operation.error}` : ''}`}</pre>}
    </details>
  </div>
}

/** Đồng hồ của thẻ đang chạy: đếm từ lúc thao tác bắt đầu, thay chỗ thời lượng (Gói AS mục 6). */
function RunningClock({ startedAt }: { startedAt: string }) {
  const now = useNow(1000)
  const started = Date.parse(startedAt)
  const elapsed = Number.isFinite(started) ? Math.max(0, now - started) : 0
  return <span role="timer" aria-label={`Đã chạy ${Math.floor(elapsed / 1000)} giây`} {...stylex.props(lv.duration)}>{formatTimer(elapsed)}</span>
}

function Operation({ operation, open, onToggle, mobile = false, onSelect }: {
  operation: LiveHistoryOperation
  open?: boolean
  onToggle?: () => void
  mobile?: boolean
  onSelect?: (operation: LiveHistoryOperation) => void
}) {
  const card = runCardModel(operation.key, operation.params, operation.result)
  const summary = operation.status === 'running' ? `đang ${card.name.toLocaleLowerCase('vi')}…` : operation.status === 'error' ? `không thực hiện được · ${card.summary}` : card.summary
  const duration = operation.status === 'running' ? '' : formatRunDuration(operation.durationMs)
  if (mobile) {
    const content = <><Icon name={icons[operation.key] ?? 'layer'} size={16} xstyle={s.opIcon} /><span {...stylex.props(s.opCopy, s.opCopyMobile)}><strong {...stylex.props(s.opName)}>{card.name}</strong><span title={summary} {...stylex.props(s.opSummary, operation.status === 'error' && s.errorText)}>{summary}</span></span>{duration !== '' && <span {...stylex.props(s.opDuration)}>{duration}</span>}<Icon name={operation.status === 'error' ? 'warning-2' : operation.status === 'running' ? 'clock' : 'tick-circle-bulk'} size={15} /></>
    return <button type="button" {...stylex.props(s.button, s.opRow, s.opRowMobile)} onClick={() => onSelect?.(operation)}>{content}</button>
  }
  return <div data-run-card={operation.id} data-status={operation.status}
    {...stylex.props(lv.card, operation.status === 'done' && !open && lv.cardHover, open && lv.cardOpen, operation.status === 'error' && lv.cardError, operation.status === 'running' && lv.cardLive)}>
    <button type="button" aria-expanded={open} onClick={onToggle} {...stylex.props(lv.cardHead)}>
      <Icon name={icons[operation.key] ?? 'layer'} size={15}
        style={{ color: operation.status === 'running' ? 'var(--kit-badge-run)' : operation.status === 'error' ? 'var(--color-err)' : 'var(--color-user-fg)' }} />
      {operation.status === 'running'
        ? <Shimmer style={{ flexShrink: 0, fontSize: 13, fontWeight: 500 }}>{card.name}</Shimmer>
        : <span {...stylex.props(lv.name)}>{card.name}</span>}
      <span title={summary} {...stylex.props(lv.summary, operation.status === 'error' && lv.summaryError)}>{summary}</span>
      {operation.status === 'running' ? <RunningClock startedAt={operation.startedAt} /> : duration !== '' && <span {...stylex.props(lv.duration)}>{duration}</span>}
      {operation.status === 'running' && <span role="img" aria-label="Đang chạy"><Spinner size={13} color="var(--kit-badge-run)" /></span>}
      {operation.status === 'done' && <Icon name="tick-b" label="Xong" size={14} style={{ color: 'var(--color-ok)' }} />}
      {operation.status === 'error' && <Icon name="warning-2" label="Lỗi" size={14} style={{ color: 'var(--color-err)' }} />}
      <Icon name="arrow-down-2" size={12} style={{ color: 'var(--kit-fg-faint)', transform: open ? 'rotate(180deg)' : 'none', transition: 'transform .15s' }} />
    </button>
    {open && <RunOperationBody operation={operation} />}
  </div>
}

/**
 * Một vòng trong khung luồng viết (Main.dc.html › lvr): dòng 30 px — orb 11 px cùng hàng với
 * “Vòng N”, tên vai, tên việc, đường kẻ, giờ, mũi tên; thẻ thụt 22 px và nối bằng đường dọc.
 * Vòng cũ hơn hai vòng gần nhất thu gọn còn “· n thao tác · tổng thời gian” + dấu xong.
 */
function Round({ round, open, first, onToggle }: {
  round: LiveHistoryRound
  open: boolean
  first: boolean
  onToggle: () => void
}) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  // Thẻ đang chạy xuống cuối vòng để dính đáy khung, không chen giữa các thẻ đã xong.
  const operations = [...round.operations].sort((a, b) => Number(a.status === 'running') - Number(b.status === 'running'))
  const settled = operations.every(operation => operation.status === 'done')
  const total = operations.reduce((sum, operation) => operation.status === 'done' ? sum + Math.max(0, operation.durationMs) : sum, 0)
  const old = [`${operations.length} thao tác`, formatRunDuration(total)].filter(Boolean).join(' · ')
  const label = roles[round.role] ?? round.role
  return <div role="group" aria-label={`Vòng ${round.round} · ${label}`} data-round={round.round}
    {...stylex.props(lv.roundRow, !first && lv.roundRowNext)}>
    <button type="button" aria-expanded={open} onClick={onToggle} {...stylex.props(lv.roundHead)}>
      <AgentOrb agent={agentKind(round.role)} size={11} />
      <b {...stylex.props(lv.roundNumber)}>Vòng {round.round}</b>
      <span style={{ color: roleFg[round.role] ?? 'var(--color-fg-3)', whiteSpace: 'nowrap' }}>{label}</span>
      {round.title && <span {...stylex.props(lv.roundTitle)}>· {round.title}{!open && <span {...stylex.props(lv.roundOld)}> · {old}</span>}</span>}
      <span aria-hidden="true" {...stylex.props(lv.roundRule)} />
      {!open && settled && <Icon name="tick-b" label="Xong" size={13} style={{ color: 'var(--color-ok)' }} />}
      <span {...stylex.props(lv.roundTime)}>{clock(round.startedAt)}</span>
      <Icon name="arrow-down-2" size={12}
        style={{ color: 'var(--kit-fg-faint)', transform: open ? 'rotate(180deg)' : 'rotate(-90deg)', transition: 'transform .15s' }} />
    </button>
    {open && <div {...stylex.props(lv.ops)}>{operations.map(operation => <Operation key={operation.id} operation={operation}
      open={!!expanded[operation.id]} onToggle={() => setExpanded(old => ({ ...old, [operation.id]: !old[operation.id] }))} />)}</div>}
  </div>
}

export function RunHistory({ project, story, mobile = false, embedded = false, empty }: { project: string; story: string; mobile?: boolean; embedded?: boolean; empty?: ReactNode }) {
  const [rounds, setRounds] = useState<LiveHistoryRound[]>([])
  const [cursor, setCursor] = useState<number | null>(null)
  const [loading, setLoading] = useState(true), [olderLoading, setOlderLoading] = useState(false), [error, setError] = useState(false)
  const [role, setRole] = useState('all'), [selected, setSelected] = useState<LiveHistoryOperation | null>(null), [away, setAway] = useState(false)
  /** Vòng người dùng đã bấm thu/mở; chưa có trong map thì hai vòng gần nhất mở sẵn. */
  const [open, setOpen] = useState<Record<number, boolean>>({})
  const scroll = useRef<HTMLDivElement>(null)
  const loaded = useRef(false)
  const positioned = useRef(false)
  const live = useStoryEvents(project, story, noop)
  const refresh = useCallback(async (signal?: AbortSignal) => {
    try {
      const page = await getLiveHistory(project, story, { limit: 20 }, signal)
      if (signal?.aborted) return
      setRounds(old => mergeRunHistory(old, page.rounds))
      if (!loaded.current) { setCursor(page.nextBeforeRound); loaded.current = true }
      setError(false)
    } catch { if (!signal?.aborted) setError(true) } finally { if (!signal?.aborted) setLoading(false) }
  }, [project, story])
  useEffect(() => { const controller = new AbortController(); loaded.current = false; positioned.current = false; setRounds([]); setCursor(null); setLoading(true); void refresh(controller.signal); return () => controller.abort() }, [refresh])
  useEffect(() => { if (!loading && live.events.length) setRounds(old => mergeRunEvents(old, live.events)) }, [live.events, loading])
  useEffect(() => { if (!positioned.current && rounds.length && scroll.current) { scroll.current.scrollTop = scroll.current.scrollHeight; positioned.current = true } }, [rounds])
  const older = async () => { if (cursor === null || olderLoading) return; setOlderLoading(true); try { const page = await getLiveHistory(project, story, { beforeRound: cursor, limit: 20 }); setRounds(old => mergeRunHistory(old, page.rounds)); setCursor(page.nextBeforeRound); setError(false) } catch { setError(true) } finally { setOlderLoading(false) } }
  const current = selected && rounds.flatMap(r => r.operations).find(o => o.id === selected.id) || selected
  const running = rounds.some(r => r.operations.some(o => o.status === 'running'))
  const page = !embedded && !mobile
  const shown = rounds.filter(round => role === 'all' || round.role === role)
  const content = <div {...stylex.props(s.root, page && lv.pageRoot)}>
    {!mobile && <div {...stylex.props(lv.bar)}>
      <div role="group" aria-label="Lọc theo trợ lý" {...stylex.props(lv.seg)}>
        {filters.map(([key, label]) => <button key={key} type="button" aria-pressed={role === key} onClick={() => setRole(key)}
          {...stylex.props(lv.segButton, role === key && lv.segSelected)}>
          {key !== 'all' && <i aria-hidden="true" style={{ background: roleFg[key] }} {...stylex.props(lv.dot)} />}
          {label}
        </button>)}
      </div>
      <span {...stylex.props(lv.grow)} />
      <span {...stylex.props(lv.count)}>{rounds.length} vòng</span>
      {embedded && <a href={`#${paths.workspace(project, story)}?view=run`} title="Mọi vòng của truyện, lưu lại toàn bộ"
        {...stylex.props(lv.history)}>Lịch sử<Icon name="arrow-right-2" size={13} /></a>}
    </div>}
    {mobile && <div {...stylex.props(s.filters)}>{filters.map(([key, label]) => <button key={key} type="button" aria-pressed={role === key} onClick={() => setRole(key)} {...stylex.props(s.button, s.filter, role === key && s.filterSelected)}>{label}</button>)}</div>}
    <div ref={scroll} onScroll={event => { const e = event.currentTarget; setAway(e.scrollHeight - e.scrollTop - e.clientHeight > 100) }}
      {...stylex.props(page ? lv.pageScroll : s.scroll, mobile && s.scrollMobile)}>
      {cursor !== null && <button type="button" disabled={olderLoading} onClick={() => void older()} {...stylex.props(mobile ? s.button : lv.more)}>
        {!mobile && <Icon name="arrow-up" size={12} />}{olderLoading ? 'Đang tải…' : 'Tải thêm vòng cũ hơn'}</button>}
      {error && <div role="alert">Không tải được lịch sử. <button type="button" {...stylex.props(s.button)} onClick={() => void refresh()}>Thử lại</button></div>}
      {loading && <p>Đang tải lịch sử…</p>}
      {!loading && rounds.length === 0 && (empty ?? <p>Chưa có lịch sử luồng viết.</p>)}
      {!loading && rounds.length > 0 && shown.length === 0 && <div {...stylex.props(lv.empty)}>
        <AgentOrb agent={agentKind(role)} size={20} style={{ opacity: 0.6 }} />
        <span {...stylex.props(lv.emptyTitle)}>Chưa có vòng nào của {roles[role] ?? role}</span>
        <span {...stylex.props(lv.emptyHint)}>Trong 20 vòng gần nhất. Bấm Tải thêm vòng cũ hơn hoặc chọn Tất cả.</span>
      </div>}
      {shown.map(round => mobile
        ? <section key={round.round} {...stylex.props(s.round)}><h3 {...stylex.props(s.roundTitle)}><span className={`m-orb ${round.role}`} /> {['Vòng ' + round.round, roles[round.role], round.title, clock(round.startedAt)].filter(Boolean).join(' · ')}</h3><div {...stylex.props(s.roundCards, s.roundCardsMobile)}>{[...round.operations].sort((a, b) => Number(a.status === 'running') - Number(b.status === 'running')).map(operation => <Operation key={operation.id} operation={operation} mobile onSelect={setSelected} />)}</div></section>
        : <Round key={round.round} round={round} first={round === shown[0]} onToggle={() => setOpen(old => ({ ...old, [round.round]: !(open[round.round] ?? rounds.indexOf(round) >= rounds.length - 2) }))}
          open={open[round.round] ?? rounds.indexOf(round) >= rounds.length - 2} />)}
    </div>
    {mobile && running && away && <button type="button" onClick={() => scroll.current?.scrollTo({ top: scroll.current.scrollHeight, behavior: 'smooth' })} {...stylex.props(s.button, s.floating)}>Đang chạy · về mới nhất</button>}
    {mobile && <BottomSheet open={!!current} onClose={() => setSelected(null)} height="auto" xstyle={s.sheet} title={current ? runCardModel(current.key, current.params, current.result).name : ''}>{current && <div {...stylex.props(s.sheetContent)}><div {...stylex.props(s.sheetMeta)}>{[roles[current.role], `vòng ${current.round}`, clock(current.startedAt), current.status === 'error' ? 'lỗi' : current.status === 'running' ? 'Đang chạy' : formatRunDuration(current.durationMs)].filter(Boolean).join(' · ')}</div><p {...stylex.props(s.sheetSummary, current.status === 'error' && s.errorText)}>{current.status === 'error' ? 'không thực hiện được · ' : ''}{runCardModel(current.key, current.params, current.result).summary}</p><RunOperationBody operation={current} mobile /></div>}</BottomSheet>}
  </div>
  return embedded ? content : mobile ? <MobilePage title="Luồng viết" onBack={() => navigate(paths.workspace(project, story))} manageScroll={false}>{content}</MobilePage> : <section className={`card ${stylex.props(s.pagePanel).className}`}>{content}</section>
}
