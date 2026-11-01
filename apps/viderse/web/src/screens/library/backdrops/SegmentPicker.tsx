import * as stylex from '@stylexjs/stylex'
import { color } from '@noverse/ui/tokens.stylex'
import { libraryVars } from '../libraryTokens.stylex'
// Chọn đoạn của một video (SegmentPicker): điểm bắt đầu bằng slider (khi biết
// độ dài video) + bước độ dài đoạn — trần 600 giây (10 phút) khớp backend.
// Chưa biết độ dài thì nhập giây bắt đầu, máy chủ kiểm lại khi tải.
import { NumberStepper, Slider } from '@noverse/ui'
import { t } from '../../../i18n/library'
import { timeRange } from './fmt'

export const SEGMENT_MAX_SEC = 600

export function SegmentPicker({ value, videoSec, onChange }: {
  value: { fromSec: number; toSec: number }
  /** tổng giây video; null = chưa biết (link mới parse) */
  videoSec: number | null
  onChange: (segment: { fromSec: number; toSec: number }) => void
}) {
  const copy = t.backdrop.tab.videos
  const segLen = Math.max(1, Math.round(value.toSec - value.fromSec))
  const duration = videoSec && Number.isFinite(videoSec) ? Math.max(1, Math.floor(videoSec)) : null
  const maxLen = duration ? Math.min(SEGMENT_MAX_SEC, duration) : SEGMENT_MAX_SEC
  const fromMax = duration ? Math.max(0, duration - Math.min(segLen, maxLen)) : undefined
  const len = Math.min(segLen, maxLen)
  const from = duration ? Math.min(Math.round(value.fromSec), Math.max(0, duration - len)) : Math.max(0, Math.round(value.fromSec))

  const commit = (fromSec: number, lenSec: number) => {
    onChange({ fromSec, toSec: fromSec + lenSec })
  }

  return (
    <div className={`vbd-segpick ${stylex.props(sx.segpick).className}`}>
      <div className={`vbd-seghead ${stylex.props(sx.seghead).className}`}>
        <span className={`vbd-segtitle ${stylex.props(sx.segtitle).className}`}>{copy.segmentTitle}</span>
        <span className={`num vbd-segval ${stylex.props(sx.numSegval).className}`}>
          {copy.segmentLen(timeRange(len))} · {timeRange(from)}–{timeRange(from + len)}
        </span>
      </div>
      {duration ? (
        <Slider
          value={from}
          min={0}
          max={fromMax ?? 0}
          step={1}
          label={copy.segmentStart}
          ticks={[{ value: 0, label: '0:00' }, { value: fromMax ?? 0, label: timeRange(fromMax ?? 0) }]}
          formatValue={(v) => timeRange(v)}
          onChange={(next) => commit(Math.round(next), len)}
        />
      ) : (
        <NumberStepper buttonStyle={sx.touchTarget}
          value={from}
          min={0}
          max={36000}
          step={10}
          label={copy.segmentStart}
          hint={copy.secondsUnit}
          onChange={(next) => commit(Math.round(next), len)}
        />
      )}
      <NumberStepper buttonStyle={sx.touchTarget}
        value={len}
        min={1}
        max={maxLen}
        step={10}
        label={copy.segmentLength}
        hint={copy.secondsUnit}
        onChange={(next) => commit(from, Math.round(next))}
      />
      <p className={`vbd-segnote ${stylex.props(sx.segnote).className}`}>
        {duration ? copy.segmentMaxNote(timeRange(duration)) : copy.segmentUnknownNote}
      </p>
    </div>
  )
}

const sx = stylex.create({
  segpick: { display: "flex", flexDirection: "column", rowGap: "var(--vv-s-10)", columnGap: "var(--vv-s-10)" },
  seghead: { display: "flex", alignItems: "center", justifyContent: "space-between" },
  segtitle: { fontSize: "var(--vv-s-12)", fontWeight: libraryVars.headingWeight },
  numSegval: { fontSize: "var(--vv-s-11)", color: color["--color-fg-2"] },
  touchTarget: { minHeight: { default: null, "@media (max-width: 767px) and (pointer: coarse)": "var(--vv-s-44)" } },
  segnote: { fontSize: libraryVars.microFontSize, color: color["--color-fg-3"], marginTop: 0, marginRight: 0, marginBottom: 0, marginLeft: 0 },
})
