// Chi tiết video trên điện thoại (artboard MCompVideo). Là màn đẩy thật của
// route #/explore?video=<id>: thanh riêng 52, hero 16:9, điểm × + Vì sao?,
// nguồn, chương (máy chủ chưa có API chương — ghi rõ trạng thái, không bịa
// hàng), chọn kênh và CTA dính đáy. Không thanh tab, không header chung.
import { useCallback, useEffect, useMemo, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionSheet, Button, Icon, MobilePage, MobilePageAction, RadioCard, RadioDot, Sheet, toast, useTabMode,
} from '@noverse/ui'
import { t } from '../../../i18n/explore'
import type { Route } from '../../../router'
import { navigate, paths } from '../../../router'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { ChannelAvatar } from '../../../components'
import { listRivals, type Video } from '../../../api/rivals'
import { listChannels } from '../../../api/channels'
import { enqueueClone, planClone, type ClonePlan } from '../../../api/clone'
import { fmtDuration, fmtScore, scoreText, videoMeta } from '../fmt'
import { hideBrokenImage } from '../VideoGrid'
import { scopeOf } from '../scope'
import { useRivalVideos } from '../useRivalVideos'
import { ReasonPopover } from '../../../components/reason/ReasonPopover'
import { mStyles } from './mexplore.stylex'
import { recallVideo } from './pickedVideo'

type ExploreRoute = Extract<Route, { name: 'explore' }>

