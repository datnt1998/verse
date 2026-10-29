import { useRef, useState } from 'react'
import { ApiError, checkProjectLedger, suggestProjectLedger } from '../../api/client'
import type { LedgerCheckResult } from '../../api/types'

type CheckState = {
  brief: string
  phase: 'checking' | 'result'
  result?: LedgerCheckResult
  previous?: { brief: string; result: LedgerCheckResult }
  changed?: number[]
}

export interface StoryCheck {
  state: CheckState | null
  blocked: boolean
  checking: boolean
  suggesting: boolean
  check: () => Promise<boolean>
  acceptConflict: (error: unknown) => boolean
  suggest: () => Promise<void>
  undo: () => void
}

/** Both entry modes share the same project check; the start endpoints remain authoritative. */
export function useStoryCheck(project: string, brief: string, onBrief: (brief: string) => void) {
  const [state, setState] = useState<CheckState | null>(null)
  const [suggesting, setSuggesting] = useState(false)
  const latest = useRef(brief)
  latest.current = brief
  const current = state?.brief === brief ? state : null
  const blocked = !!current?.result?.checks.some((check) => check.level === 'error')
  const checking = current?.phase === 'checking'

  const check = async () => {
    const submitted = brief
    if (current?.result) return !blocked
    setState({ brief: submitted, phase: 'checking' })
    try {
      const result = await checkProjectLedger(project, submitted)
      if (latest.current !== submitted) return false
      setState({ brief: submitted, phase: 'result', result })
      return !result.checks.some((item) => item.level === 'error')
    } catch (error) {
      if (latest.current === submitted) setState(null)
      throw error
    }
  }

  const acceptConflict = (error: unknown) => {
    if (!(error instanceof ApiError) || error.status !== 409 || !error.payload || typeof error.payload !== 'object' || !('checks' in error.payload) || !Array.isArray(error.payload.checks)) return false
    const checks = error.payload.checks as LedgerCheckResult['checks']
    const payload = error.payload as Partial<LedgerCheckResult>
    setState((previous) => ({
      brief: latest.current, phase: 'result',
      result: { axes: payload.axes ?? previous?.result?.axes ?? [], values: payload.values ?? previous?.result?.values ?? [], cells: payload.cells ?? previous?.result?.cells ?? [], checks },
    }))
    return true
  }

  const suggest = async () => {
    if (!current?.result || suggesting) return
    const previous = { brief, result: current.result }
    setSuggesting(true)
    try {
      const suggestion = await suggestProjectLedger(project, brief)
      const result = await checkProjectLedger(project, suggestion.brief)
      if (latest.current !== previous.brief) return
      setState({ brief: suggestion.brief, phase: 'result', result, previous, changed: suggestion.changed })
      onBrief(suggestion.brief)
    } finally {
      setSuggesting(false)
    }
  }

  const undo = () => {
    if (!current?.previous) return
    const previous = current.previous
    setState({ brief: previous.brief, phase: 'result', result: previous.result })
    onBrief(previous.brief)
  }

  return { state: current, blocked, checking, suggesting, check, acceptConflict, suggest, undo }
}


