// Hộp "Gắn kênh hoặc video" của mục Nền & hiệu ứng (Template.dc.html › picker):
// trái là SourceRail (kênh + từng video lẻ) kèm ô dán link mới, phải là chi
// tiết nguồn — kênh: chọn cách lấy (ngẫu nhiên / chỉ video đã chọn), chip loại
// video, lưới ảo hoá để ghim; video lẻ: đoạn đã chọn + nút qua Thư viện.
// Lưu = gọi onSave({backdrop}) với channels/fixedVideoIds mới.
import { useMemo, useState, type FormEvent, type UIEvent } from 'react'
import { Button, Icon, RadioCard, Spinner, TextField } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/vi'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import { useVirtualGrid } from '../../../lib/useVirtualGrid'
import {
  addBackdropChannel, addBackdropVideo, backdropKindsOf, backdropVideoFileHref,
  listBackdropChannels, listBackdropVideos, parseSource,
  type BackdropChannel, type BackdropVideo, type ChannelVideoItem,
} from '../../../api/backdrop'
import type { Recipe, RecipeBackdrop, TemplateDetail } from '../../../api/library'
import { channelTint } from '../../../lib/channelTint'
import { paths } from '../../../router'
import { SourceRail, channelUsable } from '../backdrops/SourceRail'
import { useChannelVideos } from '../backdrops/useChannelVideos'
import { duration, initials, kindName, num, platformName, segmentLabel, timeRange } from '../backdrops/fmt'

