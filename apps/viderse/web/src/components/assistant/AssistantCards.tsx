// Các thẻ kết quả của một câu nhắn: Kế hoạch (nhiều bước, bỏ tích từng bước),
// thẻ xác nhận một việc (Dialogs assistant-confirm), hỏi lại có lựa chọn, "Chưa
// làm được việc này" (assistant-unknown), đang đọc câu và lỗi (mẫu V12).
import type { KeyboardEvent } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { ActionButton } from '../ActionButton'
import { Orb } from '../Orb'
import { assistantName } from '../agents'
import { navigate } from '../../router'
import { assistantCopy as copy } from '../../i18n/assistant'
import type { AssistantAgent, AssistantErrorKind, Plan, PlanAsk, PlanStep, PlanUnknown, StepState } from '../../api/assistant'
import { planTotals, stepCostLabel, totalLine, type Selection, type StepProgress } from './planModel'
import { systemErrorText } from './systemCopy'
import { assistantStyles as styles } from './assistant.stylex'

export const Bubble = ({ text }: { text: string }) => <div {...stylex.props(styles.bubble)}>{text}</div>

const stateLabel: Record<StepState, string> = { queued: copy.progress.queued, running: copy.progress.running, done: copy.progress.done, failed: copy.progress.failed }

type PlanCardProps = {
  plan: Plan
  text: string
  selected: Selection
  /** `review`: tích/bỏ bước; `running`/`done`: khoá, hiện trạng thái từng bước. */
  mode: 'review' | 'running' | 'done'
  progress?: StepProgress
  lost?: boolean
  online: boolean
  onToggle: (id: string) => void
  onEdit: () => void
  onCancel: () => void
  onRun: () => void
  onClose: () => void
}

export function PlanCard({ plan, text, selected, mode, progress = {}, lost, online, onToggle, onEdit, onCancel, onRun, onClose }: PlanCardProps) {
  const totals = planTotals(plan, selected)
  const review = mode === 'review'
  const steps = review ? plan.steps : plan.steps.filter((step) => selected.has(step.id))
  const failed = steps.filter((step) => progress[step.id]?.state === 'failed').length
  const done = steps.filter((step) => progress[step.id]?.state === 'done').length
  const keyToggle = (id: string) => (event: KeyboardEvent) => {
    if (event.key !== ' ' && event.key !== 'Enter') return
    event.preventDefault()
    onToggle(id)
  }
  return <div role="group" aria-label={copy.plan.label} aria-busy={mode === 'running' || undefined} data-enqueues-video={totals.enqueuesVideo || undefined} {...stylex.props(styles.plan)}>
    <div {...stylex.props(styles.planHead)}><b {...stylex.props(styles.planTitle)}>{copy.plan.head(review ? totals.count : steps.length)}</b><span {...stylex.props(styles.planQuote)}>{copy.plan.quote(text)}</span></div>
    {steps.map((step) => {
      const on = selected.has(step.id)
      const run = progress[step.id]
      return <div key={step.id} role="checkbox" aria-checked={review ? on : true} aria-disabled={!review || undefined} tabIndex={review ? 0 : -1}
        onClick={review ? () => onToggle(step.id) : undefined} onKeyDown={review ? keyToggle(step.id) : undefined}
        {...stylex.props(styles.step, !review && styles.stepLocked)}>
        <span aria-hidden="true" {...stylex.props(styles.check, (review ? on : true) && styles.checkOn)}>{(review ? on : true) && <span {...stylex.props(styles.tick)} />}</span>
        <Orb of={step.agent} size={16} />
        <span {...stylex.props(styles.stepText)}>
          <b {...stylex.props(styles.stepTitle, review && !on && styles.stepOff)}>{step.title}</b>
          <small {...stylex.props(styles.stepSub, review && !on && styles.stepOff)}>{run?.message || step.sub}</small>
        </span>
        {review || !run
          ? <span {...stylex.props(styles.cost)}>{stepCostLabel(step)}</span>
          : <span role="status" {...stylex.props(styles.state, run.state === 'done' && styles.stateDone, run.state === 'failed' && styles.stateFailed)}>{stateLabel[run.state]}</span>}
      </div>
    })}
    <div {...stylex.props(styles.foot)}>
      {review ? <>
        <span {...stylex.props(styles.footTotal)}>{totalLine(plan, selected)}</span>
        <ActionButton variant="quiet" size="sm" type="button" onClick={onEdit}>{copy.plan.editText}</ActionButton>
        <ActionButton variant="quiet" size="sm" type="button" onClick={onCancel}>{copy.plan.cancel}</ActionButton>
        <ActionButton variant="primary" size="sm" type="button" data-enqueues-video={totals.enqueuesVideo || undefined}
          disabled={totals.count === 0 || !online} disabledReason={!online ? copy.states.offlineSend : copy.plan.noStep} onClick={onRun}><span>{copy.plan.run(totals.count)}</span></ActionButton>
      </> : <>
        <span role="status" aria-live="polite" aria-atomic="true" {...stylex.props(styles.footTotal, failed > 0 && styles.stateFailed)}>
          {mode === 'running' ? (lost ? copy.states.lostProgress : copy.states.running) : failed > 0 ? copy.progress.failedSummary(failed) : copy.progress.summary(done, steps.length)}
        </span>
        {mode === 'done' && <ActionButton variant="quiet" size="sm" type="button" onClick={onClose}>{copy.progress.close}</ActionButton>}
      </>}
    </div>
  </div>
}

