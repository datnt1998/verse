// Panel "Video đã chọn" ở cột phải khi bấm một thẻ: ảnh, điểm ×, popover "Vì
// sao?" (lượt xem / mức thường / ngưỡng ×3 / lúc đăng), huy hiệu phụ đề, nhóm
// "Làm cho kênh" + dòng kế hoạch + nút chính — chọn kênh và nút chính khoá
// (aria-disabled, vẫn focus được) vì chưa có kênh của bạn; lý do hiện một lần
// ngay dưới nút kèm link "Tạo kênh" (06-copy › Khám phá), cả hai điều khiển trỏ
// aria-describedby tới câu lý do. Link "Mở trên YouTube", nút "← Thông tin đối thủ".
import { useEffect, useId, useRef, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { exploreStyles } from './explore.stylex'
import { Icon, toast, useEscapeLayer } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { paths } from '../../router'
import { ActionButton, ChannelAvatar } from '../../components'
import type { Rival, Video } from '../../api/rivals'
import { enqueueClone, planClone, type ClonePlan } from '../../api/clone'
import { fmtDuration, fmtScore, fmtSubs, scoreText, videoMeta } from './fmt'
import { hideBrokenImage } from './VideoGrid'
import { channelTint } from '../../lib/channelTint'
import { ReasonPopover } from '../../components/reason/ReasonPopover'

/** Dòng "vào hàng chờ" dưới nhóm chọn kênh — kết quả plan §4 (ETA, trùng). */
function queueHintText(plan: ClonePlan | null, phase: 'loading' | 'ready' | 'failed'): { text: string; dupJob: string | null } {
  const te = t.screens.explore
  if (phase === 'loading') return { text: te.queuePlanLoading, dupJob: null }
  if (phase === 'failed' || !plan) return { text: te.queuePlan, dupJob: null }
  const dup = plan.duplicates[0]
  if (dup) return { text: te.queuePlanDup, dupJob: dup.jobId }
  const eta = plan.etaStart
  if (eta) {
    const time = new Date(eta).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    return { text: te.queuePlanEta(time), dupJob: null }
  }
  return { text: te.queuePlan, dupJob: null }
}

export function PreviewPanel({ video, rival, channels, onClose }: {
  video: Video
  rival: Rival | undefined
  /** kênh của tôi — có kênh thì chọn được kênh, nút làm video đổi lý do khoá */
  channels: { id: string; name: string; templateName?: string }[]
  onClose: () => void
}) {
  const tt = t.screens.explore
  const [whyOpen, setWhyOpen] = useState(false)
  const reasonAnchor = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!whyOpen) return
    const dismiss = (event: PointerEvent) => { if (!reasonAnchor.current?.contains(event.target as Node)) setWhyOpen(false) }
    document.addEventListener('pointerdown', dismiss)
    return () => document.removeEventListener('pointerdown', dismiss)
  }, [whyOpen])
  const [channelId, setChannelId] = useState<string | undefined>(channels[0]?.id)
  const [plan, setPlan] = useState<ClonePlan | null>(null)
  const [planPhase, setPlanPhase] = useState<'loading' | 'ready' | 'failed'>('loading')
  const [busy, setBusy] = useState(false)
  const [queuedVideo, setQueuedVideo] = useState<string | null>(null)
  const queued = queuedVideo === video.url
  useEscapeLayer(whyOpen, () => setWhyOpen(false))
  const reasonId = useId()

  // "Vào hàng chờ" gọi plan thật cho kênh đang chọn — đổi kênh đổi câu ETA.
  const effChannel = channelId ?? channels[0]?.id
  useEffect(() => {
    if (!effChannel || queued) { setPlan(null); setPlanPhase('ready'); return }
    const ctrl = new AbortController()
    setPlan(null)
    setPlanPhase('loading')
    planClone({ channelId: effChannel, urls: [video.url], priority: 'pick' }, ctrl.signal)
      .then((result) => { if (!ctrl.signal.aborted) { setPlan(result); setPlanPhase('ready') } })
      .catch(() => { if (!ctrl.signal.aborted) { setPlan(null); setPlanPhase('failed') } })
    return () => ctrl.abort()
  }, [effChannel, video.url, queued])

  const hint = queueHintText(plan, planPhase)
  const dupJob = hint.dupJob
  const duplicate = !!plan?.duplicates.length
  const canMake = planPhase === 'ready' && !!plan?.accepted.length && !duplicate && !!plan.template.id
  const selectedChannel = channels.find((channel) => channel.id === effChannel)
  const lockedReason = channels.length === 0 ? tt.lockedReason
    : queued ? tt.queuedDone
    : duplicate ? tt.queuePlanDup
    : plan && !plan.template.id ? t.screens.new.plan.blockedNoTemplate
    : !canMake ? tt.queuePlanLoading : undefined
  const make = async () => {
    if (!effChannel || busy || queued || !canMake) return
    setBusy(true)
    try {
      await enqueueClone({ channelId: effChannel, urls: [video.url], priority: 'pick' })
      setQueuedVideo(video.url)
      toast(t.screens.new.toasts.made, 'ok', { label: t.screens.new.toasts.openQueue, onClick: () => { location.hash = '#/queue' } })
    } catch {
      toast.err(t.screens.new.toasts.failed)
    } finally {
      setBusy(false)
    }
  }

  const dur = fmtDuration(video.durationSec)
  const hot = (video.score ?? 0) >= 3
  const meta = videoMeta(video.viewCount, video.publishedAt)
  const score = scoreText(video.score)


  return (
    <>
      <button type="button" className={`vx-back ${stylex.props(previewStyles.back, exploreStyles.touchTarget).className}`} onClick={onClose}>
        <Icon name="arrow-r" size={14} xstyle={previewStyles.backIcon} />
        {tt.rivalInfo}
      </button>
      <div className={`vx-pv ${stylex.props(previewStyles.pv).className}`}>
        <div className={`vx-pvthumb ${stylex.props(previewStyles.pvthumb).className}`}>
          <img {...stylex.props(previewStyles.image)} key={video.id} src={video.thumbnailUrl} width={320} height={180} alt="" onError={hideBrokenImage} />
          <span className={`vx-score ${stylex.props(exploreStyles.score, hot ? exploreStyles.scoreHot : exploreStyles.scoreLow).className}`}>{fmtScore(video.score)}</span>
          {dur && <span className={`vx-dur ${stylex.props(exploreStyles.dur, previewStyles.duration).className}`} >{dur}</span>}
        </div>
        <h2 className={`vx-pvtitle ${stylex.props(previewStyles.pvtitle).className}`}>{video.title}</h2>
        <span className={`vx-pvsub ${stylex.props(previewStyles.pvsub).className}`}>{rival ? `${rival.title} · ${meta}` : meta}</span>
        <div ref={reasonAnchor} className={`vx-pvscore ${stylex.props(previewStyles.pvscore).className}`}>
          <span className={`vx-score ${stylex.props(exploreStyles.score, hot ? exploreStyles.scoreHot : exploreStyles.scoreLow, previewStyles.scoreStatic).className}`}>{fmtScore(video.score)}</span>
          <span className={`vx-pvscore__text ${stylex.props(previewStyles.pvscoreText).className}`}>{tt.scoreTimes(score)}</span>
          {video.scoreReason && (
            <button type="button" className={`vx-why ${stylex.props(previewStyles.why, whyOpen && previewStyles.whyExpanded, exploreStyles.touchTarget).className}`} aria-haspopup="dialog" aria-expanded={whyOpen} onClick={() => setWhyOpen(!whyOpen)}>
              <Icon name="info" size={13} />
              {tt.why}
            </button>
          )}
          {whyOpen && video.scoreReason && <div {...stylex.props(previewStyles.reasonPopover)}><ReasonPopover label={tt.whyTitle} reason={video.scoreReason} /></div>}
        </div>
        {video.hasCaptions === true && (
          <span className={`vx-subsbadge ${stylex.props(previewStyles.subsbadge).className}`}><Icon name="subtitle" size={13} />{tt.hasSubs}</span>
        )}
      </div>
      <div className={`vx-makegroup ${stylex.props(previewStyles.makegroup).className}`} role="radiogroup" aria-label={tt.makeChannel}>
        <span className={`vx-makehead ${stylex.props(previewStyles.makehead).className}`}><Icon name="channel" size={14} />{tt.makeChannel}</span>
        {channels.length === 0 ? (
          <div className={`vx-makeopt ${stylex.props(previewStyles.makeopt, previewStyles.makeoptDisabled, exploreStyles.touchTarget).className}`} role="radio" aria-checked="false" aria-disabled="true" tabIndex={0} aria-describedby={reasonId}>
            <span className={`vx-makeopt__text ${stylex.props(previewStyles.makeoptText).className}`}>
              <span className={`vx-makeopt__name ${stylex.props(previewStyles.makeoptName).className}`}>{tt.notLinked}</span>
            </span>
          </div>
        ) : channels.map((channel) => (
          <button
            key={channel.id}
            type="button"
            role="radio"
            aria-checked={effChannel === channel.id}
            tabIndex={0}
            className={`vx-makeopt ${stylex.props(previewStyles.makeopt, effChannel === channel.id && previewStyles.makeoptSelected, exploreStyles.touchTarget).className}`}
            onClick={() => setChannelId(channel.id)}
          >
            <ChannelAvatar name={channel.name} size={26} color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }} style={{ fontSize: 10 }} />
            <span className={`vx-makeopt__text ${stylex.props(previewStyles.makeoptText).className}`}>
              <span className={`vx-makeopt__name ${stylex.props(previewStyles.makeoptName, effChannel === channel.id && previewStyles.makeoptNameSelected).className}`}>{channel.name}</span>
              {channel.templateName && <span className={`vx-makeopt__sub ${stylex.props(previewStyles.makeoptSub).className}`}>{channel.templateName}</span>}
            </span>
            <span className={`vx-makeopt__radio ${stylex.props(previewStyles.makeoptRadio, effChannel === channel.id && previewStyles.radioSelected).className}`} aria-hidden="true"><span {...stylex.props(previewStyles.radioDot, effChannel === channel.id && previewStyles.radioDotSelected)} /></span>
          </button>
        ))}
      </div>
      <span className={`vx-queuehint ${stylex.props(previewStyles.queuehint).className}`}><Icon name="clock" size={14} xstyle={previewStyles.queueIcon} />{hint.text}
        {dupJob && <> <a href={`#${paths.workbench({ video: dupJob })}`}>{tt.queuePlanOpen}</a></>}
      </span>
      <ActionButton
        variant="primary"
        className="vx-preview-make" xstyle={[exploreStyles.touchTarget, previewStyles.make]}
        size="lg"
        disabled={busy}
        lockedReason={lockedReason}
        aria-describedby={lockedReason ? reasonId : undefined}
        onClick={make}
      >
        <span {...stylex.props(previewStyles.makeLabel)}><Icon name="video-add" size={18} /><span className={`vx-preview-make__text ${stylex.props(previewStyles.previewMakeText).className}`}>
          <span {...stylex.props(previewStyles.makeTitle)}>{queued ? tt.queuedLabel : tt.makeVideo}</span>
          {selectedChannel && <span {...stylex.props(previewStyles.makeSubtitle)}>{tt.makeSubtitle(selectedChannel.name)}</span>}
        </span></span>
      </ActionButton>
      {lockedReason && <p className={`vx-locknote ${stylex.props(previewStyles.locknote).className}`}>
        <span id={reasonId}>{lockedReason}</span>
        {channels.length === 0 && <> <a {...stylex.props(previewStyles.lockLink)} href={`#${paths.channelNew()}`}>{tt.createChannel}</a></>}
      </p>}
      <a href={video.url} target="_blank" rel="noreferrer" className={`vx-linkbtn ${stylex.props(previewStyles.linkbtn, exploreStyles.touchTarget).className}`}>
        <Icon name="link" size={14} />
        {tt.openYouTube}
      </a>
    </>
  )
}

