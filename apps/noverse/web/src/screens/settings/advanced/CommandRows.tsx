// Nhóm “Lệnh chạy khi có báo động” — Settings.dc.html advGroups[4]: hàng “Lệnh” (`config.notify.command`, cấp toàn cục).
import { useState } from 'react'
import { TextField } from '@noverse/ui'
import type { SettingsContext } from '../types'
import { AdvButton, AdvRow } from './AdvGroup'
import { commandValue } from './logic'

export function CommandRows({ ctx }: { ctx: SettingsContext }) {
  const command = ctx.draft.notify?.command
  const [editing, setEditing] = useState(false)
  return (
    <AdvRow label="Lệnh" value={commandValue(command)} valueTitle={command || undefined}
      below={editing ? (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8, padding: '4px 0 12px' }}>
          <TextField aria-label="Lệnh chạy khi có báo động" value={command ?? ''} disabled={!ctx.canEdit} placeholder='notify-send "$NOTIFY_TITLE" "$NOTIFY_BODY"'
            onChange={(value) => ctx.setField('notify.command', value.trim() ? value : undefined)}
            style={{ height: 34, fontFamily: 'var(--font-mono)', fontSize: 12 }} />
          <span style={{ fontSize: 12, color: '#8E8E99', lineHeight: 1.5 }}>Để trống = dùng thông báo hệ thống.</span>
        </div>
      ) : undefined}>
      <AdvButton expanded={editing} disabled={!ctx.canEdit} onClick={() => setEditing((value) => !value)}>{command?.trim() ? 'Đổi' : 'Đặt'}</AdvButton>
    </AdvRow>
  )
}
