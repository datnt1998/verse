// Bước Xong (artboard CreateChannel/Onboard › step=done): kênh đã tạo, avatar
// và câu "Đạo diễn đặt theo … (tham chiếu)" suy từ đối thủ chọn đầu tiên; thẻ
// "Đang chạy nền" đọc trạng thái nhận diện thật (SSE `channels` + tiến độ từ
// `activity` kind identity); thẻ "Việc tiếp theo" là video mới nhất thật của
// đối thủ tham chiếu. Nút Làm video là việc của hàng chờ — khoá kèm lý do.
import { Icon, ProgressBar, Spinner } from '@noverse/ui'
import { ActionButton, ChannelAvatar, Orb } from '../../components'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getActivity } from '../../api/activity'
import { listChannels, getChannel } from '../../api/channels'
import { listRivalVideos } from '../../api/rivals'
import { fmtAge, fmtDuration, fmtScore } from '../explore/fmt'
import { fmtTime } from './fmt'
import { channelTint } from '../../lib/channelTint'

export function DoneStep({ channelId, onLeave }: {
  /** id kênh vừa tạo (sessionStorage) — tải lại trang thì lấy kênh mới nhất */
  channelId: string | null
  /** rời khung Onboard (ghi cờ phiên) trước khi điều hướng */
  onLeave: () => void
}) {
  const tt = t.channels.done
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const list = channels.data?.channels ?? []
  const id = channelId ?? list[0]?.id ?? null
  const channel = useResource(
    keys.channel(id ?? '_none'),
    ({ signal }) => getChannel(id!, signal),
    { enabled: !!id, keepPrevious: true },
  )
  const identityState = channel.data?.identity.state
  // Tiến độ việc nền cho biết run đang chạy đã qua bước chữ hay chưa.
  const activity = useResource(keys.activity(), ({ signal }) => getActivity(signal), {
    enabled: identityState === 'generating',
  })
  const data = channel.data
  const reference = data?.sources.find((source) => source.reference) ?? data?.sources[0]
  const latest = useResource(
    keys.rivalVideos(reference?.rivalId ?? '_none', 'newest', 0, 0, ''),
    ({ signal }) => listRivalVideos(reference!.rivalId, { sort: 'newest', limit: 1 }, signal),
    { enabled: !!reference },
  )
  const video = latest.data?.videos[0]
  const task = activity.data?.running.find((item) => item.kind === 'identity' && item.objectRef.id === data?.id)
  const percent = task?.progress ? Math.round((task.progress.done / Math.max(1, task.progress.total)) * 100) : 0
  const identity = data?.identity

  if (!data || !identity) {
    return (
      <div className="vch-new">
        <div className="vch-new__main">
          <h1>{tt.title}</h1>
          <p className="vch-intro">{channels.initialLoading || channel.initialLoading ? t.common.loading : tt.bruteForce}</p>
          <div className="vch-actions">
            <ActionButton variant="primary" onClick={() => { onLeave(); window.location.hash = `#${paths.channelNew()}` }}>
              {t.channels.create.title}
            </ActionButton>
          </div>
        </div>
      </div>
    )
  }

  // Một việc nền chạy lần lượt: bước 1 là Đạo diễn (tên + mô tả), sau đó Hoạ sĩ
  // vẽ hai ảnh. Đang chạy: Đạo diễn xong khi tiến độ đã qua bước 1. Hỏng: server
  // ghi bước hỏng; không rõ bước (run chết giữa chừng) thì báo ở bước đầu.
  const textDone = identity.state === 'ready'
    || (identity.state === 'generating' && (task?.progress?.done ?? 0) >= 1)
    || (identity.state === 'failed' && identity.error?.step === 'images')
  const reason = identity.error?.message ?? t.channels.identity.failedTitle
  const failNote = identity.retryAt ? tt.retryAt(fmtTime(identity.retryAt)) : ''
  const runText = task?.phase ?? t.channels.identity.generating
  const director: StepView = textDone
    ? { state: 'ok', what: tt.directorWhat, text: tt.ok }
    : identity.state === 'failed'
      ? { state: 'fail', what: tt.directorFailed(reason), text: failNote }
      : { state: 'run', what: tt.directorWhat, text: runText }
  const artist: StepView = identity.state === 'ready'
    ? { state: 'ok', what: tt.artistWhat, text: tt.ok }
    : !textDone
      ? { state: 'wait', what: tt.artistWhat, text: tt.waitDirector }
      : identity.state === 'failed'
        ? { state: 'fail', what: tt.artistFailed(reason), text: failNote }
        : { state: 'run', what: tt.artistWhat, text: `${percent}% · ${runText}` }

  const latestMeta = video
    ? [
      video.publishedAt ? `Đăng ${fmtAge(video.publishedAt)} trước` : '',
      fmtDuration(video.durationSec),
      video.score != null ? fmtScore(video.score) : '',
    ].filter(Boolean).join(' · ')
    : ''

  return (
    <div className="vch-new vch-done">
      <div className="vch-new__main">
        <div className="vch-done__head">
          <ChannelAvatar
            name={data.name || t.channels.page.noName}
            src={data.avatarUrl ?? undefined}
            color={{ bg: channelTint(data.id), ring: channelTint(data.id) }}
            size={64}
            style={{ fontSize: 20, fontWeight: 700 }}
          />
          <div className="vch-done__title">
            <h1>{tt.title}</h1>
            <p className="vch-done__sub">
              {reference
                ? tt.sub({
                  name: data.name || tt.noName,
                  rival: reference.title,
                  rivals: data.sources.length,
                  language: t.channels.page.languageLower[data.language],
                  pace: data.pacePerDay,
                })
                : tt.noName}
            </p>
          </div>
        </div>

        <section className="vch-card" aria-label={tt.background}>
          <div className="vch-cardhead" style={{ padding: 0 }}>
            <h2><span aria-hidden="true" className="vch-recdot" />{tt.background}</h2>
            <span className="vch-field__hint">{tt.backgroundSub}</span>
          </div>
          <StepRow who="director" label={tt.director} step={director} />
          <StepRow who="artist" label={tt.artist} step={artist} percent={percent} />
          <StepRow who="qa" label={tt.qa} step={{ state: 'ok', what: tt.qaWhat, text: tt.ok }} />
          <p className="vch-field__hint">{tt.footnote}</p>
        </section>

        <section className="vch-card" aria-label={tt.next}>
          <div className="vch-cardhead" style={{ padding: 0 }}>
            <h2><Icon name="video-play" size={16} />{tt.next}</h2>
          </div>
          {video ? (
            <div className="vch-next">
              <ChannelAvatar
                name={reference?.title ?? ''}
                color={{ bg: channelTint(reference?.rivalId ?? data.id) }}
                size={34}
                style={{ fontSize: 11, fontWeight: 700 }}
              />
              <span className="vch-next__text">
                <span className="vch-next__title" title={video.title}>{video.title}</span>
                <span className="vch-next__meta">{latestMeta}</span>
              </span>
              <ActionButton
                variant="primary"
                size="md"
                lockedReason={t.channels.queueLock}
                icon={<Icon name="video-add" size={15} />}
              >
                {tt.makeLatest(reference?.title ?? '')}
              </ActionButton>
            </div>
          ) : latest.initialLoading ? (
            <Spinner size={16} color="var(--color-fg-3)" />
          ) : (
            <p className="vch-hint">{tt.latestEmpty}</p>
          )}
        </section>

        <div className="vch-actions">
          <ActionButton size="md" onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id, 'settings')}` }}>
            {tt.openChannel}
          </ActionButton>
          <ActionButton size="md" icon={<Icon name="radar" size={15} />} onClick={() => { onLeave(); window.location.hash = `#${paths.explore()}` }}>
            {tt.explore}
          </ActionButton>
        </div>
      </div>
    </div>
  )
}

type StepView = { state: 'ok' | 'run' | 'fail' | 'wait'; what: string; text: string }

function StepRow({ who, label, step, percent }: {
  who: 'director' | 'artist' | 'qa'
  label: string
  step: StepView
  /** thanh tiến độ chung của việc nền, chỉ hiện trên bước đang chạy có số */
  percent?: number
}) {
  const tone = step.state === 'run' ? ' vch-task__state--run'
    : step.state === 'fail' ? ' vch-task__state--fail'
      : step.state === 'wait' ? ' vch-task__state--wait' : ''
  return (
    <div className="vch-task">
      <Orb of={who} size={20} live={step.state === 'run'} />
      <span className="vch-task__text">
        <span className="vch-task__who">{label}</span>
        <span className="vch-task__what">{step.what}</span>
      </span>
      <span className={`vch-task__state${tone}`}>
        {step.state === 'ok' && <Icon name="tick-b" size={13} />}
        {step.text}
      </span>
      {step.state === 'run' && percent !== undefined && (
        <span className="vch-task__bar"><ProgressBar value={percent} tone="warn" height={4} /></span>
      )}
    </div>
  )
}
