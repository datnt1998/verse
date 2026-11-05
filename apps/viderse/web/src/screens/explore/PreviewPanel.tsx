// Selected source preview. Creation hands the actual selection to the shell composer.
import { useEffect, useId, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { exploreStyles } from './explore.stylex'
import { Button, Icon, RadioCard } from '@noverse/ui'
import { t } from '../../i18n/explore'
import { paths } from '../../router'
import { ActionButton, ChannelAvatar } from '../../components'
import type { Rival, Video, VideoPreview } from '../../api/rivals'
import { getRivalPreview } from '../../api/rivals'
import { cloneTarget, planClone, type ClonePlan } from '../../api/clone'
import { openQuickCreate } from '../new/quickCreate'
import { fmtDuration, fmtScore, fmtChapterTime, scoreText, videoMeta } from './fmt'
import { hideBrokenImage } from './VideoGrid'
import { stageColor, usageHref, usageSubline } from './status'
import { channelTint } from '../../lib/channelTint'

/** Dòng "vào hàng chờ" dưới nhóm chọn kênh — kết quả plan §4 (ETA, trùng). */
function queueHintText(plan: ClonePlan | null, phase: 'loading' | 'ready' | 'failed'): { text: string; dupJob: string | null } {
  const te = t.screens.explore
  if (phase === 'loading') return { text: te.queuePlanLoading, dupJob: null }
  if (phase === 'failed' || !plan) return { text: te.queuePlan, dupJob: null }
  const dup = plan.duplicates[0]
  if (dup) return { text: te.queuePlanDup, dupJob: cloneTarget(dup) }
  const eta = plan.etaStart
  if (eta) {
    const time = new Date(eta).toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
    return { text: te.queuePlanEta(time), dupJob: null }
  }
  return { text: te.queuePlan, dupJob: null }
}

/** Giờ đăng dạng "26/10 19:00" cho dòng giải thích Tự lấy. */
function fmtPickTime(iso: string): string {
  return new Date(iso).toLocaleString('vi-VN', { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function PreviewPanel({ video, rival, channels, onClose }: {
  video: Video
  rival: Rival | undefined
  /** kênh của tôi — có kênh thì chọn được kênh, nút làm video đổi lý do khoá */
  channels: { id: string; name: string; templateName?: string }[]
  onClose: () => void
}) {
  const tt = t.screens.explore
  const [channelId, setChannelId] = useState<string | undefined>(channels[0]?.id)
  const [plan, setPlan] = useState<ClonePlan | null>(null)
  const [planPhase, setPlanPhase] = useState<'loading' | 'ready' | 'failed'>('loading')
  const [chapters, setChapters] = useState<VideoPreview | null>(null)
  const [chaptersPhase, setChaptersPhase] = useState<'loading' | 'ready' | 'failed'>('loading')
  const queued = false
  const reasonId = useId()

  // Chương thật: chỉ hỏi khi panel mở, không quét theo thẻ. Đóng panel/đổi
  // video là huỷ request — máy chủ tự huỷ lệnh dò theo ctx.
  useEffect(() => {
    const ctrl = new AbortController()
    setChapters(null)
    setChaptersPhase('loading')
    getRivalPreview(video.channelId, video.id, ctrl.signal)
      .then((result) => { if (!ctrl.signal.aborted) { setChapters(result); setChaptersPhase('ready') } })
      .catch(() => { if (!ctrl.signal.aborted) { setChaptersPhase('failed') } })
    return () => ctrl.abort()
  }, [video.id, video.channelId])

  // Kênh đã làm/đang làm video này bị khoá; mặc định chọn kênh trống đầu tiên.
  const usageByChannel = new Map((video.usage ?? []).map((u) => [u.channelId, u]))
  const firstOpen = channels.find((channel) => !usageByChannel.has(channel.id))
  const effChannel = channelId && !usageByChannel.has(channelId) ? channelId : firstOpen?.id

  // "Vào hàng chờ" gọi plan thật cho kênh đang chọn — đổi kênh đổi câu ETA.
  useEffect(() => {
    if (!effChannel || queued) { setPlan(null); setPlanPhase('ready'); return }
    const ctrl = new AbortController()
    setPlan(null)
    setPlanPhase('loading')
    planClone({ channelId: effChannel, urls: [video.url] }, ctrl.signal)
      .then((result) => { if (!ctrl.signal.aborted) { setPlan(result); setPlanPhase('ready') } })
      .catch(() => { if (!ctrl.signal.aborted) { setPlan(null); setPlanPhase('failed') } })
    return () => ctrl.abort()
  }, [effChannel, video.url, queued])

  const hint = queueHintText(plan, planPhase)
  const dupJob = hint.dupJob
  const selectedChannel = channels.find((channel) => channel.id === effChannel)
  const lockedReason = channels.length === 0 ? tt.lockedReason : undefined
  const make = () => {
    openQuickCreate({ channelId: effChannel, urls: [video.url], fromSelection: true })
  }

  const dur = fmtDuration(video.durationSec)
  const hot = (video.score ?? 0) >= 3
  const meta = videoMeta(video.viewCount, video.publishedAt)
  const score = scoreText(video.score)


  return (
    <>
      <Button variant="quiet" size="sm" onClick={onClose} icon={<Icon name="arrow-r" size={14} xstyle={previewStyles.backIcon} />}>{tt.rivalInfo}</Button>
      <div className={`vx-pv ${stylex.props(previewStyles.pv).className}`}>
        <div className={`vx-pvthumb ${stylex.props(previewStyles.pvthumb).className}`}>
          <img {...stylex.props(previewStyles.image)} key={video.id} src={video.thumbnailUrl} width={320} height={180} alt="" onError={hideBrokenImage} />
          <span className={`vx-score ${stylex.props(exploreStyles.score, hot ? exploreStyles.scoreHot : exploreStyles.scoreLow).className}`}>{fmtScore(video.score)}</span>
          {dur && <span className={`vx-dur ${stylex.props(exploreStyles.dur, previewStyles.duration).className}`} >{dur}</span>}
        </div>
        <h2 className={`vx-pvtitle ${stylex.props(previewStyles.pvtitle).className}`}>{video.title}</h2>
        <span className={`vx-pvsub ${stylex.props(previewStyles.pvsub).className}`}>{rival ? `${rival.title} · ${meta}` : meta}</span>
        <div className={`vx-pvscore ${stylex.props(previewStyles.pvscore).className}`}>
          <span className={`vx-score ${stylex.props(exploreStyles.score, hot ? exploreStyles.scoreHot : exploreStyles.scoreLow, previewStyles.scoreStatic).className}`}>{fmtScore(video.score)}</span>
          <span className={`vx-pvscore__text ${stylex.props(previewStyles.pvscoreText).className}`}>{tt.scoreTimes(score)}</span>
        </div>
        {video.hasCaptions === true && (
          <span className={`vx-subsbadge ${stylex.props(previewStyles.subsbadge).className}`}><Icon name="subtitle" size={13} />{tt.hasSubs}</span>
        )}
        {video.pickNote && (
          <p className={`vx-picknote ${stylex.props(previewStyles.picknote).className}`}>
            <Icon name="clock" size={13} xstyle={previewStyles.pickIcon} />
            <span>{tt.pickNoteText(fmtPickTime(video.pickNote.publishedAt), video.pickNote.channelName, fmtPickTime(video.pickNote.baselineAt))}</span>
          </p>
        )}
        <div className={`vx-chapters ${stylex.props(previewStyles.chapters).className}`}>
          <span className={`vx-chapters__head ${stylex.props(previewStyles.chaptersHead).className}`}>
            <Icon name="clock" size={14} />
            {chaptersPhase === 'ready' && chapters && chapters.chapters.length > 0
              ? tt.chaptersCount(String(chapters.chapters.length))
              : tt.chaptersTitle}
          </span>
          {chaptersPhase === 'loading' ? (
            <p className={`vx-chapters__empty ${stylex.props(previewStyles.chaptersEmpty).className}`}>{tt.chaptersLoading}</p>
          ) : chaptersPhase === 'failed' ? (
            <p className={`vx-chapters__empty ${stylex.props(previewStyles.chaptersEmpty).className}`}>{tt.chaptersError}</p>
          ) : chapters && chapters.chapters.length > 0 ? (
            <ol className={`vx-chapters__list ${stylex.props(previewStyles.chaptersList).className}`}>
              {chapters.chapters.map((chapter) => (
                <li key={`${chapter.startSec}-${chapter.title}`} className={`vx-chapters__row ${stylex.props(previewStyles.chapterRow).className}`}>
                  <span className={`vx-chapters__time ${stylex.props(previewStyles.chapterTime).className}`}>{fmtChapterTime(chapter.startSec)}</span>
                  <span className={`vx-chapters__name ${stylex.props(previewStyles.chapterName).className}`}>{chapter.title}</span>
                </li>
              ))}
            </ol>
          ) : (
            <p className={`vx-chapters__empty ${stylex.props(previewStyles.chaptersEmpty).className}`}>{tt.chaptersEmpty}</p>
          )}
        </div>
      </div>
      <div className={`vx-makegroup ${stylex.props(previewStyles.makegroup).className}`} role="radiogroup" aria-label={tt.makeChannel}>
        <span className={`vx-makehead ${stylex.props(previewStyles.makehead).className}`}><Icon name="channel" size={14} />{tt.makeChannel}</span>
        {channels.length === 0 ? (
          <RadioCard checked={false} disabled label={tt.notLinked} aria-describedby={reasonId} onChange={() => {}} />
        ) : channels.map((channel) => {
          const usage = usageByChannel.get(channel.id)
          if (usage) {
            return (
              <div key={channel.id} className={`vx-makeopt vx-makeopt--used ${stylex.props(previewStyles.usedRow).className}`} aria-disabled="true">
                <span aria-hidden="true" className={stylex.props(previewStyles.usedDot).className} style={{ backgroundColor: stageColor(usage.stage) }} />
                <ChannelAvatar name={channel.name} size={26} color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }} fontSize={10} />
                <span className={`vx-makeopt__text ${stylex.props(previewStyles.makeoptText).className}`}>
                  <span className={`vx-makeopt__name ${stylex.props(previewStyles.makeoptName).className}`}>{channel.name}</span>
                  <span className={`vx-makeopt__sub ${stylex.props(previewStyles.makeoptSub).className}`} style={{ color: stageColor(usage.stage) }}>
                    {usageSubline(usage)}
                  </span>
                </span>
                <a href={usageHref(usage)} className={`vx-makeopt__open ${stylex.props(previewStyles.usedOpen).className}`} aria-label={`${tt.usageOpen} ${channel.name}`}>
                  {tt.usageOpen} <Icon name="arrow-r" size={12} />
                </a>
              </div>
            )
          }
          return (
            <RadioCard
              key={channel.id}
              checked={effChannel === channel.id}
              tabIndex={effChannel === channel.id ? 0 : -1}
              onChange={() => setChannelId(channel.id)}
            >
              <ChannelAvatar name={channel.name} size={26} color={{ bg: channelTint(channel.id), ring: channelTint(channel.id) }} fontSize={10} />
              <span className={`vx-makeopt__text ${stylex.props(previewStyles.makeoptText).className}`}>
                <span className={`vx-makeopt__name ${stylex.props(previewStyles.makeoptName).className}`}>{channel.name}</span>
                {channel.templateName && <span className={`vx-makeopt__sub ${stylex.props(previewStyles.makeoptSub).className}`}>{channel.templateName}</span>}
              </span>
            </RadioCard>
          )
        })}
      </div>
      <span className={`vx-queuehint ${stylex.props(previewStyles.queuehint).className}`}><Icon name="clock" size={14} xstyle={previewStyles.queueIcon} />{hint.text}
        {dupJob && <a href={`#${dupJob}`}>{tt.queuePlanOpen}</a>}
      </span>
      <ActionButton
        variant="primary"
        size="lg"
        xstyle={previewStyles.makeBtn}
        disabled={false}
        lockedReason={lockedReason}
        aria-describedby={lockedReason ? reasonId : undefined}
        aria-label={selectedChannel ? tt.makeVideoForAria(selectedChannel.name) : undefined}
        onClick={make}
      >
        <Icon name="video-add" size={18} />
        <span {...stylex.props(previewStyles.makeBtnText)}>
          <span>{queued ? tt.queuedLabel : tt.makeVideo}</span>
          {selectedChannel && !queued && <span {...stylex.props(previewStyles.makeBtnSub)}>{tt.makeSubtitle(selectedChannel.name)}</span>}
        </span>
      </ActionButton>
      {lockedReason && <p className={`vx-locknote ${stylex.props(previewStyles.locknote).className}`}>
        <span id={reasonId}>{lockedReason}</span>
        {channels.length === 0 && <> <a href={`#${paths.channelNew()}`}>{tt.createChannel}</a></>}
      </p>}
      <a href={video.url} target="_blank" rel="noreferrer" className={`vx-yt ${stylex.props(previewStyles.yt).className}`}>
        <Icon name="link" size={14} />{tt.openYouTube}
      </a>
    </>
  )
}

export const previewStyles = stylex.create({
  "backIcon": { "transform": "rotate(180deg)" },
  "pv": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-10"] },
  "pvthumb": { position: "relative", aspectRatio: "16 / 9", borderRadius: r["--vv-r-12"], overflow: "hidden", borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-3"], backgroundColor: color["--color-card"] },
  "image": { "width": "100%", "height": "100%", "objectFit": "cover", "display": "block" },
  "pvtitle": { "margin": 0, "fontSize": "15px", "fontWeight": 600, "lineHeight": 1.4 },
  "pvsub": { "fontSize": "12px", "color": "var(--vv-fg-note)", "lineHeight": 1.5 },
  "pvscore": { "position": "relative", "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "flexWrap": "wrap" },
  "scoreStatic": { "position": "static" },
  "pvscoreText": { "fontSize": "12px", "color": color["--color-fg-3"] },
  "subsbadge": { alignSelf: "flex-start", height: s["--vv-s-22"], padding: `0 ${s["--vv-s-8"]}`, display: "inline-flex", alignItems: "center", gap: s["--vv-s-5"], borderRadius: r["--vv-r-6"], backgroundColor: "var(--vv-ok-7)", color: color["--color-ok"], fontSize: "11.5px" },
  "makegroup": { "display": "flex", "flexDirection": "column", "gap": s["--vv-s-6"] },
  "makehead": { "margin": 0, "display": "flex", "alignItems": "center", "gap": s["--vv-s-8"], "fontSize": "12.5px", "fontWeight": 500, "color": color["--color-fg-3"] },
  "makeoptText": { "flex": "1 1 auto", "minWidth": 0, "display": "flex", "flexDirection": "column" },
  "makeoptName": { "fontSize": "13px", "fontWeight": 500, "color": color["--color-fg-3"], "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "makeoptSub": { "fontSize": "11.5px", "color": "var(--vv-fg-note)", "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "queuehint": { "display": "flex", "gap": s["--vv-s-8"], "fontSize": "12px", "color": color["--color-fg-3"], "lineHeight": 1.5 },
  "queueIcon": { "marginTop": s["--vv-s-2"], "color": "var(--vv-fg-note)", "flex": "0 0 auto" },
  "picknote": { margin: 0, display: "flex", gap: s["--vv-s-6"], fontSize: "12px", lineHeight: 1.5, color: color["--color-fg-3"] },
  "pickIcon": { marginTop: s["--vv-s-2"], flex: "0 0 auto", color: "var(--vv-fg-note)" },
  "chapters": { display: "flex", flexDirection: "column", gap: s["--vv-s-5"] },
  "chaptersHead": { display: "flex", alignItems: "center", gap: s["--vv-s-6"], fontSize: "12.5px", fontWeight: 500, color: color["--color-fg-3"] },
  "chaptersList": { listStyle: "none", margin: 0, padding: 0, display: "flex", flexDirection: "column", gap: s["--vv-s-3"] },
  "chapterRow": { display: "flex", alignItems: "baseline", gap: s["--vv-s-8"], minWidth: 0 },
  "chapterTime": { flex: "0 0 auto", fontFamily: "var(--font-mono)", fontSize: "11.5px", color: "var(--vv-fg-note)", fontVariantNumeric: "tabular-nums" },
  "chapterName": { minWidth: 0, fontSize: "12.5px", lineHeight: 1.45, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" },
  "chaptersEmpty": { margin: 0, fontSize: "12px", lineHeight: 1.5, color: "var(--vv-fg-note)" },
  "usedRow": { display: "flex", alignItems: "center", gap: s["--vv-s-8"], minHeight: s["--vv-s-44"], padding: `0 ${s["--vv-s-8"]}`, borderRadius: r["--vv-r-9"], borderWidth: "1px", borderStyle: "solid", borderColor: line["--line-2"], backgroundColor: "var(--kit-wash-4)" },
  "usedDot": { width: "7px", height: "7px", borderRadius: "50%", flex: "0 0 auto" },
  "usedOpen": { flex: "0 0 auto", display: "inline-flex", alignItems: "center", gap: s["--vv-s-3"], fontSize: "12px", color: color["--color-fg-3"], textDecorationLine: "none" },
  "locknote": { "margin": 0, "fontSize": "12px", "lineHeight": 1.5, "color": "var(--vv-fg-note)" },
  /* Artboard Explore › preview: CTA rộng theo nội dung, hai dòng "Làm video / cho <kênh>". */
  "makeBtn": { "alignSelf": "flex-start", "height": "auto", "minHeight": "44px", "paddingBlock": s["--vv-s-6"] },
  "makeBtnText": { "minWidth": 0, "display": "flex", "flexDirection": "column", "alignItems": "flex-start", "lineHeight": 1.3 },
  "makeBtnSub": { "maxWidth": "100%", "fontSize": "12px", "fontWeight": 500, "color": "rgba(11,11,13,0.82)", "whiteSpace": "nowrap", "overflow": "hidden", "textOverflow": "ellipsis" },
  "yt": { "alignSelf": "flex-start", "height": "34px", "padding": `0 ${s["--vv-s-12"]}`, "display": "inline-flex", "alignItems": "center", "gap": s["--vv-s-6"], "borderRadius": r["--vv-r-9"], "borderWidth": "1px", "borderStyle": "solid", "borderColor": line["--line-4"], "color": color["--color-fg-2"], "textDecorationLine": "none", "fontSize": "13px" },
  "duration": { "position": "absolute", "right": s["--vv-s-8"], "bottom": s["--vv-s-7"] },
})
