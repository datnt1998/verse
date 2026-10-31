// Máy trạng thái của một lượt nhắn trợ lý: câu → kế hoạch (chưa chạy gì) →
// người dùng bỏ/giữ bước → Làm → tiến độ từng bước. Chỉ `confirm` gọi run-plan;
// mọi nhánh khác (huỷ, sửa câu, lỗi, hết hạn) không chạy gì.
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  assistantErrorKind, getPlanRun, planAssistant, runPlan,
  type AssistantErrorKind, type AssistantScope, type Plan, type PlanAsk, type PlanUnknown, type RunStep,
} from '../../api/assistant'
import { allSelected, chosenSteps, planTotals, progressById, readDraft, runFinished, toggleStep, type Selection, type StepProgress } from './planModel'

export type Thread =
  | { phase: 'idle' }
  | { phase: 'planning'; text: string }
  | { phase: 'plan'; text: string; plan: Plan; selected: Selection }
  | { phase: 'ask'; text: string; ask: PlanAsk }
  | { phase: 'unknown'; text: string; unknown: PlanUnknown }
  | { phase: 'error'; text: string; kind: AssistantErrorKind; message: string; retry: { type: 'plan' } | { type: 'run'; plan: Plan; selected: Selection } }
  | { phase: 'running'; text: string; plan: Plan; selected: Selection; steps: StepProgress; lost: boolean }
  | { phase: 'done'; text: string; plan: Plan; selected: Selection; steps: StepProgress }

export const POLL_RUN_MS = 1500
const MAX_POLL_FAILURES = 3

const messageOf = (error: unknown) => error instanceof Error ? error.message : ''

export type AssistantThread = {
  thread: Thread
  /** Gửi câu mới; trả false khi câu rỗng/quá dài hoặc đang bận (caller giữ nguyên chữ). */
  send: (raw: string) => boolean
  /** Chọn một lựa chọn/gợi ý: chỉ lập kế hoạch mới từ câu đó, không chạy gì. */
  choose: (text: string) => void
  toggle: (id: string) => void
  /** Bấm Làm N bước: chạy đúng các bước còn tích. */
  confirm: () => void
  /** Huỷ thẻ: không chạy gì. */
  cancel: () => void
  /** Sửa câu: đóng thẻ, trả câu gốc để ô nhập giữ lại. */
  edit: () => string | undefined
  retry: () => void
  dismiss: () => void
  busy: boolean
}

export function useAssistantThread(scope: AssistantScope, onSettled?: () => void): AssistantThread {
  const [thread, setThread] = useState<Thread>({ phase: 'idle' })
  const live = useRef(thread)
  live.current = thread
  const scopeRef = useRef(scope)
  scopeRef.current = scope
  const settled = useRef(onSettled)
  settled.current = onSettled
  const controller = useRef<AbortController | null>(null)

  const begin = () => {
    controller.current?.abort()
    controller.current = new AbortController()
    return controller.current.signal
  }
  useEffect(() => () => controller.current?.abort(), [])

  const plan = useCallback(async (text: string) => {
    const signal = begin()
    setThread({ phase: 'planning', text })
    try {
      const reply = await planAssistant({ scope: scopeRef.current, text }, signal)
      if (signal.aborted) return
      if (reply.kind === 'plan') setThread({ phase: 'plan', text, plan: reply.plan, selected: allSelected(reply.plan) })
      else if (reply.kind === 'ask') setThread({ phase: 'ask', text, ask: reply.ask })
      else setThread({ phase: 'unknown', text, unknown: reply.unknown })
    } catch (error) {
      if (signal.aborted) return
      setThread({ phase: 'error', text, kind: assistantErrorKind(error), message: messageOf(error), retry: { type: 'plan' } })
    }
  }, [])

  const send = useCallback((raw: string) => {
    const current = live.current.phase
    if (current === 'planning' || current === 'running') return false
    const draft = readDraft(raw)
    if (!draft.ok) return false
    void plan(draft.text)
    return true
  }, [plan])

  const choose = useCallback((text: string) => {
    const phase = live.current.phase
    if (phase === 'ask' || phase === 'unknown') void plan(text)
  }, [plan])

  const toggle = useCallback((id: string) => {
    setThread((current) => current.phase === 'plan' ? { ...current, selected: toggleStep(current.selected, id) } : current)
  }, [])

  const finishedOf = (state: string | undefined, steps: readonly RunStep[]) => runFinished(steps) || state === 'done' || state === 'failed'

  const poll = useCallback(async (text: string, planned: Plan, selected: Selection, runId: string, signal: AbortSignal) => {
    let failures = 0
    while (!signal.aborted) {
      const { promise, resolve } = Promise.withResolvers<void>()
      const timer = window.setTimeout(resolve, POLL_RUN_MS)
      signal.addEventListener('abort', () => { window.clearTimeout(timer); resolve() }, { once: true })
      await promise
      if (signal.aborted) return
      try {
        const status = await getPlanRun(runId, signal)
        if (signal.aborted) return
        failures = 0
        const steps = progressById(status.steps)
        if (finishedOf(status.state, status.steps)) {
          setThread({ phase: 'done', text, plan: planned, selected, steps })
          settled.current?.()
          return
        }
        setThread({ phase: 'running', text, plan: planned, selected, steps, lost: false })
      } catch {
        if (signal.aborted) return
        failures += 1
        if (failures >= MAX_POLL_FAILURES) setThread((current) => current.phase === 'running' ? { ...current, lost: true } : current)
      }
    }
  }, [])

  const run = useCallback(async (text: string, planned: Plan, selected: Selection) => {
    if (planTotals(planned, selected).count === 0) return
    const signal = begin()
    const ids = chosenSteps(planned, selected).map((step) => step.id)
    setThread({ phase: 'running', text, plan: planned, selected, steps: {}, lost: false })
    try {
      const started = await runPlan(planned.planId, ids, signal)
      if (signal.aborted) return
      const steps = progressById(started.steps)
      settled.current?.()
      if (finishedOf(started.state, started.steps)) {
        setThread({ phase: 'done', text, plan: planned, selected, steps })
        return
      }
      setThread({ phase: 'running', text, plan: planned, selected, steps, lost: false })
      void poll(text, planned, selected, started.runId, signal)
    } catch (error) {
      if (signal.aborted) return
      setThread({ phase: 'error', text, kind: assistantErrorKind(error), message: messageOf(error), retry: { type: 'run', plan: planned, selected } })
    }
  }, [poll])

  const confirm = useCallback(() => {
    const current = live.current
    if (current.phase === 'plan') void run(current.text, current.plan, current.selected)
  }, [run])

  const cancel = useCallback(() => {
    const phase = live.current.phase
    if (phase === 'running') return
    controller.current?.abort()
    setThread({ phase: 'idle' })
  }, [])

  const edit = useCallback(() => {
    const current = live.current
    if (current.phase === 'idle' || current.phase === 'running') return undefined
    controller.current?.abort()
    setThread({ phase: 'idle' })
    return current.text
  }, [])

  const retry = useCallback(() => {
    const current = live.current
    if (current.phase !== 'error') return
    if (current.retry.type === 'plan') void plan(current.text)
    else void run(current.text, current.retry.plan, current.retry.selected)
  }, [plan, run])

  const dismiss = useCallback(() => {
    if (live.current.phase === 'running') return
    controller.current?.abort()
    setThread({ phase: 'idle' })
  }, [])

  const busy = thread.phase === 'planning' || thread.phase === 'running'
  return { thread, send, choose, toggle, confirm, cancel, edit, retry, dismiss, busy }
}

export type { RunStep }
