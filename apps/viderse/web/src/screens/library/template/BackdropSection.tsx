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
import '../backdrops/backdrops.css'
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
    <div className="vbd-srcrow" data-missing={channel ? undefined : ''}>
      {channel?.avatarUrl
        ? <img className="vbd-av" src={channel.avatarUrl} alt="" width={30} height={30} style={{ borderRadius: '50%' }} />
        : <span className="vbd-av" aria-hidden="true" style={{ width: 30, height: 30, background: channelTint(link.channelId) }}>{initials(name)}</span>}
      <span className="vbd-srcmain">
        <span className="vbd-srchead">
          <span className="vbd-srcname">{name}</span>
          <span className="vbd-tag">{pick ? copy.howPick(num(link.pinned.length)) : copy.tagRandom}</span>
        </span>
        <span className="vbd-srcmeta">
          {channel && platformName(channel.platform)}{channel ? ' · ' : ''}
          {pick ? copy.howPick(num(link.pinned.length)) : copy.howRandom} · {kinds}
        </span>
      </span>
      <Button size="sm" icon={<Icon name="edit" size={13} />} onClick={onEdit}>{copy.chooseVideos}</Button>
      <Button variant="icon" icon={<Icon name="close" size={15} />} aria-label={copy.removeAria(name)} onClick={onRemove} />
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
    <div className="vbd-srcrow" data-missing={missing ? '' : undefined}>
      {video?.state === 'ready' && video.file
        ? <span className="vbd-av vbd-av--rect" style={{ width: 54, height: 30 }}>
            <video src={backdropVideoFileHref(video.id)} muted playsInline preload="metadata" />
          </span>
        : <span className="vbd-av vbd-av--rect" style={{ width: 54, height: 30 }}><Icon name="video-play" size={14} /></span>}
      <span className="vbd-srcmain">
        <span className="vbd-srcname">{name}</span>
        <span className="vbd-srcmeta">
          {video && copy.videoSub(platformName(video.platform), segmentLabel(video.segment.fromSec, video.segment.toSec))}
        </span>
      </span>
      <span className="vbd-tag" data-tone="fixed">{copy.tagFixed}</span>
      <Button variant="icon" icon={<Icon name="close" size={15} />} aria-label={copy.removeAria(name)} onClick={onRemove} />
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
    return <p className="vbd-hint">{copy.mixNoteNone}</p>
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
    <div className="vbd-mix">
      <div className="vbd-mixbar" role="img" aria-label={aria}>
        {segs.map((seg) => (
          <div key={seg.id}
            className={`vbd-mixseg ${seg.tone === 'fixed' ? 'vbd-mixseg--fixed' : seg.tone === 'loop' ? 'vbd-mixseg--loop' : `vbd-mixseg--channel ${seg.platform === 'bilibili' ? 'vbd-mixseg--bilibili' : ''}`}`}
            style={{ flexGrow: seg.sec }}>
            <span className="vbd-mixlab"><span>{seg.label}</span><span className="vbd-mixsub">{seg.sub}</span></span>
          </div>
        ))}
      </div>
      <div className="vbd-mixlegend">
        {mix.fixedSec > 0 && (
          <span className="vbd-mixleg"><i style={{ background: 'var(--vv-fixed-28)' }} />{copy.mixFixedDetail(num(fixedCount), timeRange(mix.fixedSec))}</span>
        )}
        {mix.perChannel.map((c) => {
          const channel = channelById.get(c.channelId)
          return (
            <span key={c.channelId} className="vbd-mixleg">
              <i style={{ background: `var(--vv-plat-${channel?.platform === 'bilibili' ? 'bilibili' : 'twitch'})` }} />
              {channel?.title ?? channel?.handle ?? c.channelId}{' '}
              {pinnedOf(c.channelId) > 0
                ? copy.mixPickDetail(num(c.clips), num(pinnedOf(c.channelId)))
                : c.repeats ? copy.mixRepeatDetail(num(c.clips), num(c.freshVideos)) : copy.mixRandomDetail(num(c.clips))}
            </span>
          )
        })}
        {mix.loopSec > 0 && (
          <span className="vbd-mixleg"><i style={{ background: 'var(--vv-loop-bg)' }} />{copy.mixLoopDetail(timeRange(mix.loopSec))}</span>
        )}
      </div>
      <p className="vbd-hint">
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
    <section className="vt-sec" aria-labelledby="vt-backdrop-title" aria-busy={saving || undefined}>
      <div className="vt-sec-head">
        <h2 id="vt-backdrop-title">{t.library.template.sections.backdrop.label}</h2>
        <p className="vt-help">{copy.help}</p>
      </div>

      {/* Lấy nền từ */}
      <div className="vt-card">
        <div className="vt-card-head">
          <span className="vt-label">{copy.fromLabel}</span>
          <Button size="sm" variant="primary" onClick={() => setPicker({})}>{copy.attach}</Button>
        </div>
        <div className="vbd-srcs" role="list" aria-label={copy.sourceListAria}>
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
            <div className="vbd-srcempty">
              <span className="vbd-empty-title">{copy.emptyTitle}</span>
              <span className="vbd-hint">{copy.emptyHint}</span>
            </div>
          )}
        </div>
        {missing && <Banner tone="error" title={copy.missingTitle}>{copy.missingBody}</Banner>}
        <p className="vbd-hint"><Icon name="info" size={13} />{copy.fromHelp}</p>
      </div>

      {/* Nền của một video + Cách cắt clip */}
      {(backdrop.channels.length > 0 || backdrop.fixedVideoIds.length > 0) && (
        <div className="vt-card">
          <span className="vt-label">{copy.mixLabel} · {copy.mixExample}</span>
          <MixBar mix={mix.data} backdrop={backdrop} channels={channels.data ?? []}
            onBumpClipSec={(sec) => saveBackdrop({ clipSec: sec })} />

          <div className="vbd-cut">
            <button type="button" className="vbd-cutline" aria-expanded={cutOpen}
              onClick={() => setCutOpen((open) => !open)}>
              <Icon name={cutOpen ? 'arrow-d' : 'arrow-r'} size={12} />
              {copy.cutToggle} — {manual ? copy.cutSumManual(num(backdrop.clipCount ?? 1), num(backdrop.clipSec)) : copy.cutSumAuto(num(backdrop.clipSec))}
            </button>
            {cutOpen && (
              <>
                <div role="radiogroup" aria-label={copy.modeAria} className="vbd-cutrows">
                  <button type="button" role="radio" aria-checked={!manual} className="vbd-radioline"
                    onClick={() => saveBackdrop({ clipCount: null })}>
                    <RadioDot checked={!manual} />{copy.modeAuto}
                  </button>
                  <button type="button" role="radio" aria-checked={manual} className="vbd-radioline"
                    onClick={() => saveBackdrop({ clipCount: Math.min(CLIP_MAX_COUNT, Math.max(1, backdrop.clipCount ?? 9)) })}>
                    <RadioDot checked={manual} />{copy.modeManual}
                  </button>
                  {manual && (
                    <span className="vbd-cutline">
                      <NumberStepper
                        value={backdrop.clipCount ?? 1} min={1} max={CLIP_MAX_COUNT} step={1}
                        aria-label={copy.clipCountAria}
                        onChange={(value) => saveBackdrop({ clipCount: value })} />
                      {copy.clipCountPost}
                    </span>
                  )}
                  <span className="vbd-cutline">
                    {copy.clipSecPre}
                    <NumberStepper
                      value={backdrop.clipSec} min={CLIP_MIN_SEC} max={CLIP_MAX_SEC} step={5}
                      aria-label={copy.clipSecAria}
                      onChange={(value) => saveBackdrop({ clipSec: value })} />
                    {copy.clipSecPost}
                  </span>
                </div>
                <p className="vbd-hint">{copy.cutHelp}</p>
              </>
            )}
          </div>
        </div>
      )}

      {/* Chỉnh nền */}
      <div className="vt-card vbd-part">
        <div className="vbd-part-head">
          <span className="vt-label">{copy.adjustTitle}</span>
          <Button size="sm" variant="quiet" onClick={() => void onSave({ appearance: { ...APPEARANCE_DEFAULT } })}>{copy.adjustReset}</Button>
        </div>
        <div className="vbd-adjust">
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
        <div className="vbd-flips">
          <Switch label={copy.flipX} checked={appearance.flipX} onChange={(on) => saveAppearance({ flipX: on })} />
          <Switch label={copy.flipY} checked={appearance.flipY} onChange={(on) => saveAppearance({ flipY: on })} />
        </div>
      </div>

      {/* Lớp hiệu ứng */}
      <div className="vt-card vbd-part">
        <div className="vbd-part-head">
          <span className="vt-label">{copy.overlayTitle}</span>
          <Button size="sm" variant="quiet" disabled={overlayCandidates.length === 0}
            onClick={() => setOverlayPick(true)}>{copy.overlayAdd}</Button>
        </div>
        {attachedOverlays.length === 0 ? (
          <div className="vbd-srcempty">
            <span className="vbd-empty-title">{copy.overlayEmptyTitle}</span>
            <span className="vbd-hint">{copy.overlayEmptyHint}</span>
          </div>
        ) : (
          <div className="vbd-ovlist">
            {attachedOverlays.map((id) => {
              const overlay = overlayById.get(id)
              return (
                <div key={id} className="vbd-ovrow" data-missing={overlay ? undefined : ''}>
                  <span className="vbd-ovthumb">
                    {overlay?.mime.startsWith('video/')
                      ? <video src={overlay.url} muted playsInline preload="metadata" />
                      : overlay?.mime.startsWith('image/') ? <img src={overlay.url} alt="" /> : <Icon name="layer" size={14} />}
                  </span>
                  <span className="vbd-ovmain">
                    <span className="vbd-ovname">{overlay?.name ?? id}</span>
                    <span className="vbd-ovsub">
                      {overlay ? (overlay.durationSec !== null ? copy.overlayDuration(duration(overlay.durationSec)) : copy.overlayBuiltin) : copy.missingTitle}
                    </span>
                  </span>
                  <Button variant="icon" icon={<Icon name="close" size={15} />}
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
          <div className="vbd-ovlist">
            {overlayCandidates.length === 0 && <p className="vbd-hint">{copy.overlayPickEmpty}</p>}
            {overlayCandidates.map((overlay) => (
              <button key={overlay.id} type="button" className="vbd-ovrow" style={{ cursor: 'pointer' }}
                onClick={() => { saveOverlays([...attachedOverlays, overlay.id]); setOverlayPick(false) }}>
                <span className="vbd-ovthumb">
                  {overlay.mime.startsWith('video/')
                    ? <video src={overlay.url} muted playsInline preload="metadata" />
                    : overlay.mime.startsWith('image/') ? <img src={overlay.url} alt="" /> : <Icon name="layer" size={14} />}
                </span>
                <span className="vbd-ovmain">
                  <span className="vbd-ovname">{overlay.name}</span>
                  <span className="vbd-ovsub">{overlay.durationSec !== null ? copy.overlayDuration(duration(overlay.durationSec)) : copy.overlayBuiltin}</span>
                </span>
              </button>
            ))}
          </div>
        </Dialog>
      )}

      {/* Hộp gắn kênh/video */}
      {picker && (
        <div className="vbd-dlg-backdrop" onClick={(event) => { if (event.target === event.currentTarget) setPicker(null) }}>
          <BackdropPicker template={template} onSave={onSave} onClose={() => setPicker(null)} initial={picker.edit} key={picker.edit ?? 'new'} />
        </div>
      )}
    </section>
  )
}
