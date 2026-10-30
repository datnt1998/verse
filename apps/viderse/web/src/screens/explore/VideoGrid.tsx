// Lưới video ảo hoá + thẻ video của Khám phá. Mỗi thẻ: ảnh mqdefault
// (loading=lazy, width/height giữ khung — không layout shift), huy hiệu điểm ×
// (vàng ≥3), phụ đề + thời lượng, checkbox ở chế độ chọn nhiều, nút "Làm cho
// <kênh>" khoá vì chưa có kênh của bạn, nút sao chép link.
import type { SyntheticEvent } from 'react'
import { Icon, toast } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { ActionButton } from '../../components'
import type { Video } from '../../api/rivals'
import { fmtDuration, fmtScore, scoreText, videoMeta } from './fmt'

/** Sao chép một link — navigator.clipboard, fallback execCommand. Trả false
 *  khi trình duyệt chặn (toast lỗi ở caller). */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const el = document.createElement('textarea')
      el.value = text
      el.style.position = 'fixed'
      el.style.opacity = '0'
      document.body.append(el)
      el.select()
      const ok = document.execCommand('copy')
      el.remove()
      return ok
    } catch {
      return false
    }
  }
}

/** Ảnh không tải được (mất mạng, YouTube chặn) thì ẩn để thấy nền khung thay
 *  cho biểu tượng ảnh vỡ. Ảnh gắn key theo video nên không ẩn nhầm video khác. */
export function hideBrokenImage(e: SyntheticEvent<HTMLImageElement>) {
  e.currentTarget.style.visibility = 'hidden'
}

export function VideoCard({ video, picked, selectMode, checked, onOpen, onToggle, channelLabel }: {
  video: Video
  picked: boolean
  selectMode: boolean
  checked: boolean
  onOpen: (video: Video) => void
  onToggle: (video: Video) => void
  /** "chưa có kênh của bạn" → nút Làm cho khoá kèm lý do */
  channelLabel: string | null
}) {
  const tt = t.screens.explore
  const hot = (video.score ?? 0) >= 3
  const dur = fmtDuration(video.durationSec)
  const copy = async () => {
    const ok = await copyText(video.url)
    if (ok) toast.ok(tt.copied)
    else toast.err(tt.copyFailed)
  }
  return (
    <article className="vx-card" data-picked={picked || undefined} data-checked={selectMode && checked || undefined}>
      <div className="vx-thumb">
        <button type="button" className="vx-thumb__btn" aria-label={`${tt.preview} ${video.title}`} aria-pressed={picked} onClick={() => onOpen(video)}>
          <img className="vx-thumb__img" src={video.thumbnailUrl} width={320} height={180} loading="lazy" decoding="async" alt="" onError={hideBrokenImage} />
        </button>
        <span className={`vx-score ${hot ? 'vx-score--hot' : 'vx-score--low'}`} title={tt.scoreTitle(scoreText(video.score))}>
          {fmtScore(video.score)}
        </span>
        <span className="vx-thumb__br">
          {video.hasCaptions === true && (
            <span className="vx-subs" role="img" aria-label={tt.hasSubs} title={tt.hasSubs}>
              <Icon name="subtitle" size={13} />
            </span>
          )}
          {dur && <span className="vx-dur">{dur}</span>}
        </span>
        {selectMode && (
          <label className="vx-check">
            <span style={{ position: 'absolute', width: 1, height: 1, overflow: 'hidden', clip: 'rect(0 0 0 0)' }}>{tt.pickVideo(video.title)}</span>
            <input type="checkbox" checked={checked} onChange={() => onToggle(video)} />
          </label>
        )}
      </div>
      <div className="vx-vtext">
        <button type="button" className="vx-vtitle" title={video.title} onClick={() => onOpen(video)}>{video.title}</button>
        <span className="vx-vmeta">{videoMeta(video.viewCount, video.publishedAt)}</span>
      </div>
      <div className="vx-actions">
        <ActionButton
          size="sm"
          disabled={channelLabel === null}
          disabledReason={channelLabel === null ? tt.lockedReason : undefined}
          icon={<Icon name="video-add" size={14} />}
        >
          {channelLabel ? tt.makeFor(channelLabel) : tt.makeVideo}
        </ActionButton>
        <span className="vx-grow" />
        <button type="button" className="vx-iconbtn" aria-label={tt.copyLinkVideo(video.title)} title={tt.copyLinkTitle} onClick={copy}>
          <Icon name="copy" size={15} />
        </button>
      </div>
    </article>
  )
}

export function VideoGrid({ items, cols, start, end, padTop, padBottom, pickedId, selectMode, checkedIds, onOpen, onToggle }: {
  items: Video[]
  cols: number
  /** khoảng item render (đã cắt theo hàng) */
  start: number
  end: number
  padTop: number
  padBottom: number
  pickedId: string | null
  selectMode: boolean
  checkedIds: ReadonlySet<string>
  onOpen: (video: Video) => void
  onToggle: (video: Video) => void
}) {
  const visible = items.slice(start, end)
  const rows: Video[][] = []
  for (let i = 0; i < visible.length; i += cols) rows.push(visible.slice(i, i + cols))
  return (
    <div className="vx-grid">
      {padTop > 0 && <div aria-hidden="true" style={{ height: padTop }} />}
      {rows.map((row, i) => (
        <div className="vx-row" data-vrow={start + i * cols} key={start + i * cols}
          style={{ '--cols': cols } as React.CSSProperties}>
          {row.map((video) => (
            <VideoCard
              key={video.id}
              video={video}
              picked={pickedId === video.id}
              selectMode={selectMode}
              checked={checkedIds.has(video.id)}
              onOpen={onOpen}
              onToggle={onToggle}
              channelLabel={null}
            />
          ))}
        </div>
      ))}
      {padBottom > 0 && <div aria-hidden="true" style={{ height: padBottom }} />}
    </div>
  )
}
