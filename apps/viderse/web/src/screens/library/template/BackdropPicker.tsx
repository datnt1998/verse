import * as stylex from '@stylexjs/stylex'
import { text } from '../../../styles/shared.stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Hộp "Gắn kênh hoặc video" của mục Nền & hiệu ứng (Template.dc.html › picker):
// trái là SourceRail (kênh + từng video lẻ) kèm ô dán link mới, phải là chi
// tiết nguồn — kênh: chọn cách lấy (ngẫu nhiên / chỉ video đã chọn), chip loại
// video, lưới ảo hoá để ghim; video lẻ: đoạn đã chọn + nút qua Thư viện.
// Lưu = gọi onSave({backdrop}) với channels/fixedVideoIds mới.
import { useMemo, useState, type FormEvent, type UIEvent } from 'react'
import { Button, Dialog, Icon, RadioCard, Sheet, Spinner, TextField } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../../i18n/library'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import { useVirtualGrid } from '../../../lib/useVirtualGrid'
import { useMediaQuery } from '../../../lib/useMediaQuery'
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

function ChannelDetail({ channel, link, onGo, onClose }: {
  channel: BackdropChannel
  /** link đang gắn trong recipe (null = chưa gắn) */
  link: RecipeBackdrop['channels'][number] | null
  onGo: (next: { kinds: string[]; pinned: string[] }) => void
  onClose: () => void
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
    <div className={`vbd-pick-detail ${stylex.props(sx.pickDetail).className}`}>
      <div className={`vbd-pick-head ${stylex.props(sx.pickHead).className}`}>
        {channel.avatarUrl
          ? <img className={`vbd-av ${stylex.props(sx.av).className}`} src={channel.avatarUrl} alt="" width={40} height={40} style={{ borderRadius: '50%' }} />
          : <span className={`vbd-av ${stylex.props(sx.av).className}`} aria-hidden="true" style={{ width: 40, height: 40, background: channelTint(channel.id) }}>{initials(channel.title ?? channel.handle)}</span>}
        <div className={`vbd-pick-titles ${stylex.props(sx.pickTitles).className}`}>
          <span className={`vbd-pick-name ${stylex.props(sx.pickName).className}`}>{channel.title ?? channel.handle}</span>
          <span className={`vbd-pick-meta num ${stylex.props(sx.pickMetaNum).className}`}>
            {copy.channelMeta(platformName(channel.platform), num(channelUsable(channel)),
              channel.pausedUntil ? t.backdrop.tab.channelPaused : '', link ? copy.linkedMeta : '')}
          </span>
        </div>
        <a className={`vbd-linkbtn ${stylex.props(sx.linkbtn).className}`} href={`#${paths.library('backdrops', undefined, { nguon: 'channels', kenh: channel.id })}`}>{copy.openLibrary}</a>
      </div>

      <div className={`vbd-modes ${stylex.props(sx.modes).className}`} role="radiogroup" aria-label={copy.modeAria}>
        <RadioCard xstyle={[sx.vbdModesRadioCard]} checked={!isPick} onChange={() => setMode('random')} name="vbd-mode"
          label={copy.modeRandom} description={copy.modeRandomSub(num(pool))} />
        <RadioCard xstyle={[sx.vbdModesRadioCard]} checked={isPick} onChange={() => setMode('pick')} name="vbd-mode"
          label={copy.modePick} description={copy.modePickSub} />
      </div>

      <div className={`vbd-kinds ${stylex.props(sx.kinds).className}`}>
        {platformKinds.length > 1 && <span className={`vbd-kindlabel ${stylex.props(sx.kindlabel).className}`}>{copy.kindLabel}</span>}
        {platformKinds.length > 1 && (
          <div role="group" aria-label={copy.kindAria} className={`vbd-kindchips ${stylex.props(sx.kindchips).className}`}>
            {platformKinds.map((kind) => {
              const on = kinds.includes(kind)
              const n = channel.counts[kind] ?? 0
              const last = on && kinds.length === 1
              const dis = (listed && n === 0) || last
              return (
                <button key={kind} type="button" className={`vbd-kindchip ${stylex.props(sx.kindchip, (Boolean((on))) && sx.kindchipDataOn, (Boolean((dis))) && sx.kindchipAriaDisabled).className}`} data-on={on || undefined}
                  aria-pressed={on} aria-disabled={dis || undefined} disabled={listed && n === 0 && !on}
                  onClick={() => toggleKind(kind)}>
                  {on && <Icon name="tick" size={12} />}
                  {kindName(kind)} <span className="num">{num(n)}</span>
                </button>
              )
            })}
          </div>
        )}
        <span className={`vbd-flex ${stylex.props(sx.flex).className}`} />
        <span className={`vbd-picknote num ${stylex.props(sx.picknoteNum, (Boolean((isPick && nPicked === 0))) && sx.picknoteNumDataWarn).className}`} data-warn={isPick && nPicked === 0 || undefined}>
          {isPick
            ? (nPicked ? copy.pickedCount(num(nPicked)) : copy.pickedNone)
            : copy.previewCount(num(Math.min(kindVideo.items.length, pool || kindVideo.items.length)), num(pool))}
        </span>
      </div>

      <div className={`vbd-pick-gridscroll ${stylex.props(sx.pickGridscroll).className}`} ref={grid.ref} onScroll={onScroll}>
        {kindVideo.initialLoading ? (
          <div className={`vbd-pane--load ${stylex.props(sx.paneLoad).className}`}><Spinner size={14} color="var(--color-fg-3)" /></div>
        ) : (
          <div role={isPick ? 'group' : 'list'} aria-label={isPick ? copy.gridAriaPick(channel.title ?? channel.handle) : copy.gridAria(channel.title ?? channel.handle)}
            className={`vbd-pick-grid ${stylex.props(sx.pickGrid).className}`} style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
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
                  className={`vbd-pick-item ${stylex.props(sx.pickItem, (Boolean((!kindOn))) && sx.pickItemDataDim).className}`}
                  onClick={() => toggleVideo(item)}>
                  <span className={`vbd-thumb ${stylex.props(sx.thumb, (Boolean((checked))) && sx.thumbDataRing).className}`} data-ring={checked || undefined}>
                    {item.thumbnailUrl && <img className={`vbd-thumb-img ${stylex.props(sx.thumbImg).className}`} src={item.thumbnailUrl} alt="" loading="lazy" />}
                    <span className={`vbd-kind ${stylex.props(sx.kind).className}`}>{kindName(item.kind)}</span>
                    {checked && <span className={`vbd-picktick ${stylex.props(sx.picktick).className}`}><Icon name="tick" size={12} /></span>}
                  </span>
                  <span className={`vbd-pick-title ${stylex.props(sx.pickTitle).className}`}>{item.title}</span>
                  <span className={`vbd-vsub num ${stylex.props(sx.vsubNum).className}`}>{timeRange(item.durationSec)}</span>
                </button>
              )
            })}
          </div>
        )}
        {kindVideo.loadingMore && <Spinner size={14} color="var(--color-fg-3)" />}
      </div>

      <p className={`vbd-hint ${stylex.props(sx.hint).className}`}><Icon name="info" size={13} />{isPick ? copy.helpPick : copy.helpRandom}</p>

      <div className={`vbd-pick-foot ${stylex.props(sx.pickFoot).className}`}>
        <span className={`vbd-pick-sum ${stylex.props(sx.pickSum).className}`}>
          <span className={`vbd-pick-summain ${stylex.props(sx.pickSummain).className}`}>{sum}</span>
          <span className={`vbd-pick-sumsub ${stylex.props(sx.pickSumsub, (Boolean((goDisabled))) && sx.pickSumsubDataWarn).className}`} data-warn={goDisabled || undefined}>{sumSub}</span>
        </span>
        <Button xstyle={[sx.vbdModesRadioCard]} size="md" onClick={onClose}>{copy.cancel}</Button>
        <Button xstyle={[sx.vbdModesRadioCard]} size="md" variant="primary" aria-disabled={goDisabled || undefined} disabled={goDisabled}
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
    <div className={`vbd-pick-detail ${stylex.props(sx.pickDetail).className}`}>
      <div className={`vbd-pick-head ${stylex.props(sx.pickHead).className}`}>
        <span className={`vbd-av vbd-av--rect ${stylex.props(sx.avAvRect).className}`} aria-hidden="true"><Icon name="video-play" size={18} /></span>
        <div className={`vbd-pick-titles ${stylex.props(sx.pickTitles).className}`}>
          <span className={`vbd-pick-name ${stylex.props(sx.pickName).className}`}>{video.title}</span>
          <span className={`vbd-pick-meta num ${stylex.props(sx.pickMetaNum).className}`}>
            {copy.videoMeta(platformName(video.platform), duration(video.durationSec), linked ? copy.linkedMeta : '')}
          </span>
        </div>
      </div>
      {video.state === 'ready' && video.file && (
        <video className={`vbd-pick-vthumb ${stylex.props(sx.pickVthumb).className}`} src={backdropVideoFileHref(video.id)} muted playsInline preload="metadata" />
      )}
      <div className={`vbd-segbox ${stylex.props(sx.segbox).className}`}>
        <span className={`vbd-segbox-text num ${stylex.props(sx.segboxTextNum).className}`}>
          <span className={`vbd-segbox-label ${stylex.props(sx.segboxLabel).className}`}>{copy.segLen(timeRange(segLen))}</span>
          <span className={`vbd-segbox-range ${stylex.props(sx.segboxRange).className}`}>{segmentLabel(video.segment.fromSec, video.segment.toSec)}</span>
        </span>
        <a className={`vbd-linkbtn ${stylex.props(sx.linkbtn).className}`} href={`#${paths.library('backdrops', undefined, { nguon: 'videos' })}`}>
          <Icon name="scissor" size={13} />{copy.changeInLibrary}
        </a>
      </div>
      <p className={`vbd-hint ${stylex.props(sx.hint).className}`}><Icon name="info" size={13} />{copy.videoNote}</p>
      <div className={`vbd-pick-foot ${stylex.props(sx.pickFoot).className}`}>
        <span className={`vbd-pick-sum ${stylex.props(sx.pickSum).className}`}>
          <span className={`vbd-pick-summain ${stylex.props(sx.pickSummain).className}`}>{copy.sumVideo(video.title, segmentLabel(video.segment.fromSec, video.segment.toSec))}</span>
          <span className={`vbd-pick-sumsub ${stylex.props(sx.pickSumsub).className}`}>{linked ? copy.sumSubVideoLinked : copy.sumSubVideoAttach}</span>
        </span>
        <Button xstyle={[sx.vbdModesRadioCard]} size="md" variant="primary" disabled={linked} onClick={onGo}>
          {linked ? copy.goLinked : copy.go}
        </Button>
      </div>
    </div>
  )
}

