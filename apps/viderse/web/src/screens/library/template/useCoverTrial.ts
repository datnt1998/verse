import { useCallback, useEffect, useMemo, useState } from 'react'
import { cache } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import {
  cancelCoverTrial, getCoverTrials, startCoverTrial,
  type CoverTrialStatus,
} from '../../../api/cover-trials'
import { t } from '../../../i18n/library'

/** Hỏi lại trạng thái khi việc vẽ đang chạy — khớp nhịp thanh việc nền. */
const POLL_MS = 1500

export type CoverTrialControl = {
  status: CoverTrialStatus | undefined
  loading: boolean
  error: string | null
  /** Đang có lần vẽ chạy (task hoặc hàng running) → hỏi lại + hiện tiến độ. */
  running: boolean
  start: (jobId: string) => Promise<void>
  cancel: (trialId: string) => Promise<void>
  refresh: () => Promise<unknown>
}

export function useCoverTrial(templateId: string): CoverTrialControl {
  const copy = t.library.template.cover
  const enabled = templateId !== ''
  const status = useResource(keys.coverTrials(templateId), ({ signal }) => getCoverTrials(templateId, signal), { enabled })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const running = useMemo(
    () => (status.data?.history ?? []).some((row) => row.state === 'running'),
    [status.data],
  )

  useEffect(() => {
    if (!running) return
    const timer = setInterval(() => { void status.refresh() }, POLL_MS)
    return () => clearInterval(timer)
  }, [running, status.refresh])

  const start = useCallback(async (jobId: string) => {
    setBusy(true)
    setError(null)
    try {
      await startCoverTrial(templateId, { jobId, confirm: true })
      await status.refresh()
      void cache.refresh(keys.templates())
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : copy.trialStartFail)
    } finally {
      setBusy(false)
    }
  }, [templateId, status.refresh, copy.trialStartFail])

  const cancel = useCallback(async (trialId: string) => {
    setBusy(true)
    setError(null)
    try {
      await cancelCoverTrial(templateId, trialId)
      await status.refresh()
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : copy.trialCancelFail)
    } finally {
      setBusy(false)
    }
  }, [templateId, status.refresh, copy.trialCancelFail])

  const errorText = error
    ?? (status.error instanceof Error ? status.error.message : null)

  return {
    status: status.data,
    loading: status.initialLoading,
    error: errorText,
    running: running || busy,
    start,
    cancel,
    refresh: status.refresh,
  }
}
