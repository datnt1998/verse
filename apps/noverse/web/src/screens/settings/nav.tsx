// SettingsNav — menu trái 248px của Settings.dc.html: SÁNG TÁC + HỆ THỐNG · QUẢN TRỊ (vương miện, chỉ quản trị).
import { useSession } from '../../api/session'
import { t, rulesCopy, membersCopy } from '../../i18n/vi'
import { paths } from '../../router'
import { Icon } from '@noverse/ui'
import type { ConfigScope, SystemInfo } from '../../api/types'
import { buildStamp, hasNewVersion, versionText } from '../../shell/version'
import type { PageId, SectionId } from './lib/target'
import { StorageNavSummary } from './storage/indicators'

type Item = { id: SectionId; label: string; icon: string }

const CREATIVE: Item[] = [
  { id: 'rules', label: rulesCopy.nav, icon: 'edit-2' },
  { id: 'writing', label: 'Cách viết', icon: 'note-text' },
  { id: 'models', label: 'Trợ lý AI', icon: 'cpu' },
  { id: 'budget', label: 'Ngân sách', icon: 'coin-1' },
  { id: 'info', label: 'Thông tin', icon: 'document-text' },
]

const SYSTEM: Item[] = [
  { id: 'members', label: membersCopy.title, icon: 'profile-2user' },
  { id: 'storage', label: 'Lưu trữ tệp', icon: 'box' },
  { id: 'connect', label: 'Kết nối AI', icon: 'cloud-connection' },
  { id: 'export', label: 'Xuất tự động', icon: 'export-square' },
  { id: 'notify', label: 'Báo động máy chủ', icon: 'notification' },
  { id: 'app', label: 'Giao diện', icon: 'monitor' },
  { id: 'adv', label: 'Nâng cao', icon: 'code' },
]

function NavButton({ item, current, scope, dot, count, onPick }: {
  item: Item; current: PageId; scope: ConfigScope; dot?: boolean; count?: number; onPick: (id: SectionId) => void
}) {
  const on = item.id === current
  const label = item.id === 'info' ? (scope === 'story' ? 'Thông tin truyện' : 'Thông tin dự án') : item.label
  return (
    <button type="button" className={`st-nav${on ? ' on' : ''}`} aria-current={on ? 'page' : 'false'} onClick={() => onPick(item.id)}>
      <Icon name={item.icon} size={17} style={{ color: on ? '#C9A6F5' : 'currentColor' }} />
      <span style={{ flex: '1 1 auto', minWidth: 0 }}>{label}{item.id === 'storage' && <StorageNavSummary />}</span>
      {dot && count === undefined && <span title="Có chỉnh riêng ở cấp này" style={{ width: 6, height: 6, borderRadius: '50%', background: '#C9A6F5' }} />}
      {count !== undefined && <span className="num" style={{ fontSize: 12, color: '#7E7E8A' }}>{count}</span>}
    </button>
  )
}

export function SettingsNav({ current, scope, admin, rulesCount, membersCount, dots, version, onWhatsNew, onPick }: {
  current: PageId
  scope: ConfigScope
  admin: boolean
  /** số quy tắc đang hiệu lực ở cấp này; `undefined` khi chưa tải */
  rulesCount?: number
  membersCount?: number
  /** mục có chỉnh riêng ở cấp đang chọn → chấm tím */
  dots: Partial<Record<SectionId, boolean>>
  /** D1 — `GET /api/system`: chân thanh bên hiện “Noverse 0.9.3” + badge “Có bản mới”. */
  version?: SystemInfo | null
  onWhatsNew?: () => void
  onPick: (id: SectionId) => void
}) {
  const { pendingLessons } = useSession()
  return (
    <aside className="scroll" aria-label="Mục tuỳ chỉnh" style={{
      flex: '0 0 248px', padding: '10px 10px 20px', borderRight: '1px solid rgba(255,255,255,0.06)',
      display: 'flex', flexDirection: 'column',
    }}>
      <div className="st-ngrp" style={{ paddingTop: 8 }}>SÁNG TÁC</div>
      {CREATIVE.map((item) => (
        <NavButton key={item.id} item={item} current={current} scope={scope} onPick={onPick}
          dot={dots[item.id]} count={item.id === 'rules' ? rulesCount : undefined} />
      ))}
      {admin && (
        <>
          <div className="st-ngrp">
            <Icon name="crown" size={12} style={{ color: '#B89A6E' }} />HỆ THỐNG · QUẢN TRỊ
          </div>
          {/* D15 — đứng đầu nhóm Hệ thống, kèm số bài học chờ duyệt (Settings.dc.html). */}
          <a className="st-nav" href={`#${paths.sysbrain()}`} title={t.sysbrain.navTitle(pendingLessons)}>
            <Icon name="lamp-on" size={17} />
            <span style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', lineHeight: 1.3 }}>
              <span>{t.sysbrain.title}</span>
              <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{t.sysbrain.sub}</span>
            </span>
            {pendingLessons > 0 && (
              <span className="num" style={{
                height: 18, minWidth: 18, padding: '0 5px', borderRadius: 9, background: 'var(--sysbrain-count-bg)',
                color: 'var(--kit-badge-run)', fontSize: 11, fontWeight: 600, display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
              }}>{pendingLessons}</span>
            )}
          </a>
          {SYSTEM.map((item) => <NavButton key={item.id} item={item} current={current} scope={scope} count={item.id === 'members' ? membersCount : undefined} onPick={onPick} />)}
        </>
      )}
      <div style={{ marginTop: 'auto', padding: '14px 10px 0', fontSize: 12, color: '#6E6E78', lineHeight: 1.5 }}>
        {current === 'members' ? membersCopy.navNote : 'Thay đổi ghi vào đúng cấp bạn chọn ở trên cùng khi bấm Áp dụng. Truyện kế thừa từ dự án, dự án kế thừa từ “Mọi truyện”.'}
      </div>
      <button type="button" className="hov-ghost" onClick={onWhatsNew} style={{
        margin: '12px 0 0', height: 34, padding: '0 10px', border: 'none', borderRadius: 9, background: 'transparent',
        color: 'var(--kit-fg-icon)', fontSize: 12, display: 'flex', alignItems: 'center', gap: 8, textAlign: 'left',
      }}>
        <Icon name="info-circle" size={14} />
        <span className="num" title={buildStamp(version?.built) ? `Build ${buildStamp(version?.built)}` : undefined}
          style={{ flex: '1 1 auto', whiteSpace: 'nowrap' }}>
          {versionText(version?.version) ? `Noverse ${versionText(version?.version)}` : 'Noverse'}
        </span>
        {hasNewVersion(version) && (
          <span style={{
            height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11, display: 'inline-flex',
            alignItems: 'center', background: 'var(--kit-up-badge-bg)', color: 'var(--kit-up-badge-fg)', whiteSpace: 'nowrap',
          }}>Có bản mới</span>
        )}
      </button>
    </aside>
  )
}