const rerunLabel = (agent: AssistantAgent, label: string) => label ? `${assistantName(agent)} · ${label}` : assistantName(agent)

type ConfirmCardProps = { step: PlanStep; imagesLeftToday?: number; online: boolean; onCancel: () => void; onRun: () => void }

/** Một việc trên một video: thẻ vàng nhạt có chuỗi "Làm lại" các trợ lý và nút Làm (Enter). */
export function ConfirmCard({ step, imagesLeftToday, online, onCancel, onRun }: ConfirmCardProps) {
  const images = step.cost?.images ?? 0
  return <div role="group" aria-label={copy.confirm.label(assistantName(step.agent))} data-enqueues-video={step.enqueuesVideo || undefined} {...stylex.props(styles.card, styles.cardConfirm)}>
    <div {...stylex.props(styles.cardHead)}>
      <Orb of={step.agent} size={22} />
      <span {...stylex.props(styles.cardTitle)}>{step.title}</span>
      {images > 0 && <span {...stylex.props(styles.chip, styles.chipStatic)}><Icon name="gallery" size={13} xstyle={styles.chipIcon} /><b {...stylex.props(styles.chipValue)}>{copy.confirm.image(images)}</b>{imagesLeftToday !== undefined && ` · ${copy.plan.imagesLeft(imagesLeftToday)}`}</span>}
    </div>
    {step.detail && <p {...stylex.props(styles.detail)}>{step.detail}</p>}
    <div {...stylex.props(styles.reruns)}>
      {step.reruns.length > 0 && <span {...stylex.props(styles.rerunLabel)}>{copy.confirm.rerun}</span>}
      {step.reruns.map((rerun, index) => <span key={`${rerun.agent}-${index}`} {...stylex.props(styles.contents)}>
        {index > 0 && <Icon name="arrow-r" size={13} xstyle={styles.chipIcon} />}
        <span {...stylex.props(styles.chip, styles.chipStatic)}><Orb of={rerun.agent} size={12} />{rerunLabel(rerun.agent, rerun.label)}</span>
      </span>)}
      <span {...stylex.props(styles.grow)} />
      <ActionButton variant="quiet" size="sm" type="button" onClick={onCancel}>{copy.confirm.cancel}</ActionButton>
      <ActionButton variant="primary" size="sm" type="button" kbd={copy.confirm.kbd} disabled={!online} disabledReason={copy.states.offlineSend} onClick={onRun}>{copy.confirm.run}</ActionButton>
    </div>
  </div>
}

