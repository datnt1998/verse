import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// Ô nhịp 1–50 dùng ở tab Nguồn và Tạo kênh: nút −/+ và ô số; gõ số rồi rời ô
// (hoặc Enter) thì lưu một lần. Artboard Channel/CreateChannel › "Nhịp mỗi ngày".
import { useEffect, useState } from 'react'
import { Button, Icon } from '@noverse/ui'
import { t } from '../../i18n/channels'

export function PaceStepper({ value, onSave, label }: {
  value: number
  onSave: (pace: number) => void
  /** nhãn đọc cho ô số (mặc định "Nhịp mỗi ngày") */
  label?: string
}) {
  const tt = t.channels.sources
  const [draft, setDraft] = useState(String(value))
  useEffect(() => { setDraft(String(value)) }, [value])
  const clamp = (n: number) => Math.min(50, Math.max(1, n))
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
    <span className={`vch-stepper ${stylex.props(channelStyles.stepper).className}`}>
      <Button variant="icon" aria-label={tt.decrease} disabled={value <= 1} onClick={() => step(value - 1)}>
        <Icon name="minus" size={14} />
      </Button>
      <input {...stylex.props(channelStyles.stepperInput)}
        type="number"
        inputMode="numeric"
        min={1}
        max={50}
        aria-label={label ?? tt.pace}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        onBlur={commit}
        onKeyDown={(event) => { if (event.key === 'Enter') commit() }}
      />
      <Button variant="icon" aria-label={tt.increase} disabled={value >= 50} onClick={() => step(value + 1)}>
        <Icon name="add" size={14} />
      </Button>
    </span>
  )
}
