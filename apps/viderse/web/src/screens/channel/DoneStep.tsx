import * as stylex from '@stylexjs/stylex'
import { channelStyles, channelRootMarker } from './channel.stylex'
// Bước Xong (artboard CreateChannel/Onboard › step=done): kênh đã tạo, avatar
// và câu "Đạo diễn đặt theo … (tham chiếu)" suy từ đối thủ chọn đầu tiên; thẻ
// "Đang chạy nền" đọc trạng thái nhận diện thật (SSE `channels` + tiến độ từ
// `activity` kind identity); thẻ "Việc tiếp theo" là video mới nhất thật của
// đối thủ tham chiếu. Nút Làm video đọc video mới nhất thật rồi xếp hàng chờ.
import { useState, type ReactNode } from 'react'
import { Button, Icon, ProgressBar, Spinner, toast } from '@noverse/ui'
import { invalidate, setData } from '@noverse/ui/data'
import { ActionButton, ChannelAvatar, Orb } from '../../components'
import { t } from '../../i18n/channels'
import { paths } from '../../router'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { getActivity } from '../../api/activity'
import { generateBranding, listChannels, getChannel } from '../../api/channels'
import { listRivalVideos } from '../../api/rivals'
import { fmtAge, fmtDuration, fmtScore } from '../explore/fmt'
import { fmtTime } from './fmt'
import { makeLatestRivalVideo } from './latest'
import { channelTint } from '../../lib/channelTint'
import { useMediaQuery } from '../../lib/useMediaQuery'
import { MobilePage } from '@noverse/ui'
import { navigate } from '../../router'
import { mch } from './mobile/mchannel.stylex'

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
  const [making, setMaking] = useState(false)
  const [brokenThumbnail, setBrokenThumbnail] = useState<string | null>(null)
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
    keys.rivalLatestVideo(reference?.rivalId ?? '_none'),
    ({ signal }) => listRivalVideos(reference!.rivalId, { sort: 'newest', limit: 1 }, signal),
    { enabled: !!reference },
  )
  const video = latest.data?.videos[0]
  const task = activity.data?.running.find((item) => item.kind === 'identity' && item.objectRef.id === data?.id)
  const percent = task?.progress ? Math.round((task.progress.done / Math.max(1, task.progress.total)) * 100) : 0
  const identity = data?.identity

  /** Điện thoại: một đầu trang riêng "Kênh mới" + nút đóng; không appbar/breadcrumb. */
  const wrap = (content: ReactNode) => (mobile ? (
    <MobilePage
      title={tt.noName}
      leading={(
        <Button variant="icon" size="lg" aria-label={t.common.close}
          onClick={() => { onLeave(); navigate(data ? paths.channel(data.id) : paths.workbench()) }}>
          <Icon name="close" size={20} />
        </Button>
      )}
    >
      {content}
    </MobilePage>
  ) : content)

  if (!data || !identity) {
    return wrap(
      <div className={`vch-new ${stylex.props(channelStyles.new, channelRootMarker, mobile && mch.embedded).className}`}>
        <div className={`vch-new__main ${stylex.props(channelStyles.newMain).className}`}>
          <h1 {...stylex.props(channelStyles.newTitle, channelStyles.doneHeading)}>{tt.title}</h1>
          <p className={`vch-intro ${stylex.props(channelStyles.intro).className}`}>{channels.initialLoading || channel.initialLoading ? t.common.loading : tt.bruteForce}</p>
          <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
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

  const makeLatest = async () => {
    if (!reference || !video || making) return
    setMaking(true)
    try {
      const result = await makeLatestRivalVideo(data.id, reference.rivalId)
      onLeave()
      navigate(result.jobId ? paths.workbench({ video: result.jobId }) : paths.queue())
    } catch (err) {
      toast.err(err instanceof Error ? err.message : t.channels.latestFailed)
    } finally {
      setMaking(false)
    }
  }

  const latestMeta = video
    ? [
      video.publishedAt ? `Đăng ${fmtAge(video.publishedAt)} trước` : '',
      fmtDuration(video.durationSec),
      video.score != null ? fmtScore(video.score) : '',
    ].filter(Boolean).join(' · ')
    : ''
  /* Điện thoại: điểm nằm trên ảnh thu nhỏ nên dòng meta chỉ còn ngày đăng + thời lượng. */
  const latestMetaM = video
    ? [
      video.publishedAt ? `Đăng ${fmtAge(video.publishedAt)} trước` : '',
      fmtDuration(video.durationSec),
    ].filter(Boolean).join(' · ')
    : ''

  return wrap(
    <div className={`vch-new vch-done ${stylex.props(channelStyles.new, channelRootMarker, mobile && mch.embedded).className}`}>
      <div className={`vch-new__main ${stylex.props(channelStyles.newMain).className}`}>
        <div className={`vch-done__head ${stylex.props(channelStyles.doneHead, mobile && channelStyles.doneHeadM).className}`}>
          {directorFailed ? (
            /* Tên tạm: avatar chữ “K” nền trung tính tới khi Đạo diễn đặt được tên. */
            <ChannelAvatar
              name={tt.noName}
              initials="K"
              color={{ bg: 'var(--color-surface-5)', ring: 'var(--line-5)' }}
              size={mobile ? 60 : 64}
              style={{ fontSize: 20, fontWeight: 700, color: 'var(--color-fg-2)' }}
            />
          ) : (
            <ChannelAvatar
              name={data.name || t.channels.page.noName}
              src={data.avatarUrl ?? undefined}
              color={{ bg: channelTint(data.id), ring: channelTint(data.id) }}
              size={mobile ? 60 : 64}
              style={{ fontSize: 20, fontWeight: 700 }}
            />
          )}
          <div className={`vch-done__title ${stylex.props(channelStyles.doneTitle).className}`}>
            <h1 {...stylex.props(channelStyles.newTitle, channelStyles.doneHeading, mobile && channelStyles.doneHeadingM)}>{directorFailed ? tt.titleTemp : tt.title}</h1>
            <p className={`vch-done__sub ${stylex.props(channelStyles.doneSub).className}`}>
              {mobile
                ? (reference
                  ? tt.subM({
                    name: directorFailed ? tt.noName : (data.name || tt.noName),
                    rivals: data.sources.length,
                    language: t.channels.page.languageLower[data.language],
                    pace: data.pacePerDay,
                  })
                  : tt.noName)
                : directorFailed ? (
                  <>
                    <span className={`vch-done__name`}>{tt.noName}</span>
                    {' '}
                    <span className={`vch-done__tempnote ${stylex.props(channelStyles.doneNote).className}`}>{tt.tempNote}</span>
                    {' '}
                    {tt.subTail({
                      rivals: data.sources.length,
                      language: t.channels.page.languageLower[data.language],
                      pace: data.pacePerDay,
                    })}
                  </>
                )
                  : reference
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
        {mobile && reference && (
          <p className={`vch-done__note ${stylex.props(channelStyles.doneNoteLine).className}`}>
            {directorFailed ? tt.noteTempM : tt.noteM(reference.title)}
          </p>
        )}

        <section className={`vch-card ${stylex.props(channelStyles.card, mobile ? channelStyles.doneCardM : channelStyles.doneCard).className}`} aria-label={directorFailed ? tt.needsYou : tt.background}>
          <div className={`vch-cardhead ${stylex.props(channelStyles.cardhead, channelStyles.doneCardHead, mobile && channelStyles.doneCardHeadM).className}`}>
            <h2 {...stylex.props(channelStyles.cardHeading, mobile && channelStyles.cardHeadingM)}>
              <span aria-hidden="true" className={`vch-recdot ${stylex.props(channelStyles.recdot, directorFailed && channelStyles.recdotWarn).className}`} />
              {directorFailed ? tt.needsYou : tt.background}
            </h2>
            {!(mobile && directorFailed) && (
              <span className={`vch-field__hint ${stylex.props(channelStyles.fieldHint).className}`}>{directorFailed ? tt.needsYouSub : (mobile ? tt.backgroundSubM : tt.backgroundSub)}</span>
            )}
          </div>
          <StepRow who="director" label={tt.director} step={director} mobile={mobile} onRetry={() => void retryDirector()} retrying={retrying} />
          <StepRow who="artist" label={tt.artist} step={artist} percent={percent} mobile={mobile} />
          <StepRow who="qa" label={tt.qa} step={{ state: 'ok', what: tt.qaWhat, text: tt.ok }} mobile={mobile} />
          {retryError && <p className={`vch-field__hint ${stylex.props(channelStyles.fieldError).className}`} role="alert">{retryError}</p>}
          <p className={`vch-done__foot ${stylex.props(channelStyles.doneFoot, mobile && channelStyles.doneFootM).className}`}>
            {mobile ? (directorFailed ? tt.footnoteTempM : tt.footnoteM) : (directorFailed ? tt.footnoteTemp : tt.footnote)}
          </p>
        </section>

        <section className={`vch-card ${stylex.props(channelStyles.card, mobile ? channelStyles.nextCardM : channelStyles.nextCard).className}`} aria-label={tt.next}>
          {video ? (
            mobile ? (
              <>
                <div className={`vch-next ${stylex.props(channelStyles.nextRow).className}`}>
                  <span className={`vch-next__thumb ${stylex.props(channelStyles.nextThumb).className}`}>
                    {video.thumbnailUrl && video.thumbnailUrl !== brokenThumbnail && <img className={`vch-next__img ${stylex.props(channelStyles.nextThumbImg).className}`} src={video.thumbnailUrl} alt="" loading="lazy" onError={() => setBrokenThumbnail(video.thumbnailUrl ?? null)} />}
                    {video.score != null && (
                      <span className={`vch-next__score ${stylex.props(channelStyles.nextThumbScore).className}`}>{fmtScore(video.score)}</span>
                    )}
                  </span>
                  <span className={`vch-next__text ${stylex.props(channelStyles.nextText).className}`}>
                    <span className={`vch-next__title ${stylex.props(channelStyles.nextTitle, channelStyles.nextTitleM).className}`} title={video.title}>{video.title}</span>
                    <span className={`vch-next__meta ${stylex.props(channelStyles.nextMeta).className}`}>{latestMetaM}</span>
                  </span>
                </div>
                <ActionButton
                  variant="primary"
                  size="lg"
                  block
                  pending={making}
                  onClick={() => void makeLatest()}
                  icon={<Icon name="video-add" size={18} />}
                >
                  {tt.makeLatestM}
                </ActionButton>
                <p className={`vch-next__hint ${stylex.props(channelStyles.nextHint).className}`}>{tt.nextHintM}</p>
              </>
            ) : (
              <div className={`vch-next ${stylex.props(channelStyles.next).className}`}>
                <ChannelAvatar
                  name={reference?.title ?? ''}
                  color={{ bg: channelTint(reference?.rivalId ?? data.id) }}
                  size={34}
                  style={{ fontSize: 11.5, fontWeight: 700 }}
                />
                <span className={`vch-next__text ${stylex.props(channelStyles.nextText).className}`}>
                  <span className={`vch-next__title ${stylex.props(channelStyles.nextTitle).className}`} title={video.title}>{video.title}</span>
                  <span className={`vch-next__meta ${stylex.props(channelStyles.nextMeta).className}`}>{latestMeta ? `${latestMeta} · ${tt.nextHint}` : tt.nextHint}</span>
                </span>
                <ActionButton
                  variant="primary"
                  size="md"
                  pending={making}
                  onClick={() => void makeLatest()}
                >
                  {tt.makeLatest(reference?.title ?? '')}
                </ActionButton>
              </div>
            )
          ) : latest.initialLoading ? (
            <Spinner size={16} color="var(--color-fg-3)" />
          ) : (
            <p className={`vch-hint ${stylex.props(channelStyles.hint).className}`}>{tt.latestEmpty}</p>
          )}
        </section>

        {mobile ? (
          <>
            {directorFailed && (
              <ActionButton size="lg" block icon={<Icon name="edit" size={16} />} onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id, 'settings')}` }}>
                {tt.nameInProfile}
              </ActionButton>
            )}
            <div className={`vch-actions ${stylex.props(channelStyles.actionsGridM).className}`}>
              <ActionButton size="lg" onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id)}` }}>{tt.openChannel}</ActionButton>
              <ActionButton size="lg" icon={<Icon name="radar" size={16} />} onClick={() => { onLeave(); window.location.hash = `#${paths.explore()}` }}>{tt.exploreShort}</ActionButton>
            </div>
          </>
        ) : (
          <div className={`vch-actions ${stylex.props(channelStyles.actions).className}`}>
            <ActionButton size="md" onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id)}` }}>
              {tt.openChannel}
            </ActionButton>
            {/* r38: Đạo diễn chưa đặt được tên → cho đặt tay ở Hồ sơ kênh. */}
            {directorFailed && (
              <ActionButton size="md" icon={<Icon name="edit" size={15} />} onClick={() => { onLeave(); window.location.hash = `#${paths.channel(data.id, 'settings')}` }}>
                {tt.nameInProfile}
              </ActionButton>
            )}
            <ActionButton size="md" variant="quiet" icon={<Icon name="radar" size={15} />} onClick={() => { onLeave(); window.location.hash = `#${paths.explore()}` }}>
              {tt.explore}
            </ActionButton>
          </div>
        )}
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
  const toneClass = step.state === 'run' ? ' vch-task__state--run'
    : step.state === 'fail' ? ' vch-task__state--fail'
      : step.state === 'wait' ? ' vch-task__state--wait' : ''
  const tone = step.state === 'run' ? channelStyles.taskStateRun
    : step.state === 'fail' ? channelStyles.taskStateFail
      : step.state === 'wait' ? channelStyles.taskStateWait : null
  const whoTone = who === 'director' ? channelStyles.taskWhoDirector
    : who === 'artist' ? channelStyles.taskWhoArtist : channelStyles.taskWhoQa
  const state = (
    <span className={`vch-task__state${toneClass} ${stylex.props(channelStyles.taskState, mobile && channelStyles.taskStateM, tone).className}`}>
      {step.state === 'ok' && <Icon name="tick-b" size={mobile ? 12 : 13} />}
      {step.text}
      {!mobile && step.retry && onRetry && (
        <ActionButton
          variant="outline"
          size="sm"
          icon={<Icon name="refresh" size={14} />}
          pending={retrying}
          onClick={onRetry}
        >
          {t.common.retry}
        </ActionButton>
      )}
    </span>
  )
  return (
    <div className={`vch-task ${stylex.props(channelStyles.task, mobile && channelStyles.taskM).className}`}>
      <Orb of={who} size={20} live={step.state === 'run'} />
      <span className={`vch-task__text ${stylex.props(channelStyles.taskText).className}`}>
        <span className={`vch-task__line ${stylex.props(channelStyles.taskLine).className}`}>
          <span className={`vch-task__who ${stylex.props(channelStyles.taskWho, whoTone).className}`}>{label}</span>
          <span className={`vch-task__what ${stylex.props(channelStyles.taskWhat).className}`}>{step.what}</span>
        </span>
        {mobile && state}
        {step.state === 'run' && percent !== undefined && (
          <span className={`vch-task__bar ${stylex.props(channelStyles.taskBar).className}`}><ProgressBar value={percent} tone="warn" height={4} /></span>
        )}
      </span>
      {!mobile && state}
      {mobile && step.retry && onRetry && (
        <ActionButton
          variant="outline"
          size="lg"
          icon={<Icon name="refresh" size={15} />}
          pending={retrying}
          onClick={onRetry}
        >
          {t.common.retry}
        </ActionButton>
      )}
    </div>
  )
}
