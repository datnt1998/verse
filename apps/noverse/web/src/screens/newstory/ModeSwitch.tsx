// ModeSwitch — Bắt đầu nhanh / Đồng sáng tác (NewStory.dc.html, Tab đổi chế độ).
import { t } from '../../i18n/vi'
import { Icon } from '@noverse/ui'
import { Segmented } from '@noverse/ui'

export type StoryMode = 'quick' | 'cocreate'

const MODES: { id: StoryMode; label: string; icon: string; hint: string }[] = [
  { id: 'quick', label: t.newStory.modeQuick, icon: 'magic-star-bulk', hint: t.newStory.modeQuickHint },
  { id: 'cocreate', label: t.newStory.modeCo, icon: 'messages-2', hint: t.newStory.modeCoHint },
]

export function ModeSwitch({ mode, onChange, compact }: {
  mode: StoryMode
  onChange: (mode: StoryMode) => void
  /** hàng đầu màn Đồng sáng tác: cao 30 thay vì 32 */
  compact?: boolean
}) {
  return (
    <Segmented label="Chế độ khởi động" size={compact ? 30 : 32}
      value={mode} onChange={(next) => onChange(next as StoryMode)}
      options={MODES.map((item) => ({
        id: item.id,
        label: <span title={item.hint}>{item.label}</span>,
        icon: <Icon name={item.icon} size={16} />,
        style: {
          flex: '0 1 auto', padding: compact ? '0 12px 0 10px' : '0 14px 0 12px',
          gap: 8, fontSize: 13, fontWeight: item.id === mode ? 500 : 400,
        },
      }))}
      style={{ gap: 2, background: 'var(--color-surface-1)', border: '1px solid var(--line-1)' }} />
  )
}