function ChannelDetail({ channel, link, onGo }: {
  channel: BackdropChannel
  /** link đang gắn trong recipe (null = chưa gắn) */
  link: RecipeBackdrop['channels'][number] | null
  onGo: (next: { kinds: string[]; pinned: string[] }) => void
}) {
  const copy = t.backdrop.template.picker

  const platformKinds = backdropKindsOf(channel.platform)
  const [mode, setMode] = useState<'random' | 'pick'>(link && link.pinned.length > 0 ? 'pick' : 'random')
  const [kinds, setKinds] = useState<string[]>(link?.kinds ?? platformKinds)
  const [pinned, setPinned] = useState<ReadonlySet<string>>(() => new Set(link?.pinned ?? []))
  // Lưới hiện mọi loại (mục loại đang tắt bị mờ, không chọn được); nạp không
  // lọc kind để video của kind đang tắt vẫn xem/ghim lại được.
  const kindVideo = useChannelVideos(channel.id, [])
  const grid = useVirtualGrid({ count: kindVideo.items.length, metrics: { cardMinW: 150, cardExtraH: 58, gapX: 10, gapY: 14 } })

  const listed = channel.listedAt !== null
  const pool = kinds.reduce((n, k) => n + (channel.counts[k] ?? 0), 0)
  const nPicked = pinned.size
  const isPick = mode === 'pick'

  const toggleKind = (kind: string) => {
    if (kinds.includes(kind)) {
      if (kinds.length === 1) return // kind cuối khoá bật
      setKinds(kinds.filter((k) => k !== kind))
    } else {
      setKinds([...kinds, kind])
    }
  }

  const toggleVideo = (item: ChannelVideoItem) => {
    if (!isPick || !kinds.includes(item.kind)) return
    const next = new Set(pinned)
    if (next.has(item.videoId)) next.delete(item.videoId)
    else next.add(item.videoId)
    setPinned(next)
  }

  const onScroll = (event: UIEvent<HTMLDivElement>) => {
    grid.onScroll(event)
    const el = event.currentTarget
    if (kindVideo.hasMore && !kindVideo.loadingMore && el.scrollTop + el.clientHeight > el.scrollHeight - 400) kindVideo.loadMore()
  }

  const goDisabled = isPick && nPicked === 0
  const sum = copy.sumChannel(
    channel.title ?? channel.handle,
    isPick ? copy.sumHowPick(num(nPicked)) : copy.sumHowRandom(num(pool)),
    kinds.map(kindName).join(', '),
  )
  const sumSub = goDisabled ? copy.sumSubNeedPick : link ? copy.sumSubSave : copy.sumSubAttach

  return (
    <div className="vbd-pick-detail">
      <div className="vbd-pick-head">
        {channel.avatarUrl
          ? <img className="vbd-av" src={channel.avatarUrl} alt="" width={40} height={40} style={{ borderRadius: '50%' }} />
          : <span className="vbd-av" aria-hidden="true" style={{ width: 40, height: 40, background: channelTint(channel.id) }}>{initials(channel.title ?? channel.handle)}</span>}
        <div className="vbd-pick-titles">
          <span className="vbd-pick-name">{channel.title ?? channel.handle}</span>
          <span className="vbd-pick-meta num">
            {copy.channelMeta(platformName(channel.platform), num(channelUsable(channel)),
              channel.pausedUntil ? t.backdrop.tab.channelPaused : '', link ? copy.linkedMeta : '')}
          </span>
        </div>
        <a className="vbd-linkbtn" href={`#${paths.library('backdrops', undefined, { nguon: 'channels', kenh: channel.id })}`}>{copy.openLibrary}</a>
      </div>

      <div className="vbd-modes" role="radiogroup" aria-label={copy.modeAria}>
        <RadioCard checked={!isPick} onChange={() => setMode('random')} name="vbd-mode"
          label={copy.modeRandom} description={copy.modeRandomSub(num(pool))} />
        <RadioCard checked={isPick} onChange={() => setMode('pick')} name="vbd-mode"
          label={copy.modePick} description={copy.modePickSub} />
      </div>

      <div className="vbd-kinds">
        {platformKinds.length > 1 && <span className="vbd-kindlabel">{copy.kindLabel}</span>}
        {platformKinds.length > 1 && (
          <div role="group" aria-label={copy.kindAria} className="vbd-kindchips">
            {platformKinds.map((kind) => {
              const on = kinds.includes(kind)
              const n = channel.counts[kind] ?? 0
              const last = on && kinds.length === 1
              const dis = (listed && n === 0) || last
              return (
                <button key={kind} type="button" className="vbd-kindchip" data-on={on || undefined}
                  aria-pressed={on} aria-disabled={dis || undefined} disabled={listed && n === 0 && !on}
                  onClick={() => toggleKind(kind)}>
                  {on && <Icon name="tick" size={12} />}
                  {kindName(kind)} <span className="num">{num(n)}</span>
                </button>
              )
            })}
          </div>
        )}
        <span className="vbd-flex" />
        <span className="vbd-picknote num" data-warn={isPick && nPicked === 0 || undefined}>
          {isPick
            ? (nPicked ? copy.pickedCount(num(nPicked)) : copy.pickedNone)
            : copy.previewCount(num(Math.min(kindVideo.items.length, pool || kindVideo.items.length)), num(pool))}
        </span>
      </div>

      <div className="vbd-pick-gridscroll" ref={grid.ref} onScroll={onScroll}>
        {kindVideo.initialLoading ? (
          <div className="vbd-pane--load"><Spinner size={14} color="var(--color-fg-3)" /></div>
        ) : (
          <div role={isPick ? 'group' : 'list'} aria-label={isPick ? copy.gridAriaPick(channel.title ?? channel.handle) : copy.gridAria(channel.title ?? channel.handle)}
            className="vbd-pick-grid" style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
            {kindVideo.items.slice(grid.itemStart, grid.itemEnd).map((item) => {
              const kindOn = kinds.includes(item.kind)
              const checked = isPick && pinned.has(item.videoId)
              return (
                <button key={item.videoId} type="button"
                  role={isPick ? 'checkbox' : 'listitem'}
                  aria-checked={isPick ? checked : undefined}
                  aria-disabled={isPick && !kindOn || undefined}
                  aria-label={isPick ? copy.itemCheckedAria(item.title) : undefined}
                  data-dim={!kindOn || undefined}
                  className="vbd-pick-item"
                  onClick={() => toggleVideo(item)}>
                  <span className="vbd-thumb" data-ring={checked || undefined}>
                    {item.thumbnailUrl && <img className="vbd-thumb-img" src={item.thumbnailUrl} alt="" loading="lazy" />}
                    <span className="vbd-kind">{kindName(item.kind)}</span>
                    {checked && <span className="vbd-picktick"><Icon name="tick" size={12} /></span>}
                  </span>
                  <span className="vbd-pick-title">{item.title}</span>
                  <span className="vbd-vsub num">{timeRange(item.durationSec)}</span>
                </button>
              )
            })}
          </div>
        )}
        {kindVideo.loadingMore && <Spinner size={14} color="var(--color-fg-3)" />}
      </div>

      <p className="vbd-hint"><Icon name="info" size={13} />{isPick ? copy.helpPick : copy.helpRandom}</p>

      <div className="vbd-pick-foot">
        <span className="vbd-pick-sum">
          <span className="vbd-pick-summain">{sum}</span>
          <span className="vbd-pick-sumsub" data-warn={goDisabled || undefined}>{sumSub}</span>
        </span>
        <Button size="md" onClick={() => undefined}>{copy.cancel}</Button>
        <Button size="md" variant="primary" aria-disabled={goDisabled || undefined} disabled={goDisabled}
          onClick={() => onGo({ kinds, pinned: isPick ? [...pinned] : [] })}>
          {link ? copy.goSave : copy.go}
        </Button>
      </div>
    </div>
  )
}

