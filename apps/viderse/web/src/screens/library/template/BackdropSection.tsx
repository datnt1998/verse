import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Mục Nền & hiệu ứng của trang Template (Template.dc.html › section=backdrop):
// danh sách nguồn đã gắn (kênh Ngẫu nhiên/ghim + video lẻ Luôn có), thanh mix
// "Nền của một video" (GET /templates/:id/backdrop/mix?lengthSec=2520) kèm
// cảnh báo ít clip/40-clip, cách cắt clip, các thanh trượt Chỉnh nền
// (recipe.appearance), Lớp hiệu ứng (recipe.overlays + listOverlays) và hộp
// Gắn kênh hoặc video (BackdropPicker). Lưu qua onSave — trang gộp + PATCH.
import { useMemo, useState } from 'react'
import { Banner, Button, Dialog, Icon, NumberStepper, RadioDot, Skeleton, Switch } from '@noverse/ui'

import { t } from '../../../i18n/vi'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import {
  backdropVideoFileHref, getBackdropMix, listBackdropChannels, listBackdropVideos,
  type BackdropChannel, type BackdropMix, type BackdropVideo,
} from '../../../api/backdrop'
import { listOverlays, type Recipe, type RecipeAppearance, type RecipeBackdrop, type TemplateDetail } from '../../../api/library'
import { channelTint } from '../../../lib/channelTint'
import { SliderField } from './SliderField'
import { BackdropPicker } from './BackdropPicker'

import { duration, initials, kindName, num, platformName, segmentLabel, timeRange } from '../backdrops/fmt'

/** Ví dụ cố định của thanh mix: video dài 42 phút (artboard). */
const MIX_TARGET_SEC = 42 * 60
const CLIP_MIN_SEC = 10
const CLIP_MAX_SEC = 600
const CLIP_MAX_COUNT = 40

const APPEARANCE_DEFAULT: RecipeAppearance = {
  opacity: 0.5, blurRadius: 0, brightness: 0.5, contrast: 0.5, saturation: 0.5,
  hueShift: 0, speed: 1, zoom: 1, flipX: false, flipY: false,
}

const EMPTY_BACKDROP: RecipeBackdrop = { channels: [], clipCount: null, clipSec: 60, fixedVideoIds: [] }

const pct = (v: number) => `${Math.round(v * 100)}%`
const times = (v: number) => `${Number(v.toFixed(2))}×`
const deg = (v: number) => (v === 0 ? '0°' : `+${Math.round(v)}°`)

/* ── Hàng nguồn đã gắn ── */

function ChannelRow({ link, channel, onEdit, onRemove }: {
  link: RecipeBackdrop['channels'][number]
  channel: BackdropChannel | undefined
  onEdit: () => void
  onRemove: () => void
}) {
  const copy = t.backdrop.template
  const name = channel?.title ?? channel?.handle ?? link.channelId
  const pick = link.pinned.length > 0
  const kinds = link.kinds.length ? link.kinds.map(kindName).join(', ') : copy.howRandom
  return (
    <div className={`vbd-srcrow ${stylex.props(sx.srcrow, (Boolean((channel ? undefined : ''))) && sx.srcrowDataMissing).className}`} data-missing={channel ? undefined : ''}>
      {channel?.avatarUrl
        ? <img className={`vbd-av ${stylex.props(sx.av).className}`} src={channel.avatarUrl} alt="" width={30} height={30} style={{ borderRadius: '50%' }} />
        : <span className={`vbd-av ${stylex.props(sx.av).className}`} aria-hidden="true" style={{ width: 30, height: 30, background: channelTint(link.channelId) }}>{initials(name)}</span>}
      <span className={`vbd-srcmain ${stylex.props(sx.srcmain).className}`}>
        <span className={`vbd-srchead ${stylex.props(sx.srchead).className}`}>
          <span className={`vbd-srcname ${stylex.props(sx.srcname).className}`}>{name}</span>
          <span className={`vbd-tag ${stylex.props(sx.tag).className}`}>{pick ? copy.howPick(num(link.pinned.length)) : copy.tagRandom}</span>
        </span>
        <span className={`vbd-srcmeta ${stylex.props(sx.srcmeta).className}`}>
          {channel && platformName(channel.platform)}{channel ? ' · ' : ''}
          {pick ? copy.howPick(num(link.pinned.length)) : copy.howRandom} · {kinds}
        </span>
      </span>
      <Button xstyle={[sx.vbdSrcrowButton]} size="sm" icon={<Icon name="edit" size={13} />} onClick={onEdit}>{copy.chooseVideos}</Button>
      <Button xstyle={[sx.vbdSrcrowButton]} variant="icon" icon={<Icon name="close" size={15} />} aria-label={copy.removeAria(name)} onClick={onRemove} />
    </div>
  )
}

