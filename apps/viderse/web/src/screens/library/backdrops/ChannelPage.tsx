import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
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
    <article role="listitem" className={`vbd-vcard ${stylex.props(sx.vcard).className}`} data-off={off ? '' : undefined}>
      <div className={`vbd-thumb ${stylex.props(sx.thumb).className}`}>
        {item.thumbnailUrl && <img className={`vbd-thumb-img ${stylex.props(sx.thumbImg, (Boolean((off ? '' : undefined))) && sx.thumbImgDataOff).className}`} src={item.thumbnailUrl} alt="" loading="lazy" />}
        <span className={`vbd-kind ${stylex.props(sx.kind).className}`}>{kindName(item.kind)}</span>
        {item.durationSec !== null && !off && <span className={`vbd-dur ${stylex.props(sx.dur).className}`}>{timeRange(item.durationSec)}</span>}
        {chosen.length > 0 && (
          <span className={`vbd-pinc ${stylex.props(sx.pinc).className}`} title={copy.pinTip(chosen.map((c) => c.name).join(', '))}><Icon name="tick" size={11} /></span>
        )}
        {off && <span className={`vbd-offlabel ${stylex.props(sx.offlabel).className}`}>{reason}</span>}
      </div>
      <span className={`vbd-vtitle ${stylex.props(sx.vtitle).className}`}>{item.title}</span>
      <span className={`vbd-vsub num ${stylex.props(sx.vsubNum).className}`}>
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
    <div className={`vbd-scroll ${stylex.props(sx.scroll).className}`} ref={grid.ref} onScroll={onScroll}>
      <section className={`vbd-pane ${stylex.props(sx.pane).className}`} aria-label={name}>
        <div className={`vbd-ch-head ${stylex.props(sx.chHead).className}`}>
          {channel.avatarUrl
            ? <img className={`vbd-av ${stylex.props(sx.av).className}`} src={channel.avatarUrl} alt="" width={44} height={44} style={{ borderRadius: '50%' }} />
            : <span className={`vbd-av ${stylex.props(sx.av).className}`} aria-hidden="true" style={{ width: 44, height: 44, background: channelTint(channel.id) }}>{initials(name)}</span>}
          <div className={`vbd-ch-main ${stylex.props(sx.chMain).className}`}>
            <h2 className={`vbd-ch-name ${stylex.props(sx.chName).className}`}>{name}</h2>
            <span className={`vbd-ch-meta num ${stylex.props(sx.chMetaNum).className}`}>
              <span className={`vbd-dot ${stylex.props(sx.dot).className}`} style={{ background: platformColor(channel.platform) }} />
              {platformName(channel.platform)} · <a className={`vbd-link ${stylex.props(sx.link).className}`} href={channel.url} target="_blank" rel="noreferrer">{channel.url.replace(/^https?:\/\//, '')}</a>
              {' · '}
              <span className={`vbd-status ${stylex.props(((paused ? 'paused' : reading ? 'reading' : undefined) === "paused") && sx.statusDataTonePaused, ((paused ? 'paused' : reading ? 'reading' : undefined) === "reading") && sx.statusDataToneReading).className}`} data-tone={paused ? 'paused' : reading ? 'reading' : undefined}>{status}</span>
            </span>
          </div>
          <ActionButton xstyle={[sx.vbdChHeadActionButton]} size="sm" icon={<Icon name="refresh" size={15} />} pending={refreshing}
            disabled={paused && !!pauseTime}
            disabledReason={paused && pauseTime ? ch.refreshPausedTip(platformName(channel.platform), pauseTime) : undefined}
            tooltip={ch.refreshTip}
            onClick={() => void refresh()}>
            {ch.refresh}
          </ActionButton>
          <ActionMenu triggerStyle={sx.touchTarget}
            label={ch.moreAria(name)}
            items={[{ id: 'remove', label: ch.removeItem, danger: true, icon: <Icon name="trash" size={15} />, onSelect: () => void remove() }]}
          />
        </div>

        <div className={`vbd-used ${stylex.props(sx.used).className}`}>
          <span className={`vbd-used-label ${stylex.props(sx.usedLabel).className}`}>{ch.usedLabel}</span>
          {channel.usedBy.map((use) => (
            <a key={use.templateId} className={`vbd-usechip ${stylex.props(sx.usechip).className}`} href={`#${paths.template(use.templateId, 'backdrop')}`}>
              <Icon name="layer" size={13} />
              <b>{use.name}</b>
              <span className={`vbd-usechip-note ${stylex.props(sx.usechipNote).className}`}>· {use.mode === 'pick' ? ch.usedNotePick(num(use.chosenCount)) : use.kinds.length < backdropKindsOf(channel.platform).length ? ch.usedNoteKinds(use.kinds.map(kindName).join(', ')) : ch.usedNoteRandom}</span>
            </a>
          ))}
          {channel.usedBy.length === 0 && (
            <span className={`vbd-used-none ${stylex.props(sx.usedNone).className}`}>{ch.noUsePre} <b {...stylex.props(sx.vbdUsedNoneB)}>{ch.noUseAction}</b>.</span>
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

        <div className={`vbd-filter ${stylex.props(sx.filter).className}`}>
          {kindOptions.length > 1 && (
            <Segmented buttonStyle={sx.touchTarget} size="sm" fit="content" label={ch.kindAria} value={kind} options={kindOptions}
              onChange={(id) => setKind(id)} />
          )}
          <span className={`vbd-flex ${stylex.props(sx.flex).className}`} />
          <span className={`vbd-showing num ${stylex.props(sx.showingNum).className}`}>{shownText}</span>
          <span className={`vbd-sortchip num ${stylex.props(sx.sortchipNum).className}`} title={ch.sortLabel}>{ch.sortNewest}<Icon name="arrow-d" size={12} /></span>
        </div>

        {videos.initialLoading || (reading && videos.items.length === 0) ? (
          <div className={`vbd-grid ${stylex.props(sx.grid).className}`} aria-hidden="true">
            {Array.from({ length: 8 }, (_, i) => (
              <div key={i} className={`vbd-skel ${stylex.props(sx.skel).className}`}>
                <Skeleton radius={10} style={{ aspectRatio: '16 / 9', height: 'auto' }} />
                <Skeleton height={12} width="80%" />
                <Skeleton height={10} width="45%" />
              </div>
            ))}
          </div>
        ) : (
          <div role="list" aria-label={ch.gridAria(name)} className={`vbd-grid ${stylex.props(sx.grid).className}`}
            style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
            {videos.items.slice(grid.itemStart, grid.itemEnd).map((item) => (
              <ChannelVideoCard key={item.videoId} item={item} />
            ))}
          </div>
        )}

        {videos.loadingMore && (
          <div className={`vbd-more ${stylex.props(sx.more).className}`}><Spinner size={14} color="var(--color-fg-3)" /></div>
        )}
        {videos.hasMore && !videos.loadingMore && (
          <div className={`vbd-more ${stylex.props(sx.more).className}`}>
            <Button xstyle={[sx.vbdChHeadActionButton]} size="md" onClick={videos.loadMore}>{ch.moreVideos}</Button>
          </div>
        )}
        {!!videos.error && <Banner tone="error" title={t.library.loadFail} action={{ label: t.library.retry, onClick: () => void invalidate(['backdrop']) }} />}

        <p className={`vbd-foot ${stylex.props(sx.foot).className}`}><Icon name="info" size={14} />{ch.footerInfo}</p>
      </section>
    </div>
  )
}

const sx = stylex.create({
  vcard: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  thumb: { position: "relative", aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", overflowX: "hidden", overflowY: "hidden", backgroundColor: line["--line-1"], backgroundImage: "none", display: "block" },
  thumbImg: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  thumbImgDataOff: { opacity: libraryVars.templateBackdropPickerPickItemDataDimOpacity },
  kind: { position: "absolute", top: "var(--vv-s-6)", left: "var(--vv-s-6)", fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-6)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "var(--vv-scrim)", backgroundImage: "none", color: color["--color-fg"] },
  dur: { position: "absolute", bottom: "var(--vv-s-6)", right: "var(--vv-s-6)", fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-6)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "var(--vv-scrim)", backgroundImage: "none", color: color["--color-fg"] },
  pinc: { position: "absolute", top: "var(--vv-s-6)", right: "var(--vv-s-6)", width: "var(--vv-s-18)", height: "var(--vv-s-18)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-ok"], backgroundImage: "none", color: color["--color-bg"], display: "flex", alignItems: "center", justifyContent: "center" },
  offlabel: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-11)", fontWeight: libraryVars.labelWeight, color: color["--color-fg-2"], backgroundColor: "var(--vv-scrim)", backgroundImage: "none" },
  vtitle: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.labelWeight, lineHeight: libraryVars.titleLineHeight, display: "-webkit-box", WebkitLineClamp: libraryVars.promptsTabPcardTextWebkitLineClamp, WebkitBoxOrient: "vertical", overflowX: "hidden", overflowY: "hidden" },
  vsubNum: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  scroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minHeight: { default: 0, "@media (max-width: 767px)": libraryVars.backdropsChannelPageScrollMinHeight }, overflowY: "auto", minWidth: 0, maxHeight: { default: null, "@media (max-width: 767px)": libraryVars.backdropsChannelPageScrollMaxHeight } },
  pane: { borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", paddingTop: "var(--vv-s-16)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-18)", paddingLeft: "var(--vv-s-18)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-14)" },
  chHead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)" },
  av: { borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, overflowX: "hidden", overflowY: "hidden", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], objectFit: "cover" },
  chMain: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  chName: { fontSize: "var(--vv-s-18)", fontWeight: libraryVars.headingWeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, lineHeight: libraryVars.compactLineHeight },
  chMetaNum: { fontSize: "var(--vv-s-11)", color: color["--color-fg-3"], display: "flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", flexWrap: "wrap" },
  dot: { width: "var(--vv-s-7)", height: "var(--vv-s-7)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, display: "inline-block" },
  link: { color: color["--color-info"], textDecoration: { default: "none", ":hover": "underline" }, display: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "inline-flex" }, alignItems: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "center" }, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  statusDataTonePaused: { color: color["--color-warn"] },
  statusDataToneReading: { color: color["--color-info"] },
  vbdChHeadActionButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  used: { display: "flex", alignItems: "center", flexWrap: "wrap", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  usedLabel: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  usechip: { display: { default: "inline-flex", "@media (max-width: 767px) and (pointer: coarse)": "inline-flex" }, alignItems: { default: "center", "@media (max-width: 767px) and (pointer: coarse)": "center" }, rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": color["--color-accent"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": color["--color-accent"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": color["--color-accent"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": color["--color-accent"] }, borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", paddingTop: "var(--vv-s-5)", paddingRight: "var(--vv-s-9)", paddingBottom: "var(--vv-s-5)", paddingLeft: "var(--vv-s-9)", fontSize: "var(--vv-s-11)", textDecoration: "none", color: color["--color-fg"], minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  usechipNote: { color: color["--color-fg-3"] },
  usedNone: { fontSize: "var(--vv-s-11)", color: color["--color-fg-3"] },
  vbdUsedNoneB: { color: color["--color-fg-2"] },
  filter: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  flex: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis },
  showingNum: { fontSize: "var(--vv-s-11)", color: color["--color-fg-3"] },
  sortchipNum: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", fontSize: "var(--vv-s-11)", color: color["--color-fg-2"], borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-8)", minHeight: { default: null, "@media (pointer: coarse)": "var(--vv-s-28)" } },
  grid: { display: "grid", gridTemplateColumns: { default: libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns2, "@media (max-width: 767px)": libraryVars.promptEditorCompsGridTemplateColumns }, rowGap: "var(--vv-s-16)", columnGap: "var(--vv-s-12)" },
  skel: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  more: { display: "flex", justifyContent: "center", paddingTop: "var(--vv-s-4)" },
  foot: { display: "flex", alignItems: "flex-start", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: libraryVars.microFontSize, color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-1"], paddingTop: "var(--vv-s-12)" },
})
