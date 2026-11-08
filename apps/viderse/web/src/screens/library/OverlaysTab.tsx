import * as stylex from '@stylexjs/stylex'
import { color, line } from '@noverse/ui/tokens.stylex'
import { libraryVars } from './libraryTokens.stylex'
// Tab Lớp phủ (Library.dc.html › tabs=overlays): lưới ảo hoá hiệu ứng có sẵn +
// tệp của bạn, thẻ xem trước bằng <video preload="metadata"> phát tại chỗ, ô
// thêm bằng kéo thả/chọn tệp (multipart /api/overlays), bỏ hiệu ứng = tombstone
// (template đang dùng vẫn giữ nó).
import { useRef, useState } from 'react'
import { Banner, Button, Icon, Spinner, confirm } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { t } from '../../i18n/library'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { useVirtualGrid } from '../../lib/useVirtualGrid'
import { addOverlay, listOverlays, removeOverlay, type Overlay } from '../../api/library'
import { uploadTooLarge } from '../../api/upload-limits'
import { duration } from './fmt'

const ACCEPT = '.mp4,.m4v,.mov,.webm,.png,.jpg,.jpeg,.webp'

function OverlayTile({ overlay, onRemove }: { overlay: Overlay; onRemove: () => void }) {
  const copy = t.library.overlays
  const video = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const toggle = () => {
    const node = video.current
    if (!node) return
    if (node.paused) { void node.play().catch(() => undefined); setPlaying(true) }
    else { node.pause(); setPlaying(false) }
  }
  return (
    <article className={`vlb-otile ${stylex.props(sx.otile).className}`}>
      <button type="button" className={`vlb-oart ${stylex.props(sx.oart).className}`} onClick={toggle} aria-label={`${copy.play}: ${overlay.name}`}>
        <video {...stylex.props(sx.vlbOartVideo)} ref={video} src={overlay.url} muted loop playsInline preload="metadata" tabIndex={-1}
          onEnded={() => setPlaying(false)} onPause={() => setPlaying(false)} />
        <span className={`vlb-oart-veil ${stylex.props(sx.oartVeil).className}`} aria-hidden="true" />
        {!playing && <span className={`vlb-oart-play ${stylex.props(sx.oartPlay).className}`} aria-hidden="true"><Icon name="play" size={16} /></span>}
        {overlay.durationSec !== null && <span className={`vlb-odur num ${stylex.props(sx.odurNum).className}`}>{duration(overlay.durationSec)}</span>}
        {overlay.builtIn !== '' && <span className={`vlb-chip vlb-chip--acc vlb-oart-badge ${stylex.props(sx.chipChipAccOartBadge).className}`}>{copy.builtChip}</span>}
      </button>
      <div className={`vlb-obody ${stylex.props(sx.obody).className}`}>
        <span className={`vlb-oname ${stylex.props(sx.oname).className}`}>{overlay.name}</span>
        <span className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.background[overlay.background]}</span>
      </div>
      <div className={`vlb-ofoot ${stylex.props(sx.ofoot).className}`}>
        <span className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.usedText(overlay.usedBy.map((ref) => ref.name))}</span>
        {overlay.builtIn === '' && (
          <Button variant="icon" size="sm" className="vlb-iconbtn" xstyle={sx.iconbtn} aria-label={copy.removeAria(overlay.name)} onClick={onRemove} icon={<Icon name="trash" size={14} />} />
        )}
      </div>
    </article>
  )
}

