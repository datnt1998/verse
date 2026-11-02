import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { styles } from './headless.stylex'
import { HeadlessClient, parseHeadlessCredential, type HeadlessKind, type HeadlessPageProps, type HeadlessProgress } from './headless-client'
import { runHeadlessRequest, type HeadlessOutcome } from './headless-runner'
import { measurementFailure } from '../ui/clone/measurement-runtime'

type Stage = { kind: 'init' } | { kind: 'running'; progress: HeadlessProgress } | { kind: 'done'; outcome: HeadlessOutcome } | { kind: 'error'; message: string }


function progressLabel(progress: HeadlessProgress): string {
  switch (progress.phase) {
    case 'load': return 'Đang tải dự án…'
    case 'claim': return 'Đang nhận yêu cầu…'
    case 'caps': return 'Đang kiểm tra bộ xử lý video…'
    case 'decode': return 'Đang trộn âm thanh…'
    case 'finish': return 'Đang đo độ lớn âm thanh…'
    case 'upload': return `Đang gửi âm thanh — ${((progress.bytesSent ?? 0) / 1024 ** 2).toFixed(1)} MB`
    case 'uploading': return `Đang tải lên — ${((progress.bytesSent ?? 0) / 1024 ** 2).toFixed(1)} / ${((progress.bytesTotal ?? 0) / 1024 ** 2).toFixed(1)} MB`
    case 'rendering': return progress.framesDone !== null && progress.framesTotal ? `Đang dựng video — ${progress.framesDone}/${progress.framesTotal} khung` : 'Đang dựng video…'
    default: return 'Đang xử lý…'
  }
}

export function HeadlessPage({ kind, requestId, token }: HeadlessPageProps & { kind: HeadlessKind }) {
  const [stage, setStage] = useState<Stage>({ kind: 'init' })
  useEffect(() => {
    const controller = new AbortController()
    let mounted = true
    // Defer the single-use claim until StrictMode's mount probe has been cleaned up.
    void Promise.resolve().then(async () => {
      if (controller.signal.aborted) return
      setStage({ kind: 'init' })
      const credential = parseHeadlessCredential(requestId, token, window.location.hash)
      const client = new HeadlessClient(kind, credential.requestId, credential.token)
      const outcome = await runHeadlessRequest(client, controller, progress => {
        if (mounted) setStage({ kind: 'running', progress })
      })
      if (mounted) setStage({ kind: 'done', outcome })
    }).catch((error: unknown) => {
      if (!mounted) return
      const cause = error instanceof Error && error.cause instanceof Error ? error.cause.message : ''
      const message = kind === 'measure' ? measurementFailure(error).message : error instanceof Error ? error.message : String(error)
      if (kind === 'measure') console.error('[measurement] page failure', { cause: cause || undefined, error })
      setStage({ kind: 'error', message: kind === 'measure' ? message : cause ? `${message}: ${cause}` : message })
    })
    return () => { mounted = false; controller.abort(new DOMException('Đã đóng trang xử lý', 'AbortError')) }
  }, [kind, requestId, token])

  const stageLabel = stage.kind === 'running' ? `running-${stage.progress.phase === 'rendering' ? 'render' : stage.progress.phase === 'uploading' ? 'upload' : stage.progress.phase}` : stage.kind
  useEffect(() => { document.title = `${kind === 'render' ? 'Xuất ngầm' : 'Đo Độ hoàn thiện'} — ${stageLabel}` }, [kind, stageLabel])

  return <main {...stylex.props(styles.page)} data-stage={stageLabel}>
    <h1 {...stylex.props(styles.heading)}>{kind === 'render' ? 'Xuất video ngầm' : 'Đo Độ hoàn thiện'}</h1>
    {stage.kind === 'init' && <p {...stylex.props(styles.message)}>Đang chuẩn bị…</p>}
    {stage.kind === 'running' && <p {...stylex.props(styles.message)} data-progress={stage.progress.chunksSent} data-frames-done={stage.progress.framesDone ?? undefined} data-frames-total={stage.progress.framesTotal ?? undefined} data-bytes-sent={stage.progress.bytesSent} data-bytes-total={stage.progress.bytesTotal}>{progressLabel(stage.progress)}</p>}
    {stage.kind === 'done' && <p {...stylex.props(styles.message)} data-done="true">{'exportId' in stage.outcome ? 'Xong — có thể đóng trang này.' : `Xong — độ lớn âm thanh ${stage.outcome.lufs !== null ? `${stage.outcome.lufs.toFixed(1)} LUFS` : 'không đo được'}, ${stage.outcome.durationSec.toFixed(1)}s. Có thể đóng trang này.`}</p>}
    {stage.kind === 'error' && <p {...stylex.props(styles.message, styles.error)} data-error="true" role="alert">{stage.message}</p>}
  </main>
}
