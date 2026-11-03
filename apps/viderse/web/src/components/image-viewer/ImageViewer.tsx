// Trình xem ảnh dùng chung (04-components › Trình xem ảnh, đợt 40).
//
// Bốn chế độ cùng một khung: Duyệt, Xem, Xem + Thay, Mẫu. Nội dung cột phải
// (phiên bản, prompt, góp ý, thao tác) do màn gọi truyền vào qua `side`/`footer`
// — viewer chỉ giữ phần khung, dải ảnh và phím ← → để mọi màn dùng đúng một
// cách chọn ảnh theo ID ổn định.
//
// Modal (mặc định) dùng lớp phủ của kit: `Dialog` lo scrim, Esc, focus trap và
// trả focus về trigger. `inline` dùng cho màn Duyệt đã có bố cục ba cột: root
// `display: contents`, cột giữa nhận nguyên `stage`, cột phải nhận nguyên
// `side` (ImageRail). Nhánh inline không tính lại mô hình chọn ảnh của màn.
import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Button, Dialog, Icon } from '@noverse/ui'
import { sx } from './image-viewer.stylex'
import type { ImageViewerItem, ImageViewerMode, ImageViewerProps, ModalImageViewerProps } from './types'
import { imageViewer as layout, space } from '../../styles/viderse.stylex'

const FRAME_STYLE: CSSProperties = {
  display: 'grid',
  gridTemplateRows: `${layout.header} minmax(0, 1fr) ${layout.footer}`,
  height: `min(${layout.height}, calc(100dvh - ${space['--vv-s-48']}))`,
  maxHeight: `min(${layout.height}, calc(100dvh - ${space['--vv-s-48']}))`,
  overflow: 'hidden',
}

export type { ImageViewerItem, ImageViewerMode, ImageViewerProps } from './types'

const MODE_LABEL: Record<ImageViewerMode, string> = {
  review: 'Duyệt',
  view: 'Đang làm · xem',
  'view-replace': 'Xem + thay ảnh',
  sample: 'Mẫu',
}

/** Ảnh có thể mở: có URL và không phải ô chờ/đang vẽ. */
export const isViewable = (item: ImageViewerItem) => !item.disabled && !!item.url

export function ImageViewer(props: ImageViewerProps) {
  if (props.inline) return <div {...stylex.props(sx.inlineRoot)}>{props.stage}{props.side}</div>
  return <ModalImageViewer {...props} />
}

function ModalImageViewer({
  mode, title, subtitle, modeLabel: suppliedModeLabel, stripLabel, items, selectedId, onSelect, side, footer, onClose,
}: ModalImageViewerProps) {
  const currentIndex = items.findIndex(item => item.id === selectedId)
  const current = currentIndex >= 0 ? items[currentIndex] : undefined
  const viewableIndexes = useMemo(() => {
    const indexes: number[] = []
    for (let index = 0; index < items.length; index += 1) if (isViewable(items[index]!)) indexes.push(index)
    return indexes
  }, [items])
  const position = viewableIndexes.indexOf(currentIndex)
  const move = (delta: 1 | -1) => {
    if (items.length < 2) return
    const from = currentIndex >= 0 ? currentIndex : 0
    for (let step = 1; step <= items.length; step += 1) {
      const index = (from + delta * step + items.length) % items.length
      const candidate = items[index]
      if (candidate && isViewable(candidate)) { onSelect(candidate.id); return }
    }
  }
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey) return
      const node = event.target as HTMLElement | null
      if (node?.isContentEditable || (node && ['INPUT', 'TEXTAREA', 'SELECT'].includes(node.tagName))) return
      if (event.key === 'ArrowLeft') { event.preventDefault(); move(-1) }
      else if (event.key === 'ArrowRight') { event.preventDefault(); move(1) }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  })
  const positionLabel = currentIndex < 0
    ? 'Ảnh không còn trong bộ ảnh'
    : position >= 0
      ? `${position + 1} / ${viewableIndexes.length} đã vẽ`
      : `Bản chưa vẽ xong · ${currentIndex + 1} / ${items.length}`
  const label = current?.label ?? title
  const sample = mode === 'sample'
  return <Dialog label={`Xem ảnh ${label}`} onClose={onClose} width={layout.width} padBody={false} style={FRAME_STYLE} scrimStyle={sx.scrim}>
    <header {...stylex.props(sx.top)}>
      <ViewerHead modeLabel={suppliedModeLabel ?? MODE_LABEL[mode]} title={label} position={positionLabel} sub={subtitle ?? (title !== label ? title : undefined)} sample={sample} />
      <Button variant="icon" size="sm" aria-label="Đóng (Esc)" onClick={onClose}><Icon name="close" size={16} /></Button>
    </header>
    <div {...stylex.props(sx.body, !side && sx.bodySolo)}>
      <div {...stylex.props(sx.stageCol)}>
        <ImageStage item={current} title={title} items={items} move={move} sample={sample} />
        <ImageStrip items={items} selectedId={selectedId} onSelect={onSelect} label={stripLabel ?? (sample ? 'Prompt cùng loại' : 'Ảnh của video')} sample={sample} />
      </div>
      {side && <div {...stylex.props(sx.side)}>{side}</div>}
    </div>
    <footer {...stylex.props(sx.footer)}><div {...stylex.props(sx.footerRow)}>{footer ?? '← → xem ảnh khác · Esc đóng'}</div></footer>
  </Dialog>
}

