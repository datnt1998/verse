import * as stylex from '@stylexjs/stylex'
import { color, line, tint } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Cột nguồn video nền (SourceRail): nhóm Kênh nền theo nền tảng + nhóm Video
// lẻ. Dùng ở hai chỗ theo artboard: tab Video nền (nhóm Video lẻ là một hàng
// tổng "Tất cả video lẻ") và hộp Gắn của Template (mỗi video lẻ một hàng).
import { Icon, Spinner } from '@noverse/ui'
import { t } from '../../../i18n/vi'
import { channelTint } from '../../../lib/channelTint'
import type { BackdropChannel, BackdropVideo } from '../../../api/backdrop'
import { initials, num, platformColor, platformName, segmentLabel } from './fmt'

/** Tổng video dùng được của kênh (sum counts). */
export function channelUsable(channel: BackdropChannel): number {
  return Object.values(channel.counts).reduce<number>((n, c) => n + (c ?? 0), 0)
}

/** Kênh đang đọc danh mục (lần đầu, hoặc còn trang chờ và không bị chặn). */
export function channelReading(channel: BackdropChannel): boolean {
  if (channel.listedAt === null) return true
  return channel.scanNextPage !== null && !channelPaused(channel)
}

/** Kênh/nền tảng đang bị chặn tạm (dòng phụ cam + icon đồng hồ). */
export function channelPaused(channel: BackdropChannel): boolean {
  return !!channel.pausedUntil && new Date(channel.pausedUntil).getTime() > Date.now()
}

function ChannelAvatar({ channel, size }: { channel: BackdropChannel; size: number }) {
  const name = channel.title ?? channel.handle
  if (channel.avatarUrl) {
    return <img className={`vbd-av ${stylex.props(sx.av).className}`} src={channel.avatarUrl} alt="" width={size} height={size} style={{ borderRadius: '50%' }} />
  }
  return (
    <span className={`vbd-av ${stylex.props(sx.av).className}`} aria-hidden="true" style={{ width: size, height: size, background: channelTint(channel.id) }}>
      {initials(name)}
    </span>
  )
}

function ChannelRow({ channel, current, linked, inDialog, onPick }: {
  channel: BackdropChannel
  current: boolean
  linked?: boolean
  inDialog?: boolean
  onPick: () => void
}) {
  const copy = t.backdrop.tab
  const reading = channelReading(channel)
  const paused = channelPaused(channel)
  const sub = reading
    ? copy.channelReading
    : copy.channelSub(num(channelUsable(channel)), channel.usedBy.length ? copy.channelLinked(num(channel.usedBy.length)) : copy.channelFree)
  return (
    <button type="button" className={`vbd-rail-row ${stylex.props(sx.railRow, inDialog && sx.dialogRow, current && sx.railRowAriaCurrent).className}`} aria-current={current || undefined} onClick={onPick}>
      <ChannelAvatar channel={channel} size={30} />
      <span className={`vbd-rail-text ${stylex.props(sx.railText).className}`}>
        <span className={`vbd-rail-name ${stylex.props(sx.railName).className}`}>{channel.title ?? channel.handle}</span>
        <span className={`vbd-rail-sub ${stylex.props(sx.railSub, ((paused ? 'paused' : reading ? 'reading' : undefined) === "paused") && sx.railSubDataTonePaused, ((paused ? 'paused' : reading ? 'reading' : undefined) === "reading") && sx.railSubDataToneReading).className}`} data-tone={paused ? 'paused' : reading ? 'reading' : undefined}>{sub}</span>
      </span>
      {reading && <Spinner size={12} color="var(--color-info)" />}
      {paused && <Icon xstyle={[sx.warnIc]} name="clock" size={14} label={copy.channelPaused} className="vbd-warn-ic" />}
    </button>
  )
}

function VideoRow({ video, current, linked, inDialog, onPick }: {
  video: BackdropVideo
  current: boolean
  linked?: boolean
  inDialog?: boolean
  onPick: () => void
}) {
  const busy = video.state === 'downloading'
  return (
    <button type="button" className={`vbd-rail-row ${stylex.props(sx.railRow, inDialog && sx.dialogRow, current && sx.railRowAriaCurrent).className}`} aria-current={current || undefined} onClick={onPick}>
      <span className={`vbd-rail-vthumb ${stylex.props(sx.railVthumb).className}`} aria-hidden="true"><Icon name="video-play" size={16} /></span>
      <span className={`vbd-rail-text ${stylex.props(sx.railText).className}`}>
        <span className={`vbd-rail-name ${stylex.props(sx.railName).className}`}>{video.title}</span>
        <span className={`vbd-rail-sub ${stylex.props(sx.railSub).className}`}>
          <span className={`vbd-dot ${stylex.props(sx.dot).className}`} style={{ background: platformColor(video.platform) }} />
          {platformName(video.platform)} · {segmentLabel(video.segment.fromSec, video.segment.toSec)}
        </span>
      </span>
      {busy && <Spinner size={12} color="var(--color-info)" />}
      {linked && <span className={`vbd-tag ${stylex.props(sx.tag).className}`} data-tone="linked">{t.backdrop.template.picker.linkedTag}</span>}
    </button>
  )
}