/** Thiếu thông tin: hỏi một câu, mỗi lựa chọn là một chip (Kit › Trợ lý). Bấm chỉ lập kế hoạch mới. */
export function AskCard({ text, ask, online, onChoose }: { text: string; ask: PlanAsk; online: boolean; onChoose: (text: string) => void }) {
  return <div role="group" aria-label={copy.ask.head} {...stylex.props(styles.plan)}>
    <div {...stylex.props(styles.planHead)}><b {...stylex.props(styles.planTitle)}>{copy.ask.head}</b><span {...stylex.props(styles.planQuote)}>{copy.plan.quote(text)}</span></div>
    <div {...stylex.props(styles.askQuestion)}>{ask.question}</div>
    <div {...stylex.props(styles.ask)}>
      {ask.options.map((option) => <button key={option.text} type="button" disabled={!online} title={online ? undefined : copy.states.offlineSend} {...stylex.props(styles.chip)} onClick={() => onChoose(option.text)}>{option.label}</button>)}
    </div>
  </div>
}

/** Không chuyển được thành thao tác nào (assistant-unknown): lời nói rõ vì sao + 2 việc làm được, mỗi gợi ý chỉ lập kế hoạch mới. */
export function UnknownCard({ unknown, online, onChoose }: { unknown: PlanUnknown; online: boolean; onChoose: (text: string) => void }) {
  return <div role="status" {...stylex.props(styles.card, styles.cardPlain)}>
    <div {...stylex.props(styles.cardHead)}><Orb of="artist" size={22} dim /><span {...stylex.props(styles.cardTitle)}>{copy.unknown.title}</span></div>
    <p {...stylex.props(styles.lead)}>{unknown.lead}</p>
    <div {...stylex.props(styles.suggestions)}>
      {unknown.suggestions.map((choice) => <ActionButton key={choice.text} variant="outline" size="sm" type="button" disabled={!online} disabledReason={copy.states.offlineSend}
        icon={choice.icon ? <Icon name={choice.icon} size={14} /> : undefined} onClick={() => onChoose(choice.text)}>{choice.label}</ActionButton>)}
      {unknown.link && <ActionButton variant="quiet" size="sm" type="button" onClick={() => navigate(unknown.link!.href)}>{unknown.link.label}<Icon name="arrow-r" size={13} /></ActionButton>}
    </div>
  </div>
}

export function PlanningCard() {
  return <div role="status" aria-live="polite" aria-atomic="true" {...stylex.props(styles.card, styles.cardPlain)}>
    <div {...stylex.props(styles.cardHead)}>
      <Orb of="director" size={22} live />
      <span {...stylex.props(styles.note, styles.grow)}>{copy.states.planningWho}</span>
      <span aria-hidden="true" {...stylex.props(styles.typing)}>
        <span {...stylex.props(styles.typingDot)} />
        <span {...stylex.props(styles.typingDot, styles.typingDot2)} />
        <span {...stylex.props(styles.typingDot, styles.typingDot3)} />
      </span>
    </div>
  </div>
}

type ErrorCardProps = { kind: AssistantErrorKind; message: string; running?: boolean; onRetry?: () => void; onEdit: () => void; onClose: () => void }

/** Lỗi thật của máy chủ/mất kết nối: nói đúng chuyện đã xảy ra, và nhấn mạnh chưa làm gì. */
export function ErrorCard({ kind, message, running, onRetry, onEdit, onClose }: ErrorCardProps) {
  const text = systemErrorText(kind, message, running)
  const retryable = kind === 'offline' || kind === 'busy' || kind === 'failed' || kind === 'model'
  return <div role="alert" {...stylex.props(styles.card, styles.cardPlain)}>
    <div {...stylex.props(styles.cardHead)}>
      <Orb of="director" size={22} dim />
      <span {...stylex.props(styles.cardTitle)}>{copy.states.errorTitle}</span>
    </div>
    <span {...stylex.props(styles.note, styles.noteError)}>{text}</span>
    <div {...stylex.props(styles.reruns)}>
      <span {...stylex.props(styles.grow)} />
      <ActionButton variant="quiet" size="sm" type="button" onClick={onClose}>{copy.progress.close}</ActionButton>
      {kind !== 'expired' && kind !== 'notFound' && <ActionButton variant="quiet" size="sm" type="button" onClick={onEdit}>{copy.states.edit}</ActionButton>}
      {/* Chỉ lập lại kế hoạch là endpoint an toàn để thử lại; lượt chạy đã bắt đầu thì máy chủ từ chối chạy lại bước. */}
      {retryable && onRetry && <ActionButton variant="outline" size="sm" type="button" onClick={onRetry}>{copy.states.retry}</ActionButton>}
    </div>
  </div>
}
