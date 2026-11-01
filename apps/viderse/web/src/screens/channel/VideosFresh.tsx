import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Kênh chưa có video nào (artboard Channel `fresh`): lời dẫn + hai thẻ dẫn —
// video mới nhất của đối thủ tham chiếu (số liệu thật từ API video) và "Tự lấy
// video" (trạng thái, số đối thủ và nhịp thật của kênh, giờ Trinh sát xem gần nhất).
import { Icon } from '@noverse/ui'
import { ActionButton, Orb } from '../../components'
import { t } from '../../i18n/channels'
import { navigate, paths } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { getRivalScan, listRivalVideos, type Rival } from '../../api/rivals'
import type { Channel } from '../../api/channels'
import { fmtAge, fmtDuration, fmtScore } from '../explore/fmt'
import { fmtTime } from '../settings/fmt'

export function VideosFresh({ channel, rival }: {
  channel: Channel
  /** đối thủ tham chiếu (link sớm nhất) — nguồn của thẻ "Video mới nhất" */
  rival: Rival | undefined
}) {
  const tt = t.channels
  const latest = useResource(
    keys.rivalVideos(rival?.id ?? '_none', 'newest', 0, 0, ''),
    ({ signal }) => listRivalVideos(rival!.id, { sort: 'newest', limit: 1 }, signal),
    { enabled: !!rival },
  )
  const scan = useResource(
    keys.rivalScan(rival?.id ?? '_none'),
    ({ signal }) => getRivalScan(rival!.id, signal),
    { enabled: !!rival },
  )
  const video = latest.data?.videos[0]
  const reference = channel.sources.find((source) => source.reference) ?? channel.sources[0]
  const rivalName = rival?.title ?? reference?.title ?? ''
  const hasSources = channel.sources.length > 0
  const seen = fmtTime(scan.data?.discovery.checkedAt)

  const meta = video
    ? tt.videos.latestMeta(rivalName, fmtAge(video.publishedAt), video.score != null ? fmtScore(video.score) : null)
    : ''

  return (
    <section className={`vch-card vch-fresh ${stylex.props(channelStyles.card, channelStyles.fresh).className}`} aria-label={tt.page.tabs.videos}>
      <h2 {...stylex.props(channelStyles.freshTitle)}>{tt.videos.freshTitle}</h2>
      <p {...stylex.props(channelStyles.freshBody)}>{tt.videos.freshBody}</p>
      <div className={`vch-leads ${stylex.props(channelStyles.leads).className}`}>
        <div className={`vch-lead ${stylex.props(channelStyles.lead).className}`}>
          <span {...stylex.props(channelStyles.leadHead)}>{tt.videos.latestTitle}</span>
          {video ? (
            <>
              <div {...stylex.props(channelStyles.leadRow)}>
                <span aria-hidden="true" {...stylex.props(channelStyles.leadThumb)}>
                  {video.thumbnailUrl ? <img src={video.thumbnailUrl} alt="" loading="lazy" {...stylex.props(channelStyles.leadThumbImg)} /> : null}
                  <span className={`num mono ${stylex.props(channelStyles.leadDur).className}`}>{fmtDuration(video.durationSec)}</span>
                </span>
                <span {...stylex.props(channelStyles.leadTexts)}>
                  <span {...stylex.props(channelStyles.leadTitle)} title={video.title}>{video.title}</span>
                  <span className={`num ${stylex.props(channelStyles.leadMeta).className}`}>{meta}</span>
                </span>
              </div>
              <ActionButton className={stylex.props(channelStyles.touchTarget, channelStyles.leadMake).className}
                size="sm"
                variant="primary"
                icon={<Icon name="video-add" size={14} />}
                onClick={() => navigate(paths.new(channel.id))}
              >
                {tt.videos.make}
              </ActionButton>
            </>
          ) : (
            <>
              <span {...stylex.props(channelStyles.leadMeta)}>{reference ? tt.videos.latestEmpty : tt.videos.noRivals}</span>
              <a className={`vch-chip ${stylex.props(channelStyles.chip, channelStyles.leadMake).className}`} href={`#${paths.explore({ rival: rival?.handle ?? rival?.id })}`}>
                <Icon name="radar" size={14} />
                {tt.videos.explore}
              </a>
            </>
          )}
        </div>
        <div className={`vch-lead ${stylex.props(channelStyles.lead).className}`}>
          <span {...stylex.props(channelStyles.leadHead)}>{tt.videos.autoTitle}</span>
          <div {...stylex.props(channelStyles.leadRow)}>
            <Orb of="scout" size={22} live={channel.autoPick && hasSources} />
            <span {...stylex.props(channelStyles.leadAuto)}>
              {hasSources ? tt.videos.autoLine(channel.sources.length) : tt.videos.autoPaused}
            </span>
            <span {...stylex.props(channelStyles.leadTag, !channel.autoPick && channelStyles.leadTagOff)}>
              {channel.autoPick ? tt.videos.autoOn : tt.videos.autoOff}
            </span>
          </div>
          <span className={`num ${stylex.props(channelStyles.leadSplit).className}`}>
            <span>{hasSources ? (seen ? tt.videos.paceSeen(channel.pacePerDay, seen) : tt.videos.pace(channel.pacePerDay)) : tt.videos.noRivals}</span>
            <button type="button" {...stylex.props(channelStyles.leadLink)} onClick={() => navigate(paths.channel(channel.id, 'sources'))}>{tt.videos.paceLink}</button>
          </span>
        </div>
      </div>
    </section>
  )
}
