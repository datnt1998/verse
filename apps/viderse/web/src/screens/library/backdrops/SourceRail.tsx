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
    return <img className="vbd-av" src={channel.avatarUrl} alt="" width={size} height={size} style={{ borderRadius: '50%' }} />
  }
  return (
    <span className="vbd-av" aria-hidden="true" style={{ width: size, height: size, background: channelTint(channel.id) }}>
      {initials(name)}
    </span>
  )
}

function ChannelRow({ channel, current, linked, onPick }: {
  channel: BackdropChannel
  current: boolean
  linked?: boolean
  onPick: () => void
}) {
  const copy = t.backdrop.tab
  const reading = channelReading(channel)
  const paused = channelPaused(channel)
  const sub = reading
    ? copy.channelReading
    : copy.channelSub(num(channelUsable(channel)), channel.usedBy.length ? copy.channelLinked(num(channel.usedBy.length)) : copy.channelFree)
  return (
    <button type="button" className="vbd-rail-row" aria-current={current || undefined} onClick={onPick}>
      <ChannelAvatar channel={channel} size={30} />
      <span className="vbd-rail-text">
        <span className="vbd-rail-name">{channel.title ?? channel.handle}</span>
        <span className="vbd-rail-sub" data-tone={paused ? 'paused' : reading ? 'reading' : undefined}>{sub}</span>
      </span>
      {reading && <Spinner size={12} color="var(--color-info)" />}
      {paused && <Icon name="clock" size={14} label={copy.channelPaused} className="vbd-warn-ic" />}
    </button>
  )
}

function VideoRow({ video, current, linked, onPick }: {
  video: BackdropVideo
  current: boolean
  linked?: boolean
  onPick: () => void
}) {
  const busy = video.state === 'downloading'
  return (
    <button type="button" className="vbd-rail-row" aria-current={current || undefined} onClick={onPick}>
      <span className="vbd-rail-vthumb" aria-hidden="true"><Icon name="video-play" size={16} /></span>
      <span className="vbd-rail-text">
        <span className="vbd-rail-name">{video.title}</span>
        <span className="vbd-rail-sub">
          <span className="vbd-dot" style={{ background: platformColor(video.platform) }} />
          {platformName(video.platform)} · {segmentLabel(video.segment.fromSec, video.segment.toSec)}
        </span>
      </span>
      {busy && <Spinner size={12} color="var(--color-info)" />}
      {linked && <span className="vbd-tag" data-tone="linked">{t.backdrop.template.picker.linkedTag}</span>}
    </button>
  )
}

export function SourceRail({ channels, videos, selected, linkedIds, videoRows = 'summary', onSelect, ariaLabel }: {
  channels: BackdropChannel[]
  videos: BackdropVideo[]
  /** 'videos' = trang Video lẻ; còn lại là id kênh đang chọn */
  selected: string
  /** id đã gắn vào template (hộp chọn) → thẻ "Đã gắn" */
  linkedIds?: ReadonlySet<string>
  /** 'summary' = một hàng Tất cả video lẻ (tab); 'items' = mỗi video một hàng (hộp chọn) */
  videoRows?: 'summary' | 'items'
  onSelect: (id: string) => void
  ariaLabel?: string
}) {
  const copy = t.backdrop.tab
  const platforms = [...new Set(channels.map((channel) => channel.platform))]
  const anyLoading = videos.some((video) => video.state === 'downloading')
  return (
    <nav className="vbd-rail" aria-label={ariaLabel ?? copy.railAria}>
      <span className="vbd-rail-head">{copy.groupChannels}<span className="num vbd-rail-count">{num(channels.length)}</span></span>
      {platforms.map((platform) => {
        const list = channels.filter((channel) => channel.platform === platform)
        return (
          <div className="vbd-rail-plat" key={platform}>
            <span className="vbd-rail-plathead">
              <span className="vbd-dot" style={{ background: platformColor(platform) }} />
              {platformName(platform)}<span className="num vbd-rail-count">{num(list.length)}</span>
            </span>
            {list.map((channel) => (
              <ChannelRow key={channel.id} channel={channel} current={selected === channel.id}
                linked={linkedIds?.has(channel.id)} onPick={() => onSelect(channel.id)} />
            ))}
          </div>
        )
      })}
      <span className="vbd-rail-line" aria-hidden="true" />
      <span className="vbd-rail-head">{copy.groupVideos}<span className="num vbd-rail-count">{num(videos.length)}</span></span>
      {videoRows === 'summary' ? (
        <button type="button" className="vbd-rail-row" aria-current={selected === 'videos' || undefined} onClick={() => onSelect('videos')}>
          <span className="vbd-rail-vthumb" aria-hidden="true"><Icon name="video-play" size={16} /></span>
          <span className="vbd-rail-text">
            <span className="vbd-rail-name">{copy.videoGroupTitle}</span>
            <span className="vbd-rail-sub">{copy.videoGroupSub(num(videos.length))}</span>
          </span>
          {anyLoading && <Spinner size={12} color="var(--color-info)" />}
        </button>
      ) : (
        videos.map((video) => (
          <VideoRow key={video.id} video={video} current={selected === video.id}
            linked={linkedIds?.has(video.id)} onPick={() => onSelect(video.id)} />
        ))
      )}
    </nav>
  )
}
