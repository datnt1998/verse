// Thẻ hướng dẫn khi máy chưa sẵn sàng: thiếu bộ đọc trên máy hoặc thiếu ffmpeg
// (15-audio-impl.md §3). Nút “Cài bộ đọc trên máy” gọi POST /api/tts/install; lượt
// cài có một hàng thật trong việc chạy nền (GET /api/jobs, loại `voice`) nên thẻ
// đọc đúng tiến độ/lỗi từ GET /api/tts/install và mở dock để xem chi tiết.
import { useEffect, useState } from 'react'
import { getTtsInstall, installTts } from '../../api/client'
import type { AudioReady, TtsInstallProgress } from '../../api/types'
import { keys, useAction } from '../../data'
import { dispatchDock } from '../../shell/jobs/store'
import { Button } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { toast } from '@noverse/ui'
import { t } from '../../i18n/vi'

/** Nhịp hỏi mốc cài trong lúc đang cài; khớp nhịp tải lại `S/audio/ready` cũ. */
const POLL_MS = 1500

export function InstallCard({ project, story, ready, onReady }: {
  project: string; story: string; ready: AudioReady
  onReady: () => void
}) {
  const [progress, setProgress] = useState<TtsInstallProgress | null>(null)
  const install = useAction(async () => {
    const result = await installTts()
    // POST xong là biết chắc đã vào lượt cài: đừng để nút bật lại trong lúc chờ nhịp hỏi.
    if (result.job) setProgress({ state: 'running', step: '', percent: null, bytesDone: 0, bytesTotal: 0, etaSec: 0, message: '', updated: new Date().toISOString() })
    onReady()
    dispatchDock({ type: 'open' })
    toast(t.audio.install.dangCaiBoDoc, 'ok', { label: 'Mở việc chạy nền', onClick: () => dispatchDock({ type: 'open' }) })
    return result
  }, { invalidate: [keys.tts(), keys.jobs(), keys.story(project, story, 'audio', 'ready')] })

  const running = install.pending || progress?.state === 'running' || ready.engine.state === 'installing'
  const failed = !ready.ready && !running && progress?.state === 'error'
  const percent = progress && progress.state === 'running' && progress.percent !== null ? Math.max(0, Math.min(100, progress.percent)) : null

  // Đọc mốc cài khi mở thẻ (bắt lượt đang chạy hoặc lỗi của lần trước), rồi theo dõi
  // tới khi xong/hỏng — nhờ vậy “Đang cài…” không bao giờ đứng mãi. Chỉ gọi `onReady`
  // (tải lại cả nhóm audio) đúng lúc lượt cài kết thúc, không gọi mỗi nhịp: gọi mỗi
  // 1,5 giây sẽ làm cả màn audio nháy suốt thời gian cài.
  useEffect(() => {
    if (ready.ready) return
    let alive = true
    let settled = false
    const read = () => {
      getTtsInstall()
        .then((next) => {
          if (!alive) return
          setProgress(next)
          if (!settled && next.state !== 'running') { settled = true; onReady() }
        })
        .catch(() => { /* máy chủ cũ chưa có endpoint: giữ trạng thái từ `ready` */ })
    }
    read()
    if (!running) return () => { alive = false }
    const timer = window.setInterval(read, POLL_MS)
    return () => { alive = false; window.clearInterval(timer) }
  }, [ready.ready, running, onReady])

  if (ready.ready) return null
  const missingEngine = !ready.engine.ready
  const missingFfmpeg = !ready.ffmpeg
  const title = missingFfmpeg
    ? t.audio.install.mayChuaCoFfmpeg
    : missingEngine
      ? (failed ? t.audio.install.boDocDangLoi : t.audio.install.boDocTrenMay)
      : t.audio.install.boDocDangLoi
  const detail = (failed ? (progress?.error || progress?.message) : '') || ready.message || t.audio.install.vanSoanSuaVa

  return (
    <div role="status" style={{
      display: 'flex', gap: 12, padding: '10px 12px', margin: '10px 16px 0',
      borderRadius: 12, border: '1px solid rgba(246,203,140,.2)', background: 'rgba(246,203,140,.06)',
      alignItems: 'flex-start',
    }}>
      <Icon name={missingFfmpeg ? 'warning-2' : 'cpu'} size={17} style={{ color: 'var(--color-warn)', flex: '0 0 auto', marginTop: 1 }} />
      <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 3 }}>
        <span style={{ fontSize: 13, fontWeight: 500 }}>{title}</span>
        <span style={{ fontSize: 12, color: 'var(--color-fg-3)' }}>{detail}</span>
        {running && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 3 }}>
            {percent === null
              ? <Spinner size={12} color="var(--color-warn)" />
              : (
                <span role="progressbar" aria-label={t.audio.install.dangCai} aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent}
                  style={{ flex: '0 0 168px', height: 4, borderRadius: 2, background: 'var(--line-2)', overflow: 'hidden' }}>
                  <span style={{ display: 'block', height: '100%', width: `${percent}%`, borderRadius: 2, background: 'var(--color-warn)' }} />
                </span>
              )}
            <span className="num" style={{ fontSize: 11.5, color: 'var(--color-fg-3)' }}>
              {percent === null ? t.audio.install.dangCai : t.audio.install.dangCaiPhanTram(percent)}
            </span>
          </div>
        )}
        {running && (
          <button type="button" className="hov-ghost" onClick={() => dispatchDock({ type: 'open' })}
            style={{
              alignSelf: 'flex-start', margin: 0, padding: 0, border: 'none', background: 'transparent',
              color: 'var(--color-info)', fontSize: 12, display: 'inline-flex', alignItems: 'center', gap: 4,
            }}>
            {t.audio.install.xemTienDo}<Icon name="arrow-right-2" size={12} />
          </button>
        )}
      </div>
      {!running && missingEngine && (
        <Button variant="primary" size="sm" disabled={install.pending} pending={install.pending} onClick={() => void install.run()}>
          {failed ? t.audio.install.thuLai : t.audio.install.caiBoDocTren}
        </Button>
      )}
      {!running && !missingEngine && missingFfmpeg && (
        <span style={{ fontSize: 11.5, color: 'var(--color-fg-3)', alignSelf: 'center' }}>
          {t.audio.install.caiFfmpegRoiMo}</span>
      )}
    </div>
  )
}
