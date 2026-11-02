import { useCallback, useEffect, useMemo, useState } from 'react'
import { cache } from '@noverse/ui/data'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import {
  cancelPromptSample, getPromptSamples, startPromptSample,
  type PromptSampleStatus,
} from '../../api/cover-trials'
import { t } from '../../i18n/library'

/** Hỏi lại trạng thái khi việc vẽ đang chạy — khớp nhịp thanh việc nền. */
const POLL_MS = 1500

export type PromptTrialControl = {
  status: PromptSampleStatus | undefined
  loading: boolean
  error: string | null
  running: boolean
  start: (videoId: string, sceneId?: string) => Promise<void>
  cancel: (sampleId: string) => Promise<void>
  refresh: () => Promise<unknown>
}

export function usePromptTrial(promptId: string): PromptTrialControl {
  const copy = t.library.prompts.editor
  const enabled = promptId !== ''
  const status = useResource(
    keys.promptSamples(promptId),
    ({ signal }) => getPromptSamples(promptId, signal),
    { enabled },
  )
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

  const start = useCallback(async (videoId: string, sceneId?: string) => {
    setBusy(true)
    setError(null)
    try {
      await startPromptSample(promptId, { videoId, ...(sceneId ? { sceneId } : {}), confirm: true })
      await status.refresh()
      void cache.refresh(keys.prompts('', ''))
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : copy.tryStartFail)
    } finally {
      setBusy(false)
    }
  }, [promptId, status.refresh, copy.tryStartFail])

  const cancel = useCallback(async (sampleId: string) => {
    setBusy(true)
    setError(null)
    try {
      await cancelPromptSample(promptId, sampleId)
      await status.refresh()
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : copy.tryCancelFail)
    } finally {
      setBusy(false)
    }
  }, [promptId, status.refresh, copy.tryCancelFail])

  return {
    status: status.data,
    loading: status.initialLoading,
    error: error ?? (status.error instanceof Error ? status.error.message : null),
    running: running || busy,
    start,
    cancel,
    refresh: status.refresh,
  }
}
