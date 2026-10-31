// Trang Video lẻ của tab Video nền: lưới thẻ video (đang tải có thanh tiến
// độ, lỗi có nút Tải lại = thêm lại cùng URL, sẵn sàng có ô Đoạn dùng), menu
// ⋯: Chọn đoạn khác (dialog SegmentPicker → PATCH) và Bỏ video (409 in_use
// hiện câu lỗi kèm tên template từ máy chủ).
import { useState } from 'react'
import { ActionMenu, Banner, Button, Dialog, Icon, ProgressBar } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/vi'
import { keys } from '../../../data/keys'
import {
  addBackdropVideo, backdropVideoFileHref, patchBackdropVideo, removeBackdropVideo,
  type BackdropSegment, type BackdropVideo,
} from '../../../api/backdrop'
import { bytes, duration, kindName, num, platformColor, platformName, segmentLabel, timeRange } from './fmt'
import { SegmentPicker } from './SegmentPicker'
import { ActionButton } from '../../../components'

function VideoCard({ video, onChangeSegment, onRetry, onRemove, busyId }: {
  video: BackdropVideo
  onChangeSegment: (video: BackdropVideo) => void
  onRetry: (video: BackdropVideo) => void
  onRemove: (video: BackdropVideo) => void
  busyId: string | null
}) {
  const copy = t.backdrop.tab.videos
  const segLen = video.segment.toSec - video.segment.fromSec
  const size = video.file ? copy.fileInfo(video.file.height, bytes(video.file.bytes)) : ''
  const busy = busyId === video.id
  return (
    <article role="listitem" className="vbd-vlcard">
      <div className="vbd-thumb">
        {video.state === 'ready' && video.file && (
          <video className="vbd-thumb-img" src={backdropVideoFileHref(video.id)} muted playsInline preload="metadata" />
        )}
        <span className="vbd-kind">{kindName('video')}</span>
        {video.durationSec !== null && <span className="vbd-dur">{timeRange(video.durationSec)}</span>}
      </div>
      <div className="vbd-vlbody">
        <span className="vbd-vtitle">{video.title}</span>
        <span className="vbd-vsub num">
          <span className="vbd-dot" style={{ background: platformColor(video.platform) }} />
          {copy.sub(platformName(video.platform), duration(video.durationSec), size)}
        </span>
      </div>
      {video.state === 'downloading' && (
        <div className="vbd-dl" role="group" aria-label={copy.downloadAria(video.title)}>
          <ProgressBar tone="info" value={Math.round((video.progress ?? 0) * 100)} />
          <span className="vbd-dl-text num">{copy.downloadingEta(num(Math.round((video.progress ?? 0) * 100)))}</span>
        </div>
      )}
      {video.state === 'failed' && (
        <div className="vbd-vlfail">
          <span className="vbd-vlfail-text">
            <Icon name="danger" size={14} />
            {video.errorDetail?.message ?? copy.failedTitle}
          </span>
          <ActionButton size="sm" icon={<Icon name="refresh" size={15} />} pending={busy} onClick={() => onRetry(video)}>{copy.retry}</ActionButton>
        </div>
      )}
      {video.state === 'ready' && (
        <div className="vbd-segbox">
          <span className="vbd-segbox-text num">
            <span className="vbd-segbox-label">{copy.segmentLen(timeRange(segLen))}</span>
            <span className="vbd-segbox-range">{segmentLabel(video.segment.fromSec, video.segment.toSec)}</span>
          </span>
          <Button size="sm" icon={<Icon name="scissor" size={14} />} onClick={() => onChangeSegment(video)}>
            {copy.changeSegment}
          </Button>
        </div>
      )}
      <span className="vbd-vused" data-tone={video.usedBy.length ? 'linked' : undefined}>
        <Icon name="layer" size={13} />
        {video.usedBy.length ? copy.usedFixed(video.usedBy.map((u) => u.name).join(', ')) : copy.usedNone}
      </span>
      <span className="vbd-vlmore">
        <ActionMenu
          label={copy.moreAria(video.title)}
          items={[
            { id: 'segment', label: copy.changeSegment, icon: <Icon name="scissor" size={15} />, onSelect: () => onChangeSegment(video) },
            { id: 'remove', label: copy.removeItem, danger: true, icon: <Icon name="trash" size={15} />, onSelect: () => onRemove(video) },
          ]}
        />
      </span>
    </article>
  )
}

