// Điểm Kiểm định: huy hiệu ảnh, tóm tắt và điểm của ảnh đang xem.
import * as stylex from '@stylexjs/stylex'
import { Banner } from '@noverse/ui'
import { t } from '../../i18n/autonomy'
import { readImageScore } from '../../api/autonomy'
import { Orb } from '../Orb'
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

/** Điểm của ảnh đang xem; không mở lớp giải thích riêng. */
export function ScorePanel({ rawScore, min, issue }: { rawScore: unknown; min: number; issue?: string }) {
  const score = readImageScore(rawScore)
  if (issue) return <Banner tone="warning" title={tt.instructionIssue}>{issue}</Banner>
  if (!score) return null
  return (
    <div {...stylex.props(sx.panel)}>
      <div {...stylex.props(sx.panelHead)}>
        <h2 {...stylex.props(sx.panelTitle)}>{tt.panelTitle}</h2>
        <ScoreBadge value={score.value} low={isLow(score, min)} />
      </div>
    </div>
  )
}