/** Dòng đầu của modal: nhãn ảnh + vị trí, tên video làm dòng phụ. */
function ViewerHead({ title, position, sub, modeLabel, sample }: { title: string; position: string; sub?: string; modeLabel: string; sample: boolean }) {
  return <span {...stylex.props(sx.head)}>
    <Icon name="gallery" size={16} xstyle={sx.headIcon} />
    <span {...stylex.props(sx.headTitle)}>{title}</span>
    <span {...stylex.props(sx.headPos)}>· {position}</span>
    {sub && <span {...stylex.props(sx.headSub)} title={sub}>{sub}</span>}
    <span {...stylex.props(sx.headSpacer)} />
    <span {...stylex.props(sx.tag, sample && sx.tagSample)}>{modeLabel}</span>
  </span>
}

function ImageStage({ item, title, items, move, sample }: {
  item: ImageViewerItem | undefined
  title: string
  items: readonly ImageViewerItem[]
  move: (delta: 1 | -1) => void
  sample: boolean
}) {
  const [failed, setFailed] = useState(false)
  const [nonce, setNonce] = useState(0)
  const url = item?.url ?? undefined
  useEffect(() => { setFailed(false) }, [item?.id, url, nonce])
  const hasOther = items.some(candidate => candidate.id !== item?.id && isViewable(candidate))
  const label = item?.label ?? title
  return <div {...stylex.props(sx.stageBox)}>
    {item && url && !failed
      ? <img
          key={`${item.id}:${url}:${nonce}`}
          src={url}
          alt={label}
          onError={() => setFailed(true)}
          {...stylex.props(sx.image)}
        />
      : <span role={failed ? 'alert' : 'status'} {...stylex.props(sx.stageNotice)}>
          {failed
            ? <><span>Chưa tải được ảnh.</span><Button size="sm" onClick={() => setNonce(value => value + 1)}>Thử lại</Button></>
            : item
              ? 'Ảnh này chưa vẽ xong.'
              : 'Ảnh đã chọn không còn trong bộ ảnh.'}
        </span>}
    {item && hasOther && <>
      <Button variant="icon" size="sm" aria-label={`${sample ? 'Mẫu' : 'Ảnh'} trước (←)`} tooltip={`${sample ? 'Mẫu' : 'Ảnh'} trước (←)`} onClick={() => move(-1)} xstyle={[sx.nav, sx.prev]}>
        <Icon name="arrow-r" size={16} xstyle={sx.prevIcon} />
      </Button>
      <Button variant="icon" size="sm" aria-label={`${sample ? 'Mẫu' : 'Ảnh'} sau (→)`} tooltip={`${sample ? 'Mẫu' : 'Ảnh'} sau (→)`} onClick={() => move(1)} xstyle={[sx.nav, sx.next]}>
        <Icon name="arrow-r" size={16} />
      </Button>
    </>}
  </div>
}

function ImageStrip({ items, selectedId, onSelect, label, sample }: { items: readonly ImageViewerItem[]; selectedId: string; onSelect: (id: string) => void; label: string; sample: boolean }) {
  const strip = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const list = strip.current
    if (!list) return
    const active = document.activeElement
    const current = list.querySelector<HTMLElement>('[aria-selected="true"]')
    current?.scrollIntoView?.({ block: 'nearest', inline: 'nearest' })
    // Nếu người dùng đang đi trong dải ảnh, focus đi theo ảnh mới; click chuột
    // ở nơi khác thì không bị cướp focus.
    if (active instanceof HTMLElement && list.contains(active)) current?.focus({ preventScroll: true })
  }, [selectedId, items.length])
  return <div role="listbox" aria-label={label} ref={strip} {...stylex.props(sx.strip)}>
    {items.map((item, index) => {
      const on = item.id === selectedId
      const available = isViewable(item)
      const badge = item.badge ?? String(index + 1)
      return <button
        key={item.id}
        type="button"
        role="option"
        aria-selected={on}
        aria-disabled={available ? undefined : true}
        aria-label={`${item.label}${available ? '' : ' · chưa mở được'}`}
        title={available ? item.label : `${item.label} · chưa vẽ xong`}
        tabIndex={available ? 0 : -1}
        onClick={() => { if (available) onSelect(item.id) }}
        {...stylex.props(sx.thumb, on && sx.thumbOn, !available && sx.thumbWait)}
      >
        {available && item.url && <img alt="" src={item.url} {...stylex.props(sx.thumbImage)} />}
        {!sample && <b {...stylex.props(sx.thumbKey)}>{badge}</b>}
      </button>
    })}
  </div>
}
