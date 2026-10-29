// Màu token cú pháp (13-commands.md §3) + tông nhãn/trạng thái của hai màn lệnh.
import type { SlashBadge, UsageTone } from './model'

export const CMD_COLOR: Record<UsageTone, string> = {
  name: 'var(--cmd-name)',
  value: 'var(--cmd-value)',
  flag: 'var(--cmd-flag)',
  optional: 'var(--cmd-optional)',
  filled: 'var(--cmd-filled)',
}

export const BADGE_TONE: Record<SlashBadge['tone'], { bg: string; bd: string; fg: string }> = {
  neutral: { bg: 'rgba(255,255,255,0.04)', bd: 'rgba(255,255,255,0.08)', fg: 'var(--color-fg-2)' },
  warn: { bg: 'rgba(246,203,140,0.1)', bd: 'rgba(246,203,140,0.3)', fg: 'var(--color-warn)' },
  accent: { bg: 'rgba(201,166,245,0.08)', bd: 'rgba(201,166,245,0.22)', fg: '#DCC8F8' },
  ok: { bg: 'rgba(166,237,193,0.08)', bd: 'rgba(166,237,193,0.25)', fg: 'var(--color-ok)' },
}

export const STATUS_COLOR = {
  err: 'var(--cmd-error-fg)',
  warn: 'var(--color-warn)',
  ok: 'var(--color-ok)',
} as const