function FixedVideoRow({ video, missing, onRemove }: {
  video: BackdropVideo | undefined
  missing: boolean
  onRemove: () => void
}) {
  const copy = t.backdrop.template
  const name = video?.title ?? '—'
  return (
    <div className={`vbd-srcrow ${stylex.props(sx.srcrow, (Boolean((missing ? '' : undefined))) && sx.srcrowDataMissing).className}`} data-missing={missing ? '' : undefined}>
      {video?.state === 'ready' && video.file
        ? <span className={`vbd-av vbd-av--rect ${stylex.props(sx.avAvRect).className}`} style={{ width: 54, height: 30 }}>
            <video {...stylex.props(sx.vbdAvVideo)} src={backdropVideoFileHref(video.id)} muted playsInline preload="metadata" />
          </span>
        : <span className={`vbd-av vbd-av--rect ${stylex.props(sx.avAvRect).className}`} style={{ width: 54, height: 30 }}><Icon name="video-play" size={14} /></span>}
      <span className={`vbd-srcmain ${stylex.props(sx.srcmain).className}`}>
        <span className={`vbd-srcname ${stylex.props(sx.srcname).className}`}>{name}</span>
        <span className={`vbd-srcmeta ${stylex.props(sx.srcmeta).className}`}>
          {video && copy.videoSub(platformName(video.platform), segmentLabel(video.segment.fromSec, video.segment.toSec))}
        </span>
      </span>
      <span className={`vbd-tag ${stylex.props(sx.tag2).className}`} data-tone="fixed">{copy.tagFixed}</span>
      <Button xstyle={[sx.vbdSrcrowButton]} variant="icon" icon={<Icon name="close" size={15} />} aria-label={copy.removeAria(name)} onClick={onRemove} />
    </div>
  )
}

/* ── Thanh mix ── */

