// Sheet Kế hoạch của trợ lý trên điện thoại (artboard MMain › plan). Cùng một máy
// trạng thái với ô Nhắn trợ lý ở máy tính (useAssistantThread): câu → đọc câu →
// kế hoạch chưa chạy gì → Làm → tiến độ thật từng bước. Không có phần trăm,
// không có bước tự báo xong, không gọi endpoint nào ngoài plan/run-plan/runs.
import { useEffect } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Checkbox, Sheet } from '@noverse/ui'
import type { AssistantScope } from '../../../api/assistant'
import { Orb } from '../../../components/Orb'
import { planTotals, stepCostLabel, totalLine, type Selection, type StepProgress } from '../../../components/assistant/planModel'
import { useAssistantThread } from '../../../components/assistant/useAssistantThread'
import { assistantCopy } from '../../../i18n/assistant'
import { systemErrorText } from '../../../components/assistant/systemCopy'
import { screenWorkbench } from '../../../i18n/screens'
import { mw } from './mworkbench.stylex'

const c = screenWorkbench.mobile.plan

export function MobilePlanSheet({ open, text, scope, onClose, onSettled }: {
  open: boolean
  text: string
  scope: AssistantScope
  onClose: () => void
  onSettled: () => void
}) {
  return open ? <OpenPlanSheet text={text} scope={scope} onClose={onClose} onSettled={onSettled} /> : null
}

