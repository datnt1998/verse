// IdleCard — thẻ ở giữa khung luồng khi không có trợ lý nào chạy (thay ô
// “Chưa có trợ lý nào đang viết”). Gói P · P4, Main.dc.html › phase=wait|ready|done.
// Nội dung lấy từ snapshot: `pendingAsk`/`hasAdvanceHold` (wait), số chương + chữ
// (done), hoặc kế hoạch đã lưu (ready). Mọi nút dùng lại hành động có sẵn.
import * as stylex from '@stylexjs/stylex'
import type { PendingAsk, Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { formatNumber } from '../../lib/format'
import { Icon } from '@noverse/ui'
import { isCompleted, isGate } from './GateCard'
import { s } from './idleCard.stylex'
import { StreamMarkdown } from '../../lib/StreamMarkdown'
import { humanizeFreeText } from '../../lib/labels'

export type IdlePhase = 'wait' | 'ready' | 'done'

/** Dữ liệu thẻ; `wait` ưu tiên câu hỏi thật của Trọng tài, rơi về lý do tạm dừng. */
export type IdleModel = {
  phase: IdlePhase
  question: string
  options: { label: string; reply: string; isDefault: boolean }[]
  chapter: number
}

export type IdleActions = {
  /** “Bắt đầu viết chương 1” — hành động bắt đầu/duyệt chương có sẵn. */
  onStart: () => void
  /** “Xem kế hoạch” — mở tab Dàn ý của truyện. */
  onPlan: () => void
  /** “Não truyện”. */
  onBrain: () => void
  onRead: (chapter?: number) => void
  onAudio: () => void
  onExport: () => void
  /** Trả lời Trọng tài — đi đúng đường ô nhập (đã trim, không rỗng). */
  onReply: (text: string) => void
}

/**
 * Trạng thái thẻ theo snapshot. Trả `null` khi khung luồng nên giữ nguyên:
 * đang chạy, đang lập kế hoạch, cổng thẩm duyệt, hoặc tạm dừng không có câu hỏi.
 */
export function idleModel(snapshot: Snapshot | null): IdleModel | null {
  if (!snapshot || snapshot.isRunning) return null
  if (isCompleted(snapshot)) {
    return { phase: 'done', question: '', options: [], chapter: 0 }
  }
  // Cổng thẩm duyệt giữ khung riêng (GateCard), không thay bằng thẻ.
  if (isGate(snapshot)) return null
  const ask = snapshot.pendingAsk ?? null
  if (ask) return askModel(ask)
  if (snapshot.hasAdvanceHold) {
    return {
      phase: 'wait',
      question: translateHost(snapshot.advanceHoldReason.trim()),
      options: [],
      chapter: snapshot.advanceHoldChapter ?? 0,
    }
  }
  // Tạm dừng dùng không thẻ: giữ luồng viết đang dở, nút Tiếp tục ở đầu màn.
  if (snapshot.runtimeState === 'paused' || snapshot.runtimeState === 'pausing') return null
  if (snapshot.plan) return null
  const outlined = snapshot.outline?.length ?? 0
  const planned = snapshot.phase === 'outline' || snapshot.phase === 'writing'
  if (planned && snapshot.completedCount === 0 && (outlined > 0 || snapshot.totalChapters > 0)) {
    return { phase: 'ready', question: '', options: [], chapter: 0 }
  }
  return null
}

function askModel(ask: PendingAsk): IdleModel {
  const options = (ask.options ?? []).map((option) => ({
    label: option.label,
    reply: (option.reply ?? '').trim() || option.label,
    isDefault: !!option.isDefault,
  }))
  // Không có phương án mặc định: nút đầu là nút chính (đọc theo thứ tự Trọng tài đưa).
  if (options.length > 0 && !options.some((option) => option.isDefault)) options[0]!.isDefault = true
  return { phase: 'wait', question: ask.question, options, chapter: ask.chapter ?? 0 }
}

const PHASE_ICON: Record<IdlePhase, string> = { wait: 'messages-2', ready: 'play', done: 'tick-circle-bulk' }
const CARD_STYLE = { wait: s.wait, ready: s.ready, done: s.done } as const
const GLYPH_STYLE = { wait: s.glyphWait, ready: s.glyphReady, done: s.glyphDone } as const

function Action({ label, icon, primary, onClick, disabled }: {
  label: string
  icon: string
  primary?: boolean
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button type="button" onClick={onClick} disabled={disabled} title={humanizeFreeText(label)}
      className={stylex.props(s.action, primary ? s.primary : s.ghost).className}
      style={{ opacity: disabled ? .5 : 1, color: primary ? 'var(--color-bg)' : undefined }}>
      <Icon name={icon} size={15} />
      <span className={stylex.props(s.actionLabel).className}>{humanizeFreeText(label)}</span>
    </button>
  )
}
function IdleCardBody({ phase, title, text, hint, children, mobile }: {
  phase: IdlePhase; title: string; text: string; hint: string; children: React.ReactNode; mobile: boolean
}) {
  return (
    <div role="status" className={stylex.props(s.card, CARD_STYLE[phase]).className}>
      <span className={stylex.props(s.glyph, GLYPH_STYLE[phase]).className}><Icon name={PHASE_ICON[phase]} size={18} /></span>
      <div className={stylex.props(s.body).className}>
        <span className={stylex.props(s.title).className}>{title}</span>
        <div className={stylex.props(s.text).className}><StreamMarkdown text={text} /></div>
        <div className={stylex.props(s.actions).className}>{children}</div>
        {!mobile && <span className={stylex.props(s.hint).className}>{hint}</span>}
      </div>
    </div>
  )
}

/** The existing wait card, shared with mobile rather than a second ask UI. */
export function PendingQuestionCard({ model, snapshot, onReply, onRead, mobile = false, disabled = false }: {
  model: IdleModel; snapshot: Snapshot; onReply: (text: string) => void; onRead: (chapter?: number) => void; mobile?: boolean; disabled?: boolean
}) {
  const readChapter = model.chapter > 0 ? model.chapter : snapshot.completedCount
  return (
    <IdleCardBody phase="wait" title={t.idle.waitTitle} text={model.question || t.idle.waitDefaultHint} hint={t.idle.waitHint} mobile={mobile}>
      {model.options.map((option) => (
        <Action key={option.label} label={option.label} icon={option.isDefault ? 'tick-circle-bulk' : 'edit-2'}
          primary={option.isDefault} disabled={disabled} onClick={() => onReply(option.reply)} />
      ))}
      <Action label={t.idle.readChapter(readChapter)} icon="book-1" onClick={() => onRead(readChapter || undefined)} />
    </IdleCardBody>
  )
}


export function IdleCard({ model, snapshot, actions, mobile = false }: {
  model: IdleModel
  snapshot: Snapshot
  actions: IdleActions
  mobile?: boolean
}) {
  const { phase } = model
  const titled = (title: string, text: string, hint: string, buttons: React.ReactNode) => (
    <IdleCardBody phase={phase} title={title} text={text} hint={hint} mobile={mobile}>{buttons}</IdleCardBody>
  )

  if (phase === 'wait') {
    return <PendingQuestionCard model={model} snapshot={snapshot} onReply={actions.onReply} onRead={actions.onRead} mobile={mobile} />
  }

  if (phase === 'ready') {
    const characters = snapshot.characters?.length ?? 0
    const chapters = snapshot.outline?.length || snapshot.totalChapters
    return titled(
      t.idle.readyTitle,
      t.idle.readyText(characters, chapters),
      t.idle.readyHint,
      <>
        <Action label={t.idle.readyStart} icon="play" primary onClick={actions.onStart} />
        <Action label={t.idle.readyPlan} icon="note-text" onClick={actions.onPlan} />
        <Action label={t.idle.readyBrain} icon="routing-2" onClick={actions.onBrain} />
      </>,
    )
  }

  return titled(
    t.idle.doneTitle(snapshot.completedCount, formatNumber(snapshot.totalWordCount)),
    t.idle.doneText,
    t.idle.doneHint,
    <>
      <Action label={t.idle.doneRead} icon="book-1" primary onClick={() => actions.onRead(1)} />
      <Action label={t.idle.doneAudio} icon="headphone" onClick={actions.onAudio} />
      <Action label={t.idle.doneExport} icon="export-curve" onClick={actions.onExport} />
    </>,
  )
}

/** Dòng phụ đề của tiêu đề khung theo trạng thái (artboard `streamMeta`). */
export const IDLE_SUBTITLE: Record<IdlePhase, string> = {
  wait: t.idle.subtitleWait, ready: t.idle.subtitleReady, done: t.idle.subtitleDone,
}
