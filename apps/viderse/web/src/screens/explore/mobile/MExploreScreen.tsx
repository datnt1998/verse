// Khám phá trên điện thoại (artboard MExplore). Một vùng cuộn của pane:
// tiêu đề + Thông tin/Chọn 44, dải đối thủ ngang, hàng Sắp xếp + lọc ngang,
// dòng đếm, thẻ video 16:9. Chế độ chọn thay thanh tab bằng thanh chọn dính
// đáy; tấm thông tin đối thủ và tấm thêm đối thủ dùng Sheet của kit.
import { useCallback, useEffect, useMemo, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import {
  ActionSheet, Button, Checkbox, ConfirmSheet, Icon, RadioCard, Segmented, Sheet, TextField, toast, useMobilePaneActive, useTabMode,
} from '@noverse/ui'
import { ResponsiveChipSelect } from '../../../components/responsive-overlays'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/explore'
import type { Route } from '../../../router'
import { navigate, paths } from '../../../router'
import { useResource } from '../../../data/useResource'
import { keys } from '../../../data/keys'
import { events, useEventsStatus } from '../../../data/events'
import { useDebounced } from '../../../lib/useDebounced'
import { ChannelAvatar, Orb } from '../../../components'
import {
  controlRivalScan, getPostingSlots, getRivalScan, listRivals, removeRival,
  type PostingZone, type Rival, type Video, type VideoSort,
} from '../../../api/rivals'
import { linkSource, listChannels } from '../../../api/channels'
import { addRival } from '../../../api/rivals'
import { enqueueClone } from '../../../api/clone'
import { scanProgress, scanningRival, exploreListState } from '../listState'
import { ListStateBlock, ScanBanner, SkeletonTiles } from '../ListStates'
import { ScanCard } from '../ScanCard'
import { PostingCard } from '../PostingCard'
import { WITHIN, type StatusFilter } from '../FilterBar'
import { bulkEnqueueOutcome } from '../bulk'
import { addRivalError, type AddError } from '../AddRivalForm'
import { fmtDuration, fmtScore, fmtSubs, videoMeta } from '../fmt'
import { copyText } from '../media'
import { hideBrokenImage } from '../VideoGrid'
import { stageColor, stageLabel, usageHref } from '../status'
import { scopeOf, type Scope } from '../scope'
import { useRivalVideos, type VideosFilter } from '../useRivalVideos'
import { mStyles } from './mexplore.stylex'
import { MobileVideoList } from './MobileVideoList'
import { rememberVideo } from './pickedVideo'

const num = new Intl.NumberFormat('vi-VN')

type ExploreRoute = Extract<Route, { name: 'explore' }>

const handleOf = (scope: Scope, rivals: Rival[]): string | undefined =>
  scope === 'all' ? undefined : (rivals.find((rival) => rival.id === scope)?.handle ?? scope)

export function MExploreScreen({ route }: { route: ExploreRoute }) {
  const tt = t.screens.explore
  const list = useResource(keys.rivals(), ({ signal }) => listRivals(signal))
  const rivals = useMemo(() => list.data?.rivals ?? [], [list.data])
  const channelList = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const channels = useMemo(
    () => (channelList.data?.channels ?? []).map((channel) => ({
      id: channel.id,
      name: channel.name || t.channels.page.noName,
      perDay: channel.perDay,
      sources: channel.sources,
      templateName: channel.defaultTemplateName,
    })),
    [channelList.data],
  )

  const scope = scopeOf(route.rival, rivals)
  const scopeRival = scope === 'all' ? undefined : rivals.find((rival) => rival.id === scope)
  const infoRival = scopeRival ?? rivals[0]

  const pickRival = useCallback((id: Scope) => {
    navigate(paths.explore({ rival: handleOf(id, rivals) }), true)
  }, [rivals])

  const [sort, setSort] = useState<VideoSort>('outlier')
  const [long, setLong] = useState(false)
  const [within, setWithin] = useState<0 | 7 | 30 | 90>(30)
  // Chip "Trạng thái" lọc thật ở máy chủ (cùng tham số made/notMade như bản máy tính).
  const [status, setStatus] = useState<StatusFilter>('all')
  const [query, setQuery] = useState('')
  const typedQ = query.trim()
  const debouncedQ = useDebounced(typedQ, typedQ === '' ? 0 : 250)
  const q = typedQ === '' ? '' : debouncedQ
  const filter: VideosFilter = useMemo(
    () => ({ scope, sort, long, within, q, status }),
    [scope, sort, long, within, q, status],
  )
  const videos = useRivalVideos(filter)
  const items = videos.items
  const scanned = scopeRival ? scopeRival.videoCount : list.data?.totalVideos ?? 0
  const withinLabel = WITHIN.find((w) => w.value === within)?.label
  const activeFilters = [
    ...(long ? [tt.noMatchLong] : []),
    ...(within > 0 && withinLabel ? [tt.noMatchWithin(withinLabel)] : []),
    ...(status === 'notMade' ? [tt.noMatchStatusNotMade] : []),
    ...(status === 'made' ? [tt.noMatchStatusMade] : []),
  ]
  const eventsStatus = useEventsStatus()
  const offline = eventsStatus.state === 'retrying'
  const scanning = scanningRival(rivals, scope)
  const listState = exploreListState({
    count: items.length,
    loading: videos.initialLoading,
    error: videos.error,
    offline,
    scanning,
    q: videos.shownQ,
  })
  const [clock, setClock] = useState(() => Date.now())
  useEffect(() => {
    if (listState !== 'offline' || eventsStatus.reconnectAt == null) return
    const timer = setInterval(() => setClock(Date.now()), 500)
    return () => clearInterval(timer)
  }, [listState, eventsStatus.reconnectAt])
  const retryIn = eventsStatus.reconnectAt == null ? 8 : Math.max(0, Math.ceil((eventsStatus.reconnectAt - clock) / 1000))
  const scopeName = scopeRival?.title ?? tt.noMatchAll
  const fresh = scopeRival?.newOutliers ?? list.data?.newOutliers ?? 0
  const retryList = useCallback(() => {
    void videos.refresh()
    void list.refresh()
  }, [videos.refresh, list.refresh])
  const retryNow = useCallback(() => {
    events.retryNow()
    retryList()
  }, [retryList])
  const clearFilters = useCallback(() => {
    setLong(false)
    setWithin(30)
    setStatus('all')
  }, [])

  // ── chọn nhiều ───────────────────────────────────────────────────────────
  const [selectMode, setSelectMode] = useState(false)
  useTabMode(selectMode ? 'hide' : undefined)
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set())
  const toggleChecked = useCallback((video: Video) => {
    setCheckedIds((prev) => {
      const next = new Set(prev)
      if (next.has(video.id)) next.delete(video.id)
      else next.add(video.id)
      return next
    })
  }, [])
  const selectedVideos = useMemo(() => items.filter((v) => checkedIds.has(v.id)), [items, checkedIds])
  const exitSelect = useCallback(() => { setSelectMode(false); setCheckedIds(new Set()) }, [])
  const selectTop = useCallback((n: number) => {
    setCheckedIds(new Set(items.slice(0, n).map((v) => v.id)))
  }, [items])
  const [bulkBusy, setBulkBusy] = useState(false)
  const [bulkPickerOpen, setBulkPickerOpen] = useState(false)
  const bulkEnqueue = useCallback(async (channelId: string) => {
    if (bulkBusy || !channels.some((channel) => channel.id === channelId) || selectedVideos.length === 0) return
    setBulkBusy(true)
    try {
      const result = await enqueueClone({ channelId, urls: selectedVideos.map((v) => v.url), priority: 'batch' })
      const outcome = bulkEnqueueOutcome(result, selectedVideos)
      // Toàn bộ đã có sẵn: không báo thành công, giữ nguyên lựa chọn và mở job có sẵn.
      if (outcome.added === 0 && outcome.duplicateCount > 0) {
        toast(tt.bulkAllDuplicate(String(outcome.duplicateCount)), 'warn', {
          label: outcome.duplicateJobId ? tt.queuePlanOpen : t.screens.new.toasts.openQueue,
          onClick: () => navigate(outcome.duplicateJobId ? paths.workbench({ video: outcome.duplicateJobId }) : paths.queue()),
        })
        return
      }
      if (outcome.added === 0) {
        toast.err(t.screens.new.toasts.failed)
        return
      }
      if (outcome.duplicateCount > 0) {
        // Chỉ đếm video thật sự vào hàng chờ; giữ lựa chọn đúng nhóm trùng để chọn kênh khác.
        setCheckedIds(new Set(outcome.duplicateVideoIds))
        toast(tt.bulkMadePartial(String(outcome.added), String(selectedVideos.length), String(outcome.duplicateCount)), 'ok', {
          label: t.screens.new.toasts.openQueue,
          onClick: () => navigate(paths.queue()),
        })
        return
      }
      toast(t.screens.new.toasts.made, 'ok', { label: t.screens.new.toasts.openQueue, onClick: () => navigate(paths.queue()) })
      exitSelect()
    } catch {
      toast.err(t.screens.new.toasts.failed)
    } finally { setBulkBusy(false) }
  }, [bulkBusy, channels, selectedVideos, exitSelect, tt.bulkAllDuplicate, tt.bulkMadePartial, tt.queuePlanOpen])

  // ── tấm thông tin / thêm đối thủ ─────────────────────────────────────────
  const [infoOpen, setInfoOpen] = useState(false)
  const [addOpen, setAddOpen] = useState(false)
  const [zone, setZone] = useState<PostingZone>('vn')
  const [infoMenuOpen, setInfoMenuOpen] = useState(false)
  const [infoLinkOpen, setInfoLinkOpen] = useState(false)
  const [unfollowOpen, setUnfollowOpen] = useState(false)
  const [addValue, setAddValue] = useState('')
  const [addError, setAddError] = useState<AddError | null>(null)
  const [addPending, setAddPending] = useState(false)
  const [addLinks, setAddLinks] = useState<Set<string>>(new Set())
  const paneActive = useMobilePaneActive()
  useEffect(() => {
    if (paneActive) return
    setInfoOpen(false)
    setAddOpen(false)
    setInfoMenuOpen(false)
    setInfoLinkOpen(false)
    setUnfollowOpen(false)
    setBulkPickerOpen(false)
  }, [paneActive])

  const infoScan = useResource(
    keys.rivalScan(infoRival?.id ?? '_none'),
    ({ signal }) => getRivalScan(infoRival!.id, signal),
    { enabled: !!infoRival, keepPrevious: true },
  )
  const infoPosting = useResource(
    keys.rivalPosting(infoRival?.id ?? '_none', zone),
    ({ signal }) => getPostingSlots(infoRival!.id, zone, signal),
    { enabled: !!infoRival, keepPrevious: true },
  )
  const linkedChannel = infoRival
    ? channels.find((channel) => channel.sources.some((source) => source.rivalId === infoRival.id))
    : undefined

  const openInfo = () => {
    if (!infoRival) return
    setAddLinks(new Set())
    setInfoOpen(true)
  }
  const openAdd = () => {
    setAddValue('')
    setAddError(null)
    setAddLinks(new Set(channels[0] ? [channels[0].id] : []))
    setAddOpen(true)
  }
  const submitAdd = async () => {
    const input = addValue.trim()
    if (!input || addPending) return
    setAddPending(true)
    setAddError(null)
    try {
      const rival = await addRival(input)
      void invalidate(keys.rivals())
      if (addLinks.size > 0) {
        try {
          await Promise.all([...addLinks].map((channelId) => linkSource(channelId, rival.id)))
          void invalidate(keys.channels())
        } catch {
          toast.err(tt.linkFailed)
        }
      }
      setAddValue('')
      setAddOpen(false)
      pickRival(rival.id)
    } catch (err) {
      setAddError(addRivalError(err))
    } finally {
      setAddPending(false)
    }
  }
  const checkInfo = () => {
    if (!infoRival) return
    controlRivalScan(infoRival.id, 'check')
      .catch((err) => toast.err(err instanceof Error ? err.message : tt.addErrors.failed))
  }
  const unfollow = async () => {
    if (!infoRival) return
    await removeRival(infoRival.id)
    void invalidate(keys.rivals())
    void invalidate(keys.rivalScan(infoRival.id))
    setInfoOpen(false)
    pickRival('all')
  }


  const openVideo = useCallback((video: Video) => {
    rememberVideo(video)
    setSelectMode(false)
    navigate(paths.explore({ rival: handleOf(scope, rivals), video: video.id }))
  }, [scope, rivals])
  const copy = useCallback(async (url: string) => {
    const ok = await copyText(url)
    if (ok) toast.ok(tt.copied)
    else toast.err(tt.copyFailed)
  }, [tt.copied, tt.copyFailed])

  const countLine = listState === 'firstScan' && scanning
    ? tt.mobileCountScanning(String(scanProgress(scanning).done), String(scanProgress(scanning).total), scanning.title)
    : listState === 'filterEmpty'
      ? tt.mobileCountFiltered(num.format(scanned), scopeName)
      : listState === 'error' || listState === 'offline'
        ? tt.mobileCountScanned(num.format(scanned), scopeName)
        : listState === 'loading'
          ? ''
          : tt.mobileCount(
            num.format(videos.total ?? items.length),
            num.format(scanned),
            scopeName,
            fresh > 0 ? String(fresh) : '',
          )

  const empty = !list.initialLoading && rivals.length === 0 && list.error == null
  const firstChannel = channels[0]

  return (
    <div {...stylex.props(mStyles.root, empty && mStyles.rootEmpty, selectMode && mStyles.selectionPad)}>
      <div {...stylex.props(mStyles.header)}>
        <h1 {...stylex.props(mStyles.title)}>{tt.mobileTitle}</h1>
        {!empty && infoRival && (
          <Button variant="icon" size="lg" aria-label={tt.mobileInfoAria(infoRival.title)} xstyle={mStyles.headerAction} onClick={openInfo}>
            <Icon name="info-circle" size={18} />
          </Button>
        )}
        {!empty && (
          <Button
            variant={selectMode ? 'ghost' : 'quiet'}
            size="lg"
            icon={<Icon name="tick" size={16} />}
            aria-pressed={selectMode}
            active={selectMode}
            xstyle={mStyles.headerAction}
            onClick={() => (selectMode ? exitSelect() : setSelectMode(true))}
          >
            {selectMode ? tt.exitSelect : tt.mobileSelect}
          </Button>
        )}
      </div>

      {empty ? (
        <div {...stylex.props(mStyles.noRivalsBox)}>
          <Orb of="scout" size={40} live={false} />
          <span {...stylex.props(mStyles.noRivalsTitle)}>{tt.emptyTitle}</span>
          <p {...stylex.props(mStyles.emptyBody)}>{tt.emptyBody}</p>
          <Button variant="primary" size="xl" block icon={<Icon name="add" size={18} />} xstyle={mStyles.noRivalsFollow} onClick={openAdd}>
            {tt.followRival}
          </Button>
          <span {...stylex.props(mStyles.emptyExamples)}>
            <span>{tt.example}</span>
            {tt.emptyExamples.map((example) => <code key={example} {...stylex.props(mStyles.tag)}>{example}</code>)}
          </span>
        </div>
      ) : (
        <>
          <div {...stylex.props(mStyles.chipsRow)} role="radiogroup" aria-label={tt.rivalsGroup}>
            <RadioCard checked={scope === 'all'} tabIndex={scope === 'all' ? 0 : -1} onChange={() => pickRival('all')} mark="none" xstyle={mStyles.rivalChip}>
              <Icon name="radar" size={16} /><span {...stylex.props(mStyles.rivalChipName)}>{tt.allRivals}</span>
            </RadioCard>
            {rivals.map((rival) => (
              <RadioCard key={rival.id} checked={scope === rival.id} tabIndex={scope === rival.id ? 0 : -1} onChange={() => pickRival(rival.id)} mark="none" xstyle={mStyles.rivalChip}>
                <ChannelAvatar name={rival.title} src={rival.avatarUrl ?? undefined} size={30} fontSize={11} />
                <span {...stylex.props(mStyles.rivalChipName)} title={rival.title}>{rival.title}</span>
                {rival.newOutliers > 0 && (
                  <span {...stylex.props(mStyles.newBadge)} aria-label={tt.newBadgeAria(String(rival.newOutliers))}>
                    {rival.newOutliers}
                  </span>
                )}
              </RadioCard>
            ))}
            <Button
              variant="ghost"
              size="lg"
              xstyle={mStyles.followChip}
              aria-label={tt.mobileAddAria}
              icon={<Icon name="add" size={16} />}
              onClick={openAdd}
            >
              {tt.follow}
            </Button>
          </div>

          <div {...stylex.props(mStyles.filterRow)}>
            <Segmented
              label={tt.sortLabel}
              size={44}
              fit="content"
              xstyle={mStyles.filterSeg}
              value={sort}
              onChange={(id) => setSort(id as VideoSort)}
              options={[
                { id: 'outlier', label: tt.sorts.outlier },
                { id: 'newest', label: tt.sorts.newest },
                { id: 'views', label: tt.sorts.views },
              ]}
            />
            <Button
              variant="ghost"
              size="lg"
              aria-pressed={long}
              active={long}
              icon={<Icon name="clock" size={15} />}
              onClick={() => setLong(!long)}
            >
              {tt.filterLong} <b>{tt.filterLongValue}</b>
            </Button>
            <ResponsiveChipSelect
              label={tt.withinLabel}
              size="m"
              options={WITHIN.map((w) => ({ value: String(w.value), label: w.label }))}
              value={String(within)}
              onChange={(value) => setWithin(Number(value) as 0 | 7 | 30 | 90)}
              aria-label={tt.withinLabel}
              icon={<Icon name="calendar" size={15} />}
              prefix={tt.filterWithin}
              custom={within !== 30}
            />
            <ResponsiveChipSelect
              label={tt.statusLabel}
              size="m"
              options={[
                { value: 'all', label: tt.statusOptions.all },
                { value: 'notMade', label: tt.statusOptions.notMade },
                { value: 'made', label: tt.statusOptions.made },
              ]}
              value={status}
              onChange={(value) => setStatus(value as StatusFilter)}
              aria-label={tt.statusLabel}
              icon={<Icon name="filter" size={15} />}
              prefix={tt.filterStatus}
              custom={status !== 'all'}
            />
          </div>

          <span {...stylex.props(mStyles.count)}>{countLine}</span>

          {listState === 'firstScan' && scanning ? (
            <>
              <ScanBanner rival={scanning} />
              <SkeletonTiles />
            </>
          ) : listState === 'filterEmpty' ? (
            <ListStateBlock
              tone="accent"
              icon="filter"
              title={tt.listFilterTitle}
              text={tt.listFilterText(activeFilters, num.format(scanned), null)}
              action={tt.listFilterClear}
              actionIcon="close"
              onAction={clearFilters}
            />
          ) : listState === 'error' ? (
            <ListStateBlock
              tone="danger"
              icon="danger"
              title={tt.listErrorTitle}
              text={tt.listErrorText(num.format(scanned))}
              action={tt.listRetry}
              actionIcon="refresh"
              onAction={retryList}
            />
          ) : listState === 'offline' ? (
            <ListStateBlock
              tone="danger"
              icon="danger"
              title={tt.listOfflineTitle}
              text={tt.listOfflineTextMobile(retryIn)}
              action={tt.listRetryNow}
              actionIcon="refresh"
              onAction={retryNow}
            />
          ) : listState === 'loading' ? (
            <SkeletonTiles />
          ) : listState === 'nomatch' ? (
            <div {...stylex.props(mStyles.emptyBox)} role="status">
              <Icon name="search" size={22} />
              <span {...stylex.props(mStyles.emptyTitle)}>{tt.noMatchTitle(videos.shownQ)}</span>
              <p {...stylex.props(mStyles.emptyBody)}>{tt.noMatchWhere(num.format(scanned), scopeRival?.title ?? tt.noMatchAll, activeFilters)}</p>
              <Button size="lg" onClick={() => setQuery('')}>{tt.searchClear}</Button>
            </div>
          ) : (
            <MobileVideoList items={items} hasMore={videos.hasMore} needMore={videos.needMore}>
              {(video) => {
                const checked = checkedIds.has(video.id)
                const hot = (video.score ?? 0) >= 3
                const dur = fmtDuration(video.durationSec)
                const rival = rivals.find((r) => r.id === video.channelId)
                const meta = [rival?.title, videoMeta(video.viewCount, video.publishedAt)].filter(Boolean).join(' · ')
                const usage = video.usage?.[0]
                const usageTitle = video.usage?.map((u) => `${u.channelName} · ${stageLabel(u)}`).join('\n')
                const activate = () => (selectMode ? toggleChecked(video) : openVideo(video))
                return (
                  <article key={video.id} {...stylex.props(mStyles.card)}>
                    <div {...stylex.props(mStyles.thumbWrap)}>
                      <button
                        type="button"
                        {...stylex.props(mStyles.thumb)}
                        aria-label={`${tt.preview}: ${video.title}`}
                        onClick={activate}
                      >
                        <img {...stylex.props(mStyles.thumbImg)} src={video.thumbnailUrl} width={640} height={360} alt="" loading="lazy" onError={hideBrokenImage} />
                        <span {...stylex.props(mStyles.score, hot ? mStyles.scoreHot : mStyles.scoreLow)}>{fmtScore(video.score)}</span>
                        {dur && <span {...stylex.props(mStyles.dur)}>{dur}</span>}
                      </button>
                      {selectMode && (
                        <Checkbox
                          checked={checked}
                          onChange={() => toggleChecked(video)}
                          size={18}
                          tone="mono"
                          aria-label={tt.pickVideo(video.title)}
                          xstyle={mStyles.check}
                        />
                      )}
                    </div>
                    <div {...stylex.props(mStyles.cardBody)}>
                      <div {...stylex.props(mStyles.cardText)}>
                        <a
                          {...stylex.props(mStyles.cardTitle)}
                          href={`#${paths.explore({ rival: handleOf(scope, rivals), video: video.id })}`}
                          onClick={(event) => { event.preventDefault(); activate() }}
                        >{video.title}</a>
                        <span {...stylex.props(mStyles.cardMeta)}>{meta}</span>
                        {usage && (
                          <a
                            {...stylex.props(mStyles.cardUsage)}
                            href={usageHref(usage)}
                            title={usageTitle}
                            aria-label={`${usage.channelName} · ${stageLabel(usage)}`}
                          >
                            <span aria-hidden="true" {...stylex.props(mStyles.usageDot)} style={{ backgroundColor: stageColor(usage.stage) }} />
                            <span {...stylex.props(mStyles.usageText)} style={{ color: stageColor(usage.stage) }}>
                              {usage.channelName} · {stageLabel(usage)}
                            </span>
                          </a>
                        )}
                      </div>
                      <Button
                        variant="icon"
                        size="lg"
                        aria-label={`${tt.copyLinkTitle} · ${video.title}`}
                        xstyle={mStyles.copyBtn}
                        onClick={() => void copy(video.url)}
                      >
                        <Icon name="copy" size={18} />
                      </Button>
                    </div>
                  </article>
                )
              }}
            </MobileVideoList>
          )}
        </>
      )}

      {selectMode && (
        <div role="toolbar" aria-label={tt.selectBarLabel} {...stylex.props(mStyles.selectionBar)}>
          <div {...stylex.props(mStyles.selectionRow)}>
            <span {...stylex.props(mStyles.selectionCount)}>{tt.selected(checkedIds.size)}</span>
            <Button size="lg" onClick={() => selectTop(10)}>{tt.top10}</Button>
            <Button
              size="lg"
              icon={<Icon name="copy" size={15} />}
              disabled={selectedVideos.length === 0}
              aria-label={tt.copySelectedAria(selectedVideos.length)}
              onClick={() => void copy(selectedVideos.map((video) => video.url).join('\n'))}
            >
              {tt.copyLink}
            </Button>
          </div>
          <Button
            variant="primary"
            size="xl"
            block
            icon={<Icon name="video-add" size={18} />}
            disabled={selectedVideos.length === 0 || channels.length === 0 || bulkBusy}
            pending={bulkBusy}
            onClick={() => setBulkPickerOpen(true)}
          >
            {tt.bulkMakeFor(selectedVideos.length, firstChannel?.name ?? tt.notLinked)}
          </Button>
          <span {...stylex.props(mStyles.barHint)}>
            {firstChannel ? tt.selectBarHint(firstChannel.perDay) : tt.selectBarHintNoChannel}
          </span>
        </div>
      )}

      {/* Tấm thông tin đối thủ — cùng dữ liệu thật với cột phải trên máy tính. */}
      <Sheet open={infoOpen} onClose={() => setInfoOpen(false)} size="md" height="86%" label={tt.rivalInfo} bodyStyle={mStyles.sheetBody}>
        {infoRival && (
          <div {...stylex.props(mStyles.infoStack)}>
            <div {...stylex.props(mStyles.infoHead)}>
              <ChannelAvatar name={infoRival.title} src={infoRival.avatarUrl ?? undefined} size={48} fontSize={16} />
              <span {...stylex.props(mStyles.infoHeadText)}>
                <span {...stylex.props(mStyles.infoName)}>{infoRival.title}</span>
                <span {...stylex.props(mStyles.infoSub)}>
                  {tt.infoSub(
                    infoRival.subscriberCount != null ? fmtSubs(infoRival.subscriberCount) : '—',
                    infoPosting.data?.perWeek != null ? String(Math.round(infoPosting.data.perWeek)) : '—',
                  )}
                </span>
              </span>
              <Button variant="icon" size="lg" aria-label={tt.infoMoreAria(infoRival.title)} onClick={() => setInfoMenuOpen(true)}>
                <Icon name="more" size={18} />
              </Button>
            </div>

            <div {...stylex.props(mStyles.infoLinkCard)}>
              <Icon name="channel" size={17} />
              <span {...stylex.props(mStyles.infoLinkText)}>
                <span {...stylex.props(mStyles.infoLinkTitle)}>
                  {linkedChannel ? tt.linkedTo(linkedChannel.name) : tt.notLinked}
                </span>
                <span {...stylex.props(mStyles.infoLinkSub)}>{tt.infoLinkHint}</span>
              </span>
              {linkedChannel ? (
                <a {...stylex.props(mStyles.infoChange)} href={`#${paths.channel(linkedChannel.id, 'sources')}`}>{tt.infoChange}</a>
              ) : (
                <Button size="lg" onClick={() => setInfoLinkOpen(true)} disabled={channels.length === 0}>{tt.linkPick}</Button>
              )}
            </div>

            <div {...stylex.props(mStyles.infoStats)}>
              <div {...stylex.props(mStyles.infoStat)}>
                <b {...stylex.props(mStyles.infoStatValue)}>{infoRival.medianViews != null ? fmtSubs(infoRival.medianViews) : tt.statPending}</b>
                <span {...stylex.props(mStyles.infoStatLabel)}>{tt.statBaseline}</span>
              </div>
              <div {...stylex.props(mStyles.infoStat, mStyles.infoStatDivider)}>
                <b {...stylex.props(mStyles.infoStatValue)}>{infoPosting.data?.perWeek != null ? tt.perWeekValue(String(Math.round(infoPosting.data.perWeek))) : tt.statPending}</b>
                <span {...stylex.props(mStyles.infoStatLabel)}>{tt.statPerWeek}</span>
              </div>
              <div {...stylex.props(mStyles.infoStat, mStyles.infoStatDivider)}>
                <b {...stylex.props(mStyles.infoStatValue, mStyles.infoStatHot)}>{num.format(infoRival.outliers)}</b>
                <span {...stylex.props(mStyles.infoStatLabel)}>{tt.statOutliers}</span>
              </div>
            </div>

            {infoScan.data && <ScanCard status={infoScan.data} rivalId={infoRival.id} mobile />}
            <PostingCard schedule={infoPosting.data} zone={zone} onZone={setZone} />
          </div>
        )}
      </Sheet>

      <ActionSheet
        open={infoMenuOpen}
        onClose={() => setInfoMenuOpen(false)}
        title={tt.rivalActions(infoRival?.title ?? '')}
        actions={[
          { key: 'check', label: tt.menuCheck, description: tt.menuCheckHint(infoScan.data?.rescanHours ?? 6), icon: 'refresh', onSelect: checkInfo },
          { key: 'youtube', label: tt.openYouTube, icon: 'link', onSelect: () => { if (infoRival) window.open(infoRival.url, '_blank', 'noreferrer') } },
          { key: 'link', label: tt.menuLink, description: tt.menuLinkHint, icon: 'channel', disabled: channels.length === 0, onSelect: () => setInfoLinkOpen(true) },
          { key: 'unfollow', label: tt.menuUnfollow, description: tt.menuUnfollowHint, icon: 'trash', destructive: true, onSelect: () => setUnfollowOpen(true) },
        ]}
      />

      <ActionSheet
        open={infoLinkOpen}
        onClose={() => setInfoLinkOpen(false)}
        title={tt.linkDialogTitle(infoRival?.title ?? '')}
        description={tt.linkDialogHint}
        actions={channels.length === 0
          ? [{ key: 'empty', label: tt.linkEmpty, disabled: true, onSelect: () => {} }]
          : channels.map((channel) => {
            const linked = channel.sources.some((source) => source.rivalId === infoRival?.id)
            return {
              key: channel.id,
              label: channel.name,
              description: linked ? tt.linkedTo(channel.name) : channel.templateName,
              icon: 'channel' as const,
              disabled: linked,
              onSelect: async () => {
                if (!infoRival) return
                await linkSource(channel.id, infoRival.id)
                void invalidate(keys.channels())
                toast.ok(tt.linkDone)
              },
            }
          })}
      />

      <ConfirmSheet
        open={unfollowOpen}
        onClose={() => setUnfollowOpen(false)}
        title={tt.unfollowTitle(infoRival?.title ?? '')}
        description={`${tt.unfollowBody} ${tt.unfollowItem(num.format(infoRival?.videoCount ?? 0))} ${tt.unfollowQueue}`}
        action={{ label: tt.unfollowOk, destructive: true, onSelect: unfollow }}
        tone="danger"
      />

      {/* Tấm thêm đối thủ — thêm thật rồi liên kết các kênh đã chọn. */}
      <Sheet open={addOpen} onClose={() => setAddOpen(false)} size="md" title={tt.addSheetTitle} height="auto" bodyStyle={mStyles.sheetBody}>
        <div {...stylex.props(mStyles.addBody)}>
          <TextField
            value={addValue}
            onChange={(value) => { setAddValue(value); setAddError(null) }}
            label={tt.addSheetLabel}
            placeholder={tt.addSheetLabel}
            aria-label={tt.addSheetLabel}
            disabled={addPending}
            autoFocus
            frameStyle={mStyles.addInput}
            inputStyle={mStyles.addInput}
            error={addError?.message}
          />
          {addError?.action === 'open' && addError.rivalId && (
            <Button variant="quiet" size="lg" onClick={() => { setAddOpen(false); pickRival(addError.rivalId!) }}>{tt.addErrors.existsOpen}</Button>
          )}
          {addError?.action === 'accounts' && (
            <a href={`#${paths.settings('accounts')}`}>{tt.addErrors.blockedOpen}</a>
          )}
          {channels.length > 0 && (
            <>
              <span {...stylex.props(mStyles.addLinkNote)}>{tt.addSheetLink}</span>
              <div role="group" aria-label={tt.addSheetLinkGroup} {...stylex.props(mStyles.addGroup)}>
                {channels.map((channel) => (
                  <Checkbox
                    key={channel.id}
                    variant="chip"
                    checked={addLinks.has(channel.id)}
                    onChange={(checked) => {
                      setAddLinks((prev) => {
                        const next = new Set(prev)
                        if (checked) next.add(channel.id)
                        else next.delete(channel.id)
                        return next
                      })
                    }}
                    xstyle={mStyles.addChip}
                    label={(
                      <span {...stylex.props(mStyles.addChipLabel)}>
                        <ChannelAvatar name={channel.name} size={24} fontSize={9.5} />
                        <span {...stylex.props(mStyles.addChipName)}>{channel.name}</span>
                      </span>
                    )}
                  />
                ))}
              </div>
            </>
          )}
          <p {...stylex.props(mStyles.addHint)}>{tt.addSheetHint}</p>
          <Button
            variant="primary"
            size="xl"
            block
            pending={addPending}
            disabled={!addValue.trim()}
            onClick={() => void submitAdd()}
          >
            {tt.follow}{tt.followWithLinks(addLinks.size)}
          </Button>
        </div>
      </Sheet>

      <ActionSheet
        open={bulkPickerOpen}
        onClose={() => setBulkPickerOpen(false)}
        title={tt.pickerTitle}
        actions={channels.length === 0
          ? [{ key: 'empty', label: tt.pickerEmpty, disabled: true, onSelect: () => {} }]
          : channels.map((channel) => ({
            key: channel.id,
            label: channel.name,
            description: channel.templateName ? tt.pickerNext(channel.templateName) : tt.pickerNoTemplate,
            icon: 'channel' as const,
            onSelect: () => bulkEnqueue(channel.id),
          }))}
      />
    </div>
  )
}
