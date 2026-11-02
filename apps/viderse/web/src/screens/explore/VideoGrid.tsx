import * as stylex from '@stylexjs/stylex'
import { exploreStyles, videoCardMarker } from './explore.stylex'
// Lưới video ảo hoá + thẻ video của Khám phá. Mỗi thẻ: ảnh mqdefault
// (loading=lazy, width/height giữ khung — không layout shift), huy hiệu điểm ×
// (vàng ≥3), phụ đề + thời lượng, checkbox ở chế độ chọn nhiều, nút "Làm cho
// <kênh>" mở panel xem trước (chọn kênh + kế hoạch ở đó), nút sao chép link.
import type { SyntheticEvent } from 'react'
import { Button, Checkbox, Icon, toast } from '@noverse/ui'
import { t } from '../../i18n/explore'
import { ActionButton } from '../../components'
import type { Video } from '../../api/rivals'
import { fmtDuration, fmtScore, scoreText, videoMeta } from './fmt'
import { stageColor, stageLabel, usageHref } from './status'

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
  /** kênh đích đầu tiên — nút ghi "Làm cho <kênh>" như artboard */
  channelLabel: string | null
}) {
  const tt = t.screens.explore
  const hot = (video.score ?? 0) >= 3
  const dur = fmtDuration(video.durationSec)
  const usage = video.usage?.[0]
  const usageTitle = video.usage?.map((u) => `${u.channelName} · ${stageLabel(u)}`).join('\n')
  const copy = async () => {
    const ok = await copyText(video.url)
    if (ok) toast.ok(tt.copied)
    else toast.err(tt.copyFailed)
  }
  return (
    <article className={`vx-card ${stylex.props(exploreStyles.card, videoCardMarker).className}`} data-picked={picked || undefined} data-checked={selectMode && checked || undefined}>
      <div className={`vx-thumb ${stylex.props(exploreStyles.thumb, picked && exploreStyles.thumbPicked, selectMode && checked && exploreStyles.thumbChecked).className}`}>
        <button type="button" className={`vx-thumb__btn ${stylex.props(exploreStyles.thumbBtn).className}`} aria-label={`${tt.preview} ${video.title}`} aria-pressed={picked} onClick={() => onOpen(video)}>
          <img className={`vx-thumb__img ${stylex.props(exploreStyles.thumbImg).className}`} src={video.thumbnailUrl} width={320} height={180} loading="lazy" decoding="async" alt="" onError={hideBrokenImage} />
        </button>
        <span className={`vx-score ${hot ? 'vx-score--hot' : 'vx-score--low'} ${stylex.props(exploreStyles.score, hot ? exploreStyles.scoreHot : exploreStyles.scoreLow).className}`} title={tt.scoreTitle(scoreText(video.score))}>
          {fmtScore(video.score)}
        </span>
        <span className={`vx-thumb__br ${stylex.props(exploreStyles.thumbBr).className}`}>
          {video.hasCaptions === true && (
            <span className={`vx-subs ${stylex.props(exploreStyles.subs).className}`} role="img" aria-label={tt.hasSubs} title={tt.hasSubs}>
              <Icon name="subtitle" size={13} />
            </span>
          )}
          {dur && <span className={`vx-dur ${stylex.props(exploreStyles.dur).className}`}>{dur}</span>}
        </span>
        {selectMode && (
          <span className={`vx-check ${stylex.props(exploreStyles.check).className}`}>
            <Checkbox aria-label={tt.pickVideo(video.title)} checked={checked} onChange={() => onToggle(video)} />
          </span>
        )}
      </div>
      <div className={`vx-vtext ${stylex.props(exploreStyles.vtext).className}`}>
        <button type="button" className={`vx-vtitle ${stylex.props(exploreStyles.vtitle).className}`} title={video.title} onClick={() => onOpen(video)}>{video.title}</button>
        <span className={`vx-vmeta ${stylex.props(exploreStyles.vmeta).className}`}>{videoMeta(video.viewCount, video.publishedAt)}</span>
      </div>
      <div className={`vx-actions ${stylex.props(exploreStyles.actions).className}`}>
        {usage ? (
          <a
            href={usageHref(usage)}
            className={`vx-used ${stylex.props(exploreStyles.usedStatus).className}`}
            style={{ color: stageColor(usage.stage) }}
            title={usageTitle}
            aria-label={`${usage.channelName} · ${stageLabel(usage)}`}
          >
            <span aria-hidden="true" className={stylex.props(exploreStyles.usedDot).className} style={{ backgroundColor: stageColor(usage.stage) }} />
            <span className={stylex.props(exploreStyles.usedText).className}>{usage.channelName} · {stageLabel(usage)}</span>
          </a>
        ) : (
          <ActionButton
            size="sm"
            onClick={() => onOpen(video)}
            icon={<Icon name="video-add" size={14} />}
          >
            {channelLabel ? tt.makeFor(channelLabel) : tt.makeVideo}
          </ActionButton>
        )}
        <span className={`vx-grow ${stylex.props(exploreStyles.grow).className}`} />
        <Button variant="icon" size="sm" aria-label={tt.copyLinkVideo(video.title)} title={tt.copyLinkTitle} onClick={copy}><Icon name="copy" size={15} /></Button>
      </div>
    </article>
  )
}

export function VideoGrid({ items, cols, start, end, padTop, padBottom, pickedId, selectMode, checkedIds, onOpen, onToggle, makeChannelName }: {
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
  /** kênh của tôi đầu tiên — nhãn "Làm cho <kênh>" trên thẻ */
  makeChannelName?: string
}) {
  const visible = items.slice(start, end)
  const rows: Video[][] = []
  for (let i = 0; i < visible.length; i += cols) rows.push(visible.slice(i, i + cols))
  return (
    <div className={`vx-grid ${stylex.props(exploreStyles.grid).className}`}>
      {padTop > 0 && <div aria-hidden="true" style={{ height: padTop }} />}
      {rows.map((row, i) => (
        <div className={`vx-row ${stylex.props(exploreStyles.row).className}`} data-vrow={start + i * cols} key={start + i * cols}
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
              channelLabel={makeChannelName ?? null}
            />
          ))}
        </div>
      ))}
      {padBottom > 0 && <div aria-hidden="true" style={{ height: padBottom }} />}
    </div>
  )
}