type BackdropPickerProps = {
  template: TemplateDetail
  onSave: (patch: Partial<Recipe>) => Promise<void>
  onClose: () => void
  /** Nguồn mở sẵn khi sửa video của kênh đã gắn. */
  initial?: string
}

const dialogStyle = {
  height: `min(${libraryVars.templateBackdropPickerDlgMaxHeight}, calc(100dvh - var(--vv-s-48)))`,
  padding: 0, borderWidth: 0, background: 'transparent', boxShadow: 'none',
}

export function BackdropPicker(props: BackdropPickerProps) {
  const phone = useMediaQuery('(max-width: 767px)')
  // Giữ vỏ đang mở khi đổi breakpoint để không mất lựa chọn chưa gắn.
  const [asSheet] = useState(phone)
  const [open, setOpen] = useState(true)
  const close = () => { if (asSheet) setOpen(false); else props.onClose() }
  const content = <BackdropPickerContent {...props} onClose={close} hideHeader={asSheet} />
  const title = t.backdrop.template.picker.title(props.template.name)
  return asSheet
    ? <Sheet open={open} onClose={close} onExited={props.onClose} title={title} subtitle={t.backdrop.template.picker.sub} size="lg" height="auto" bodyStyle={sx.sheetBody}>{content}</Sheet>
    : <Dialog label={title} onClose={close} width={libraryVars.templateBackdropPickerDlgMaxWidth} padBody={false} style={dialogStyle}>{content}</Dialog>
}