export const previewStyles = stylex.create({
  "back": { alignSelf: "flex-start", height: "30px", flexShrink: 0, padding: `0 ${s["--vv-s-8"]}`, display: "inline-flex", alignItems: "center", gap: s["--vv-s-6"], borderRadius: r["--vv-r-8"], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover)" }, color: color["--color-fg-3"], fontFamily: "inherit", fontSize: "12.5px", fontWeight: "inherit", lineHeight: 1.55, fontStyle: "inherit", fontVariant: "inherit", fontStretch: "inherit", cursor: "pointer" },
  "backIcon": { "transform": "rotate(180deg)" },
  "pv": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-10"] },
  "pvthumb": { position: "relative", aspectRatio: "16 / 9", borderRadius: r["--vv-r-12"], overflow: "hidden", borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-3"], backgroundColor: color["--color-card"] },
  "image": { "width": "100%", "height": "100%", "objectFit": "cover", "display": "block" },
  "pvtitle": { "margin": 0, "fontSize": "15px", "fontWeight": 600, "lineHeight": 1.4 },
  "pvsub": { "fontSize": "12px", "color": "var(--vv-fg-note)", "lineHeight": 1.5 },
  "pvscore": { "position": "relative", "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "flexWrap": "wrap" },
  "scoreStatic": { "position": "static" },
  "pvscoreText": { "fontSize": "12px", "color": color["--color-fg-3"] },
  "why": { display: "inline-flex", alignItems: "center", gap: s["--vv-s-4"], height: s["--vv-s-22"], padding: `0 ${s["--vv-s-2"]}`, borderWidth: 0, borderStyle: "none", borderColor: "currentColor", backgroundColor: "transparent", color: { default: color["--color-fg-3"], ":hover": color["--color-fg"] }, fontFamily: "inherit", fontSize: "12px", fontWeight: "inherit", lineHeight: "inherit", fontStyle: "inherit", fontVariant: "inherit", fontStretch: "inherit", whiteSpace: "nowrap", textDecoration: "underline dotted var(--vv-line-35)", textUnderlineOffset: "3px", cursor: "pointer" },
  "whyExpanded": { "color": color["--color-fg"] },
  "reasonPopover": { position: "absolute", top: "100%", right: 0, width: "320px", maxWidth: "100%", zIndex: 4, paddingTop: s["--vv-s-6"] },
  "subsbadge": { alignSelf: "flex-start", height: s["--vv-s-22"], padding: `0 ${s["--vv-s-8"]}`, display: "inline-flex", alignItems: "center", gap: s["--vv-s-5"], borderRadius: r["--vv-r-6"], backgroundColor: "var(--vv-ok-7)", color: color["--color-ok"], fontSize: "11.5px" },
  "makegroup": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-6"] },
  "makehead": { "margin": 0, "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "fontSize": "12.5px", "fontWeight": 500, "color": color["--color-fg-3"] },
  "makeopt": { display: "flex", alignItems: "center", gap: s["--vv-s-10"], padding: `${s["--vv-s-7"]} ${s["--vv-s-10"]}`, borderRadius: r["--vv-r-10"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"], backgroundColor: color["--color-surface-1"], width: "100%", fontFamily: "inherit", fontSize: "inherit", fontWeight: "inherit", lineHeight: "inherit", fontStyle: "inherit", fontVariant: "inherit", fontStretch: "inherit", color: "inherit", textAlign: "left", cursor: "pointer" },
  "makeoptDisabled": { "opacity": 0.7 },
  "makeoptSelected": { borderColor: "var(--vv-line-30)", backgroundColor: color["--color-surface-6"] },
  "makeoptNameSelected": { "color": color["--color-fg"] },
  "makeoptText": { "flex": "1 1 auto", "minWidth": 0, "display": "flex", "flexDirection": "column" },
  "makeoptName": { "fontSize": "13px", "fontWeight": 500, "color": color["--color-fg-3"], "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "makeoptSub": { "fontSize": "11.5px", "color": "var(--vv-fg-note)", "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "makeoptRadio": { width: "16px", height: "16px", flex: "0 0 auto", borderWidth: "1.5px", borderStyle: "solid", borderColor: line["--line-5"], borderRadius: "50%", display: "flex", alignItems: "center", justifyContent: "center" },
  "radioDot": { "width": "8px", "height": "8px", "borderRadius": "50%" },
  "radioSelected": { "borderColor": color["--color-fg"] },
  "radioDotSelected": { backgroundColor: color["--color-fg"] },
  "make": { flexShrink: 0, height: "auto", minHeight: "46px", paddingBlock: s["--vv-s-6"], paddingInline: s["--vv-s-12"], borderWidth: 0, borderStyle: "none", borderColor: "currentColor", borderRadius: r["--vv-r-10"] },
  "makeLabel": { "gap": s["--vv-s-10"], "display": "inline-flex", "alignItems": "center" },
  "previewMakeText": { "minWidth": 0, "display": "flex", "flexDirection": "column", "alignItems": "flex-start", "lineHeight": 1.3 },
  "makeTitle": { "fontSize": "13.5px", "fontWeight": 600 },
  "makeSubtitle": { "maxWidth": "100%", "fontSize": "12px", "fontWeight": 400, "color": `color-mix(in srgb, ${color["--color-bg"]} 82%, transparent)`, "overflow": "hidden", "textOverflow": "ellipsis", "whiteSpace": "nowrap" },
  "queuehint": { "display": "flex", "gap": s["--vv-s-8"], "fontSize": "12px", "color": color["--color-fg-3"], "lineHeight": 1.5 },
  "queueIcon": { "marginTop": s["--vv-s-2"], "color": "var(--vv-fg-note)", "flex": "0 0 auto" },
  "locknote": { "margin": 0, "fontSize": "12px", "lineHeight": 1.5, "color": "var(--vv-fg-note)" },
  "lockLink": { "color": color["--color-accent"], "whiteSpace": "nowrap" },
  "linkbtn": { height: "34px", flexShrink: 0, borderRadius: r["--vv-r-10"], fontSize: "13px", fontWeight: 400, display: "inline-flex", alignItems: "center", justifyContent: "center", gap: s["--vv-s-7"], color: color["--color-fg"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-4"], backgroundColor: { default: "transparent", ":hover": "var(--vv-hover)" }, textDecoration: "none", whiteSpace: "nowrap" },
  "duration": { "position": "absolute", "right": s["--vv-s-8"], "bottom": s["--vv-s-7"] },
})