function MixBar({ mix, backdrop, channels, onBumpClipSec }: {
  mix: BackdropMix | undefined
  backdrop: RecipeBackdrop
  channels: BackdropChannel[]
  /** "Tăng mỗi clip lên N giây" — N = ceil(cần / 40) */
  onBumpClipSec: (sec: number) => void
}) {
  const pinnedOf = (channelId: string) => backdrop.channels.find((c) => c.channelId === channelId)?.pinned.length ?? 0
  const copy = t.backdrop.template
  if (backdrop.channels.length === 0 && backdrop.fixedVideoIds.length === 0) {
    return <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.mixNoteNone}</p>
  }
  if (!mix) {
    return <Skeleton height={34} width="100%" />
  }
  const channelById = new Map(channels.map((c) => [c.id, c]))
  const segs: { id: string; sec: number; tone: 'fixed' | 'channel' | 'loop'; platform?: string; label: string; sub: string }[] = []
  if (mix.fixedSec > 0) {
    segs.push({
      id: 'fixed', sec: mix.fixedSec, tone: 'fixed',
      label: copy.mixFixed, sub: copy.mixFixedDetail(num(backdrop.fixedVideoIds.length), timeRange(mix.fixedSec)),
    })
  }
  for (const c of mix.perChannel) {
    const channel = channelById.get(c.channelId)
    segs.push({
      id: c.channelId, sec: c.clips * backdrop.clipSec, tone: 'channel', platform: channel?.platform ?? 'twitch',
      label: channel?.title ?? channel?.handle ?? c.channelId,
      sub: pinnedOf(c.channelId) > 0
        ? copy.mixPickDetail(num(c.clips), num(pinnedOf(c.channelId)))
        : c.repeats ? copy.mixRepeatDetail(num(c.clips), num(c.freshVideos)) : copy.mixRandomDetail(num(c.clips)),
    })
  }
  if (mix.loopSec > 0) {
    segs.push({ id: 'loop', sec: mix.loopSec, tone: 'loop', label: copy.mixLoop, sub: copy.mixLoopDetail(timeRange(mix.loopSec)) })
  }
  const total = Math.max(1, segs.reduce((n, s) => n + s.sec, 0))
  const aria = copy.mixAria('42', segs.map((s) => s.label).join(', '))
  const fixedCount = backdrop.fixedVideoIds.length
  const hasChannels = backdrop.channels.length > 0
  // Cảnh báo ít video bốc clip: bất kỳ kênh nào lặp video → gợi ý gắn thêm.
  const fewest = mix.perChannel.length ? Math.min(...mix.perChannel.map((c) => c.freshVideos)) : 0
  const cappedSec = mix.need * backdrop.clipSec
  const neededClipSec = Math.min(CLIP_MAX_SEC, Math.max(CLIP_MIN_SEC, Math.ceil((MIX_TARGET_SEC - mix.fixedSec) / CLIP_MAX_COUNT)))
  return (
    <div className={`vbd-mix ${stylex.props(sx.mix).className}`}>
      <div className={`vbd-mixbar ${stylex.props(sx.mixbar).className}`} role="img" aria-label={aria}>
        {segs.map((seg) => (
          <div key={seg.id}
            className={`vbd-mixseg ${stylex.props(sx.mixseg, (seg.tone === 'fixed') && sx.mixsegVbdMixsegFixed, (seg.tone !== 'fixed' && seg.tone !== 'loop') && sx.mixsegVbdMixsegChannel, (seg.tone !== 'fixed' && seg.tone !== 'loop' && seg.platform === 'bilibili') && sx.mixsegVbdMixsegBilibili, (seg.tone === 'loop') && sx.mixsegVbdMixsegLoop).className}`}
            style={{ flexGrow: seg.sec }}>
            <span className={`vbd-mixlab ${stylex.props(sx.mixlab, (seg.tone === 'loop') && sx.mixlabVbdMixlab).className}`}><span>{seg.label}</span><span className={`vbd-mixsub ${stylex.props(sx.mixsub).className}`}>{seg.sub}</span></span>
          </div>
        ))}
      </div>
      <div className={`vbd-mixlegend ${stylex.props(sx.mixlegend).className}`}>
        {mix.fixedSec > 0 && (
          <span className={`vbd-mixleg ${stylex.props(sx.mixleg).className}`}><i {...stylex.props(sx.vbdMixlegI)} style={{ background: 'var(--vv-fixed-28)' }} />{copy.mixFixedDetail(num(fixedCount), timeRange(mix.fixedSec))}</span>
        )}
        {mix.perChannel.map((c) => {
          const channel = channelById.get(c.channelId)
          return (
            <span key={c.channelId} className={`vbd-mixleg ${stylex.props(sx.mixleg).className}`}>
              <i {...stylex.props(sx.vbdMixlegI)} style={{ background: `var(--vv-plat-${channel?.platform === 'bilibili' ? 'bilibili' : 'twitch'})` }} />
              {channel?.title ?? channel?.handle ?? c.channelId}{' '}
              {pinnedOf(c.channelId) > 0
                ? copy.mixPickDetail(num(c.clips), num(pinnedOf(c.channelId)))
                : c.repeats ? copy.mixRepeatDetail(num(c.clips), num(c.freshVideos)) : copy.mixRandomDetail(num(c.clips))}
            </span>
          )
        })}
        {mix.loopSec > 0 && (
          <span className={`vbd-mixleg ${stylex.props(sx.mixleg).className}`}><i {...stylex.props(sx.vbdMixlegI)} style={{ background: 'var(--vv-loop-bg)' }} />{copy.mixLoopDetail(timeRange(mix.loopSec))}</span>
        )}
      </div>
      <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>
        {hasChannels ? copy.mixNote : copy.mixNoteFixed(timeRange(mix.fixedSec || backdrop.clipSec))}
      </p>
      {mix.capped ? (
        <Banner tone="warning" title={copy.mixWarn(num(Math.ceil((MIX_TARGET_SEC - mix.fixedSec) / backdrop.clipSec)), timeRange(cappedSec))}
          action={{ label: copy.mixFix(num(neededClipSec)), onClick: () => onBumpClipSec(neededClipSec) }} />
      ) : mix.perChannel.some((c) => c.repeats) ? (
        <Banner tone="warning" title={copy.fewClips(num(fewest))}
          action={mix.loopSec > 0 ? { label: copy.mixFix(num(neededClipSec)), onClick: () => onBumpClipSec(neededClipSec) } : undefined} />
      ) : null}
    </div>
  )
}

/* ── Mục chính ── */