function BackdropPickerContent({ template, onSave, onClose, initial, hideHeader }: BackdropPickerProps & { hideHeader: boolean }) {
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
    <div className={`vbd-dlg ${stylex.props(sx.dlg, hideHeader && sx.mobileDlg).className}`}>
      {!hideHeader && <div className={`vbd-dlg-head ${stylex.props(sx.dlgHead).className}`}>
        <div className={`vbd-dlg-titles ${stylex.props(sx.dlgTitles).className}`}>
          <h2 className={`vbd-dlg-title ${stylex.props(sx.dlgTitle).className}`}>{copy.title(template.name)}</h2>
          <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.sub}</p>
        </div>
        <Button xstyle={[sx.vbdModesRadioCard]} variant="icon" icon={<Icon name="close" size={16} />} onClick={onClose} aria-label={t.library.close} />
      </div>}
      <div className={`vbd-dlg-body ${stylex.props(sx.dlgBody).className}`}>
        <div className={`vbd-dlg-rail ${stylex.props(sx.dlgRail).className}`}>
          <form className={`vbd-dlg-add ${stylex.props(sx.dlgAdd).className}`} onSubmit={(event) => void addByLink(event)}>
            <label htmlFor="vbd-pick-link" {...stylex.props(text.srOnly)}>{copy.linkLabel}</label>
            <TextField inputStyle={[sx.vbdDlgAddTextField]} id="vbd-pick-link" value={link} placeholder={copy.linkPlaceholder}
              onChange={(value) => setLink(value)} disabled={addBusy} />
            <Button xstyle={[sx.vbdModesRadioCard]} size="sm" type="submit" pending={addBusy} disabled={!link.trim()}>{copy.add}</Button>
          </form>
          {addError && <p className={`vbd-fielderr ${stylex.props(sx.fielderr).className}`} role="alert">{addError}</p>}
          {addNote && <p className={`vbd-addnote ${stylex.props(sx.addnote).className}`}><Icon name="tick-b" size={13} />{addNote}</p>}
          <SourceRail inDialog
            channels={channelList}
            videos={videoList}
            selected={sel}
            linkedIds={linkedIds}
            videoRows="items"
            onSelect={setSelected}
            ariaLabel={copy.railAria}
          />
        </div>
        <div className={`vbd-dlg-detail ${stylex.props(sx.dlgDetail).className}`}>
          {selChannel && (
            <ChannelDetail key={selChannel.id} channel={selChannel}
              link={backdrop.channels.find((c) => c.channelId === selChannel.id) ?? null}
              onGo={(next) => saveChannel(selChannel.id, next)} onClose={onClose} />
          )}
          {selVideo && !selChannel && (
            <VideoDetail video={selVideo} linked={backdrop.fixedVideoIds.includes(selVideo.id)}
              onGo={() => attachVideo(selVideo.id)} />
          )}
          {!selChannel && !selVideo && (
            <div className={`vbd-pane--load ${stylex.props(sx.paneLoad).className}`}><Spinner size={16} color="var(--color-fg-3)" /></div>
          )}
        </div>
      </div>
    </div>
  )
}

