// Panel "Video đã chọn" ở cột phải khi bấm một thẻ: ảnh, điểm ×, popover "Vì
// sao?" (lượt xem / mức thường / ngưỡng ×3 / lúc đăng), huy hiệu phụ đề, nhóm
// "Làm cho kênh" + dòng kế hoạch + nút chính — chọn kênh và nút chính khoá
// (aria-disabled, vẫn focus được) vì chưa có kênh của bạn; lý do hiện một lần
// ngay dưới nút kèm link "Tạo kênh" (06-copy › Khám phá), cả hai điều khiển trỏ
// aria-describedby tới câu lý do. Link "Mở trên YouTube", nút "← Thông tin đối thủ".
import { useId, useState } from 'react'
import { Icon, useEscapeLayer } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { paths } from '../../router'
import { ActionButton, Orb } from '../../components'
import type { Rival, Video } from '../../api/rivals'
import { fmtDateTime, fmtDuration, fmtScore, fmtSubs, fmtViews, scoreText, videoMeta } from './fmt'
import { hideBrokenImage } from './VideoGrid'

export function PreviewPanel({ video, rival, onClose }: {
  video: Video
  rival: Rival | undefined
  onClose: () => void
}) {
  const tt = t.screens.explore
  const [whyOpen, setWhyOpen] = useState(false)
  useEscapeLayer(whyOpen, () => setWhyOpen(false))
  const reasonId = useId()

  const dur = fmtDuration(video.durationSec)
  const hot = (video.score ?? 0) >= 3
  const meta = videoMeta(video.viewCount, video.publishedAt)
  const score = scoreText(video.score)

  // "Vì sao?" chỉ kể khi có số (06-copy: không có số thì không viết lý do).
  // Ba dòng bằng chứng như artboard (hợp đồng §17 cho 2–4): lượt xem · mức
  // thường · tuổi video. Ngưỡng ×3 đã nằm trong câu lý do nên không lặp lại.
  const facts = video.score != null ? [
    { l: tt.whyFacts.views, v: video.viewCount != null ? fmtViews(video.viewCount) : '—', s: tt.whyFacts.youtubeSrc },
    { l: tt.whyFacts.median, v: rival?.medianViews != null ? fmtViews(rival.medianViews) : '—', s: tt.whyFacts.medianSrc },
    { l: tt.whyFacts.age, v: video.publishedAt ? fmtDateTime(video.publishedAt) : '—', s: video.exactTime ? tt.whyFacts.ageSrc : tt.whyFacts.ageEst },
  ] : null

  return (
    <>
      <button type="button" className="vx-back" onClick={onClose}>
        <Icon name="arrow-r" size={14} />
        {tt.rivalInfo}
      </button>
      <div className="vx-pv">
        <div className="vx-pvthumb">
          <img key={video.id} src={video.thumbnailUrl} width={320} height={180} alt="" onError={hideBrokenImage} />
          <span className={`vx-score ${hot ? 'vx-score--hot' : 'vx-score--low'}`}>{fmtScore(video.score)}</span>
          {dur && <span className="vx-dur" style={{ position: 'absolute', right: 'var(--vv-s-8)', bottom: 'var(--vv-s-7)' }}>{dur}</span>}
        </div>
        <h2 className="vx-pvtitle">{video.title}</h2>
        <span className="vx-pvsub">{rival ? `${rival.title} · ${meta}` : meta}</span>
        <div className="vx-pvscore">
          <span className={`vx-score ${hot ? 'vx-score--hot' : 'vx-score--low'}`}>{fmtScore(video.score)}</span>
          <span className="vx-pvscore__text">{tt.scoreTimes(score)}</span>
          {facts && (
            <button type="button" className="vx-why" aria-haspopup="dialog" aria-expanded={whyOpen} onClick={() => setWhyOpen(!whyOpen)}>
              <Icon name="info" size={13} />
              {tt.why}
            </button>
          )}
        </div>
        {whyOpen && facts && (
          <div className="vx-reason" role="region" aria-label={tt.whyTitle}>
            <div className="vx-reason__head"><Orb of="scout" size={16} /><span>{tt.whyHead}</span></div>
            <div className="vx-reason__text">{tt.whyText(score, rival?.title ?? '')}</div>
            <div className="vx-reason__facts">
              {facts.map((f) => (
                <div className="vx-reason__fact" key={f.l}>
                  <span>{f.l}</span><b>{f.v}</b><span className="vx-reason__src">{f.s}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        {video.hasCaptions === true && (
          <span className="vx-subsbadge"><Icon name="subtitle" size={13} />{tt.hasSubs}</span>
        )}
      </div>
      <div className="vx-makegroup" role="radiogroup" aria-label={tt.makeChannel}>
        <span className="vx-makehead"><Icon name="channel" size={14} />{tt.makeChannel}</span>
        <div className="vx-makeopt" role="radio" aria-checked="false" aria-disabled="true" tabIndex={0} aria-describedby={reasonId}>
          <span className="vx-makeopt__text">
            <span className="vx-makeopt__name">{tt.notLinked}</span>
          </span>
        </div>
      </div>
      <span className="vx-queuehint"><Icon name="clock" size={14} />{tt.queuePlan}</span>
      <ActionButton
        variant="primary"
        size="lg"
        lockedReason={tt.lockedReason}
        aria-describedby={reasonId}
        icon={<Icon name="video-add" size={18} />}
      >
        {tt.makeVideo}
      </ActionButton>
      <p className="vx-locknote">
        <span id={reasonId}>{tt.lockedReason}</span>{' '}
        <a href={`#${paths.channelNew()}`}>{tt.createChannel}</a>
      </p>
      <a href={video.url} target="_blank" rel="noreferrer" className="vx-linkbtn">
        <Icon name="link" size={14} />
        {tt.openYouTube}
      </a>
    </>
  )
}
