// Tuỳ chỉnh › Nâng cao (admin) — Settings.dc.html sec adv: ghi chú vàng, h1, các thẻ nhóm thu gọn.
// Mặc định chỉ nhóm đầu mở (artboard: adv { 0: true } — nhóm “Phiên bản Noverse”); `open` (nút “Đổi mẫu tên
// trong Nâng cao”, nút “Có gì mới”) mở đúng nhóm đó.
import { useEffect, useState, type ReactNode } from 'react'
import { t } from '../../../i18n/vi'
import { Icon } from '@noverse/ui'
import { useVersionInfo, versionNote } from '../../../shell/version'
import type { SettingsContext } from '../types'
import { AccessRows } from './AccessRows'
import { AdvGroup } from './AdvGroup'
import { CommandRows } from './CommandRows'
import { LocationRows } from './LocationRows'
import { MachineDetailCard } from './MachineRows'
import { NamesRows } from './NamesRows'
import { NaoRows } from './NaoRows'
import { TuningRows } from './TuningRows'
import { VersionRows } from './VersionRows'

export type AdvGroupId = 'version' | 'nao' | 'location' | 'names' | 'tuning' | 'access' | 'command'

export function AdvancedSection({ ctx, open: openId, onWhatsNew }: {
  ctx: SettingsContext
  open?: string
  /** D1 — mở hộp “Có gì mới” từ hàng “Bản mới nhất”. */
  onWhatsNew?: () => void
}) {
  const [open, setOpen] = useState<Record<string, boolean>>(() => ({ [openId ?? 'version']: true }))
  // Artboard goAdv: chuyển sang Nâng cao với đúng một nhóm mở.
  useEffect(() => { if (openId) setOpen({ [openId]: true }) }, [openId])
  const info = useVersionInfo()
  const toggle = (id: AdvGroupId) => setOpen((prev) => ({ ...prev, [id]: !prev[id] }))
  const groups: { id: AdvGroupId; icon: string; title: string; note: string; body: ReactNode }[] = [
    {
      id: 'version', icon: 'info-circle', title: 'Phiên bản Noverse', note: versionNote(info),
      body: <VersionRows onWhatsNew={() => onWhatsNew?.()} />,
    },
    { id: 'nao', icon: 'routing-2', title: t.adv.nao.title, note: t.adv.nao.note, body: <NaoRows ctx={ctx} /> },
    { id: 'location', icon: 'folder-open', ...t.adv.location, body: <LocationRows ctx={ctx} /> },
    { id: 'names', icon: 'tag', ...t.adv.names, body: <NamesRows ctx={ctx} /> },
    { id: 'tuning', icon: 'cpu', ...t.adv.tuning, body: <TuningRows ctx={ctx} /> },
    { id: 'access', icon: 'shield-tick', title: t.adv.access.title, note: t.adv.access.note, body: <AccessRows /> },
    { id: 'command', icon: 'code', ...t.adv.command, body: <CommandRows ctx={ctx} /> },
  ]
  return (
    <div style={{ maxWidth: 820, marginTop: 4, display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div style={{
        display: 'flex', gap: 12, alignItems: 'flex-start', padding: '12px 14px', borderRadius: 14,
        background: 'rgba(246,203,140,0.06)', border: '1px solid rgba(246,203,140,0.2)',
      }}>
        <Icon name="warning-2" size={18} style={{ color: '#F6CB8C', marginTop: 1 }} />
        <span style={{ fontSize: 13, color: '#E9D9BD', lineHeight: 1.55 }}>{t.adv.notice}</span>
      </div>
      <h1 style={{ margin: 0, fontSize: 22, fontWeight: 600 }}>{t.adv.title}</h1>
      <MachineDetailCard />
      {groups.map((group) => (
        <AdvGroup key={group.id} icon={group.icon} label={group.title} note={group.note} open={!!open[group.id]} onToggle={() => toggle(group.id)}>
          {group.body}
        </AdvGroup>
      ))}
    </div>
  )
}
