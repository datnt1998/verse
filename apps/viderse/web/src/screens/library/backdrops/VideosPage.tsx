import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
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
    <article role="listitem" className={`vbd-vlcard ${stylex.props(sx.vlcard).className}`}>
      <div className={`vbd-thumb ${stylex.props(sx.thumb).className}`}>
        {video.state === 'ready' && video.file && (
          <video className={`vbd-thumb-img ${stylex.props(sx.thumbImg).className}`} src={backdropVideoFileHref(video.id)} muted playsInline preload="metadata" />
        )}
        <span className={`vbd-kind ${stylex.props(sx.kind).className}`}>{kindName('video')}</span>
        {video.durationSec !== null && <span className={`vbd-dur ${stylex.props(sx.dur).className}`}>{timeRange(video.durationSec)}</span>}
      </div>
      <div className={`vbd-vlbody ${stylex.props(sx.vlbody).className}`}>
        <span className={`vbd-vtitle ${stylex.props(sx.vtitle).className}`}>{video.title}</span>
        <span className={`vbd-vsub num ${stylex.props(sx.vsubNum).className}`}>
          <span className={`vbd-dot ${stylex.props(sx.dot).className}`} style={{ background: platformColor(video.platform) }} />
          {copy.sub(platformName(video.platform), duration(video.durationSec), size)}
        </span>
      </div>
      {video.state === 'downloading' && (
        <div className={`vbd-dl ${stylex.props(sx.dl).className}`} role="group" aria-label={copy.downloadAria(video.title)}>
          <ProgressBar tone="info" value={Math.round((video.progress ?? 0) * 100)} />
          <span className={`vbd-dl-text num ${stylex.props(sx.dlTextNum).className}`}>{copy.downloadingEta(num(Math.round((video.progress ?? 0) * 100)))}</span>
        </div>
      )}
      {video.state === 'failed' && (
        <div className={`vbd-vlfail ${stylex.props(sx.vlfail).className}`}>
          <span className={`vbd-vlfail-text ${stylex.props(sx.vlfailText).className}`}>
            <Icon name="danger" size={14} />
            {video.errorDetail?.message ?? copy.failedTitle}
          </span>
          <ActionButton xstyle={[sx.vbdVlfailActionButton]} size="sm" icon={<Icon name="refresh" size={15} />} pending={busy} onClick={() => onRetry(video)}>{copy.retry}</ActionButton>
        </div>
      )}
      {video.state === 'ready' && (
        <div className={`vbd-segbox ${stylex.props(sx.segbox).className}`}>
          <span className={`vbd-segbox-text num ${stylex.props(sx.segboxTextNum).className}`}>
            <span className={`vbd-segbox-label ${stylex.props(sx.segboxLabel).className}`}>{copy.segmentLen(timeRange(segLen))}</span>
            <span className={`vbd-segbox-range ${stylex.props(sx.segboxRange).className}`}>{segmentLabel(video.segment.fromSec, video.segment.toSec)}</span>
          </span>
          <Button xstyle={[sx.vbdVlfailActionButton]} size="sm" icon={<Icon name="scissor" size={14} />} onClick={() => onChangeSegment(video)}>
            {copy.changeSegment}
          </Button>
        </div>
      )}
      <span className={`vbd-vused ${stylex.props(sx.vused, ((video.usedBy.length ? 'linked' : undefined) === "linked") && sx.vusedDataToneLinked).className}`} data-tone={video.usedBy.length ? 'linked' : undefined}>
        <Icon name="layer" size={13} />
        {video.usedBy.length ? copy.usedFixed(video.usedBy.map((u) => u.name).join(', ')) : copy.usedNone}
      </span>
      <span className={`vbd-vlmore ${stylex.props(sx.vlmore).className}`}>
        <ActionMenu triggerStyle={sx.touchTarget}
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
    <div className={`vbd-scroll ${stylex.props(sx.scroll).className}`}>
      <section className={`vbd-pane ${stylex.props(sx.pane).className}`} aria-label={copy.title}>
        <div className={`vbd-vl-head ${stylex.props(sx.vlHead).className}`}>
          <h2 className={`vbd-ch-name ${stylex.props(sx.chName).className}`}>{copy.title}</h2>
          <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>
            {copy.descPre} <b {...stylex.props(sx.vbdHintB)}>{copy.descAlways}</b> {copy.descPost}
          </p>
        </div>
        {actionError && <Banner tone="error" title={actionError} />}
        {!!error && <Banner tone="error" title={t.library.loadFail} action={{ label: t.library.retry, onClick: () => void invalidate(keys.backdropVideos()) }} />}
        {loading && videos.length === 0 ? (
          <div className={`vbd-grid vbd-grid--videos ${stylex.props(sx.gridGridVideos).className}`} aria-hidden="true">
            {Array.from({ length: 6 }, (_, i) => <div key={i} className={`vbd-vlcard vbd-vlcard--skel ${stylex.props(sx.vlcardVlcardSkel).className}`} />)}
          </div>
        ) : (
          <div role="list" aria-label={copy.listAria} className={`vbd-grid vbd-grid--videos ${stylex.props(sx.gridGridVideos).className}`}>
            {videos.map((video) => (
              <VideoCard key={video.id} video={video} busyId={busyId}
                onChangeSegment={openSegment}
                onRetry={(v) => void retry(v)}
                onRemove={(v) => void remove(v)} />
            ))}
          </div>
        )}
        {!loading && videos.length === 0 && !error && (
          <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{t.backdrop.tab.emptyHint}</p>
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
          <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.segmentDialogHelp}</p>
          <SegmentPicker value={segment} videoSec={segmentOf.durationSec} onChange={setSegment} />
          {actionError && <Banner tone="error" title={actionError} />}
        </Dialog>
      )}
    </div>
  )
}

