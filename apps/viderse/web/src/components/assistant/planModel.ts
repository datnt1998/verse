// Logic thuần của thẻ Kế hoạch: chọn bước, giá, nút "Làm N bước". Không React,
// để test được chọn bước, trạng thái nhấn mạnh và tổng giá mà không dựng DOM.
import type { AssistantScope, Plan, PlanStep, RunStep } from '../../api/assistant'
import { MAX_TEXT_RUNES } from '../../api/assistant'
import { assistantCopy as copy } from '../../i18n/assistant'

export type Selection = ReadonlySet<string>

/** Mọi bước đều được tích sẵn khi thẻ hiện ra; người dùng bỏ từng bước. */
export const allSelected = (plan: Plan): Selection => new Set(plan.steps.map((step) => step.id))

export const toggleStep = (selected: Selection, id: string): Selection => {
  const next = new Set(selected)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  return next
}

export const chosenSteps = (plan: Plan, selected: Selection): PlanStep[] => plan.steps.filter((step) => selected.has(step.id))

export type PlanTotals = { count: number; images: number; enqueuesVideo: boolean }

export function planTotals(plan: Plan, selected: Selection): PlanTotals {
  const chosen = chosenSteps(plan, selected)
  return {
    count: chosen.length,
    images: chosen.reduce((sum, step) => sum + (step.cost?.images ?? 0), 0),
    enqueuesVideo: chosen.some((step) => step.enqueuesVideo),
  }
}

export const stepCostLabel = (step: PlanStep): string => step.cost?.images ? copy.plan.cost(step.cost.images) : copy.plan.free

/** Dòng tổng giá: số ảnh các bước còn tích + ghi chú nguồn (nếu máy chủ có). */
export function totalLine(plan: Plan, selected: Selection): string {
  const { images } = planTotals(plan, selected)
  const base = images ? copy.plan.cost(images) : copy.plan.noImages
  return plan.total.note ? `${base} · ${plan.total.note}` : base
}

/** Một việc của một video → thẻ xác nhận (Dialogs assistant-confirm); còn lại → thẻ Kế hoạch. */
export const isSingleJobStep = (plan: Plan, scope: AssistantScope): boolean =>
  plan.steps.length === 1 && typeof scope === 'object' && 'jobId' in scope

export const textRunes = (text: string): number => [...text].length

export type Draft = { ok: true; text: string } | { ok: false; reason: 'empty' | 'tooLong' }

export function readDraft(raw: string): Draft {
  const text = raw.trim()
  if (!text) return { ok: false, reason: 'empty' }
  if (textRunes(text) > MAX_TEXT_RUNES) return { ok: false, reason: 'tooLong' }
  return { ok: true, text }
}

export type StepProgress = Record<string, RunStep>

export const progressById = (steps: readonly RunStep[]): StepProgress => Object.fromEntries(steps.map((step) => [step.id, step]))

export const runFinished = (steps: readonly RunStep[]): boolean => steps.length > 0 && steps.every((step) => step.state === 'done' || step.state === 'failed')