export function VideosPage({ videos, loading, error }: {
  videos: BackdropVideo[]
  loading: boolean
  error: unknown
}) {
  const copy = t.backdrop.tab.videos
  const [segmentOf, setSegmentOf] = useState<BackdropVideo | null>(null)
  const [segment, setSegment] = useState<BackdropSegment>({ fromSec: 0, toSec: 60 })
  const [busyId, setBusyId] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const openSegment = (video: BackdropVideo) => {
    setSegmentOf(video)
    setSegment({ fromSec: video.segment.fromSec, toSec: video.segment.toSec })
    setActionError(null)
  }

  const saveSegment = async () => {
    if (!segmentOf) return
    setBusyId(segmentOf.id)
    setActionError(null)
    try {
      await patchBackdropVideo(segmentOf.id, segment)
      await invalidate(['backdrop'])
      setSegmentOf(null)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusyId(null)
    }
  }

  const retry = async (video: BackdropVideo) => {
    setBusyId(video.id)
    setActionError(null)
    try {
      // Thêm lại cùng URL = tải lại khi lần trước hỏng (backend).
      await addBackdropVideo(video.url, video.segment)
      await invalidate(['backdrop'])
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusyId(null)
    }
  }

  const remove = async (video: BackdropVideo) => {
    setBusyId(video.id)
    setActionError(null)
    try {
      await removeBackdropVideo(video.id)
      await invalidate(['backdrop'])
    } catch (err) {
      // 409 in_use: câu máy chủ đã mang tên template đang dùng.
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusyId(null)
    }
  }

  return (
    <div className="vbd-scroll">
      <section className="vbd-pane" aria-label={copy.title}>
        <div className="vbd-vl-head">
          <h2 className="vbd-ch-name">{copy.title}</h2>
          <p className="vbd-hint">
            {copy.descPre} <b>{copy.descAlways}</b> {copy.descPost}
          </p>
        </div>
        {actionError && <Banner tone="error" title={actionError} />}
        {!!error && <Banner tone="error" title={t.library.loadFail} action={{ label: t.library.retry, onClick: () => void invalidate(keys.backdropVideos()) }} />}
        {loading && videos.length === 0 ? (
          <div className="vbd-grid vbd-grid--videos" aria-hidden="true">
            {Array.from({ length: 6 }, (_, i) => <div key={i} className="vbd-vlcard vbd-vlcard--skel" />)}
          </div>
        ) : (
          <div role="list" aria-label={copy.listAria} className="vbd-grid vbd-grid--videos">
            {videos.map((video) => (
              <VideoCard key={video.id} video={video} busyId={busyId}
                onChangeSegment={openSegment}
                onRetry={(v) => void retry(v)}
                onRemove={(v) => void remove(v)} />
            ))}
          </div>
        )}
        {!loading && videos.length === 0 && !error && (
          <p className="vbd-hint">{t.backdrop.tab.emptyHint}</p>
        )}
      </section>

      {segmentOf && (
        <Dialog
          title={copy.segmentDialogTitle}
          onClose={() => setSegmentOf(null)}
          width={460}
          footer={<>
            <Button size="sm" onClick={() => setSegmentOf(null)}>{t.library.cancel}</Button>
            <Button size="sm" variant="primary" disabled={busyId === segmentOf.id} onClick={() => void saveSegment()}>
              {busyId === segmentOf.id ? copy.segmentBusy : copy.segmentAction}
            </Button>
          </>}>
          <p className="vbd-hint">{copy.segmentDialogHelp}</p>
          <SegmentPicker value={segment} videoSec={segmentOf.durationSec} onChange={setSegment} />
          {actionError && <Banner tone="error" title={actionError} />}
        </Dialog>
      )}
    </div>
  )
}
