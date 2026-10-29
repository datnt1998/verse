// Chip “Đang viết chương N” của header khi đang mở trang Hướng dẫn lệnh
// (Commands.dc.html trạng thái busy). Tự ẩn khi trợ lý rảnh hoặc chưa rõ truyện.
import { useCommandSnapshot } from './target'
import { writingChapterOf } from './model'
import { CMD_UI } from '../i18n/commands.vi'
import { AgentOrb } from '@noverse/ui'
import './slash.css'

export function HelpHeaderChip() {
  const { snapshot } = useCommandSnapshot()
  if (!snapshot?.isRunning) return null
  return (
    <span role="status" style={{
      height: 28, padding: '0 10px', borderRadius: 999, background: '#131315',
      border: '1px solid rgba(255,255,255,0.08)', color: 'var(--color-fg-2)', fontSize: 12,
      display: 'flex', alignItems: 'center', gap: 8, whiteSpace: 'nowrap',
    }}>
      <AgentOrb agent="writer" size={12} />
      <span className="cmd-shim">{CMD_UI.busyChip(writingChapterOf(snapshot, snapshot.completedCount))}</span>
    </span>
  )
}