function VideoDetail({ video, linked, onGo }: {
  video: BackdropVideo
  linked: boolean
  onGo: () => void
}) {
  const copy = t.backdrop.template.picker
  const segLen = video.segment.toSec - video.segment.fromSec
  return (
    <div className="vbd-pick-detail">
      <div className="vbd-pick-head">
        <span className="vbd-av vbd-av--rect" aria-hidden="true"><Icon name="video-play" size={18} /></span>
        <div className="vbd-pick-titles">
          <span className="vbd-pick-name">{video.title}</span>
          <span className="vbd-pick-meta num">
            {copy.videoMeta(platformName(video.platform), duration(video.durationSec), linked ? copy.linkedMeta : '')}
          </span>
        </div>
      </div>
      {video.state === 'ready' && video.file && (
        <video className="vbd-pick-vthumb" src={backdropVideoFileHref(video.id)} muted playsInline preload="metadata" />
      )}
      <div className="vbd-segbox">
        <span className="vbd-segbox-text num">
          <span className="vbd-segbox-label">{copy.segLen(timeRange(segLen))}</span>
          <span className="vbd-segbox-range">{segmentLabel(video.segment.fromSec, video.segment.toSec)}</span>
        </span>
        <a className="vbd-linkbtn" href={`#${paths.library('backdrops', undefined, { nguon: 'videos' })}`}>
          <Icon name="scissor" size={13} />{copy.changeInLibrary}
        </a>
      </div>
      <p className="vbd-hint"><Icon name="info" size={13} />{copy.videoNote}</p>
      <div className="vbd-pick-foot">
        <span className="vbd-pick-sum">
          <span className="vbd-pick-summain">{copy.sumVideo(video.title, segmentLabel(video.segment.fromSec, video.segment.toSec))}</span>
          <span className="vbd-pick-sumsub">{linked ? copy.sumSubVideoLinked : copy.sumSubVideoAttach}</span>
        </span>
        <Button size="md" variant="primary" disabled={linked} onClick={onGo}>
          {linked ? copy.goLinked : copy.go}
        </Button>
      </div>
    </div>
  )
}