export function SourceRail({ channels, videos, selected, linkedIds, videoRows = 'summary', inDialog = false, onSelect, ariaLabel }: {
  channels: BackdropChannel[]
  videos: BackdropVideo[]
  /** 'videos' = trang Video lẻ; còn lại là id kênh đang chọn */
  selected: string
  /** id đã gắn vào template (hộp chọn) → thẻ "Đã gắn" */
  linkedIds?: ReadonlySet<string>
  /** 'summary' = một hàng Tất cả video lẻ (tab); 'items' = mỗi video một hàng (hộp chọn) */
  videoRows?: 'summary' | 'items'
  inDialog?: boolean
  onSelect: (id: string) => void
  ariaLabel?: string
}) {
  const copy = t.backdrop.tab
  const platforms = [...new Set(channels.map((channel) => channel.platform))]
  const anyLoading = videos.some((video) => video.state === 'downloading')
  return (
    <nav className={`vbd-rail ${stylex.props(sx.rail, inDialog && sx.dialogRail).className}`} aria-label={ariaLabel ?? copy.railAria}>
      <span className={`vbd-rail-head ${stylex.props(sx.railHead).className}`}>{copy.groupChannels}<span className={`num vbd-rail-count ${stylex.props(sx.numRailCount).className}`}>{num(channels.length)}</span></span>
      {platforms.map((platform) => {
        const list = channels.filter((channel) => channel.platform === platform)
        return (
          <div className={`vbd-rail-plat ${stylex.props(sx.railPlat).className}`} key={platform}>
            <span className={`vbd-rail-plathead ${stylex.props(sx.railPlathead).className}`}>
              <span className={`vbd-dot ${stylex.props(sx.dot).className}`} style={{ background: platformColor(platform) }} />
              {platformName(platform)}<span className={`num vbd-rail-count ${stylex.props(sx.numRailCount).className}`}>{num(list.length)}</span>
            </span>
            {list.map((channel) => (
              <ChannelRow key={channel.id} channel={channel} current={selected === channel.id}
                inDialog={inDialog} linked={linkedIds?.has(channel.id)} onPick={() => onSelect(channel.id)} />
            ))}
          </div>
        )
      })}
      <span className={`vbd-rail-line ${stylex.props(sx.railLine).className}`} aria-hidden="true" />
      <span className={`vbd-rail-head ${stylex.props(sx.railHead).className}`}>{copy.groupVideos}<span className={`num vbd-rail-count ${stylex.props(sx.numRailCount).className}`}>{num(videos.length)}</span></span>
      {videoRows === 'summary' ? (
        <button type="button" className={`vbd-rail-row ${stylex.props(sx.railRow, inDialog && sx.dialogRow, selected === 'videos' && sx.railRowAriaCurrent).className}`} aria-current={selected === 'videos' || undefined} onClick={() => onSelect('videos')}>
          <span className={`vbd-rail-vthumb ${stylex.props(sx.railVthumb).className}`} aria-hidden="true"><Icon name="video-play" size={16} /></span>
          <span className={`vbd-rail-text ${stylex.props(sx.railText).className}`}>
            <span className={`vbd-rail-name ${stylex.props(sx.railName).className}`}>{copy.videoGroupTitle}</span>
            <span className={`vbd-rail-sub ${stylex.props(sx.railSub).className}`}>{copy.videoGroupSub(num(videos.length))}</span>
          </span>
          {anyLoading && <Spinner size={12} color="var(--color-info)" />}
        </button>
      ) : (
        videos.map((video) => (
          <VideoRow key={video.id} video={video} current={selected === video.id}
            inDialog={inDialog} linked={linkedIds?.has(video.id)} onPick={() => onSelect(video.id)} />
        ))
      )}
    </nav>
  )
}

