import * as stylex from '@stylexjs/stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// Bước Xong (artboard CreateChannel/Onboard › step=done): kênh đã tạo, avatar
// và câu "Đạo diễn đặt theo … (tham chiếu)" suy từ đối thủ chọn đầu tiên; thẻ
// "Đang chạy nền" đọc trạng thái nhận diện thật (SSE `channels` + tiến độ từ
// `activity` kind identity); thẻ "Việc tiếp theo" là video mới nhất thật của
// đối thủ tham chiếu. Nút Làm video là việc của hàng chờ — khoá kèm lý do.
import { useState } from 'react'
import { Icon, ProgressBar, Spinner } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar, Orb } from '../../components'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getActivity } from '../../api/activity'
import { generateBranding, listChannels, getChannel } from '../../api/channels'
import { listRivalVideos } from '../../api/rivals'
import { fmtAge, fmtDuration, fmtScore } from '../explore/fmt'
import { fmtTime } from './fmt'
import { channelTint } from '../../lib/channelTint'
import { useMediaQuery } from '../../lib/useMediaQuery'

export function DoneStep({ channelId, onLeave }: {
  /** id kênh vừa tạo (sessionStorage) — tải lại trang thì lấy kênh mới nhất */
  channelId: string | null
  /** rời khung Onboard (ghi cờ phiên) trước khi điều hướng */
  onLeave: () => void
}) {
  const tt = t.channels.done
  const mobile = useMediaQuery('(max-width: 767px)')
  const [retrying, setRetrying] = useState(false)
  const [retryError, setRetryError] = useState<string | null>(null)
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
      <div className={`vch-new ${stylex.props(channelStyles.new, channelRootMarker).className}`}>
        <div className={`vch-new__main ${stylex.props(channelStyles.newMain).className}`}>
          <h1 {...stylex.props(channelStyles.newTitle, channelStyles.doneHeading)}>{tt.title}</h1>
          <p className={`vch-intro ${stylex.props(channelStyles.intro).className}`}>{channels.initialLoading || channel.initialLoading ? t.common.loading : tt.bruteForce}</p>
          <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
            <ActionButton className={stylex.props(channelStyles.touchTarget).className} variant="primary" onClick={() => { onLeave(); window.location.hash = `#${paths.channelNew()}` }}>
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
  // r38: Đạo diễn chưa đặt được tên/mô tả → kênh vẫn tạo với tên tạm "Kênh mới",
  // thẻ đổi thành "Một việc cần bạn", dòng Đạo diễn có Thử lại.
  const directorFailed = identity.state === 'failed' && !textDone
  const reason = identity.error?.message ?? t.channels.identity.failedTitle
  const failNote = identity.retryAt ? tt.retryAt(fmtTime(identity.retryAt)) : ''
  const runText = task?.phase ?? t.channels.identity.generating
  const director: StepView = textDone
    ? { state: 'ok', what: tt.directorWhat, text: tt.ok }
    : directorFailed
      ? { state: 'fail', what: tt.directorFailed(reason), text: '', retry: true }
      : { state: 'run', what: tt.directorWhat, text: runText }
  const artist: StepView = identity.state === 'ready'
    ? { state: 'ok', what: tt.artistWhat, text: tt.ok }
    : !textDone
      ? (directorFailed
        ? { state: 'wait', what: tt.artistWaitWhat, text: tt.wait }
        : { state: 'wait', what: tt.artistWhat, text: tt.waitDirector })
      : identity.state === 'failed'
        ? { state: 'fail', what: tt.artistFailed(reason), text: failNote }
        : { state: 'run', what: tt.artistWhat, text: `${percent}% · ${runText}` }

  /** Thử lại = chạy lại bước đặt tên của Đạo diễn (part=name), không đụng ảnh. */
  const retryDirector = async () => {
    if (retrying) return
    setRetrying(true)
    setRetryError(null)
    try {
      const result = await generateBranding(data.id, { part: 'name' })
      setData(keys.channel(data.id), result.channel)
      void invalidate(keys.channels())
    } catch (err) {
      setRetryError(err instanceof Error ? err.message : t.channels.identity.retryNameFail)
    } finally {
      setRetrying(false)
    }
  }

  const latestMeta = video
    ? [
      video.publishedAt ? `Đăng ${fmtAge(video.publishedAt)} trước` : '',
      fmtDuration(video.durationSec),
      video.score != null ? fmtScore(video.score) : '',
    ].filter(Boolean).join(' · ')
    : ''

  return (
    <div className={`vch-new vch-done ${stylex.props(channelStyles.new, channelRootMarker).className}`}>
      <div className={`vch-new__main ${stylex.props(channelStyles.newMain).className}`}>
        <div className={`vch-done__head ${stylex.props(channelStyles.doneHead).className}`}>
          {directorFailed ? (
            /* Tên tạm: avatar chữ “K” nền trung tính tới khi Đạo diễn đặt được tên. */
            <ChannelAvatar
              name={tt.noName}
              initials="K"
              color={{ bg: 'var(--color-surface-5)', ring: 'var(--line-5)' }}
              size={64}
              style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-fg-2)' }}
            />
          ) : (
            <ChannelAvatar
              name={data.name || t.channels.page.noName}
              src={data.avatarUrl ?? undefined}
              color={{ bg: channelTint(data.id), ring: channelTint(data.id) }}
              size={64}
              style={{ fontSize: 20, fontWeight: 700 }}
            />
          )}
          <div className={`vch-done__title ${stylex.props(channelStyles.doneTitle).className}`}>
            <h1 {...stylex.props(channelStyles.newTitle, channelStyles.doneHeading)}>{directorFailed ? tt.titleTemp : tt.title}</h1>
            <p className={`vch-done__sub ${stylex.props(channelStyles.doneSub).className}`}>
              {directorFailed ? (
                <>
                  <span className={`vch-done__name ${stylex.props(channelStyles.doneName).className}`}>{tt.noName}</span>
                  {' '}
                  {tt.tempNote}
                  {' '}
                  {tt.subTail({
                    rivals: data.sources.length,
                    language: t.channels.page.languageLower[data.language],
                    pace: data.pacePerDay,
                  })}
                </>
              ) : reference
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

        <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={directorFailed ? tt.needsYou : tt.background}>
          <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
            <h2 {...stylex.props(channelStyles.cardHeading)}>
              <span aria-hidden="true" className={`vch-recdot ${stylex.props(channelStyles.recdot, directorFailed && channelStyles.recdotWarn).className}`} />
              {directorFailed ? tt.needsYou : tt.background}
            </h2>
            <span className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{directorFailed ? tt.needsYouSub : tt.backgroundSub}</span>
          </div>
          <StepRow who="director" label={tt.director} step={director} mobile={mobile} onRetry={() => void retryDirector()} retrying={retrying} />
          <StepRow who="artist" label={tt.artist} step={artist} percent={percent} mobile={mobile} />
          <StepRow who="qa" label={tt.qa} step={{ state: 'ok', what: tt.qaWhat, text: tt.ok }} mobile={mobile} />
          {retryError && <p className={`vch-field__hint ${stylex.props(channelStyles.fieldError).className}`} role="alert">{retryError}</p>}
          <p className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{directorFailed ? tt.footnoteTemp : tt.footnote}</p>
        </section>

        <section className={`vch-card ${stylex.props(channelStyles.card).className}`} aria-label={tt.next}>
          <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead).className}`} style={{ padding: 0 }}>
            <h2 {...stylex.props(channelStyles.cardHeading)}><Icon name="video-play" size={16} />{tt.next}</h2>
          </div>
          {video ? (
            <div className={`vch-next ${stylex.props(channelStyles.next).className}`}>
              <ChannelAvatar
                name={reference?.title ?? ''}
                color={{ bg: channelTint(reference?.rivalId ?? data.id) }}
                size={34}
                style={{ fontSize: 11, fontWeight: 700 }}
              />
              <span className={`vch-next__text ${stylex.props(channelStyles.nextText).className}`}>
                <span className={`vch-next__title ${stylex.props(channelStyles.nextTitle).className}`} title={video.title}>{video.title}</span>
                <span className={`vch-next__meta ${stylex.props(channelStyles.nextMeta).className}`}>{latestMeta}</span>
              </span>
              <ActionButton className={stylex.props(channelStyles.touchTarget).className}
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
            <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.latestEmpty}</p>
          )}
        </section>

        <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
          <ActionButton className={stylex.props(channelStyles.touchTarget).className} size="md" onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id, 'settings')}` }}>
            {tt.openChannel}
          </ActionButton>
          {/* r38: Đạo diễn chưa đặt được tên → cho đặt tay ở Hồ sơ kênh. */}
          {directorFailed && (
            <ActionButton className={stylex.props(channelStyles.touchTarget).className} size="md" icon={<Icon name="edit" size={15} />} onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id, 'settings')}` }}>
              {tt.nameInProfile}
            </ActionButton>
          )}
          <ActionButton className={stylex.props(channelStyles.touchTarget).className} size="md" icon={<Icon name="radar" size={15} />} onClick={() => { onLeave(); window.location.hash = `#${paths.explore()}` }}>
            {tt.explore}
          </ActionButton>
        </div>
      </div>
    </div>
  )
}

