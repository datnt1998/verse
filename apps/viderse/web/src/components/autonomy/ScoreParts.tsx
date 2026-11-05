// Điểm Kiểm định trên Review (artboard Review › score=true): huy hiệu điểm góc
// ảnh, dòng tóm tắt có nút đi tới ảnh thấp nhất, và khối "Điểm của Kiểm định" +
// Vì sao? ở cột phải (ReasonPopover dùng chung).
import * as stylex from '@stylexjs/stylex'
import { useState } from 'react'
import { Banner, Icon } from '@noverse/ui'
import { t } from '../../i18n/autonomy'
import { readImageScore, type ImageScore } from '../../api/autonomy'
import { Orb } from '../Orb'
import { ReasonPopover } from '../reason/ReasonPopover'
import { isLow, type ScoreSummary } from './scoreSummary'
import { scoreStyles as sx } from './score.stylex'

const tt = t.autonomy.score

/** Số điểm trong khung tối; dưới ngưỡng thì nền cam. `corner` = góc dưới trái của ô ảnh. */
export function ScoreBadge({ value, low, corner = false }: { value: number; low: boolean; corner?: boolean }) {
  return <span aria-label={tt.badge(value)} {...stylex.props(sx.badge, low && sx.badgeLow, corner && sx.badgeCorner)}>{value}</span>
}

const lower = (label: string) => label.charAt(0).toLocaleLowerCase('vi') + label.slice(1)

/** Dòng tóm tắt trên dải ảnh; nút nhảy tới ảnh thấp nhất. */
export function ScoreBanner({ summary, autoApprove, channelName, onJump }: {
  summary: ScoreSummary
  autoApprove: boolean
  channelName: string
  onJump: (id: string) => void
}) {
  const [lowest] = summary.below
  const tail = !channelName
    ? ''
    : autoApprove
      ? lowest ? tt.scoreTail(channelName, summary.min) : ''
      : tt.askTail(channelName)
  return (
    <div role="status" {...stylex.props(sx.banner)}>
      <Orb of="qa" size={16} />
      <span {...stylex.props(sx.bannerText)}>
        {tt.lead(summary.scored)}{' '}
        {summary.passed > 0 && <b {...stylex.props(sx.strong)}>{tt.passed(summary.passed, summary.min, summary.below.length === 0)}</b>}
        {summary.passed > 0 && summary.below.length > 0 && ', '}
        {summary.below.map((image, index) => (
          <span key={image.id}>
            {index > 0 && ', '}
            <b {...stylex.props(sx.strong, sx.strongLow)}>{tt.below(lower(image.label), image.value)}</b>
          </span>
        ))}
        {summary.below.length > 0 ? tt.lookFirst : '.'}
        {tail && ` ${tail}`}
        {summary.pending > 0 && ` ${tt.pending(summary.pending)}`}
      </span>
      {lowest && <button type="button" {...stylex.props(sx.jump)} onClick={() => onJump(lowest.id)}>{tt.jump(lower(lowest.label))}</button>}
    </div>
  )
}

/** Cột phải: điểm của ảnh đang xem + Vì sao? (mở sẵn khi ảnh dưới ngưỡng). */
export function ScorePanel({ imageId, rawScore, min, issue }: { imageId: string; rawScore: unknown; min: number; issue?: string }) {
  const score = readImageScore(rawScore)
  // Mở/đóng nhớ theo từng ảnh: đổi ảnh thì quay về mặc định (ảnh thấp mở sẵn).
  const [toggled, setToggled] = useState<{ id: string; open: boolean } | null>(null)
  if (issue) return <Banner tone="warning" title={tt.instructionIssue}>{issue}</Banner>
  if (!score) return null
  const low = isLow(score, min)
  const open = toggled?.id === imageId ? toggled.open : low
  return <ScorePanelBody score={score} low={low} open={open} onToggle={() => setToggled({ id: imageId, open: !open })} />
}

function ScorePanelBody({ score, low, open, onToggle }: { score: ImageScore; low: boolean; open: boolean; onToggle: () => void }) {
  return (
    <div {...stylex.props(sx.panel)}>
      <div {...stylex.props(sx.panelHead)}>
        <h2 {...stylex.props(sx.panelTitle)}>{tt.panelTitle}</h2>
        <ScoreBadge value={score.value} low={low} />
        <span {...stylex.props(sx.spacer)} />
        <button type="button" aria-expanded={open} {...stylex.props(sx.why, open && sx.whyOpen)} onClick={onToggle}>
          <Icon name="info-circle" size={13} />{tt.why}
        </button>
      </div>
      {open && <ReasonPopover reason={score.reason} />}
    </div>
  )
}
