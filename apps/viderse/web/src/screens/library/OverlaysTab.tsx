// Tab Lớp phủ (Library.dc.html › tabs=overlays): lưới ảo hoá hiệu ứng có sẵn +
// tệp của bạn, thẻ xem trước bằng <video preload="metadata"> phát tại chỗ, ô
// thêm bằng kéo thả/chọn tệp (multipart /api/overlays), bỏ hiệu ứng = tombstone
// (template đang dùng vẫn giữ nó).
import { useRef, useState } from 'react'
import { Banner, Icon, Spinner, confirm } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { vi as t } from '../../i18n/vi'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { useVirtualGrid } from '../../lib/useVirtualGrid'
import { addOverlay, listOverlays, removeOverlay, type Overlay } from '../../api/library'
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
    <article className="vlb-otile">
      <button type="button" className="vlb-oart" onClick={toggle} aria-label={`${copy.play}: ${overlay.name}`}>
        <video ref={video} src={overlay.url} muted loop playsInline preload="metadata" tabIndex={-1}
          onEnded={() => setPlaying(false)} onPause={() => setPlaying(false)} />
        <span className="vlb-oart-veil" aria-hidden="true" />
        {!playing && <span className="vlb-oart-play" aria-hidden="true"><Icon name="play" size={16} /></span>}
        {overlay.durationSec !== null && <span className="vlb-odur mono num">{duration(overlay.durationSec)}</span>}
        {overlay.builtIn !== '' && <span className="vlb-chip vlb-chip--acc vlb-oart-badge">{copy.builtChip}</span>}
      </button>
      <div className="vlb-obody">
        <span className="vlb-oname">{overlay.name}</span>
        <span className="vlb-hint">{copy.background[overlay.background]}</span>
      </div>
      <div className="vlb-ofoot">
        <span className="vlb-hint">{copy.usedText(overlay.usedBy.map((ref) => ref.name))}</span>
        {overlay.builtIn === '' && (
          <button type="button" className="vlb-iconbtn" aria-label={copy.removeAria(overlay.name)} onClick={onRemove}>
            <Icon name="trash" size={14} />
          </button>
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
    <div className="vlb-scroll" ref={grid.ref} onScroll={grid.onScroll}>
      <div className="vlb-overlay-head">
        <div>
          <h2 className="vlb-subtitle">{copy.builtTitle}</h2>
          <p className="vlb-hint">{copy.note}</p>
        </div>
      </div>
      {error && <Banner tone="error" title={error} />}
      {items.length === 0 && !overlays.initialLoading && (
        <div className="vlb-empty">
          <p className="vlb-empty-title">{copy.emptyTitle}</p>
          <p className="vlb-hint">{copy.emptyHint}</p>
        </div>
      )}
      <div className="vlb-grid vlb-grid--overlays" style={{ paddingTop: grid.padTop, paddingBottom: grid.padBottom }}>
        {items.slice(grid.itemStart, grid.itemEnd).map((overlay) => (
          <OverlayTile key={overlay.id} overlay={overlay} onRemove={() => void remove(overlay)} />
        ))}
        {grid.itemEnd >= items.length + 1 && (
          <button
            type="button"
            className="vlb-newtile vlb-newtile--drop"
            data-dragging={dragging || undefined}
            onClick={() => input.current?.click()}
            onDragOver={(event) => { event.preventDefault(); setDragging(true) }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => { event.preventDefault(); setDragging(false); void upload(event.dataTransfer.files[0]) }}
          >
            <span className="vlb-newtile-icon" aria-hidden="true">{busy ? <Spinner size={16} /> : <Icon name="add" size={18} />}</span>
            <span className="vlb-newtile-title">{busy ? copy.addBusy : copy.addTitle}</span>
            <span className="vlb-hint">{copy.addHint}</span>
            <span className="vlb-hint">{copy.addFormats}</span>
          </button>
        )}
      </div>
      <input ref={input} type="file" accept={ACCEPT} hidden onChange={(event) => void upload(event.currentTarget.files?.[0])} />
      <p className="vlb-hint vlb-foot-note">{copy.footer}</p>
    </div>
  )
}