export function BackdropPicker({ template, onSave, onClose, initial }: {
  template: TemplateDetail
  onSave: (patch: Partial<Recipe>) => Promise<void>
  onClose: () => void
  /** nguồn mở sẵn (nút "Chọn video" trên hàng kênh đã gắn) */
  initial?: string
}) {
  const copy = t.backdrop.template.picker
  const channels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const videos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))
  const [link, setLink] = useState('')
  const [addBusy, setAddBusy] = useState(false)
  const [addError, setAddError] = useState<string | null>(null)
  const [addNote, setAddNote] = useState<string | null>(null)
  const [selected, setSelected] = useState(initial ?? '')

  const backdrop = template.recipe.backdrop ?? { channels: [], clipCount: null, clipSec: 60, fixedVideoIds: [] }
  const linkedIds = useMemo(
    () => new Set([...backdrop.channels.map((c) => c.channelId), ...backdrop.fixedVideoIds]),
    [backdrop],
  )
  const channelList = channels.data ?? []
  const videoList = videos.data ?? []
  const sel = selected || channelList[0]?.id || videoList[0]?.id || ''
  const selChannel = channelList.find((c) => c.id === sel)
  const selVideo = videoList.find((v) => v.id === sel)

  const saveBackdrop = (patch: Partial<RecipeBackdrop>) => {
    void onSave({ backdrop: { ...backdrop, ...patch } })
  }

  const saveChannel = (channelId: string, next: { kinds: string[]; pinned: string[] }) => {
    const links = backdrop.channels
    const index = links.findIndex((c) => c.channelId === channelId)
    const entry = { channelId, kinds: next.kinds, pinned: next.pinned }
    saveBackdrop({
      channels: index >= 0 ? links.map((c, i) => (i === index ? entry : c)) : [...links, entry],
    })
    onClose()
  }

  const attachVideo = (id: string) => {
    saveBackdrop({ fixedVideoIds: [...backdrop.fixedVideoIds, id] })
    onClose()
  }

  const addByLink = async (event: FormEvent) => {
    event.preventDefault()
    const input = link.trim()
    if (!input || addBusy) return
    setAddBusy(true)
    setAddError(null)
    setAddNote(null)
    try {
      const parsed = await parseSource(input)
      if (parsed.kind === 'channel') {
        const created = await addBackdropChannel(parsed.url ?? input)
        setAddNote(copy.addedChannel)
        void invalidate(keys.backdropChannels())
        setLink('')
        setSelected(created.id)
      } else if (parsed.kind === 'video') {
        const created = await addBackdropVideo(parsed.url ?? input)
        setAddNote(copy.addedVideo)
        void invalidate(keys.backdropVideos())
        setLink('')
        setSelected(created.id)
      } else if (parsed.kind === 'unsupported') {
        setAddError(t.backdrop.tab.unsupported(parsed.label))
      } else {
        setAddError(parsed.recovery ? `${parsed.message ?? t.backdrop.tab.invalidLink} ${parsed.recovery}` : (parsed.message ?? t.backdrop.tab.invalidLink))
      }
    } catch (err) {
      setAddError(err instanceof Error ? err.message : String(err))
    } finally {
      setAddBusy(false)
    }
  }

  return (
    <div className="vbd-dlg" role="dialog" aria-modal="true" aria-label={copy.title(template.name)}>
      <div className="vbd-dlg-head">
        <div className="vbd-dlg-titles">
          <h2 className="vbd-dlg-title">{copy.title(template.name)}</h2>
          <p className="vbd-hint">{copy.sub}</p>
        </div>
        <Button variant="icon" icon={<Icon name="close" size={16} />} onClick={onClose} aria-label={t.library.close} />
      </div>
      <div className="vbd-dlg-body">
        <div className="vbd-dlg-rail">
          <form className="vbd-dlg-add" onSubmit={(event) => void addByLink(event)}>
            <label htmlFor="vbd-pick-link" className="sr-only">{copy.linkLabel}</label>
            <TextField id="vbd-pick-link" value={link} placeholder={copy.linkPlaceholder}
              onChange={(value) => setLink(value)} disabled={addBusy} />
            <Button size="sm" type="submit" pending={addBusy} disabled={!link.trim()}>{copy.add}</Button>
          </form>
          {addError && <p className="vbd-fielderr" role="alert">{addError}</p>}
          {addNote && <p className="vbd-addnote"><Icon name="tick-b" size={13} />{addNote}</p>}
          <SourceRail
            channels={channelList}
            videos={videoList}
            selected={sel}
            linkedIds={linkedIds}
            videoRows="items"
            onSelect={setSelected}
            ariaLabel={copy.railAria}
          />
        </div>
        <div className="vbd-dlg-detail">
          {selChannel && (
            <ChannelDetail key={selChannel.id} channel={selChannel}
              link={backdrop.channels.find((c) => c.channelId === selChannel.id) ?? null}
              onGo={(next) => saveChannel(selChannel.id, next)} />
          )}
          {selVideo && !selChannel && (
            <VideoDetail video={selVideo} linked={backdrop.fixedVideoIds.includes(selVideo.id)}
              onGo={() => attachVideo(selVideo.id)} />
          )}
          {!selChannel && !selVideo && (
            <div className="vbd-pane--load"><Spinner size={16} color="var(--color-fg-3)" /></div>
          )}
        </div>
      </div>
    </div>
  )
}