type StepView = { state: 'ok' | 'run' | 'fail' | 'wait'; what: string; text: string; retry?: boolean }

function StepRow({ who, label, step, percent, mobile = false, onRetry, retrying = false }: {
  who: 'director' | 'artist' | 'qa'
  label: string
  step: StepView
  /** thanh tiến độ chung của việc nền, chỉ hiện trên bước đang chạy có số */
  percent?: number
  /** bản điện thoại: nút Thử lại cao 40px (artboard MCreate) */
  mobile?: boolean
  /** bước hỏng chạy lại được (r38: Đạo diễn chưa đặt được tên) */
  onRetry?: () => void
  retrying?: boolean
}) {
  const tone = step.state === 'run' ? ' vch-task__state--run'
    : step.state === 'fail' ? ' vch-task__state--fail'
      : step.state === 'wait' ? ' vch-task__state--wait' : ''
  return (
    <div className={`vch-task ${stylex.props(channelStyles.task).className}`}>
      <Orb of={who} size={20} live={step.state === 'run'} />
      <span className={`vch-task__text ${stylex.props(channelStyles.taskText).className}`}>
        <span className={`vch-task__who ${stylex.props(channelStyles.taskWho).className}`}>{label}</span>
        <span className={`vch-task__what ${stylex.props(channelStyles.taskWhat).className}`}>{step.what}</span>
      </span>
      <span className={`vch-task__state${tone} ${stylex.props(channelStyles.taskState, step.state === 'run' && channelStyles.taskStateRun, step.state === 'fail' && channelStyles.taskStateFail, step.state === 'wait' && channelStyles.taskStateWait).className}`}>
        {step.state === 'ok' && <Icon name="tick-b" size={13} />}
        {step.text}
        {step.retry && onRetry && (
          <ActionButton
            variant="outline"
            size={mobile ? 'lg' : 'sm'}
            xstyle={mobile ? [channelStyles.retryM] : undefined}
            icon={<Icon name="refresh" size={mobile ? 15 : 14} />}
            pending={retrying}
            onClick={onRetry}
          >
            {t.common.retry}
          </ActionButton>
        )}
      </span>
      {step.state === 'run' && percent !== undefined && (
        <span className={`vch-task__bar ${stylex.props(channelStyles.taskBar).className}`}><ProgressBar value={percent} tone="warn" height={4} /></span>
      )}
    </div>
  )
}