export function OverlaysTab() {
  const copy = t.library.overlays
  const overlays = useResource(keys.overlays(), ({ signal }) => listOverlays(signal))
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [dragging, setDragging] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const items = overlays.data?.items ?? []
  const grid = useVirtualGrid({ count: items.length + 1, metrics: { cardMinW: 190, cardExtraH: 78, gapX: 12, gapY: 14 } })

  const upload = async (file: File | undefined) => {
    if (!file) return
    setError(null)
    // Qua internet, tệp vượt trần bị edge chặn bằng trang 413 HTML: tự chặn
    // trước và nói rõ trần (cùng câu với lỗi của máy chủ).
    const tooLarge = await uploadTooLarge('overlay', file.size)
    if (tooLarge) {
      setError(tooLarge)
      if (input.current) input.current.value = ''
      return
    }
    setBusy(true)
    try {
      await addOverlay(file)
      await invalidate(keys.overlays())
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
      if (input.current) input.current.value = ''
    }
  }

  const remove = async (overlay: Overlay) => {
    const accepted = await confirm({
      title: copy.removeConfirmTitle(overlay.name),
      body: copy.removeConfirmBody,
      tone: 'danger',
      confirmLabel: t.library.delete,
      cancelLabel: t.library.cancel,
      onConfirm: () => undefined,
    })
    if (!accepted) return
    try {
      await removeOverlay(overlay.id)
      await invalidate(keys.overlays())
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className={`vlb-scroll ${stylex.props(sx.scroll).className}`} ref={grid.ref} onScroll={grid.onScroll}>
      <div className={`vlb-overlay-head ${stylex.props(sx.overlayHead).className}`}>
        <div>
          <h2 className={`vlb-subtitle ${stylex.props(sx.subtitle).className}`}>{copy.builtTitle}</h2>
          <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.note}</p>
        </div>
      </div>
      {error && <Banner tone="error" title={error} />}
      {items.length === 0 && !overlays.initialLoading && (
        <div className={`vlb-empty ${stylex.props(sx.empty).className}`}>
          <p className={`vlb-empty-title ${stylex.props(sx.emptyTitle).className}`}>{copy.emptyTitle}</p>
          <p className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.emptyHint}</p>
        </div>
      )}
      <div className={`vlb-grid vlb-grid--overlays ${stylex.props(sx.gridGridOverlays).className}`} style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
        {items.slice(grid.itemStart, grid.itemEnd).map((overlay) => (
          <OverlayTile key={overlay.id} overlay={overlay} onRemove={() => void remove(overlay)} />
        ))}
        {grid.itemEnd >= items.length + 1 && (
          <button
            type="button"
            className={`vlb-newtile vlb-newtile--drop ${stylex.props(sx.newtileNewtileDrop, (Boolean((dragging))) && sx.newtileNewtileDropDataDragging).className}`}
            data-dragging={dragging || undefined}
            onClick={() => input.current?.click()}
            onDragOver={(event) => { event.preventDefault(); setDragging(true) }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => { event.preventDefault(); setDragging(false); void upload(event.dataTransfer.files[0]) }}
          >
            <span className={`vlb-newtile-icon ${stylex.props(sx.newtileIcon).className}`} aria-hidden="true">{busy ? <Spinner size={16} /> : <Icon name="add" size={18} />}</span>
            <span className={`vlb-newtile-title ${stylex.props(sx.newtileTitle).className}`}>{busy ? copy.addBusy : copy.addTitle}</span>
            <span className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.addHint}</span>
            <span className={`vlb-hint ${stylex.props(sx.hint).className}`}>{copy.addFormats}</span>
          </button>
        )}
      </div>
      <input ref={input} type="file" accept={ACCEPT} hidden onChange={(event) => void upload(event.currentTarget.files?.[0])} />
      <p className={`vlb-hint vlb-foot-note ${stylex.props(sx.hintFootNote).className}`}>{copy.footer}</p>
    </div>
  )
}