const sx = stylex.create({
  av: { borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, overflowX: "hidden", overflowY: "hidden", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], objectFit: "cover" },
  railRow: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-9)", columnGap: "var(--vv-s-9)", width: libraryVars.fullExtent, paddingTop: "var(--vv-s-7)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-7)", paddingLeft: "var(--vv-s-8)", borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", cursor: "pointer", backgroundColor: { default: "transparent", ":hover": line["--line-1"] }, backgroundImage: { default: "none", ":hover": "none" }, textAlign: "left", color: color["--color-fg"], fontFamily: "inherit", minHeight: { default: "var(--vv-s-44)", "@media (pointer: coarse)": "var(--vv-s-44)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, minWidth: { default: null, "@media (max-width: 767px)": 0 }, flexGrow: { default: null, "@media (max-width: 767px)": 0 }, flexShrink: { default: null, "@media (max-width: 767px)": 0 }, flexBasis: { default: null, "@media (max-width: 767px)": "auto" }, maxWidth: { default: null, "@media (max-width: 767px)": libraryVars.backdropsSourceRailRailRowMaxWidth }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": libraryVars.backdropsSourceRailRailRowOutlineOffset } },
  railRowAriaCurrent: { backgroundColor: tint["--tint-accent"], backgroundImage: "none" },
  railText: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  railName: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.labelWeight, lineHeight: libraryVars.compactLineHeight, whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis" },
  railSub: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"], display: "flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis" },
  railSubDataTonePaused: { color: color["--color-warn"] },
  railSubDataToneReading: { color: color["--color-info"] },
  warnIc: { color: color["--color-warn"], flexShrink: 0 },
  railVthumb: { width: "var(--vv-s-30)", height: "var(--vv-s-30)", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", flexShrink: 0, backgroundColor: line["--line-1"], backgroundImage: "none", color: color["--color-fg-3"], display: "flex", alignItems: "center", justifyContent: "center" },
  dot: { width: "var(--vv-s-7)", height: "var(--vv-s-7)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, display: "inline-block" },
  tag: { fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", flexShrink: 0, backgroundColor: tint["--tint-ok"], backgroundImage: "none", color: color["--color-ok"] },
  rail: { display: "flex", flexDirection: { default: "column", "@media (max-width: 767px)": "row" }, rowGap: { default: "var(--vv-s-2)", "@media (max-width: 767px)": "var(--vv-s-6)" }, columnGap: { default: "var(--vv-s-2)", "@media (max-width: 767px)": "var(--vv-s-6)" }, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: { default: "var(--vv-s-10)", "@media (max-width: 767px)": "var(--vv-s-8)" }, paddingRight: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-8)" }, paddingBottom: { default: "var(--vv-s-10)", "@media (max-width: 767px)": "var(--vv-s-8)" }, paddingLeft: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-8)" }, overflowY: { default: "auto", "@media (max-width: 767px)": "hidden" }, maxHeight: { default: libraryVars.fullExtent, "@media (max-width: 767px)": "none" }, overflowX: { default: null, "@media (max-width: 767px)": "auto" } },
  railHead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-11)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], paddingTop: { default: "var(--vv-s-6)", "@media (max-width: 767px)": "var(--vv-s-4)" }, paddingRight: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-6)" }, paddingBottom: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-4)" }, paddingLeft: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-6)" }, alignSelf: { default: null, "@media (max-width: 767px)": "center" }, whiteSpace: { default: null, "@media (max-width: 767px)": "nowrap" } },
  numRailCount: { color: color["--color-fg-4"], fontWeight: libraryVars.regularWeight },
  railPlat: { display: "flex", flexDirection: { default: "column", "@media (max-width: 767px)": "row" }, rowGap: { default: "var(--vv-s-2)", "@media (max-width: 767px)": "var(--vv-s-6)" }, columnGap: { default: "var(--vv-s-2)", "@media (max-width: 767px)": "var(--vv-s-6)" }, marginBottom: { default: "var(--vv-s-6)", "@media (max-width: 767px)": 0 } },
  railPlathead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-10)", color: color["--color-fg-3"], paddingTop: { default: "var(--vv-s-2)", "@media (max-width: 767px)": "var(--vv-s-2)" }, paddingRight: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-6)" }, paddingBottom: { default: "var(--vv-s-2)", "@media (max-width: 767px)": "var(--vv-s-2)" }, paddingLeft: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-6)" }, alignSelf: { default: null, "@media (max-width: 767px)": "center" }, whiteSpace: { default: null, "@media (max-width: 767px)": "nowrap" } },
  railLine: { borderTopWidth: { default: "var(--vv-s-1)", "@media (max-width: 767px)": 0 }, borderTopStyle: { default: "solid", "@media (max-width: 767px)": "none" }, borderTopColor: { default: line["--line-1"], "@media (max-width: 767px)": "currentColor" }, marginTop: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-6)" }, marginRight: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-2)" }, marginBottom: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-6)" }, marginLeft: { default: "var(--vv-s-8)", "@media (max-width: 767px)": "var(--vv-s-2)" } },
  dialogRail: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, borderTopWidth: 0, borderRightWidth: 0, borderBottomWidth: 0, borderLeftWidth: 0, borderTopStyle: "none", borderRightStyle: "none", borderBottomStyle: "none", borderLeftStyle: "none", borderTopColor: "currentColor", borderRightColor: "currentColor", borderBottomColor: "currentColor", borderLeftColor: "currentColor", borderTopLeftRadius: 0, borderTopRightRadius: 0, borderBottomRightRadius: 0, borderBottomLeftRadius: 0, paddingTop: { default: 0, "@media (max-width: 767px)": 0 }, paddingRight: { default: 0, "@media (max-width: 767px)": 0 }, paddingBottom: { default: 0, "@media (max-width: 767px)": 0 }, paddingLeft: { default: 0, "@media (max-width: 767px)": 0 }, maxHeight: "none", flexDirection: { default: "column", "@media (max-width: 767px)": "row" }, overflowX: { default: null, "@media (max-width: 767px)": "auto" } },
  dialogRow: { minWidth: { default: null, "@media (max-width: 767px)": libraryVars.sourcePickerRowMinWidth } },
})
