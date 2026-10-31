// Trang một kênh nền trong tab Video nền: đầu kênh (avatar, nền tảng, trạng
// thái đọc), banner nền tảng tạm chặn / đang đọc, lọc loại video, lưới ảo hoá
// keyset (24/trang, cuộn gần cuối tự nạp + nút Hiện thêm), nhãn chosenBy và
// mờ video không còn xem được. Xoá kênh qua menu ⋯ (409 in_use hiện câu lỗi).
import { useMemo, useState, type UIEvent } from 'react'
import { ActionMenu, Banner, Button, Icon, Segmented, Skeleton, Spinner, confirm, type Segment } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/vi'
import { paths, navigate } from '../../../router'
import { keys } from '../../../data/keys'
import { channelTint } from '../../../lib/channelTint'
import { useVirtualGrid } from '../../../lib/useVirtualGrid'
import { backdropKindsOf, refreshBackdropChannel, removeBackdropChannel, type BackdropChannel, type ChannelVideoItem } from '../../../api/backdrop'
import type { Platform } from '../../../api/platforms'
import { ActionButton } from '../../../components'
import { ageLabel, clock, initials, kindName, num, platformColor, platformName, timeRange, updatedLabel } from './fmt'
import { channelPaused, channelReading, channelUsable } from './SourceRail'
import { useChannelVideos } from './useChannelVideos'

/** id kind 'all' của Segmented; các kind còn lại giữ đúng mã backend. */
const ALL_KIND = ''

function ChannelVideoCard({ item }: { item: ChannelVideoItem }) {
  const copy = t.backdrop.tab.channel
  const off = item.unavailable
  const chosen = item.chosenBy
  const reason = off ? (copy.unavailableReason[off.reason] ?? off.reason) : ''
  return (
    <article role="listitem" className="vbd-vcard" data-off={off ? '' : undefined}>
      <div className="vbd-thumb">
        {item.thumbnailUrl && <img className="vbd-thumb-img" src={item.thumbnailUrl} alt="" loading="lazy" />}
        <span className="vbd-kind">{kindName(item.kind)}</span>
        {item.durationSec !== null && !off && <span className="vbd-dur">{timeRange(item.durationSec)}</span>}
        {chosen.length > 0 && (
          <span className="vbd-pinc" title={copy.pinTip(chosen.map((c) => c.name).join(', '))}><Icon name="tick" size={11} /></span>
        )}
        {off && <span className="vbd-offlabel">{reason}</span>}
      </div>
      <span className="vbd-vtitle">{item.title}</span>
      <span className="vbd-vsub num">
        {off ? `${kindName(item.kind)} · ${copy.unavailable}` : [kindName(item.kind), ageLabel(item.publishedAt)].filter(Boolean).join(' · ')}
      </span>
    </article>
  )
}

