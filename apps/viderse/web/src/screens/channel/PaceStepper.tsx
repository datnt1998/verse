import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Ô “Video mỗi ngày” 1–20 dùng ở tab Nguồn & tự động và Tạo kênh: nút −/+ và
// ô số; gõ số rồi rời ô (hoặc Enter) thì lưu một lần. Artboard
// Channel/CreateChannel/MCreate › run card. Rộng ngoài 1–20 kẹp vào biên khi
// gõ, nút −/+
import { useEffect, useState } from 'react'
import { Button, Icon } from '@noverse/ui'
import { t } from '../../i18n/channels'

const MIN = 1
const MAX = 20

export function PaceStepper({ value, onSave, label, disabled = false }: {
  value: number
  onSave: (pace: number) => void
  /** nhãn đọc cho ô số (mặc định "Video mỗi ngày") */
  label?: string
  disabled?: boolean
}) {
  const tt = t.channels.automation
  const [draft, setDraft] = useState(String(value))
  useEffect(() => { setDraft(String(value)) }, [value])
  const clamp = (n: number) => Math.min(MAX, Math.max(MIN, n))
  const step = (next: number) => {
    const clamped = clamp(next)
    setDraft(String(clamped))
    if (clamped !== value) onSave(clamped)
  }
  const commit = () => {
    const next = clamp(Number.parseInt(draft, 10) || value)
    setDraft(String(next))
    if (next !== value) onSave(next)
  }
  return (
    <span className={`vch-stepper ${stylex.props(channelStyles.stepper).className}`} aria-busy={disabled || undefined}>
      <Button variant="icon" xstyle={channelStyles.stepperBtn} aria-label={tt.decrease} disabled={disabled || value <= MIN} onClick={() => step(value - 1)}>
        <Icon name="minus" size={14} />
      </Button>
      <input {...stylex.props(channelStyles.stepperInput)}
        type="number"
        inputMode="numeric"
        min={MIN}
        max={MAX}
        disabled={disabled}
        aria-label={label ?? t.channels.automation.perDayLabel}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); commit() } }}
      />
      <Button variant="icon" xstyle={channelStyles.stepperBtn} aria-label={tt.increase} disabled={disabled || value >= MAX} onClick={() => step(value + 1)}>
        <Icon name="add" size={14} />
      </Button>
    </span>
  )
}
