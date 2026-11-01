// Sheet Kế hoạch của trợ lý trên điện thoại (artboard MMain › plan). Gọi đúng
// /api/assistant/plan + /run-plan, hiện câu hỏi/chưa hiểu, tích bước, chạy và
// theo dõi tiến độ thật. Không dựng dữ liệu kế hoạch giả.
import { useEffect, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Checkbox, Sheet } from '@noverse/ui'
import { getPlanRun, planAssistant, runPlan, type AssistantScope, type PlanReply, type PlanRun } from '../../../api/assistant'
import { allSelected, chosenSteps, planTotals, stepCostLabel, toggleStep, totalLine, type Selection } from '../../../components/assistant/planModel'
import { screenWorkbench } from '../../../i18n/screens'
import { mw } from './mworkbench.stylex'

const c = screenWorkbench.mobile.plan
const STATE_LABEL: Record<string, string> = { queued: 'Đang chờ', running: 'Đang làm', done: 'Xong', failed: 'Chưa làm được' }

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
  const [current, setCurrent] = useState(text)
  const [reply, setReply] = useState<PlanReply | null>(null)
  const [selected, setSelected] = useState<Selection>(new Set())
  const [run, setRun] = useState<PlanRun | null>(null)
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const settled = useRef(false)

  const plan = reply?.kind === 'plan' ? reply.plan : null

  const ask = (input: string) => {
    setBusy(true); setError(''); setReply(null); setRun(null); settled.current = false
    planAssistant({ scope, text: input })
      .then((next) => {
        setReply(next)
        setSelected(next.kind === 'plan' ? allSelected(next.plan) : new Set())
      })
      .catch((err: unknown) => setError(err instanceof Error ? err.message : 'Không lập được kế hoạch.'))
      .finally(() => setBusy(false))
  }

  useEffect(() => { ask(text) }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!run || run.state === 'done' || run.state === 'failed') return
    let cancelled = false
    const timer = setInterval(() => {
      getPlanRun(run.runId)
        .then((next) => {
          if (cancelled) return
          setRun(next)
          if (next.state === 'done' || next.state === 'failed') {
            if (!settled.current) { settled.current = true; onSettled() }
          }
        })
        .catch((err: unknown) => { if (!cancelled) setError(err instanceof Error ? err.message : c.failed) })
    }, 1200)
    return () => { cancelled = true; clearInterval(timer) }
  }, [run, onSettled])

  const start = async () => {
    if (!plan) return
    setBusy(true); setError('')
    try {
      const next = await runPlan(plan.planId, chosenSteps(plan, selected).map((step) => step.id))
      setRun(next)
      if (next.state === 'done' || next.state === 'failed') { settled.current = true; onSettled() }
    } catch (err) { setError(err instanceof Error ? err.message : 'Không chạy được kế hoạch.') } finally { setBusy(false) }
  }

  const totals = plan ? planTotals(plan, selected) : null
  const runState = run ? Object.fromEntries(run.steps.map((step) => [step.id, step])) : {}
  const finished = run?.state === 'done' || run?.state === 'failed'

  return <Sheet open onClose={onClose} label={c.title} size="md" height="auto">
    <div {...stylex.props(mw.planBody)}>
      <div {...stylex.props(mw.planHead)}>
        <h2 {...stylex.props(mw.planTitle)}>{plan ? c.head(plan.steps.length) : c.title}</h2>
        <p {...stylex.props(mw.planQuote)}>{c.quote(current)}</p>
      </div>

      {busy && !reply && <p role="status" {...stylex.props(mw.notice)}>Đang lập kế hoạch…</p>}
      {error && <div role="alert" {...stylex.props(mw.error)}>{error}</div>}

      {reply?.kind === 'ask' && <div {...stylex.props(mw.planAsk)}>
        <strong {...stylex.props(mw.planStepTitle)}>{c.ask}</strong>
        <p {...stylex.props(mw.planStepSub)}>{reply.ask.question}</p>
        {reply.ask.options.map((option) => <Button key={option.text} variant="ghost" block disabled={busy} onClick={() => { setCurrent(option.text); ask(option.text) }}>{option.label}</Button>)}
      </div>}

      {reply?.kind === 'unknown' && <div {...stylex.props(mw.planAsk)}>
        <strong {...stylex.props(mw.planStepTitle)}>{c.unknown}</strong>
        <p {...stylex.props(mw.planStepSub)}>{reply.unknown.lead}</p>
        {reply.unknown.suggestions.map((option) => <Button key={option.text} variant="ghost" block disabled={busy} onClick={() => { setCurrent(option.text); ask(option.text) }}>{option.label}</Button>)}
        {reply.unknown.link && <a {...stylex.props(mw.moreLink)} href={reply.unknown.link.href}>{reply.unknown.link.label} ›</a>}
      </div>}

      {plan && <>
        {plan.steps.map((step) => <div key={step.id} {...stylex.props(mw.planStep)}>
          <Checkbox checked={selected.has(step.id)} onChange={() => setSelected((current) => toggleStep(current, step.id))} disabled={busy || Boolean(run)} label={
            <span {...stylex.props(mw.planStepText)}>
              <span {...stylex.props(mw.planStepTitle)}>{step.title}</span>
              <span {...stylex.props(mw.planStepSub)}>{step.sub}</span>
              <span {...stylex.props(mw.planCost)}>{stepCostLabel(step)}</span>
            </span>
          } />
        </div>)}
        {run && <div {...stylex.props(mw.card)}>
          <div {...stylex.props(mw.cardHead)}>
            <strong {...stylex.props(mw.cardTitle)}>{finished ? (run.state === 'failed' ? c.failed : c.done) : c.running}</strong>
            <span {...stylex.props(mw.cardHint)}>{c.summary(run.steps.filter((step) => step.state === 'done').length, run.steps.length)}</span>
          </div>
          {plan.steps.map((step) => {
            const state = runState[step.id]
            if (!state) return null
            return <div key={step.id} {...stylex.props(mw.runRow)}>
              <span>{step.title}</span>
              <span {...stylex.props(mw.runState)}>{STATE_LABEL[state.state] ?? state.state}{state.message ? ` · ${state.message}` : ''}</span>
            </div>
          })}
        </div>}
        {!run && <div {...stylex.props(mw.planFooter)}>
          <span {...stylex.props(mw.planTotal)}>{totalLine(plan, selected)}{plan.imagesLeftToday != null ? ` · còn ${plan.imagesLeftToday} hôm nay` : ''}</span>
          <Button variant="primary" size="lg" disabled={busy || !totals?.count} onClick={() => void start()}>{totals?.count ? c.run(totals.count) : c.noStep}</Button>
        </div>}
      </>}

      <div {...stylex.props(mw.planFooter)}>
        <Button variant="quiet" size="sm" onClick={onClose}>{finished ? c.close : c.editText}</Button>
        {!finished && <Button variant="ghost" size="sm" onClick={onClose}>{c.cancel}</Button>}
      </div>
    </div>
  </Sheet>
}