const sx = stylex.create({
  otile: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", minWidth: 0 },
  oart: { position: "relative", display: "block", width: libraryVars.fullExtent, aspectRatio: libraryVars.videoAspectRatio, paddingTop: 0, paddingRight: 0, paddingBottom: 0, paddingLeft: 0, borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: { default: line["--line-2"], ":hover": line["--line-4"] }, borderTopLeftRadius: "var(--vv-r-10)", borderTopRightRadius: "var(--vv-r-10)", borderBottomRightRadius: "var(--vv-r-10)", borderBottomLeftRadius: "var(--vv-r-10)", overflowX: "hidden", overflowY: "hidden", backgroundColor: color["--color-surface-1"], backgroundImage: "none", cursor: "pointer", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  vlbOartVideo: { width: libraryVars.fullExtent, height: libraryVars.fullExtent, objectFit: "cover", display: "block" },
  oartVeil: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, backgroundColor: "transparent", backgroundImage: libraryVars.overlaysTabOartVeilBackgroundImage },
  oartPlay: { position: "absolute", left: "50%", top: "50%", transform: libraryVars.overlaysTabOartPlayTransform, display: "inline-flex", alignItems: "center", justifyContent: "center", width: "var(--vv-s-28)", height: "var(--vv-s-28)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", backgroundColor: "var(--vv-glass)", backgroundImage: "none", color: color["--color-fg"], backdropFilter: libraryVars.overlaysTabOartPlayBackdropFilter },
  odurNum: { position: "absolute", right: "var(--vv-s-6)", bottom: "var(--vv-s-6)", paddingTop: 0, paddingRight: "var(--vv-s-5)", paddingBottom: 0, paddingLeft: "var(--vv-s-5)", borderTopLeftRadius: "var(--vv-r-4)", borderTopRightRadius: "var(--vv-r-4)", borderBottomRightRadius: "var(--vv-r-4)", borderBottomLeftRadius: "var(--vv-r-4)", backgroundColor: "var(--vv-glass)", backgroundImage: "none", color: color["--color-fg-2"], fontSize: libraryVars.microFontSize, lineHeight: "var(--vv-s-17)" },
  chipChipAccOartBadge: { display: "inline-flex", alignItems: "center", rowGap: "var(--vv-s-4)", columnGap: "var(--vv-s-4)", paddingTop: "var(--vv-s-2)", paddingRight: "var(--vv-s-7)", paddingBottom: "var(--vv-s-2)", paddingLeft: "var(--vv-s-7)", borderTopLeftRadius: "var(--vv-r-pill)", borderTopRightRadius: "var(--vv-r-pill)", borderBottomRightRadius: "var(--vv-r-pill)", borderBottomLeftRadius: "var(--vv-r-pill)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "var(--vv-accent-45)", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "var(--vv-accent-45)", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "var(--vv-accent-45)", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "var(--vv-accent-45)", color: color["--color-accent"], fontSize: "var(--vv-s-11)", whiteSpace: "nowrap", position: "absolute", left: "var(--vv-s-6)", top: "var(--vv-s-6)", backgroundColor: color["--color-bg"], backgroundImage: "none" },
  obody: { display: "flex", alignItems: "baseline", justifyContent: "space-between", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", minWidth: 0, minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  oname: { minWidth: 0, overflowX: "hidden", overflowY: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontSize: libraryVars.bodySmallFontSize, color: color["--color-fg-2"] },
  hint: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight },
  ofoot: { display: "flex", alignItems: "center", justifyContent: "space-between", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)" },
  iconbtn: { display: "inline-flex", alignItems: "center", justifyContent: "center", width: "var(--vv-s-26)", height: "var(--vv-s-26)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: "transparent", borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: "transparent", borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: "transparent", borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: "transparent", borderTopLeftRadius: "var(--vv-r-7)", borderTopRightRadius: "var(--vv-r-7)", borderBottomRightRadius: "var(--vv-r-7)", borderBottomLeftRadius: "var(--vv-r-7)", backgroundColor: { default: "transparent", ":hover": "var(--vv-hover)" }, backgroundImage: { default: "none", ":hover": "none" }, color: { default: color["--color-fg-4"], ":hover": color["--color-fg"] }, cursor: "pointer", minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, minWidth: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, outlineWidth: { default: null, ":focus-visible": "var(--vv-s-2)" }, outlineStyle: { default: null, ":focus-visible": "solid" }, outlineColor: { default: null, ":focus-visible": color["--color-accent"] }, outlineOffset: { default: null, ":focus-visible": "var(--vv-s-1)" } },
  scroll: { flexGrow: libraryVars.fillRatio, flexShrink: libraryVars.fillRatio, flexBasis: "auto", minHeight: 0, minWidth: 0, overflowY: "auto", overscrollBehavior: "contain", paddingTop: { default: "var(--vv-s-4)", "@media (max-width: 767px)": "var(--vv-s-4)" }, paddingRight: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, paddingBottom: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-24)" }, paddingLeft: { default: "var(--vv-s-28)", "@media (max-width: 767px)": "var(--vv-s-16)" }, display: "flex", flexDirection: "column", rowGap: "var(--vv-s-12)", columnGap: "var(--vv-s-12)" },
  overlayHead: { display: "flex", alignItems: "flex-end", justifyContent: "space-between", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  subtitle: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-15)", fontWeight: libraryVars.labelWeight },
  empty: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-28)", paddingRight: 0, paddingBottom: "var(--vv-s-28)", paddingLeft: 0 },
  emptyTitle: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-14)", fontWeight: libraryVars.labelWeight, color: color["--color-fg-2"] },
  gridGridOverlays: { display: "grid", rowGap: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-12)" }, columnGap: { default: "var(--vv-s-12)", "@media (max-width: 767px)": "var(--vv-s-12)" }, gridTemplateColumns: { default: libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns2, "@media (max-width: 767px)": libraryVars.overlaysTabGridGridOverlaysGridTemplateColumns3 }, alignContent: "start" },
  newtileNewtileDrop: { minHeight: { default: libraryVars.overlaysTabNewtileNewtileDropMinHeight2, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" }, display: "flex", flexDirection: "column", alignItems: "flex-start", rowGap: "var(--vv-s-6)", columnGap: "var(--vv-s-6)", paddingTop: "var(--vv-s-14)", paddingRight: "var(--vv-s-14)", paddingBottom: "var(--vv-s-14)", paddingLeft: "var(--vv-s-14)", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "dashed", borderTopColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderRightWidth: "var(--vv-s-1)", borderRightStyle: "dashed", borderRightColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "dashed", borderBottomColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "dashed", borderLeftColor: { default: line["--line-3"], ":hover": color["--color-accent"] }, borderTopLeftRadius: "var(--vv-r-12)", borderTopRightRadius: "var(--vv-r-12)", borderBottomRightRadius: "var(--vv-r-12)", borderBottomLeftRadius: "var(--vv-r-12)", backgroundColor: { default: "transparent", ":hover": "var(--vv-accent-6)" }, backgroundImage: { default: "none", ":hover": "none" }, color: color["--color-fg-2"], cursor: "pointer", textAlign: "left" },
  newtileNewtileDropDataDragging: { borderTopColor: color["--color-accent"], borderRightColor: color["--color-accent"], borderBottomColor: color["--color-accent"], borderLeftColor: color["--color-accent"], backgroundColor: "var(--vv-accent-6)", backgroundImage: "none" },
  newtileIcon: { display: "inline-flex", alignItems: "center", justifyContent: "center", width: "var(--vv-s-30)", height: "var(--vv-s-30)", borderTopLeftRadius: "50%", borderTopRightRadius: "50%", borderBottomRightRadius: "50%", borderBottomLeftRadius: "50%", borderTopWidth: "var(--vv-s-1)", borderTopStyle: "solid", borderTopColor: line["--line-3"], borderRightWidth: "var(--vv-s-1)", borderRightStyle: "solid", borderRightColor: line["--line-3"], borderBottomWidth: "var(--vv-s-1)", borderBottomStyle: "solid", borderBottomColor: line["--line-3"], borderLeftWidth: "var(--vv-s-1)", borderLeftStyle: "solid", borderLeftColor: line["--line-3"], color: color["--color-fg-3"] },
  newtileTitle: { fontSize: libraryVars.controlFontSize, fontWeight: libraryVars.labelWeight },
  hintFootNote: { marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0, fontSize: "var(--vv-s-12)", color: color["--color-fg-4"], lineHeight: libraryVars.bodyLineHeight, paddingTop: "var(--vv-s-4)" },
})