export function ChannelPage({ channel, platform, viewport }: {
  channel: BackdropChannel
  /** trạng thái nền tảng (banner tạm chặn + giờ được làm mới) */
  platform: Platform | undefined
  /** kích thước ép cho test (happy-dom không layout) */
  viewport?: { width: number; height: number }
}) {
  const copy = t.backdrop.tab
  const ch = copy.channel
  const [kind, setKind] = useState<string>(ALL_KIND)
  const [refreshing, setRefreshing] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const videos = useChannelVideos(channel.id, kind === ALL_KIND ? [] : [kind])

  const name = channel.title ?? channel.handle
  // Chặn tạm (kênh hoặc cả nền tảng) áp đảo trạng thái đang đọc: video đã đọc
  // được vẫn hiện đủ dưới banner, "Đang đọc…" chỉ còn cho kênh chưa từng list.
  const paused = channelPaused(channel) || platform?.state === 'paused'
  const reading = channelReading(channel) && !paused
  const pausedUntil = platform?.pausedUntil ?? channel.pausedUntil ?? channel.bilibili?.pausedUntil ?? null
  const pauseTime = pausedUntil ? clock(pausedUntil) : ''
  const doneCount = num(channelUsable(channel))
  const totalCount = channel.scanTotal > 0 ? num(channel.scanTotal) : ''
  const grid = useVirtualGrid({ count: videos.items.length, viewport, metrics: { cardMinW: 200, cardExtraH: 64, gapX: 12, gapY: 16 } })

  const kindOptions = useMemo(() => {
    const kinds = backdropKindsOf(channel.platform)
    const total = Object.values(videos.counts).reduce<number>((n, c) => n + (c ?? 0), 0)
    const options: Segment[] = kinds.length > 1
      ? [{ id: ALL_KIND, label: `${ch.kindAll} ${num(total)}` }]
      : []
    for (const k of kinds) {
      const n = videos.counts[k] ?? 0
      options.push({ id: k, label: `${kindName(k)} ${num(n)}`, disabled: n === 0 && kind !== k })
    }
    return options
  }, [channel.platform, videos.counts, kind, ch.kindAll])

  const refresh = async () => {
    setRefreshing(true)
    setActionError(null)
    try {
      await refreshBackdropChannel(channel.id)
      await invalidate(['backdrop'])
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setRefreshing(false)
    }
  }

  const remove = async () => {
    const accepted = await confirm({
      title: ch.removeTitle(name),
      body: ch.removeBody,
      tone: 'danger',
      confirmLabel: ch.removeAction,
      cancelLabel: t.library.cancel,
      onConfirm: () => undefined,
    })
    if (!accepted) return
    try {
      await removeBackdropChannel(channel.id)
      await invalidate(['backdrop'])
      navigate(paths.library('backdrops'))
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    }
  }

  const onScroll = (event: UIEvent<HTMLDivElement>) => {
    grid.onScroll(event)
    const el = event.currentTarget
    if (videos.hasMore && !videos.loadingMore && el.scrollTop + el.clientHeight > el.scrollHeight - 600) videos.loadMore()
  }

  const shownText = reading
    ? ch.readingShort
    : kind === ALL_KIND
      ? ch.showing(num(videos.items.length), num(videos.total))
      : ch.shownPage(num(videos.items.length))

  const status = paused && pauseTime
    ? ch.readNextAt(pauseTime)
    : channel.listedAt ? updatedLabel(channel.listedAt) : ch.reading

  return (
    <div className="vbd-scroll" ref={grid.ref} onScroll={onScroll}>
      <section className="vbd-pane" aria-label={name}>
        <div className="vbd-ch-head">
          {channel.avatarUrl
            ? <img className="vbd-av" src={channel.avatarUrl} alt="" width={44} height={44} style={{ borderRadius: '50%' }} />
            : <span className="vbd-av" aria-hidden="true" style={{ width: 44, height: 44, background: channelTint(channel.id) }}>{initials(name)}</span>}
          <div className="vbd-ch-main">
            <h2 className="vbd-ch-name">{name}</h2>
            <span className="vbd-ch-meta num">
              <span className="vbd-dot" style={{ background: platformColor(channel.platform) }} />
              {platformName(channel.platform)} · <a className="vbd-link" href={channel.url} target="_blank" rel="noreferrer">{channel.url.replace(/^https?:\/\//, '')}</a>
              {' · '}
              <span className="vbd-status" data-tone={paused ? 'paused' : reading ? 'reading' : undefined}>{status}</span>
            </span>
          </div>
          <ActionButton size="sm" icon={<Icon name="refresh" size={15} />} pending={refreshing}
            disabled={paused && !!pauseTime}
            disabledReason={paused && pauseTime ? ch.refreshPausedTip(platformName(channel.platform), pauseTime) : undefined}
            tooltip={ch.refreshTip}
            onClick={() => void refresh()}>
            {ch.refresh}
          </ActionButton>
          <ActionMenu
            label={ch.moreAria(name)}
            items={[{ id: 'remove', label: ch.removeItem, danger: true, icon: <Icon name="trash" size={15} />, onSelect: () => void remove() }]}
          />
        </div>

        <div className="vbd-used">
          <span className="vbd-used-label">{ch.usedLabel}</span>
          {channel.usedBy.map((use) => (
            <a key={use.templateId} className="vbd-usechip" href={`#${paths.template(use.templateId, 'backdrop')}`}>
              <Icon name="layer" size={13} />
              <b>{use.name}</b>
              <span className="vbd-usechip-note">· {use.mode === 'pick' ? ch.usedNotePick(num(use.chosenCount)) : use.kinds.length < backdropKindsOf(channel.platform).length ? ch.usedNoteKinds(use.kinds.map(kindName).join(', ')) : ch.usedNoteRandom}</span>
            </a>
          ))}
          {channel.usedBy.length === 0 && (
            <span className="vbd-used-none">{ch.noUsePre} <b>{ch.noUseAction}</b>.</span>
          )}
        </div>

        {paused && (
          <Banner tone="warning" icon="clock"
            title={ch.pausedTitle(platformName(channel.platform), pauseTime || '…', doneCount, totalCount)}
            action={{ label: ch.pausedAction, onClick: () => navigate(paths.settings('accounts')) }}>
            {ch.pausedText(pauseTime || '…', doneCount)}
          </Banner>
        )}
        {!paused && reading && (
          <Banner tone="info" title={ch.readingTitle(doneCount)}>{ch.readingText}</Banner>
        )}
        {channel.listError && !reading && (
          <Banner tone="error" title={ch.listFailed}>{channel.listError.message}</Banner>
        )}
        {actionError && <Banner tone="error" title={actionError} />}

        <div className="vbd-filter">
          {kindOptions.length > 1 && (
            <Segmented size="sm" fit="content" label={ch.kindAria} value={kind} options={kindOptions}
              onChange={(id) => setKind(id)} />
          )}
          <span className="vbd-flex" />
          <span className="vbd-showing num">{shownText}</span>
          <span className="vbd-sortchip num" title={ch.sortLabel}>{ch.sortNewest}<Icon name="arrow-d" size={12} /></span>
        </div>

        {videos.initialLoading || (reading && videos.items.length === 0) ? (
          <div className="vbd-grid" aria-hidden="true">
            {Array.from({ length: 8 }, (_, i) => (
              <div key={i} className="vbd-skel">
                <Skeleton radius={10} style={{ aspectRatio: '16 / 9', height: 'auto' }} />
                <Skeleton height={12} width="80%" />
                <Skeleton height={10} width="45%" />
              </div>
            ))}
          </div>
        ) : (
          <div role="list" aria-label={ch.gridAria(name)} className="vbd-grid"
            style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
            {videos.items.slice(grid.itemStart, grid.itemEnd).map((item) => (
              <ChannelVideoCard key={item.videoId} item={item} />
            ))}
          </div>
        )}

        {videos.loadingMore && (
          <div className="vbd-more"><Spinner size={14} color="var(--color-fg-3)" /></div>
        )}
        {videos.hasMore && !videos.loadingMore && (
          <div className="vbd-more">
            <Button size="md" onClick={videos.loadMore}>{ch.moreVideos}</Button>
          </div>
        )}
        {!!videos.error && <Banner tone="error" title={t.library.loadFail} action={{ label: t.library.retry, onClick: () => void invalidate(['backdrop']) }} />}

        <p className="vbd-foot"><Icon name="info" size={14} />{ch.footerInfo}</p>
      </section>
    </div>
  )
}
