import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Kênh chưa có video nào (artboard Channel `fresh`): lời dẫn + hai thẻ dẫn —
// video mới nhất của đối thủ tham chiếu (số liệu thật từ API video) và "Tự lấy
// video" (trạng thái, số đối thủ và nhịp thật của kênh, giờ Trinh sát xem gần nhất).
import { Icon } from '@noverse/ui'
import { useMediaQuery } from '../../lib/useMediaQuery'
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
  const narrow = useMediaQuery('(max-width: 767px)')
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

  // MChannel › kênh mới: đầu trung tâm + thẻ “Video mới nhất” + thẻ “Tự lấy
  // video” (nhịp/xem gần nhất thật) + nút mở Khám phá.
  if (narrow) {
    return (
      <div className={`vch-fresh ${stylex.props(channelStyles.freshMobile).className}`} aria-label={tt.page.tabs.videos}>
        <div {...stylex.props(channelStyles.freshHero)}>
          <span aria-hidden="true" {...stylex.props(channelStyles.freshHeroIcon)}><Icon name="video-add" size={20} /></span>
          <span {...stylex.props(channelStyles.freshHeroTitle)}>{tt.videos.freshTitle}</span>
          <span {...stylex.props(channelStyles.freshHeroBody)}>{tt.videos.freshBody}</span>
        </div>
        <section aria-label={tt.videos.latestTitle} {...stylex.props(channelStyles.freshCard)}>
          <span {...stylex.props(channelStyles.freshSec)}>{tt.videos.latestTitle}</span>
          {video ? (
            <>
              <div {...stylex.props(channelStyles.freshRow)}>
                <span aria-hidden="true" {...stylex.props(channelStyles.freshThumb)}>
                  {video.thumbnailUrl ? <img src={video.thumbnailUrl} alt="" loading="lazy" {...stylex.props(channelStyles.freshThumbImg)} /> : null}
                </span>
                <span {...stylex.props(channelStyles.freshTexts)}>
                  <span {...stylex.props(channelStyles.freshTitleSm)} title={video.title}>{video.title}</span>
                  <span className={`num ${stylex.props(channelStyles.freshMeta).className}`}>{meta} · {fmtDuration(video.durationSec)}</span>
                </span>
              </div>
              <ActionButton
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
              <span {...stylex.props(channelStyles.freshMeta)}>{reference ? tt.videos.latestEmpty : tt.videos.noRivals}</span>
            </>
          )}
        </section>
        <a className={`vch-auto ${stylex.props(channelStyles.freshCard, channelStyles.freshAuto).className}`} href={`#${paths.channel(channel.id, 'sources')}`}>
          <Orb of="scout" size={22} live={channel.autoPick && hasSources} />
          <span {...stylex.props(channelStyles.freshAutoTexts)}>
            <span {...stylex.props(channelStyles.freshAutoTitle)}>
              {tt.videos.autoTitle} · <span {...stylex.props(channelStyles.freshAutoOn, !channel.autoPick && channelStyles.leadTagOff)}>{channel.autoPick ? tt.videos.autoOn : tt.videos.autoOff}</span>
            </span>
            <span className={`num ${stylex.props(channelStyles.freshAutoSub).className}`}>
              {hasSources
                ? tt.videos.autoLine(channel.sources.length) + (seen ? ` · ${tt.videos.paceSeen(channel.pacePerDay, seen)}` : ` · ${tt.videos.pace(channel.pacePerDay)}`)
                : tt.videos.noRivals}
            </span>
          </span>
          <Icon name="arrow-r" size={16} className={stylex.props(channelStyles.freshChevron).className} />
        </a>
        <ActionButton
          size="md"
          icon={<Icon name="radar" size={16} />}
          onClick={() => navigate(paths.explore({ rival: rival?.handle ?? rival?.id }))}
        >
          {tt.videos.explorePick}
        </ActionButton>
      </div>
    )
  }

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
              <ActionButton xstyle={channelStyles.leadMake}
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
            <ActionButton variant="quiet" size="sm" onClick={() => navigate(paths.channel(channel.id, 'sources'))}>{tt.videos.paceLink}</ActionButton>
          </span>
        </div>
      </div>
    </section>
  )
}