export function BackdropSection({ template, onSave, saving }: {
  template: TemplateDetail
  onSave: (patch: Partial<Recipe>) => Promise<void>
  saving: boolean
}) {
  const copy = t.backdrop.template
  const backdrop = template.recipe.backdrop ?? EMPTY_BACKDROP
  const appearance = template.recipe.appearance
  const channels = useResource(keys.backdropChannels(), ({ signal }) => listBackdropChannels(signal))
  const videos = useResource(keys.backdropVideos(), ({ signal }) => listBackdropVideos(signal))
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const mix = useResource(
    keys.backdropMix(template.id, MIX_TARGET_SEC),
    ({ signal }) => getBackdropMix(template.id, MIX_TARGET_SEC, signal),
  )
  const [picker, setPicker] = useState<{ edit?: string } | null>(null)
  const [overlayPick, setOverlayPick] = useState(false)
  const [cutOpen, setCutOpen] = useState(false)

  const channelById = useMemo(() => new Map((channels.data ?? []).map((c) => [c.id, c])), [channels.data])
  const videoById = useMemo(() => new Map((videos.data ?? []).map((v) => [v.id, v])), [videos.data])
  const overlayById = useMemo(() => new Map((overlays.data?.items ?? []).map((o) => [o.id, o])), [overlays.data])
  const attachedOverlays = template.recipe.overlays
  const overlayCandidates = (overlays.data?.items ?? []).filter((o) => !attachedOverlays.includes(o.id))
  const missing = backdrop.channels.some((c) => !channelById.has(c.channelId)) || backdrop.fixedVideoIds.some((id) => !videoById.has(id))

  const saveBackdrop = (patch: Partial<RecipeBackdrop>) => {
    void onSave({ backdrop: { ...backdrop, ...patch } })
  }
  const saveAppearance = (patch: Partial<RecipeAppearance>) => {
    void onSave({ appearance: { ...appearance, ...patch } })
  }
  const saveOverlays = (next: string[]) => {
    void onSave({ overlays: next })
  }

  const removeChannel = (id: string) => saveBackdrop({ channels: backdrop.channels.filter((c) => c.channelId !== id) })
  const removeVideo = (id: string) => saveBackdrop({ fixedVideoIds: backdrop.fixedVideoIds.filter((v) => v !== id) })

  const manual = backdrop.clipCount !== null

  return (
    <section className={`vt-sec ${stylex.props(sx.sec).className}`} aria-labelledby="vt-backdrop-title" aria-busy={saving || undefined}>
      <div className={`vt-sec-head ${stylex.props(sx.secHead).className}`}>
        <h2 {...stylex.props(sx.vtSecHeadH2)} id="vt-backdrop-title">{t.library.template.sections.backdrop.label}</h2>
        <p className={`vt-help ${stylex.props(sx.help).className}`}>{copy.help}</p>
      </div>

      {/* Lấy nền từ */}
      <div className={`vt-card ${stylex.props(sx.card).className}`}>
        <div className={`vt-card-head ${stylex.props(sx.cardHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.fromLabel}</span>
          <Button xstyle={[sx.vtCardHeadButton]} size="sm" variant="primary" onClick={() => setPicker({})}>{copy.attach}</Button>
        </div>
        <div className={`vbd-srcs ${stylex.props(sx.srcs).className}`} role="list" aria-label={copy.sourceListAria}>
          {backdrop.channels.map((link) => (
            <ChannelRow key={link.channelId} link={link} channel={channelById.get(link.channelId)}
              onEdit={() => setPicker({ edit: link.channelId })}
              onRemove={() => removeChannel(link.channelId)} />
          ))}
          {backdrop.fixedVideoIds.map((id) => (
            <FixedVideoRow key={id} video={videoById.get(id)} missing={!videoById.get(id)}
              onRemove={() => removeVideo(id)} />
          ))}
          {backdrop.channels.length === 0 && backdrop.fixedVideoIds.length === 0 && (
            <div className={`vbd-srcempty ${stylex.props(sx.srcempty).className}`}>
              <span className={`vbd-empty-title ${stylex.props(sx.emptyTitle).className}`}>{copy.emptyTitle}</span>
              <span className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.emptyHint}</span>
            </div>
          )}
        </div>
        {missing && <Banner tone="error" title={copy.missingTitle}>{copy.missingBody}</Banner>}
        <p className={`vbd-hint ${stylex.props(sx.hint).className}`}><Icon name="info" size={13} />{copy.fromHelp}</p>
      </div>

      {/* Nền của một video + Cách cắt clip */}
      {(backdrop.channels.length > 0 || backdrop.fixedVideoIds.length > 0) && (
        <div className={`vt-card ${stylex.props(sx.card).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.mixLabel} · {copy.mixExample}</span>
          <MixBar mix={mix.data} backdrop={backdrop} channels={channels.data ?? []}
            onBumpClipSec={(sec) => saveBackdrop({ clipSec: sec })} />

          <div className={`vbd-cut ${stylex.props(sx.cut).className}`}>
            <button type="button" className={`vbd-cutline ${stylex.props(sx.cutline).className}`} aria-expanded={cutOpen}
              onClick={() => setCutOpen((open) => !open)}>
              <Icon name={cutOpen ? 'arrow-d' : 'arrow-r'} size={12} />
              {copy.cutToggle} — {manual ? copy.cutSumManual(num(backdrop.clipCount ?? 1), num(backdrop.clipSec)) : copy.cutSumAuto(num(backdrop.clipSec))}
            </button>
            {cutOpen && (
              <>
                <div role="radiogroup" aria-label={copy.modeAria} className={`vbd-cutrows ${stylex.props(sx.cutrows).className}`}>
                  <button type="button" role="radio" aria-checked={!manual} className={`vbd-radioline ${stylex.props(sx.radioline).className}`}
                    onClick={() => saveBackdrop({ clipCount: null })}>
                    <RadioDot checked={!manual} />{copy.modeAuto}
                  </button>
                  <button type="button" role="radio" aria-checked={manual} className={`vbd-radioline ${stylex.props(sx.radioline).className}`}
                    onClick={() => saveBackdrop({ clipCount: Math.min(CLIP_MAX_COUNT, Math.max(1, backdrop.clipCount ?? 9)) })}>
                    <RadioDot checked={manual} />{copy.modeManual}
                  </button>
                  {manual && (
                    <span className={`vbd-cutline ${stylex.props(sx.cutline2).className}`}>
                      <NumberStepper buttonStyle={sx.touchTarget} inputStyle={sx.touchTarget}
                        value={backdrop.clipCount ?? 1} min={1} max={CLIP_MAX_COUNT} step={1}
                        aria-label={copy.clipCountAria}
                        onChange={(value) => saveBackdrop({ clipCount: value })} />
                      {copy.clipCountPost}
                    </span>
                  )}
                  <span className={`vbd-cutline ${stylex.props(sx.cutline2).className}`}>
                    {copy.clipSecPre}
                    <NumberStepper buttonStyle={sx.touchTarget} inputStyle={sx.touchTarget}
                      value={backdrop.clipSec} min={CLIP_MIN_SEC} max={CLIP_MAX_SEC} step={5}
                      aria-label={copy.clipSecAria}
                      onChange={(value) => saveBackdrop({ clipSec: value })} />
                    {copy.clipSecPost}
                  </span>
                </div>
                <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.cutHelp}</p>
              </>
            )}
          </div>
        </div>
      )}

      {/* Chỉnh nền */}
      <div className={`vt-card vbd-part ${stylex.props(sx.cardPart).className}`}>
        <div className={`vbd-part-head ${stylex.props(sx.partHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.adjustTitle}</span>
          <Button xstyle={[sx.vtCardHeadButton]} size="sm" variant="quiet" onClick={() => void onSave({ appearance: { ...APPEARANCE_DEFAULT } })}>{copy.adjustReset}</Button>
        </div>
        <div className={`vbd-adjust ${stylex.props(sx.adjust).className}`}>
          <SliderField label={copy.adjust.opacity} value={appearance.opacity} min={0} max={1} step={0.05}
            formatValue={pct} onChange={(v) => saveAppearance({ opacity: v })} />
          <SliderField label={copy.adjust.blur} value={appearance.blurRadius} min={0} max={20} step={1}
            formatValue={(v) => `${Math.round(v)} px`} onChange={(v) => saveAppearance({ blurRadius: Math.round(v) })} />
          <SliderField label={copy.adjust.brightness} value={appearance.brightness} min={0} max={1} step={0.05}
            formatValue={pct} onChange={(v) => saveAppearance({ brightness: v })} />
          <SliderField label={copy.adjust.contrast} value={appearance.contrast} min={0} max={1} step={0.05}
            formatValue={pct} onChange={(v) => saveAppearance({ contrast: v })} />
          <SliderField label={copy.adjust.saturation} value={appearance.saturation} min={0} max={1} step={0.05}
            formatValue={pct} onChange={(v) => saveAppearance({ saturation: v })} />
          <SliderField label={copy.adjust.hue} value={appearance.hueShift * 360} min={-180} max={180} step={5}
            formatValue={deg} onChange={(v) => saveAppearance({ hueShift: v / 360 })} />
          <SliderField label={copy.adjust.speed} value={appearance.speed} min={0.5} max={2} step={0.05}
            formatValue={times} onChange={(v) => saveAppearance({ speed: v })} />
          <SliderField label={copy.adjust.zoom} value={appearance.zoom} min={0.5} max={2} step={0.05}
            formatValue={times} onChange={(v) => saveAppearance({ zoom: v })} />
        </div>
        <div className={`vbd-flips ${stylex.props(sx.flips).className}`}>
          <Switch xstyle={[sx.vtCardHeadButton]} label={copy.flipX} checked={appearance.flipX} onChange={(on) => saveAppearance({ flipX: on })} />
          <Switch xstyle={[sx.vtCardHeadButton]} label={copy.flipY} checked={appearance.flipY} onChange={(on) => saveAppearance({ flipY: on })} />
        </div>
      </div>

      {/* Lớp hiệu ứng */}
      <div className={`vt-card vbd-part ${stylex.props(sx.cardPart).className}`}>
        <div className={`vbd-part-head ${stylex.props(sx.partHead).className}`}>
          <span className={`vt-label ${stylex.props(sx.label).className}`}>{copy.overlayTitle}</span>
          <Button xstyle={[sx.vtCardHeadButton]} size="sm" variant="quiet" disabled={overlayCandidates.length === 0}
            onClick={() => setOverlayPick(true)}>{copy.overlayAdd}</Button>
        </div>
        {attachedOverlays.length === 0 ? (
          <div className={`vbd-srcempty ${stylex.props(sx.srcempty).className}`}>
            <span className={`vbd-empty-title ${stylex.props(sx.emptyTitle).className}`}>{copy.overlayEmptyTitle}</span>
            <span className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.overlayEmptyHint}</span>
          </div>
        ) : (
          <div className={`vbd-ovlist ${stylex.props(sx.ovlist).className}`}>
            {attachedOverlays.map((id) => {
              const overlay = overlayById.get(id)
              return (
                <div key={id} className={`vbd-ovrow ${stylex.props(sx.ovrow, (Boolean((overlay ? undefined : ''))) && sx.ovrowDataMissing).className}`} data-missing={overlay ? undefined : ''}>
                  <span className={`vbd-ovthumb ${stylex.props(sx.ovthumb).className}`}>
                    {overlay?.mime.startsWith('video/')
                      ? <video {...stylex.props(sx.vbdOvthumbVideo)} src={overlay.url} muted playsInline preload="metadata" />
                      : overlay?.mime.startsWith('image/') ? <img {...stylex.props(sx.vbdOvthumbImg)} src={overlay.url} alt="" /> : <Icon name="layer" size={14} />}
                  </span>
                  <span className={`vbd-ovmain ${stylex.props(sx.ovmain).className}`}>
                    <span className={`vbd-ovname ${stylex.props(sx.ovname).className}`}>{overlay?.name ?? id}</span>
                    <span className={`vbd-ovsub ${stylex.props(sx.ovsub).className}`}>
                      {overlay ? (overlay.durationSec !== null ? copy.overlayDuration(duration(overlay.durationSec)) : copy.overlayBuiltin) : copy.missingTitle}
                    </span>
                  </span>
                  <Button xstyle={[sx.vtCardHeadButton]} variant="icon" icon={<Icon name="close" size={15} />}
                    aria-label={copy.overlayRemoveAria(overlay?.name ?? id)}
                    onClick={() => saveOverlays(attachedOverlays.filter((x) => x !== id))} />
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Hộp chọn hiệu ứng */}
      {overlayPick && (
        <Dialog title={copy.overlayPickTitle} onClose={() => setOverlayPick(false)} width={520}>
          <div className={`vbd-ovlist ${stylex.props(sx.ovlist).className}`}>
            {overlayCandidates.length === 0 && <p className={`vbd-hint ${stylex.props(sx.hint).className}`}>{copy.overlayPickEmpty}</p>}
            {overlayCandidates.map((overlay) => (
              <button key={overlay.id} type="button" className={`vbd-ovrow ${stylex.props(sx.ovrow2).className}`} style={{ cursor: 'pointer' }}
                onClick={() => { saveOverlays([...attachedOverlays, overlay.id]); setOverlayPick(false) }}>
                <span className={`vbd-ovthumb ${stylex.props(sx.ovthumb).className}`}>
                  {overlay.mime.startsWith('video/')
                    ? <video {...stylex.props(sx.vbdOvthumbVideo)} src={overlay.url} muted playsInline preload="metadata" />
                    : overlay.mime.startsWith('image/') ? <img {...stylex.props(sx.vbdOvthumbImg)} src={overlay.url} alt="" /> : <Icon name="layer" size={14} />}
                </span>
                <span className={`vbd-ovmain ${stylex.props(sx.ovmain).className}`}>
                  <span className={`vbd-ovname ${stylex.props(sx.ovname).className}`}>{overlay.name}</span>
                  <span className={`vbd-ovsub ${stylex.props(sx.ovsub).className}`}>{overlay.durationSec !== null ? copy.overlayDuration(duration(overlay.durationSec)) : copy.overlayBuiltin}</span>
                </span>
              </button>
            ))}
          </div>
        </Dialog>
      )}

      {/* Hộp gắn kênh/video */}
      {picker && (
        <div className={`vbd-dlg-backdrop ${stylex.props(sx.dlgBackdrop).className}`} onClick={(event) => { if (event.target === event.currentTarget) setPicker(null) }}>
          <BackdropPicker template={template} onSave={onSave} onClose={() => setPicker(null)} initial={picker.edit} key={picker.edit ?? 'new'} />
        </div>
      )}
    </section>
  )
}

const sx = stylex.create({
  srcrow: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", width: libraryVars.fullExtent, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", paddingTop: "var(--vv-s-8)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-8)", paddingLeft: "var(--vv-s-10)", backgroundColor: color["--color-bg"], backgroundImage: "none" },
  srcrowDataMissing: { borderTopColor: color["--color-err"], borderRightColor: color["--color-err"], borderBottomColor: color["--color-err"], borderLeftColor: color["--color-err"] },
  av: { borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", flexShrink: 0, overflowX: "hidden", overflowY: "hidden", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], objectFit: "cover" },
  srcmain: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)", textAlign: "left" },
  srchead: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)", minWidth: 0 },
  srcname: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.labelWeight, lineHeight: libraryVars.compactLineHeight, whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis" },
  tag: { fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", flexShrink: 0, backgroundColor: line["--line-1"], backgroundImage: "none", color: color["--color-fg-3"], position: "static" },
  srcmeta: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"], display: "flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)" },
  vbdSrcrowButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  avAvRect: { borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", flexShrink: 0, overflowX: "hidden", overflowY: "hidden", display: "inline-flex", alignItems: "center", justifyContent: "center", fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg-3"], objectFit: "cover", width: "var(--vv-s-40)", height: "var(--vv-s-40)", backgroundColor: line["--line-1"], backgroundImage: "none" },
  vbdAvVideo: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  tag2: { fontSize: libraryVars.badgeFontSize, fontWeight: libraryVars.headingWeight, lineHeight: libraryVars.fillRatio, paddingTop: "var(--vv-s-4)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-4)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", flexShrink: 0, backgroundColor: "var(--vv-fixed-10)", backgroundImage: "none", color: "var(--vv-fixed)" },
  hint: { fontSize: libraryVars.captionFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, display: "flex", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", alignItems: "flex-start" },
  mix: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  mixbar: { display: "flex", height: "var(--vv-s-34)", borderTopLeftRadius: "var(--vv-r-8)", borderTopRightRadius: "var(--vv-r-8)", borderBottomRightRadius: "var(--vv-r-8)", borderBottomLeftRadius: "var(--vv-r-8)", overflowX: "hidden", overflowY: "hidden", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-1"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-1"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-1"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-1"] },
  mixseg: { position: "relative", minWidth: 0 },
  mixsegVbdMixsegFixed: { backgroundColor: "var(--vv-fixed-28)", backgroundImage: "none" },
  mixsegVbdMixsegChannel: { backgroundColor: "var(--vv-plat-twitch)", backgroundImage: "none" },
  mixsegVbdMixsegBilibili: { backgroundColor: "var(--vv-plat-bilibili)", backgroundImage: "none" },
  mixsegVbdMixsegLoop: { backgroundColor: "var(--vv-loop-bg)", backgroundImage: "none" },
  mixlab: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, display: "flex", flexDirection: "column", alignItems: "flex-start", justifyContent: "center", paddingTop: { default: 0, "@media (max-width: 700px)": 0 }, paddingRight: { default: "var(--vv-s-8)", "@media (max-width: 700px)": "var(--vv-s-4)" }, paddingBottom: { default: 0, "@media (max-width: 700px)": 0 }, paddingLeft: { default: "var(--vv-s-8)", "@media (max-width: 700px)": "var(--vv-s-4)" }, rowGap: "var(--vv-s-1)", columnGap: "var(--vv-s-1)", fontSize: { default: libraryVars.badgeFontSize, "@media (max-width: 700px)": "var(--vv-s-9)" }, fontWeight: libraryVars.headingWeight, color: color["--color-fg"], whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", pointerEvents: "none", maxWidth: libraryVars.fullExtent },
  mixlabVbdMixlab: { alignItems: "flex-end" },
  mixsub: { fontSize: "var(--vv-s-9)", fontWeight: libraryVars.regularWeight, color: color["--color-fg-2"] },
  mixlegend: { display: "flex", alignItems: "center", flexWrap: "wrap", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  mixleg: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-5)", columnGap: "var(--vv-s-5)", fontSize: libraryVars.microFontSize, color: color["--color-fg-3"] },
  vbdMixlegI: { width: "var(--vv-s-9)", height: "var(--vv-s-9)", borderTopLeftRadius: "var(--vv-r-3)", borderTopRightRadius: "var(--vv-r-3)", borderBottomRightRadius: "var(--vv-r-3)", borderBottomLeftRadius: "var(--vv-r-3)", fontStyle: "normal" },
  sec: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)", minWidth: 0 },
  secHead: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  vtSecHeadH2: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-16)", fontWeight: libraryVars.headingWeight },
  help: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-3"], lineHeight: libraryVars.helpLineHeight, maxWidth: libraryVars.templateBackdropSectionHelpMaxWidth },
  card: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
  cardHead: { display: "flex", alignItems: "baseline", justifyContent: "space-between", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  label: { fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg"] },
  vtCardHeadButton: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  srcs: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  srcempty: { textAlign: "center", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-10)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-10)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", alignItems: "center" },
  emptyTitle: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight, color: color["--color-fg"], marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 },
  cut: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-8)", columnGap: "var(--vv-s-8)" },
  cutline: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-12)", color: color["--color-fg-2"], borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, cursor: "pointer", fontFamily: "inherit", textAlign: "left", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  cutrows: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", flexWrap: "wrap" },
  radioline: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-12)", color: color["--color-fg"], cursor: "pointer", borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, fontFamily: "inherit", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  cutline2: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", fontSize: "var(--vv-s-12)", color: color["--color-fg-2"], borderTopWidth: 0, borderTopStyle: "none", borderTopColor: "currentColor", borderRightWidth: 0, borderRightStyle: "none", borderRightColor: "currentColor", borderBottomWidth: 0, borderBottomStyle: "none", borderBottomColor: "currentColor", borderLeftWidth: 0, borderLeftStyle: "none", borderLeftColor: "currentColor", backgroundColor: "transparent", backgroundImage: "none", paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, cursor: "pointer", fontFamily: "inherit", textAlign: "left" },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  cardPart: { borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], paddingTop: "var(--vv-s-14)", display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: color["--color-card"], backgroundImage: "none", minWidth: 0 },
  partHead: { display: "flex", alignItems: "center", justifyContent: "space-between" },
  adjust: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)" },
  flips: { display: "flex", rowGap: "var(--vv-s-24)", columnGap: "var(--vv-s-24)" },
  ovlist: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  ovrow: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", width: libraryVars.fullExtent, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", paddingTop: "var(--vv-s-6)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-6)", paddingLeft: "var(--vv-s-8)", backgroundColor: color["--color-bg"], backgroundImage: "none", fontFamily: "inherit", color: color["--color-fg"], textAlign: "left" },
  ovrowDataMissing: { borderTopColor: color["--color-err"], borderRightColor: color["--color-err"], borderBottomColor: color["--color-err"], borderLeftColor: color["--color-err"] },
  ovthumb: { width: "var(--vv-s-58)", height: "var(--vv-s-34)", borderTopLeftRadius: "var(--vv-r-6)", borderTopRightRadius: "var(--vv-r-6)", borderBottomRightRadius: "var(--vv-r-6)", borderBottomLeftRadius: "var(--vv-r-6)", overflowX: "hidden", overflowY: "hidden", flexShrink: 0, backgroundColor: line["--line-1"], backgroundImage: "none", display: "flex", alignItems: "center", justifyContent: "center" },
  vbdOvthumbVideo: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  vbdOvthumbImg: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  ovmain: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: libraryVars.flexFillBasis, minWidth: 0, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-2)", columnGap: "var(--vv-s-2)" },
  ovname: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.labelWeight, whiteSpace: "nowrap", overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis" },
  ovsub: { fontSize: "var(--vv-s-10)", color: color["--color-fg-3"] },
  ovrow2: { display: "flex", alignItems: "center", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)", width: libraryVars.fullExtent, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-2"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-2"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-2"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-2"], borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", paddingTop: "var(--vv-s-6)", paddingRight: "var(--vv-s-8)", paddingBottom: "var(--vv-s-6)", paddingLeft: "var(--vv-s-8)", backgroundColor: color["--color-bg"], backgroundImage: "none", fontFamily: "inherit", color: color["--color-fg"], textAlign: "left", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  dlgBackdrop: { position: "fixed", top: 0, right: 0, bottom: 0, left: 0, zIndex: "var(--vv-z-overlay)", backgroundColor: "var(--vv-scrim-60)", backgroundImage: "none", display: "flex", alignItems: "center", justifyContent: "center", paddingTop: "var(--vv-s-24)", paddingRight: "var(--vv-s-24)", paddingBottom: "var(--vv-s-24)", paddingLeft: "var(--vv-s-24)" },
})
