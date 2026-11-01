import { useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Dialog, Icon, Spinner } from '@noverse/ui'
import { formatBytes } from '../../lib/units'
import { rangeTime } from './FixPanel'
import { sx } from './editor.stylex'
import { exportStyles as s } from './export.stylex'

export type ExportProgress = { phase?: string; framesDone?: number; framesTotal?: number; bytesSent?: number; bytesTotal?: number }

export function ExportingDialog({ seconds, fps, progress, onCancel }: { seconds: number; fps: number; progress?: ExportProgress; onCancel: () => void }) {
  const percent = progress?.framesTotal ? Math.round((progress.framesDone ?? 0) / progress.framesTotal * 100) : progress?.bytesTotal ? Math.round((progress.bytesSent ?? 0) / progress.bytesTotal * 100) : null
  const rendered = progress?.phase === 'uploading' || progress?.phase === 'compressing'
  const compressing = progress?.phase === 'compressing'
  return <Dialog label="Đang xuất video" width={520}><div {...stylex.props(sx.dialogBody)}>
    <div {...stylex.props(s.row)}><Spinner size={22} /><h2 {...stylex.props(sx.dialogTitle, sx.spread)}>Đang xuất video</h2><span {...stylex.props(sx.secondary)}>{percent != null ? `${percent}%` : compressing ? 'Mã hoá MP4' : progress?.phase === 'rendering' ? 'Dựng khung hình' : 'Đang chuẩn bị…'}</span></div>
    <span {...stylex.props(s.meta)}>1080p · {fps} fps · toàn bộ {rangeTime(seconds)} · nén sau khi xuất</span>
    <div role="progressbar" aria-label="Tiến độ xuất" aria-valuemin={0} aria-valuemax={100} aria-valuenow={percent ?? undefined} {...stylex.props(sx.scoreTrack)}><span {...stylex.props(sx.scoreFill(percent ?? 0, true))} /></div>
    <div {...stylex.props(sx.stack)}>
      <div {...stylex.props(s.row)}>{rendered ? <Icon name="tick-b" size={16} {...stylex.props(sx.ok)} /> : <Spinner size={14} />}<span {...stylex.props(sx.spread)}>Dựng khung hình</span></div>
      <div {...stylex.props(s.row, !compressing && s.muted)}>{compressing ? <Spinner size={14} /> : <span aria-hidden="true" {...stylex.props(s.dot)}>·</span>}<span {...stylex.props(sx.spread)}>Mã hoá MP4{compressing && progress?.framesDone != null ? ` · ${rangeTime(progress.framesDone / fps)} / ${rangeTime(seconds)}` : ''}</span></div>
      <div {...stylex.props(s.row, s.muted)}><span aria-hidden="true" {...stylex.props(s.dot)}>·</span><span {...stylex.props(sx.spread)}>Lưu lên máy và kiểm tra tệp</span></div>
    </div>
    <div {...stylex.props(sx.notice)}>Bạn vẫn xem được timeline nhưng không sửa được.</div>
    <div {...stylex.props(sx.dialogActions)}><button type="button" {...stylex.props(sx.button, sx.danger)} onClick={onCancel}>Huỷ xuất</button></div>
  </div></Dialog>
}

export function ExportDoneDialog({ seconds, bytes, download, onClose, onBack }: { seconds: number; bytes?: number; download?: string; onClose: () => void; onBack: () => void }) {
  const video = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const [error, setError] = useState<string>()
  const play = () => { void video.current?.play().catch(cause => setError(cause instanceof Error ? cause.message : String(cause))) }
  return <Dialog label="Đã xuất MP4" width={520} onClose={onClose}><div {...stylex.props(sx.dialogBody)}>
    <div {...stylex.props(s.row)}><Icon name="tick-b" size={22} {...stylex.props(sx.ok)} /><h2 {...stylex.props(sx.dialogTitle, sx.spread)}>Đã xuất MP4</h2><span {...stylex.props(s.meta)}>{rangeTime(seconds)}{bytes != null ? ` · ${formatBytes(bytes)}` : ''}</span></div>
    {download && <div {...stylex.props(s.videoFrame)}><video ref={video} controls={playing} src={download} preload="metadata" aria-label="Bản vừa xuất" {...stylex.props(s.video)} onPlay={() => { setPlaying(true); setError(undefined) }} onPause={() => setPlaying(false)} onEnded={() => setPlaying(false)} />{!playing && <><button type="button" aria-label="Phát bản vừa xuất" {...stylex.props(s.play)} onClick={play}><Icon name="play" size={20} /></button><span {...stylex.props(s.duration)}>{rangeTime(seconds)}</span></>}</div>}
    {error && <p role="alert" {...stylex.props(sx.dialogCopy)}>{error}</p>}
    <div {...stylex.props(s.notice)}><Icon name="tick-b" size={16} {...stylex.props(sx.ok)} /><span>Quay lại Bàn dựng, Kiểm định sẽ đo lại độ hoàn thiện cho bản này. Đạt từ 90 thì video chuyển sang Sẵn sàng.</span></div>
    <div {...stylex.props(sx.dialogActions)}>{download && <a href={download} download {...stylex.props(sx.button)}><Icon name="download" size={15} />Tải MP4</a>}<button type="button" {...stylex.props(sx.button, sx.primary)} onClick={onBack}>Quay lại Bàn dựng</button></div>
  </div></Dialog>
}