const sx = stylex.create({
  vlcard: { position: "relative", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-10)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-10)", paddingLeft: "var(--vv-s-10)" },
  thumb: { position: "relative", aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", overflowX: "hidden", overflowY: "hidden", backgroundColor: line["--line-1"], backgroundImage: "none", display: "block" },
  thumbImg: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  kind: { position: "absolute", top: "var(--vv-s-6)", left: "var(--vv-s-6)", fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-6)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "var(--vv-scrim)", backgroundImage: "none", color: color["--color-fg"] },
  dur: { position: "absolute", bottom: "var(--vv-s-6)", right: "var(--vv-s-6)", fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-6)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "var(--vv-scrim)", backgroundImage: "none", color: color["--color-fg"] },
  vlbody: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-3)", columnGap: "var(--vv-s-3)" },
  vtitle: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.labelWeight, lineHeight: libraryVars.titleLineHeight, display: "-webkit-box", WebkitLineClamp: libraryVars.promptsTabPcardTextWebkitLineClamp, WebkitBoxOrient: "vertical", overflowX: "hidden", overflowY: "hidden" },
  vsubNum: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  dot: { width: "var(--vv-s-7)", height: "var(--vv-s-7)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, display: "inline-block" },
  dl: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  dlTextNum: { fontSize: libraryVars.microFontSize, color: color["--color-info"] },
  vlfail: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vlfailText: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-11)", color: color["--color-err"], lineHeight: libraryVars.backdropsVideosPageVlfailTextLineHeight },
  vbdVlfailActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  segbox: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-1"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-1"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-1"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-1"], borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", paddingTop: "var(--vv-s-7)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-7)", paddingLeft: "var(--vv-s-10)" },
  segboxTextNum: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  segboxLabel: { fontSize: "var(--vv-s-11)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"] },
  segboxRange: { fontSize: "var(--vv-s-11)", color: color["--color-fg-3"] },
  vused: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  vusedDataToneLinked: { color: color["--color-fg-2"] },
  vlmore: { position: "absolute", top: "var(--vv-s-6)", right: "var(--vv-s-6)" },
  scroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minHeight: { default: 0, "@media (max-width: 700px)": libraryVars.backdropsChannelPageScrollMinHeight }, overflowY: "auto", minWidth: 0, maxHeight: { default: null, "@media (max-width: 700px)": libraryVars.backdropsChannelPageScrollMaxHeight } },
  pane: { borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-16)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-18)", paddingLeft: "var(--vv-s-18)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-14)" },
  vlHead: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  chName: { fontSize: "var(--vv-s-18)", fontWeight: libraryVars.headingWeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, lineHeight: libraryVars.compactLineHeight },
  hint: { fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, display: "flex", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", alignItems: "flex-start" },
  vbdHintB: { color: color["--color-fg-2"] },
  gridGridVideos: { display: "grid", gridTemplateColumns: { default: libraryVars.backdropsVideosPageGridGridVideosGridTemplateColumns2, "@media (max-width: 700px)": libraryVars.promptEditorCompsGridTemplateColumns }, rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-12)" },
  vlcardVlcardSkel: { position: "relative", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-10)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-10)", paddingLeft: "var(--vv-s-10)", minHeight: "var(--vv-s-160)" },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
})