function OpenPlanSheet({ text, scope, onClose, onSettled }: { text: string; scope: AssistantScope; onClose: () => void; onSettled: () => void }) {
  const { thread, send, toggle, confirm, cancel, retry } = useAssistantThread(scope, onSettled)
  // Mở sheet là gửi câu. Dọn effect huỷ lượt đọc câu còn treo (đóng sheet, đổi
  // phạm vi, StrictMode chạy lại effect); lượt chạy đã bắt đầu thì không huỷ.
  useEffect(() => {
    send(text)
    return () => cancel()
  }, [send, text, cancel])

  const running = thread.phase === 'running'
  const quote = 'text' in thread ? thread.text : text
  const plan = thread.phase === 'plan' || thread.phase === 'running' || thread.phase === 'done' ? thread.plan : null
  const selected: Selection | null = thread.phase === 'plan' || thread.phase === 'running' || thread.phase === 'done' ? thread.selected : null
  const progress: StepProgress = thread.phase === 'running' || thread.phase === 'done' ? thread.steps : {}
  const totals = plan && selected ? planTotals(plan, selected) : null
  const steps = plan && selected ? plan.steps.filter((step) => selected.has(step.id)) : []
  const done = steps.filter((step) => progress[step.id]?.state === 'done').length
  const failed = steps.filter((step) => progress[step.id]?.state === 'failed').length

  /** Đang chạy thì không đóng: máy chủ không có endpoint huỷ lượt chạy. */
  const closeSheet = () => { if (!running) onClose() }
  const cancelPlan = () => { cancel(); onClose() }

  const announce = thread.phase === 'planning' ? assistantCopy.a11y.planning
    : thread.phase === 'plan' ? assistantCopy.a11y.plan(thread.plan.steps.length)
      : thread.phase === 'ask' ? assistantCopy.a11y.ask
        : thread.phase === 'unknown' ? assistantCopy.a11y.unknown
          : thread.phase === 'error' ? assistantCopy.a11y.error
            : thread.phase === 'running' ? assistantCopy.a11y.running(done, steps.length)
              : thread.phase === 'done' ? (failed > 0 ? assistantCopy.a11y.failed(failed, steps.length) : assistantCopy.a11y.done(done, steps.length))
                : ''

  const heading = plan ? c.head(plan.steps.length) : c.title

  return <Sheet open onClose={closeSheet} label={c.title} size="md" height="auto" grab={!running}>
    <div {...stylex.props(mw.planBody)} role="group" aria-label={c.title}>
      <span role="status" aria-live="polite" aria-atomic="true" {...stylex.props(mw.srOnly)}>{announce}</span>

      <div {...stylex.props(mw.planHead)}>
        <h2 {...stylex.props(mw.planTitle)}>{heading}</h2>
        <p {...stylex.props(mw.planQuote)}>{c.quote(quote)}</p>
      </div>

      {thread.phase === 'planning' && <div role="status" aria-live="polite" {...stylex.props(mw.planAck)}>
        <Orb of="director" size={18} live />
        <span>{assistantCopy.states.planningWho}</span>
      </div>}

      {thread.phase === 'ask' && <div {...stylex.props(mw.planAsk)}>
        <strong {...stylex.props(mw.planStepTitle)}>{assistantCopy.ask.head}</strong>
        <p {...stylex.props(mw.planStepSub)}>{thread.ask.question}</p>
        {thread.ask.options.map((option) => <Button key={option.text} variant="ghost" block onClick={() => void send(option.text)}>{option.label}</Button>)}
        <Button variant="quiet" size="sm" onClick={closeSheet}>{c.editText}</Button>
      </div>}

      {thread.phase === 'unknown' && <div {...stylex.props(mw.planAsk)}>
        <strong {...stylex.props(mw.planStepTitle)}>{assistantCopy.unknown.title}</strong>
        <p {...stylex.props(mw.planStepSub)}>{thread.unknown.lead}</p>
        {thread.unknown.suggestions.map((option) => <Button key={option.text} variant="ghost" block onClick={() => void send(option.text)}>{option.label}</Button>)}
        {thread.unknown.link && <a {...stylex.props(mw.moreLink)} href={thread.unknown.link.href}>{thread.unknown.link.label} ›</a>}
        <Button variant="quiet" size="sm" onClick={closeSheet}>{c.editText}</Button>
      </div>}

      {thread.phase === 'error' && <div role="alert" {...stylex.props(mw.planAsk)}>
        <strong {...stylex.props(mw.planStepTitle)}>{assistantCopy.states.errorTitle}</strong>
        <p {...stylex.props(mw.planStepSub)}>{systemErrorText(thread.kind, thread.message, thread.retry.type === 'run')}</p>
        <div {...stylex.props(mw.planFooter)}>
          <Button variant="quiet" size="sm" onClick={closeSheet}>{assistantCopy.states.edit}</Button>
          {thread.retry.type === 'plan' && (thread.kind === 'offline' || thread.kind === 'busy' || thread.kind === 'failed' || thread.kind === 'model') && <Button variant="primary" size="sm" onClick={() => retry()}>{assistantCopy.states.retry}</Button>}
        </div>
      </div>}

      {plan && selected && <>
        {(thread.phase === 'plan' ? plan.steps : steps).map((step) => {
          const on = selected.has(step.id)
          const run = progress[step.id]
          return <div key={step.id} {...stylex.props(mw.planStep)}>
            <Checkbox checked={on} onChange={() => toggle(step.id)} disabled={thread.phase !== 'plan'} label={
              <span {...stylex.props(mw.planStepText)}>
                <span {...stylex.props(mw.planStepTitle, thread.phase === 'plan' && !on && mw.planStepOff)}>{step.title}</span>
                <span {...stylex.props(mw.planStepSub, thread.phase === 'plan' && !on && mw.planStepOff)}>{run?.message || step.sub}</span>
                {thread.phase === 'plan'
                  ? <span {...stylex.props(mw.planCost)}>{stepCostLabel(step)}</span>
                  : run && <span role="status" {...stylex.props(mw.runState, run.state === 'done' && mw.runStateDone, run.state === 'failed' && mw.runStateFailed)}>{assistantCopy.progress[run.state]}</span>}
              </span>
            } />
          </div>
        })}

        {thread.phase === 'plan' && totals && <div {...stylex.props(mw.planFooter)}>
          <span {...stylex.props(mw.planTotal)}>{totalLine(thread.plan, selected)}{thread.plan.imagesLeftToday != null ? ` · ${assistantCopy.plan.imagesLeft(thread.plan.imagesLeftToday)}` : ''}</span>
          <Button variant="quiet" size="sm" onClick={cancelPlan}>{c.cancel}</Button>
          <Button variant="primary" size="lg" disabled={totals.count === 0} onClick={() => confirm()}>{c.run(totals.count)}</Button>
        </div>}

        {thread.phase !== 'plan' && <div {...stylex.props(mw.planFooter)}>
          <span role="status" aria-live="polite" {...stylex.props(mw.planTotal, failed > 0 && mw.planFailed)}>
            {thread.phase === 'running'
              ? (thread.lost ? assistantCopy.states.lostProgress : c.summary(done, steps.length))
              : failed > 0 ? assistantCopy.progress.failedSummary(failed) : c.summary(done, steps.length)}
          </span>
          {running
            ? <span {...stylex.props(mw.planKeep)}>{assistantCopy.states.keepOpenWhileRunning}</span>
            : <Button variant="primary" size="sm" onClick={closeSheet}>{c.close}</Button>}
        </div>}
      </>}
    </div>
  </Sheet>
}