const sx = stylex.create({
  pickDetail: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)" },
  pickHead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)" },
  av: { borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, overflowX: "hidden", overflowY: "hidden", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], objectFit: "cover" },
  pickTitles: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-3)", columnGap: "var(--vv-s-3)" },
  pickName: { fontSize: "var(--vv-s-14)", fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.compactLineHeight },
  pickMetaNum: { fontSize: "var(--vv-s-11)", color: color["--color-fg-3"] },
  linkbtn: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", fontSize: "var(--vv-s-11)", color: color["--color-info"], textDecoration: { default: "none", ":hover": "underline" }, flexShrink: 0 },
  modes: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  vbdModesRadioCard: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  kinds: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  kindlabel: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  kindchips: { display: "flex", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", flexWrap: "wrap" },
  kindchip: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", paddingTop: "var(--vv-s-5)", paddingRight: "var(--vv-s-9)", paddingBottom: "var(--vv-s-5)", paddingLeft: "var(--vv-s-9)", fontSize: "var(--vv-s-11)", cursor: "pointer", backgroundColor: "transparent", backgroundImage: "none", color: color["--color-fg-2"], fontFamily: "inherit", minHeight: { default: null, "@media (pointer: coarse)": "var(--vv-s-28)", "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  kindchipDataOn: { borderTopColor: color["--color-accent"], borderRightColor: color["--color-accent"], borderBottomColor: color["--color-accent"], borderLeftColor: color["--color-accent"], color: color["--color-accent"] },
  kindchipAriaDisabled: { opacity: libraryVars.templateBackdropPickerKindchipAriaDisabledOpacity, cursor: "not-allowed" },
  flex: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis },
  picknoteNum: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  picknoteNumDataWarn: { color: color["--color-warn"] },
  pickGridscroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minHeight: libraryVars.promptsTabSearchFlexBasis, maxHeight: libraryVars.templateBackdropPickerPickGridscrollMaxHeight, overflowY: "auto" },
  paneLoad: { display: "flex", alignItems: "center", justifyContent: "center", minHeight: "var(--vv-s-240)" },
  pickGrid: { display: "grid", gridTemplateColumns: libraryVars.templateBackdropPickerPickGridGridTemplateColumns, rowGap: "var(--vv-s-14)", columnGap: "var(--vv-s-10)" },
  pickItem: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", textAlign: "left", borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", cursor: "pointer", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, fontFamily: "inherit", color: color["--color-fg"], minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": "var(--vv-s-2)" }, borderTopLeftRadius: { default: null, ":focus-visible": "var(--vv-r-8)" }, borderTopRightRadius: { default: null, ":focus-visible": "var(--vv-r-8)" }, borderBottomRightRadius: { default: null, ":focus-visible": "var(--vv-r-8)" }, borderBottomLeftRadius: { default: null, ":focus-visible": "var(--vv-r-8)" } },
  pickItemDataDim: { opacity: libraryVars.templateBackdropPickerPickItemDataDimOpacity },
  thumb: { position: "relative", aspectRatio: libraryVars.videoAspectRatio, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", overflowX: "hidden", overflowY: "hidden", backgroundColor: line["--line-1"], backgroundImage: "none", display: "block", width: libraryVars.fullExtent },
  thumbDataRing: { outlineWidth: "var(--vv-s-2)", outlineStyle: "solid", outlineColor: color["--color-accent"], outlineOffset: "var(--vv-s-2)" },
  thumbImg: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  kind: { position: "absolute", top: "var(--vv-s-6)", left: "var(--vv-s-6)", fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-6)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-6)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", backgroundColor: "var(--vv-scrim)", backgroundImage: "none", color: color["--color-fg"] },
  picktick: { position: "absolute", top: "var(--vv-s-6)", right: "var(--vv-s-6)", width: "var(--vv-s-18)", height: "var(--vv-s-18)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: color["--color-accent"], backgroundImage: "none", color: color["--color-bg"], display: "flex", alignItems: "center", justifyContent: "center" },
  pickTitle: { fontSize: libraryVars.microFontSize, fontWeight: libraryVars.labelWeight, lineHeight: libraryVars.compactLineHeight, display: "-webkit-box", WebkitLineClamp: libraryVars.promptsTabPcardTextWebkitLineClamp, WebkitBoxOrient: "vertical", overflowX: "hidden", overflowY: "hidden" },
  vsubNum: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  hint: { fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, display: "flex", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", alignItems: "flex-start" },
  pickFoot: { display: "flex", alignItems: "center", justifyContent: "flex-end", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-1"], paddingTop: "var(--vv-s-12)", marginTop: "auto" },
  pickSum: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  pickSummain: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.labelWeight, whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis" },
  pickSumsub: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  pickSumsubDataWarn: { color: color["--color-warn"] },
  avAvRect: { borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", flexShrink: 0, overflowX: "hidden", overflowY: "hidden", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg-3"], objectFit: "cover", width: "var(--vv-s-40)", height: "var(--vv-s-40)", backgroundColor: line["--line-1"], backgroundImage: "none" },
  pickVthumb: { width: libraryVars.fullExtent, maxHeight: libraryVars.promptsTabSearchFlexBasis, objectFit: "cover", borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", backgroundColor: line["--line-1"], backgroundImage: "none" },
  segbox: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-1"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-1"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-1"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-1"], borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", paddingTop: "var(--vv-s-7)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-7)", paddingLeft: "var(--vv-s-10)" },
  segboxTextNum: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  segboxLabel: { fontSize: "var(--vv-s-11)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"] },
  segboxRange: { fontSize: "var(--vv-s-11)", color: color["--color-fg-3"] },
  dlg: { width: libraryVars.fullExtent, maxWidth: libraryVars.templateBackdropPickerDlgMaxWidth, height: libraryVars.fullExtent, maxHeight: libraryVars.templateBackdropPickerDlgMaxHeight, backgroundColor: color["--color-bg"], backgroundImage: "none", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-14)", borderTopRightRadius: "var(--vv-r-14)", borderBottomRightRadius: "var(--vv-r-14)", borderBottomLeftRadius: "var(--vv-r-14)", display: "flex", flexDirection: "column", overflowX: "hidden", overflowY: "hidden" },
  sheetBody: { display: 'flex', minHeight: 0, overflow: 'hidden' },
  mobileDlg: { borderWidth: 0, borderRadius: 0, height: libraryVars.templateBackdropPickerDlgMaxHeight, maxHeight: 'calc(100dvh - var(--vv-s-160))', minHeight: 0 },
  dlgHead: { display: "flex", alignItems: "flex-start", justifyContent: "space-between", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", paddingTop: "var(--vv-s-16)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-18)", content: { default: null, "::after": "''" } },
  dlgTitles: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-3)", columnGap: "var(--vv-s-3)" },
  dlgTitle: { fontSize: "var(--vv-s-15)", fontWeight: libraryVars.headingWeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 },
  dlgBody: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minHeight: 0, display: "grid", gridTemplateColumns: { default: libraryVars.templateBackdropPickerDlgBodyGridTemplateColumns, "@media (max-width: 767px)": libraryVars.templateBackdropPickerDlgBodyGridTemplateColumns2 }, overflowY: { default: null, "@media (max-width: 767px)": "auto" } },
  dlgRail: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", paddingTop: "var(--vv-s-12)", paddingRight: "var(--vv-s-12)", paddingBottom: "var(--vv-s-12)", paddingLeft: "var(--vv-s-12)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-1"], overflowX: { default: "hidden", "@media (max-width: 767px)": "visible" }, overflowY: { default: "hidden", "@media (max-width: 767px)": "visible" } },
  dlgAdd: { display: "flex", alignItems: "flex-end", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  vbdDlgAddTextField: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  fielderr: { fontSize: "var(--vv-s-11)", color: color["--color-err"], marginTop: "var(--vv-s-4)", marginRight: 0, marginBottom: 0, marginLeft: 0 },
  addnote: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-11)", color: color["--color-ok"], marginTop: "var(--vv-s-4)", marginRight: 0, marginBottom: 0, marginLeft: 0 },
  dlgDetail: { minWidth: 0, overflowY: { default: "auto", "@media (max-width: 767px)": "visible" }, paddingTop: "var(--vv-s-16)", paddingRight: "var(--vv-s-18)", paddingBottom: "var(--vv-s-16)", paddingLeft: "var(--vv-s-18)", display: "flex", overflowX: { default: null, "@media (max-width: 767px)": "visible" } },
})