const clock = (iso: string | null): string => {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

export function MVideoScreen({ route }: { route: ExploreRoute }) {
  const tt = t.screens.explore
  useTabMode('hide')
  const videoId = route.video ?? ''
  const [cached] = useState<Video | null>(() => recallVideo(videoId))

  const list = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const rivals = useMemo(() => list.data?.rivals ?? [], [list.data])
  const channelList = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const channels = useMemo(
    () => (channelList.data?.channels ?? []).map((channel) => ({
      id: channel.id,
      name: channel.name || t.channels.page.noName,
      templateName: channel.templates?.[channel.templateCursor % channel.templates.length]?.name,
    })),
    [channelList.data],
  )

  // Liên kết trực tiếp không có bản nhớ: tìm trong trang đầu mặc định (mọi thời
  // gian) của phạm vi. API chưa có endpoint một video — ghi ở báo cáo.
  const videos = useRivalVideos({ scope: scopeOf(route.rival, rivals), sort: 'outlier', long: false, within: 0, q: '' })
  const video = (cached?.id === videoId ? cached : null) ?? videos.items.find((item) => item.id === videoId) ?? null
  const rival = video ? rivals.find((item) => item.id === video.channelId) : undefined

  const [whyOpen, setWhyOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [pickerOpen, setPickerOpen] = useState(false)
  const [picked, setPicked] = useState<string>('')
  const [plans, setPlans] = useState<Record<string, ClonePlan>>({})
  const [planPhase, setPlanPhase] = useState<'idle' | 'loading' | 'ready' | 'error'>('idle')
  const [planRetry, setPlanRetry] = useState(0)
  const [busy, setBusy] = useState(false)
  const [queued, setQueued] = useState(false)

  useEffect(() => {
    if (!pickerOpen || !video || channels.length === 0) return
    const ctrl = new AbortController()
    setPlanPhase('loading')
    setPicked((current) => current || channels[0].id)
    Promise.all(channels.map(async (channel) => {
      const plan = await planClone({ channelId: channel.id, urls: [video.url], priority: 'pick' }, ctrl.signal)
      return [channel.id, plan] as const
    }))
      .then((entries) => {
        if (ctrl.signal.aborted) return
        setPlans(Object.fromEntries(entries))
        setPlanPhase('ready')
      })
      .catch(() => { if (!ctrl.signal.aborted) setPlanPhase('error') })
    return () => ctrl.abort()
  }, [pickerOpen, video, channels, planRetry])

  const selectedPlan = plans[picked]
  const duplicate = (selectedPlan?.duplicates.length ?? 0) > 0
  const canMake = planPhase === 'ready' && !!selectedPlan?.accepted.length && !duplicate && !!selectedPlan.template.id && !busy && !queued
  const planHint = channels.length === 0
    ? tt.pickerEmpty
    : planPhase === 'loading'
      ? tt.queuePlanLoading
      : duplicate
        ? tt.queuePlanDup
        : selectedPlan?.etaStart
          ? tt.queuePlanEta(clock(selectedPlan.etaStart))
          : tt.queuePlan

  const copy = useCallback(async () => {
    if (!video) return
    try {
      await navigator.clipboard.writeText(video.url)
      toast.ok(tt.copied)
    } catch {
      toast.err(tt.copyFailed)
    }
  }, [video, tt.copied, tt.copyFailed])

  const back = useCallback(() => {
    navigate(paths.explore({ rival: route.rival }), true)
  }, [route.rival])

  const make = async () => {
    if (!video || !picked || !canMake) return
    setBusy(true)
    try {
      await enqueueClone({ channelId: picked, urls: [video.url], priority: 'pick' })
      setQueued(true)
      setPickerOpen(false)
      toast(t.screens.new.toasts.made, 'ok', { label: t.screens.new.toasts.openQueue, onClick: () => navigate(paths.queue()) })
    } catch {
      toast.err(t.screens.new.toasts.failed)
    } finally {
      setBusy(false)
    }
  }

  if (!video && (videos.initialLoading || list.initialLoading)) {
    return <MobilePage title={tt.sourceTitle} onBack={back}><div role="status" {...stylex.props(mStyles.notFound)}>{tt.queuePlanLoading}</div></MobilePage>
  }

  if (!video) {
    return (
      <MobilePage title={tt.sourceTitle} onBack={back}>
        <div {...stylex.props(mStyles.notFound)}>
          <Icon name="search" size={24} />
          <span {...stylex.props(mStyles.notFoundTitle)}>{tt.detailNotFoundTitle}</span>
          <p {...stylex.props(mStyles.notFoundBody)}>{tt.detailNotFoundBody}</p>
          <Button size="lg" onClick={back}>{tt.backToExplore}</Button>
        </div>
      </MobilePage>
    )
  }

  const hot = (video.score ?? 0) >= 3
  const dur = fmtDuration(video.durationSec)
  const meta = [rival?.title, videoMeta(video.viewCount, video.publishedAt), video.hasCaptions === true ? tt.detailCaptions : '']
    .filter(Boolean).join(' · ')
  const selectedChannel = channels.find((channel) => channel.id === picked)

  return (
    <MobilePage
      title={(
        <span {...stylex.props(mStyles.detailBarTitle)}>
          <ChannelAvatar name={rival?.title ?? video.title} src={rival?.avatarUrl ?? undefined} size={24} fontSize={9} />
          <span {...stylex.props(mStyles.detailBarName)}>{rival?.title ?? tt.sourceTitle}</span>
        </span>
      )}
      onBack={back}
      actions={(
        <>
          <MobilePageAction icon="copy" label={tt.copyLinkTitle} onClick={() => void copy()} />
          <MobilePageAction icon="more" label={tt.openYouTube} onClick={() => setMoreOpen(true)} />
        </>
      )}
      footer={(
        <div {...stylex.props(mStyles.footer)}>
          <Button
            variant="primary"
            size="xl"
            block
            icon={<Icon name="video-add" size={18} />}
            disabled={queued}
            onClick={() => setPickerOpen(true)}
          >
            {queued ? tt.queuedLabel : tt.detailMake}
          </Button>
          <span {...stylex.props(mStyles.footerHint)}>{queued ? tt.queuedDone : tt.detailMakeHint}</span>
        </div>
      )}
    >
      <div {...stylex.props(mStyles.detail)}>
        <div {...stylex.props(mStyles.thumbWrap)}>
          <div {...stylex.props(mStyles.thumbFrame)}>
            <img {...stylex.props(mStyles.thumbImg)} src={video.thumbnailUrl} alt="" onError={hideBrokenImage} />
            <span {...stylex.props(mStyles.score, hot ? mStyles.scoreHot : mStyles.scoreLow)}>{fmtScore(video.score)}</span>
            {dur && <span {...stylex.props(mStyles.dur)}>{dur}</span>}
          </div>
        </div>

        <h1 {...stylex.props(mStyles.detailTitle)}>{video.title}</h1>
        <span {...stylex.props(mStyles.detailMeta)}>{meta}</span>

        <div {...stylex.props(mStyles.scoreRow)}>
          <span {...stylex.props(mStyles.scoreStatic, hot ? mStyles.scoreHot : mStyles.scoreLow)}>{fmtScore(video.score)}</span>
          <span {...stylex.props(mStyles.scoreText)}>{tt.scoreTimes(scoreText(video.score))}</span>
          {video.scoreReason && (
            <Button
              size="lg"
              icon={<Icon name="info" size={13} />}
              aria-haspopup="dialog"
              aria-expanded={whyOpen}
              onClick={() => setWhyOpen(true)}
            >
              {tt.why}
            </Button>
          )}
        </div>

        {rival && (
          <a href={video.url} target="_blank" rel="noreferrer" {...stylex.props(mStyles.detailSourceLink)}>
            <div {...stylex.props(mStyles.detailCard)}>
              <ChannelAvatar name={rival.title} src={rival.avatarUrl ?? undefined} size={32} fontSize={11} />
              <span {...stylex.props(mStyles.detailCardText)}>
                <span {...stylex.props(mStyles.detailCardTitle)}>{tt.sourceTitle} · {rival.title}</span>
                <span {...stylex.props(mStyles.detailCardSub)}>{tt.openYouTube}</span>
              </span>
              <Icon name="arrow-r" size={16} />
            </div>
          </a>
        )}

        <div {...stylex.props(mStyles.section)}>
          <div {...stylex.props(mStyles.sectionHead)}>
            <Icon name="doc" size={16} />
            <span {...stylex.props(mStyles.sectionTitle)}>{tt.chaptersTitle}</span>
            <span {...stylex.props(mStyles.sectionCount)}>—</span>
          </div>
          <p {...stylex.props(mStyles.sectionNote)}>{tt.chaptersPending}</p>
        </div>
      </div>

      <Sheet open={whyOpen} onClose={() => setWhyOpen(false)} size="md" title={tt.whyTitle} height="auto" bodyStyle={mStyles.sheetBody}>
        {video.scoreReason && <ReasonPopover reason={video.scoreReason} label={tt.whyTitle} />}
      </Sheet>

      <ActionSheet
        open={moreOpen}
        onClose={() => setMoreOpen(false)}
        title={video.title}
        actions={[
          { key: 'youtube', label: tt.openYouTube, icon: 'link', onSelect: () => { window.open(video.url, '_blank', 'noreferrer') } },
          { key: 'copy', label: tt.copyLinkTitle, icon: 'copy', onSelect: copy },
          { key: 'back', label: tt.backToExplore, icon: 'arrow-left-2', onSelect: back },
        ]}
      />

      <Sheet open={pickerOpen} onClose={() => setPickerOpen(false)} size="md" title={tt.pickerTitle} height="auto" bodyStyle={mStyles.sheetBody}>
        <div {...stylex.props(mStyles.pickerBody)}>
          {channels.length === 0 ? (
            <p {...stylex.props(mStyles.addHint)}>{tt.pickerEmpty}</p>
          ) : (
            <div role="radiogroup" aria-label={tt.pickerTitle} {...stylex.props(mStyles.pickerGroup)}>
              {channels.map((channel) => {
                const plan = plans[channel.id]
                const dup = (plan?.duplicates.length ?? 0) > 0
                const on = picked === channel.id
                return (
                  <RadioCard
                    key={channel.id}
                    checked={on}
                    onChange={() => setPicked(channel.id)}
                    disabled={dup}
                    mark="none"
                    xstyle={[mStyles.pickerRow, on && mStyles.pickerRowOn, dup && mStyles.pickerRowOff]}
                  >
                    <span {...stylex.props(mStyles.pickerRowContent)}>
                      <ChannelAvatar name={channel.name} size={32} fontSize={11} />
                      <span {...stylex.props(mStyles.pickerText)}>
                        <span {...stylex.props(mStyles.pickerName)}>{channel.name}</span>
                        <span {...stylex.props(mStyles.pickerSub, dup && mStyles.pickerSubOn)}>
                          {dup
                            ? tt.pickerDuplicate
                            : plan?.template.name
                              ? tt.pickerNext(plan.template.name)
                              : channel.templateName ? tt.pickerNext(channel.templateName) : tt.pickerNoTemplate}
                        </span>
                      </span>
                    </span>
                    <RadioDot checked={on} />
                  </RadioCard>
                )
              })}
            </div>
          )}
          <span {...stylex.props(mStyles.pickerHint)}><Icon name="clock" size={15} />{planHint}</span>
          {planPhase === 'error' && (
            <div role="alert" {...stylex.props(mStyles.planError)}>
              <p {...stylex.props(mStyles.planErrorText)}>{tt.detailPlanFailed}</p>
              <Button size="lg" onClick={() => setPlanRetry((n) => n + 1)}>{tt.listRetry}</Button>
            </div>
          )}
          <Button
            variant="primary"
            size="xl"
            block
            icon={<Icon name="video-add" size={18} />}
            pending={busy}
            disabled={!canMake}
            onClick={() => void make()}
          >
            {queued ? tt.queuedLabel : tt.detailMakeFor(selectedChannel?.name ?? '')}
          </Button>
        </div>
      </Sheet>
    </MobilePage>
  )
}
